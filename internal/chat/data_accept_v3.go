package chat

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"log"
	"net"
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
func (e *Engine) receiveV3Transfer(conn net.Conn) error {
	first, err := ReadBinaryFrameV3(conn, maxBinaryFileFramePayload)
	if err != nil {
		return err
	}
	attachmentID := ""
	e.mu.RLock()
	for id := range e.incoming {
		if binaryTransferID(id) == first.TransferID {
			attachmentID = id
			break
		}
	}
	transfer := e.incoming[attachmentID]
	e.mu.RUnlock()
	if transfer == nil {
		return fmt.Errorf("unknown v3 transfer")
	}
	if transfer.file == nil {
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

	ack := func(frame BinaryFrameV3, nack bool) error {
		typ := FrameChunkAck
		if nack {
			typ = FrameChunkNack
		}
		ackFrame := BinaryFrameV3{Type: typ, TransferID: frame.TransferID, StreamID: frame.StreamID, Sequence: frame.Sequence, Offset: frame.Offset, Length: frame.Length, SessionID: frame.SessionID, Generation: frame.Generation}
		raw, err := ackFrame.MarshalBinary()
		if err != nil {
			return err
		}
		_, err = conn.Write(raw)
		return err
	}
	consume := func(frame BinaryFrameV3) error {
		if frame.SessionID != transfer.v3SessionID || frame.Generation != transfer.v3Generation {
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
				transfer.v3Mu.Unlock()
				return nil // idempotent duplicate
			}
			if int64(frame.Offset) < r.End && r.Start < int64(frame.Offset)+int64(len(frame.Payload)) {
				transfer.v3Mu.Unlock()
				return fmt.Errorf("v3 chunk overlaps completed range")
			}
		}
		transfer.v3Mu.Unlock()
		if _, err := transfer.file.WriteAt(frame.Payload, int64(frame.Offset)); err != nil {
			return err
		}
		transfer.v3Mu.Lock()
		transfer.v3Ranges = MergeRanges(append(transfer.v3Ranges, ByteRange{Start: int64(frame.Offset), End: int64(frame.Offset) + int64(len(frame.Payload))}))
		var covered int64
		for _, r := range transfer.v3Ranges {
			covered += r.End - r.Start
		}
		transfer.received = covered
		transfer.v3Mu.Unlock()
		return nil
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
		if frame.Type == FrameEndFile {
			transfer.v3Mu.Lock()
			stream.ended = true
			complete := transfer.expected == 0 || (len(transfer.v3Ranges) == 1 && transfer.v3Ranges[0].Start == 0 && transfer.v3Ranges[0].End == transfer.expected)
			complete = complete && !transfer.v3Finalizing
			if complete {
				transfer.v3Finalizing = true
			}
			transfer.v3Mu.Unlock()
			if complete {
				if status := e.finishIncomingFile(attachmentID); status != "completed" {
					return fmt.Errorf("v3 transfer failed: %s", status)
				}
			}
			return nil
		}
		if err := consume(frame); err != nil {
			_ = ack(frame, true)
			return err
		}
		if err := ack(frame, false); err != nil {
			return err
		}
	}
}
