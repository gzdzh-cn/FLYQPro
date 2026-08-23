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

type incomingFile struct {
	file      *os.File
	tempPath  string
	messageID string
	senderID  string
	fileName  string
	expected  int64
	received  int64
	sha256    string
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
	for _, peer := range e.Peers() {
		if peer.Relation == PeerRelation {
			go e.retryOutbox(peer)
		}
	}
	go e.acceptLoop()
	go e.discoveryTCPLoop()
	go e.discoveryLoop()
	go e.scanLoop()
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
	close(e.stop)
	_ = e.listener.Close()
	_ = e.discoveryTCP.Close()
	_ = e.udp.Close()
	for attachmentID, transfer := range e.incoming {
		_ = transfer.file.Close()
		delete(e.incoming, attachmentID)
	}
	done := e.done
	e.started = false
	e.serviceStopped = true
	e.mu.Unlock()
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
	if err := json.NewDecoder(conn).Decode(&message); err != nil || message.Magic != DiscoveryMagic || message.Type != "discover" || message.DeviceID == e.identity.DeviceID {
		return
	}
	if e.Profile().Discoverable {
		_ = writeWire(conn, e.helloMessage("announce"))
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
		return
	}
	if err := e.upsertWirePeer(hello); err != nil {
		_ = writeWire(conn, wireMessage{Type: "error", Status: err.Error()})
		return
	}
	e.emit("chat:peer-updated", e.Peers())
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
	_ = writeWire(conn, e.helloMessage("hello_ack"))
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
		_ = writeWire(conn, wireMessage{Type: "pong", Protocol: ProtocolName, Major: ProtocolMajor, Minor: ProtocolMinor})
	case "friend_request":
		// A known peer may send a request even when discovery is disabled. The
		// discoverable flag controls presence broadcasts, not direct requests
		// received over an already authenticated connection.
		request := FriendRequest{RequestID: message.RequestID, DeviceID: hello.DeviceID, Nickname: hello.Nickname, Message: message.Content, Status: "pending", CreatedAt: nowString()}
		if err := SaveFriendRequest(context.Background(), request); err == nil {
			e.emit("chat:friend-request", request)
		}
	case "friend_request_response":
		status := message.Status
		if status == "accepted" {
			if err := SetPeerRelation(context.Background(), hello.DeviceID, PeerRelation); err == nil {
				e.updatePeerRelation(hello.DeviceID, PeerRelation)
			}
		}
		_ = UpdateFriendRequest(context.Background(), message.RequestID, status)
		e.emit("chat:friend-request-updated", map[string]any{"requestId": message.RequestID, "status": status, "deviceId": hello.DeviceID})
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
		if exists, existsErr := MessageExists(context.Background(), message.MessageID); existsErr != nil || exists {
			return
		}
		tempPath := filepath.Join(AppDataDir(), "temp", attachmentID+".part")
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		messageRecord := Message{MessageID: message.MessageID, ConversationID: conversationID, SenderDeviceID: hello.DeviceID, Kind: "file", Content: message.FileName, Status: "receiving", CreatedAt: nowString(), AttachmentID: attachmentID, AttachmentName: message.FileName, AttachmentSize: message.FileSize, AttachmentMime: message.MimeType, AttachmentStatus: "receiving"}
		if err := SaveMessage(context.Background(), messageRecord); err != nil {
			_ = file.Close()
			return
		}
		_ = IncrementConversationUnread(context.Background(), conversationID)
		_ = SaveAttachment(context.Background(), Attachment{AttachmentID: attachmentID, MessageID: message.MessageID, FileName: message.FileName, MimeType: message.MimeType, FileSize: message.FileSize, SHA256: message.SHA256, LocalPath: tempPath, Status: "receiving"})
		e.mu.Lock()
		e.incoming[attachmentID] = &incomingFile{file: file, tempPath: tempPath, messageID: message.MessageID, senderID: hello.DeviceID, fileName: message.FileName, expected: message.FileSize, sha256: message.SHA256}
		e.mu.Unlock()
		e.emit("chat:message", messageRecord)
		e.emit("chat:attachment", messageRecord)
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
		if _, err := transfer.file.Write(data); err != nil {
			return
		}
		transfer.received += int64(len(data))
		e.emit("chat:transfer-progress", map[string]any{"attachmentId": message.AttachmentID, "received": transfer.received, "total": transfer.expected})
	case "file_complete":
		e.finishIncomingFile(message.AttachmentID)
	}
}

