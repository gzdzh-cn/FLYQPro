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

func TestStaleFriendRemovalMarkerDoesNotRestoreHiddenFriend(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-hidden-marker", Nickname: "隐藏好友", Relation: PeerRelation, VisibleInFriends: true}); err != nil {
		t.Fatal(err)
	}
	// Older relationship-sync data may leave a removal tombstone behind while
	// the peer row is still a friend.  Hiding the friend must remain effective;
	// ListPeers must not normalize that active row back to visible=true.
	if err := MarkFriendRemoved(ctx, "peer-hidden-marker"); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerVisibleInFriends(ctx, "peer-hidden-marker", false); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, PeerRelation)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != PeerRelation || peers[0].VisibleInFriends {
		t.Fatalf("旧解除标记不应覆盖好友隐藏状态: %+v", peers)
	}
}

func TestHiddenFriendPersistsAcrossRestartAndPeerRefresh(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "chat.db")
	t.Setenv("GOFLY_DB_PATH", dbPath)
	ctx := context.Background()
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-hidden-restart", Nickname: "隐藏好友", Relation: PeerRelation, VisibleInFriends: true}); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerVisibleInFriends(ctx, "peer-hidden-restart", false); err != nil {
		t.Fatal(err)
	}
	// Simulate the stale snapshot produced by discovery or a reconnect.
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-hidden-restart", Nickname: "重新发现", Relation: PeerRelation, VisibleInFriends: true, IP: "192.168.1.20", Port: 39190}); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, PeerRelation)
	if err != nil || len(peers) != 1 || peers[0].VisibleInFriends {
		t.Fatalf("刷新 peer 后隐藏状态丢失: %v, %+v", err, peers)
	}
	if err := db.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if err := MigrateHiddenFriendDevices(ctx); err != nil {
		t.Fatal(err)
	}
	peers, err = ListPeers(ctx, PeerRelation)
	if err != nil || len(peers) != 1 || peers[0].VisibleInFriends {
		t.Fatalf("重启后隐藏状态丢失: %v, %+v", err, peers)
	}
	if hidden, err := IsHiddenFriend(ctx, "peer-hidden-restart"); err != nil || !hidden {
		t.Fatalf("重启后隐藏标记丢失: %v, %v", err, hidden)
	}
	// Only the explicit restore path clears the durable marker.
	if err := SetPeerVisibleInFriends(ctx, "peer-hidden-restart", true); err != nil {
		t.Fatal(err)
	}
	if hidden, err := IsHiddenFriend(ctx, "peer-hidden-restart"); err != nil || hidden {
		t.Fatalf("显式恢复后隐藏标记仍存在: %v, %v", err, hidden)
	}
}

func TestFriendRequestDoesNotPrecedeWithRestore(t *testing.T) {
	if shouldSendFriendRestore("friend_request") {
		t.Fatal("显式好友申请前不应发送 friend_restore")
	}
	if shouldSendFriendRestore("friend_restore") {
		t.Fatal("friend_restore 不应再次发送 friend_restore")
	}
	if shouldSendFriendRestore("friend_removed") {
		t.Fatal("解除好友关系的控制帧前不应发送 friend_restore")
	}
	if shouldSendFriendRestore("friend_request_response") {
		t.Fatal("好友申请响应前不应发送旧的 friend_restore")
	}
	for _, messageType := range []string{"friend_request", "friend_request_response", "friend_removed"} {
		if !allowsRemovedFriendshipFrame(messageType) {
			t.Fatalf("关系控制帧应允许穿过旧删除状态: %s", messageType)
		}
	}
	for _, messageType := range []string{"message", "file_offer", "read_receipt"} {
		if allowsRemovedFriendshipFrame(messageType) {
			t.Fatalf("普通业务帧不应穿过旧删除状态: %s", messageType)
		}
	}
	for _, messageType := range []string{"message", "file_offer", "read_receipt"} {
		if !shouldSendFriendRestore(messageType) {
			t.Fatalf("正常好友消息应允许恢复关系: %s", messageType)
		}
	}
}

