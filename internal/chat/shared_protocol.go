package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const sharedDriveCapability = "shared-drive-v2"
const sharedThumbnailBatchCapability = "shared-thumbnail-batch-v1"
const maxSharedPreviewSize = 32 * 1024 * 1024

type sharedTransferSession struct {
	mu         sync.Mutex
	conn       net.Conn
	cancel     chan struct{}
	cancelOnce sync.Once
	stop       context.CancelFunc
	running    bool
	paused     bool
	tempPath   string
	finalPath  string
	transfer   SharedTransfer
	peer       Peer
	target     string
}

func (e *Engine) sharedAccessAllowed(deviceID string) bool {
	if !e.isFriend(deviceID) {
		return false
	}
	profile := e.Profile()
	if !profile.SharedEnabled {
		return false
	}
	folders, err := ListSharedFolders(context.Background())
	return err == nil && len(folders) > 0
}

func (e *Engine) sharedError(deviceID string) string {
	if !e.isFriend(deviceID) {
		return "FRIENDSHIP_REQUIRED"
	}
	if !e.Profile().SharedEnabled {
		return SharedDisabledError
	}
	return SharedUnavailableError
}

const sharedDriveUnsupportedError = "对方客户端不支持多共享文件夹"

func writeSharedDriveUnsupportedError(conn net.Conn, session *wireSession, transferID string) {
	_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: transferID, Status: sharedDriveUnsupportedError})
}

func (e *Engine) sharedFolderForRequest(id string) (SharedFolder, string, error) {
	folder, err := GetSharedFolder(context.Background(), id)
	if err != nil {
		return SharedFolder{}, "", fmt.Errorf("%s", SharedPathInvalidError)
	}
	root, err := sharedRootPath(folder.RootPath)
	if err != nil {
		return SharedFolder{}, "", err
	}
	folder.RootPath = root
	return folder, root, nil
}

func (e *Engine) handleSharedFoldersRequest(conn net.Conn, hello wireMessage) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: e.sharedError(hello.DeviceID)})
		return
	}
	if !hasCapability(hello.Capabilities, sharedDriveCapability) {
		writeSharedDriveUnsupportedError(conn, nil, "")
		return
	}
	e.mu.RLock()
	provider := e.sharedFoldersProvider
	e.mu.RUnlock()
	var folders []SharedFolder
	var err error
	if provider != nil {
		folders, err = provider()
	} else {
		folders, err = ListSharedFolders(context.Background())
	}
	if err != nil {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: SharedUnavailableError})
		return
	}
	for index := range folders {
		folders[index].RootPath = ""
	}
	_ = writeWire(conn, wireMessage{Type: "share_folders_response", Status: "ok", SharedFolders: folders})
}

func (e *Engine) handleSharedListRequest(conn net.Conn, hello, message wireMessage) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: e.sharedError(hello.DeviceID)})
		return
	}
	if !hasCapability(hello.Capabilities, sharedDriveCapability) {
		writeSharedDriveUnsupportedError(conn, nil, "")
		return
	}
	profile := e.Profile()
	_, root, folderErr := e.sharedFolderForRequest(message.SharedFolderID)
	if folderErr != nil {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: SharedPathInvalidError})
		return
	}
	page, err := ListSharedEntriesPage(root, message.RelativePath, message.ListOffset, message.ListLimit, profile.ShowHiddenFiles)
	if err != nil {
		status := SharedPathInvalidError
		if strings.Contains(err.Error(), SharedUnavailableError) {
			status = SharedUnavailableError
		}
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: status})
		return
	}
	_ = writeWire(conn, wireMessage{Type: "share_list_response", Status: "ok", SharedFolderID: message.SharedFolderID, RelativePath: message.RelativePath, Entries: page.Entries, NextOffset: page.NextOffset, HasMore: page.HasMore})
}

func (e *Engine) handleSharedThumbnailRequest(conn net.Conn, hello, message wireMessage) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: e.sharedError(hello.DeviceID)})
		return
	}
	if !hasCapability(hello.Capabilities, sharedDriveCapability) {
		writeSharedDriveUnsupportedError(conn, nil, "")
		return
	}
	_, root, folderErr := e.sharedFolderForRequest(message.SharedFolderID)
	if folderErr != nil {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: SharedPathInvalidError})
		return
	}
	e.mu.RLock()
	provider := e.sharedThumbnailProvider
	e.mu.RUnlock()
	var data, mimeType string
	var err error
	if provider != nil {
		data, mimeType, err = provider(root, message.RelativePath)
	} else {
		data, mimeType, err = GetSharedEntryThumbnail(root, message.RelativePath)
	}
	if err != nil {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: "SHARED_THUMBNAIL_UNAVAILABLE"})
		return
	}
	status := "ok"
	if data == "" {
		status = "pending"
	}
	_ = writeWire(conn, wireMessage{Type: "share_thumbnail_response", Status: status, SharedFolderID: message.SharedFolderID, RelativePath: message.RelativePath, MimeType: mimeType, Payload: data})
}

