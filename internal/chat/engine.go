package chat

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Engine struct {
	mu                  sync.RWMutex
	profile             Profile
	identity            Identity
	listener            net.Listener
	discoveryTCP        net.Listener
	udp                 *net.UDPConn
	stop                chan struct{}
	done                chan struct{}
	peers               map[string]Peer
	incoming            map[string]*incomingFile
	lastScan            time.Time
	lastErr             string
	started             bool
	serviceStopped      bool
	attachmentMigration bool
	friendRestoreAt     map[string]time.Time
}

func (e *Engine) SetAttachmentMigrationActive(active bool) {
	e.mu.Lock()
	e.attachmentMigration = active
	e.mu.Unlock()
}

func (e *Engine) IsAttachmentMigrationActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.attachmentMigration
}

func (e *Engine) CancelIncomingForPeer(peerDeviceID string) {
	e.mu.Lock()
	transfers := make([]*incomingFile, 0)
	for attachmentID, transfer := range e.incoming {
		if transfer.senderID != peerDeviceID {
			continue
		}
		delete(e.incoming, attachmentID)
		transfers = append(transfers, transfer)
	}
	e.mu.Unlock()
	for _, transfer := range transfers {
		if transfer.file != nil {
			_ = transfer.file.Close()
		}
	}
}

type incomingFile struct {
	file         *os.File
	tempPath     string
	messageID    string
	senderID     string
	fileName     string
	mimeType     string
	expected     int64
	received     int64
	lastProgress int64
	sha256       string
	digest       hash.Hash
}

func NewEngine() *Engine {
	return &Engine{peers: make(map[string]Peer), incoming: make(map[string]*incomingFile), friendRestoreAt: make(map[string]time.Time)}
}

