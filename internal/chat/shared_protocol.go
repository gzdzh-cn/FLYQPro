package chat

import (
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
	"time"
)

const sharedDriveCapability = "shared-drive-v1"

type sharedTransferSession struct {
	conn   net.Conn
	cancel chan struct{}
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
	transferID := strings.TrimSpace(message.TransferID)
	if transferID == "" {
		transferID = newID()
	}
	if err := sessionWrite(session, conn, wireMessage{Type: "share_download_response", TransferID: transferID, Status: "accepted", RelativePath: entry.RelativePath, FileName: entry.Name, FileSize: entry.Size, MimeType: entry.MimeType, SHA256: entry.SHA256}); err != nil {
		return
	}
	buffer := make([]byte, 256*1024)
	var transferred int64
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
	for _, dialect := range dialects {
		conn, dialErr := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), clientTLS)
		if dialErr != nil {
			lastErr = dialErr
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

func (e *Engine) DownloadFriendSharedEntry(ctx context.Context, deviceID, relativePath, targetPath string) (SharedTransfer, error) {
	transfer := SharedTransfer{TransferID: newID(), DeviceID: deviceID, RelativePath: relativePath, Direction: "receive", Status: "starting", TargetPath: targetPath}
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
	conn, decoder, _, err := e.dialSharedPeer(peer)
	if err != nil {
		return transfer, err
	}
	defer conn.Close()
	cancel := make(chan struct{})
	e.mu.Lock()
	e.sharedTransfers[transfer.TransferID] = &sharedTransferSession{conn: conn, cancel: cancel}
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.sharedTransfers, transfer.TransferID)
		e.mu.Unlock()
	}()
	if err := writeWire(conn, wireMessage{Type: "share_download_request", TransferID: transfer.TransferID, RelativePath: relativePath}); err != nil {
		return transfer, err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
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
	finalPath := uniqueSharedTarget(filepath.Clean(targetPath))
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return transfer, err
	}
	tempPath := finalPath + ".part-" + transfer.TransferID
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return transfer, err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}()
	hash := sha256.New()
	transfer.Status = "transferring"
	e.emitSharedProgress(transfer)
	for {
		select {
		case <-ctx.Done():
			close(cancel)
			_ = conn.Close()
			transfer.Status = "canceled"
			return transfer, ctx.Err()
		case <-cancel:
			transfer.Status = "canceled"
			return transfer, nil
		default:
		}
		var message wireMessage
		if err := decoder.Decode(&message); err != nil {
			select {
			case <-cancel:
				transfer.Status = "canceled"
				return transfer, nil
			default:
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
			e.emitSharedProgress(transfer)
			return transfer, nil
		}
	}
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
	select {
	case <-session.cancel:
	default:
		close(session.cancel)
	}
	_ = session.conn.Close()
	return nil
}

func (e *Engine) emitSharedProgress(transfer SharedTransfer) {
	e.emit("chat:shared-progress", transfer)
}