func (e *Engine) handleSharedThumbnailBatchRequest(conn net.Conn, hello, message wireMessage) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: e.sharedError(hello.DeviceID)})
		return
	}
	if !hasCapability(hello.Capabilities, sharedDriveCapability) {
		writeSharedDriveUnsupportedError(conn, nil, "")
		return
	}
	requests := message.ThumbnailRequests
	if len(requests) == 0 {
		_ = writeWire(conn, wireMessage{Type: "share_thumbnail_batch_response", Status: "ok", ThumbnailResults: []SharedThumbnailResult{}})
		return
	}
	if len(requests) > 24 {
		requests = requests[:24]
	}
	e.mu.RLock()
	provider := e.sharedThumbnailProvider
	e.mu.RUnlock()
	results := make([]SharedThumbnailResult, 0, len(requests))
	for _, request := range requests {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(request.RelativePath)))
		result := SharedThumbnailResult{SharedFolderID: request.SharedFolderID, RelativePath: clean, Status: "unavailable"}
		if clean == "." || strings.Contains(clean, "..") {
			result.Error = SharedPathInvalidError
			results = append(results, result)
			continue
		}
		_, requestRoot, requestErr := e.sharedFolderForRequest(request.SharedFolderID)
		if requestErr != nil {
			result.Error = SharedPathInvalidError
			results = append(results, result)
			continue
		}
		var data, mimeType string
		var err error
		if provider != nil {
			data, mimeType, err = provider(requestRoot, clean)
		} else {
			data, mimeType, err = GetSharedEntryThumbnail(requestRoot, clean)
		}
		if err != nil {
			result.Error = "SHARED_THUMBNAIL_UNAVAILABLE"
		} else if data == "" {
			result.Status = "pending"
		} else {
			result.Status = "ready"
			result.MimeType = mimeType
			result.ThumbnailMime = mimeType
			result.Payload = data
		}
		results = append(results, result)
	}
	_ = writeWire(conn, wireMessage{Type: "share_thumbnail_batch_response", Status: "ok", ThumbnailResults: results})
}

func (e *Engine) handleSharedDownloadRequest(conn net.Conn, hello, message wireMessage, session *wireSession) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: e.sharedError(hello.DeviceID)})
		return
	}
	if !hasCapability(hello.Capabilities, sharedDriveCapability) {
		writeSharedDriveUnsupportedError(conn, session, message.TransferID)
		return
	}
	_, root, folderErr := e.sharedFolderForRequest(message.SharedFolderID)
	if folderErr != nil {
		_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: SharedPathInvalidError})
		return
	}
	entry, path, err := GetSharedEntry(root, message.RelativePath, true)
	if err != nil || entry.IsDirectory {
		status := SharedPathInvalidError
		if err != nil && strings.Contains(err.Error(), SharedUnavailableError) {
			status = SharedUnavailableError
		}
		_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: status})
		return
	}
	file, err := os.Open(path)
	if err != nil {
		_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: SharedUnavailableError})
		return
	}
	defer file.Close()
	if message.Offset < 0 || message.Offset > entry.Size {
		_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: "SHARED_OFFSET_INVALID"})
		return
	}
	if message.Offset > 0 {
		if _, err := file.Seek(message.Offset, io.SeekStart); err != nil {
			_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: SharedUnavailableError})
			return
		}
	}
	transferID := strings.TrimSpace(message.TransferID)
	if transferID == "" {
		transferID = newID()
	}
	if err := sessionWrite(session, conn, wireMessage{Type: "share_download_response", TransferID: transferID, Status: "accepted", SharedFolderID: message.SharedFolderID, RelativePath: entry.RelativePath, FileName: entry.Name, FileSize: entry.Size, MimeType: entry.MimeType, SHA256: entry.SHA256, Offset: message.Offset}); err != nil {
		return
	}
	buffer := make([]byte, 256*1024)
	transferred := message.Offset
	for {
		if !e.sharedAccessAllowed(hello.DeviceID) {
			_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: transferID, Status: SharedDisabledError})
			return
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			payload := base64.StdEncoding.EncodeToString(buffer[:read])
			transferred += int64(read)
			if err := sessionWrite(session, conn, wireMessage{Type: "share_chunk", TransferID: transferID, Payload: payload, Transferred: transferred, FileSize: entry.Size}); err != nil {
				return
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				_ = sessionWrite(session, conn, wireMessage{Type: "share_complete", TransferID: transferID, Status: "completed", Transferred: transferred, FileSize: entry.Size, SHA256: entry.SHA256})
			}
			return
		}
	}
}

