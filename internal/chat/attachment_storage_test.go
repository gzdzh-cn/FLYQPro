package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"helpfly/internal/service/db"
)

func TestAttachmentTargetPathUsesStablePeerDirectoryAndSuffix(t *testing.T) {
	root := t.TempDir()
	first, err := AttachmentTargetPath(root, "peer/with spaces", "report?.txt")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(first)) != "peer_with_spaces" {
		t.Fatalf("unexpected peer directory: %s", first)
	}
	if err := os.WriteFile(first, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := AttachmentTargetPath(root, "peer/with spaces", "report?.txt")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(second) != "report_ (1).txt" {
		t.Fatalf("unexpected collision suffix: %s", second)
	}
}

func TestMigrateAttachmentsClassifiesAndVerifiesFiles(t *testing.T) {
	root := t.TempDir()
	oldRoot := filepath.Join(root, "old")
	targetRoot := filepath.Join(root, "new")
	if err := os.MkdirAll(oldRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(oldRoot, "legacy.bin")
	data := []byte("legacy attachment")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	dbPath := filepath.Join(root, "chat.db")
	if err := os.Setenv("GOFLY_DB_PATH", dbPath); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("GOFLY_DB_PATH")
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := EnsureDefaults(ctx, oldRoot); err != nil {
		t.Fatal(err)
	}
	conversation, err := EnsureConversation(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveMessage(ctx, Message{MessageID: "migration-message", ConversationID: conversation, SenderDeviceID: "sender", Kind: "file", Content: "legacy.bin", Status: "sent", CreatedAt: nowString()}); err != nil {
		t.Fatal(err)
	}
	if err := SaveAttachment(ctx, Attachment{AttachmentID: "migration-attachment", MessageID: "migration-message", FileName: "legacy.bin", FileSize: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), LocalPath: path, Status: "saved"}); err != nil {
		t.Fatal(err)
	}
	result, err := MigrateAttachments(ctx, oldRoot, targetRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed || result.Migrated != 1 || result.Unclassified != 1 {
		t.Fatalf("unexpected migration result: %+v", result)
	}
	newPath := filepath.Join(targetRoot, "_unclassified", "legacy.bin")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("migrated file missing: %v", err)
	}
	attachment, err := GetAttachment(ctx, "migration-attachment")
	if err != nil || attachment.LocalPath != newPath {
		t.Fatalf("attachment path not persisted: %+v, %v", attachment, err)
	}
}
