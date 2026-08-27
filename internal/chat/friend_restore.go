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

// shouldSendFriendRestore reports whether a normal authenticated connection
// may restore an existing friendship before its payload. An explicit friend
// request is deliberately excluded: it must reach the receiver while the
// relationship is still unknown so a freshly initialized database can save it
// as a pending request instead of silently auto-accepting it.
func shouldSendFriendRestore(messageType string) bool {
	// Relationship control frames must be delivered without a stale restore
	// frame in front of them. In particular, an accepted response is the frame
	// that establishes the new relationship after a previous removal.
	return messageType != "friend_restore" && messageType != "friend_request" && messageType != "friend_request_response" && messageType != "friend_removed"
}

// allowsRemovedFriendshipFrame identifies relationship-control frames that
// must still cross a stale removal handshake. In particular, the acceptance
// response is the only frame that can complete a new add cycle.
func allowsRemovedFriendshipFrame(messageType string) bool {
	return messageType == "friend_request" || messageType == "friend_request_response" || messageType == "friend_removed"
}

func friendRestorePayload(domain, sourceDeviceID, targetDeviceID, sourcePublicKey string) []byte {
	return []byte(fmt.Sprintf("%s/friend-restore/v1\n%s\n%s\n%s", domain, sourceDeviceID, targetDeviceID, sourcePublicKey))
}

func (e *Engine) friendRestoreMessage(targetDeviceID string) (wireMessage, error) {
	return e.friendRestoreMessageForDialect(targetDeviceID, protocolDialects[0])
}

func (e *Engine) friendRestoreMessageForDialect(targetDeviceID string, _ ProtocolDialect) (wireMessage, error) {
	e.mu.RLock()
	identity := e.identity
	e.mu.RUnlock()
	privateKey, err := parseIdentityPrivateKey(identity.PrivateKeyPEM)
	if err != nil {
		return wireMessage{}, err
	}
	digest := sha256.Sum256(friendRestorePayload(ProtocolName, identity.DeviceID, targetDeviceID, identity.PublicKeyPEM))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return wireMessage{}, err
	}
	message := wireMessage{Type: "friend_restore", SourceDeviceID: identity.DeviceID, TargetDeviceID: targetDeviceID, SourcePublicKey: identity.PublicKeyPEM, RestoreVersion: friendRestoreVersion, RestoreSignature: base64.StdEncoding.EncodeToString(signature)}
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
	if message.RestoreSignature == "" || hello.Protocol != ProtocolName || !verify(message.RestoreSignature, ProtocolName) {
		return errors.New("好友恢复签名校验失败")
	}
	return nil
}
