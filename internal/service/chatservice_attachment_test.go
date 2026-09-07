package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"flyqpro/internal/chat"
	"flyqpro/internal/service/db"
)

func TestGetAttachmentDetailsAllowsInProgressAttachments(t *testing.T) {
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
	if err := chat.EnsureDefaults(ctx, filepath.Join(root, "attachments")); err != nil {
		t.Fatal(err)
	}

	conversationID, err := chat.EnsureConversation(ctx, "peer-details")
	if err != nil {
		t.Fatal(err)
	}
	statuses := []string{"pending", "sending", "receiving", "saved"}
	service := &ChatService{engine: chat.NewEngine()}
	for _, status := range statuses {
		messageID := "details-message-" + status
		attachmentID := "details-attachment-" + status
		if err := chat.SaveMessage(ctx, chat.Message{MessageID: messageID, ConversationID: conversationID, SenderDeviceID: "peer-details", Kind: "file", Content: "report.bin", Status: status, CreatedAt: "2026-01-01T00:00:00Z", AttachmentID: attachmentID, AttachmentName: "report.bin", AttachmentSize: 42, AttachmentMime: "application/octet-stream", AttachmentStatus: status}); err != nil {
			t.Fatal(err)
		}
		if err := chat.SaveAttachment(ctx, chat.Attachment{AttachmentID: attachmentID, MessageID: messageID, FileName: "report.bin", MimeType: "application/octet-stream", FileSize: 42, Status: status}); err != nil {
			t.Fatal(err)
		}
		details, err := service.GetAttachmentDetails(attachmentID)
		if err != nil {
			t.Fatalf("status %s: %v", status, err)
		}
		if details.FileName != "report.bin" || details.FileSize != 42 || details.Status != status {
			t.Fatalf("status %s returned incomplete details: %+v", status, details)
		}
	}

	senderPath := filepath.Join(root, "sender.bin")
	senderData := []byte("sender attachment")
	if err := os.WriteFile(senderPath, senderData, 0o600); err != nil {
		t.Fatal(err)
	}
	senderMessageID := "details-message-sender"
	senderAttachmentID := "details-attachment-sender"
	if err := chat.SaveMessage(ctx, chat.Message{MessageID: senderMessageID, ConversationID: conversationID, SenderDeviceID: "local-device", Kind: "file", Content: "sender.bin", Status: "sent", CreatedAt: "2026-01-01T00:00:00Z", AttachmentID: senderAttachmentID, AttachmentName: "sender.bin", AttachmentSize: int64(len(senderData)), AttachmentMime: "application/octet-stream", AttachmentStatus: "sent", AttachmentPath: senderPath}); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveAttachment(ctx, chat.Attachment{AttachmentID: senderAttachmentID, MessageID: senderMessageID, FileName: "sender.bin", MimeType: "application/octet-stream", FileSize: int64(len(senderData)), LocalPath: senderPath, Status: "sent"}); err != nil {
		t.Fatal(err)
	}
	wantSum := sha256.Sum256(senderData)
	details, err := service.GetAttachmentDetails(senderAttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if details.SHA256 != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("sender SHA-256 = %q, want %q", details.SHA256, hex.EncodeToString(wantSum[:]))
	}
	stored, err := chat.GetAttachment(ctx, senderAttachmentID)
	if err != nil || stored.SHA256 != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("sender SHA-256 was not persisted: %q, want %q (err: %v)", stored.SHA256, hex.EncodeToString(wantSum[:]), err)
	}
}