func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()
	if err := EnsureDataDirs(); err != nil {
		return err
	}
	if err := EnsureDefaults(ctx, DefaultAttachmentDir()); err != nil {
		return err
	}
	profile, err := GetProfile(ctx)
	if err != nil {
		return err
	}
	if profile.AvatarPath != "" && profile.AvatarHash == "" {
		if data, avatarErr := os.ReadFile(profile.AvatarPath); avatarErr == nil && len(data) > 0 && len(data) <= 5*1024*1024 {
			profile.AvatarHash = sha256Hex(data)
			if saveErr := SaveProfile(ctx, profile); saveErr != nil {
				return saveErr
			}
		}
	}
	identity, err := LoadOrCreateIdentity(ctx)
	if err != nil {
		return err
	}
	if err := RecoverSendingMessages(ctx, identity.DeviceID); err != nil {
		return err
	}
	platform, osVersion := platformInfo()
	identity.Platform = platform
	identity.OSVersion = osVersion
	identity.IP = localIPv4()

	tlsCert, err := identity.TLSCertificate()
	if err != nil {
		return err
	}
	listener, err := tls.Listen("tcp", ":0", &tls.Config{Certificates: []tls.Certificate{tlsCert}, ClientAuth: tls.RequireAnyClientCert, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("启动聊天端口失败: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	identity.Port = port
	udp, udpErr := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: DiscoveryPort})
	if udpErr != nil {
		_ = listener.Close()
		return fmt.Errorf("启动局域网发现失败: %w", udpErr)
	}
	discoveryTCP, tcpErr := net.Listen("tcp4", fmt.Sprintf(":%d", DiscoveryPort))
	if tcpErr != nil {
		_ = udp.Close()
		_ = listener.Close()
		return fmt.Errorf("启动 TCP 发现失败: %w", tcpErr)
	}

	e.mu.Lock()
	e.profile, e.identity, e.listener, e.discoveryTCP, e.udp = profile, identity, listener, discoveryTCP, udp
	e.serviceStopped = false
	if peers, peerErr := ListPeers(ctx, ""); peerErr == nil {
		for _, peer := range peers {
			e.peers[peer.DeviceID] = peer
		}
	}
	e.stop, e.done, e.started = make(chan struct{}), make(chan struct{}), true
	e.mu.Unlock()
	go e.acceptLoop()
	go e.discoveryTCPLoop()
	go e.discoveryLoop()
	go e.scanLoop()
	go e.livenessLoop()
	go e.probeKnownPeers()
	go e.scanNetwork(true)
	e.emit("chat:network-status", e.NetworkStatus())
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return
	}
	stop, listener, discoveryTCP, udp := e.stop, e.listener, e.discoveryTCP, e.udp
	e.started = false
	e.serviceStopped = true
	e.mu.Unlock()

	// Notify peers while the discovery socket is still available. This is an
	// immediate best-effort signal; the liveness probe remains the fallback for
	// crashes, force quits, and network failures.
	e.broadcastPresence("offline")
	close(stop)
	_ = listener.Close()
	_ = discoveryTCP.Close()
	_ = udp.Close()

	e.mu.Lock()
	for attachmentID, transfer := range e.incoming {
		_ = transfer.file.Close()
		delete(e.incoming, attachmentID)
	}
	done := e.done
	e.mu.Unlock()
	e.emit("chat:peer-updated", e.Peers())
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (e *Engine) discoveryTCPLoop() {
	for {
		conn, err := e.discoveryTCP.Accept()
		if err != nil {
			select {
			case <-e.stop:
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		go e.handleDiscoveryTCP(conn)
	}
}

func (e *Engine) handleDiscoveryTCP(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(800 * time.Millisecond))
	var message wireMessage
	if err := json.NewDecoder(conn).Decode(&message); err != nil || message.Type != "discover" || message.DeviceID == e.identity.DeviceID {
		return
	}
	dialect, compatible := protocolDialectForMessage(message)
	if !compatible {
		return
	}
	if e.canRespondToDiscovery(message.DeviceID) {
		_ = writeWire(conn, e.helloMessageForDialect("announce", dialect))
	}
}

func (e *Engine) acceptLoop() {
	for {
		conn, err := e.listener.Accept()
		if err != nil {
			select {
			case <-e.stop:
				close(e.done)
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		go e.handleConnection(conn)
	}
}

func (e *Engine) handleConnection(raw net.Conn) {
	defer raw.Close()
	conn, ok := raw.(*tls.Conn)
	if !ok {
		return
	}
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	if err := conn.Handshake(); err != nil {
		return
	}
	var hello wireMessage
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&hello); err != nil || hello.Type != "hello" || hello.DeviceID == e.identity.DeviceID {
		_ = writeWire(conn, wireMessage{Type: "error", Status: "PROTOCOL_UNSUPPORTED"})
		return
	}
	dialect, compatible := protocolDialectForMessage(hello)
	if !compatible {
		_ = writeWire(conn, wireMessage{Type: "error", Status: "PROTOCOL_UNSUPPORTED"})
		return
	}
	// The chat port is also probed by peers that still have an old discovery
	// record. Do not let that path make a stranger visible after discovery has
	// been disabled. Friends and peers with an active friend request still need
	// a direct connection for messaging and request responses.
	if !e.canAcceptPeerConnection(hello.DeviceID) {
		_ = writeWire(conn, wireMessage{Type: "error", Status: "DISCOVERY_DISABLED"})
		return
	}
	wasOnline := false
	if existing, existingErr := e.peer(hello.DeviceID); existingErr == nil {
		wasOnline = existing.Online
	}
	if err := e.upsertWirePeerWithOptions(hello); err != nil {
		_ = writeWire(conn, wireMessage{Type: "error", Status: err.Error()})
		return
	}
	if !hello.Probe || !wasOnline {
		e.emit("chat:peer-updated", e.Peers())
	}
	peer, peerErr := e.peer(hello.DeviceID)
	if peerErr != nil {
		_ = writeWire(conn, wireMessage{Type: "error", Status: peerErr.Error()})
		return
	}
	if verifyErr := verifyPeerCertificate(conn, peer); verifyErr != nil {
		_ = writeWire(conn, wireMessage{Type: "error", Status: verifyErr.Error()})
		return
	}
	e.touchPeer(hello.DeviceID)
	_ = writeWire(conn, e.helloMessageForDialect("hello_ack", dialect))
	_ = conn.SetDeadline(time.Time{})
	for {
		var message wireMessage
		if err := decoder.Decode(&message); err != nil {
			return
		}
		e.handleWire(conn, hello, message)
	}
}

func (e *Engine) handleWire(conn net.Conn, hello wireMessage, message wireMessage) {
	switch message.Type {
	case "ping":
		dialect, ok := protocolDialectForMessage(hello)
		if !ok {
			dialect = protocolDialects[0]
		}
		_ = writeWire(conn, wireMessage{Type: "pong", Protocol: dialect.Name, Major: dialect.Major, Minor: ProtocolMinor})
	case "friend_request":
		// A known peer may send a request even when discovery is disabled. The
		// discoverable flag controls presence broadcasts, not direct requests
		// received over an already authenticated connection.
		status := "pending"
		acceptedAt := ""
		if e.isFriend(hello.DeviceID) {
			status = "accepted"
			acceptedAt = nowString()
		}
		duplicate := false
		if requests, listErr := listFriendRequestRows(context.Background(), ""); listErr == nil {
			for _, existing := range requests {
				if existing.RequestID == message.RequestID {
					duplicate = true
					break
				}
			}
		}
		request := FriendRequest{RequestID: message.RequestID, DeviceID: hello.DeviceID, Nickname: hello.Nickname, Message: message.Content, Status: status, Direction: "received", CreatedAt: nowString(), AcceptedAt: acceptedAt}
		if err := SaveFriendRequest(context.Background(), request); err == nil {
			if status == "accepted" {
				_ = UpdateFriendRequestsForDevice(context.Background(), hello.DeviceID, "accepted", acceptedAt)
				_ = writeWire(conn, wireMessage{Type: "friend_request_response", RequestID: message.RequestID, Status: "accepted", AcceptedAt: acceptedAt})
			} else if !duplicate {
				if aggregated, ok := e.friendRequestForDevice(hello.DeviceID); ok {
					e.emit("chat:friend-request", aggregated)
				}
			}
		}
	case "friend_request_response":
		status := message.Status
		acceptedAt := ""
		if status == "accepted" {
			acceptedAt = message.AcceptedAt
			if requests, listErr := listFriendRequestRows(context.Background(), ""); listErr == nil {
				localAcceptedAt := earliestAcceptedAt(requests, hello.DeviceID)
				if localAcceptedAt != "" && (acceptedAt == "" || requestTimeBefore(localAcceptedAt, acceptedAt)) {
					acceptedAt = localAcceptedAt
				}
			}
			if acceptedAt == "" {
				acceptedAt = nowString()
			}
			if err := SetPeerRelation(context.Background(), hello.DeviceID, PeerRelation); err == nil {
				e.updatePeerRelation(hello.DeviceID, PeerRelation)
			}
			_ = UpdateFriendRequestsForDevice(context.Background(), hello.DeviceID, status, acceptedAt)
		} else if status == "rejected" {
			_ = UpdateFriendRequestsForDevice(context.Background(), hello.DeviceID, status, "")
		} else {
			_ = UpdateFriendRequest(context.Background(), message.RequestID, status)
		}
		if aggregated, ok := e.friendRequestForDevice(hello.DeviceID); ok {
			e.emit("chat:friend-request-updated", aggregated)
		} else {
			e.emit("chat:friend-request-updated", map[string]any{"requestId": message.RequestID, "status": status, "deviceId": hello.DeviceID, "acceptedAt": acceptedAt})
		}
		e.emit("chat:peer-updated", e.Peers())
	case "friend_restore":
		e.mu.RLock()
		localDeviceID := e.identity.DeviceID
		e.mu.RUnlock()
		if err := verifyFriendRestore(message, hello, localDeviceID); err != nil {
			_ = writeWire(conn, wireMessage{Type: "friend_restore_ack", Status: "rejected"})
			return
		}
		if err := SetPeerRelation(context.Background(), hello.DeviceID, PeerRelation); err != nil {
			_ = writeWire(conn, wireMessage{Type: "friend_restore_ack", Status: "rejected"})
			return
		}
		e.updatePeerRelation(hello.DeviceID, PeerRelation)
		e.emit("chat:peer-updated", e.Peers())
		_ = writeWire(conn, wireMessage{Type: "friend_restore_ack", SourceDeviceID: localDeviceID, TargetDeviceID: hello.DeviceID, RestoreVersion: friendRestoreVersion, Status: "accepted"})
	case "friend_restore_ack":
		// Control message only; it has no UI or message side effects.
	case "message":
		if !e.isFriend(hello.DeviceID) {
			_ = writeWire(conn, wireMessage{Type: "error", Status: "FRIENDSHIP_REQUIRED"})
			return
		}
		conversationID, err := EnsureConversation(context.Background(), hello.DeviceID)
		if err != nil {
			return
		}
		exists, err := MessageExists(context.Background(), message.MessageID)
		if err != nil {
			return
		}
		_ = writeWire(conn, wireMessage{Type: "ack", MessageID: message.MessageID, Status: "sent"})
		if exists {
			return
		}
		messageRecord := Message{MessageID: message.MessageID, ConversationID: conversationID, SenderDeviceID: hello.DeviceID, Kind: message.Kind, Content: message.Content, Status: "sent", CreatedAt: nowString()}
		if err := SaveMessage(context.Background(), messageRecord); err == nil {
			_ = IncrementConversationUnread(context.Background(), conversationID)
			e.emit("chat:message", messageRecord)
		}
	case "avatar_request":
		if !e.isFriend(hello.DeviceID) {
			return
		}
		profile := e.Profile()
		if profile.AvatarPath == "" || profile.AvatarHash == "" {
			_ = writeWire(conn, wireMessage{Type: "avatar_response", DeviceID: e.identity.DeviceID})
			return
		}
		data, err := os.ReadFile(profile.AvatarPath)
		if err != nil || len(data) > 5*1024*1024 {
			return
		}
		mimeType := mime.TypeByExtension(filepath.Ext(profile.AvatarPath))
		if mimeType == "" {
			mimeType = "image/png"
		}
		_ = writeWire(conn, wireMessage{Type: "avatar_response", DeviceID: e.identity.DeviceID, AvatarHash: profile.AvatarHash, AvatarVersion: profile.AvatarVersion, AvatarMime: mimeType, AvatarData: base64.StdEncoding.EncodeToString(data)})
	case "avatar_response":
		if !e.isFriend(hello.DeviceID) || message.AvatarData == "" {
			return
		}
		data, err := base64.StdEncoding.DecodeString(message.AvatarData)
		if err != nil || len(data) == 0 || len(data) > 5*1024*1024 || sha256Hex(data) != message.AvatarHash {
			return
		}
		ext := ".png"
		if strings.HasPrefix(message.AvatarMime, "image/") {
			ext = "." + strings.TrimPrefix(message.AvatarMime, "image/")
		}
		cacheDir := filepath.Join(AppDataDir(), "avatar-cache")
		if os.MkdirAll(cacheDir, 0o700) != nil {
			return
		}
		path := filepath.Join(cacheDir, safeFileName(hello.DeviceID)+ext)
		if os.WriteFile(path, data, 0o600) != nil {
			return
		}
		if SetPeerAvatar(context.Background(), hello.DeviceID, path, message.AvatarHash, message.AvatarVersion) == nil {
			e.mu.Lock()
			peer := e.peers[hello.DeviceID]
			peer.AvatarPath, peer.AvatarHash, peer.AvatarVersion = path, message.AvatarHash, message.AvatarVersion
			e.peers[hello.DeviceID] = peer
			e.mu.Unlock()
			e.emit("chat:peer-updated", e.Peers())
		}
	case "read_receipt":
		for _, messageID := range message.MessageIDs {
			if err := UpdateMessageStatus(context.Background(), messageID, "read"); err == nil {
				e.emit("chat:message-status", map[string]any{"messageId": messageID, "status": "read"})
			}
		}
	case "file_offer":
		if !e.isFriend(hello.DeviceID) {
			_ = writeWire(conn, wireMessage{Type: "error", Status: "FRIENDSHIP_REQUIRED"})
			return
		}
		if message.FileSize < 0 {
			if hasCapability(hello.Capabilities, "storage-preflight-v1") {
				_ = writeWire(conn, wireMessage{Type: "file_offer_response", MessageID: message.MessageID, AttachmentID: message.AttachmentID, Status: "rejected", Reason: "INVALID_FILE_SIZE"})
			}
			return
		}
		conversationID, err := EnsureConversation(context.Background(), hello.DeviceID)
		if err != nil {
			return
		}
		attachmentID := message.AttachmentID
		if attachmentID == "" {
			attachmentID = newID()
		}
		if message.MessageID == "" {
			return
		}
		attachmentMime := message.MimeType
		if attachmentMime == "" {
			attachmentMime = mime.TypeByExtension(filepath.Ext(message.FileName))
		}
		if attachmentMime == "" {
			attachmentMime = "application/octet-stream"
		}
		if exists, existsErr := MessageExists(context.Background(), message.MessageID); existsErr != nil || exists {
			return
		}
		tempDir := filepath.Join(AppDataDir(), "temp")
		preflight := hasCapability(hello.Capabilities, "storage-preflight-v1")
		if preflight {
			available, availableErr := availableDiskBytes(tempDir)
			required := requiredAttachmentBytes(message.FileSize)
			if availableErr != nil {
				_ = writeWire(conn, wireMessage{Type: "file_offer_response", MessageID: message.MessageID, AttachmentID: attachmentID, Status: "rejected", Reason: "STORAGE_UNAVAILABLE"})
				e.emit("chat:attachment", map[string]any{"attachmentId": attachmentID, "messageId": message.MessageID, "fileName": message.FileName, "status": "rejected", "reason": "STORAGE_UNAVAILABLE"})
				return
			}
			if available < required {
				_ = writeWire(conn, wireMessage{Type: "file_offer_response", MessageID: message.MessageID, AttachmentID: attachmentID, Status: "rejected", Reason: "INSUFFICIENT_STORAGE", AvailableBytes: available, RequiredBytes: required})
				e.emit("chat:attachment", map[string]any{"attachmentId": attachmentID, "messageId": message.MessageID, "fileName": message.FileName, "status": "rejected", "reason": "INSUFFICIENT_STORAGE", "availableBytes": available, "requiredBytes": required})
				return
			}
		}
		tempPath := filepath.Join(tempDir, attachmentID+".part")
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			if preflight {
				_ = writeWire(conn, wireMessage{Type: "file_offer_response", MessageID: message.MessageID, AttachmentID: attachmentID, Status: "rejected", Reason: "STORAGE_UNAVAILABLE"})
			}
			return
		}
		if preflight {
			if err := writeWire(conn, wireMessage{Type: "file_offer_response", MessageID: message.MessageID, AttachmentID: attachmentID, Status: "accepted"}); err != nil {
				_ = file.Close()
				_ = os.Remove(tempPath)
				return
			}
		}
		messageRecord := Message{MessageID: message.MessageID, ConversationID: conversationID, SenderDeviceID: hello.DeviceID, Kind: "file", Content: message.FileName, Status: "receiving", CreatedAt: nowString(), AttachmentID: attachmentID, AttachmentName: message.FileName, AttachmentSize: message.FileSize, AttachmentMime: attachmentMime, AttachmentStatus: "receiving"}
		if err := SaveMessage(context.Background(), messageRecord); err != nil {
			_ = file.Close()
			return
		}
		_ = IncrementConversationUnread(context.Background(), conversationID)
		_ = SaveAttachment(context.Background(), Attachment{AttachmentID: attachmentID, MessageID: message.MessageID, FileName: message.FileName, MimeType: attachmentMime, FileSize: message.FileSize, SHA256: message.SHA256, LocalPath: tempPath, Status: "receiving"})
		e.mu.Lock()
		e.incoming[attachmentID] = &incomingFile{file: file, tempPath: tempPath, messageID: message.MessageID, senderID: hello.DeviceID, fileName: message.FileName, mimeType: attachmentMime, expected: message.FileSize, sha256: message.SHA256, digest: sha256.New()}
		e.mu.Unlock()
		e.emit("chat:message", messageRecord)
		e.emit("chat:attachment", messageRecord)
		e.emitTransferProgress(messageRecord.MessageID, attachmentID, hello.DeviceID, 0, message.FileSize, "receive", "receiving")
	case "file_chunk":
		e.mu.RLock()
		transfer := e.incoming[message.AttachmentID]
		e.mu.RUnlock()
		if transfer == nil {
			return
		}
		data, err := base64.StdEncoding.DecodeString(message.Payload)
		if err != nil {
			return
		}
		if transfer.expected < 0 || transfer.received > transfer.expected || int64(len(data)) > transfer.expected-transfer.received {
			e.failIncomingFile(message.AttachmentID, "FILE_SIZE_EXCEEDED")
			_ = writeWire(conn, wireMessage{Type: "file_progress", MessageID: transfer.messageID, AttachmentID: message.AttachmentID, FileSize: transfer.expected, Transferred: transfer.received, Status: "failed", Reason: "FILE_SIZE_EXCEEDED"})
			return
		}
		if _, err := transfer.file.Write(data); err != nil {
			e.failIncomingFile(message.AttachmentID, "INSUFFICIENT_STORAGE")
			_ = writeWire(conn, wireMessage{Type: "file_progress", MessageID: transfer.messageID, AttachmentID: message.AttachmentID, FileSize: transfer.expected, Transferred: transfer.received, Status: "failed", Reason: "INSUFFICIENT_STORAGE"})
			return
		}
		if transfer.digest != nil {
			_, _ = transfer.digest.Write(data)
		}
		transfer.received += int64(len(data))
		if transfer.received-transfer.lastProgress >= 256*1024 || (transfer.expected > 0 && transfer.received >= transfer.expected) {
			e.emitTransferProgress(transfer.messageID, message.AttachmentID, transfer.senderID, transfer.received, transfer.expected, "receive", "transferring")
			transfer.lastProgress = transfer.received
		}
		// New clients acknowledge every chunk so the sender can show the
		// receiver's actual progress. Peers without the optional progress capability ignore this frame.
		_ = writeWire(conn, wireMessage{Type: "file_progress", MessageID: transfer.messageID, AttachmentID: message.AttachmentID, FileSize: transfer.expected, Transferred: transfer.received, Status: "receiving"})
	case "file_complete":
		e.mu.RLock()
		transfer := e.incoming[message.AttachmentID]
		messageID, total := message.MessageID, message.FileSize
		if transfer != nil {
			messageID, total = transfer.messageID, transfer.expected
		}
		e.mu.RUnlock()
		status := e.finishIncomingFile(message.AttachmentID)
		// finishIncomingFile removes the transfer, so use the message metadata
		// supplied by the sender for the final optional acknowledgement.
		_ = writeWire(conn, wireMessage{Type: "file_progress", MessageID: messageID, AttachmentID: message.AttachmentID, Transferred: total, FileSize: total, Status: status})
	}
}

const attachmentSafetyMargin int64 = 16 * 1024 * 1024

func requiredAttachmentBytes(fileSize int64) int64 {
	if fileSize < 0 || fileSize > int64(^uint64(0)>>1)-attachmentSafetyMargin {
		return int64(^uint64(0) >> 1)
	}
	return fileSize + attachmentSafetyMargin
}

func formatBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	if value < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(value)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(value)/(1024*1024*1024))
}

