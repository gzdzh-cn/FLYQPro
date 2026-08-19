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
	mu       sync.RWMutex
	profile  Profile
	identity Identity
	listener net.Listener
	udp      *net.UDPConn
	stop     chan struct{}
	done     chan struct{}
	peers    map[string]Peer
	incoming map[string]*incomingFile
	started  bool
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
	return &Engine{peers: make(map[string]Peer), incoming: make(map[string]*incomingFile)}
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
	listener, err := tls.Listen("tcp", ":0", &tls.Config{Certificates: []tls.Certificate{tlsCert}, MinVersion: tls.VersionTLS12})
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

	e.mu.Lock()
	e.profile, e.identity, e.listener, e.udp = profile, identity, listener, udp
	e.stop, e.done, e.started = make(chan struct{}), make(chan struct{}), true
	e.mu.Unlock()
	go e.acceptLoop()
	go e.discoveryLoop()
	go e.scanLoop()
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
	_ = e.udp.Close()
	done := e.done
	e.started = false
	e.mu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
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
		return
	}
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
		profile := e.Profile()
		if !profile.Discoverable {
			return
		}
		request := FriendRequest{RequestID: message.RequestID, DeviceID: hello.DeviceID, Nickname: hello.Nickname, Message: message.Content, Status: "pending", CreatedAt: nowString()}
		if err := SaveFriendRequest(context.Background(), request); err == nil {
			e.emit("chat:friend-request", request)
		}
	case "friend_request_response":
		status := message.Status
		if status == "accepted" {
			_ = SetPeerRelation(context.Background(), hello.DeviceID, PeerRelation)
		}
		_ = UpdateFriendRequest(context.Background(), message.RequestID, status)
		e.emit("chat:friend-request-updated", map[string]any{"requestId": message.RequestID, "status": status, "deviceId": hello.DeviceID})
		e.emit("chat:peer-updated", e.Peers())
	case "message":
		if !e.isFriend(hello.DeviceID) {
			_ = writeWire(conn, wireMessage{Type: "error", Status: "FRIENDSHIP_REQUIRED"})
			return
		}
		conversationID, err := EnsureConversation(context.Background(), hello.DeviceID)
		if err != nil {
			return
		}
		messageRecord := Message{MessageID: message.MessageID, ConversationID: conversationID, SenderDeviceID: hello.DeviceID, Kind: message.Kind, Content: message.Content, Status: "delivered", CreatedAt: nowString()}
		if err := SaveMessage(context.Background(), messageRecord); err == nil {
			_ = writeWire(conn, wireMessage{Type: "ack", MessageID: message.MessageID, Status: "delivered"})
			e.emit("chat:message", messageRecord)
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
	if valid && e.Profile().AutoSave {
		localPath = filepath.Join(e.Profile().FileSavePath, safeFileName(transfer.fileName))
		if err := os.MkdirAll(e.Profile().FileSavePath, 0o700); err == nil {
			if os.Rename(transfer.tempPath, localPath) == nil {
				status = "saved"
			}
		}
	}
	if !valid {
		status = "failed"
	}
	_ = SaveAttachment(context.Background(), Attachment{AttachmentID: attachmentID, MessageID: transfer.messageID, FileName: transfer.fileName, FileSize: transfer.expected, SHA256: transfer.sha256, LocalPath: localPath, Status: status})
	e.emit("chat:attachment", map[string]any{"attachmentId": attachmentID, "messageId": transfer.messageID, "fileName": transfer.fileName, "status": status, "localPath": localPath, "valid": valid})
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
		}
	}
}

func (e *Engine) scanLoop() {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.broadcastDiscovery()
		case <-e.stop:
			return
		}
	}
}

func (e *Engine) broadcastDiscovery() {
	message := e.helloMessage("discover")
	addresses := broadcastAddresses()
	if len(addresses) == 0 {
		addresses = []string{"255.255.255.255"}
	}
	for _, address := range addresses {
		_ = e.sendDiscovery(&net.UDPAddr{IP: net.ParseIP(address), Port: DiscoveryPort}, message)
	}
	e.emit("chat:network-status", e.NetworkStatus())
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
	return wireMessage{Magic: DiscoveryMagic, Type: kind, Protocol: ProtocolName, Major: ProtocolMajor, Minor: ProtocolMinor, MinMajor: ProtocolMajor, MinMinor: 0, DeviceID: identity.DeviceID, Nickname: e.Profile().Nickname, Platform: identity.Platform, OSVersion: identity.OSVersion, IP: identity.IP, Port: identity.Port, PublicKey: identity.PublicKeyPEM, CertFP: identity.CertificateFingerprint, Capabilities: []string{"text", "image", "file"}}
}

