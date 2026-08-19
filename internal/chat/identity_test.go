package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