func (e *Engine) failIncomingFile(attachmentID, reason string) {
	e.mu.Lock()
	transfer := e.incoming[attachmentID]
	delete(e.incoming, attachmentID)
	e.mu.Unlock()
	if transfer == nil {
		return
	}
	if transfer.file != nil {
		_ = transfer.file.Close()
	}
	_ = os.Remove(transfer.tempPath)
	_ = SaveAttachment(context.Background(), Attachment{AttachmentID: attachmentID, MessageID: transfer.messageID, FileName: transfer.fileName, MimeType: transfer.mimeType, FileSize: transfer.expected, SHA256: transfer.sha256, LocalPath: transfer.tempPath, Status: "failed"})
	_ = exec(context.Background(), `UPDATE messages SET status=? WHERE message_id=?`, "failed", transfer.messageID)
	e.emit("chat:attachment", map[string]any{"attachmentId": attachmentID, "messageId": transfer.messageID, "fileName": transfer.fileName, "status": "failed", "reason": reason})
	e.emitTransferProgress(transfer.messageID, attachmentID, transfer.senderID, transfer.received, transfer.expected, "receive", "failed")
}

func (e *Engine) finishIncomingFile(attachmentID string) string {
	e.mu.Lock()
	transfer := e.incoming[attachmentID]
	delete(e.incoming, attachmentID)
	e.mu.Unlock()
	if transfer == nil {
		return "failed"
	}
	_ = transfer.file.Close()
	valid := transfer.digest != nil && hex.EncodeToString(transfer.digest.Sum(nil)) == transfer.sha256 && transfer.received == transfer.expected
	status := "pending"
	localPath := transfer.tempPath
	if valid && e.Profile().AutoSave && !e.IsAttachmentMigrationActive() {
		if target, targetErr := AttachmentTargetPath(e.Profile().FileSavePath, transfer.senderID, transfer.fileName); targetErr == nil {
			localPath = target
			if os.Rename(transfer.tempPath, localPath) == nil {
				status = "saved"
			}
		}
	}
	if !valid {
		status = "failed"
		_ = os.Remove(transfer.tempPath)
	}
	attachmentMime := transfer.mimeType
	if attachmentMime == "" {
		attachmentMime = mime.TypeByExtension(filepath.Ext(transfer.fileName))
	}
	if attachmentMime == "" {
		attachmentMime = "application/octet-stream"
	}
	_ = SaveAttachment(context.Background(), Attachment{AttachmentID: attachmentID, MessageID: transfer.messageID, FileName: transfer.fileName, MimeType: attachmentMime, FileSize: transfer.expected, SHA256: transfer.sha256, LocalPath: localPath, Status: status})
	messageStatus := "sent"
	if !valid {
		messageStatus = "failed"
	}
	if messageRecord, messageErr := GetMessage(context.Background(), transfer.messageID); messageErr == nil {
		messageRecord.Status = messageStatus
		messageRecord.AttachmentMime = attachmentMime
		messageRecord.AttachmentStatus = status
		messageRecord.AttachmentPath = localPath
		e.emit("chat:message", messageRecord)
	}
	_ = exec(context.Background(), `UPDATE messages SET status=? WHERE message_id=?`, messageStatus, transfer.messageID)
	e.emit("chat:attachment", map[string]any{"attachmentId": attachmentID, "messageId": transfer.messageID, "fileName": transfer.fileName, "status": status, "localPath": localPath, "valid": valid})
	e.emitTransferProgress(transfer.messageID, attachmentID, transfer.senderID, transfer.received, transfer.expected, "receive", map[bool]string{true: "completed", false: "failed"}[valid])
	if valid {
		return "completed"
	}
	return "failed"
}

func (e *Engine) emitTransferProgress(messageID, attachmentID, peerDeviceID string, transferred, total int64, direction, phase string) {
	if transferred < 0 {
		transferred = 0
	}
	if total > 0 && transferred > total {
		transferred = total
	}
	percent := 0
	if total > 0 {
		percent = int(transferred * 100 / total)
	}
	value := map[string]any{
		"messageId":    messageID,
		"attachmentId": attachmentID,
		"peerDeviceId": peerDeviceID,
		"transferred":  transferred,
		"total":        total,
		"percent":      percent,
		"direction":    direction,
		"phase":        phase,
	}
	switch direction {
	case "send":
		value["sent"] = transferred
	case "receive":
		value["received"] = transferred
	case "remote-receive":
		value["remoteReceived"] = transferred
	}
	e.emit("chat:transfer-progress", value)
}

// ArchivePendingAttachments moves files that arrived during a storage
// migration out of the app temp directory after the migration lock is
// released. Manual-receive attachments remain pending and are handled by the
// explicit AcceptAttachment action.
func (e *Engine) ArchivePendingAttachments() {
	if e.IsAttachmentMigrationActive() || !e.Profile().AutoSave {
		return
	}
	profile := e.Profile()
	tempRoot, err := absoluteCleanPath(filepath.Join(AppDataDir(), "temp"))
	if err != nil {
		return
	}
	rows, err := ListAttachmentMigrationRows(context.Background())
	if err != nil {
		return
	}
	for _, row := range rows {
		if row.Status != "pending" {
			continue
		}
		source, sourceErr := absoluteCleanPath(row.LocalPath)
		if sourceErr != nil || !isWithin(source, tempRoot) {
			continue
		}
		if _, statErr := os.Stat(source); statErr != nil {
			continue
		}
		target, targetErr := AttachmentTargetPath(profile.FileSavePath, row.PeerDeviceID, row.FileName)
		if targetErr != nil || os.Rename(source, target) != nil {
			continue
		}
		if err := UpdateAttachmentLocalPath(context.Background(), row.AttachmentID, target); err != nil {
			_ = os.Rename(target, source)
			continue
		}
		_ = SaveAttachment(context.Background(), Attachment{AttachmentID: row.AttachmentID, MessageID: row.MessageID, FileName: row.FileName, FileSize: row.FileSize, SHA256: row.SHA256, LocalPath: target, Status: "saved"})
		e.emit("chat:attachment", map[string]any{"attachmentId": row.AttachmentID, "status": "saved", "localPath": target})
	}
}

