package chat

import (
	"context"
	"fmt"
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

func TestAcceptedFriendRequestMakesRequesterVisible(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-accepted", Nickname: "新好友", Relation: DiscoveredState, LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	// A newly discovered peer starts hidden from the friends list. The
	// acceptance response must promote both the relation and visibility.
	engine := NewEngine()
	engine.handleWire(nil, wireMessage{DeviceID: "peer-accepted"}, wireMessage{
		Type:      "friend_request_response",
		RequestID: "request-accepted",
		Status:    "accepted",
	}, nil)
	peers, err := ListPeers(ctx, PeerRelation)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || !peers[0].VisibleInFriends {
		t.Fatalf("接受好友申请后未进入好友列表: %+v", peers)
	}
}

func TestOpeningHiddenFriendRestoresFriendsListVisibility(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-hidden", Nickname: "隐藏好友", Relation: PeerRelation, VisibleInFriends: false}); err != nil {
		t.Fatal(err)
	}

	if err := NewEngine().MarkConversationRead(ctx, "peer-hidden"); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, PeerRelation)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || !peers[0].VisibleInFriends {
		t.Fatalf("从通讯录打开隐藏好友后未恢复好友列表显示: %+v", peers)
	}
}

func TestRemoteFriendshipRejectionDowngradesLocalPeer(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-remote-removed", Nickname: "远端已删除", Relation: PeerRelation, VisibleInFriends: true, DiscoveryVisible: true}); err != nil {
		t.Fatal(err)
	}

	NewEngine().handleRemoteFriendshipRequired("peer-remote-removed")
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != DiscoveredState || peers[0].VisibleInFriends || peers[0].DiscoveryVisible {
		t.Fatalf("收到远端好友关系拒绝后未降级为陌生设备: %+v", peers)
	}
	removed, err := IsFriendRemoved(ctx, "peer-remote-removed")
	if err != nil || !removed {
		t.Fatalf("远端关系删除标记未保存: %v, %v", err, removed)
	}
}

func TestRemovedFriendCanBeRediscoveredAsStranger(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := MarkFriendRemoved(ctx, "peer-rediscover"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	if err := engine.handleAnnounce(wireMessage{
		Type:           "announce",
		DeviceID:       "peer-rediscover",
		Nickname:       "可重新发现",
		Protocol:       ProtocolName,
		Major:          ProtocolMajor,
		Magic:          DiscoveryMagic,
		DiscoveryScope: DiscoveryScopePublic,
	}); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != DiscoveredState || !peers[0].DiscoveryVisible {
		t.Fatalf("删除好友后未按陌生设备重新发现: %+v", peers)
	}
}

func TestRemovedFriendAcceptsLegacyScopeLessAnnounce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := MarkFriendRemoved(ctx, "peer-legacy-announce"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	message := wireMessage{
		Type:     "announce",
		DeviceID: "peer-legacy-announce",
		Nickname: "旧版设备",
		Protocol: ProtocolName,
		Major:    ProtocolMajor,
		Magic:    DiscoveryMagic,
	}
	if got := engine.compatibilityDiscoveryScope(message); got != DiscoveryScopePublic {
		t.Fatalf("删除好友的旧版 announce 未兼容为 public: %q", got)
	}
	message.DiscoveryScope = engine.compatibilityDiscoveryScope(message)
	if err := engine.handleAnnounce(message); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != DiscoveredState || !peers[0].DiscoveryVisible {
		t.Fatalf("旧版设备删除后未重新显示在发现列表: %+v", peers)
	}
}

func TestSendFailureStatusDistinguishesRemovedFriend(t *testing.T) {
	if got := sendFailureStatus(fmt.Errorf("对方握手失败: FRIENDSHIP_REQUIRED")); got != "not_friend" {
		t.Fatalf("好友关系拒绝未转换为 not_friend: %s", got)
	}
	if got := sendFailureStatus(fmt.Errorf("连接超时")); got != "failed" {
		t.Fatalf("普通传输错误被错误转换: %s", got)
	}
}

func TestFriendRemovedFrameDowngradesRemotePeer(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-removed-notify", Nickname: "远端好友", Relation: PeerRelation, VisibleInFriends: true, DiscoveryVisible: true, LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	engine.handleWire(nil, wireMessage{DeviceID: "peer-removed-notify"}, wireMessage{Type: "friend_removed"}, nil)
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != DiscoveredState || peers[0].VisibleInFriends || peers[0].DiscoveryVisible {
		t.Fatalf("解除好友帧未实时降级远端关系: %+v", peers)
	}
	removed, err := IsFriendRemoved(ctx, "peer-removed-notify")
	if err != nil || !removed {
		t.Fatalf("解除好友帧未留下关系删除标记: %v, %v", err, removed)
	}
}
