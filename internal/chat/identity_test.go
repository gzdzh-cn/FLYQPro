package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helpfly/internal/service/db"
)

func TestIdentityAndProfilePersist(t *testing.T) {
	root := t.TempDir()
	_ = os.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	_ = os.Setenv("LANCHAT_DATA_DIR", filepath.Join(root, "data"))
	defer os.Unsetenv("GOFLY_DB_PATH")
	defer os.Unsetenv("LANCHAT_DATA_DIR")
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	if err := EnsureDefaults(context.Background(), filepath.Join(root, "attachments")); err != nil {
		t.Fatal(err)
	}
	first, err := LoadOrCreateIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.DeviceID == "" || first.CertificateFingerprint == "" {
		t.Fatal("身份指纹不能为空")
	}
	if first.DeviceID != second.DeviceID || first.CertificateFingerprint != second.CertificateFingerprint {
		t.Fatal("身份未保持稳定")
	}
	if certificate, err := first.TLSCertificate(); err != nil || len(certificate.Certificate) != 1 {
		t.Fatalf("TLS 证书不可用: %v", err)
	}
	profile, err := GetProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if profile.Nickname == "" || profile.Nickname == "新用户" {
		t.Fatalf("默认昵称未生成: %q", profile.Nickname)
	}
	profile.Nickname = "测试用户"
	if err := SaveProfile(context.Background(), profile); err != nil {
		t.Fatal(err)
	}
	loaded, err := GetProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Nickname != "测试用户" {
		t.Fatalf("资料未持久化: %q", loaded.Nickname)
	}
	if err := db.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	loadedAgain, err := GetProfile(context.Background())
	if err != nil || loadedAgain.Nickname != "测试用户" {
		t.Fatalf("SQLite 重启恢复失败: %v, %+v", err, loadedAgain)
	}
}

func TestWireMessageIsPortableJSON(t *testing.T) {
	message := wireMessage{Type: "message", Protocol: ProtocolName, Major: ProtocolMajor, Minor: ProtocolMinor, MessageID: "message-1", Content: "你好", ChunkIndex: 2}
	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["type"] != "message" || decoded["content"] != "你好" {
		t.Fatalf("协议 JSON 不可解析: %s", data)
	}
}

func TestRandomChineseNicknameUsesThemeWords(t *testing.T) {
	prefixes := []string{"薄荷", "星际", "云端", "彩虹", "泡泡", "月光", "棉花糖", "银河", "森林", "魔法"}
	themes := []string{"水母", "熊猫", "仙人掌", "小狐狸", "独角兽", "向日葵", "蒲公英", "月亮", "小精灵", "鲸鱼", "樱桃", "小火车"}
	for index := 0; index < 32; index++ {
		nickname := randomChineseNickname()
		if len([]rune(nickname)) > 8 || nickname == "" || nickname == "新用户" {
			t.Fatalf("昵称格式异常: %q", nickname)
		}
		prefixMatch := false
		for _, prefix := range prefixes {
			prefixMatch = prefixMatch || strings.HasPrefix(nickname, prefix)
		}
		themeMatch := false
		for _, theme := range themes {
			themeMatch = themeMatch || strings.HasSuffix(nickname, theme)
		}
		if !prefixMatch || !themeMatch {
			t.Fatalf("昵称未使用主题词组合: %q", nickname)
		}
	}
}

func TestUpsertPeerPreservesFriendRelationAndRemark(t *testing.T) {
	root := t.TempDir()
	_ = os.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	defer os.Unsetenv("GOFLY_DB_PATH")
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	if err := UpsertPeer(context.Background(), Peer{DeviceID: "peer-1", Nickname: "旧昵称", Relation: DiscoveredState, LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerRelation(context.Background(), "peer-1", PeerRelation); err != nil {
		t.Fatal(err)
	}
	if err := SetPeerRemark(context.Background(), "peer-1", "我的备注"); err != nil {
		t.Fatal(err)
	}
	if err := UpsertPeer(context.Background(), Peer{DeviceID: "peer-1", Nickname: "新昵称", Relation: DiscoveredState, LastSeen: nowString()}); err != nil {
		t.Fatal(err)
	}
	peers, err := ListPeers(context.Background(), "")
	if err != nil || len(peers) != 1 {
		t.Fatalf("读取好友失败: %v, %d", err, len(peers))
	}
	if peers[0].Relation != PeerRelation || peers[0].Remark != "我的备注" {
		t.Fatalf("好友关系被发现更新覆盖: %+v", peers[0])
	}
}

func TestConversationUnreadAndOutboxPersistence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLY_DB_PATH", filepath.Join(root, "chat.db"))
	if err := db.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer db.Close(context.Background())
	ctx := context.Background()
	if err := EnsureDefaults(ctx, filepath.Join(root, "attachments")); err != nil {
		t.Fatal(err)
	}
	conversationID, err := EnsureConversation(ctx, "peer-unread")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveMessage(ctx, Message{MessageID: "unread-message", ConversationID: conversationID, SenderDeviceID: "peer-unread", Kind: "text", Content: "hello", Status: "sent", CreatedAt: nowString()}); err != nil {
		t.Fatal(err)
	}
	if err := IncrementConversationUnread(ctx, conversationID); err != nil {
		t.Fatal(err)
	}
	conversations, err := ListConversations(ctx)
	if err != nil || len(conversations) != 1 || conversations[0].UnreadCount != 1 {
		t.Fatalf("未读数未持久化: %v, %+v", err, conversations)
	}
	if err := ClearConversationUnread(ctx, conversationID); err != nil {
		t.Fatal(err)
	}
	conversations, err = ListConversations(ctx)
	if err != nil || len(conversations) != 1 || conversations[0].UnreadCount != 0 {
		t.Fatalf("未读数未清理: %v, %+v", err, conversations)
	}
	if err := SaveOutbox(ctx, "outbox-message", "peer-unread", "message", `{"type":"message","messageId":"unread-message"}`); err != nil {
		t.Fatal(err)
	}
	items, err := ListOutbox(ctx, "peer-unread")
	if err != nil || len(items) != 1 || items[0].ItemID != "outbox-message" {
		t.Fatalf("outbox 未持久化: %v, %+v", err, items)
	}
	if err := MarkOutboxRetry(ctx, items[0].ItemID, items[0].Attempts); err != nil {
		t.Fatal(err)
	}
	if err := DeleteOutbox(ctx, items[0].ItemID); err != nil {
		t.Fatal(err)
	}
}
