package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	SharedDisabledError    = "SHARED_DISABLED"
	SharedUnavailableError = "SHARED_UNAVAILABLE"
	SharedPathInvalidError = "SHARED_PATH_INVALID"
)

func normalizeSharedRelativePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." {
		return "", nil
	}
	if strings.IndexByte(value, 0) >= 0 || strings.Contains(value, `\`) || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", fmt.Errorf("%s", SharedPathInvalidError)
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == "" {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s", SharedPathInvalidError)
	}
	return filepath.ToSlash(clean), nil
}

func sharedRootPath(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("%s", SharedUnavailableError)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%s", SharedUnavailableError)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("%s", SharedUnavailableError)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s", SharedUnavailableError)
	}
	return filepath.Clean(resolved), nil
}

// ValidateSharedRoot is the public boundary used by the service before a
// profile is saved. The wire layer uses the same resolver for every request.
func ValidateSharedRoot(root string) (string, error) { return sharedRootPath(root) }

func AvailableDiskBytes(path string) (int64, error) { return availableDiskBytes(path) }

func sharedPathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func resolveSharedPath(root, relative string, mustExist bool) (string, string, error) {
	normalized, err := normalizeSharedRelativePath(relative)
	if err != nil {
		return "", "", err
	}
	resolvedRoot, err := sharedRootPath(root)
	if err != nil {
		return "", "", err
	}
	candidate := filepath.Join(resolvedRoot, filepath.FromSlash(normalized))
	abs, err := filepath.Abs(candidate)
	if err != nil || !sharedPathWithinRoot(resolvedRoot, abs) {
		return "", "", fmt.Errorf("%s", SharedPathInvalidError)
	}
	if !mustExist {
		parentResolved, parentErr := filepath.EvalSymlinks(filepath.Dir(abs))
		if parentErr != nil || !sharedPathWithinRoot(resolvedRoot, parentResolved) {
			return "", "", fmt.Errorf("%s", SharedPathInvalidError)
		}
		return abs, normalized, nil
	}
	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil || !sharedPathWithinRoot(resolvedRoot, realPath) {
		return "", "", fmt.Errorf("%s", SharedPathInvalidError)
	}
	return realPath, normalized, nil
}

func sharedEntryID(relative string) string {
	digest := sha256.Sum256([]byte(relative))
	return hex.EncodeToString(digest[:])
}

func sharedEntryFromPath(root, relative, path string, info os.FileInfo, includeHash bool) (SharedEntry, error) {
	mimeType := ""
	if !info.IsDir() {
		mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}
	entry := SharedEntry{
		EntryID:      sharedEntryID(relative),
		Name:         info.Name(),
		RelativePath: filepath.ToSlash(relative),
		IsDirectory:  info.IsDir(),
		Size:         info.Size(),
		MimeType:     mimeType,
		ModifiedAt:   info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	if includeHash && !info.IsDir() {
		value, err := fileSHA256(path)
		if err != nil {
			return SharedEntry{}, err
		}
		entry.SHA256 = value
	}
	return entry, nil
}

func ListSharedEntries(root, relative string, showHiddenFiles ...bool) ([]SharedEntry, error) {
	showHidden := len(showHiddenFiles) > 0 && showHiddenFiles[0]
	directory, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%s", SharedPathInvalidError)
	}
	items, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("无法读取共享目录: %w", err)
	}
	entries := make([]SharedEntry, 0, len(items))
	for _, item := range items {
		itemInfo, infoErr := item.Info()
		if infoErr != nil || itemInfo.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !showHidden && strings.HasPrefix(item.Name(), ".") {
			continue
		}
		itemRelative := item.Name()
		if normalized != "" {
			itemRelative = filepath.Join(filepath.FromSlash(normalized), item.Name())
		}
		entry, entryErr := sharedEntryFromPath(root, filepath.ToSlash(itemRelative), filepath.Join(directory, item.Name()), itemInfo, false)
		if entryErr != nil {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDirectory != entries[j].IsDirectory {
			return entries[i].IsDirectory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

const defaultSharedEntriesPageSize = 100
const maxSharedEntriesPageSize = 200

// ListSharedEntriesPage reads only one bounded page from a directory. Unlike
// os.ReadDir it does not materialise a large directory before returning.
func ListSharedEntriesPage(root, relative string, offset, limit int, showHiddenFiles ...bool) (SharedEntriesPage, error) {
	showHidden := len(showHiddenFiles) > 0 && showHiddenFiles[0]
	directory, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil {
		return SharedEntriesPage{}, err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return SharedEntriesPage{}, fmt.Errorf("%s", SharedPathInvalidError)
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = defaultSharedEntriesPageSize
	}
	if limit > maxSharedEntriesPageSize {
		limit = maxSharedEntriesPageSize
	}
	dir, err := os.Open(directory)
	if err != nil {
		return SharedEntriesPage{}, fmt.Errorf("无法读取共享目录: %w", err)
	}
	defer dir.Close()

	// Readdir(1) deliberately keeps memory bounded, including directories with
	// hundreds of thousands of files. Offset counts raw directory entries so
	// skipped symbolic links do not make a later page repeat earlier items.
	cursor := 0
	for cursor < offset {
		items, readErr := dir.ReadDir(1)
		if len(items) > 0 {
			cursor++
		}
		if readErr == io.EOF || len(items) == 0 {
			return SharedEntriesPage{Entries: []SharedEntry{}, NextOffset: cursor}, nil
		}
		if readErr != nil {
			return SharedEntriesPage{}, fmt.Errorf("无法读取共享目录: %w", readErr)
		}
	}

	entries := make([]SharedEntry, 0, limit)
	for len(entries) < limit {
		items, readErr := dir.ReadDir(1)
		if len(items) == 0 {
			if readErr == io.EOF {
				return SharedEntriesPage{Entries: entries, NextOffset: cursor}, nil
			}
			if readErr != nil {
				return SharedEntriesPage{}, fmt.Errorf("无法读取共享目录: %w", readErr)
			}
			continue
		}
		item := items[0]
		cursor++
		if !showHidden && strings.HasPrefix(item.Name(), ".") {
			continue
		}
		itemInfo, infoErr := item.Info()
		if infoErr != nil || itemInfo.Mode()&os.ModeSymlink != 0 {
			if readErr != nil && readErr != io.EOF {
				return SharedEntriesPage{}, fmt.Errorf("无法读取共享目录: %w", readErr)
			}
			continue
		}
		itemRelative := item.Name()
		if normalized != "" {
			itemRelative = filepath.Join(filepath.FromSlash(normalized), item.Name())
		}
		entry, entryErr := sharedEntryFromPath(root, filepath.ToSlash(itemRelative), filepath.Join(directory, item.Name()), itemInfo, false)
		if entryErr == nil {
			entries = append(entries, entry)
		}
		if readErr != nil && readErr != io.EOF {
			return SharedEntriesPage{}, fmt.Errorf("无法读取共享目录: %w", readErr)
		}
	}

	// Probe one raw entry to learn whether a subsequent page exists, but keep
	// the returned next offset before the probe so the next request re-reads it.
	nextOffset := cursor
	items, _ := dir.ReadDir(1)
	return SharedEntriesPage{Entries: entries, NextOffset: nextOffset, HasMore: len(items) > 0}, nil
}

func GetSharedEntry(root, relative string, includeHash bool) (SharedEntry, string, error) {
	path, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil {
		return SharedEntry{}, "", err
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return SharedEntry{}, "", fmt.Errorf("%s", SharedPathInvalidError)
	}
	entry, err := sharedEntryFromPath(root, normalized, path, info, includeHash)
	if err != nil {
		return SharedEntry{}, "", err
	}
	if entry.IsDirectory && includeHash {
		size, err := sharedDirectorySize(path)
		if err != nil {
			return SharedEntry{}, "", err
		}
		entry.Size = size
	}
	return entry, path, nil
}

// GetSharedEntryThumbnail returns a bounded JPEG preview for an image in the
// shared root. It deliberately never returns the original file bytes, so the
// shared-drive grid can render previews without downloading large originals.
func GetSharedEntryThumbnail(root, relative string) (string, string, error) {
	entry, path, err := GetSharedEntry(root, relative, false)
	if err != nil {
		return "", "", err
	}
	if entry.IsDirectory || !isSharedImageEntry(entry) {
		return "", "", fmt.Errorf("该文件不是图片")
	}
	return buildImageThumbnail(path, entry.MimeType)
}

func isSharedImageEntry(entry SharedEntry) bool {
	mimeType := strings.ToLower(strings.TrimSpace(entry.MimeType))
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(entry.Name)) {
	case ".avif", ".bmp", ".gif", ".heic", ".heif", ".jpeg", ".jpg", ".png", ".webp":
		return true
	default:
		return false
	}
}

// sharedDirectorySize reports the total byte size of regular files below a
// directory. Symbolic links are intentionally excluded because shared-drive
// paths must never escape the configured root.
func sharedDirectorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func DefaultSharedDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(DefaultAttachmentDir(), "shared-downloads")
	}
	return filepath.Join(home, "Downloads", "FLYQPro")
}

func uniqueSharedTarget(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	extension := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), extension)
	directory := filepath.Dir(path)
	for index := 1; ; index++ {
		candidate := filepath.Join(directory, fmt.Sprintf("%s (%d)%s", base, index, extension))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func sharedEntryName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	return nil
}

func CreateSharedFolder(root, relative, name string) (SharedEntry, error) {
	if err := sharedEntryName(name); err != nil {
		return SharedEntry{}, err
	}
	parent, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil {
		return SharedEntry{}, err
	}
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return SharedEntry{}, fmt.Errorf("%s", SharedPathInvalidError)
	}
	target := filepath.Join(parent, name)
	if _, _, err := resolveSharedPath(root, filepath.ToSlash(filepath.Join(normalized, name)), false); err != nil {
		return SharedEntry{}, err
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		return SharedEntry{}, err
	}
	created, err := os.Stat(target)
	if err != nil {
		return SharedEntry{}, err
	}
	entryRelative := filepath.ToSlash(filepath.Join(normalized, name))
	return sharedEntryFromPath(root, entryRelative, target, created, false)
}

func RenameSharedEntry(root, relative, newName string) error {
	if err := sharedEntryName(newName); err != nil {
		return err
	}
	source, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil || normalized == "" {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	parent := filepath.Dir(source)
	target := filepath.Join(parent, newName)
	if _, _, err := resolveSharedPath(root, filepath.ToSlash(filepath.Join(filepath.Dir(normalized), newName)), false); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("目标名称已存在")
	}
	return os.Rename(source, target)
}

func MoveSharedEntry(root, relative, targetDirectory string) error {
	source, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil || normalized == "" {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	directory, targetNormalized, err := resolveSharedPath(root, targetDirectory, true)
	if err != nil {
		return err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	target := filepath.Join(directory, filepath.Base(source))
	if _, _, err := resolveSharedPath(root, filepath.ToSlash(filepath.Join(targetNormalized, filepath.Base(source))), false); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("目标名称已存在")
	}
	if filepath.Clean(source) == filepath.Clean(directory) || strings.HasPrefix(filepath.Clean(directory), filepath.Clean(source)+string(filepath.Separator)) {
		return fmt.Errorf("不能将文件夹移动到自身内部")
	}
	return os.Rename(source, target)
}

func copySharedTree(source, target string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	if info.IsDir() {
		if err := os.Mkdir(target, 0o700); err != nil {
			return err
		}
		items, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, item := range items {
			childInfo, err := item.Info()
			if err != nil || childInfo.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%s", SharedPathInvalidError)
			}
			if err := copySharedTree(filepath.Join(source, item.Name()), filepath.Join(target, item.Name()), childInfo); err != nil {
				return err
			}
		}
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func CopySharedEntry(root, relative, targetDirectory string) error {
	source, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil || normalized == "" {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	directory, targetNormalized, err := resolveSharedPath(root, targetDirectory, true)
	if err != nil {
		return err
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil || !directoryInfo.IsDir() {
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	info, err := os.Stat(source)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s", SharedPathInvalidError)
	}
	name := filepath.Base(source)
	target := filepath.Join(directory, name)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("目标名称已存在")
	}
	if info.IsDir() && (filepath.Clean(source) == filepath.Clean(directory) || strings.HasPrefix(filepath.Clean(directory), filepath.Clean(source)+string(filepath.Separator))) {
		return fmt.Errorf("不能将文件夹复制到自身内部")
	}
	if _, _, err := resolveSharedPath(root, filepath.ToSlash(filepath.Join(targetNormalized, name)), false); err != nil {
		return err
	}
	return copySharedTree(source, target, info)
}

func DeleteSharedEntry(root, relative string) error {
	path, normalized, err := resolveSharedPath(root, relative, true)
	if err != nil {
		return err
	}
	if normalized == "" {
		return fmt.Errorf("不能删除共享根目录")
	}
	return os.RemoveAll(path)
}
