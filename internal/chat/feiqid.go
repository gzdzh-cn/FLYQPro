package chat

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
)

// FeiqID is a short, user-facing identifier derived from the stable device
// identity. It is deliberately separate from DeviceID, which remains the
// complete protocol identity used for authentication and routing.
func FeiqID(deviceID string) string {
	value := strings.TrimSpace(deviceID)
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:])
	if len(encoded) > 11 {
		encoded = encoded[:11]
	}
	check := byte(0)
	for _, char := range digest {
		check ^= char
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return encoded + string(alphabet[int(check)%len(alphabet)])
}