func (e *Engine) dialSharedPeer(peer Peer) (net.Conn, *json.Decoder, ProtocolDialect, error) {
	return e.dialSharedPeerContext(context.Background(), peer)
}

func (e *Engine) dialSharedPeerContext(ctx context.Context, peer Peer) (net.Conn, *json.Decoder, ProtocolDialect, error) {
	if peer.IP == "" || peer.Port == 0 {
		return nil, nil, ProtocolDialect{}, fmt.Errorf("好友地址不可用")
	}
	if len(peer.Capabilities) > 0 && !hasCapability(peer.Capabilities, sharedDriveCapability) {
		return nil, nil, ProtocolDialect{}, fmt.Errorf("对方客户端不支持多共享文件夹")
	}
	clientTLS, err := e.clientTLSConfig()
	if err != nil {
		return nil, nil, ProtocolDialect{}, err
	}
	dialects := protocolDialectsForPeer(peer)
	if len(dialects) == 0 {
		dialects = protocolDialects
	}
	var lastErr error
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: clientTLS}
	for _, dialect := range dialects {
		rawConn, dialErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)))
		if dialErr != nil {
			lastErr = dialErr
			continue
		}
		conn, ok := rawConn.(*tls.Conn)
		if !ok {
			_ = rawConn.Close()
			lastErr = fmt.Errorf("好友连接类型无效")
			continue
		}
		if err := verifyPeerCertificate(conn, peer); err != nil {
			_ = conn.Close()
			lastErr = err
			continue
		}
		if err := writeWire(conn, e.helloMessageForDialect("hello", dialect)); err != nil {
			_ = conn.Close()
			lastErr = err
			continue
		}
		decoder := json.NewDecoder(conn)
		// A remote SAF provider can be slow or disappear while resolving the
		// shared tree. Never leave a shared-drive page spinning forever while
		// waiting for the hello response.
		_ = conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		var response wireMessage
		if err := decoder.Decode(&response); err != nil {
			_ = conn.Close()
			lastErr = err
			continue
		}
		if response.Type == "error" {
			_ = conn.Close()
			lastErr = fmt.Errorf("%s", response.Status)
			continue
		}
		if response.Type != "hello_ack" {
			_ = conn.Close()
			lastErr = fmt.Errorf("对方握手失败")
			continue
		}
		if response.FriendshipState == "removed" {
			e.handleRemoteFriendshipRequired(peer.DeviceID)
			_ = conn.Close()
			lastErr = fmt.Errorf("FRIENDSHIP_REQUIRED")
			continue
		}
		_ = conn.SetReadDeadline(time.Time{})
		responseDialect, compatible := protocolDialectForMessage(response)
		if !compatible || !hasCapability(response.Capabilities, sharedDriveCapability) {
			_ = conn.Close()
			lastErr = fmt.Errorf("对方客户端不支持多共享文件夹")
			continue
		}
		return conn, decoder, responseDialect, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无法连接好友")
	}
	return nil, nil, ProtocolDialect{}, lastErr
}

func (e *Engine) ListFriendSharedEntries(ctx context.Context, deviceID, folderID, relativePath string, showHiddenFiles ...bool) ([]SharedEntry, error) {
	page, err := e.ListFriendSharedEntriesPage(ctx, deviceID, folderID, relativePath, 0, defaultSharedEntriesPageSize, showHiddenFiles...)
	return page.Entries, err
}

func (e *Engine) ListFriendSharedFolders(ctx context.Context, deviceID string) ([]SharedFolder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return nil, fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	conn, decoder, _, err := e.dialSharedPeer(peer)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := writeWire(conn, wireMessage{Type: "share_folders_request"}); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	if response.Type == "share_error" {
		return nil, fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_folders_response" || response.Status != "ok" {
		return nil, fmt.Errorf("共享文件夹响应无效")
	}
	for index := range response.SharedFolders {
		response.SharedFolders[index].RootPath = ""
	}
	return response.SharedFolders, nil
}

