package chat

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

type v3ProgressSample struct {
	bytes, chunkBytes       int64
	ackLatency              time.Duration
	throughput              float64
	tuningState, reason     string
	goodSamples, badSamples int
}

type v3AdaptiveTuner struct {
	chunkBytes                int
	ewmaThroughput, ewmaAckMS float64
	goodSamples, badSamples   int
	cooldown                  int
}

func newV3AdaptiveTuner(initial int) *v3AdaptiveTuner {
	if !validTransferChunkSize(initial) {
		initial = defaultTransferChunkSize
	}
	return &v3AdaptiveTuner{chunkBytes: initial}
}

func (t *v3AdaptiveTuner) Observe(bytes int64, latency time.Duration) v3ProgressSample {
	seconds := latency.Seconds()
	if seconds <= 0 {
		seconds = time.Millisecond.Seconds()
	}
	throughput := float64(bytes) / seconds
	previousThroughput := t.ewmaThroughput
	t.ewmaThroughput = updateTransferEWMA(t.ewmaThroughput, throughput)
	t.ewmaAckMS = updateTransferEWMA(t.ewmaAckMS, float64(latency.Milliseconds()))
	sample := v3ProgressSample{bytes: bytes, chunkBytes: int64(t.chunkBytes), ackLatency: latency, throughput: t.ewmaThroughput, tuningState: "stable"}
	if t.cooldown > 0 {
		t.cooldown--
		sample.tuningState = "cooldown"
		sample.goodSamples, sample.badSamples = t.goodSamples, t.badSamples
		return sample
	}
	bad := t.ewmaAckMS > 150 || previousThroughput > 0 && t.ewmaThroughput < previousThroughput*0.70
	good := t.ewmaAckMS <= 75 && (previousThroughput == 0 || t.ewmaThroughput >= previousThroughput*0.85)
	switch {
	case bad:
		t.badSamples++
		t.goodSamples = 0
		if t.badSamples >= 3 {
			previous := t.chunkBytes
			t.chunkBytes = maxInt(minTransferChunkSize, t.chunkBytes/2)
			t.badSamples = 0
			t.cooldown = 2
			if t.chunkBytes < previous {
				sample.tuningState, sample.reason = "backing_off", "连续 3 个确认延迟或吞吐坏样本，降低 v3 分块"
			}
		}
	case good:
		t.goodSamples++
		t.badSamples = 0
		if t.goodSamples >= 5 {
			previous := t.chunkBytes
			t.chunkBytes = nextTransferChunkSize(t.chunkBytes)
			t.goodSamples = 0
			t.cooldown = 2
			if t.chunkBytes > previous {
				sample.tuningState, sample.reason = "accelerating", "连续 5 个稳定样本，增大 v3 分块"
			}
		}
	default:
		t.goodSamples, t.badSamples = 0, 0
	}
	sample.chunkBytes = int64(t.chunkBytes)
	sample.goodSamples, sample.badSamples = t.goodSamples, t.badSamples
	return sample
}

func isTemporaryNetError(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(net.Error); ok {
		return e.Timeout() || e.Temporary()
	}
	return false
}

func (e *Engine) dialV3PeerData(ctx context.Context, peer Peer) (net.Conn, error) {
	if peer.DataPort <= 0 {
		return nil, os.ErrInvalid
	}
	config, err := e.clientTLSConfig(peer)
	if err != nil {
		return nil, err
	}
	transport := newTLSTCPV3DataTransport(append([]string{peer.IP}, peer.LocalAddresses...), peer.DataPort, config)
	conn, err := transport.OpenStream(ctx)
	if err != nil {
		return nil, err
	}
	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := verifyPeerCertificate(tlsConn, peer); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

func (e *Engine) pooledV3DataConn(ctx context.Context, peer Peer, slot *PoolSlot) (net.Conn, error) {
	if slot == nil {
		return nil, os.ErrInvalid
	}
	if conn := slot.Connection(); conn != nil {
		_ = conn.SetDeadline(time.Time{})
		return conn, nil
	}
	conn, err := e.dialV3PeerData(ctx, peer)
	if err != nil {
		return nil, err
	}
	slot.SetConnection(conn)
	return conn, nil
}

func (e *Engine) v3Pool(peerID string, limit int) *PeerPool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.peerPools == nil {
		e.peerPools = make(map[string]*PeerPool)
	}
	pool := e.peerPools[peerID]
	if pool == nil {
		pool = NewPeerPool(peerID, limit)
		e.peerPools[peerID] = pool
	} else {
		pool.EnsureLimit(limit)
	}
	return pool
}