func TestFriendRequestCycleSupersedesBothDirectionsAndKeepsHistory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	requests := []FriendRequest{
		{RequestID: "old-accepted", DeviceID: "peer-cycle", Status: "accepted", Direction: "sent", CreatedAt: "2026-01-01T00:00:00Z", AcceptedAt: "2026-01-01T00:00:01Z"},
		{RequestID: "old-sent", DeviceID: "peer-cycle", Status: "sent", Direction: "sent", CreatedAt: "2026-01-02T00:00:00Z"},
		{RequestID: "old-pending", DeviceID: "peer-cycle", Status: "pending", Direction: "received", CreatedAt: "2026-01-03T00:00:00Z"},
	}
	for _, request := range requests {
		if err := SaveFriendRequest(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
	// Creating a new received request supersedes only the older received
	// request, while the latest outgoing request remains for mutual display.
	if err := SupersedeActiveFriendRequestsForNew(ctx, "peer-cycle", "received", "new-pending", "old-sent"); err != nil {
		t.Fatal(err)
	}
	if err := SaveFriendRequest(ctx, FriendRequest{RequestID: "new-pending", DeviceID: "peer-cycle", Status: "pending", Direction: "received", CreatedAt: "2026-01-04T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	all, err := ListFriendRequests(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("申请历史不应被覆盖: got %d rows", len(all))
	}
	byID := make(map[string]FriendRequest, len(all))
	for _, request := range all {
		byID[request.RequestID] = request
	}
	if byID["old-accepted"].Status != "accepted" || byID["old-sent"].Status != "sent" || byID["old-pending"].Status != "superseded" || byID["new-pending"].Status != "pending" {
		t.Fatalf("同设备申请未正确收敛: %+v", byID)
	}
	// Once one side resolves the cycle, the opposite active request is also
	// superseded and remains available only as historical data.
	if err := SupersedeActiveFriendRequestsExcept(ctx, "peer-cycle", "new-pending"); err != nil {
		t.Fatal(err)
	}
	all, err = ListFriendRequests(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	byID = make(map[string]FriendRequest, len(all))
	for _, request := range all {
		byID[request.RequestID] = request
	}
	if byID["old-sent"].Status != "superseded" || byID["new-pending"].Status != "pending" {
		t.Fatalf("申请解决后另一方向未收敛: %+v", byID)
	}
}

func TestIncomingRequestAfterDatabaseResetIsPending(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := SaveProfile(ctx, Profile{Discoverable: true, Nickname: "重置设备"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-reset", Nickname: "另一台设备", Relation: DiscoveredState}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	engine.mu.Lock()
	engine.profile.Discoverable = true
	engine.mu.Unlock()
	engine.handleWire(nil, wireMessage{DeviceID: "peer-reset", Nickname: "另一台设备"}, wireMessage{
		Type:      "friend_request",
		RequestID: "request-after-reset",
		Content:   "重新添加好友",
	}, nil)

	requests, err := listFriendRequestRows(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].Status != "pending" || requests[0].Direction != "received" {
		t.Fatalf("清库后的好友申请未保存为待处理申请: %+v", requests)
	}
}

func TestPendingFriendRequestSurvivesDatabaseReopen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	ctx := context.Background()
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	request := FriendRequest{
		RequestID: "request-persisted",
		DeviceID:  "peer-persisted",
		Nickname:  "持久化申请",
		Message:   "请通过好友申请",
		Status:    "pending",
		Direction: "received",
		CreatedAt: nowString(),
	}
	if err := SaveFriendRequest(ctx, request); err != nil {
		_ = db.Close(ctx)
		t.Fatal(err)
	}
	if err := db.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	requests, err := ListFriendRequests(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].RequestID != request.RequestID || requests[0].Status != "pending" || requests[0].Direction != "received" {
		t.Fatalf("待处理好友申请重启后未恢复: %+v", requests)
	}
}

func TestNewRequestIsNotHiddenByAcceptedHistory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := SaveProfile(ctx, Profile{Discoverable: true, Nickname: "接收端"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-readd", Nickname: "重新申请", Relation: PeerRelation, RelationshipVersion: "old-version"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveFriendRequest(ctx, FriendRequest{RequestID: "old-request", DeviceID: "peer-readd", Status: "accepted", Direction: "received", CreatedAt: "2026-08-26T10:00:00Z", AcceptedAt: "2026-08-26T10:01:00Z"}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	engine.mu.Lock()
	engine.profile.Discoverable = true
	engine.mu.Unlock()
	engine.handleWire(nil, wireMessage{DeviceID: "peer-readd", Nickname: "重新申请"}, wireMessage{Type: "friend_request", RequestID: "new-request", Content: "再次添加"}, nil)
	requests, err := listFriendRequestRows(ctx, "")
	if err != nil || len(requests) != 2 {
		t.Fatalf("新申请不应覆盖历史申请: %v, %+v", err, requests)
	}
	current, ok := friendRequestByID("new-request")
	if !ok || current.Status != "pending" {
		t.Fatalf("新申请没有进入待处理状态: %+v", current)
	}
	old, ok := friendRequestByID("old-request")
	if !ok || old.Status != "accepted" {
		t.Fatalf("旧已同意记录被错误修改: %+v", old)
	}
	if err := engine.AcceptFriendRequest(ctx, "new-request"); err != nil {
		t.Fatal(err)
	}
	old, _ = friendRequestByID("old-request")
	current, _ = friendRequestByID("new-request")
	if old.Status != "accepted" || current.Status != "accepted" || old.AcceptedAt == current.AcceptedAt {
		t.Fatalf("同意新申请错误影响历史记录: old=%+v new=%+v", old, current)
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
	if response.Type != "friend_restore_ack" || response.Status != "rejected" || response.Reason != "FRIENDSHIP_REQUIRED" {
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
	if err := SaveFriendRequest(ctx, FriendRequest{RequestID: "request-accepted", DeviceID: "peer-accepted", Nickname: "新好友", Status: "sent", Direction: "sent", CreatedAt: nowString()}); err != nil {
		t.Fatal(err)
	}
	// A newly discovered peer starts hidden from the friends list. The
	// matching acceptance response must promote both the relation and visibility.
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

	if err := NewEngine().SetPeerVisibleInFriends(ctx, "peer-hidden", true); err != nil {
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

func TestMarkConversationReadDoesNotRestoreHiddenFriend(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-read-hidden", Nickname: "隐藏好友", Relation: DiscoveredState}); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerRelation(ctx, "peer-read-hidden", PeerRelation); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerVisibleInFriends(ctx, "peer-read-hidden", false); err != nil {
		t.Fatal(err)
	}
	if err := NewEngine().MarkConversationRead(ctx, "peer-read-hidden"); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, PeerRelation)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].VisibleInFriends {
		t.Fatalf("读取聊天不应恢复隐藏好友: %+v", peers)
	}
}

func TestEngineHideGuardRejectsStaleVisibleSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-hide-guard", Nickname: "隐藏好友", Relation: PeerRelation, VisibleInFriends: true}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	if err := engine.SetPeerVisibleInFriends(ctx, "peer-hide-guard", false); err != nil {
		t.Fatal(err)
	}
	// Simulate a late discovery/connection writer that carries an old
	// visible=true value.  It must not make the just-hidden row reappear.
	if err := SetPeerVisibleInFriends(ctx, "peer-hide-guard", true); err != nil {
		t.Fatal(err)
	}
	peers := engine.Peers()
	if len(peers) != 1 || peers[0].VisibleInFriends {
		t.Fatalf("过期可见状态不应恢复隐藏好友: %+v", peers)
	}
	if err := engine.SetPeerVisibleInFriends(ctx, "peer-hide-guard", true); err != nil {
		t.Fatal(err)
	}
	peers = engine.Peers()
	if len(peers) != 1 || !peers[0].VisibleInFriends {
		t.Fatalf("显式恢复好友应清除隐藏保护: %+v", peers)
	}
}

func TestRemovalTombstoneDoesNotOverrideHiddenFriendsRow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-hidden-removed", Nickname: "隐藏的旧好友", Relation: PeerRelation, VisibleInFriends: true}); err != nil {
		t.Fatal(err)
	}
	if err := MarkFriendRemoved(ctx, "peer-hidden-removed"); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerRelation(ctx, "peer-hidden-removed", DiscoveredState); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerVisibleInFriends(ctx, "peer-hidden-removed", false); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].FriendshipState != "removed" || peers[0].VisibleInFriends {
		t.Fatalf("解除关系标记不应覆盖本机隐藏状态: %+v", peers)
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
	if len(peers) != 1 || peers[0].Relation != DiscoveredState || peers[0].FriendshipState != "removed" || !peers[0].VisibleInFriends || !peers[0].DiscoveryVisible {
		t.Fatalf("收到远端好友关系拒绝后列表项或关系状态错误: %+v", peers)
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
	if len(peers) != 1 || peers[0].Relation != DiscoveredState || peers[0].FriendshipState != "removed" || !peers[0].VisibleInFriends || !peers[0].DiscoveryVisible {
		t.Fatalf("解除好友帧未保留列表项并降级关系: %+v", peers)
	}
	removed, err := IsFriendRemoved(ctx, "peer-removed-notify")
	if err != nil || !removed {
		t.Fatalf("解除好友帧未留下关系删除标记: %v, %v", err, removed)
	}
}

func TestFriendRemovedFrameDoesNotClearNewPendingRequest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	ctx := context.Background()
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-pending-after-remove", Relation: PeerRelation, RelationshipVersion: "old-version"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveFriendRequest(ctx, FriendRequest{
		RequestID: "readd-pending",
		DeviceID:  "peer-pending-after-remove",
		Nickname:  "重新申请设备",
		Status:    "pending",
		Direction: "received",
		CreatedAt: nowString(),
	}); err != nil {
		t.Fatal(err)
	}

	NewEngine().handleWire(nil, wireMessage{DeviceID: "peer-pending-after-remove"}, wireMessage{
		Type:                "friend_removed",
		RelationshipVersion: "old-version",
	}, nil)
	requests, err := ListFriendRequests(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].RequestID != "readd-pending" || requests[0].Status != "pending" {
		t.Fatalf("关系同步不应清除新的待处理申请: %+v", requests)
	}
}

