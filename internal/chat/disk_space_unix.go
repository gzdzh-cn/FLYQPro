//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package chat

import (
	"syscall"
)

func availableDiskBytes(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
