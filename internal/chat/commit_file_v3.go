package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// The source has already passed whole-file verification. A cross-filesystem
// copy is verified and synced under a temporary name before becoming visible.
func commitVerifiedV3File(source, target string, size int64, sum string) error {
	persistence := currentTransferPersistenceIO()
	if err := persistence.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	if err := persistence.Rename(source, target); err == nil {
		return syncCommittedV3File(target)
	}
	input, err := persistence.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := persistence.CreateTemp(filepath.Dir(target), ".flyqpro-commit-*")
	if err != nil {
		return err
	}
	temporary := output.Name()
	defer persistence.Remove(temporary)
	digest := sha256.New()
	n, err := io.Copy(io.MultiWriter(output, digest), input)
	if err == nil && (n != size || !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), sum)) {
		err = fmt.Errorf("v3 copy verification failed")
	}
	if err == nil {
		err = persistence.SyncFile(output)
	}
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := persistence.Rename(temporary, target); err != nil {
		return err
	}
	if err := syncCommittedV3File(target); err != nil {
		return err
	}
	// The verified destination is committed even if cleaning the source fails.
	_ = persistence.Remove(source)
	return nil
}

func syncCommittedV3File(path string) error {
	persistence := currentTransferPersistenceIO()
	file, err := persistence.Open(path)
	if err != nil {
		return err
	}
	err = persistence.SyncFile(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return persistence.SyncDirectory(filepath.Dir(path))
}
