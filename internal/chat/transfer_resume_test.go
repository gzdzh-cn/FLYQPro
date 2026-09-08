package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTransferResumeStateRoundTripAndRangeNormalization(t *testing.T) {
	t.Setenv("FLYQPRO_DATA_DIR", t.TempDir())
	state := transferResumeState{
		AttachmentID:   "attachment-1",
		TransferID:     "transfer-1",
		MessageID:      "message-1",
		SenderDeviceID: "sender-1",
		FileSize:       100,
		SHA256:         "abc",
		CompletedRanges: []TransferRange{
			{Offset: 50, Length: 75},
			{Offset: 0, Length: 25},
			{Offset: 20, Length: 40},
			{Offset: -1, Length: 2},
		},
	}
	part, _, _ := transferResumePaths(state.AttachmentID)
	if err := os.MkdirAll(filepath.Dir(part), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, make([]byte, state.FileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveTransferResumeState(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadTransferResumeState(state.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	want := []TransferRange{{Offset: 0, Length: 100}}
	if !reflect.DeepEqual(loaded.CompletedRanges, want) || contiguousTransferOffset(loaded.CompletedRanges, loaded.FileSize) != 100 {
		t.Fatalf("normalized ranges = %+v, want %+v", loaded.CompletedRanges, want)
	}
}

func TestTransferResumeStateRejectsExpiredAndMismatchedIdentity(t *testing.T) {
	t.Setenv("FLYQPRO_DATA_DIR", t.TempDir())
	state := transferResumeState{AttachmentID: "attachment-2", MessageID: "message", SenderDeviceID: "sender", FileSize: 10, SHA256: "hash"}
	part, _, _ := transferResumePaths(state.AttachmentID)
	if err := os.MkdirAll(filepath.Dir(part), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, make([]byte, state.FileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveTransferResumeState(state); err != nil {
		t.Fatal(err)
	}
	_, path, _ := transferResumePaths(state.AttachmentID)
	loaded, err := loadTransferResumeState(state.AttachmentID)
	if err != nil || !transferResumeMatches(loaded, "message", "sender", 10, "HASH") {
		t.Fatalf("matching state rejected: state=%+v err=%v", loaded, err)
	}
	if transferResumeMatches(loaded, "message", "other", 10, "hash") || transferResumeMatches(loaded, "message", "sender", 11, "hash") {
		t.Fatal("mismatched resume identity was accepted")
	}
	loaded.UpdatedAt = time.Now().Add(-transferResumeTTL - time.Minute)
	// Save refreshes UpdatedAt, so write an explicitly expired sidecar.
	loaded.Version = transferResumeVersion
	encoded, marshalErr := json.Marshal(loaded)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTransferResumeState(state.AttachmentID); err == nil {
		t.Fatal("expired resume state was accepted")
	}
}

func TestBinaryFramePayloadCRC32C(t *testing.T) {
	payload := bytes.Repeat([]byte("crc32c"), 1024)
	var encoded bytes.Buffer
	if err := writeBinaryFramePayload(&encoded, payload); err != nil {
		t.Fatal(err)
	}
	decoded := make([]byte, len(payload))
	if err := readBinaryFramePayload(&encoded, decoded); err != nil || !bytes.Equal(decoded, payload) {
		t.Fatalf("valid CRC frame rejected: %v", err)
	}
}

func TestBinaryFramePayloadRejectsCorruptionAndTruncatedTrailer(t *testing.T) {
	payload := []byte("frame payload")
	var encoded bytes.Buffer
	if err := writeBinaryFramePayload(&encoded, payload); err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), encoded.Bytes()...)
	corrupt[0] ^= 0xff
	if err := readBinaryFramePayload(bytes.NewReader(corrupt), make([]byte, len(payload))); err == nil {
		t.Fatal("corrupt payload passed CRC32C")
	}
	truncated := encoded.Bytes()[:encoded.Len()-1]
	if err := readBinaryFramePayload(bytes.NewReader(truncated), make([]byte, len(payload))); err == nil {
		t.Fatal("truncated CRC32C trailer was accepted")
	}
}

func TestTransferSchedulerSerializesPeerAndLimitsGlobalConcurrency(t *testing.T) {
	scheduler := newTransferScheduler()
	cancel := make(chan struct{})
	releasePeer, err := scheduler.acquirePeer(context.Background(), "peer-a", cancel)
	if err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	go func() {
		release, acquireErr := scheduler.acquirePeer(context.Background(), "peer-a", cancel)
		if acquireErr == nil {
			release()
		}
		close(blocked)
	}()
	select {
	case <-blocked:
		t.Fatal("second file for one peer was not serialized")
	case <-time.After(20 * time.Millisecond):
	}
	releasePeer()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("queued peer transfer did not start")
	}

	release1, _ := scheduler.acquireGlobal(context.Background(), cancel)
	release2, _ := scheduler.acquireGlobal(context.Background(), cancel)
	ctx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if _, err := scheduler.acquireGlobal(ctx, cancel); err == nil {
		t.Fatal("global scheduler allowed more than two active peers")
	}
	release1()
	release2()
}

func TestTransferRetryBackoffAndCancellation(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for attempt, expected := range want {
		if got := retryDelay(attempt); got != expected {
			t.Fatalf("attempt %d delay = %s, want %s", attempt, got, expected)
		}
	}
	slot := make(chan struct{}, 1)
	slot <- struct{}{}
	canceled := make(chan struct{})
	close(canceled)
	if err := acquireTransferSlot(context.Background(), slot, canceled); err != errAttachmentCanceled {
		t.Fatalf("queued acquisition was not canceled: %v", err)
	}
}
