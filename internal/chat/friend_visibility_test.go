package chat

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"flyqpro/internal/service/db"
)

func TestPeerVisibilityAndConversationActionsPersist(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-visible", Nickname: "好友", Relation: DiscoveredState, LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerRelation(ctx, "peer-visible", PeerRelation); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerVisibleInFriends(ctx, "peer-visible", false); err != nil {
		t.Fatal(err)
	}
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-visible", Nickname: "更新后的好友", Relation: DiscoveredState, LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, PeerRelation)
	if err != nil || len(peers) != 1 || peers[0].VisibleInFriends {
		t.Fatalf("发现更新不应重置好友隐藏状态: %v, %+v", err, peers)
	}

	conversationID, err := EnsureConversation(ctx, "peer-visible")
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkConversationUnread(ctx, conversationID); err != nil {
		t.Fatal(err)
	}
	if err := MarkConversationUnread(ctx, conversationID); err != nil {
		t.Fatal(err)
	}
	if err := SetConversationPinned(ctx, conversationID, true); err != nil {
		t.Fatal(err)
	}
	conversations, err := ListConversations(ctx)
	if err != nil || len(conversations) != 1 || conversations[0].UnreadCount != 1 || !conversations[0].Pinned {
		t.Fatalf("好友会话操作未持久化: %v, %+v", err, conversations)
	}
	if err := SetPeerVisibleInFriends(ctx, "peer-visible", true); err != nil {
		t.Fatal(err)
	}
	peers, err = ListPeers(ctx, PeerRelation)
	if err != nil || len(peers) != 1 || !peers[0].VisibleInFriends {
		t.Fatalf("好友可见状态恢复失败: %v, %+v", err, peers)
	}
}

func TestRemovedFriendRejectsLegacyRestoreFrame(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := MarkFriendRemoved(ctx, "peer-removed"); err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		NewEngine().handleWire(server, wireMessage{DeviceID: "peer-removed"}, wireMessage{Type: "friend_restore"}, nil)
		close(done)
	}()
	var response wireMessage
	if err := readWire(client, &response); err != nil {
		t.Fatal(err)
	}
	if response.Type != "friend_restore_ack" || response.Status != "rejected" || response.Reason != "FRIENDSHIP_REMOVED" {
		t.Fatalf("删除后的恢复帧未被拒绝: %+v", response)
	}
	<-done
}