func (e *Engine) upsertWirePeer(message wireMessage) error {
	if message.PublicKey != "" && !validDevicePublicKey(message.DeviceID, message.PublicKey) {
		return fmt.Errorf("设备身份校验失败")
	}
	peer := Peer{DeviceID: message.DeviceID, Nickname: message.Nickname, Platform: message.Platform, OSVersion: message.OSVersion, IP: message.IP, Port: message.Port, PublicKeyPEM: message.PublicKey, CertificateFingerprint: message.CertFP, Relation: DiscoveredState, LastSeen: nowString()}
	if err := UpsertPeer(context.Background(), peer); err != nil {
		return err
	}
	e.mu.Lock()
	if old, exists := e.peers[peer.DeviceID]; exists {
		peer.Relation, peer.Remark = old.Relation, old.Remark
	}
	peer.Online = true
	e.peers[peer.DeviceID] = peer
	e.mu.Unlock()
	go e.retryOutbox(peer)
	return nil
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
	peer, err := e.peer(deviceID)
	if err != nil {
		return message, err
	}
	wire := wireMessage{Type: "message", MessageID: message.MessageID, Kind: "text", Content: content}
	if err := e.sendToPeer(peer, wire); err != nil {
		message.Status = "queued"
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

func (e *Engine) SendFile(ctx context.Context, deviceID, path string) (Message, error) {
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
	if err := SaveAttachment(ctx, Attachment{AttachmentID: attachmentID, MessageID: messageID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum, Status: "sending"}); err != nil {
		return Message{}, err
	}
	peer, err := e.peer(deviceID)
	if err != nil {
		return message, err
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err != nil {
		return message, err
	}
	defer conn.Close()
	if err := verifyPeerCertificate(conn, peer); err != nil {
		return message, err
	}
	decoder := json.NewDecoder(conn)
	if err := writeWire(conn, e.helloMessage("hello")); err != nil {
		return message, err
	}
	var response wireMessage
	if err := decoder.Decode(&response); err != nil || response.Type != "hello_ack" {
		return message, fmt.Errorf("对方握手失败")
	}
	if err := writeWire(conn, wireMessage{Type: "file_offer", MessageID: messageID, AttachmentID: attachmentID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum}); err != nil {
		return message, err
	}
	buffer := make([]byte, 32*1024)
	index := 0
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			if err := writeWire(conn, wireMessage{Type: "file_chunk", AttachmentID: attachmentID, ChunkIndex: index, Payload: base64.StdEncoding.EncodeToString(buffer[:n])}); err != nil {
				return message, err
			}
			index++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return message, readErr
		}
	}
	if err := writeWire(conn, wireMessage{Type: "file_complete", AttachmentID: attachmentID}); err != nil {
		return message, err
	}
	message.Status, message.AttachmentStatus = "sent", "sent"
	_ = exec(ctx, `UPDATE messages SET status=? WHERE message_id=?`, message.Status, message.MessageID)
	_ = SaveAttachment(ctx, Attachment{AttachmentID: attachmentID, MessageID: messageID, FileName: fileName, MimeType: message.AttachmentMime, FileSize: info.Size(), SHA256: sum, Status: "sent"})
	e.emit("chat:message", message)
	return message, nil
}

func (e *Engine) sendToPeer(peer Peer, message wireMessage) error {
	if peer.IP == "" || peer.Port == 0 {
		return fmt.Errorf("好友地址不可用")
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(peer.IP, fmt.Sprint(peer.Port)), &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
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
	if err := decoder.Decode(&response); err != nil || response.Type != "hello_ack" {
		return fmt.Errorf("对方握手失败")
	}
	return writeWire(conn, message)
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
	actual := sha256Hex(state.PeerCertificates[0].Raw)
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
func (e *Engine) Profile() Profile { e.mu.RLock(); defer e.mu.RUnlock(); return e.profile }
func (e *Engine) DeviceInfo() DeviceInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.identity.DeviceInfo
}

func (e *Engine) UpdateProfile(profile Profile) {
	e.mu.Lock()
	e.profile = profile
	e.mu.Unlock()
	e.emit("chat:profile-updated", profile)
}

func (e *Engine) Peers() []Peer {
	peers, _ := ListPeers(context.Background(), "")
	return peers
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
	e.mu.RUnlock()
	return NetworkStatus{Status: status, Interfaces: names, LocalIPs: ips, DiscoveryPort: DiscoveryPort, ChatPort: chatPort, PeerCount: len(peers), OnlineCount: online, LastScanAt: nowString()}
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

func broadcastAddresses() []string {
	interfaces, _ := net.Interfaces()
	result := make([]string, 0)
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
			result = append(result, broadcast.String())
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
	value = strings.ReplaceAll(value, "\x00", "")
	if value == "" || value == "." {
		return "attachment"
	}
	return value
}
