package chat

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// dataAcceptLoop reserves the dedicated listener for v3 data sessions. The
// transfer dispatcher will attach an accepted connection after the control
// offer identifies its transfer ID; unknown probes are closed immediately.
func (e *Engine) dataAcceptLoop() {
	e.mu.RLock()
	ln := e.dataListener
	stop := e.stop
	e.mu.RUnlock()
	if ln == nil {
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-stop:
				return
			default:
				log.Printf("v3 数据连接接受失败: %s", redactDiagnosticError(err))
				return
			}
		}
		go func(c net.Conn) {
			defer c.Close()
			authenticatedPeerID := ""
			if tc, ok := c.(*tls.Conn); ok {
				if err := tc.HandshakeContext(context.Background()); err != nil {
					return
				}
				peer, err := e.authenticateInboundDataPeer(tc.ConnectionState())
				if err != nil {
					return
				}
				authenticatedPeerID = peer.DeviceID
			}
			_ = e.receiveV3DataConnection(c, authenticatedPeerID)
		}(conn)
	}
}

func (e *Engine) authenticateInboundDataPeer(state tls.ConnectionState) (Peer, error) {
	if len(state.PeerCertificates) == 0 {
		return Peer{}, newTransferError(ErrDeviceNotTrusted, false, fmt.Errorf("客户端未提供证书"))
	}
	e.mu.RLock()
	candidates := make([]Peer, 0, len(e.peers))
	for _, peer := range e.peers {
		if peer.Relation == PeerRelation && peer.DeviceID != "" && peer.PublicKeyPEM != "" && peer.CertificateFingerprint != "" {
			candidates = append(candidates, peer)
		}
	}
	e.mu.RUnlock()
	var identityChange error
	for _, peer := range candidates {
		if err := verifyPeerCertificateState(state, peer); err == nil {
			return peer, nil
		} else if code, _ := transferErrorInfo(err); code == ErrCertificateChanged && peer.PublicKeyPEM != "" {
			// A matching public key with a changed certificate is attributable to
			// this friend even when Android's generated certificate CN is an alias.
			identityChange = err
		}
	}
	if identityChange != nil {
		return Peer{}, identityChange
	}
	return Peer{}, newTransferError(ErrDeviceNotTrusted, false, fmt.Errorf("客户端设备未受信任"))
}

func (e *Engine) receiveV3DataConnection(conn net.Conn, authenticatedPeerIDs ...string) error {
	reader := newV3FrameReader(conn, maxBinaryFileFramePayload)
	defer reader.Close()
	for {
		if err := e.receiveV3TransferWithReader(conn, reader, authenticatedPeerIDs...); err != nil {
			return err
		}
	}
}

// receiveV3Transfer is kept as a small test/helper entry point. Production
// connections use receiveV3DataConnection so one reader lives for the whole
// TLS connection and can continue draining frames during a checkpoint.
func (e *Engine) receiveV3Transfer(conn net.Conn, authenticatedPeerIDs ...string) error {
	reader := newV3FrameReader(conn, maxBinaryFileFramePayload)
	defer reader.Close()
	return e.receiveV3TransferWithReader(conn, reader, authenticatedPeerIDs...)
}

// v3FrameReader owns the only read side of a v3 data connection. In
// particular, no transfer handler can leave unread bytes in a TLS record while
// it is syncing the file or persisting a resume checkpoint.
type v3FrameReader struct {
	conn     net.Conn
	frames   chan BinaryFrameV3
	done     chan struct{}
	closeOne sync.Once
	doneOne  sync.Once
	writeMu  sync.Mutex
	errMu    sync.Mutex
	err      error
}

func newV3FrameReader(conn net.Conn, maxPayload uint32) *v3FrameReader {
	// Clear any handshake/accept deadline before the long-lived reader starts.
	// Frame boundaries are enforced by ReadBinaryFrameV3, never by a short
	// socket read deadline.
	_ = conn.SetReadDeadline(time.Time{})
	r := &v3FrameReader{
		conn:   conn,
		frames: make(chan BinaryFrameV3, 16), // four 1 MiB chunks plus control frames
		done:   make(chan struct{}),
	}
	go r.run(maxPayload)
	return r
}

