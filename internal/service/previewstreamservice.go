package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net"
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

type friendPreviewStream func(
	context.Context,
	string,
	string,
	string,
	int64,
	func(chat.SharedEntry) error,
	func([]byte) error,
) error

type friendPreviewDetails func(string, string, string) (chat.SharedEntry, error)

// PreviewStreamService exposes short-lived, identifier-bound media URLs. The
// browser can consume these URLs as normal HTTP streams, avoiding a large
// base64 string and allowing a preview to begin as soon as the first bytes
// arrive.
type PreviewStreamService struct {
	chatService *ChatService
	mu          sync.Mutex
	tokens      map[string]previewToken
	server      *http.Server
	serverURL   string
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
	baseURL, err := s.ensureLoopbackServer()
	if err != nil {
		return "", err
	}
	return baseURL + (&url.URL{Path: "/preview/image", RawQuery: url.Values{"token": []string{value}}.Encode()}).String(), nil
}

func (s *PreviewStreamService) token(value string) (previewToken, bool) {
	key := strings.TrimSpace(value)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tokens[key]
	if !ok || now.After(item.expiresAt) {
		if ok {
			delete(s.tokens, key)
		}
		return previewToken{}, false
	}
	// A long video may need several HTTP Range requests over many minutes.
	// Renew on each active request so seeking later in the same preview does
	// not fail just because the original URL was issued five minutes ago.
	item.expiresAt = now.Add(previewTokenLifetime)
	s.tokens[key] = item
	return item, true
}

// ensureLoopbackServer is deliberately separate from Wails' asset server.
// WebView2 buffers responses produced by a custom Wails scheme until the
// handler returns, which makes a Wails service route unsuitable for a 2GB
// video. A loopback HTTP server is consumed by the browser as a normal
// streaming response on every desktop platform.
func (s *PreviewStreamService) ensureLoopbackServer() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serverURL != "" {
		return s.serverURL, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("无法启动预览流服务: %w", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(s.ServeHTTP)}
	s.server = server
	s.serverURL = "http://" + listener.Addr().String()
	go func() {
		_ = server.Serve(listener)
	}()
	return s.serverURL, nil
}

// ServiceShutdown releases the loopback listener when the desktop app exits.
func (s *PreviewStreamService) ServiceShutdown() error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.serverURL = ""
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Close()
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
	// The loopback endpoint is requested from the Wails webview origin. Media
	// playback itself does not require CORS in every engine, but exposing the
	// range headers and handling preflight keeps the endpoint consistent across
	// WebKit and WebView2.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	// Chromium may send this preflight when the Wails origin is treated as a
	// public/private-network boundary. The endpoint is loopback-only and still
	// requires a short-lived preview token.
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Length, Content-Range, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
		serveFriendPreview(w, r, item,
			func(ctx context.Context, deviceID, folderID, relativePath string, offset int64, before func(chat.SharedEntry) error, write func([]byte) error) error {
				return s.chatService.engine.StreamFriendSharedEntryRange(ctx, deviceID, folderID, relativePath, offset, before, write)
			},
			s.chatService.GetFriendSharedEntryDetails,
		)
		return
	}
	http.Error(w, "preview unavailable", http.StatusNotFound)
}

func serveFriendPreview(w http.ResponseWriter, r *http.Request, item previewToken, stream friendPreviewStream, details friendPreviewDetails) {
	rangeSpec, rangeErr := parsePreviewRange(r.Header.Get("Range"))
	if rangeErr != nil {
		w.Header().Set("Content-Range", "bytes */*")
		http.Error(w, rangeErr.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// HEAD and suffix-range requests need the total size before a body can be
	// described. They receive metadata only; a HEAD must never start reading the
	// video itself.
	var metadata chat.SharedEntry
	needsMetadata := r.Method == http.MethodHead || (rangeSpec != nil && !rangeSpec.hasStart)
	if needsMetadata {
		if details == nil {
			http.Error(w, "preview metadata unavailable", http.StatusBadGateway)
			return
		}
		var err error
		metadata, err = details(item.deviceID, item.sharedFolderID, item.relativePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if metadata.IsDirectory || (!isImageMime(metadata.MimeType, metadata.Name) && !isVideoMime(metadata.MimeType, metadata.Name)) {
			http.Error(w, "该文件不支持媒体预览", http.StatusUnsupportedMediaType)
			return
		}
	}

	if r.Method == http.MethodHead {
		if _, err := writeFriendPreviewHeaders(w, metadata, rangeSpec); err != nil {
			writePreviewRangeError(w, err)
		}
		return
	}

	streamOffset := int64(0)
	if rangeSpec != nil {
		if rangeSpec.hasStart {
			streamOffset = rangeSpec.start
		} else {
			start, _, ok := resolvePreviewRange(rangeSpec, metadata.Size)
			if !ok {
				writePreviewRangeError(w, &previewRangeError{contentRange: fmt.Sprintf("bytes */%d", metadata.Size)})
				return
			}
			streamOffset = start
		}
	}

	started := false
	remaining := int64(-1)
	var headerErr error
	err := stream(r.Context(), item.deviceID, item.sharedFolderID, item.relativePath, streamOffset,
		func(entry chat.SharedEntry) error {
			if entry.IsDirectory || (!isImageMime(entry.MimeType, entry.Name) && !isVideoMime(entry.MimeType, entry.Name)) {
				return fmt.Errorf("该文件不支持媒体预览")
			}
			remaining, headerErr = writeFriendPreviewHeaders(w, entry, rangeSpec)
			if headerErr != nil {
				return headerErr
			}
			started = true
			flushPreview(w)
			return nil
		},
		func(data []byte) error {
			if !started {
				return fmt.Errorf("预览尚未建立")
			}
			if rangeSpec != nil {
				if remaining <= 0 {
					return io.EOF
				}
				if int64(len(data)) > remaining {
					data = data[:remaining]
				}
			}
			n, writeErr := w.Write(data)
			if rangeSpec != nil {
				remaining -= int64(n)
			}
			flushPreview(w)
			if writeErr == nil && rangeSpec != nil && remaining == 0 {
				return io.EOF
			}
			return writeErr
		})
	if headerErr != nil {
		writePreviewRangeError(w, headerErr)
		return
	}
	if err != nil && !started {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

// writeFriendPreviewHeaders returns the number of bytes still expected for a
// response. Headers are sent before the first body write, so the browser can
// begin buffering as soon as the first remote chunk arrives.
func writeFriendPreviewHeaders(w http.ResponseWriter, entry chat.SharedEntry, spec *previewRangeSpec) (int64, error) {
	if entry.Size < 0 {
		return 0, fmt.Errorf("共享文件大小无效")
	}
	start, end, ok := resolvePreviewRange(spec, entry.Size)
	if !ok {
		return 0, &previewRangeError{contentRange: fmt.Sprintf("bytes */%d", entry.Size)}
	}
	w.Header().Set("Content-Type", previewMime(entry.MimeType, entry.Name))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	if spec == nil {
		w.Header().Set("Content-Length", fmt.Sprint(entry.Size))
		w.WriteHeader(http.StatusOK)
		return entry.Size, nil
	}
	remaining := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprint(remaining))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, entry.Size))
	w.WriteHeader(http.StatusPartialContent)
	return remaining, nil
}

func writePreviewRangeError(w http.ResponseWriter, err error) {
	if rangeErr, ok := err.(*previewRangeError); ok {
		w.Header().Set("Content-Range", rangeErr.contentRange)
		http.Error(w, rangeErr.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

func flushPreview(w http.ResponseWriter) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
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
