//go:build darwin && !ios

package startup

import (
	"encoding/xml"
	"os"
	"path/filepath"
)

type plist struct {
	XMLName xml.Name  `xml:"plist"`
	Version string    `xml:"version,attr"`
	Dict    plistDict `xml:"dict"`
}

type plistDict struct {
	Keys    []string `xml:"key"`
	Strings []string `xml:"string"`
}

func Set(enabled bool, executable string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "Library", "LaunchAgents", "com.dzh.flyqpro.desktop.plist")
	legacyProductPath := filepath.Join(home, "Library", "LaunchAgents", "com.dzh.popchat.desktop.plist")
	legacyPath := filepath.Join(home, "Library", "LaunchAgents", "com.lanchat.desktop.plist")
	if !enabled {
		for _, oldPath := range []string{path, legacyProductPath, legacyPath} {
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	for _, oldPath := range []string{legacyProductPath, legacyPath} {
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	content := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict><key>Label</key><string>com.dzh.flyqpro.desktop</string><key>ProgramArguments</key><array><string>" + xmlEscape(executable) + "</string></array><key>RunAtLoad</key><true/></dict></plist>\n")
	return os.WriteFile(path, content, 0o600)
}

func xmlEscape(value string) string {
	result, _ := xml.Marshal(value)
	if len(result) > 2 {
		return string(result[1 : len(result)-1])
	}
	return value
}