func (e *Engine) ListFriendSharedEntriesPage(ctx context.Context, deviceID, folderID, relativePath string, offset, limit int, showHiddenFiles ...bool) (SharedEntriesPage, error) {
	if err := ctx.Err(); err != nil {
		return SharedEntriesPage{}, err
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return SharedEntriesPage{}, fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	conn, decoder, _, err := e.dialSharedPeer(peer)
	if err != nil {
		return SharedEntriesPage{}, err
	}
	defer conn.Close()
	showHidden := len(showHiddenFiles) > 0 && showHiddenFiles[0]
	if strings.TrimSpace(folderID) == "" {
		return SharedEntriesPage{}, fmt.Errorf("共享文件夹 ID 不能为空")
	}
	if err := writeWire(conn, wireMessage{Type: "share_list_request", SharedFolderID: folderID, RelativePath: relativePath, ListOffset: offset, ListLimit: limit, ShowHiddenFiles: showHidden}); err != nil {
		return SharedEntriesPage{}, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return SharedEntriesPage{}, err
	}
	_ = conn.SetReadDeadline(time.Time{})
	if response.Type == "share_error" {
		return SharedEntriesPage{}, fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_list_response" || response.Status != "ok" {
		return SharedEntriesPage{}, fmt.Errorf("共享目录响应无效")
	}
	if response.SharedFolderID != folderID {
		return SharedEntriesPage{}, fmt.Errorf("共享文件夹响应无效")
	}
	return SharedEntriesPage{Entries: response.Entries, NextOffset: response.NextOffset, HasMore: response.HasMore}, nil
}

func sharedPreviewableEntry(entry SharedEntry) bool {
	mimeType := strings.ToLower(strings.TrimSpace(entry.MimeType))
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "video/") || mimeType == "application/pdf" {
		return true
	}
	switch strings.ToLower(filepath.Ext(entry.Name)) {
	case ".3gp", ".avif", ".avi", ".bmp", ".flv", ".gif", ".heic", ".heif", ".jpeg", ".jpg", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".ogv", ".png", ".ts", ".webm", ".webp", ".wmv", ".pdf":
		return true
	default:
		return false
	}
}

func (e *Engine) findFriendSharedEntry(ctx context.Context, deviceID, folderID, relativePath string) (SharedEntry, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." || strings.Contains(clean, "..") {
		return SharedEntry{}, fmt.Errorf("SHARED_PATH_INVALID")
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(clean)))
	if parent == "." {
		parent = ""
	}
	for offset := 0; ; {
		page, err := e.ListFriendSharedEntriesPage(ctx, deviceID, folderID, parent, offset, maxSharedEntriesPageSize)
		if err != nil {
			return SharedEntry{}, err
		}
		for _, entry := range page.Entries {
			if entry.RelativePath == clean {
				return entry, nil
			}
		}
		if !page.HasMore || page.NextOffset <= offset {
			break
		}
		offset = page.NextOffset
	}
	return SharedEntry{}, fmt.Errorf("共享文件不存在")
}

// StreamFriendSharedEntry streams a validated remote shared file to the
// caller. It deliberately exposes bytes through a callback instead of
// assembling the complete file in memory, which lets the desktop preview
// window start rendering while a large image is still arriving.
func (e *Engine) StreamFriendSharedEntry(ctx context.Context, deviceID, folderID, relativePath string, before func(SharedEntry) error, write func([]byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." || strings.Contains(clean, "..") {
		return fmt.Errorf("SHARED_PATH_INVALID")
	}
	entry, err := e.findFriendSharedEntry(ctx, deviceID, folderID, clean)
	if err != nil {
		return err
	}
	if entry.IsDirectory || !sharedPreviewableEntry(entry) {
		return fmt.Errorf("该文件类型不支持在线预览")
	}
	if entry.Size < 0 || entry.Size > maxSharedPreviewSize {
		return fmt.Errorf("在线预览文件不能超过 %d MB，请先下载", maxSharedPreviewSize/(1024*1024))
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	conn, decoder, _, err := e.dialSharedPeerContext(ctx, peer)
	if err != nil {
		return err
	}
	defer conn.Close()
	transferID := newID()
	if err := writeWire(conn, wireMessage{Type: "share_download_request", TransferID: transferID, SharedFolderID: folderID, RelativePath: clean, Offset: 0}); err != nil {
		return err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return err
	}
	if response.Type == "share_error" {
		return fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_download_response" || response.Status != "accepted" || response.SharedFolderID != folderID || response.Offset != 0 {
		return fmt.Errorf("共享文件预览响应无效")
	}
	if response.FileSize < 0 || response.FileSize > maxSharedPreviewSize {
		return fmt.Errorf("在线预览文件过大，请先下载")
	}
	entry.Name = response.FileName
	entry.Size = response.FileSize
	if response.MimeType != "" {
		entry.MimeType = response.MimeType
	}
	if before != nil {
		if err := before(entry); err != nil {
			return err
		}
	}
	hash := sha256.New()
	var received int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var message wireMessage
		if err := decoder.Decode(&message); err != nil {
			return err
		}
		switch message.Type {
		case "share_chunk":
			if message.TransferID != transferID {
				return fmt.Errorf("共享传输标识无效")
			}
			payload, decodeErr := base64.StdEncoding.DecodeString(message.Payload)
			if decodeErr != nil || received+int64(len(payload)) > response.FileSize {
				return fmt.Errorf("共享文件数据无效")
			}
			if _, err := hash.Write(payload); err != nil {
				return err
			}
			if write != nil {
				if err := write(payload); err != nil {
					return err
				}
			}
			received += int64(len(payload))
		case "share_error":
			return fmt.Errorf("%s", message.Status)
		case "share_complete":
			if received != response.FileSize || (message.SHA256 != "" && hex.EncodeToString(hash.Sum(nil)) != message.SHA256) {
				return fmt.Errorf("共享文件校验失败")
			}
			return nil
		}
	}
}