func (r *v3FrameReader) run(maxPayload uint32) {
	for {
		frame, err := ReadBinaryFrameV3(r.conn, maxPayload)
		if err != nil {
			r.errMu.Lock()
			r.err = err
			r.errMu.Unlock()
			r.doneOne.Do(func() { close(r.done) })
			return
		}
		if frame.Type == FramePoolPing {
			// Reply from the reader goroutine, so a slow Sync/SQLite checkpoint
			// cannot make the sender's liveness probe wait behind the data loop.
			pong := BinaryFrameV3{Type: FramePoolPong, TransferID: frame.TransferID, StreamID: frame.StreamID, Sequence: frame.Sequence, Offset: frame.Offset, SessionID: frame.SessionID, Generation: frame.Generation}
			if err := r.WriteFrame(pong); err != nil {
				r.errMu.Lock()
				r.err = err
				r.errMu.Unlock()
				r.doneOne.Do(func() { close(r.done) })
				return
			}
		}
		select {
		case r.frames <- frame:
		case <-r.done:
			return
		}
	}
}

func (r *v3FrameReader) WriteFrame(frame BinaryFrameV3) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return writeV3Frame(r.conn, frame)
}

func (r *v3FrameReader) Next() (BinaryFrameV3, error) {
	frame, ready, err := r.Wait(0)
	if !ready && err == nil {
		err = net.ErrClosed
	}
	return frame, err
}

// Wait returns ready=false,nil when the bounded low-speed batch timer fires.
// It is deliberately separate from a socket deadline: the reader continues
// consuming complete frames, so a timer can never split a frame in flight.
func (r *v3FrameReader) Wait(timeout time.Duration) (BinaryFrameV3, bool, error) {
	var timer *time.Timer
	var timerC <-chan time.Time
	if timeout > 0 {
		timer = time.NewTimer(timeout)
		timerC = timer.C
		defer timer.Stop()
	}
	select {
	case frame := <-r.frames:
		return frame, true, nil
	case <-r.done:
		r.errMu.Lock()
		err := r.err
		r.errMu.Unlock()
		if err == nil {
			err = net.ErrClosed
		}
		return BinaryFrameV3{}, false, err
	case <-timerC:
		return BinaryFrameV3{}, false, nil
	}
}

func (r *v3FrameReader) Close() {
	r.closeOne.Do(func() {
		r.doneOne.Do(func() { close(r.done) })
		_ = r.conn.Close()
	})
}

