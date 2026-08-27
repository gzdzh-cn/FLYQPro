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

const sharedDriveCapability = "shared-drive-v1"
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
	_, err := sharedRootPath(profile.SharedRootPath)
	return err == nil
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

func (e *Engine) handleSharedListRequest(conn net.Conn, hello, message wireMessage) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: e.sharedError(hello.DeviceID)})
		return
	}
	profile := e.Profile()
	entries, err := ListSharedEntries(profile.SharedRootPath, message.RelativePath)
	if err != nil {
		status := SharedPathInvalidError
		if strings.Contains(err.Error(), SharedUnavailableError) {
			status = SharedUnavailableError
		}
		_ = writeWire(conn, wireMessage{Type: "share_error", Status: status})
		return
	}
	_ = writeWire(conn, wireMessage{Type: "share_list_response", Status: "ok", RelativePath: message.RelativePath, Entries: entries})
}

func (e *Engine) handleSharedDownloadRequest(conn net.Conn, hello, message wireMessage, session *wireSession) {
	if !e.sharedAccessAllowed(hello.DeviceID) {
		_ = sessionWrite(session, conn, wireMessage{Type: "share_error", TransferID: message.TransferID, Status: e.sharedError(hello.DeviceID)})
		return
	}
	profile := e.Profile()
	entry, path, err := GetSharedEntry(profile.SharedRootPath, message.RelativePath, true)
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
	if err := sessionWrite(session, conn, wireMessage{Type: "share_download_response", TransferID: transferID, Status: "accepted", RelativePath: entry.RelativePath, FileName: entry.Name, FileSize: entry.Size, MimeType: entry.MimeType, SHA256: entry.SHA256, Offset: message.Offset}); err != nil {
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
		return nil, nil, ProtocolDialect{}, fmt.Errorf("对方客户端不支持共享盘")
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
		responseDialect, compatible := protocolDialectForMessage(response)
		if !compatible || !hasCapability(response.Capabilities, sharedDriveCapability) {
			_ = conn.Close()
			lastErr = fmt.Errorf("对方客户端不支持共享盘")
			continue
		}
		return conn, decoder, responseDialect, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无法连接好友")
	}
	return nil, nil, ProtocolDialect{}, lastErr
}

func (e *Engine) ListFriendSharedEntries(ctx context.Context, deviceID, relativePath string) ([]SharedEntry, error) {
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
	if err := writeWire(conn, wireMessage{Type: "share_list_request", RelativePath: relativePath}); err != nil {
		return nil, err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	if response.Type == "share_error" {
		return nil, fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_list_response" || response.Status != "ok" {
		return nil, fmt.Errorf("共享目录响应无效")
	}
	return response.Entries, nil
}

func sharedPreviewableEntry(entry SharedEntry) bool {
	mimeType := strings.ToLower(strings.TrimSpace(entry.MimeType))
	if strings.HasPrefix(mimeType, "image/") || mimeType == "application/pdf" {
		return true
	}
	switch strings.ToLower(filepath.Ext(entry.Name)) {
	case ".avif", ".bmp", ".gif", ".heic", ".heif", ".jpeg", ".jpg", ".png", ".webp", ".pdf":
		return true
	default:
		return false
	}
}

func (e *Engine) GetFriendSharedEntryPreview(ctx context.Context, deviceID, relativePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." || strings.Contains(clean, "..") {
		return "", fmt.Errorf("SHARED_PATH_INVALID")
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(clean)))
	if parent == "." {
		parent = ""
	}
	entries, err := e.ListFriendSharedEntries(ctx, deviceID, parent)
	if err != nil {
		return "", err
	}
	var entry SharedEntry
	for _, candidate := range entries {
		if candidate.RelativePath == clean {
			entry = candidate
			break
		}
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
	if err := writeWire(conn, wireMessage{Type: "share_download_request", TransferID: transferID, RelativePath: clean, Offset: 0}); err != nil {
		return "", err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return "", err
	}
	if response.Type == "share_error" {
		return "", fmt.Errorf("%s", response.Status)
	}
	if response.Type != "share_download_response" || response.Status != "accepted" || response.Offset != 0 {
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

func (e *Engine) DownloadFriendSharedEntry(ctx context.Context, deviceID, relativePath, targetPath string) (SharedTransfer, error) {
	transfer := SharedTransfer{TransferID: newID(), DeviceID: deviceID, RelativePath: relativePath, Direction: "receive", Status: "starting", TargetPath: targetPath, FileName: filepath.Base(filepath.FromSlash(relativePath))}
	if err := ctx.Err(); err != nil {
		return transfer, err
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
	if err := writeWire(conn, wireMessage{Type: "share_download_request", TransferID: transfer.TransferID, RelativePath: transfer.RelativePath, Offset: offset}); err != nil {
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