func (e *Engine) discoveryLoop() {
	buffer := make([]byte, 16*1024)
	for {
		_ = e.udp.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, addr, err := e.udp.ReadFromUDP(buffer)
		if err != nil {
			select {
			case <-e.stop:
				return
			default:
				continue
			}
		}
		var message wireMessage
		if json.Unmarshal(buffer[:n], &message) != nil || message.DeviceID == e.identity.DeviceID {
			continue
		}
		dialect, compatible := protocolDialectForMessage(message)
		if !compatible {
			continue
		}
		switch message.Type {
		case "discover":
			if e.canRespondToDiscovery(message.DeviceID) {
				_ = e.sendDiscovery(addr, e.helloMessageForDialect("announce", dialect))
			}
		case "announce":
			message.IP = addr.IP.String()
			e.handleAnnounce(message)
		case "withdraw":
			e.forgetDiscoveredPeer(message.DeviceID)
		case "offline":
			e.handleOffline(message.DeviceID)
		}
	}
}

func (e *Engine) scanLoop() {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()
	lastUnicastProbe := time.Now()
	for {
		select {
		case <-ticker.C:
			unicastProbe := time.Since(lastUnicastProbe) >= 30*time.Second
			e.scanNetwork(unicastProbe)
			// Discovery has no explicit "goodbye" packet. Re-publish the
			// computed presence so the UI can turn stale peers offline.
			e.emit("chat:peer-updated", e.Peers())
			if unicastProbe {
				lastUnicastProbe = time.Now()
			}
		case <-e.stop:
			return
		}
	}
}

// Scan sends a discovery request immediately instead of waiting for the
// periodic scan ticker. It is used by the Discover page's manual refresh.
func (e *Engine) Scan() {
	if e.isStarted() {
		e.scanNetwork(true)
	}
}

func (e *Engine) isStarted() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.started
}

