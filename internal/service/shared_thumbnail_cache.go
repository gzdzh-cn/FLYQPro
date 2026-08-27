package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	sharedThumbnailVersion    = 1
	sharedThumbnailMaxBytes   = 512 * 1024 * 1024
	sharedThumbnailMaxAge     = 30 * 24 * time.Hour
	sharedThumbnailPartMaxAge = 2 * time.Hour
)

type sharedThumbnailRecord struct {
	CacheKey          string `json:"cacheKey"`
	RootPath          string `json:"rootPath"`
	SourcePath        string `json:"sourcePath"`
	FileIdentity      string `json:"fileIdentity,omitempty"`
	FileSize          int64  `json:"fileSize"`
	ModifiedAt        int64  `json:"modifiedAt"`
	MimeType          string `json:"mimeType"`
	ThumbnailMimeType string `json:"thumbnailMimeType"`
	ThumbnailVersion  int    `json:"thumbnailVersion"`
	ThumbnailPath     string `json:"thumbnailPath"`
	LastAccessedAt    string `json:"lastAccessedAt"`
}

type sharedThumbnailIndex struct {
	Records map[string]sharedThumbnailRecord `json:"records"`
}

type sharedThumbnailStore struct {
	mu       sync.Mutex
	root     string
	records  map[string]sharedThumbnailRecord
	inflight map[string]chan sharedThumbnailResult
}

type sharedThumbnailResult struct {
	path string
	mime string
	err  error
}

func newSharedThumbnailStore() *sharedThumbnailStore {
	root, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(root) == "" {
		root = os.TempDir()
	}
	root = filepath.Join(root, "FlyQPro", "shared-thumbnails")
	store := &sharedThumbnailStore{root: root, records: make(map[string]sharedThumbnailRecord), inflight: make(map[string]chan sharedThumbnailResult)}
	store.load()
	return store
}

func (s *sharedThumbnailStore) indexPath() string { return filepath.Join(s.root, "index.json") }

func (s *sharedThumbnailStore) load() {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return
	}
	var index sharedThumbnailIndex
	if json.Unmarshal(data, &index) == nil && index.Records != nil {
		s.records = index.Records
	}
}

func (s *sharedThumbnailStore) saveLocked() {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(sharedThumbnailIndex{Records: s.records})
	if err != nil {
		return
	}
	tmp := s.indexPath() + ".part"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, s.indexPath())
}

func sharedThumbnailIdentity(info os.FileInfo) string {
	if info == nil || info.Sys() == nil {
		return ""
	}
	v := reflect.ValueOf(info.Sys())
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	read := func(names ...string) (uint64, bool) {
		for _, name := range names {
			field := v.FieldByName(name)
			if !field.IsValid() || !field.CanUint() {
				continue
			}
			return field.Uint(), true
		}
		return 0, false
	}
	dev, hasDev := read("Dev", "VolumeSerialNumber")
	ino, hasIno := read("Ino", "FileIndex", "FileIndexLow")
	if !hasIno {
		high, highOK := read("FileIndexHigh")
		low, lowOK := read("FileIndexLow")
		if highOK && lowOK {
			ino = (high << 32) | low
			hasIno = true
		}
	}
	if !hasDev && !hasIno {
		return ""
	}
	return fmt.Sprintf("%d:%d", dev, ino)
}

func sharedThumbnailSignature(root, source string, info os.FileInfo, mimeType string) (string, string) {
	identity := sharedThumbnailIdentity(info)
	signature := fmt.Sprintf("%s\x00%d\x00%d\x00%s", identity, info.Size(), info.ModTime().UnixNano(), strings.ToLower(mimeType))
	return identity, signature
}

func sharedThumbnailCacheKey(root, source string) string {
	return filepath.Clean(root) + "\x00" + filepath.ToSlash(filepath.Clean(source))
}

func (s *sharedThumbnailStore) cacheFile(cacheKey string) string {
	digest := sha256.Sum256([]byte(cacheKey))
	return filepath.Join(s.root, hex.EncodeToString(digest[:])+".jpg")
}

