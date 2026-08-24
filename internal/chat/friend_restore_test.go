package chat

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func testIdentity(t *testing.T) Identity {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	deviceID := sha256Hex(publicDER)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: deviceID}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return Identity{DeviceInfo: DeviceInfo{DeviceID: deviceID, PublicKeyPEM: publicPEM, CertificateFingerprint: sha256Hex(certificateDER)}, PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER})), CertificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}))}
}

func TestFriendRestoreSignature(t *testing.T) {
	source := testIdentity(t)
	engine := NewEngine()
	engine.identity = source
	message, err := engine.friendRestoreMessage("fresh-device")
	if err != nil {
		t.Fatal(err)
	}
	hello := wireMessage{DeviceID: source.DeviceID, PublicKey: source.PublicKeyPEM}
	if err := verifyFriendRestore(message, hello, "fresh-device"); err != nil {
		t.Fatalf("有效好友恢复声明被拒绝: %v", err)
	}
	message.TargetDeviceID = "another-device"
	if err := verifyFriendRestore(message, hello, "fresh-device"); err == nil {
		t.Fatal("错误目标设备应被拒绝")
	}
}

func TestFriendRestoreRejectsTamperedSignature(t *testing.T) {
	source := testIdentity(t)
	engine := NewEngine()
	engine.identity = source
	message, err := engine.friendRestoreMessage("fresh-device")
	if err != nil {
		t.Fatal(err)
	}
	message.RestoreSignature = message.RestoreSignature[:len(message.RestoreSignature)-2] + "aa"
	hello := wireMessage{DeviceID: source.DeviceID, PublicKey: source.PublicKeyPEM}
	if err := verifyFriendRestore(message, hello, "fresh-device"); err == nil {
		t.Fatal("篡改后的好友恢复签名应被拒绝")
	}
}

func TestFriendRestoreSupportsDzhgoAndPopchatSignatures(t *testing.T) {
	source := testIdentity(t)
	engine := NewEngine()
	engine.identity = source

	dzhgoMessage, err := engine.friendRestoreMessageForDialect("fresh-device", protocolDialects[0])
	if err != nil {
		t.Fatal(err)
	}
	dzhgoHello := wireMessage{DeviceID: source.DeviceID, PublicKey: source.PublicKeyPEM, Protocol: "dzhgo", Major: 2, Magic: "DZHGO_DISCOVERY_V1"}
	if err := verifyFriendRestore(dzhgoMessage, dzhgoHello, "fresh-device"); err != nil {
		t.Fatalf("dzhgo 好友恢复签名校验失败: %v", err)
	}

	popMessage, err := engine.friendRestoreMessageForDialect("fresh-device", protocolDialects[2])
	if err != nil {
		t.Fatal(err)
	}
	if popMessage.RestoreSignatureV2 != "" {
		t.Fatal("POPChat/v1 不应携带 dzhgo v2 签名")
	}
	popHello := wireMessage{DeviceID: source.DeviceID, PublicKey: source.PublicKeyPEM, Protocol: "POPChat", Major: 1, Magic: "POPCHAT_DISCOVERY_V1"}
	if err := verifyFriendRestore(popMessage, popHello, "fresh-device"); err != nil {
		t.Fatalf("POPChat 好友恢复签名校验失败: %v", err)
	}
}