func (e *Engine) scanNetwork(includeUnicastProbe bool) {
	// A discovery request contains the sender's identity. It must not be
	// broadcast while discovery is disabled, because older clients may treat a
	// request as a visible peer instead of waiting for an announce response.
	// Known friends are queried directly so their presence can still recover
	// without exposing this device to strangers.
	if !e.Profile().Discoverable {
		firstErr := e.scanKnownFriends()
		e.mu.Lock()
		e.lastScan = time.Now()
		if firstErr != nil {
			e.lastErr = firstErr.Error()
		} else {
			e.lastErr = ""
		}
		e.mu.Unlock()
		e.emit("chat:network-status", e.NetworkStatus())
		return
	}

	targets := broadcastAddresses()
	if len(targets) == 0 {
		targets = []net.UDPAddr{{IP: net.IPv4bcast, Port: DiscoveryPort}}
	}
	var subnetTargets []net.UDPAddr
	if includeUnicastProbe {
		subnetTargets = localSubnetTargets()
		targets = append(targets, subnetTargets...)
	}

	var firstErr error
	for _, dialect := range protocolDialects {
		message := e.helloMessageForDialect("discover", dialect)
		for index := range targets[:len(targets)-len(subnetTargets)] {
			if err := e.sendDiscovery(&targets[index], message); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		for index := len(targets) - len(subnetTargets); index < len(targets); index++ {
			// Individual hosts can be offline; a failed unicast probe is expected.
			_ = e.sendDiscovery(&targets[index], message)
		}
		if includeUnicastProbe {
			if probeErr := e.probeTCPSubnets(message, subnetTargets); probeErr != nil && firstErr == nil {
				firstErr = probeErr
			}
		}
	}
	e.mu.Lock()
	e.lastScan = time.Now()
	if firstErr != nil {
		e.lastErr = firstErr.Error()
	} else {
		e.lastErr = ""
	}
	e.mu.Unlock()
	e.emit("chat:network-status", e.NetworkStatus())
}

func (e *Engine) scanKnownFriends() error {
	var firstErr error
	for _, peer := range e.PeersByRelation(PeerRelation) {
		ip := net.ParseIP(strings.TrimSpace(peer.IP))
		if ip == nil || ip.To4() == nil {
			continue
		}
		target := &net.UDPAddr{IP: ip.To4(), Port: DiscoveryPort}
		for _, dialect := range protocolDialectsForPeer(peer) {
			if err := e.sendDiscovery(target, e.helloMessageForDialect("discover", dialect)); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (e *Engine) probeTCPSubnets(message wireMessage, targets []net.UDPAddr) error {
	const parallelism = 64
	sem := make(chan struct{}, parallelism)
	var wait sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	for index := range targets {
		target := targets[index]
		wait.Add(1)
		go func() {
			defer wait.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			address := net.JoinHostPort(target.IP.String(), fmt.Sprint(DiscoveryPort))
			conn, err := net.DialTimeout("tcp4", address, 150*time.Millisecond)
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
			if err := writeWire(conn, message); err != nil {
				return
			}
			var response wireMessage
			if err := json.NewDecoder(conn).Decode(&response); err != nil || response.Type != "announce" || response.DeviceID == e.identity.DeviceID {
				return
			}
			if _, ok := protocolDialectForMessage(response); !ok {
				return
			}
			if response.IP == "" {
				response.IP = target.IP.String()
			}
			if err := e.handleAnnounce(response); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	wait.Wait()
	return firstErr
}

func (e *Engine) sendDiscovery(addr *net.UDPAddr, message wireMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	e.mu.RLock()
	udp := e.udp
	e.mu.RUnlock()
	if udp == nil {
		return errors.New("discovery_not_started")
	}
	_, err = udp.WriteToUDP(data, addr)
	return err
}

func (e *Engine) helloMessage(kind string) wireMessage {
	return e.helloMessageForDialect(kind, protocolDialects[0])
}

func (e *Engine) helloMessageForDialect(kind string, dialect ProtocolDialect) wireMessage {
	e.mu.RLock()
	identity := e.identity
	e.mu.RUnlock()
	profile := e.Profile()
	capabilities := []string{"text", "image", "file"}
	if dialect.Major >= 2 {
		capabilities = append(capabilities, "file-progress-v1", "avatar-sync-v1", "offline-v1", "friend-restore-v2", "storage-preflight-v1")
	}
	return wireMessage{Magic: dialect.Magic, Type: kind, Protocol: dialect.Name, Major: dialect.Major, Minor: ProtocolMinor, MinMajor: dialect.Major, MinMinor: 0, DeviceID: identity.DeviceID, Nickname: profile.Nickname, AvatarHash: profile.AvatarHash, AvatarVersion: profile.AvatarVersion, Platform: identity.Platform, OSVersion: identity.OSVersion, IP: identity.IP, Port: identity.Port, PublicKey: identity.PublicKeyPEM, CertFP: identity.CertificateFingerprint, Capabilities: capabilities}
}

func (e *Engine) upsertWirePeer(message wireMessage) error {
	return e.upsertWirePeerWithOptions(message)
}

func (e *Engine) upsertWirePeerWithOptions(message wireMessage) error {
	if message.PublicKey != "" && !validDevicePublicKey(message.DeviceID, message.PublicKey) {
		return fmt.Errorf("设备身份校验失败")
	}
	if strings.TrimSpace(message.DeviceID) == "" {
		return fmt.Errorf("设备身份为空")
	}
	peer := Peer{DeviceID: message.DeviceID, Nickname: message.Nickname, AvatarHash: message.AvatarHash, AvatarVersion: message.AvatarVersion, Platform: message.Platform, OSVersion: message.OSVersion, IP: message.IP, Port: message.Port, PublicKeyPEM: message.PublicKey, CertificateFingerprint: message.CertFP, ProtocolName: message.Protocol, ProtocolMajor: message.Major, DiscoveryMagic: message.Magic, Capabilities: message.Capabilities, Relation: DiscoveredState, LastSeen: nowString()}
	if existing, existingErr := e.peer(message.DeviceID); existingErr == nil {
		if existing.PublicKeyPEM != "" && message.PublicKey != "" && !strings.EqualFold(existing.PublicKeyPEM, message.PublicKey) {
			return fmt.Errorf("DEVICE_KEY_CHANGED")
		}
		if existing.CertificateFingerprint != "" && message.CertFP == "" {
			message.CertFP = existing.CertificateFingerprint
		}
		if peer.CertificateFingerprint == "" {
			peer.CertificateFingerprint = existing.CertificateFingerprint
		}
		peer.Relation, peer.Remark, peer.AvatarPath = existing.Relation, existing.Remark, existing.AvatarPath
		if peer.AvatarHash == "" {
			peer.AvatarHash, peer.AvatarVersion = existing.AvatarHash, existing.AvatarVersion
		}
		if peer.ProtocolName == "" || peer.ProtocolMajor == 0 {
			peer.ProtocolName, peer.ProtocolMajor, peer.DiscoveryMagic, peer.Capabilities = existing.ProtocolName, existing.ProtocolMajor, existing.DiscoveryMagic, existing.Capabilities
		} else if len(peer.Capabilities) == 0 {
			peer.Capabilities = append([]string(nil), existing.Capabilities...)
		}
	}
	if err := UpsertPeer(context.Background(), peer); err != nil {
		return err
	}
	e.mu.Lock()
	if old, exists := e.peers[peer.DeviceID]; exists {
		peer.Relation, peer.Remark = old.Relation, old.Remark
		peer.AvatarPath = old.AvatarPath
		if peer.AvatarHash == "" {
			peer.AvatarHash, peer.AvatarVersion = old.AvatarHash, old.AvatarVersion
		}
		if peer.ProtocolName == "" || peer.ProtocolMajor == 0 {
			peer.ProtocolName, peer.ProtocolMajor, peer.DiscoveryMagic, peer.Capabilities = old.ProtocolName, old.ProtocolMajor, old.DiscoveryMagic, old.Capabilities
		} else if len(peer.Capabilities) == 0 {
			peer.Capabilities = append([]string(nil), old.Capabilities...)
		}
	}
	peer.Online = true
	e.peers[peer.DeviceID] = peer
	e.mu.Unlock()
	return nil
}

// handleAnnounce is the only discovery path that may initiate the optional
// friend restoration handshake. TLS health probes use upsertWirePeer directly
// and therefore remain side-effect free.
func (e *Engine) handleAnnounce(message wireMessage) error {
	if !compatibleProtocol(message) {
		return errors.New("PROTOCOL_UNSUPPORTED")
	}
	wasFriend := false
	if existing, err := e.peer(message.DeviceID); err == nil {
		wasFriend = existing.Relation == PeerRelation
	}
	if err := e.upsertWirePeer(message); err != nil {
		return err
	}
	e.emit("chat:peer-updated", e.Peers())
	if wasFriend {
		if peer, err := e.peer(message.DeviceID); err == nil {
			e.maybeSendFriendRestore(peer)
		}
	}
	return nil
}

func compatibleProtocol(message wireMessage) bool {
	_, ok := protocolDialectForMessage(message)
	return ok
}

func (e *Engine) maybeSendFriendRestore(peer Peer) {
	e.mu.Lock()
	last := e.friendRestoreAt[peer.DeviceID]
	if !last.IsZero() && time.Since(last) < 30*time.Second {
		e.mu.Unlock()
		return
	}
	e.friendRestoreAt[peer.DeviceID] = time.Now()
	e.mu.Unlock()
	go func() {
		for _, dialect := range protocolDialectsForPeer(peer) {
			message, err := e.friendRestoreMessageForDialect(peer.DeviceID, dialect)
			if err != nil {
				continue
			}
			if e.sendToPeerWithDialect(peer, message, dialect) == nil {
				return
			}
		}
	}()
}

func validDevicePublicKey(deviceID, publicKeyPEM string) bool {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return false
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false
	}
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return false
	}
	return strings.EqualFold(deviceID, sha256Hex(der))
}

func (e *Engine) friendRequestForDevice(deviceID string) (FriendRequest, bool) {
	requests, err := ListFriendRequests(context.Background(), "")
	if err != nil {
		return FriendRequest{}, false
	}
	for _, request := range requests {
		if request.DeviceID == deviceID {
			return request, true
		}
	}
	return FriendRequest{}, false
}

func earliestAcceptedAt(requests []FriendRequest, deviceID string) string {
	acceptedAt := ""
	for _, request := range requests {
		if request.DeviceID == deviceID && request.AcceptedAt != "" && requestTimeBefore(request.AcceptedAt, acceptedAt) {
			acceptedAt = request.AcceptedAt
		}
		if request.DeviceID == deviceID && acceptedAt == "" && request.AcceptedAt != "" {
			acceptedAt = request.AcceptedAt
		}
	}
	return acceptedAt
}

func (e *Engine) emitFriendRequestUpdate(deviceID string) {
	if request, ok := e.friendRequestForDevice(deviceID); ok {
		e.emit("chat:friend-request-updated", request)
	}
}

func (e *Engine) SendFriendRequest(ctx context.Context, deviceID, message string) (FriendRequest, error) {
	peer, err := e.peer(deviceID)
	if err != nil {
		return FriendRequest{}, err
	}
	if existing, ok := e.friendRequestForDevice(deviceID); ok {
		rows, rowsErr := listFriendRequestRows(ctx, "")
		if rowsErr == nil {
			for _, row := range rows {
				if row.DeviceID == deviceID && row.Direction == "sent" && (row.Status == "sent" || row.Status == "queued" || row.Status == "pending") {
					return existing, nil
				}
			}
		}
		if existing.Status == "accepted" && e.isFriend(deviceID) {
			return existing, nil
		}
	}
	request := FriendRequest{RequestID: newID(), DeviceID: deviceID, Nickname: peer.Nickname, Message: strings.TrimSpace(message), Status: "queued", Direction: "sent", CreatedAt: nowString()}
	if err := SaveFriendRequest(ctx, request); err != nil {
		return FriendRequest{}, err
	}
	if err := e.sendToPeer(peer, wireMessage{Type: "friend_request", RequestID: request.RequestID, Content: request.Message}); err != nil {
		return request, err
	}
	_ = UpdateFriendRequest(ctx, request.RequestID, "sent")
	if aggregated, ok := e.friendRequestForDevice(deviceID); ok {
		e.emit("chat:friend-request-updated", aggregated)
		return aggregated, nil
	}
	request.Status = "sent"
	e.emit("chat:friend-request-updated", request)
	return request, nil
}

func (e *Engine) AcceptFriendRequest(ctx context.Context, requestID string) error {
	requests, err := listFriendRequestRows(ctx, "")
	if err != nil {
		return err
	}
	var target *FriendRequest
	for index := range requests {
		if requests[index].RequestID == requestID {
			target = &requests[index]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("好友申请不存在")
	}
	if err := SetPeerRelation(ctx, target.DeviceID, PeerRelation); err != nil {
		return err
	}
	e.updatePeerRelation(target.DeviceID, PeerRelation)
	acceptedAt := earliestAcceptedAt(requests, target.DeviceID)
	if acceptedAt == "" {
		acceptedAt = nowString()
	}
	if err := UpdateFriendRequestsForDevice(ctx, target.DeviceID, "accepted", acceptedAt); err != nil {
		return err
	}
	if peer, peerErr := e.peer(target.DeviceID); peerErr == nil {
		seen := make(map[string]struct{})
		for _, request := range requests {
			if request.DeviceID != target.DeviceID || request.Status != "pending" || request.Direction == "sent" {
				continue
			}
			if _, exists := seen[request.RequestID]; exists {
				continue
			}
			seen[request.RequestID] = struct{}{}
			_ = e.sendToPeer(peer, wireMessage{Type: "friend_request_response", RequestID: request.RequestID, Status: "accepted", AcceptedAt: acceptedAt})
		}
	}
	e.emitFriendRequestUpdate(target.DeviceID)
	e.emit("chat:peer-updated", e.Peers())
	return nil
}

func (e *Engine) RejectFriendRequest(ctx context.Context, requestID string) error {
	requests, err := listFriendRequestRows(ctx, "")
	if err != nil {
		return err
	}
	var target *FriendRequest
	for index := range requests {
		if requests[index].RequestID == requestID {
			target = &requests[index]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("好友申请不存在")
	}
	if err := UpdateFriendRequestsForDevice(ctx, target.DeviceID, "rejected", ""); err != nil {
		return err
	}
	if peer, peerErr := e.peer(target.DeviceID); peerErr == nil {
		seen := make(map[string]struct{})
		for _, request := range requests {
			if request.DeviceID != target.DeviceID || request.Status != "pending" || request.Direction == "sent" {
				continue
			}
			if _, exists := seen[request.RequestID]; exists {
				continue
			}
			seen[request.RequestID] = struct{}{}
			_ = e.sendToPeer(peer, wireMessage{Type: "friend_request_response", RequestID: request.RequestID, Status: "rejected"})
		}
	}
	e.emitFriendRequestUpdate(target.DeviceID)
	return nil
}

func (e *Engine) SendMessage(ctx context.Context, deviceID, content string) (Message, error) {
	if strings.TrimSpace(content) == "" {
		return Message{}, fmt.Errorf("消息不能为空")
	}
	if !e.isFriend(deviceID) {
		return Message{}, fmt.Errorf("不是好友")
	}
	conversationID, err := EnsureConversation(ctx, deviceID)
	if err != nil {
		return Message{}, err
	}
	message := Message{MessageID: newID(), ConversationID: conversationID, SenderDeviceID: e.identity.DeviceID, Kind: "text", Content: content, Status: "sending", CreatedAt: nowString()}
	if err := SaveMessage(ctx, message); err != nil {
		return Message{}, err
	}
	e.emit("chat:message", message)
	wire := wireMessage{Type: "message", MessageID: message.MessageID, Kind: "text", Content: content}
	peer, err := e.peer(deviceID)
	if err != nil {
		message.Status = "failed"
		_ = UpdateMessageStatus(ctx, message.MessageID, message.Status)
		e.emit("chat:message", message)
		return message, nil
	}
	if err := e.sendToPeer(peer, wire); err != nil {
		message.Status = "failed"
	} else {
		message.Status = "sent"
	}
	_ = exec(ctx, `UPDATE messages SET status=? WHERE message_id=?`, message.Status, message.MessageID)
	e.emit("chat:message", message)
	return message, nil
}

func (e *Engine) RetryMessage(ctx context.Context, messageID string) (Message, error) {
	message, err := GetMessage(ctx, messageID)
	if err != nil {
		return Message{}, err
	}
	e.mu.RLock()
	localDeviceID := e.identity.DeviceID
	e.mu.RUnlock()
	if message.SenderDeviceID != localDeviceID || message.Kind != "text" {
		return Message{}, fmt.Errorf("该消息不支持重发")
	}
	if message.Status == "sent" {
		return message, nil
	}
	if message.Status == "sending" {
		return Message{}, fmt.Errorf("消息正在发送")
	}
	deviceID := strings.TrimPrefix(message.ConversationID, "conv-")
	if !e.isFriend(deviceID) {
		return Message{}, fmt.Errorf("不是好友")
	}
	message.Status = "sending"
	if err := UpdateMessageStatus(ctx, message.MessageID, message.Status); err != nil {
		return Message{}, err
	}
	e.emit("chat:message", message)
	wire := wireMessage{Type: "message", MessageID: message.MessageID, Kind: "text", Content: message.Content}
	peer, err := e.peer(deviceID)
	if err != nil {
		return e.finishTextRetry(ctx, message, "failed"), err
	}
	if err := e.sendToPeer(peer, wire); err != nil {
		return e.finishTextRetry(ctx, message, "failed"), err
	}
	return e.finishTextRetry(ctx, message, "sent"), nil
}

func (e *Engine) finishTextRetry(ctx context.Context, message Message, status string) Message {
	message.Status = status
	_ = UpdateMessageStatus(ctx, message.MessageID, status)
	e.emit("chat:message", message)
	return message
}

func (e *Engine) MarkConversationRead(ctx context.Context, deviceID string) error {
	if !e.isFriend(deviceID) {
		return fmt.Errorf("不是好友")
	}
	conversationID, err := EnsureConversation(ctx, deviceID)
	if err != nil {
		return err
	}
	messages, err := ListMessages(ctx, conversationID)
	if err != nil {
		return err
	}
	readIDs := make([]string, 0)
	for _, message := range messages {
		if message.SenderDeviceID != deviceID || message.Status == "read" {
			continue
		}
		if err := UpdateMessageStatus(ctx, message.MessageID, "read"); err != nil {
			return err
		}
		readIDs = append(readIDs, message.MessageID)
	}
	if len(readIDs) == 0 {
		_ = ClearConversationUnread(ctx, conversationID)
		return nil
	}
	if err := ClearConversationUnread(ctx, conversationID); err != nil {
		return err
	}
	peer, err := e.peer(deviceID)
	if err != nil {
		return err
	}
	return e.sendToPeer(peer, wireMessage{Type: "read_receipt", MessageIDs: readIDs})
}

func (e *Engine) SendFile(ctx context.Context, deviceID, path string) (Message, error) {
	if e.IsAttachmentMigrationActive() {
		return Message{}, fmt.Errorf("附件迁移正在进行")
	}
	if !e.isFriend(deviceID) {
		return Message{}, fmt.Errorf("不是好友")
	}
	path = filepath.Clean(path)
	info, sum, err := inspectTransferFile(path)
	if err != nil {
		return Message{}, err
	}
	fileName := safeFileName(filepath.Base(path))
	conversationID, err := EnsureConversation(ctx, deviceID)
	if err != nil {
		return Message{}, err
	}
	messageID, attachmentID := newID(), newID()
	message := Message{MessageID: messageID, ConversationID: conversationID, SenderDeviceID: e.identity.DeviceID, Kind: "file", Content: fileName, Status: "sending", CreatedAt: nowString(), AttachmentID: attachmentID, AttachmentName: fileName, AttachmentSize: info.Size(), AttachmentMime: mime.TypeByExtension(filepath.Ext(fileName)), AttachmentStatus: "sending", AttachmentPath: path}
	if message.AttachmentMime == "" {
		message.AttachmentMime = "application/octet-stream"
	}
	if err := SaveMessage(ctx, message); err != nil {
		return Message{}, err
	}
	if err := SaveAttachment(ctx, Attachment{AttachmentID: attachmentID, MessageID: messageID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum, LocalPath: path, Status: "sending"}); err != nil {
		return Message{}, err
	}
	e.emit("chat:message", message)
	if err := e.transferFile(ctx, deviceID, message, path, sum); err != nil {
		return e.finishAttachmentSend(ctx, message, "failed"), err
	}
	return e.finishAttachmentSend(ctx, message, "sent"), nil
}

func (e *Engine) RetryAttachment(ctx context.Context, messageID string) (Message, error) {
	if e.IsAttachmentMigrationActive() {
		return Message{}, fmt.Errorf("附件迁移正在进行")
	}
	message, err := GetMessage(ctx, messageID)
	if err != nil {
		return Message{}, err
	}
	e.mu.RLock()
	localDeviceID := e.identity.DeviceID
	e.mu.RUnlock()
	if message.SenderDeviceID != localDeviceID || message.Kind != "file" || message.AttachmentID == "" {
		return Message{}, fmt.Errorf("该消息不支持重发")
	}
	if message.Status == "sent" {
		return message, nil
	}
	if message.Status == "sending" {
		return Message{}, fmt.Errorf("文件正在发送")
	}
	if !e.isFriend(strings.TrimPrefix(message.ConversationID, "conv-")) {
		return Message{}, fmt.Errorf("不是好友")
	}
	info, sum, err := inspectTransferFile(message.AttachmentPath)
	if err != nil {
		return Message{}, err
	}
	attachment, attachmentErr := GetAttachment(ctx, message.AttachmentID)
	if attachmentErr != nil {
		return Message{}, attachmentErr
	}
	// The original checksum and size are authoritative. A changed source file
	// must be selected again instead of sending different bytes under the same ID.
	if info.Size() != message.AttachmentSize || (attachment.SHA256 != "" && attachment.SHA256 != sum) {
		return Message{}, fmt.Errorf("原文件内容已变化，请重新选择文件")
	}
	message.Status, message.AttachmentStatus = "sending", "sending"
	if err := UpdateMessageStatus(ctx, message.MessageID, message.Status); err != nil {
		return Message{}, err
	}
	if err := SaveAttachment(ctx, Attachment{AttachmentID: message.AttachmentID, MessageID: message.MessageID, FileName: message.AttachmentName, MimeType: message.AttachmentMime, FileSize: message.AttachmentSize, SHA256: sum, LocalPath: message.AttachmentPath, Status: "sending"}); err != nil {
		return Message{}, err
	}
	e.emit("chat:message", message)
	if err := e.transferFile(ctx, strings.TrimPrefix(message.ConversationID, "conv-"), message, message.AttachmentPath, sum); err != nil {
		return e.finishAttachmentSend(ctx, message, "failed"), err
	}
	return e.finishAttachmentSend(ctx, message, "sent"), nil
}

func inspectTransferFile(path string) (os.FileInfo, string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, "", fmt.Errorf("文件路径为空")
	}
	info, err := os.Stat(filepath.Clean(path))
	if err != nil || info.IsDir() {
		return nil, "", fmt.Errorf("文件不存在")
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, "", err
	}
	return info, hex.EncodeToString(hash.Sum(nil)), nil
}

func (e *Engine) transferFile(ctx context.Context, deviceID string, message Message, path, sum string) error {
	peer, err := e.peer(deviceID)
	if err != nil {
		return err
	}
	var lastErr error
	for _, dialect := range protocolDialectsForPeer(peer) {
		if err := e.transferFileWithDialect(ctx, peer, message, path, sum, dialect); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (e *Engine) transferFileWithDialect(ctx context.Context, peer Peer, message Message, path, sum string, dialect ProtocolDialect) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer file.Close()
	clientTLS, err := e.clientTLSConfig()
	if err != nil {
		return err
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), clientTLS)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := verifyPeerCertificate(conn, peer); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	if err := writeWire(conn, e.helloMessageForDialect("hello", dialect)); err != nil {
		return err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("对方握手失败")
	}
	if response.Type == "error" {
		return fmt.Errorf("对方握手失败: %s", response.Status)
	}
	if response.Type != "hello_ack" {
		return fmt.Errorf("对方握手失败")
	}
	responseDialect, responseCompatible := protocolDialectForMessage(response)
	if !responseCompatible {
		return fmt.Errorf("对方握手协议不兼容")
	}
	e.rememberPeerDialect(peer.DeviceID, responseDialect, response.Capabilities)
	supportsProgress := hasCapability(response.Capabilities, "file-progress-v1") && responseDialect.Major >= 2
	supportsPreflight := hasCapability(response.Capabilities, "storage-preflight-v1") && responseDialect.Major >= 2
	e.touchPeer(peer.DeviceID)
	if !peer.Online {
		e.emit("chat:peer-updated", e.Peers())
	}
	if err := e.writeFriendRestoreIfNeeded(conn, peer, responseDialect); err != nil {
		return err
	}
	if err := writeWire(conn, wireMessage{Type: "file_offer", MessageID: message.MessageID, AttachmentID: message.AttachmentID, FileName: message.AttachmentName, MimeType: message.AttachmentMime, FileSize: message.AttachmentSize, SHA256: sum}); err != nil {
		return err
	}
	if supportsPreflight {
		offerResponse, err := readFileOfferResponse(decoder, message.AttachmentID)
		if err != nil {
			return err
		}
		if offerResponse.Status != "accepted" {
			if offerResponse.Reason == "INSUFFICIENT_STORAGE" {
				return fmt.Errorf("对方设备存储空间不足（可用 %s，需要 %s）", formatBytes(offerResponse.AvailableBytes), formatBytes(offerResponse.RequiredBytes))
			}
			return fmt.Errorf("对方拒绝接收文件: %s", offerResponse.Reason)
		}
	}
	e.emitTransferProgress(message.MessageID, message.AttachmentID, peer.DeviceID, 0, message.AttachmentSize, "send", "transferring")
	buffer := make([]byte, 32*1024)
	index := 0
	var sent, lastProgress int64
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			if err := writeWire(conn, wireMessage{Type: "file_chunk", AttachmentID: message.AttachmentID, ChunkIndex: index, Payload: base64.StdEncoding.EncodeToString(buffer[:n])}); err != nil {
				return err
			}
			index++
			sent += int64(n)
			if sent-lastProgress >= 256*1024 || sent == message.AttachmentSize {
				e.emitTransferProgress(message.MessageID, message.AttachmentID, peer.DeviceID, sent, message.AttachmentSize, "send", "transferring")
				lastProgress = sent
			}
			if supportsProgress {
				progress, err := readFileProgress(decoder, message.AttachmentID)
				if err != nil {
					return err
				}
				phase := "receiving"
				if progress.Status == "failed" {
					phase = "failed"
				}
				e.emitTransferProgress(message.MessageID, message.AttachmentID, peer.DeviceID, progress.Transferred, message.AttachmentSize, "remote-receive", phase)
				if progress.Status == "failed" {
					return fmt.Errorf("对方接收文件失败")
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := writeWire(conn, wireMessage{Type: "file_complete", MessageID: message.MessageID, AttachmentID: message.AttachmentID, FileSize: message.AttachmentSize}); err != nil {
		return err
	}
	if supportsProgress {
		progress, err := readFileProgress(decoder, message.AttachmentID)
		if err != nil {
			return err
		}
		phase := "completed"
		if progress.Status == "failed" {
			phase = "failed"
		}
		e.emitTransferProgress(message.MessageID, message.AttachmentID, peer.DeviceID, progress.Transferred, message.AttachmentSize, "remote-receive", phase)
		if progress.Status == "failed" {
			return fmt.Errorf("对方接收文件失败")
		}
	}
	return nil
}

func hasCapability(capabilities []string, expected string) bool {
	for _, capability := range capabilities {
		if capability == expected {
			return true
		}
	}
	return false
}

func readFileProgress(decoder *json.Decoder, attachmentID string) (wireMessage, error) {
	for {
		var progress wireMessage
		if err := decoder.Decode(&progress); err != nil {
			return wireMessage{}, err
		}
		// A friend restore acknowledgement may be queued before the first file
		// progress frame. It is a separate optional control message and should
		// not interrupt the attachment transfer.
		if progress.Type == "friend_restore_ack" {
			continue
		}
		if progress.Type != "file_progress" || progress.AttachmentID != attachmentID {
			return wireMessage{}, fmt.Errorf("文件进度回执无效")
		}
		return progress, nil
	}
}

func readFileOfferResponse(decoder *json.Decoder, attachmentID string) (wireMessage, error) {
	for {
		var response wireMessage
		if err := decoder.Decode(&response); err != nil {
			return wireMessage{}, err
		}
		if response.Type == "friend_restore_ack" {
			continue
		}
		if response.Type != "file_offer_response" || response.AttachmentID != attachmentID {
			return wireMessage{}, fmt.Errorf("文件接收预检查回执无效")
		}
		return response, nil
	}
}

func (e *Engine) finishAttachmentSend(ctx context.Context, message Message, status string) Message {
	message.Status, message.AttachmentStatus = status, status
	_ = UpdateMessageStatus(ctx, message.MessageID, status)
	_ = SaveAttachment(ctx, Attachment{AttachmentID: message.AttachmentID, MessageID: message.MessageID, FileName: message.AttachmentName, MimeType: message.AttachmentMime, FileSize: message.AttachmentSize, SHA256: messageAttachmentSHA(ctx, message), LocalPath: message.AttachmentPath, Status: status})
	e.emit("chat:message", message)
	return message
}

func messageAttachmentSHA(ctx context.Context, message Message) string {
	if message.AttachmentID == "" {
		return ""
	}
	attachment, err := GetAttachment(ctx, message.AttachmentID)
	if err != nil {
		return ""
	}
	return attachment.SHA256
}

func (e *Engine) sendToPeer(peer Peer, message wireMessage) error {
	var lastErr error
	for _, dialect := range protocolDialectsForPeer(peer) {
		if err := e.sendToPeerWithDialect(peer, message, dialect); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("对方握手失败")
}

func (e *Engine) sendToPeerWithDialect(peer Peer, message wireMessage, dialect ProtocolDialect) error {
	if peer.IP == "" || peer.Port == 0 {
		return fmt.Errorf("好友地址不可用")
	}
	clientTLS, err := e.clientTLSConfig()
	if err != nil {
		return err
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), clientTLS)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := verifyPeerCertificate(conn, peer); err != nil {
		return err
	}
	if err := writeWire(conn, e.helloMessageForDialect("hello", dialect)); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("对方握手失败")
	}
	if response.Type == "error" {
		return fmt.Errorf("对方握手失败: %s", response.Status)
	}
	if response.Type != "hello_ack" {
		return fmt.Errorf("对方握手失败")
	}
	responseDialect, responseCompatible := protocolDialectForMessage(response)
	if !responseCompatible {
		return fmt.Errorf("对方握手协议不兼容")
	}
	e.rememberPeerDialect(peer.DeviceID, responseDialect, response.Capabilities)
	e.touchPeer(peer.DeviceID)
	if !peer.Online {
		e.emit("chat:peer-updated", e.Peers())
	}
	// A known friend may have reinstalled FlyQPro and lost its local database.
	// Restore the relationship over this already authenticated connection before
	// sending the message. The optional control message is only sent after the canonical dzhgo/v2 handshake.
	if message.Type != "friend_restore" {
		if err := e.writeFriendRestoreIfNeeded(conn, peer, responseDialect); err != nil {
			return err
		}
	}
	if peer.Relation == PeerRelation && peer.AvatarHash != "" && hasCapability(response.Capabilities, "avatar-sync-v1") && !cachedAvatarMatches(peer) {
		_ = conn.SetReadDeadline(time.Now().Add(700 * time.Millisecond))
		if err := writeWire(conn, wireMessage{Type: "avatar_request"}); err == nil {
			var avatar wireMessage
			if decoder.Decode(&avatar) == nil && avatar.Type == "avatar_response" {
				e.handleWire(conn, wireMessage{DeviceID: peer.DeviceID}, avatar)
			}
		}
		_ = conn.SetReadDeadline(time.Time{})
	}
	if err := writeWire(conn, message); err != nil {
		return err
	}
	if message.Type == "message" {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			var ack wireMessage
			if err := decoder.Decode(&ack); err != nil {
				return err
			}
			if ack.Type == "friend_restore_ack" {
				continue
			}
			if ack.Type == "error" {
				return fmt.Errorf("%s", ack.Status)
			}
			if ack.Type != "ack" || ack.MessageID != message.MessageID || ack.Status != "sent" {
				return fmt.Errorf("消息确认无效")
			}
			break
		}
	}
	return nil
}

// writeFriendRestoreIfNeeded keeps the relationship recoverable after the
// remote app was reinstalled. Signing may be unavailable on a device whose
// secure key has been reset; in that case the normal message path still gets
// a chance to report the real transport result.
func (e *Engine) writeFriendRestoreIfNeeded(conn net.Conn, peer Peer, dialect ProtocolDialect) error {
	if peer.Relation != PeerRelation {
		return nil
	}
	restore, err := e.friendRestoreMessageForDialect(peer.DeviceID, dialect)
	if err != nil {
		return nil
	}
	return writeWire(conn, restore)
}

func (e *Engine) touchPeer(deviceID string) {
	seen := nowString()
	_ = exec(context.Background(), `UPDATE peers SET last_seen=?, updated_at=? WHERE device_id=?`, seen, seen, deviceID)
	e.mu.Lock()
	if peer, ok := e.peers[deviceID]; ok {
		peer.LastSeen = seen
		peer.Online = true
		e.peers[deviceID] = peer
	}
	e.mu.Unlock()
}

func (e *Engine) rememberPeerDialect(deviceID string, dialect ProtocolDialect, capabilities []string) {
	_ = SetPeerProtocol(context.Background(), deviceID, dialect, capabilities)
	e.mu.Lock()
	if peer, ok := e.peers[deviceID]; ok {
		peer.ProtocolName, peer.ProtocolMajor, peer.DiscoveryMagic, peer.Capabilities = dialect.Name, dialect.Major, dialect.Magic, append([]string(nil), capabilities...)
		e.peers[deviceID] = peer
	}
	e.mu.Unlock()
}

func (e *Engine) setPeerOnline(deviceID string, online bool) bool {
	e.mu.Lock()
	peer, ok := e.peers[deviceID]
	if !ok || peer.Online == online {
		e.mu.Unlock()
		return false
	}
	peer.Online = online
	e.peers[deviceID] = peer
	e.mu.Unlock()
	return true
}

func (e *Engine) handleOffline(deviceID string) {
	peer, err := e.peer(deviceID)
	if err != nil {
		return
	}
	if peer.Relation != PeerRelation {
		e.forgetDiscoveredPeer(deviceID)
		return
	}
	if e.setPeerOnline(deviceID, false) {
		e.emit("chat:peer-updated", e.Peers())
	}
}

func (e *Engine) probePeer(peer Peer) error {
	var lastErr error
	for _, dialect := range protocolDialectsForPeer(peer) {
		if err := e.probePeerWithDialect(peer, dialect); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (e *Engine) probePeerWithDialect(peer Peer, dialect ProtocolDialect) error {
	if peer.IP == "" || peer.Port == 0 {
		return fmt.Errorf("好友地址不可用")
	}
	clientTLS, err := e.clientTLSConfig()
	if err != nil {
		return err
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), clientTLS)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if err := verifyPeerCertificate(conn, peer); err != nil {
		return err
	}
	hello := e.helloMessageForDialect("hello", dialect)
	hello.Probe = true
	if err := writeWire(conn, hello); err != nil {
		return err
	}
	var response wireMessage
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return err
	}
	if response.Type == "error" {
		return fmt.Errorf("%s", response.Status)
	}
	if response.Type != "hello_ack" {
		return fmt.Errorf("对方握手失败")
	}
	responseDialect, responseCompatible := protocolDialectForMessage(response)
	if !responseCompatible {
		return fmt.Errorf("对方握手协议不兼容")
	}
	e.rememberPeerDialect(peer.DeviceID, responseDialect, response.Capabilities)
	e.touchPeer(peer.DeviceID)
	return nil
}

func (e *Engine) livenessLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.probeKnownPeers()
		case <-e.stop:
			return
		}
	}
}

func (e *Engine) probeKnownPeers() {
	peers := e.Peers()
	changed := false
	var changedMu sync.Mutex
	var wait sync.WaitGroup
	for _, peer := range peers {
		peer := peer
		wait.Add(1)
		go func() {
			defer wait.Done()
			online := e.probePeer(peer) == nil
			// probePeer updates lastSeen (and may optimistically mark the in-memory
			// peer online) before returning. Compare with the snapshot used for the
			// probe so an offline peer loaded from SQLite still produces the online
			// event that the frontend needs after a successful probe.
			stateChanged := peer.Online != online
			e.setPeerOnline(peer.DeviceID, online)
			if stateChanged {
				changedMu.Lock()
				changed = true
				changedMu.Unlock()
			}
		}()
	}
	wait.Wait()
	if changed {
		e.emit("chat:peer-updated", e.Peers())
	}
}

func (e *Engine) clientTLSConfig() (*tls.Config, error) {
	e.mu.RLock()
	identity := e.identity
	e.mu.RUnlock()
	certificate, err := identity.TLSCertificate()
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, nil
}

func cachedAvatarMatches(peer Peer) bool {
	if peer.AvatarPath == "" || peer.AvatarHash == "" {
		return false
	}
	data, err := os.ReadFile(peer.AvatarPath)
	return err == nil && sha256Hex(data) == peer.AvatarHash
}

func verifyPeerCertificate(conn *tls.Conn, peer Peer) error {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("对方没有提供证书")
	}
	certificate := state.PeerCertificates[0]
	actual := sha256Hex(certificate.Raw)
	if peer.PublicKeyPEM != "" {
		block, _ := pem.Decode([]byte(peer.PublicKeyPEM))
		if block == nil {
			return fmt.Errorf("设备公钥无效")
		}
		expected, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("设备公钥无效")
		}
		expectedDER, err := x509.MarshalPKIXPublicKey(expected)
		if err != nil {
			return err
		}
		actualDER, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
		if err != nil || !strings.EqualFold(hex.EncodeToString(actualDER), hex.EncodeToString(expectedDER)) {
			return fmt.Errorf("设备身份不匹配")
		}
		return nil
	}
	if peer.CertificateFingerprint != "" && !strings.EqualFold(actual, peer.CertificateFingerprint) {
		return fmt.Errorf("CERTIFICATE_CHANGED")
	}
	return nil
}