func (e *Engine) GetFriendSharedEntryPreview(ctx context.Context, deviceID, folderID, relativePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." || strings.Contains(clean, "..") {
		return "", fmt.Errorf("SHARED_PATH_INVALID")
	}
	entry, err := e.findFriendSharedEntry(ctx, deviceID, folderID, clean)
	if err != nil {
		return "", err
	}
	if entry.RelativePath == "" || entry.IsDirectory {
		return "", fmt.Errorf("共享文件不存在")
	}
	if !sharedPreviewableEntry(entry) {
		return "", fmt.Errorf("该文件类型不支持在线预览")
	}
	if entry.Size < 0 || entry.Size > maxSharedPreviewSize {
		return "", fmt.Errorf("在线预览文件不能超过 %d MB，请先下载", maxSharedPreviewSize/(1024*1024))
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return "", fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	conn, decoder, _, err := e.dialSharedPeerContext(ctx, peer)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	transferID := newID()
	if err := writeWire(conn, wireMessage{Type: "share_download_request", TransferID: transferID, SharedFolderID: folderID, RelativePath: clean, Offset: 0}); err != nil {
		return "", err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return "", err
	}
	if response.Type == "share_error" {
		return "", fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_download_response" || response.Status != "accepted" || response.SharedFolderID != folderID || response.Offset != 0 {
		return "", fmt.Errorf("共享文件预览响应无效")
	}
	if response.FileSize < 0 || response.FileSize > maxSharedPreviewSize {
		return "", fmt.Errorf("在线预览文件过大，请先下载")
	}
	var data bytes.Buffer
	hash := sha256.New()
	for {
		var message wireMessage
		if err := decoder.Decode(&message); err != nil {
			return "", err
		}
		switch message.Type {
		case "share_chunk":
			if message.TransferID != transferID {
				return "", fmt.Errorf("共享传输标识无效")
			}
			payload, decodeErr := base64.StdEncoding.DecodeString(message.Payload)
			if decodeErr != nil || int64(data.Len()+len(payload)) > response.FileSize {
				return "", fmt.Errorf("共享文件数据无效")
			}
			if _, err := data.Write(payload); err != nil {
				return "", err
			}
			_, _ = hash.Write(payload)
		case "share_error":
			return "", fmt.Errorf("%s", message.Status)
		case "share_complete":
			if int64(data.Len()) != response.FileSize || (message.SHA256 != "" && hex.EncodeToString(hash.Sum(nil)) != message.SHA256) {
				return "", fmt.Errorf("共享文件校验失败")
			}
			mimeType := response.MimeType
			if mimeType == "" {
				mimeType = entry.MimeType
			}
			return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data.Bytes()), nil
		}
	}
}

// GetFriendSharedEntryThumbnail fetches only the bounded preview generated by
// the remote peer. It is used by the shared-drive thumbnail grid and avoids
// transferring or decoding the original file in the webview.
func (e *Engine) GetFriendSharedEntryThumbnail(ctx context.Context, deviceID, folderID, relativePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(folderID) == "" {
		return "", fmt.Errorf("共享文件夹 ID 不能为空")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." || strings.Contains(clean, "..") {
		return "", fmt.Errorf("SHARED_PATH_INVALID")
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return "", fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	conn, decoder, _, err := e.dialSharedPeerContext(ctx, peer)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := writeWire(conn, wireMessage{Type: "share_thumbnail_request", SharedFolderID: folderID, RelativePath: clean}); err != nil {
		return "", err
	}
	// A peer that predates the thumbnail frame may keep the authenticated
	// connection open while ignoring it. Do not leave a card in "加载中…"
	// forever; the caller can then show a placeholder or use its fallback.
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return "", err
	}
	if response.Type == "share_error" {
		return "", fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_thumbnail_response" || (response.Status != "ok" && response.Status != "pending") {
		return "", fmt.Errorf("共享图片缩略图响应无效")
	}
	if response.SharedFolderID != folderID {
		return "", fmt.Errorf("共享文件夹响应无效")
	}
	if response.Status == "pending" || response.Payload == "" {
		return "", nil
	}
	mimeType := response.MimeType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return "data:" + mimeType + ";base64," + response.Payload, nil
}

