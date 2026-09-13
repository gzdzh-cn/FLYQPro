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

func TestResumeIncomingAttachmentKeepsAcceptedResumingState(t *testing.T) {
	ctx := openSharedFolderTestDatabase(t)
	conversationID, err := EnsureConversation(ctx, "resume-peer")
	if err != nil {
		t.Fatal(err)
	}
	message := Message{
		MessageID:        "resume-incoming-message",
		ConversationID:   conversationID,
		SenderDeviceID:   "resume-peer",
		Kind:             "file",
		Content:          "resume.bin",
		Status:           "paused",
		CreatedAt:        nowString(),
		AttachmentID:     "resume-incoming-attachment",
		AttachmentName:   "resume.bin",
		AttachmentSize:   128,
		AttachmentStatus: "paused",
	}
	if err := SaveMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	if err := SaveAttachment(ctx, Attachment{AttachmentID: message.AttachmentID, MessageID: message.MessageID, FileName: message.AttachmentName, FileSize: message.AttachmentSize, Status: "paused"}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	engine.identity.DeviceID = "local-device"
	resumed, err := engine.ResumeAttachment(ctx, message.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != "resuming" || resumed.AttachmentStatus != "resuming" {
		t.Fatalf("resume returned initial-offer state: %+v", resumed)
	}
	stored, err := GetMessage(ctx, message.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "resuming" || stored.AttachmentStatus != "resuming" {
		t.Fatalf("resume persisted initial-offer state: %+v", stored)
	}
}

func TestTransferResumeStateRejectsExpiredAndMismatchedIdentity(t *testing.T) {
	openSharedFolderTestDatabase(t)
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
	if err := exec(context.Background(), `UPDATE transfer_resumes SET updated_at=? WHERE attachment_id=?`, loaded.UpdatedAt.UTC().Format(time.RFC3339Nano), loaded.AttachmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTransferResumeState(state.AttachmentID); err == nil {
		t.Fatal("expired resume state was accepted")
	}
}

func TestTransferResumeChoosesNewestCheckpointWithoutMerging(t *testing.T) {
	openSharedFolderTestDatabase(t)
	attachmentID := "checkpoint-choice"
	part, sidecar, err := transferResumePaths(attachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(part), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	databaseState := transferResumeState{Version: transferResumeVersion, AttachmentID: attachmentID, TransferID: attachmentID, Direction: "receive", FileSize: 100, CheckpointSeq: 2, State: TransferPausedNetwork, CompletedRanges: []TransferRange{{Offset: 0, Length: 40}}, UpdatedAt: time.Now().UTC()}
	if err := saveTransferResumeRecord(context.Background(), databaseState); err != nil {
		t.Fatal(err)
	}
	sidecarState := databaseState
	sidecarState.CheckpointSeq = 1
	sidecarState.CompletedRanges = []TransferRange{{Offset: 60, Length: 20}}
	encoded, err := json.Marshal(sidecarState)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadTransferResumeState(attachmentID)
	if err != nil {
		t.Fatal(err)
	}
	want := []TransferRange{{Offset: 0, Length: 40}}
	if loaded.CheckpointSeq != 2 || !reflect.DeepEqual(loaded.CompletedRanges, want) {
		t.Fatalf("newest checkpoint not selected: %+v", loaded)
	}
	if err := os.Remove(sidecar); err != nil {
		t.Fatal(err)
	}
	if loaded, err = loadTransferResumeState(attachmentID); err != nil || loaded.CheckpointSeq != 2 {
		t.Fatalf("SQLite fallback failed: state=%+v err=%v", loaded, err)
	}
	// A newer record that claims bytes beyond the durable .part length must
	// never override an older, valid checkpoint.
	if err := os.Truncate(part, 80); err != nil {
		t.Fatal(err)
	}
	databaseState.CheckpointSeq = 3
	databaseState.CompletedRanges = []TransferRange{{Offset: 90, Length: 10}}
	databaseState.UpdatedAt = time.Now().UTC()
	if err := saveTransferResumeRecord(context.Background(), databaseState); err != nil {
		t.Fatal(err)
	}
	sidecarState.CompletedRanges = []TransferRange{{Offset: 0, Length: 60}}
	encoded, err = json.Marshal(sidecarState)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadTransferResumeState(attachmentID)
	if err != nil || loaded.CheckpointSeq != 1 || !reflect.DeepEqual(loaded.CompletedRanges, sidecarState.CompletedRanges) {
		t.Fatalf("out-of-bounds checkpoint was selected: state=%+v err=%v", loaded, err)
	}
}

func TestOutgoingResumePersistsConfirmedRangesInBoundedCheckpoints(t *testing.T) {
	openSharedFolderTestDatabase(t)
	const size = int64(8 * 1024 * 1024)
	state := transferResumeState{AttachmentID: "outgoing-checkpoint", TransferID: "outgoing-checkpoint", Direction: "send", FileSize: size, SourceMTimeNS: 10, State: TransferQueued}
	if err := persistOutgoingResumeState(state); err != nil {
		t.Fatal(err)
	}
	persisted, err := loadTransferResumeRecord(context.Background(), state.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	engine.outgoing[state.AttachmentID] = &outgoingTransfer{resumeState: persisted}
	if err := engine.persistOutgoingConfirmedRange(state.AttachmentID, 0, 1024*1024, "session-1", 7); err != nil {
		t.Fatal(err)
	}
	row, err := loadTransferResumeRecord(context.Background(), state.AttachmentID)
	if err != nil || len(row.CompletedRanges) != 0 {
		t.Fatalf("sub-threshold checkpoint was persisted: state=%+v err=%v", row, err)
	}
	if err := engine.persistOutgoingConfirmedRange(state.AttachmentID, 1024*1024, 4*1024*1024, "session-1", 7); err != nil {
		t.Fatal(err)
	}
	row, err = loadTransferResumeRecord(context.Background(), state.AttachmentID)
	if err != nil || contiguousTransferOffset(row.CompletedRanges, row.FileSize) != 4*1024*1024 || row.Generation != 7 || row.SessionID != "session-1" {
		t.Fatalf("bounded checkpoint missing: state=%+v err=%v", row, err)
	}
	if err := engine.persistOutgoingConfirmedRange(state.AttachmentID, 4*1024*1024, size, "session-1", 7); err != nil {
		t.Fatal(err)
	}
	row, err = loadTransferResumeRecord(context.Background(), state.AttachmentID)
	if err != nil || contiguousTransferOffset(row.CompletedRanges, row.FileSize) != size {
		t.Fatalf("final checkpoint missing: state=%+v err=%v", row, err)
	}
	_, sidecar, err := transferResumePaths(state.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("outgoing checkpoint sidecar missing: %v", err)
	}
}

func TestOutgoingResumeLoadsSidecarWithoutPartFile(t *testing.T) {
	openSharedFolderTestDatabase(t)
	source := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(source, bytes.Repeat([]byte("x"), 128), 0o600); err != nil {
		t.Fatal(err)
	}
	attachmentID := "outgoing-sidecar"
	_, sidecar, err := transferResumePaths(attachmentID)
	if err != nil {
		t.Fatal(err)
	}
	state := transferResumeState{Version: transferResumeVersion, AttachmentID: attachmentID, TransferID: attachmentID, Direction: "send", FileSize: 128, TargetPath: source, CheckpointSeq: 4, State: TransferPausedNetwork, CompletedRanges: []TransferRange{{Offset: 0, Length: 64}}, UpdatedAt: time.Now().UTC()}
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o700); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := exec(context.Background(), `DELETE FROM transfer_resumes WHERE attachment_id=?`, attachmentID); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadTransferResumeState(attachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CheckpointSeq != 4 || contiguousTransferOffset(loaded.CompletedRanges, loaded.FileSize) != 64 {
		t.Fatalf("sidecar-only outgoing recovery failed: %+v", loaded)
	}
}

func TestOutgoingResumeDetectsSourceMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	state := transferResumeState{Direction: "send", FileSize: info.Size(), SourceMTimeNS: info.ModTime().UnixNano()}
	if outgoingResumeSourceChanged(state, info) {
		t.Fatal("unchanged source was rejected")
	}
	state.SourceMTimeNS--
	if !outgoingResumeSourceChanged(state, info) {
		t.Fatal("source mtime change was not detected")
	}
}

func TestOutgoingResumeValidationRejectsChangedSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	state := transferResumeState{
		Version:       transferResumeVersion,
		AttachmentID:  "source-validation",
		Direction:     "send",
		FileSize:      info.Size(),
		SourceMTimeNS: info.ModTime().UnixNano(),
		TargetPath:    source,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := validateTransferResumeCandidateWithSource(state, state.AttachmentID, -1); err != nil {
		t.Fatalf("unchanged source rejected: %v", err)
	}
	if err := os.WriteFile(source, []byte("changed-size"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateTransferResumeCandidateWithSource(state, state.AttachmentID, -1); err == nil {
		t.Fatal("changed source was accepted for recovery")
	}
}

func TestRecoveryTasksIncludeOnlyRecoverableTerminalFailures(t *testing.T) {
	openSharedFolderTestDatabase(t)
	for _, state := range []transferResumeState{
		{AttachmentID: "retryable-failure", TransferID: "retryable-failure", Direction: "send", FileSize: 1, State: TransferFailed, ErrorCode: ErrSessionNotReady, Retryable: true, UpdatedAt: time.Now().UTC()},
		{AttachmentID: "permanent-failure", TransferID: "permanent-failure", Direction: "send", FileSize: 1, State: TransferFailed, ErrorCode: ErrDeviceKeyChanged, Retryable: false, UpdatedAt: time.Now().UTC()},
	} {
		if err := saveTransferResumeRecord(context.Background(), state); err != nil {
			t.Fatal(err)
		}
	}
	tasks := NewEngine().ListRecoveryTasks()
	if len(tasks) != 1 || tasks[0].TransferID != "retryable-failure" || !tasks[0].Retryable {
		t.Fatalf("unexpected recovery task list: %+v", tasks)
	}
}

func TestTerminalFailureKeepsSidecarAndClearsRecoverableRanges(t *testing.T) {
	openSharedFolderTestDatabase(t)
	state := transferResumeState{AttachmentID: "terminal-evidence", TransferID: "terminal-evidence", Direction: "receive", FileSize: 16, State: TransferActive, CompletedRanges: []TransferRange{{Offset: 0, Length: 8}}}
	part, sidecar, err := transferResumePaths(state.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(part), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, make([]byte, state.FileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveTransferResumeState(state); err != nil {
		t.Fatal(err)
	}
	if err := markTransferResumeTerminal(context.Background(), state.AttachmentID, TransferFailed, ErrChunkVerifyFailed, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("terminal sidecar missing: %v", err)
	}
	var terminal transferResumeState
	if err := json.Unmarshal(data, &terminal); err != nil {
		t.Fatal(err)
	}
	if terminal.State != TransferFailed || terminal.ErrorCode != ErrChunkVerifyFailed || len(terminal.CompletedRanges) != 0 {
		t.Fatalf("terminal evidence invalid: %+v", terminal)
	}
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("verification failure lost part file: %v", err)
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
