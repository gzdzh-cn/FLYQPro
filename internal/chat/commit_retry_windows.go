//go:build windows

package chat

import (
	"errors"
	"syscall"
	"time"
)

func renameV3WithRetry(persistence transferPersistenceIO, source, target string) error {
	var err error
	for attempt, delay := range []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond} {
		if attempt > 0 {
			time.Sleep(delay)
		}
		err = persistence.Rename(source, target)
		if err == nil || !retryableWindowsRenameError(err) {
			return err
		}
	}
	return err
}

func retryableWindowsRenameError(err error) bool {
	var code syscall.Errno
	if !errors.As(err, &code) {
		return false
	}
	switch code {
	case syscall.Errno(32), // ERROR_SHARING_VIOLATION
		syscall.Errno(33), // ERROR_LOCK_VIOLATION
		syscall.ERROR_ACCESS_DENIED:
		return true
	default:
		return false
	}
}