// GetFriendSharedEntryThumbnails fetches a group of bounded previews over one
// authenticated connection. A pending result is intentionally non-blocking:
// the remote peer generates it in the background and the caller can retry.
func (e *Engine) GetFriendSharedEntryThumbnails(ctx context.Context, deviceID string, requests []SharedThumbnailRequest) ([]SharedThumbnailResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return []SharedThumbnailResult{}, nil
	}
	if len(requests) > 24 {
		requests = requests[:24]
	}
	for _, request := range requests {
		if strings.TrimSpace(request.SharedFolderID) == "" {
			return nil, fmt.Errorf("共享文件夹 ID 不能为空")
		}
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return nil, fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	if len(peer.Capabilities) == 0 || !hasCapability(peer.Capabilities, sharedThumbnailBatchCapability) {
		return nil, fmt.Errorf("SHARED_THUMBNAIL_BATCH_UNSUPPORTED")
	}
	conn, decoder, _, err := e.dialSharedPeerContext(ctx, peer)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	for _, request := range requests {
		if strings.TrimSpace(request.SharedFolderID) == "" {
			return nil, fmt.Errorf("共享文件夹 ID 不能为空")
		}
	}
	if err := writeWire(conn, wireMessage{Type: "share_thumbnail_batch_request", ThumbnailRequests: requests}); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	if response.Type == "share_error" {
		return nil, fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_thumbnail_batch_response" {
		return nil, fmt.Errorf("共享图片缩略图批量响应无效")
	}
	if len(response.ThumbnailResults) != len(requests) {
		return nil, fmt.Errorf("共享图片缩略图批量响应数量无效")
	}
	for index, result := range response.ThumbnailResults {
		if result.SharedFolderID != requests[index].SharedFolderID {
			return nil, fmt.Errorf("共享文件夹响应无效")
		}
	}
	return response.ThumbnailResults, nil
}

