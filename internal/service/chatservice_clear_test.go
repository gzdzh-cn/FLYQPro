package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"flyqpro/internal/chat"
	"flyqpro/internal/service/db"
)

func TestClearConversationRemovesManagedFilesAndKeepsOtherPeers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	t.Setenv("FLYQPRO_DATA_DIR", filepath.Join(root, "data"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := chat.EnsureDataDirs(); err != nil {
		t.Fatal(err)
	}
	attachmentRoot := filepath.Join(root, "attachments")
	if err := chat.EnsureDefaults(ctx, attachmentRoot); err != nil {
		t.Fatal(err)
	}

	conversationA, err := chat.EnsureConversation(ctx, "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	conversationB, err := chat.EnsureConversation(ctx, "peer-b")
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveMessage(ctx, chat.Message{MessageID: "clear-a-text", ConversationID: conversationA, SenderDeviceID: "peer-a", Kind: "text", Content: "hello", Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveMessage(ctx, chat.Message{MessageID: "clear-a-file", ConversationID: conversationA, SenderDeviceID: "peer-a", Kind: "file", Content: "photo.png", Status: "sent", CreatedAt: "2026-01-01T00:00:01Z"}); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveMessage(ctx, chat.Message{MessageID: "keep-b-text", ConversationID: conversationB, SenderDeviceID: "peer-b", Kind: "text", Content: "keep", Status: "sent", CreatedAt: "2026-01-01T00:00:02Z"}); err != nil {
		t.Fatal(err)
	}

	managedPath := filepath.Join(attachmentRoot, "peer-a", "photo.png")
	if err := os.MkdirAll(filepath.Dir(managedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedPath, []byte("received"), 0o600); err != nil {
		t.Fatal(err)
	}
	externalPath := filepath.Join(root, "original.png")
	if err := os.WriteFile(externalPath, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveAttachment(ctx, chat.Attachment{AttachmentID: "clear-managed", MessageID: "clear-a-file", FileName: "photo.png", FileSize: 8, LocalPath: managedPath, Status: "saved"}); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveAttachment(ctx, chat.Attachment{AttachmentID: "clear-external", MessageID: "clear-a-file", FileName: "original.png", FileSize: 8, LocalPath: externalPath, Status: "sent"}); err != nil {
		t.Fatal(err)
	}
	result, err := (&ChatService{engine: chat.NewEngine()}).ClearConversation("peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedMessages != 2 || result.DeletedAttachments != 2 || result.DeletedFiles != 1 || result.SkippedExternalFiles != 1 {
		t.Fatalf("unexpected clear result: %+v", result)
	}
	if _, err := os.Stat(managedPath); !os.IsNotExist(err) {
		t.Fatalf("managed attachment still exists: %v", err)
	}
	if _, err := os.Stat(externalPath); err != nil {
		t.Fatalf("external original was removed: %v", err)
	}
	if messages, err := chat.ListMessages(ctx, conversationA); err != nil || len(messages) != 0 {
		t.Fatalf("cleared conversation still has messages: %d, %v", len(messages), err)
	}
	if messages, err := chat.ListMessages(ctx, conversationB); err != nil || len(messages) != 1 {
		t.Fatalf("other conversation changed: %d, %v", len(messages), err)
	}
}