func TestStaleFriendRemovedFrameCannotUndoReaddedFriendship(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-versioned", Relation: PeerRelation, RelationshipVersion: "relationship-new", LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	engine.handleWire(nil, wireMessage{DeviceID: "peer-versioned"}, wireMessage{Type: "friend_removed", RelationshipVersion: "relationship-old"}, nil)
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != PeerRelation {
		t.Fatalf("旧关系删除通知错误解除新好友关系: %+v", peers)
	}
}

func TestUnversionedFriendRemovedFrameCannotUndoVersionedFriendship(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	ctx := context.Background()
	if err := db.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if err := UpsertPeer(ctx, Peer{DeviceID: "peer-versioned-legacy", Relation: PeerRelation, RelationshipVersion: "relationship-new", LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}

	NewEngine().handleWire(nil, wireMessage{DeviceID: "peer-versioned-legacy"}, wireMessage{Type: "friend_removed"}, nil)
	peers, err := ListPeers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Relation != PeerRelation {
		t.Fatalf("无版本旧关系删除通知错误解除新好友关系: %+v", peers)
	}
}

func TestHandshakeReportsRemovedFriendship(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := MarkFriendRemovedWithVersion(ctx, "peer-handshake-removed", "removed-version", "", ""); err != nil {
		t.Fatal(err)
	}
	state, version := NewEngine().friendshipStateForPeer("peer-handshake-removed")
	if state != "removed" || version != "removed-version" {
		t.Fatalf("握手删除状态错误: state=%q version=%q", state, version)
	}
}