func (e *Engine) DownloadFriendSharedEntry(ctx context.Context, deviceID, folderID, relativePath, targetPath string) (SharedTransfer, error) {
	transfer := SharedTransfer{TransferID: newID(), DeviceID: deviceID, SharedFolderID: folderID, RelativePath: relativePath, Direction: "receive", Status: "starting", TargetPath: targetPath, FileName: filepath.Base(filepath.FromSlash(relativePath))}
	if err := ctx.Err(); err != nil {
		return transfer, err
	}
	if strings.TrimSpace(folderID) == "" {
		return transfer, fmt.Errorf("共享文件夹 ID 不能为空")
	}
	if strings.TrimSpace(targetPath) == "" {
		return transfer, fmt.Errorf("下载目标不能为空")
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation != PeerRelation {
		return transfer, fmt.Errorf("FRIENDSHIP_REQUIRED")
	}
	cancel := make(chan struct{})
	downloadCtx, stop := context.WithCancel(context.Background())
	session := &sharedTransferSession{cancel: cancel, stop: stop, running: true, peer: peer, target: targetPath, transfer: transfer}
	e.mu.Lock()
	e.sharedTransfers[transfer.TransferID] = session
	e.mu.Unlock()
	e.emitSharedProgress(transfer)
	go e.runSharedTransfer(downloadCtx, peer, transfer, targetPath, session)
	return transfer, nil
}

func (e *Engine) runSharedTransfer(ctx context.Context, peer Peer, transfer SharedTransfer, targetPath string, session *sharedTransferSession) {
	result, downloadErr := e.downloadFriendSharedEntry(ctx, peer, transfer, targetPath, session)
	stopStatus := e.sharedTransferStopStatus(session)
	if stopStatus == "canceled" {
		result.Status = "canceled"
		downloadErr = nil
	} else if stopStatus == "paused" {
		result.Status = "paused"
		downloadErr = nil
	} else if downloadErr != nil {
		result.Status = "failed"
		result.ErrorMessage = downloadErr.Error()
	}
	session.mu.Lock()
	session.running = false
	stop := session.stop
	session.stop = nil
	session.transfer = result
	session.mu.Unlock()
	if stop != nil {
		stop()
	}
	if result.Status == "completed" || result.Status == "canceled" {
		e.mu.Lock()
		delete(e.sharedTransfers, result.TransferID)
		e.mu.Unlock()
		if result.Status == "canceled" {
			e.removeSharedPartial(session)
		}
	}
	e.emitSharedProgress(result)
}

func (e *Engine) sharedTransferStopStatus(session *sharedTransferSession) string {
	select {
	case <-session.cancel:
		return "canceled"
	default:
	}
	session.mu.Lock()
	paused := session.paused
	session.mu.Unlock()
	if paused {
		return "paused"
	}
	return ""
}

func (e *Engine) removeSharedPartial(session *sharedTransferSession) {
	session.mu.Lock()
	tempPath := session.tempPath
	session.mu.Unlock()
	if tempPath != "" {
		_ = os.Remove(tempPath)
	}
}

func (e *Engine) downloadFriendSharedEntry(ctx context.Context, peer Peer, transfer SharedTransfer, targetPath string, session *sharedTransferSession) (SharedTransfer, error) {
	session.mu.Lock()
	resumeTempPath := session.tempPath
	resumeFinalPath := session.finalPath
	session.mu.Unlock()
	offset := int64(0)
	if resumeTempPath != "" {
		if info, statErr := os.Stat(resumeTempPath); statErr == nil {
			offset = info.Size()
		}
	}
	conn, decoder, _, err := e.dialSharedPeerContext(ctx, peer)
	if err != nil {
		select {
		case <-session.cancel:
			transfer.Status = "canceled"
			return transfer, nil
		default:
		}
		return transfer, err
	}
	session.mu.Lock()
	session.conn = conn
	session.mu.Unlock()
	defer func() {
		_ = conn.Close()
		session.mu.Lock()
		session.conn = nil
		session.mu.Unlock()
	}()
	select {
	case <-session.cancel:
		transfer.Status = "canceled"
		return transfer, nil
	default:
	}
	if err := writeWire(conn, wireMessage{Type: "share_download_request", TransferID: transfer.TransferID, SharedFolderID: transfer.SharedFolderID, RelativePath: transfer.RelativePath, Offset: offset}); err != nil {
		return transfer, err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		select {
		case <-session.cancel:
			transfer.Status = "canceled"
			return transfer, nil
		default:
		}
		return transfer, err
	}
	if response.Type == "share_error" {
		return transfer, fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_download_response" || response.Status != "accepted" {
		return transfer, fmt.Errorf("共享文件响应无效")
	}
	transfer.FileName, transfer.FileSize, transfer.RelativePath = response.FileName, response.FileSize, response.RelativePath
	if response.SharedFolderID != transfer.SharedFolderID {
		return transfer, fmt.Errorf("共享文件夹响应无效")
	}
	if transfer.FileSize < 0 {
		return transfer, fmt.Errorf("共享文件大小无效")
	}
	if response.Offset != offset {
		return transfer, fmt.Errorf("共享文件续传位置无效")
	}
	finalPath := resumeFinalPath
	if finalPath == "" {
		finalPath = uniqueSharedTarget(filepath.Clean(targetPath))
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return transfer, err
	}
	tempPath := resumeTempPath
	if tempPath == "" {
		tempPath = finalPath + ".part-" + transfer.TransferID
	}
	if offset > transfer.FileSize {
		return transfer, fmt.Errorf("共享文件续传位置无效")
	}
	openFlags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		openFlags |= os.O_TRUNC
	} else {
		openFlags |= os.O_APPEND
	}
	file, err := os.OpenFile(tempPath, openFlags, 0o600)
	if err != nil {
		return transfer, err
	}
	hash := sha256.New()
	if offset > 0 {
		partial, openErr := os.Open(tempPath)
		if openErr != nil {
			_ = file.Close()
			return transfer, openErr
		}
		_, hashErr := io.CopyN(hash, partial, offset)
		_ = partial.Close()
		if hashErr != nil {
			_ = file.Close()
			return transfer, hashErr
		}
	}
	session.mu.Lock()
	session.tempPath = tempPath
	session.finalPath = finalPath
	session.transfer = transfer
	session.mu.Unlock()
	transfer.Transferred = offset
	transfer.Status = "transferring"
	session.mu.Lock()
	session.transfer = transfer
	session.mu.Unlock()
	e.emitSharedProgress(transfer)
	defer func() { _ = file.Close() }()
	for {
		select {
		case <-ctx.Done():
			if e.sharedTransferStopStatus(session) == "paused" {
				transfer.Status = "paused"
			} else {
				transfer.Status = "canceled"
			}
			return transfer, nil
		case <-session.cancel:
			transfer.Status = "canceled"
			return transfer, nil
		default:
		}
		var message wireMessage
		if err := decoder.Decode(&message); err != nil {
			if status := e.sharedTransferStopStatus(session); status != "" {
				transfer.Status = status
				return transfer, nil
			}
			return transfer, err
		}
		switch message.Type {
		case "share_chunk":
			if message.TransferID != transfer.TransferID {
				return transfer, fmt.Errorf("共享传输标识无效")
			}
			payload, decodeErr := base64.StdEncoding.DecodeString(message.Payload)
			if decodeErr != nil || transfer.Transferred+int64(len(payload)) > transfer.FileSize {
				return transfer, fmt.Errorf("共享文件数据无效")
			}
			if _, err := file.Write(payload); err != nil {
				return transfer, err
			}
			if _, err := hash.Write(payload); err != nil {
				return transfer, err
			}
			transfer.Transferred += int64(len(payload))
			session.mu.Lock()
			session.transfer = transfer
			session.mu.Unlock()
			e.emitSharedProgress(transfer)
		case "share_error":
			return transfer, fmt.Errorf("%s", message.Status)
		case "share_complete":
			if transfer.Transferred != transfer.FileSize || (message.SHA256 != "" && hex.EncodeToString(hash.Sum(nil)) != message.SHA256) {
				return transfer, fmt.Errorf("共享文件校验失败")
			}
			if err := file.Close(); err != nil {
				return transfer, err
			}
			if err := os.Rename(tempPath, finalPath); err != nil {
				return transfer, err
			}
			transfer.TargetPath, transfer.Status = finalPath, "completed"
			transfer.Transferred = transfer.FileSize
			session.mu.Lock()
			session.transfer = transfer
			session.mu.Unlock()
			return transfer, nil
		}
	}
}