func (e *Engine) sendV3FileDataParallel(ctx context.Context, peer Peer, message Message, file *os.File, fullSHA string, streams int, startOffset int64, completed []ByteRange) error {
	if startOffset < 0 || startOffset > message.AttachmentSize {
		return os.ErrInvalid
	}
	if startOffset > 0 {
		completed = append(completed, ByteRange{Start: 0, End: startOffset})
	}
	if streams < 1 {
		streams = 1
	}
	if streams > parallelMaxStreams {
		streams = parallelMaxStreams
	}
	if message.RelativePath != "" && streams > 4 {
		streams = 4
	}
	for _, r := range completed {
		if r.Start < 0 || r.End < r.Start || r.End > message.AttachmentSize {
			return os.ErrInvalid
		}
	}
	assignments := partitionV3Ranges(message.AttachmentSize, completed, streams)
	// Even empty and fully resumed files must complete the receiver's hash/rename handshake.
	if len(assignments) == 0 {
		assignments = [][]ByteRange{{}}
	}
	digest, err := v3FileDigest(file, fullSHA)
	if err != nil {
		return err
	}
	pool := e.v3Pool(peer.DeviceID, len(assignments))
	var progressMu sync.Mutex
	completedBytes := message.AttachmentSize
	progressStarted := time.Now()
	lastProgressAt := progressStarted
	lastProgressBytes := completedBytes
	for _, ranges := range assignments {
		for _, r := range ranges {
			completedBytes -= r.End - r.Start
		}
	}
	progress := func(sample v3ProgressSample) {
		progressMu.Lock()
		defer progressMu.Unlock()
		completedBytes += sample.bytes
		now := time.Now()
		elapsed := now.Sub(lastProgressAt)
		if elapsed <= 0 {
			elapsed = time.Millisecond
		}
		rate := 0.0
		if elapsed >= transferSpeedMinimumSampleInterval && completedBytes > lastProgressBytes {
			rate = float64(completedBytes-lastProgressBytes) / elapsed.Seconds()
			lastProgressAt, lastProgressBytes = now, completedBytes
		}
		if rate <= 0 && sample.throughput > 0 {
			rate = sample.throughput
		}
		avg := float64(completedBytes) / now.Sub(progressStarted).Seconds()
		options := transferProgressOptions{sessionID: fmt.Sprintf("%x", pool.SessionID()), generation: pool.Generation(), chunkSize: int(sample.chunkBytes), windowBytes: int64(pool.Active()) * sample.chunkBytes, activeStreams: pool.Active(), streamCount: len(assignments), transferMode: v3TransferMode, transport: "TLS13/TCP-v3", ackLatency: sample.ackLatency, confirmedThroughput: rate, windowThroughput: rate, displayLocalMetrics: true, tuningState: sample.tuningState, tuningReason: sample.reason, goodSamples: sample.goodSamples, badSamples: sample.badSamples}
		e.emitTransferProgress(message.MessageID, message.AttachmentID, peer.DeviceID, completedBytes, message.AttachmentSize, "send", "transferring", options)
		// The sender's primary progress is the remote durable byte count. Emit
		// the same sample under remote-receive so the UI can calculate ETA from
		// confirmed bytes instead of the local socket write position.
		e.emitTransferProgress(message.MessageID, message.AttachmentID, peer.DeviceID, completedBytes, message.AttachmentSize, "remote-receive", "receiving", options)
		_ = avg
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(assignments))
	for _, ranges := range assignments {
		wg.Add(1)
		go func(ranges []ByteRange) {
			defer wg.Done()
			errs <- e.sendV3Worker(ctx, peer, message, file, digest, pool, ranges, progress)
		}(ranges)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// ACKs are scoped to the exact chunk and authenticated session, not just a
// sequence number that can be reused by a different file or replacement slot.
func matchesV3Reply(reply, request BinaryFrameV3, typ V3FrameType) bool {
	return reply.Type == typ && reply.TransferID == request.TransferID &&
		reply.SessionID == request.SessionID && reply.Generation == request.Generation &&
		reply.StreamID == request.StreamID && reply.Sequence == request.Sequence &&
		reply.Offset == request.Offset && reply.Length == 0 && len(reply.Payload) == 0
}

func v3FileDigest(file *os.File, fullSHA string) ([32]byte, error) {
	var digest [32]byte
	if fullSHA != "" {
		b, err := hex.DecodeString(fullSHA)
		if err != nil || len(b) != len(digest) {
			return digest, fmt.Errorf("invalid file SHA-256")
		}
		copy(digest[:], b)
	} else {
		info, err := file.Stat()
		if err != nil {
			return digest, err
		}
		h := sha256.New()
		if _, err := io.Copy(h, io.NewSectionReader(file, 0, info.Size())); err != nil {
			return digest, err
		}
		copy(digest[:], h.Sum(nil))
	}
	return digest, nil
}

func writeV3Frame(conn net.Conn, frame BinaryFrameV3) error {
	raw, err := frame.MarshalBinary()
	if err != nil {
		return err
	}
	for len(raw) > 0 {
		n, err := conn.Write(raw)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		raw = raw[n:]
	}
	return nil
}

func (e *Engine) sendV3Worker(ctx context.Context, peer Peer, message Message, file *os.File, digest [32]byte, pool *PeerPool, ranges []ByteRange, progress ...func(v3ProgressSample)) (result error) {
	slot, err := pool.Acquire(ctx, message.AttachmentID)
	if err != nil {
		return err
	}
	defer pool.Release(slot)
	defer func() {
		if result != nil {
			slot.Kill()
			pool.ReplaceDead()
		}
	}()
	conn, err := e.pooledV3DataConn(ctx, peer, slot)
	if err != nil {
		return err
	}
	// Interrupt blocking IO when the transfer is canceled. Stop this callback
	// before returning the connection to the pool.
	canceled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { slot.Kill(); close(canceled) })
	defer func() {
		if !stop() {
			<-canceled
		}
	}()
	id, sessionID := binaryTransferID(message.AttachmentID), pool.SessionID()
	streamID, generation := uint16(slot.ID), slot.Generation
	begin := BinaryFrameV3{Type: FrameBeginFile, TransferID: id, StreamID: streamID, SessionID: sessionID, Generation: generation}
	open := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		conn, err = e.pooledV3DataConn(ctx, peer, slot)
		if err != nil {
			return err
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		return writeV3Frame(conn, begin)
	}
	if err := open(); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != message.AttachmentSize {
		return fmt.Errorf("SOURCE_CHANGED")
	}
	profile := LinkProfileV3{Type: peer.LinkType, SpeedMbps: peer.LinkSpeedMbps}
	tuner := newV3AdaptiveTuner(profile.ChunkBytes(message.AttachmentSize))
	buf := make([]byte, maxTransferChunkSize)
	var seq uint64
	for _, r := range ranges {
		if r.Start < 0 || r.End < r.Start || r.End > message.AttachmentSize {
			return os.ErrInvalid
		}
		for off := r.Start; off < r.End; {
			if err := ctx.Err(); err != nil {
				return err
			}
			n := min(int64(tuner.chunkBytes), r.End-off)
			if _, err := io.ReadFull(io.NewSectionReader(file, off, n), buf[:n]); err != nil {
				return err
			}
			frame := NewChunkFrame(id, streamID, seq, uint64(off), buf[:n])
			frame.SessionID, frame.Generation = sessionID, generation
			ackStarted := time.Now()
			acknowledged := false
			for attempt := 0; attempt < 3; attempt++ {
				if conn == nil {
					if err = open(); err != nil {
						continue
					}
				}
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				err = writeV3Frame(conn, frame)
				var ack BinaryFrameV3
				if err == nil {
					ack, err = ReadBinaryFrameV3(conn, 1024)
				}
				if err == nil && matchesV3Reply(ack, frame, FrameChunkAck) {
					acknowledged = true
					break
				}
				if err == nil && !matchesV3Reply(ack, frame, FrameChunkNack) {
					return fmt.Errorf("v3 stale or invalid acknowledgement")
				}
				if attempt == 2 {
					break
				}
				// A timed-out read may have consumed half a frame. Never reuse that byte
				// stream: reconnect and resend only this unconfirmed, idempotent chunk.
				slot.CloseConnection()
				if err = open(); err != nil {
					continue
				}
			}
			if !acknowledged {
				return fmt.Errorf("v3 chunk %d unconfirmed: %w", seq, errors.Join(err, io.ErrUnexpectedEOF))
			}
			if err := e.persistOutgoingConfirmedRange(message.AttachmentID, off, off+n, fmt.Sprintf("%x", sessionID), generation); err != nil {
				return newTransferError(ErrSessionNotReady, true, fmt.Errorf("保存发送恢复范围失败: %w", err))
			}
			slot.Progress(n, n)
			sample := tuner.Observe(n, time.Since(ackStarted))
			if len(progress) > 0 {
				progress[0](sample)
			}
			off += n
			seq++
		}
	}
	current, err := file.Stat()
	if err != nil {
		return err
	}
	if current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("SOURCE_CHANGED")
	}
	end := BinaryFrameV3{Type: FrameEndFile, TransferID: id, StreamID: streamID, Sequence: seq, Offset: uint64(message.AttachmentSize), ChunkHash: digest, SessionID: sessionID, Generation: generation}
	// Hashing a multi-gigabyte file on slow storage can take minutes.
	slot.Drain()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Minute))
	if err := writeV3Frame(conn, end); err != nil {
		return err
	}
	reply, err := ReadBinaryFrameV3(conn, 1024)
	if err != nil {
		return err
	}
	if !matchesV3Reply(reply, end, FrameEndFile) || reply.ChunkHash != digest {
		return fmt.Errorf("v3 receiver did not confirm file verification")
	}
	_ = conn.SetDeadline(time.Time{})
	return nil
}