func (e *Engine) peer(deviceID string) (Peer, error) {
	e.mu.RLock()
	peer, ok := e.peers[deviceID]
	e.mu.RUnlock()
	if ok {
		return peer, nil
	}
	peers, err := ListPeers(context.Background(), "")
	if err != nil {
		return Peer{}, err
	}
	for _, item := range peers {
		if item.DeviceID == deviceID {
			e.mu.Lock()
			e.peers[deviceID] = item
			e.mu.Unlock()
			return item, nil
		}
	}
	return Peer{}, fmt.Errorf("设备不存在")
}

func (e *Engine) isFriend(deviceID string) bool {
	peer, err := e.peer(deviceID)
	return err == nil && peer.Relation == PeerRelation
}

// canRespondToDiscovery is the privacy boundary for the discovery protocol.
// A friend is addressable even when the local device is not generally
// discoverable; an unknown device must opt in through the profile setting.
func (e *Engine) canRespondToDiscovery(deviceID string) bool {
	return e.Profile().Discoverable || e.isFriend(deviceID)
}

// canAcceptPeerConnection also permits the direct response to a friend
// request. This keeps the request/accept flow working when the requester has
// discovery disabled, without making the requester generally discoverable.
func (e *Engine) canAcceptPeerConnection(deviceID string) bool {
	if e.canRespondToDiscovery(deviceID) {
		return true
	}
	requests, err := listFriendRequestRows(context.Background(), "")
	if err != nil {
		return false
	}
	for _, request := range requests {
		if request.DeviceID == deviceID && (request.Status == "pending" || request.Status == "sent" || request.Status == "queued") {
			return true
		}
	}
	return false
}

