package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"flyqpro/internal/chat"
	"github.com/gogf/gf/v2/os/gctx"
)

const previewTokenLifetime = 5 * time.Minute

type previewToken struct {
	kind         string
	attachmentID string
	source       string
	deviceID     string
	relativePath string
	expiresAt    time.Time
}

// PreviewStreamService exposes short-lived, identifier-bound media URLs. The
// browser can consume these URLs as normal HTTP streams, avoiding a large
// base64 string and allowing a preview to begin as soon as the first bytes
// arrive.
type PreviewStreamService struct {
	chatService *ChatService
	mu          sync.Mutex
	tokens      map[string]previewToken
}

func NewPreviewStreamService(chatService *ChatService) *PreviewStreamService {
	return &PreviewStreamService{chatService: chatService, tokens: make(map[string]previewToken)}
}

func newPreviewToken() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *PreviewStreamService) issueToken(token previewToken) (string, error) {
	value, err := newPreviewToken()
	if err != nil {
		return "", err
	}
	token.expiresAt = time.Now().Add(previewTokenLifetime)
	s.mu.Lock()
	for key, item := range s.tokens {
		if time.Now().After(item.expiresAt) {
			delete(s.tokens, key)
		}
	}
	s.tokens[value] = token
	s.mu.Unlock()
	return (&url.URL{Path: "/preview/image", RawQuery: url.Values{"token": []string{value}}.Encode()}).String(), nil
}

func (s *PreviewStreamService) token(value string) (previewToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tokens[strings.TrimSpace(value)]
	if !ok || time.Now().After(item.expiresAt) {
		if ok {
			delete(s.tokens, strings.TrimSpace(value))
		}
		return previewToken{}, false
	}
	return item, true
}

func (s *PreviewStreamService) CreateAttachmentPreviewURL(attachmentID string) (string, error) {
	if s.chatService == nil {
		return "", fmt.Errorf("预览服务尚未初始化")
	}
	attachment, _, err := s.chatService.attachmentFile(strings.TrimSpace(attachmentID))
	if err != nil {
		return "", err
	}
	if !isImageMime(attachment.MimeType, attachment.FileName) {
		return "", fmt.Errorf("附件不是图片")
	}
	return s.issueToken(previewToken{kind: "attachment", attachmentID: attachment.AttachmentID})
}

func (s *PreviewStreamService) CreateSharedPreviewURL(source, deviceID, relativePath string) (string, error) {
	if s.chatService == nil {
		return "", fmt.Errorf("预览服务尚未初始化")
	}
	source = strings.TrimSpace(source)
	deviceID = strings.TrimSpace(deviceID)
	relativePath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if relativePath == "." || strings.Contains(relativePath, "..") {
		return "", fmt.Errorf("共享路径无效")
	}
	switch source {
	case "shared-owner":
		profile := s.chatService.engine.Profile()
		entry, _, err := chat.GetSharedEntry(profile.SharedRootPath, relativePath, false)
		if err != nil {
			return "", err
		}
		if entry.IsDirectory || !isImageMime(entry.MimeType, entry.Name) {
			return "", fmt.Errorf("该文件不是图片")
		}
	case "shared-friend":
		peers, err := chat.ListPeers(gctx.New(), chat.PeerRelation)
		if err != nil {
			return "", err
		}
		found := false
		for _, peer := range peers {
			if peer.DeviceID == deviceID {
				found = true
				if !peer.Online {
					return "", fmt.Errorf("好友不在线，暂不支持打开共享盘")
				}
				break
			}
		}
		if !found {
			return "", fmt.Errorf("不是好友，无法访问共享盘")
		}
	default:
		return "", fmt.Errorf("共享预览来源无效")
	}
	return s.issueToken(previewToken{kind: source, source: source, deviceID: deviceID, relativePath: relativePath})
}

func (s *PreviewStreamService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	item, ok := s.token(r.URL.Query().Get("token"))
	if !ok {
		http.Error(w, "preview token expired", http.StatusNotFound)
		return
	}
	if item.kind == "attachment" {
		attachment, info, err := s.chatService.attachmentFile(item.attachmentID)
		if err != nil || !isImageMime(attachment.MimeType, attachment.FileName) {
			http.Error(w, "image unavailable", http.StatusNotFound)
			return
		}
		serveLocalPreview(w, r, attachment.FileName, attachment.MimeType, attachment.LocalPath, info)
		return
	}
	if item.kind == "shared-owner" {
		profile := s.chatService.engine.Profile()
		entry, path, err := chat.GetSharedEntry(profile.SharedRootPath, item.relativePath, false)
		if err != nil || entry.IsDirectory || !isImageMime(entry.MimeType, entry.Name) {
			http.Error(w, "shared image unavailable", http.StatusNotFound)
			return
		}
		serveLocalPreview(w, r, entry.Name, entry.MimeType, path, mustStat(path))
		return
	}
	if item.kind == "shared-friend" {
		if s.chatService == nil {
			http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
			return
		}
		started := false
		err := s.chatService.engine.StreamFriendSharedEntry(r.Context(), item.deviceID, item.relativePath,
			func(entry chat.SharedEntry) error {
				if !isImageMime(entry.MimeType, entry.Name) {
					return fmt.Errorf("该文件不是图片")
				}
				w.Header().Set("Content-Type", previewMime(entry.MimeType, entry.Name))
				if entry.Size >= 0 {
					w.Header().Set("Content-Length", fmt.Sprint(entry.Size))
				}
				w.Header().Set("Cache-Control", "no-store")
				started = true
				return nil
			},
			func(data []byte) error {
				if !started {
					return fmt.Errorf("预览尚未建立")
				}
				_, err := w.Write(data)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				return err
			})
		if err != nil && !started {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
	http.Error(w, "preview unavailable", http.StatusNotFound)
}

func previewMime(mimeType, name string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/") {
		return strings.ToLower(strings.TrimSpace(mimeType))
	}
	return "image/" + strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

func serveLocalPreview(w http.ResponseWriter, r *http.Request, name, mimeType, path string, info os.FileInfo) {
	if info == nil || !info.Mode().IsRegular() {
		http.Error(w, "image unavailable", http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "image unavailable", http.StatusNotFound)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", previewMime(mimeType, name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func mustStat(path string) os.FileInfo {
	info, _ := os.Stat(path)
	return info
}