func (e *Engine) finishIncomingFile(attachmentID string) {
	e.mu.Lock()
	transfer := e.incoming[attachmentID]
	delete(e.incoming, attachmentID)
	e.mu.Unlock()
	if transfer == nil {
		return
	}
	_ = transfer.file.Close()
	data, err := os.ReadFile(transfer.tempPath)
	if err != nil {
		return
	}
	valid := sha256Hex(data) == transfer.sha256
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
	}
	attachmentMime := mime.TypeByExtension(filepath.Ext(transfer.fileName))
	if attachmentMime == "" {
		attachmentMime = "application/octet-stream"
	}
	_ = SaveAttachment(context.Background(), Attachment{AttachmentID: attachmentID, MessageID: transfer.messageID, FileName: transfer.fileName, MimeType: attachmentMime, FileSize: transfer.expected, SHA256: transfer.sha256, LocalPath: localPath, Status: status})
	messageStatus := "sent"
	if !valid {
		messageStatus = "failed"
	}
	_ = exec(context.Background(), `UPDATE messages SET status=? WHERE message_id=?`, messageStatus, transfer.messageID)
	e.emit("chat:attachment", map[string]any{"attachmentId": attachmentID, "messageId": transfer.messageID, "fileName": transfer.fileName, "status": status, "localPath": localPath, "valid": valid})
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
		if json.Unmarshal(buffer[:n], &message) != nil || message.Magic != DiscoveryMagic || message.DeviceID == e.identity.DeviceID {
			continue
		}
		switch message.Type {
		case "discover":
			if e.Profile().Discoverable {
				_ = e.sendDiscovery(addr, e.helloMessage("announce"))
			}
		case "announce":
			message.IP = addr.IP.String()
			if err := e.upsertWirePeer(message); err == nil {
				e.emit("chat:peer-updated", e.Peers())
			}
		case "withdraw":
			e.forgetDiscoveredPeer(message.DeviceID)
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
	message := e.helloMessage("discover")
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
			if err := json.NewDecoder(conn).Decode(&response); err != nil || response.Magic != DiscoveryMagic || response.Type != "announce" || response.DeviceID == e.identity.DeviceID {
				return
			}
			if response.IP == "" {
				response.IP = target.IP.String()
			}
			if err := e.upsertWirePeer(response); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			} else {
				e.emit("chat:peer-updated", e.Peers())
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
	e.mu.RLock()
	identity := e.identity
	e.mu.RUnlock()
	profile := e.Profile()
	return wireMessage{Magic: DiscoveryMagic, Type: kind, Protocol: ProtocolName, Major: ProtocolMajor, Minor: ProtocolMinor, MinMajor: ProtocolMajor, MinMinor: 0, DeviceID: identity.DeviceID, Nickname: profile.Nickname, AvatarHash: profile.AvatarHash, AvatarVersion: profile.AvatarVersion, Platform: identity.Platform, OSVersion: identity.OSVersion, IP: identity.IP, Port: identity.Port, PublicKey: identity.PublicKeyPEM, CertFP: identity.CertificateFingerprint, Capabilities: []string{"text", "image", "file"}}
}