func (e *Engine) PauseSharedTransfer(transferID string) error {
	transferID = strings.TrimSpace(transferID)
	if transferID == "" {
		return fmt.Errorf("共享传输标识不能为空")
	}
	e.mu.RLock()
	session := e.sharedTransfers[transferID]
	e.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("共享传输不存在")
	}
	session.mu.Lock()
	if session.transfer.Status == "completed" || session.transfer.Status == "canceled" {
		session.mu.Unlock()
		return fmt.Errorf("当前下载无法暂停")
	}
	if session.paused {
		session.mu.Unlock()
		return nil
	}
	session.paused = true
	stop := session.stop
	conn := session.conn
	running := session.running
	pausedTransfer := session.transfer
	pausedTransfer.Status = "paused"
	session.transfer = pausedTransfer
	session.mu.Unlock()
	if stop != nil {
		stop()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if !running {
		e.emitSharedProgress(pausedTransfer)
	}
	return nil
}

func (e *Engine) ResumeSharedTransfer(transferID string) (SharedTransfer, error) {
	transferID = strings.TrimSpace(transferID)
	if transferID == "" {
		return SharedTransfer{}, fmt.Errorf("共享传输标识不能为空")
	}
	e.mu.RLock()
	session := e.sharedTransfers[transferID]
	e.mu.RUnlock()
	if session == nil {
		return SharedTransfer{}, fmt.Errorf("共享传输不存在")
	}
	session.mu.Lock()
	if session.running {
		session.mu.Unlock()
		return SharedTransfer{}, fmt.Errorf("下载正在进行中")
	}
	if session.transfer.Status != "paused" && session.transfer.Status != "failed" {
		session.mu.Unlock()
		return SharedTransfer{}, fmt.Errorf("当前下载无法继续")
	}
	ctx, stop := context.WithCancel(context.Background())
	session.paused = false
	session.stop = stop
	session.running = true
	transfer := session.transfer
	transfer.Status = "starting"
	transfer.ErrorMessage = ""
	session.transfer = transfer
	peer := session.peer
	target := session.target
	session.mu.Unlock()
	e.emitSharedProgress(transfer)
	go e.runSharedTransfer(ctx, peer, transfer, target, session)
	return transfer, nil
}

func (e *Engine) CancelSharedTransfer(transferID string) error {
	transferID = strings.TrimSpace(transferID)
	if transferID == "" {
		return fmt.Errorf("共享传输标识不能为空")
	}
	e.mu.RLock()
	session := e.sharedTransfers[transferID]
	e.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("共享传输不存在")
	}
	session.mu.Lock()
	session.cancelOnce.Do(func() { close(session.cancel) })
	stop := session.stop
	conn := session.conn
	running := session.running
	canceled := session.transfer
	canceled.Status = "canceled"
	session.transfer = canceled
	session.mu.Unlock()
	if stop != nil {
		stop()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if !running {
		e.mu.Lock()
		delete(e.sharedTransfers, transferID)
		e.mu.Unlock()
		e.removeSharedPartial(session)
		e.emitSharedProgress(canceled)
	}
	return nil
}

func (e *Engine) emitSharedProgress(transfer SharedTransfer) {
	e.emit("chat:shared-progress", transfer)
}
