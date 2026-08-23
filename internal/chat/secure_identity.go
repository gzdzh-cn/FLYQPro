package chat

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	secureIdentityService = "com.dzh.popchat.identity"
	secureIdentityVersion = 1
)

var errSecureIdentityNotFound = errors.New("secure identity not found")

type secureIdentityRecord struct {
	Version                int    `json:"version"`
	DeviceID               string `json:"deviceId"`
	PublicKeyPEM           string `json:"publicKeyPem"`
	PrivateKeyPEM          string `json:"privateKeyPem"`
	CertificatePEM         string `json:"certificatePem"`
	CertificateFingerprint string `json:"certificateFingerprint"`
}

func secureIdentityUser() string {
	sum := sha256.Sum256([]byte(AppDataDir()))
	return "device-identity-" + hex.EncodeToString(sum[:])[:24]
}

func secureIdentityEnabled() bool {
	return strings.TrimSpace(os.Getenv("POPCHAT_DISABLE_SECURE_IDENTITY")) != "1"
}

func loadSecureIdentity() (Identity, error) {
	if !secureIdentityEnabled() {
		return Identity{}, errSecureIdentityNotFound
	}
	value, err := keyring.Get(secureIdentityService, secureIdentityUser())
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Identity{}, errSecureIdentityNotFound
		}
		return Identity{}, err
	}
	var record secureIdentityRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return Identity{}, fmt.Errorf("解析系统设备身份失败: %w", err)
	}
	if record.Version != secureIdentityVersion {
		return Identity{}, fmt.Errorf("不支持的系统设备身份版本: %d", record.Version)
	}
	identity := Identity{DeviceInfo: DeviceInfo{DeviceID: record.DeviceID, PublicKeyPEM: record.PublicKeyPEM, CertificateFingerprint: record.CertificateFingerprint, IdentityStatus: "restored"}, PrivateKeyPEM: record.PrivateKeyPEM, CertificatePEM: record.CertificatePEM}
	if err := validateIdentity(identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func saveSecureIdentity(identity Identity) error {
	if !secureIdentityEnabled() {
		return nil
	}
	record := secureIdentityRecord{Version: secureIdentityVersion, DeviceID: identity.DeviceID, PublicKeyPEM: identity.PublicKeyPEM, PrivateKeyPEM: identity.PrivateKeyPEM, CertificatePEM: identity.CertificatePEM, CertificateFingerprint: identity.CertificateFingerprint}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return keyring.Set(secureIdentityService, secureIdentityUser(), string(data))
}

func validateIdentity(identity Identity) error {
	if identity.DeviceID == "" || identity.PublicKeyPEM == "" || identity.PrivateKeyPEM == "" || identity.CertificatePEM == "" {
		return errors.New("设备身份材料不完整")
	}
	publicBlock, _ := pem.Decode([]byte(identity.PublicKeyPEM))
	if publicBlock == nil {
		return errors.New("设备公钥无效")
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析设备公钥失败: %w", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil || !strings.EqualFold(identity.DeviceID, sha256Hex(publicDER)) {
		return errors.New("设备 ID 与公钥不匹配")
	}
	privateKey, err := parseIdentityPrivateKey(identity.PrivateKeyPEM)
	if err != nil {
		return err
	}
	privatePublic, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return errors.New("设备私钥类型不支持")
	}
	privateDER, err := x509.MarshalPKIXPublicKey(privatePublic)
	if err != nil || !strings.EqualFold(hex.EncodeToString(privateDER), hex.EncodeToString(publicDER)) {
		return errors.New("设备私钥与公钥不匹配")
	}
	certificateBlock, _ := pem.Decode([]byte(identity.CertificatePEM))
	if certificateBlock == nil {
		return errors.New("设备证书无效")
	}
	certificate, err := x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析设备证书失败: %w", err)
	}
	certificateDER, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil || !strings.EqualFold(hex.EncodeToString(certificateDER), hex.EncodeToString(publicDER)) {
		return errors.New("设备证书与公钥不匹配")
	}
	return nil
}

func parseIdentityPrivateKey(privatePEM string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return nil, errors.New("设备私钥无效")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析设备私钥失败: %w", err)
	}
	privateKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("设备私钥类型不支持")
	}
	return privateKey, nil
}
