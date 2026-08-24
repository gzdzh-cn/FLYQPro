package chat

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

const friendRestoreVersion = 1

func friendRestorePayload(domain, sourceDeviceID, targetDeviceID, sourcePublicKey string) []byte {
	return []byte(fmt.Sprintf("%s/friend-restore/v1\n%s\n%s\n%s", domain, sourceDeviceID, targetDeviceID, sourcePublicKey))
}

func (e *Engine) friendRestoreMessage(targetDeviceID string) (wireMessage, error) {
	return e.friendRestoreMessageForDialect(targetDeviceID, protocolDialects[0])
}

func (e *Engine) friendRestoreMessageForDialect(targetDeviceID string, dialect ProtocolDialect) (wireMessage, error) {
	e.mu.RLock()
	identity := e.identity
	e.mu.RUnlock()
	privateKey, err := parseIdentityPrivateKey(identity.PrivateKeyPEM)
	if err != nil {
		return wireMessage{}, err
	}
	legacyDomain := "FlyQPro"
	if dialect.Name == "POPChat" {
		legacyDomain = "POPChat"
	}
	digest := sha256.Sum256(friendRestorePayload(legacyDomain, identity.DeviceID, targetDeviceID, identity.PublicKeyPEM))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return wireMessage{}, err
	}
	message := wireMessage{Type: "friend_restore", SourceDeviceID: identity.DeviceID, TargetDeviceID: targetDeviceID, SourcePublicKey: identity.PublicKeyPEM, RestoreVersion: friendRestoreVersion, RestoreSignature: base64.StdEncoding.EncodeToString(signature)}
	if dialect.Name == ProtocolName {
		v2Digest := sha256.Sum256(friendRestorePayload(ProtocolName, identity.DeviceID, targetDeviceID, identity.PublicKeyPEM))
		v2Signature, signErr := ecdsa.SignASN1(rand.Reader, privateKey, v2Digest[:])
		if signErr != nil {
			return wireMessage{}, signErr
		}
		message.RestoreSignatureV2 = base64.StdEncoding.EncodeToString(v2Signature)
	}
	return message, nil
}

func verifyFriendRestore(message wireMessage, hello wireMessage, localDeviceID string) error {
	if message.RestoreVersion != friendRestoreVersion {
		return errors.New("不支持的好友恢复版本")
	}
	if message.TargetDeviceID != localDeviceID || message.SourceDeviceID == "" || message.SourceDeviceID != hello.DeviceID {
		return errors.New("好友恢复目标设备不匹配")
	}
	if message.SourcePublicKey == "" || !strings.EqualFold(message.SourcePublicKey, hello.PublicKey) || !validDevicePublicKey(message.SourceDeviceID, message.SourcePublicKey) {
		return errors.New("好友恢复公钥不匹配")
	}
	block, _ := pem.Decode([]byte(message.SourcePublicKey))
	if block == nil {
		return errors.New("好友恢复公钥无效")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	ecdsaKey, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("好友恢复公钥类型不支持")
	}
	verify := func(value string, domains ...string) bool {
		if value == "" {
			return false
		}
		signature, decodeErr := base64.StdEncoding.DecodeString(value)
		if decodeErr != nil || len(signature) == 0 {
			return false
		}
		for _, domain := range domains {
			if domain == "" {
				continue
			}
			digest := sha256.Sum256(friendRestorePayload(domain, message.SourceDeviceID, message.TargetDeviceID, message.SourcePublicKey))
			if ecdsa.VerifyASN1(ecdsaKey, digest[:], signature) {
				return true
			}
		}
		return false
	}
	if message.RestoreSignatureV2 != "" && !verify(message.RestoreSignatureV2, ProtocolName) {
		return errors.New("好友恢复新版签名校验失败")
	}
	if message.RestoreSignature != "" && !verify(message.RestoreSignature, hello.Protocol, "FlyQPro", "POPChat") {
		return errors.New("好友恢复签名校验失败")
	}
	if message.RestoreSignature == "" && message.RestoreSignatureV2 == "" {
		return errors.New("好友恢复签名校验失败")
	}
	return nil
}
