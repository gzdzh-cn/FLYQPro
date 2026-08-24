//go:build windows

package chat

import (
	"golang.org/x/sys/windows"
	"strings"
)

func availableDiskBytes(path string) (int64, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = `C:\`
	}
	var free, total, available uint64
	if err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(path), &free, &total, &available); err != nil {
		return 0, err
	}
	return int64(available), nil
}
