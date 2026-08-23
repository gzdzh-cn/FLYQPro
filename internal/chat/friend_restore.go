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

func friendRestorePayload(sourceDeviceID, targetDeviceID, sourcePublicKey string) []byte {
	return []byte(fmt.Sprintf("POPChat/friend-restore/v1\n%s\n%s\n%s", sourceDeviceID, targetDeviceID, sourcePublicKey))
}

func (e *Engine) friendRestoreMessage(targetDeviceID string) (wireMessage, error) {
	e.mu.RLock()
	identity := e.identity
	e.mu.RUnlock()
	privateKey, err := parseIdentityPrivateKey(identity.PrivateKeyPEM)
	if err != nil {
		return wireMessage{}, err
	}
	digest := sha256.Sum256(friendRestorePayload(identity.DeviceID, targetDeviceID, identity.PublicKeyPEM))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return wireMessage{}, err
	}
	return wireMessage{Type: "friend_restore", SourceDeviceID: identity.DeviceID, TargetDeviceID: targetDeviceID, SourcePublicKey: identity.PublicKeyPEM, RestoreVersion: friendRestoreVersion, RestoreSignature: base64.StdEncoding.EncodeToString(signature)}, nil
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
	signature, err := base64.StdEncoding.DecodeString(message.RestoreSignature)
	if err != nil || len(signature) == 0 {
		return errors.New("好友恢复签名无效")
	}
	digest := sha256.Sum256(friendRestorePayload(message.SourceDeviceID, message.TargetDeviceID, message.SourcePublicKey))
	if !ecdsa.VerifyASN1(ecdsaKey, digest[:], signature) {
		return errors.New("好友恢复签名校验失败")
	}
	return nil
}
