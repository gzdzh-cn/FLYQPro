package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"flyqpro/internal/chat"
	"github.com/gogf/gf/v2/os/gctx"
)

const previewTokenLifetime = 5 * time.Minute

type previewRangeSpec struct {
	start    int64
	end      int64
	suffix   int64
	hasStart bool
	hasEnd   bool
}

type previewRangeError struct {
	contentRange string
}

func (e *previewRangeError) Error() string { return "请求的视频范围无效" }

func parsePreviewRange(value string) (*previewRangeSpec, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(strings.ToLower(value), "bytes=") {
		return nil, fmt.Errorf("不支持的预览范围")
	}
	value = strings.TrimSpace(value[len("bytes="):])
	if value == "" || strings.Contains(value, ",") {
		return nil, fmt.Errorf("不支持多个预览范围")
	}
	hyphen := strings.IndexByte(value, '-')
	if hyphen < 0 {
		return nil, fmt.Errorf("预览范围格式无效")
	}
	left, right := strings.TrimSpace(value[:hyphen]), strings.TrimSpace(value[hyphen+1:])
	if left == "" && right == "" {
		return nil, fmt.Errorf("预览范围格式无效")
	}
	spec := &previewRangeSpec{start: -1, end: -1, suffix: -1}
	if left == "" {
		suffix, err := strconv.ParseInt(right, 10, 64)
		if err != nil || suffix <= 0 {
			return nil, fmt.Errorf("预览范围格式无效")
		}
		spec.suffix = suffix
		return spec, nil
	}
	start, err := strconv.ParseInt(left, 10, 64)
	if err != nil || start < 0 {
		return nil, fmt.Errorf("预览范围格式无效")
	}
	spec.start = start
	spec.hasStart = true
	if right != "" {
		end, err := strconv.ParseInt(right, 10, 64)
		if err != nil || end < start {
			return nil, fmt.Errorf("预览范围格式无效")
		}
		spec.end = end
		spec.hasEnd = true
	}
	return spec, nil
}

func resolvePreviewRange(spec *previewRangeSpec, size int64) (start, end int64, ok bool) {
	if spec == nil || size <= 0 {
		return 0, 0, spec == nil && size >= 0
	}
	if spec.suffix > 0 {
		length := spec.suffix
		if length > size {
			length = size
		}
		return size - length, size - 1, true
	}
	if !spec.hasStart || spec.start >= size {
		return 0, 0, false
	}
	end = size - 1
	if spec.hasEnd && spec.end < end {
		end = spec.end
	}
	return spec.start, end, end >= spec.start
}

type previewToken struct {
	kind           string
	attachmentID   string
	source         string
	deviceID       string
	sharedFolderID string
	relativePath   string
	expiresAt      time.Time
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

func (s *PreviewStreamService) CreateSharedPreviewURL(source, deviceID, sharedFolderID, relativePath string) (string, error) {
	if s.chatService == nil {
		return "", fmt.Errorf("预览服务尚未初始化")
	}
	source = strings.TrimSpace(source)
	deviceID = strings.TrimSpace(deviceID)
	sharedFolderID = strings.TrimSpace(sharedFolderID)
	relativePath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if relativePath == "." || strings.Contains(relativePath, "..") {
		return "", fmt.Errorf("共享路径无效")
	}
	switch source {
	case "shared-owner":
		folder, err := localSharedFolder(sharedFolderID)
		if err != nil {
			return "", err
		}
		entry, _, err := chat.GetSharedEntry(folder.RootPath, relativePath, false)
		if err != nil {
			return "", err
		}
		if entry.IsDirectory || (!isImageMime(entry.MimeType, entry.Name) && !isVideoMime(entry.MimeType, entry.Name)) {
			return "", fmt.Errorf("该文件不支持媒体预览")
		}
	case "shared-friend":
		if sharedFolderID == "" {
			return "", fmt.Errorf("共享文件夹 ID 不能为空")
		}
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
	return s.issueToken(previewToken{kind: source, source: source, deviceID: deviceID, sharedFolderID: sharedFolderID, relativePath: relativePath})
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
		folder, err := localSharedFolder(item.sharedFolderID)
		if err != nil {
			http.Error(w, "shared media unavailable", http.StatusNotFound)
			return
		}
		entry, path, err := chat.GetSharedEntry(folder.RootPath, item.relativePath, false)
		if err != nil || entry.IsDirectory || (!isImageMime(entry.MimeType, entry.Name) && !isVideoMime(entry.MimeType, entry.Name)) {
			http.Error(w, "shared media unavailable", http.StatusNotFound)
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
		rangeSpec, rangeErr := parsePreviewRange(r.Header.Get("Range"))
		if rangeErr != nil {
			w.Header().Set("Content-Range", "bytes */*")
			http.Error(w, rangeErr.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		streamOffset := int64(0)
		if rangeSpec != nil && rangeSpec.hasStart {
			streamOffset = rangeSpec.start
		}
		started := false
		var invalidRange *previewRangeError
		skipBytes := int64(0)
		remaining := int64(-1)
		err := s.chatService.engine.StreamFriendSharedEntryRange(r.Context(), item.deviceID, item.sharedFolderID, item.relativePath, streamOffset,
			func(entry chat.SharedEntry) error {
				if !isImageMime(entry.MimeType, entry.Name) && !isVideoMime(entry.MimeType, entry.Name) {
					return fmt.Errorf("该文件不支持媒体预览")
				}
				start, end, ok := resolvePreviewRange(rangeSpec, entry.Size)
				if !ok {
					invalidRange = &previewRangeError{contentRange: fmt.Sprintf("bytes */%d", entry.Size)}
					return invalidRange
				}
				w.Header().Set("Content-Type", previewMime(entry.MimeType, entry.Name))
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Cache-Control", "no-store")
				if rangeSpec == nil {
					w.Header().Set("Content-Length", fmt.Sprint(entry.Size))
				} else {
					remaining = end - start + 1
					w.Header().Set("Content-Length", fmt.Sprint(remaining))
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, entry.Size))
					w.WriteHeader(http.StatusPartialContent)
					// A suffix range is resolved after the peer tells us the file
					// size, so discard the leading bytes from the offset-zero stream.
					if !rangeSpec.hasStart {
						skipBytes = start
					}
				}
				started = true
				return nil
			},
			func(data []byte) error {
				if !started {
					return fmt.Errorf("预览尚未建立")
				}
				if skipBytes > 0 {
					if int64(len(data)) <= skipBytes {
						skipBytes -= int64(len(data))
						return nil
					}
					data = data[skipBytes:]
					skipBytes = 0
				}
				if rangeSpec != nil {
					if remaining <= 0 {
						return io.EOF
					}
					if int64(len(data)) > remaining {
						data = data[:remaining]
					}
				}
				n, err := w.Write(data)
				if rangeSpec != nil {
					remaining -= int64(n)
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				if err == nil && rangeSpec != nil && remaining == 0 {
					return io.EOF
				}
				return err
			})
		if invalidRange != nil {
			w.Header().Set("Content-Range", invalidRange.contentRange)
			http.Error(w, invalidRange.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if err != nil && !started {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
	http.Error(w, "preview unavailable", http.StatusNotFound)
}

func previewMime(mimeType, name string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "video/") {
		return mimeType
	}
	if guessed := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); guessed != "" {
		return guessed
	}
	return "application/octet-stream"
}

func serveLocalPreview(w http.ResponseWriter, r *http.Request, name, mimeType, path string, info os.FileInfo) {
	if info == nil || !info.Mode().IsRegular() {
		http.Error(w, "media unavailable", http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "media unavailable", http.StatusNotFound)
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
