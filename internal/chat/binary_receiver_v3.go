package chat

import (
	"context"
	"errors"
	"io"
)

type V3ReceiveResult struct {
	Completed int64
	Status    string
	Err       error
}

// ReceiveBinaryFileV3 consumes a dedicated binary data stream. Control frames
// stay on the JSON session; this function is deliberately transport agnostic.
func ReceiveBinaryFileV3(ctx context.Context, r io.Reader, w *RangeWriterV3, maxPayload uint32, ack func(BinaryFrameV3, error) error) V3ReceiveResult {
	if w == nil {
		return V3ReceiveResult{Status: "failed", Err: errors.New("nil range writer")}
	}
	for {
		select {
		case <-ctx.Done():
			return V3ReceiveResult{Status: "cancelled", Err: ctx.Err()}
		default:
		}
		frame, err := ReadBinaryFrameV3(r, maxPayload)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return V3ReceiveResult{Status: "failed", Err: err}
			}
			return V3ReceiveResult{Status: "failed", Err: err}
		}
		switch frame.Type {
		case FrameChunkData:
			ok, writeErr := w.WriteChunk(int64(frame.Offset), frame.Payload, frame.ChunkHash[:])
			if ack != nil {
				if e := ack(frame, writeErr); e != nil {
					return V3ReceiveResult{Status: "failed", Err: e}
				}
			}
			if writeErr != nil {
				continue
			}
			_ = ok
		case FrameEndFile:
			if err := w.Complete(frame.ChunkHash[:]); err != nil {
				return V3ReceiveResult{Status: "failed", Err: err}
			}
			return V3ReceiveResult{Status: "completed", Completed: w.size}
		case FrameCancel:
			return V3ReceiveResult{Status: "cancelled", Err: errors.New("peer cancelled")}
		}
	}
}
