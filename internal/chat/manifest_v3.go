package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ManifestEntryV3 struct {
	RelativePath string `json:"relativePath"`
	FileName     string `json:"fileName"`
	FileSize     int64  `json:"fileSize"`
	Mtime        int64  `json:"mtime"`
	SHA256       string `json:"sha256"`
	IsDirectory  bool   `json:"isDirectory"`
	MimeType     string `json:"mimeType,omitempty"`
}

func ValidateManifestPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("manifest path must be relative")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("manifest path escapes root")
	}
	for _, part := range strings.FieldsFunc(clean, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("invalid manifest path")
		}
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("manifest target escapes root")
	}
	return target, nil
}

func ValidateManifest(entries []ManifestEntryV3, root string, maxFiles int, maxBytes int64) error {
	if len(entries) > maxFiles {
		return errors.New("manifest file count exceeded")
	}
	var total int64
	for _, e := range entries {
		if _, err := ValidateManifestPath(root, e.RelativePath); err != nil {
			return err
		}
		if e.FileSize < 0 {
			return errors.New("manifest file size invalid")
		}
		total += e.FileSize
		if total > maxBytes {
			return errors.New("manifest size exceeded")
		}
	}
	return nil
}

// BuildManifestV3 produces a deterministic folder manifest before any data
// connection is opened. Hashing happens from the source files and the
// relative paths are normalized so the receiver can validate containment.
func BuildManifestV3(root string) ([]ManifestEntryV3, error) {
	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return nil, errors.New("manifest root must be a directory")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var entries []ManifestEntryV3
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, err := ValidateManifestPath(root, rel); err != nil {
			return err
		}
		entry := ManifestEntryV3{RelativePath: rel, FileName: info.Name(), FileSize: info.Size(), Mtime: info.ModTime().UnixMilli(), IsDirectory: info.IsDir()}
		if !info.IsDir() {
			h := sha256.New()
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(h, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			entry.SHA256 = hex.EncodeToString(h.Sum(nil))
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
