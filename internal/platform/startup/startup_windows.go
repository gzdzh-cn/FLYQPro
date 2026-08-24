//go:build windows

package startup

import (
	"os"
	"path/filepath"
)

func Set(enabled bool, executable string) error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return nil
	}
	path := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "FlyQPro.cmd")
	legacyPath := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "POPChat.cmd")
	if !enabled {
		for _, oldPath := range []string{path, legacyPath} {
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	content := "@echo off\nstart \"\" \"" + executable + "\"\n"
	return os.WriteFile(path, []byte(content), 0o600)
}
