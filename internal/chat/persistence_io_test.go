package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type faultingTransferPersistenceIO struct {
	transferPersistenceIO
	databaseError  error
	syncError      func(string) error
	renameError    func(string, string) error
	directoryError func(string) error
}

func (f faultingTransferPersistenceIO) SaveResumeRecord(ctx context.Context, state transferResumeState) error {
	if f.databaseError != nil {
		return f.databaseError
	}
	return f.transferPersistenceIO.SaveResumeRecord(ctx, state)
}

func (f faultingTransferPersistenceIO) SyncFile(file *os.File) error {
	if f.syncError != nil {
		if err := f.syncError(file.Name()); err != nil {
			return err
		}
	}
	return f.transferPersistenceIO.SyncFile(file)
}

func (f faultingTransferPersistenceIO) Rename(source, target string) error {
	if f.renameError != nil {
		if err := f.renameError(source, target); err != nil {
			return err
		}
	}
	return f.transferPersistenceIO.Rename(source, target)
}

func (f faultingTransferPersistenceIO) SyncDirectory(path string) error {
	if f.directoryError != nil {
		if err := f.directoryError(path); err != nil {
			return err
		}
	}
	return f.transferPersistenceIO.SyncDirectory(path)
}

func installTransferPersistenceFault(t *testing.T, fault faultingTransferPersistenceIO) {
	t.Helper()
	if fault.transferPersistenceIO == nil {
		fault.transferPersistenceIO = osTransferPersistenceIO{}
	}
	restore := replaceTransferPersistenceIO(fault)
	t.Cleanup(restore)
}

func TestResumeCheckpointUsesSidecarWhenSQLiteFails(t *testing.T) {
	openSharedFolderTestDatabase(t)
	installTransferPersistenceFault(t, faultingTransferPersistenceIO{databaseError: errors.New("sqlite unavailable")})
	state := transferResumeState{AttachmentID: "sidecar-fallback", TransferID: "sidecar-fallback", Direction: "send", FileSize: 1, State: TransferPausedNetwork}
	if err := saveTransferResumeState(state); err != nil {
		t.Fatalf("sidecar fallback rejected: %v", err)
	}
	_, sidecar, err := transferResumePaths(state.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("durable sidecar missing: %v", err)
	}
}

func TestResumeCheckpointFailsWhenSQLiteAndSidecarFail(t *testing.T) {
	openSharedFolderTestDatabase(t)
	installTransferPersistenceFault(t, faultingTransferPersistenceIO{
		databaseError: errors.New("sqlite unavailable"),
		syncError: func(path string) error {
			if strings.HasSuffix(path, ".resume.json.tmp") {
				return syscall.ENOSPC
			}
			return nil
		},
	})
	state := transferResumeState{AttachmentID: "all-metadata-failed", TransferID: "all-metadata-failed", Direction: "receive", FileSize: 1, State: TransferActive}
	if err := saveTransferResumeState(state); err == nil {
		t.Fatal("checkpoint succeeded after both metadata stores failed")
	}
	_, sidecar, _ := transferResumePaths(state.AttachmentID)
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("failed temporary sidecar became visible: %v", err)
	}
}