func (e *Engine) upsertWirePeer(message wireMessage) error {
	if message.PublicKey != "" && !validDevicePublicKey(message.DeviceID, message.PublicKey) {
		return fmt.Errorf("设备身份校验失败")
	}
	if strings.TrimSpace(message.DeviceID) == "" {
		return fmt.Errorf("设备身份为空")
	}
	peer := Peer{DeviceID: message.DeviceID, Nickname: message.Nickname, AvatarHash: message.AvatarHash, AvatarVersion: message.AvatarVersion, Platform: message.Platform, OSVersion: message.OSVersion, IP: message.IP, Port: message.Port, PublicKeyPEM: message.PublicKey, CertificateFingerprint: message.CertFP, Relation: DiscoveredState, LastSeen: nowString()}
	wasFriend := false
	if existing, existingErr := e.peer(message.DeviceID); existingErr == nil {
		wasFriend = existing.Relation == PeerRelation
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
	}
	peer.Online = true
	e.peers[peer.DeviceID] = peer
	e.mu.Unlock()
	go e.retryOutbox(peer)
	if wasFriend {
		e.maybeSendFriendRestore(peer)
	}
	return nil
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
		message, err := e.friendRestoreMessage(peer.DeviceID)
		if err != nil {
			return
		}
		_ = e.sendToPeer(peer, message)
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

func (e *Engine) SendFriendRequest(ctx context.Context, deviceID, message string) (FriendRequest, error) {
	peer, err := e.peer(deviceID)
	if err != nil {
		return FriendRequest{}, err
	}
	request := FriendRequest{RequestID: newID(), DeviceID: deviceID, Message: strings.TrimSpace(message), Status: "queued", CreatedAt: nowString()}
	if err := SaveFriendRequest(ctx, request); err != nil {
		return FriendRequest{}, err
	}
	if err := e.sendToPeer(peer, wireMessage{Type: "friend_request", RequestID: request.RequestID, Content: request.Message}); err != nil {
		return request, err
	}
	_ = UpdateFriendRequest(ctx, request.RequestID, "sent")
	request.Status = "sent"
	e.emit("chat:friend-request-updated", request)
	return request, nil
}

func (e *Engine) AcceptFriendRequest(ctx context.Context, requestID string) error {
	requests, err := ListFriendRequests(ctx, "pending")
	if err != nil {
		return err
	}
	for _, request := range requests {
		if request.RequestID != requestID {
			continue
		}
		if err := SetPeerRelation(ctx, request.DeviceID, PeerRelation); err != nil {
			return err
		}
		e.updatePeerRelation(request.DeviceID, PeerRelation)
		_ = UpdateFriendRequest(ctx, requestID, "accepted")
		if peer, peerErr := e.peer(request.DeviceID); peerErr == nil {
			_ = e.sendToPeer(peer, wireMessage{Type: "friend_request_response", RequestID: requestID, Status: "accepted"})
		}
		e.emit("chat:peer-updated", e.Peers())
		return nil
	}
	return fmt.Errorf("好友申请不存在")
}

func (e *Engine) RejectFriendRequest(ctx context.Context, requestID string) error {
	requests, err := ListFriendRequests(ctx, "pending")
	if err != nil {
		return err
	}
	for _, request := range requests {
		if request.RequestID != requestID {
			continue
		}
		_ = UpdateFriendRequest(ctx, requestID, "rejected")
		if peer, peerErr := e.peer(request.DeviceID); peerErr == nil {
			_ = e.sendToPeer(peer, wireMessage{Type: "friend_request_response", RequestID: requestID, Status: "rejected"})
		}
		return nil
	}
	return fmt.Errorf("好友申请不存在")
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
		if payload, marshalErr := json.Marshal(wire); marshalErr == nil {
			_ = SaveOutbox(ctx, message.MessageID, deviceID, "message", string(payload))
		}
		e.emit("chat:message", message)
		return message, nil
	}
	if err := e.sendToPeer(peer, wire); err != nil {
		message.Status = "failed"
		if payload, marshalErr := json.Marshal(wire); marshalErr == nil {
			_ = SaveOutbox(ctx, message.MessageID, deviceID, "message", string(payload))
		}
	} else {
		message.Status = "sent"
	}
	_ = exec(ctx, `UPDATE messages SET status=? WHERE message_id=?`, message.Status, message.MessageID)
	e.emit("chat:message", message)
	return message, nil
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
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return Message{}, fmt.Errorf("文件不存在")
	}
	if info.Size() > 100*1024*1024 {
		return Message{}, fmt.Errorf("文件不能超过 100 MB")
	}
	fileName := safeFileName(filepath.Base(path))
	file, err := os.Open(path)
	if err != nil {
		return Message{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return Message{}, err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Message{}, err
	}
	conversationID, err := EnsureConversation(ctx, deviceID)
	if err != nil {
		return Message{}, err
	}
	messageID, attachmentID := newID(), newID()
	message := Message{MessageID: messageID, ConversationID: conversationID, SenderDeviceID: e.identity.DeviceID, Kind: "file", Content: fileName, Status: "sending", CreatedAt: nowString(), AttachmentID: attachmentID, AttachmentName: fileName, AttachmentSize: info.Size(), AttachmentMime: mime.TypeByExtension(filepath.Ext(fileName)), AttachmentStatus: "sending"}
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
	fail := func(sendErr error) (Message, error) {
		message.Status, message.AttachmentStatus = "failed", "failed"
		_ = UpdateMessageStatus(ctx, message.MessageID, message.Status)
		_ = SaveAttachment(ctx, Attachment{AttachmentID: attachmentID, MessageID: messageID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum, LocalPath: path, Status: "failed"})
		e.emit("chat:message-status", map[string]any{"messageId": message.MessageID, "status": message.Status})
		return message, sendErr
	}
	peer, err := e.peer(deviceID)
	if err != nil {
		return fail(err)
	}
	clientTLS, err := e.clientTLSConfig()
	if err != nil {
		return fail(err)
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), clientTLS)
	if err != nil {
		return fail(err)
	}
	defer conn.Close()
	if err := verifyPeerCertificate(conn, peer); err != nil {
		return fail(err)
	}
	decoder := json.NewDecoder(conn)
	if err := writeWire(conn, e.helloMessage("hello")); err != nil {
		return fail(err)
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil {
		return fail(fmt.Errorf("对方握手失败"))
	}
	if response.Type == "error" {
		return fail(fmt.Errorf("对方握手失败: %s", response.Status))
	}
	if response.Type != "hello_ack" {
		return fail(fmt.Errorf("对方握手失败"))
	}
	e.touchPeer(peer.DeviceID)
	if err := writeWire(conn, wireMessage{Type: "file_offer", MessageID: messageID, AttachmentID: attachmentID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum}); err != nil {
		return fail(err)
	}
	buffer := make([]byte, 32*1024)
	index := 0
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			if err := writeWire(conn, wireMessage{Type: "file_chunk", AttachmentID: attachmentID, ChunkIndex: index, Payload: base64.StdEncoding.EncodeToString(buffer[:n])}); err != nil {
				return fail(err)
			}
			index++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fail(readErr)
		}
	}
	if err := writeWire(conn, wireMessage{Type: "file_complete", AttachmentID: attachmentID}); err != nil {
		return fail(err)
	}
	message.Status, message.AttachmentStatus = "sent", "sent"
	_ = exec(ctx, `UPDATE messages SET status=? WHERE message_id=?`, message.Status, message.MessageID)
	_ = SaveAttachment(ctx, Attachment{AttachmentID: attachmentID, MessageID: messageID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum, LocalPath: path, Status: "sent"})
	e.emit("chat:message", message)
	return message, nil
}