// cached returns only an already complete thumbnail. It never decodes the
// source image, which keeps list rendering and double-click handling cheap.
func (s *sharedThumbnailStore) cached(root, source, mimeType string, info os.FileInfo) (string, string, bool) {
	if info == nil || info.IsDir() {
		return "", "", false
	}
	identity, _ := sharedThumbnailSignature(root, source, info, mimeType)
	cacheKey := sharedThumbnailCacheKey(root, source)
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[cacheKey]
	if !ok || record.ThumbnailVersion != sharedThumbnailVersion || record.FileSize != info.Size() || record.ModifiedAt != info.ModTime().UnixNano() || record.FileIdentity != identity || record.MimeType != strings.ToLower(strings.TrimSpace(mimeType)) {
		return "", "", false
	}
	cachedInfo, err := os.Stat(record.ThumbnailPath)
	if err != nil || !cachedInfo.Mode().IsRegular() || cachedInfo.Size() == 0 {
		return "", "", false
	}
	record.LastAccessedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.records[cacheKey] = record
	s.saveLocked()
	return record.ThumbnailPath, record.ThumbnailMimeType, true
}

func (s *sharedThumbnailStore) getOrCreate(root, source, mimeType string, info os.FileInfo) (string, string, error) {
	if info == nil || info.IsDir() {
		return "", "", fmt.Errorf("图片文件不存在")
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	identity, _ := sharedThumbnailSignature(root, source, info, mimeType)
	cacheKey := sharedThumbnailCacheKey(root, source)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	s.mu.Lock()
	if record, ok := s.records[cacheKey]; ok && record.ThumbnailVersion == sharedThumbnailVersion && record.FileSize == info.Size() && record.ModifiedAt == info.ModTime().UnixNano() && record.FileIdentity == identity && record.MimeType == mimeType {
		if cachedInfo, err := os.Stat(record.ThumbnailPath); err == nil && cachedInfo.Mode().IsRegular() && cachedInfo.Size() > 0 {
			record.LastAccessedAt = now
			s.records[cacheKey] = record
			s.saveLocked()
			s.mu.Unlock()
			return record.ThumbnailPath, record.ThumbnailMimeType, nil
		}
	}
	// A stable filesystem/SAF identity lets a rename reuse the generated image.
	// Path-only or changed files deliberately take the safe regeneration path.
	if identity != "" {
		for oldKey, record := range s.records {
			if record.RootPath != filepath.Clean(root) || record.FileIdentity != identity || record.FileSize != info.Size() || record.ModifiedAt != info.ModTime().UnixNano() || record.MimeType != mimeType || record.ThumbnailVersion != sharedThumbnailVersion {
				continue
			}
			if cachedInfo, err := os.Stat(record.ThumbnailPath); err != nil || !cachedInfo.Mode().IsRegular() || cachedInfo.Size() == 0 {
				continue
			}
			delete(s.records, oldKey)
			record.CacheKey = cacheKey
			record.SourcePath = filepath.ToSlash(filepath.Clean(source))
			record.LastAccessedAt = now
			s.records[cacheKey] = record
			s.saveLocked()
			s.mu.Unlock()
			return record.ThumbnailPath, record.ThumbnailMimeType, nil
		}
	}
	if wait, ok := s.inflight[cacheKey]; ok {
		s.mu.Unlock()
		result := <-wait
		return result.path, result.mime, result.err
	}
	wait := make(chan sharedThumbnailResult, 1)
	s.inflight[cacheKey] = wait
	s.mu.Unlock()

	data, generatedMime, err := generateSharedThumbnail(root, source, mimeType)
	if err != nil {
		s.finishThumbnail(cacheKey, wait, sharedThumbnailResult{err: err})
		return "", "", err
	}
	if len(data) == 0 {
		err := fmt.Errorf("生成缩略图失败")
		s.finishThumbnail(cacheKey, wait, sharedThumbnailResult{err: err})
		return "", "", err
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		s.finishThumbnail(cacheKey, wait, sharedThumbnailResult{err: err})
		return "", "", err
	}
	cachePath := s.cacheFile(cacheKey)
	partPath := cachePath + ".part"
	if err := os.WriteFile(partPath, data, 0o600); err != nil {
		s.finishThumbnail(cacheKey, wait, sharedThumbnailResult{err: err})
		return "", "", err
	}
	if err := os.Rename(partPath, cachePath); err != nil {
		_ = os.Remove(partPath)
		s.finishThumbnail(cacheKey, wait, sharedThumbnailResult{err: err})
		return "", "", err
	}
	s.mu.Lock()
	s.records[cacheKey] = sharedThumbnailRecord{
		CacheKey:          cacheKey,
		RootPath:          filepath.Clean(root),
		SourcePath:        filepath.ToSlash(filepath.Clean(source)),
		FileIdentity:      identity,
		FileSize:          info.Size(),
		ModifiedAt:        info.ModTime().UnixNano(),
		MimeType:          strings.ToLower(strings.TrimSpace(mimeType)),
		ThumbnailMimeType: generatedMime,
		ThumbnailVersion:  sharedThumbnailVersion,
		ThumbnailPath:     cachePath,
		LastAccessedAt:    now,
	}
	s.saveLocked()
	s.mu.Unlock()
	s.finishThumbnail(cacheKey, wait, sharedThumbnailResult{path: cachePath, mime: generatedMime})
	return cachePath, generatedMime, nil
}

func (s *sharedThumbnailStore) finishThumbnail(key string, wait chan sharedThumbnailResult, result sharedThumbnailResult) {
	s.mu.Lock()
	if current, ok := s.inflight[key]; ok && current == wait {
		delete(s.inflight, key)
	}
	s.mu.Unlock()
	wait <- result
	close(wait)
}

func (s *sharedThumbnailStore) dataURL(path, mimeType string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("缩略图缓存不存在")
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (s *sharedThumbnailStore) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var total int64
	for key, record := range s.records {
		remove := false
		if info, err := os.Stat(record.ThumbnailPath); err != nil || !info.Mode().IsRegular() {
			remove = true
		} else {
			total += info.Size()
			if sourceInfo, err := os.Stat(filepath.Join(record.RootPath, filepath.FromSlash(record.SourcePath))); err != nil || sourceInfo.IsDir() {
				remove = true
			}
			if accessed, err := time.Parse(time.RFC3339Nano, record.LastAccessedAt); err == nil && now.Sub(accessed) > sharedThumbnailMaxAge {
				remove = true
			}
		}
		if remove {
			_ = os.Remove(record.ThumbnailPath)
			delete(s.records, key)
		}
	}
	if entries, err := os.ReadDir(s.root); err == nil {
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".part") {
				continue
			}
			if info, statErr := entry.Info(); statErr == nil && now.Sub(info.ModTime()) > sharedThumbnailPartMaxAge {
				_ = os.Remove(filepath.Join(s.root, entry.Name()))
			}
		}
	}
	if total > sharedThumbnailMaxBytes {
		// A subsequent access will rebuild a deleted entry. The index is kept
		// small and deterministic; remove the oldest records first.
		for total > sharedThumbnailMaxBytes {
			var oldestKey string
			var oldest time.Time
			for key, record := range s.records {
				accessed, err := time.Parse(time.RFC3339Nano, record.LastAccessedAt)
				if err != nil || oldestKey == "" || accessed.Before(oldest) {
					oldestKey, oldest = key, accessed
				}
			}
			if oldestKey == "" {
				break
			}
			if info, err := os.Stat(s.records[oldestKey].ThumbnailPath); err == nil {
				total -= info.Size()
			}
			_ = os.Remove(s.records[oldestKey].ThumbnailPath)
			delete(s.records, oldestKey)
		}
	}
	s.saveLocked()
}

func (s *sharedThumbnailStore) startJanitor(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.prune()
			case <-stop:
				return
			}
		}
	}()
}