func partitionV3Ranges(size int64, completed []ByteRange, streams int) [][]ByteRange {
	if size <= 0 {
		return nil
	}
	missing := MissingRanges(size, completed)
	var total int64
	for _, r := range missing {
		total += r.End - r.Start
	}
	if total == 0 {
		return nil
	}
	if streams < 1 {
		streams = 1
	}
	if int64(streams) > total {
		streams = int(total)
	}
	out := make([][]ByteRange, streams)
	target := (total + int64(streams) - 1) / int64(streams)
	index, assigned := 0, int64(0)
	for _, r := range missing {
		for r.Start < r.End {
			if index >= streams {
				index = streams - 1
			}
			capacity := target - assigned
			if capacity <= 0 && index < streams-1 {
				index++
				assigned = 0
				capacity = target
			}
			length := r.End - r.Start
			if length > capacity {
				length = capacity
			}
			out[index] = append(out[index], ByteRange{Start: r.Start, End: r.Start + length})
			r.Start += length
			assigned += length
		}
	}
	return out
}

func (e *Engine) sendV3FileData(ctx context.Context, peer Peer, message Message, file *os.File, fullSHA string, startOffset int64) error {
	return e.sendV3FileDataRanges(ctx, peer, message, file, fullSHA, []ByteRange{{Start: startOffset, End: message.AttachmentSize}})
}

func (e *Engine) sendV3FileDataRanges(ctx context.Context, peer Peer, message Message, file *os.File, fullSHA string, ranges []ByteRange) error {
	digest, err := v3FileDigest(file, fullSHA)
	if err != nil {
		return err
	}
	return e.sendV3Worker(ctx, peer, message, file, digest, e.v3Pool(peer.DeviceID, 1), ranges)
}
