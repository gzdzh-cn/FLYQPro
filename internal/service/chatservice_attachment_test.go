package service

import (
	"context"
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
}