func (e *Engine) sendToPeer(peer Peer, message wireMessage) error {
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
	if err := writeWire(conn, e.helloMessage("hello")); err != nil {
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
	e.touchPeer(peer.DeviceID)
	if peer.Relation == PeerRelation && peer.AvatarHash != "" && !cachedAvatarMatches(peer) {
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
		var ack wireMessage
		if err := decoder.Decode(&ack); err != nil {
			return err
		}
		if ack.Type == "error" {
			return fmt.Errorf("%s", ack.Status)
		}
		if ack.Type != "ack" || ack.MessageID != message.MessageID || ack.Status != "sent" {
			return fmt.Errorf("消息确认无效")
		}
	}
	return nil
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

func (e *Engine) retryOutbox(peer Peer) {
	items, err := ListOutbox(context.Background(), peer.DeviceID)
	if err != nil {
		return
	}
	for _, item := range items {
		var message wireMessage
		if json.Unmarshal([]byte(item.Payload), &message) != nil {
			_ = DeleteOutbox(context.Background(), item.ItemID)
			continue
		}
		if err := e.sendToPeer(peer, message); err != nil {
			_ = MarkOutboxRetry(context.Background(), item.ItemID, item.Attempts)
			continue
		}
		_ = DeleteOutbox(context.Background(), item.ItemID)
		_ = exec(context.Background(), `UPDATE messages SET status='sent' WHERE message_id=?`, message.MessageID)
		e.emit("chat:message-status", map[string]any{"messageId": message.MessageID, "status": "sent"})
	}
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
	return e.identity.DeviceInfo
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
	message := e.helloMessage("withdraw")
	targets := broadcastAddresses()
	targets = append(targets, localSubnetTargets()...)
	for index := range targets {
		_ = e.sendDiscovery(&targets[index], message)
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
	e.mu.RUnlock()
	for index := range peers {
		if serviceStopped {
			peers[index].Online = false
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
