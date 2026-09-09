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
				log.Printf("v3 数据连接接受失败: %v", err)
				return
			}
		}
		go func(c net.Conn) {
			defer c.Close()
			if tc, ok := c.(*tls.Conn); ok {
				if err := tc.HandshakeContext(context.Background()); err != nil {
					return
				}
			}
			_ = e.receiveV3DataConnection(c)
		}(conn)
	}
}

func (e *Engine) receiveV3DataConnection(conn net.Conn) error {
	for {
		if err := e.receiveV3Transfer(conn); err != nil {
			return err
		}
	}
}

// receiveV3Transfer consumes exactly one BeginFile ... EndFile sequence. The
// underlying TLS connection remains open so the sender can reuse the slot for
// the next file without another handshake.
func (e *Engine) receiveV3Transfer(conn net.Conn) (result error) {
	first, err := ReadBinaryFrameV3(conn, maxBinaryFileFramePayload)
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
	if tc, ok := conn.(*tls.Conn); ok {
		peer, err := e.peer(transfer.senderID)
		if err != nil {
			return err
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
	if transfer.v3SessionID == ([16]byte{}) {
		transfer.v3SessionID = first.SessionID
		transfer.v3Generation = first.Generation
	} else if transfer.v3SessionID != first.SessionID || transfer.v3Generation != first.Generation {
		transfer.v3Mu.Unlock()
		return fmt.Errorf("v3 session or generation mismatch")
	}
	transfer.v3Mu.Unlock()
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
	lastProgressBytes := transfer.received

	ack := func(frame BinaryFrameV3, nack bool) error {
		typ := FrameChunkAck
		if frame.Type == FrameEndFile {
			typ = FrameEndFile
		}
		if nack {
			typ = FrameChunkNack
		}
		ackFrame := BinaryFrameV3{Type: typ, TransferID: frame.TransferID, StreamID: frame.StreamID, Sequence: frame.Sequence, Offset: frame.Offset, ChunkHash: frame.ChunkHash, SessionID: frame.SessionID, Generation: frame.Generation}
		return writeV3Frame(conn, ackFrame)
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
				return commitV3Checkpoint(transfer, version) // idempotent, durable duplicate
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
		return commitV3Checkpoint(transfer, version)
	}
	if err := consume(first); err != nil {
		_ = ack(first, true)
		return err
	}
	if first.Type == FrameChunkData {
		if err := ack(first, false); err != nil {
			return err
		}
	}
	for {
		frame, err := ReadBinaryFrameV3(conn, maxBinaryFileFramePayload)
		if err != nil {
			return err
		}
		if frame.TransferID != first.TransferID || frame.StreamID != first.StreamID || frame.SessionID != first.SessionID || frame.Generation != first.Generation {
			return fmt.Errorf("v3 frame identity mismatch")
		}
		if frame.Type == FrameEndFile {
			transfer.v3IOMu.Lock()
			defer transfer.v3IOMu.Unlock()
			if !strings.EqualFold(hex.EncodeToString(frame.ChunkHash[:]), transfer.sha256) {
				_ = ack(frame, true)
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
					return fmt.Errorf("v3 transfer failed: %s", status)
				}
			}
			return ack(frame, false)
		}
		if err := consume(frame); err != nil {
			_ = ack(frame, true)
			return err
		}
		if err := ack(frame, false); err != nil {
			return err
		}
		if frame.Type == FrameChunkData {
			now := time.Now()
			elapsed := now.Sub(lastProgressAt)
			if elapsed <= 0 {
				elapsed = time.Millisecond
			}
			transfer.v3Mu.Lock()
			received := transfer.received
			transfer.v3Mu.Unlock()
			rate := float64(received-lastProgressBytes) / elapsed.Seconds()
			if rate <= 0 {
				rate = float64(len(frame.Payload)) / elapsed.Seconds()
			}
			lastProgressAt, lastProgressBytes = now, received
			e.emitTransferProgress(transfer.messageID, attachmentID, transfer.senderID, received, transfer.expected, "receive", "transferring", transferProgressOptions{
				chunkSize: len(frame.Payload), windowBytes: int64(len(frame.Payload)), activeStreams: 1, streamCount: 1,
				transferMode: v3TransferMode, transport: "TLS13/TCP-v3", protocol: fmt.Sprintf("%s/%d", ProtocolName, ProtocolMajor),
				confirmedThroughput: rate, windowThroughput: rate, displayLocalMetrics: true, tuningState: "stable",
			})
		}
	}
}