// receiveV3Transfer consumes exactly one BeginFile ... EndFile sequence. The
// underlying TLS connection remains open so the sender can reuse the slot for
// the next file without another handshake.
func (e *Engine) receiveV3TransferWithReader(conn net.Conn, reader *v3FrameReader, authenticatedPeerIDs ...string) (result error) {
	first, err := reader.Next()
	if err != nil {
		return err
	}
	if first.Type != FrameBeginFile {
		return fmt.Errorf("v3 stream must start with BeginFile")
	}
	attachmentID := ""
	var transfer *incomingFile
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.mu.RLock()
		for id, candidate := range e.incoming {
			if binaryTransferID(id) == first.TransferID {
				attachmentID, transfer = id, candidate
				break
			}
		}
		e.mu.RUnlock()
		if transfer != nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("unknown v3 transfer")
		}
		time.Sleep(10 * time.Millisecond) // control response may arrive after the data connection
	}
	if len(authenticatedPeerIDs) > 0 && authenticatedPeerIDs[0] != "" && transfer.senderID != authenticatedPeerIDs[0] {
		return newTransferError(ErrDeviceNotTrusted, false, fmt.Errorf("数据连接设备与传输发送方不一致"))
	}
	if tc, ok := conn.(*tls.Conn); ok {
		e.mu.RLock()
		peer, found := e.peers[transfer.senderID]
		e.mu.RUnlock()
		if !found {
			return newTransferError(ErrDeviceNotTrusted, false, fmt.Errorf("v3 peer not found"))
		}
		if peer.PublicKeyPEM == "" && peer.CertificateFingerprint == "" {
			return fmt.Errorf("v3 peer identity missing")
		}
		if err := verifyPeerCertificate(tc, peer); err != nil {
			return err
		}
	}
	if transfer.v3Sink != nil {
		defer func() {
			if result != nil {
				select {
				case transfer.v3Done <- "failed":
				default:
				}
			}
		}()
	}
	if transfer.file == nil && transfer.v3Sink == nil {
		return fmt.Errorf("v3 transfer has no writable file")
	}
	transfer.v3Mu.Lock()
	newSessionIdentity := false
	if transfer.v3SessionID == ([16]byte{}) {
		transfer.v3SessionID = first.SessionID
		transfer.v3Generation = first.Generation
		newSessionIdentity = true
	} else if transfer.v3SessionID != first.SessionID || transfer.v3Generation != first.Generation {
		transfer.v3Mu.Unlock()
		return fmt.Errorf("v3 session or generation mismatch")
	}
	transfer.v3Mu.Unlock()
	if newSessionIdentity {
		transfer.resumeMu.Lock()
		state := transfer.resumeState
		state.SessionID = hex.EncodeToString(first.SessionID[:])
		state.Generation = first.Generation
		state.TransferMode = v3TransferMode
		state.CheckpointSeq++
		if state.AttachmentID != "" {
			if err := saveTransferResumeState(state); err != nil {
				transfer.resumeMu.Unlock()
				return err
			}
		}
		transfer.resumeState = state
		transfer.resumeMu.Unlock()
	}
	transfer.v3Mu.Lock()
	if transfer.v3Streams == nil {
		transfer.v3Streams = make(map[uint16]*v3StreamState)
	}
	stream := transfer.v3Streams[first.StreamID]
	if stream == nil {
		stream = &v3StreamState{}
		transfer.v3Streams[first.StreamID] = stream
	}
	transfer.v3Mu.Unlock()
	progressStarted := time.Now()
	lastProgressAt := progressStarted
	transfer.v3Mu.Lock()
	lastProgressBytes := transfer.received
	transfer.v3Mu.Unlock()
	var lastCheckpointDuration time.Duration
	deferCheckpoint := false
	metricsEnabled := false
	e.mu.RLock()
	if peer, ok := e.peers[transfer.senderID]; ok {
		metricsEnabled = hasCapability(peer.Capabilities, ackBatchCapability) && hasCapability(peer.Capabilities, transferMetricsCapability)
	}
	e.mu.RUnlock()
	var metricSeq uint64

	ack := func(frame BinaryFrameV3, nack bool, snapshot *TransferMetricsSnapshotV1) error {
		typ := FrameChunkAck
		if frame.Type == FrameEndFile {
			typ = FrameEndFile
		}
		if nack {
			typ = FrameChunkNack
		}
		ackFrame := BinaryFrameV3{Type: typ, TransferID: frame.TransferID, StreamID: frame.StreamID, Sequence: frame.Sequence, Offset: frame.Offset, ChunkHash: frame.ChunkHash, SessionID: frame.SessionID, Generation: frame.Generation}
		if !nack && snapshot != nil && metricsEnabled {
			payload, err := encodeTransferMetricsSnapshot(*snapshot)
			if err != nil {
				return err
			}
			ackFrame.Payload, ackFrame.Length = payload, uint32(len(payload))
		}
		return reader.WriteFrame(ackFrame)
	}
	consume := func(frame BinaryFrameV3) error {
		transfer.v3IOMu.RLock()
		defer transfer.v3IOMu.RUnlock()
		if frame.SessionID != first.SessionID || frame.Generation != first.Generation || frame.StreamID != first.StreamID {
			return fmt.Errorf("v3 session or generation mismatch")
		}
		if frame.TransferID != binaryTransferID(attachmentID) {
			return fmt.Errorf("v3 transfer id mismatch")
		}
		if frame.Type != FrameChunkData {
			return nil
		}
		if frame.Offset > uint64(transfer.expected) || uint64(len(frame.Payload)) > uint64(transfer.expected)-frame.Offset {
			return fmt.Errorf("v3 chunk outside file")
		}
		expectedHash := sha256.Sum256(frame.Payload)
		if expectedHash != frame.ChunkHash {
			return fmt.Errorf("v3 chunk hash mismatch")
		}
		transfer.v3Mu.Lock()
		for _, r := range transfer.v3Ranges {
			if int64(frame.Offset) >= r.Start && int64(frame.Offset)+int64(len(frame.Payload)) <= r.End {
				version := transfer.v3WrittenVersion
				transfer.v3Mu.Unlock()
				if transfer.v3Sink != nil {
					return nil
				}
				if deferCheckpoint {
					return nil
				}
				checkpointStarted := time.Now()
				err := commitV3Checkpoint(transfer, version) // idempotent, durable duplicate
				lastCheckpointDuration = time.Since(checkpointStarted)
				return err
			}
			if int64(frame.Offset) < r.End && r.Start < int64(frame.Offset)+int64(len(frame.Payload)) {
				transfer.v3Mu.Unlock()
				return fmt.Errorf("v3 chunk overlaps completed range")
			}
		}
		if transfer.v3Sink != nil {
			if int64(frame.Offset) != transfer.received {
				transfer.v3Mu.Unlock()
				return fmt.Errorf("unordered preview range")
			}
			if err := transfer.v3Sink(frame.Payload); err != nil {
				transfer.v3Mu.Unlock()
				return err
			}
			_, _ = transfer.digest.Write(frame.Payload)
			transfer.received += int64(len(frame.Payload))
			transfer.v3Ranges = []ByteRange{{Start: 0, End: transfer.received}}
			transfer.v3Mu.Unlock()
			return nil
		}
		if _, err := transfer.file.WriteAt(frame.Payload, int64(frame.Offset)); err != nil {
			transfer.v3Mu.Unlock()
			return err
		}
		transfer.v3Ranges = MergeRanges(append(transfer.v3Ranges, ByteRange{Start: int64(frame.Offset), End: int64(frame.Offset) + int64(len(frame.Payload))}))
		var covered int64
		for _, r := range transfer.v3Ranges {
			covered += r.End - r.Start
		}
		transfer.received = covered
		transfer.v3WrittenVersion++
		version := transfer.v3WrittenVersion
		transfer.v3Mu.Unlock()
		if deferCheckpoint {
			return nil
		}
		checkpointStarted := time.Now()
		err := commitV3Checkpoint(transfer, version)
		lastCheckpointDuration = time.Since(checkpointStarted)
		return err
	}
	if err := consume(first); err != nil {
		_ = ack(first, true, nil)
		return err
	}
	if first.Type == FrameChunkData {
		if err := ack(first, false, nil); err != nil {
			return err
		}
	}
	const batchLimit = int64(4 * 1024 * 1024)
	pending := make([]BinaryFrameV3, 0, 8)
	var pendingBytes int64
	versionBefore := func() uint64 { transfer.v3Mu.Lock(); defer transfer.v3Mu.Unlock(); return transfer.v3WrittenVersion }()
	for {
		deferCheckpoint = true
		flush := func() error {
			deferCheckpoint = false
			versionAfter := func() uint64 { transfer.v3Mu.Lock(); defer transfer.v3Mu.Unlock(); return transfer.v3WrittenVersion }()
			if versionAfter > versionBefore {
				transfer.v3Mu.Lock()
				received, durable := transfer.received, transfer.durableBytes
				transfer.v3Mu.Unlock()
				phaseOptions := transferProgressOptions{
					sessionID: fmt.Sprintf("%x", first.SessionID), generation: first.Generation,
					transferMode: v3TransferMode, transport: "TLS13/TCP-v3",
					metricSource: "receiver-durable", durableBytes: durable,
				}
				e.emitTransferProgress(transfer.messageID, attachmentID, transfer.senderID, received, transfer.expected, "receive", "durability_sync", phaseOptions)
				started := time.Now()
				if err := commitV3Checkpoint(transfer, versionAfter); err != nil {
					return err
				}
				lastCheckpointDuration = time.Since(started)
				transfer.v3Mu.Lock()
				received, durable = transfer.received, transfer.durableBytes
				transfer.v3Mu.Unlock()
				phaseOptions.durableBytes = durable
				phaseOptions.diskWriteMs = lastCheckpointDuration.Milliseconds()
				phaseOptions.checkpointSeq = versionAfter
				e.emitTransferProgress(transfer.messageID, attachmentID, transfer.senderID, received, transfer.expected, "receive", "checkpoint_persist", phaseOptions)
			}
			transfer.v3Mu.Lock()
			received := transfer.received
			checkpointSeq := transfer.v3WrittenVersion
			transfer.v3Mu.Unlock()
			metricSeq++
			now := time.Now()
			elapsed := now.Sub(lastProgressAt)
			if elapsed <= 0 {
				elapsed = time.Millisecond
			}
			rate := float64(received-lastProgressBytes) / elapsed.Seconds()
			if rate < 0 {
				rate = 0
			}
			lastProgressAt, lastProgressBytes = now, received
			acceptedChunkSize := defaultTransferChunkSize
			for _, frame := range pending {
				if len(frame.Payload) > acceptedChunkSize {
					acceptedChunkSize = len(frame.Payload)
				}
			}
			snapshot := TransferMetricsSnapshotV1{MetricSeq: metricSeq, CheckpointSeq: checkpointSeq, DurableBytes: received, Speed: rate, AverageSpeed: rate, PeakSpeed: rate, DiskWriteMs: lastCheckpointDuration.Milliseconds(), AckLatencyMs: elapsed.Milliseconds(), ChunkSize: acceptedChunkSize, WindowSize: len(pending), WindowBytes: pendingBytes, AckTargetBytes: batchLimit, StreamCount: 1, ActiveStreams: 1}
			e.emitTransferProgress(transfer.messageID, attachmentID, transfer.senderID, received, transfer.expected, "receive", "transferring", transferProgressOptions{
				chunkSize: snapshot.ChunkSize, windowSize: snapshot.WindowSize, windowBytes: snapshot.WindowBytes, activeStreams: 1, streamCount: 1,
				transferMode: v3TransferMode, transport: "TLS13/TCP-v3", protocol: fmt.Sprintf("%s/%d", ProtocolName, ProtocolMajor),
				confirmedThroughput: snapshot.Speed, windowThroughput: snapshot.Speed, diskWriteMs: snapshot.DiskWriteMs, durableBytes: snapshot.DurableBytes,
				averageSpeed: snapshot.AverageSpeed, peakSpeed: snapshot.PeakSpeed,
				displayLocalMetrics: true, tuningState: "stable", metricSource: "receiver-durable", metricSeq: snapshot.MetricSeq, checkpointSeq: snapshot.CheckpointSeq,
			})
			for i, frame := range pending {
				if err := ack(frame, false, func() *TransferMetricsSnapshotV1 {
					if i == len(pending)-1 {
						return &snapshot
					}
					return nil
				}()); err != nil {
					return err
				}
			}
			pending = pending[:0]
			pendingBytes = 0
			versionBefore = versionAfter
			return nil
		}
		// ReadBinaryFrameV3 uses io.ReadFull for both header and payload. A
		// short read deadline can expire halfway through a payload and make the
		// next read start at the wrong byte. 4MiB windows and PoolPing provide
		// explicit batch boundaries, so complete frame reads need no deadline.
		frame, ready, err := reader.Wait(150 * time.Millisecond)
		if err != nil {
			return err
		}
		if !ready {
			if pendingBytes > 0 {
				if err := flush(); err != nil {
					return err
				}
			}
			continue
		}
		if frame.TransferID != first.TransferID || frame.StreamID != first.StreamID || frame.SessionID != first.SessionID || frame.Generation != first.Generation {
			return fmt.Errorf("v3 frame identity mismatch")
		}
		if frame.Type != FrameChunkData && pendingBytes > 0 {
			if err := flush(); err != nil {
				return err
			}
		}
		if frame.Type == FramePoolPing {
			// v3FrameReader already sent the Pong without waiting for this
			// transfer's checkpoint. The control frame remains here solely to
			// delimit and flush a short batch.
			continue
		}
		if frame.Type == FrameCancel {
			transfer.v3Mu.Lock()
			transfer.v3Finalizing = true
			transfer.v3Mu.Unlock()
			return errAttachmentCanceled
		}
		if frame.Type == FrameEndFile {
			if err := flush(); err != nil {
				_ = ack(frame, true, nil)
				return err
			}
			transfer.v3IOMu.Lock()
			defer transfer.v3IOMu.Unlock()
			if !strings.EqualFold(hex.EncodeToString(frame.ChunkHash[:]), transfer.sha256) {
				_ = ack(frame, true, nil)
				return fmt.Errorf("v3 final SHA-256 mismatch")
			}
			transfer.v3Mu.Lock()
			stream.ended = true
			complete := transfer.expected == 0 || (len(transfer.v3Ranges) == 1 && transfer.v3Ranges[0].Start == 0 && transfer.v3Ranges[0].End == transfer.expected)
			complete = complete && !transfer.v3Finalizing
			if complete {
				transfer.v3Finalizing = true
			}
			transfer.v3Mu.Unlock()
			if complete && transfer.v3Sink != nil {
				// Range previews validate every binary chunk and the authenticated
				// file hash; a whole-file preview additionally verifies SHA-256.
				if transfer.v3StartOffset == 0 && !strings.EqualFold(hex.EncodeToString(transfer.digest.Sum(nil)), transfer.sha256) {
					return fmt.Errorf("preview checksum mismatch")
				}
				transfer.v3Done <- "completed"
			} else if complete {
				if status := e.finishIncomingFile(attachmentID); status != "completed" {
					_ = ack(frame, true, nil)
					return fmt.Errorf("v3 transfer failed: %s", status)
				}
			}
			return ack(frame, false, nil)
		}
		if err := consume(frame); err != nil {
			_ = ack(frame, true, nil)
			return err
		}
		if frame.Type == FrameChunkData {
			pending = append(pending, frame)
			pendingBytes += int64(len(frame.Payload))
		}
		if !metricsEnabled || pendingBytes >= batchLimit {
			if err := flush(); err != nil {
				_ = ack(frame, true, nil)
				return err
			}
		}
	}
}