func (e *Engine) updatePeerRelation(deviceID, relation string) {
	e.mu.Lock()
	if peer, ok := e.peers[deviceID]; ok {
		peer.Relation = relation
		e.peers[deviceID] = peer
	}
	e.mu.Unlock()
}
func (e *Engine) Profile() Profile { e.mu.RLock(); defer e.mu.RUnlock(); return e.profile }
func (e *Engine) DeviceInfo() DeviceInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	info := e.identity.DeviceInfo
	info.ProtocolName = ProtocolName
	info.ProtocolMajor = ProtocolMajor
	return info
}

func (e *Engine) UpdateProfile(profile Profile) {
	e.mu.Lock()
	wasDiscoverable := e.profile.Discoverable
	e.profile = profile
	e.mu.Unlock()
	if wasDiscoverable && !profile.Discoverable {
		go e.broadcastWithdrawal()
	}
	e.emit("chat:profile-updated", profile)
}

func (e *Engine) broadcastWithdrawal() {
	e.broadcastPresence("withdraw")
}

func (e *Engine) broadcastPresence(kind string) {
	targets := broadcastAddresses()
	targets = append(targets, localSubnetTargets()...)
	for _, dialect := range protocolDialects {
		if kind == "offline" && dialect.Major < 2 {
			continue
		}
		message := e.helloMessageForDialect(kind, dialect)
		for index := range targets {
			_ = e.sendDiscovery(&targets[index], message)
		}
	}
}

