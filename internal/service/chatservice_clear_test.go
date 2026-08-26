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

func TestHideFriendAndClearLocalDataKeepsFriendshipAndConversation(t *testing.T) {
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
	if err := chat.UpsertPeer(ctx, chat.Peer{DeviceID: "peer-hide", Nickname: "隐藏好友", Relation: chat.PeerRelation, LastSeen: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	conversationID, err := chat.EnsureConversation(ctx, "peer-hide")
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SetConversationPinned(ctx, conversationID, true); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveMessage(ctx, chat.Message{MessageID: "hide-message", ConversationID: conversationID, SenderDeviceID: "peer-hide", Kind: "text", Content: "保留关系", Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}

	if err := (&ChatService{engine: chat.NewEngine()}).HideFriendAndClearLocalData("peer-hide"); err != nil {
		t.Fatal(err)
	}
	peers, err := chat.ListPeers(ctx, chat.PeerRelation)
	if err != nil || len(peers) != 1 || peers[0].VisibleInFriends {
		t.Fatalf("好友关系或隐藏状态错误: %v, %+v", err, peers)
	}
	conversations, err := chat.ListConversations(ctx)
	if err != nil || len(conversations) != 1 || !conversations[0].Pinned {
		t.Fatalf("删除后会话身份或置顶状态未保留: %v, %+v", err, conversations)
	}
	if messages, err := chat.ListMessages(ctx, conversationID); err != nil || len(messages) != 0 {
		t.Fatalf("本地聊天记录未清理: %v, %d", err, len(messages))
	}
}

func TestRemoveFriendAndClearLocalDataRemovesFriendshipAndLocalRecords(t *testing.T) {
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
	if err := chat.UpsertPeer(ctx, chat.Peer{DeviceID: "peer-remove", Nickname: "彻底删除", Relation: chat.PeerRelation}); err != nil {
		t.Fatal(err)
	}
	conversationID, err := chat.EnsureConversation(ctx, "peer-remove")
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveFriendRequest(ctx, chat.FriendRequest{RequestID: "request-remove", DeviceID: "peer-remove", Nickname: "彻底删除", Status: "accepted", Direction: "received", CreatedAt: "2026-01-01T00:00:00Z", AcceptedAt: "2026-01-01T00:00:01Z"}); err != nil {
		t.Fatal(err)
	}
	if err := chat.SaveMessage(ctx, chat.Message{MessageID: "remove-message", ConversationID: conversationID, SenderDeviceID: "peer-remove", Kind: "text", Content: "待删除", Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	engine := chat.NewEngine()
	if err := (&ChatService{engine: engine}).RemoveFriendAndClearLocalData("peer-remove"); err != nil {
		t.Fatal(err)
	}
	if peers, err := chat.ListPeers(ctx, ""); err != nil || len(peers) != 0 {
		t.Fatalf("好友关系或 Peer 未清理: %v, %+v", err, peers)
	}
	if conversations, err := chat.ListConversations(ctx); err != nil || len(conversations) != 0 {
		t.Fatalf("会话未清理: %v, %+v", err, conversations)
	}
	if messages, err := chat.ListMessages(ctx, conversationID); err != nil || len(messages) != 0 {
		t.Fatalf("聊天记录未清理: %v, %d", err, len(messages))
	}
	if requests, err := chat.ListFriendRequests(ctx, "peer-remove"); err != nil || len(requests) != 0 {
		t.Fatalf("好友申请记录未清理: %v, %+v", err, requests)
	}
	if removed, err := chat.IsFriendRemoved(ctx, "peer-remove"); err != nil || !removed {
		t.Fatalf("好友删除标记未保留，旧端可能通过 friend_restore 恢复关系: %v, %v", err, removed)
	}
	if err := chat.ClearFriendRemoval(ctx, "peer-remove"); err != nil {
		t.Fatal(err)
	}
	if removed, err := chat.IsFriendRemoved(ctx, "peer-remove"); err != nil || removed {
		t.Fatalf("好友删除标记未清除: %v, %v", err, removed)
	}
}
