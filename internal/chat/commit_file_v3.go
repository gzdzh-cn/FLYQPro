package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The source has already passed whole-file verification. A cross-filesystem
// copy is verified and synced under a temporary name before becoming visible.
func commitVerifiedV3File(source, target string, size int64, sum string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(target), ".flyqpro-commit-*")
	if err != nil {
		return err
	}
	temporary := output.Name()
	defer os.Remove(temporary)
	digest := sha256.New()
	n, err := io.Copy(io.MultiWriter(output, digest), input)
	if err == nil && (n != size || !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), sum)) {
		err = fmt.Errorf("v3 copy verification failed")
	}
	if err == nil {
		err = output.Sync()
	}
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		return err
	}
	// The verified destination is committed even if cleaning the source fails.
	_ = os.Remove(source)
	return nil
}
