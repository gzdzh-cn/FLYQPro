package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"flyqpro/internal/chat"
)

type remoteThumbnailRecord struct {
	CacheKey       string `json:"cacheKey"`
	DeviceID       string `json:"deviceId"`
	EntryID        string `json:"entryId,omitempty"`
	RelativePath   string `json:"relativePath"`
	FileSize       int64  `json:"fileSize"`
	ModifiedAt     string `json:"modifiedAt"`
	MimeType       string `json:"mimeType"`
	ThumbnailPath  string `json:"thumbnailPath"`
	LastAccessedAt string `json:"lastAccessedAt"`
}

type remoteThumbnailCache struct {
	mu      sync.Mutex
	root    string
	records map[string]remoteThumbnailRecord
}

func newRemoteThumbnailCache() *remoteThumbnailCache {
	root, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(root) == "" {
		root = os.TempDir()
	}
	root = filepath.Join(root, "FlyQPro", "shared-thumbnails", "remote")
	cache := &remoteThumbnailCache{root: root, records: make(map[string]remoteThumbnailRecord)}
	cache.load()
	return cache
}

func (c *remoteThumbnailCache) indexPath() string { return filepath.Join(c.root, "index.json") }

func (c *remoteThumbnailCache) load() {
	data, err := os.ReadFile(c.indexPath())
	if err != nil {
		return
	}
	var records map[string]remoteThumbnailRecord
	if json.Unmarshal(data, &records) == nil && records != nil {
		c.records = records
	}
}

func (c *remoteThumbnailCache) saveLocked() {
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(c.records)
	if err != nil {
		return
	}
	part := c.indexPath() + ".part"
	if err := os.WriteFile(part, data, 0o600); err == nil {
		_ = os.Rename(part, c.indexPath())
	}
}

func remoteThumbnailKey(deviceID string, request chat.SharedThumbnailRequest) string {
	return fmt.Sprintf("1\x00%s\x00%s\x00%s\x00%d\x00%s", deviceID, request.EntryID, filepath.ToSlash(filepath.Clean(request.RelativePath)), request.FileSize, request.ModifiedAt)
}

func (c *remoteThumbnailCache) cachePath(key string) string {
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(c.root, hex.EncodeToString(digest[:])+".jpg")
}

func (c *remoteThumbnailCache) cached(deviceID string, request chat.SharedThumbnailRequest) (chat.SharedThumbnailResult, bool) {
	key := remoteThumbnailKey(deviceID, request)
	c.mu.Lock()
	defer c.mu.Unlock()
	record, ok := c.records[key]
	if !ok && request.EntryID != "" {
		for oldKey, candidate := range c.records {
			if candidate.DeviceID == deviceID && candidate.EntryID == request.EntryID && candidate.FileSize == request.FileSize && candidate.ModifiedAt == request.ModifiedAt {
				if info, err := os.Stat(candidate.ThumbnailPath); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
					delete(c.records, oldKey)
					candidate.CacheKey = key
					candidate.RelativePath = request.RelativePath
					c.records[key] = candidate
					record, ok = candidate, true
					break
				}
			}
		}
	}
	if !ok {
		return chat.SharedThumbnailResult{}, false
	}
	info, err := os.Stat(record.ThumbnailPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		delete(c.records, key)
		c.saveLocked()
		return chat.SharedThumbnailResult{}, false
	}
	data, err := os.ReadFile(record.ThumbnailPath)
	if err != nil || len(data) == 0 {
		return chat.SharedThumbnailResult{}, false
	}
	record.LastAccessedAt = time.Now().UTC().Format(time.RFC3339Nano)
	c.records[key] = record
	c.saveLocked()
	return chat.SharedThumbnailResult{RelativePath: request.RelativePath, Status: "ready", MimeType: record.MimeType, ThumbnailMime: record.MimeType, Payload: base64.StdEncoding.EncodeToString(data)}, true
}

func (c *remoteThumbnailCache) put(deviceID string, request chat.SharedThumbnailRequest, result chat.SharedThumbnailResult) chat.SharedThumbnailResult {
	if result.Payload == "" {
		return result
	}
	data, err := base64.StdEncoding.DecodeString(result.Payload)
	if err != nil || len(data) == 0 {
		return chat.SharedThumbnailResult{RelativePath: request.RelativePath, Status: "unavailable", Error: "缩略图数据无效"}
	}
	key := remoteThumbnailKey(deviceID, request)
	path := c.cachePath(key)
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return result
	}
	part := path + ".part"
	if err := os.WriteFile(part, data, 0o600); err != nil {
		return result
	}
	if err := os.Rename(part, path); err != nil {
		_ = os.Remove(part)
		return result
	}
	mimeType := result.ThumbnailMime
	if mimeType == "" {
		mimeType = result.MimeType
	}
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	c.mu.Lock()
	c.records[key] = remoteThumbnailRecord{CacheKey: key, DeviceID: deviceID, EntryID: request.EntryID, RelativePath: request.RelativePath, FileSize: request.FileSize, ModifiedAt: request.ModifiedAt, MimeType: mimeType, ThumbnailPath: path, LastAccessedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	c.saveLocked()
	c.mu.Unlock()
	result.Status = "ready"
	result.MimeType = mimeType
	result.ThumbnailMime = mimeType
	return result
}

func (c *remoteThumbnailCache) prune() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	var total int64
	for key, record := range c.records {
		info, err := os.Stat(record.ThumbnailPath)
		accessed, parseErr := time.Parse(time.RFC3339Nano, record.LastAccessedAt)
		remove := err != nil || !info.Mode().IsRegular() || info.Size() == 0 || (parseErr == nil && now.Sub(accessed) > 30*24*time.Hour)
		if remove {
			_ = os.Remove(record.ThumbnailPath)
			delete(c.records, key)
		} else {
			total += info.Size()
		}
	}
	if entries, err := os.ReadDir(c.root); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".part") {
				if info, statErr := entry.Info(); statErr == nil && now.Sub(info.ModTime()) > 2*time.Hour {
					_ = os.Remove(filepath.Join(c.root, entry.Name()))
				}
			}
		}
	}
	if total > 512*1024*1024 {
		// The next request can regenerate an evicted item. For a small cache
		// implementation, remove oldest records until under the limit.
		for total > 512*1024*1024 {
			var oldestKey string
			var oldest time.Time
			for key, record := range c.records {
				accessed, err := time.Parse(time.RFC3339Nano, record.LastAccessedAt)
				if oldestKey == "" || err != nil || accessed.Before(oldest) {
					oldestKey, oldest = key, accessed
				}
			}
			if oldestKey == "" {
				break
			}
			if info, err := os.Stat(c.records[oldestKey].ThumbnailPath); err == nil {
				total -= info.Size()
			}
			_ = os.Remove(c.records[oldestKey].ThumbnailPath)
			delete(c.records, oldestKey)
		}
	}
	c.saveLocked()
}
