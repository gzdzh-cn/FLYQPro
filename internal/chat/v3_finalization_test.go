package chat

import (
	"net"
	"testing"
	"time"
)

func TestV3FinalizationErrorRoundTrip(t *testing.T) {
	want := v3FinalizationError{
		Status:        "failed",
		ErrorCode:     ErrDestinationCommit,
		Retryable:     true,
		Verified:      true,
		DurableBytes:  1234,
		CommittedPath: `C:\Users\test\payload.bin`,
	}
	payload, err := encodeV3FinalizationError(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeV3FinalizationError(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != want.Status || got.ErrorCode != want.ErrorCode || !got.Retryable || !got.Verified || got.DurableBytes != want.DurableBytes || got.CommittedPath != want.CommittedPath {
		t.Fatalf("decoded finalization result = %+v, want %+v", got, want)
	}
}

func TestV3FrameReaderCloseUnblocksRead(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	reader := newV3FrameReader(left, 1024)
	defer reader.Close()
	result := make(chan error, 1)
	go func() {
		_, err := reader.Next()
		result <- err
	}()
	reader.Close()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("reader returned success after close")
		}
	case <-time.After(time.Second):
		t.Fatal("reader remained blocked after close")
	}
}