func TestBinaryReceiverReportsCheckpointFailureBeforeAck(t *testing.T) {
	root := t.TempDir()
	w, err := NewRangeWriterV3(filepath.Join(root, "payload.bin"), 4)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	installTransferPersistenceFault(t, faultingTransferPersistenceIO{
		syncError: func(path string) error {
			if strings.HasSuffix(path, ".part") {
				return syscall.ENOSPC
			}
			return nil
		},
	})
	id := [16]byte{7}
	chunk := NewChunkFrame(id, 0, 1, 0, []byte("data"))
	whole := sha256.Sum256([]byte("data"))
	end := BinaryFrameV3{Type: FrameEndFile, TransferID: id, Sequence: 2, ChunkHash: whole}
	var wire bytes.Buffer
	for _, frame := range []BinaryFrameV3{chunk, end} {
		encoded, marshalErr := frame.MarshalBinary()
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		wire.Write(encoded)
	}
	var acknowledgements []error
	result := ReceiveBinaryFileV3(context.Background(), &wire, w, 1024, func(_ BinaryFrameV3, checkpointErr error) error {
		acknowledgements = append(acknowledgements, checkpointErr)
		return nil
	})
	if len(acknowledgements) != 1 || acknowledgements[0] == nil {
		t.Fatalf("checkpoint failure was acknowledged as success: %+v", acknowledgements)
	}
	if result.Status != "failed" || result.Err == nil {
		t.Fatalf("receiver did not fail after non-durable chunk: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "payload.bin")); !os.IsNotExist(err) {
		t.Fatalf("non-durable transfer was committed: %v", err)
	}
}

func TestFinalCommitReportsDirectorySyncFailure(t *testing.T) {
	root := t.TempDir()
	finalPath := filepath.Join(root, "payload.bin")
	w, err := NewRangeWriterV3(finalPath, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	payload := []byte("data")
	hash := sha256.Sum256(payload)
	if _, err := w.WriteChunk(0, payload, hash[:]); err != nil {
		t.Fatal(err)
	}
	installTransferPersistenceFault(t, faultingTransferPersistenceIO{
		directoryError: func(path string) error {
			if path == root {
				return errors.New("directory sync failed")
			}
			return nil
		},
	})
	if err := w.Complete(hash[:]); err == nil {
		t.Fatal("directory sync failure was ignored")
	}
}

func TestFinalCommitRenameFailurePreservesPart(t *testing.T) {
	root := t.TempDir()
	finalPath := filepath.Join(root, "rename.bin")
	w, err := NewRangeWriterV3(finalPath, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	payload := []byte("data")
	hash := sha256.Sum256(payload)
	if _, err := w.WriteChunk(0, payload, hash[:]); err != nil {
		t.Fatal(err)
	}
	installTransferPersistenceFault(t, faultingTransferPersistenceIO{
		renameError: func(_, target string) error {
			if target == finalPath {
				return errors.New("rename failed")
			}
			return nil
		},
	})
	if err := w.Complete(hash[:]); err == nil {
		t.Fatal("rename failure was ignored")
	}
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("target unexpectedly committed after rename failure: %v", err)
	}
	if _, err := os.Stat(finalPath + ".part"); err != nil {
		t.Fatalf("part file was not preserved: %v", err)
	}
}

func TestFinalCommitTargetSyncFailureReportsFailureAfterAtomicRename(t *testing.T) {
	root := t.TempDir()
	finalPath := filepath.Join(root, "target-sync.bin")
	w, err := NewRangeWriterV3(finalPath, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	payload := []byte("data")
	hash := sha256.Sum256(payload)
	if _, err := w.WriteChunk(0, payload, hash[:]); err != nil {
		t.Fatal(err)
	}
	installTransferPersistenceFault(t, faultingTransferPersistenceIO{
		syncError: func(path string) error {
			if path == finalPath {
				return errors.New("target sync failed")
			}
			return nil
		},
	})
	if err := w.Complete(hash[:]); err == nil {
		t.Fatal("target sync failure was ignored")
	}
	if got, err := os.ReadFile(finalPath); err != nil || string(got) != "data" {
		t.Fatalf("atomic destination was not retained after sync failure: %v %q", err, got)
	}
	if _, err := os.Stat(finalPath + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part file survived an already committed rename: %v", err)
	}
}

func TestDuplicateEndFileAfterCommitIsIdempotent(t *testing.T) {
	ctx := openSharedFolderTestDatabase(t)
	root := t.TempDir()
	finalPath := filepath.Join(root, "already-saved.bin")
	if err := os.WriteFile(finalPath, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	conversationID, err := EnsureConversation(ctx, "duplicate-peer")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveMessage(ctx, Message{
		MessageID:        "duplicate-message",
		ConversationID:   conversationID,
		SenderDeviceID:   "duplicate-peer",
		Kind:             "file",
		Content:          "already-saved.bin",
		Status:           "sent",
		CreatedAt:        nowString(),
		AttachmentID:     "duplicate-end-file",
		AttachmentName:   "already-saved.bin",
		AttachmentSize:   4,
		AttachmentStatus: "saved",
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveAttachment(ctx, Attachment{
		AttachmentID: "duplicate-end-file",
		MessageID:    "duplicate-message",
		FileName:     "already-saved.bin",
		FileSize:     4,
		LocalPath:    finalPath,
		Status:       "saved",
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	if got := engine.finishIncomingFile("duplicate-end-file"); got != "completed" {
		t.Fatalf("duplicate EndFile was not acknowledged idempotently: %s", got)
	}
	if got, err := os.ReadFile(finalPath); err != nil || string(got) != "data" {
		t.Fatalf("committed destination was modified: %v %q", err, got)
	}
}
