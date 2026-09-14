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
	persistence := currentTransferPersistenceIO()
	if err := persistence.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	// EndFile may be retried after the destination was committed but before
	// metadata persistence completed. Treat an already committed, matching
	// destination as success. This is especially important on Windows where
	// Rename cannot replace an existing file and returns ACCESS_DENIED.
	if targetInfo, statErr := os.Stat(target); statErr == nil {
		if targetInfo.Size() != size {
			return fmt.Errorf("destination exists with unexpected size")
		}
		input, openErr := persistence.Open(target)
		if openErr != nil {
			return openErr
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), sum) {
			return fmt.Errorf("destination exists with different SHA-256")
		}
		if source != target {
			_ = persistence.Remove(source)
		}
		return syncCommittedV3Directory(filepath.Dir(target))
	}
	if err := renameV3WithRetry(persistence, source, target); err == nil {
		// The source was synced and closed before this function is called. On
		// Windows reopening the destination read-only and calling Sync can fail
		// with ERROR_ACCESS_DENIED, even though the atomic commit succeeded.
		return syncCommittedV3Directory(filepath.Dir(target))
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
	if err := renameV3WithRetry(persistence, temporary, target); err != nil {
		return err
	}
	if err := syncCommittedV3Directory(filepath.Dir(target)); err != nil {
		return err
	}
	// The verified destination is committed even if cleaning the source fails.
	_ = persistence.Remove(source)
	return nil
}

func syncCommittedV3File(path string) error {
	return syncCommittedV3Directory(filepath.Dir(path))
}

func syncCommittedV3Directory(directory string) error {
	return currentTransferPersistenceIO().SyncDirectory(directory)
}
