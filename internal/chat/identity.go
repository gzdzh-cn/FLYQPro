package chat

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Identity struct {
	DeviceInfo
	PrivateKeyPEM  string
	CertificatePEM string
}

func AppDataDir() string {
	if value := strings.TrimSpace(os.Getenv("LANCHAT_DATA_DIR")); value != "" {
		return value
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = "."
	}
	return filepath.Join(base, "LANChat")
}

func DefaultAttachmentDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, "Downloads", "POPChat")
	}
	return filepath.Join(AppDataDir(), "attachments")
}

func EnsureDataDirs() error {
	for _, path := range []string{AppDataDir(), filepath.Join(AppDataDir(), "attachments"), filepath.Join(AppDataDir(), "temp")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func LoadOrCreateIdentity(ctx context.Context) (Identity, error) {
	if row, err := GetIdentity(ctx); err == nil {
		return Identity{DeviceInfo: DeviceInfo{DeviceID: row.DeviceID, PublicKeyPEM: row.PublicKeyPEM, CertificateFingerprint: row.CertificateFingerprint}, PrivateKeyPEM: row.PrivateKeyPEM, CertificatePEM: row.CertificatePEM}, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Identity{}, fmt.Errorf("生成设备密钥失败: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Identity{}, fmt.Errorf("编码设备私钥失败: %w", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return Identity{}, fmt.Errorf("编码设备公钥失败: %w", err)
	}
	deviceID := sha256Hex(publicDER)

	serial := newSerial()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: deviceID}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(20, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return Identity{}, fmt.Errorf("生成设备证书失败: %w", err)
	}
	privatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER}))
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	certificatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}))
	identity := Identity{DeviceInfo: DeviceInfo{DeviceID: deviceID, PublicKeyPEM: publicPEM, CertificateFingerprint: sha256Hex(certificateDER)}, PrivateKeyPEM: privatePEM, CertificatePEM: certificatePEM}
	if err := SaveIdentity(ctx, identity.DeviceInfo, identity.PrivateKeyPEM, identity.CertificatePEM); err != nil {
		return Identity{}, fmt.Errorf("保存设备身份失败: %w", err)
	}
	return identity, nil
}

func (i Identity) TLSCertificate() (tlsCertificate tls.Certificate, err error) {
	return tls.X509KeyPair([]byte(i.CertificatePEM), []byte(i.PrivateKeyPEM))
}

func platformInfo() (string, string) {
	platform := runtime.GOOS
	switch platform {
	case "darwin":
		platform = "macOS"
	case "windows":
		platform = "Windows"
	case "linux":
		platform = "Linux"
	}
	return platform, runtime.GOOS + " " + runtime.GOARCH
}

func localIPv4() string {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip != nil && ip.To4() != nil {
				return ip.To4().String()
			}
		}
	}
	return ""
}

func sha256Hex(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func newSerial() *big.Int {
	value, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return value
}