func (e *Engine) forgetDiscoveredPeer(deviceID string) {
	if strings.TrimSpace(deviceID) == "" {
		return
	}
	peer, err := e.peer(deviceID)
	if err != nil || peer.Relation == PeerRelation {
		return
	}
	_ = exec(context.Background(), `DELETE FROM peers WHERE device_id=? AND relation=?`, deviceID, DiscoveredState)
	e.mu.Lock()
	delete(e.peers, deviceID)
	e.mu.Unlock()
	e.emit("chat:peer-updated", e.Peers())
}

func (e *Engine) Peers() []Peer {
	peers, _ := ListPeers(context.Background(), "")
	e.mu.RLock()
	serviceStopped := e.serviceStopped
	onlineStates := make(map[string]bool, len(e.peers))
	for deviceID, peer := range e.peers {
		onlineStates[deviceID] = peer.Online
	}
	e.mu.RUnlock()
	for index := range peers {
		if serviceStopped {
			peers[index].Online = false
		} else if online, ok := onlineStates[peers[index].DeviceID]; ok {
			// Immediate online/offline transitions live in memory; lastSeen
			// remains persisted for restart recovery and display.
			peers[index].Online = online
		}
		if peers[index].AvatarPath == "" {
			continue
		}
		data, err := os.ReadFile(peers[index].AvatarPath)
		if err != nil || len(data) == 0 || len(data) > 5*1024*1024 {
			continue
		}
		mimeType := mime.TypeByExtension(filepath.Ext(peers[index].AvatarPath))
		if mimeType == "" {
			mimeType = "image/png"
		}
		peers[index].AvatarData = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return peers
}

func (e *Engine) PeersByRelation(relation string) []Peer {
	peers := e.Peers()
	filtered := make([]Peer, 0, len(peers))
	for _, peer := range peers {
		if peer.Relation == relation {
			filtered = append(filtered, peer)
		}
	}
	return filtered
}

func (e *Engine) NetworkStatus() NetworkStatus {
	interfaces, _ := net.Interfaces()
	names := make([]string, 0, len(interfaces))
	ips := make([]string, 0)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		names = append(names, iface.Name)
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			if strings.Contains(address.String(), ".") {
				ips = append(ips, strings.Split(address.String(), "/")[0])
			}
		}
	}
	peers := e.Peers()
	online := 0
	for _, peer := range peers {
		if peer.Online {
			online++
		}
	}
	status := "normal"
	if len(ips) == 0 {
		status = "warning"
	}
	e.mu.RLock()
	chatPort := e.identity.Port
	lastScan := e.lastScan
	lastErr := e.lastErr
	e.mu.RUnlock()
	lastScanAt := ""
	if !lastScan.IsZero() {
		lastScanAt = lastScan.UTC().Format(time.RFC3339Nano)
	}
	return NetworkStatus{Status: status, Interfaces: names, LocalIPs: ips, DiscoveryPort: DiscoveryPort, ChatPort: chatPort, PeerCount: len(peers), OnlineCount: online, LastScanAt: lastScanAt, LastError: lastErr}
}

func (e *Engine) emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

func writeWire(writer io.Writer, message wireMessage) error {
	return json.NewEncoder(writer).Encode(message)
}
func readWire(reader io.Reader, message *wireMessage) error {
	return json.NewDecoder(reader).Decode(message)
}

func broadcastAddresses() []net.UDPAddr {
	interfaces, _ := net.Interfaces()
	result := make([]net.UDPAddr, 0)
	seen := make(map[string]struct{})
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ipNet, ok := address.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ip := ipNet.IP.To4()
			mask := ipNet.Mask
			if len(mask) != 4 {
				continue
			}
			broadcast := net.IPv4(ip[0]|^mask[0], ip[1]|^mask[1], ip[2]|^mask[2], ip[3]|^mask[3])
			key := broadcast.String()
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				result = append(result, net.UDPAddr{IP: broadcast, Port: DiscoveryPort})
			}
		}
	}
	return result
}

// localSubnetTargets adds a bounded unicast fallback for networks that block
// UDP broadcasts, such as phone hotspots and restrictive Wi-Fi access points.
func localSubnetTargets() []net.UDPAddr {
	interfaces, _ := net.Interfaces()
	result := make([]net.UDPAddr, 0)
	seen := make(map[string]struct{})
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ipNet, ok := address.(*net.IPNet)
			if !ok {
				continue
			}
			for _, target := range subnetHostTargets(ipNet) {
				key := target.String()
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				result = append(result, net.UDPAddr{IP: target, Port: DiscoveryPort})
			}
		}
	}
	return result
}

func subnetHostTargets(ipNet *net.IPNet) []net.IP {
	ip := ipNet.IP.To4()
	mask := ipNet.Mask
	ones, bits := mask.Size()
	if ip == nil || bits != 32 || ones < 24 || ones > 30 {
		return nil
	}
	hosts := 1 << (bits - ones)
	if hosts > 256 {
		return nil
	}

	network := ip.Mask(mask).To4()
	result := make([]net.IP, 0, hosts-2)
	for host := 1; host < hosts-1; host++ {
		target := net.IPv4(network[0]|byte(host>>24), network[1]|byte(host>>16), network[2]|byte(host>>8), network[3]|byte(host))
		if !target.Equal(ip) {
			result = append(result, target)
		}
	}
	return result
}

func newID() string { return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomToken()) }
func randomToken() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())))
	return hex.EncodeToString(sum[:8])
}

func safeFileName(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.Map(func(char rune) rune {
		if char < 0x20 || strings.ContainsRune(`<>:"/\\|?*`, char) {
			return '_'
		}
		return char
	}, value)
	value = strings.Trim(value, " .")
	if value == "" || value == "." || value == ".." {
		return "attachment"
	}
	return value
}
