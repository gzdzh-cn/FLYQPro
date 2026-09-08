package chat

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

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
	config, err := e.clientTLSConfig()
	if err != nil {
		return nil, err
	}
	conn, err := DialV3DataCandidates(ctx, append([]string{peer.IP}, peer.LocalAddresses...), peer.DataPort, config)
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
	if startOffset > 0 {
		completed = append(completed, ByteRange{Start: 0, End: startOffset})
	}
	assignments := partitionV3Ranges(message.AttachmentSize, completed, streams)
	if len(assignments) == 0 {
		return nil
	}
	streams = len(assignments)
	if streams < 2 {
		first := assignments[0]
		return e.sendV3FileDataRanges(ctx, peer, message, file, fullSHA, first)
	}
	if streams > 8 {
		streams = 8
	}
	pool := e.v3Pool(peer.DeviceID, streams)
	var wg sync.WaitGroup
	errs := make(chan error, streams)
	for i := 0; i < streams; i++ {
		ranges := assignments[i]
		wg.Add(1)
		go func(worker int, ranges []ByteRange) {
			defer wg.Done()
			slot, err := pool.Acquire(ctx, message.AttachmentID)
			if err != nil {
				errs <- err
				return
			}
			defer pool.Release(slot)
			conn, err := e.pooledV3DataConn(ctx, peer, slot)
			if err != nil {
				errs <- err
				return
			}
			id := binaryTransferID(message.AttachmentID)
			sessionID := pool.SessionID()
			generation := slot.Generation
			begin := BinaryFrameV3{Type: FrameBeginFile, TransferID: id, StreamID: uint16(worker), SessionID: sessionID, Generation: generation}
			raw, _ := begin.MarshalBinary()
			if _, err = conn.Write(raw); err != nil {
				slot.Kill()
				errs <- err
				return
			}
			buf := make([]byte, 1<<20)
			var seq uint64
			for _, r := range ranges {
				for off := r.Start; off < r.End; {
					n := int64(len(buf))
					if n > r.End-off {
						n = r.End - off
					}
					if _, err = file.ReadAt(buf[:n], off); err != nil && err != io.EOF {
						errs <- err
						return
					}
					frame := NewChunkFrame(id, uint16(worker), seq, uint64(off), buf[:n])
					frame.SessionID, frame.Generation = sessionID, generation
					raw, _ = frame.MarshalBinary()
					acked := false
					for attempt := 0; attempt < 3 && !acked; attempt++ {
						_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
						if _, err = conn.Write(raw); err != nil {
							slot.Kill()
							break
						}
						ack, readErr := ReadBinaryFrameV3(conn, 1024)
						acked = readErr == nil && ack.Type == FrameChunkAck && ack.Sequence == seq
					}
					if !acked {
						slot.Kill()
						errs <- os.ErrInvalid
						return
					}
					slot.Progress(n, n)
					off += n
					seq++
				}
			}
			endOffset := uint64(ranges[len(ranges)-1].End)
			endFrame := BinaryFrameV3{Type: FrameEndFile, TransferID: id, StreamID: uint16(worker), Offset: endOffset, SessionID: sessionID, Generation: generation}
			raw, _ = endFrame.MarshalBinary()
			_, err = conn.Write(raw)
			if err != nil {
				slot.Kill()
				errs <- err
			}
		}(i, ranges)
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
	if peer.DataPort <= 0 || peer.IP == "" {
		return os.ErrInvalid
	}
	if len(ranges) == 0 {
		return nil
	}
	startOffset := ranges[0].Start
	if startOffset < 0 || startOffset > message.AttachmentSize {
		return os.ErrInvalid
	}
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return err
	}
	pool := e.v3Pool(peer.DeviceID, 1)
	slot, err := pool.Acquire(ctx, message.AttachmentID)
	if err != nil {
		return err
	}
	defer pool.Release(slot)
	conn, err := e.pooledV3DataConn(ctx, peer, slot)
	if err != nil {
		return err
	}
	id := binaryTransferID(message.AttachmentID)
	sessionID := pool.SessionID()
	generation := slot.Generation
	begin := BinaryFrameV3{Type: FrameBeginFile, TransferID: id, SessionID: sessionID, Generation: generation}
	raw, _ := begin.MarshalBinary()
	if _, err = conn.Write(raw); err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	var offset, seq uint64 = uint64(startOffset), 0
	for _, current := range ranges {
		if _, err := file.Seek(current.Start, io.SeekStart); err != nil {
			return err
		}
		offset = uint64(current.Start)
		for offset < uint64(current.End) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			readSize := int64(len(buf))
			if remaining := int64(current.End) - int64(offset); readSize > remaining {
				readSize = remaining
			}
			n, readErr := file.Read(buf[:readSize])
			if n > 0 {
				frame := NewChunkFrame(id, 0, seq, offset, buf[:n])
				frame.SessionID, frame.Generation = sessionID, generation
				raw, err = frame.MarshalBinary()
				if err != nil {
					return err
				}
				acked := false
				for attempt := 0; attempt < 3; attempt++ {
					_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					if _, err = conn.Write(raw); err != nil {
						slot.Kill()
						return err
					}
					ack, readErr := ReadBinaryFrameV3(conn, 1024)
					if readErr == nil && ack.Type == FrameChunkAck && ack.Sequence == seq {
						acked = true
						break
					}
					if readErr != nil && !isTemporaryNetError(readErr) && attempt == 2 {
						slot.Kill()
						return readErr
					}
					if readErr == nil && ack.Type == FrameChunkNack {
						continue
					}
				}
				_ = conn.SetReadDeadline(time.Time{})
				if !acked {
					slot.Kill()
					return os.ErrInvalid
				}
				offset += uint64(n)
				seq++
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
	}
	var digest []byte
	if fullSHA != "" {
		digest, err = hex.DecodeString(fullSHA)
		if err != nil {
			return err
		}
	} else {
		_, _ = file.Seek(0, io.SeekStart)
		h := sha256.New()
		if _, err = io.Copy(h, file); err != nil {
			return err
		}
		digest = h.Sum(nil)
	}
	var finalHash [32]byte
	copy(finalHash[:], digest)
	end := BinaryFrameV3{Type: FrameEndFile, TransferID: id, Offset: offset, ChunkHash: finalHash, SessionID: sessionID, Generation: generation}
	raw, err = end.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = conn.Write(raw)
	if err != nil {
		slot.Kill()
	}
	return err
}
