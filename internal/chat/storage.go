package chat

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"helpfly/internal/service/db"
)

type profileRow struct {
	Nickname        string `orm:"nickname"`
	AvatarPath      string `orm:"avatar_path"`
	AvatarHash      string `orm:"avatar_hash"`
	AvatarVersion   int64  `orm:"avatar_version"`
	Discoverable    int    `orm:"discoverable"`
	AutoSave        int    `orm:"auto_save"`
	FileSavePath    string `orm:"file_save_path"`
	Theme           string `orm:"theme"`
	LaunchAtStartup int    `orm:"launch_at_startup"`
}

type identityRow struct {
	DeviceID               string `orm:"device_id"`
	PublicKeyPEM           string `orm:"public_key_pem"`
	PrivateKeyPEM          string `orm:"private_key_pem"`
	CertificatePEM         string `orm:"certificate_pem"`
	CertificateFingerprint string `orm:"certificate_fingerprint"`
}

type peerRow struct {
	DeviceID               string `orm:"device_id"`
	Nickname               string `orm:"nickname"`
	AvatarPath             string `orm:"avatar_path"`
	AvatarHash             string `orm:"avatar_hash"`
	AvatarVersion          int64  `orm:"avatar_version"`
	Platform               string `orm:"platform"`
	OSVersion              string `orm:"os_version"`
	IP                     string `orm:"ip"`
	Port                   int    `orm:"port"`
	PublicKeyPEM           string `orm:"public_key_pem"`
	CertificateFingerprint string `orm:"certificate_fingerprint"`
	Relation               string `orm:"relation"`
	Remark                 string `orm:"remark"`
	LastSeen               string `orm:"last_seen"`
	UpdatedAt              string `orm:"updated_at"`
}

type requestRow struct {
	RequestID string `orm:"request_id"`
	DeviceID  string `orm:"device_id"`
	Nickname  string `orm:"nickname"`
	Message   string `orm:"message"`
	Status    string `orm:"status"`
	CreatedAt string `orm:"created_at"`
	UpdatedAt string `orm:"updated_at"`
}

type conversationRow struct {
	ConversationID string `orm:"conversation_id"`
	PeerDeviceID   string `orm:"peer_device_id"`
	LastMessage    string `orm:"last_message"`
	LastMessageAt  string `orm:"last_message_at"`
	UnreadCount    int    `orm:"unread_count"`
	Pinned         int    `orm:"pinned"`
}

type messageRow struct {
	MessageID      string `orm:"message_id"`
	ConversationID string `orm:"conversation_id"`
	SenderDeviceID string `orm:"sender_device_id"`
	Kind           string `orm:"kind"`
	Content        string `orm:"content"`
	Status         string `orm:"status"`
	CreatedAt      string `orm:"created_at"`
}

type attachmentRow struct {
	AttachmentID string `orm:"attachment_id"`
	MessageID    string `orm:"message_id"`
	FileName     string `orm:"file_name"`
	MimeType     string `orm:"mime_type"`
	FileSize     int64  `orm:"file_size"`
	SHA256       string `orm:"sha256"`
	LocalPath    string `orm:"local_path"`
	Status       string `orm:"status"`
}

type attachmentMigrationRow struct {
	AttachmentID string `orm:"attachment_id"`
	MessageID    string `orm:"message_id"`
	FileName     string `orm:"file_name"`
	LocalPath    string `orm:"local_path"`
	FileSize     int64  `orm:"file_size"`
	SHA256       string `orm:"sha256"`
	Status       string `orm:"status"`
	PeerDeviceID string `orm:"peer_device_id"`
}

type outboxRow struct {
	ItemID        string `orm:"item_id"`
	PeerDeviceID  string `orm:"peer_device_id"`
	Payload       string `orm:"payload"`
	Attempts      int    `orm:"attempts"`
	NextAttemptAt string `orm:"next_attempt_at"`
}

func nowString() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func query(ctx context.Context, sql string, args ...any) (gdb.Result, error) {
	if database := db.DB(); database != nil {
		return database.GetAll(ctx, sql, args...)
	}
	return nil, fmt.Errorf("数据库尚未初始化")
}

func exec(ctx context.Context, sql string, args ...any) error {
	if database := db.DB(); database != nil {
		_, err := database.Exec(ctx, sql, args...)
		return err
	}
	return fmt.Errorf("数据库尚未初始化")
}

func EnsureDefaults(ctx context.Context, defaultPath string) error {
	now := nowString()
	return exec(ctx, `INSERT INTO profiles(id, nickname, file_save_path, theme, created_at, updated_at)
		VALUES(1, ?, ?, 'system', ?, ?)
		ON CONFLICT(id) DO NOTHING`, randomChineseNickname(), defaultPath, now, now)
}

func randomChineseNickname() string {
	prefixes := []string{"薄荷", "星际", "云端", "彩虹", "泡泡", "月光", "棉花糖", "银河", "森林", "魔法"}
	themes := []string{"水母", "熊猫", "仙人掌", "小狐狸", "独角兽", "向日葵", "蒲公英", "月亮", "小精灵", "鲸鱼", "樱桃", "小火车"}
	return prefixes[randomIndex(len(prefixes))] + themes[randomIndex(len(themes))]
}

func randomIndex(size int) int {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(size)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(size))
	}
	return int(value.Int64())
}

func GetProfile(ctx context.Context) (Profile, error) {
	var rows []profileRow
	result, err := query(ctx, `SELECT nickname, avatar_path, avatar_hash, avatar_version, discoverable, auto_save, file_save_path, theme, launch_at_startup FROM profiles WHERE id = 1`)
	if err != nil {
		return Profile{}, err
	}
	if err := result.Structs(&rows); err != nil {
		return Profile{}, err
	}
	if len(rows) == 0 {
		return Profile{}, fmt.Errorf("个人资料不存在")
	}
	row := rows[0]
	profile := Profile{Nickname: row.Nickname, AvatarPath: row.AvatarPath, AvatarHash: row.AvatarHash, AvatarVersion: row.AvatarVersion, Discoverable: row.Discoverable != 0, AutoSave: row.AutoSave != 0, FileSavePath: row.FileSavePath, Theme: row.Theme, LaunchAtStartup: row.LaunchAtStartup != 0}
	if strings.TrimSpace(profile.Nickname) == "" || strings.TrimSpace(profile.Nickname) == "新用户" {
		profile.Nickname = randomChineseNickname()
		if err := SaveProfile(ctx, profile); err != nil {
			return Profile{}, err
		}
	}
	if strings.TrimSpace(profile.FileSavePath) == "" {
		profile.FileSavePath = DefaultAttachmentDir()
		if err := SaveProfile(ctx, profile); err != nil {
			return Profile{}, err
		}
	}
	return profile, nil
}

func SaveProfile(ctx context.Context, profile Profile) error {
	return exec(ctx, `UPDATE profiles SET nickname=?, avatar_path=?, avatar_hash=?, avatar_version=?, discoverable=?, auto_save=?, file_save_path=?, theme=?, launch_at_startup=?, updated_at=? WHERE id=1`,
		strings.TrimSpace(profile.Nickname), profile.AvatarPath, profile.AvatarHash, profile.AvatarVersion, boolInt(profile.Discoverable), boolInt(profile.AutoSave), profile.FileSavePath, profile.Theme, boolInt(profile.LaunchAtStartup), nowString())
}

func GetIdentity(ctx context.Context) (identityRow, error) {
	var rows []identityRow
	result, err := query(ctx, `SELECT device_id, public_key_pem, private_key_pem, certificate_pem, certificate_fingerprint FROM device_identity WHERE id=1`)
	if err != nil {
		return identityRow{}, err
	}
	if err := result.Structs(&rows); err != nil {
		return identityRow{}, err
	}
	if len(rows) == 0 {
		return identityRow{}, fmt.Errorf("identity_not_found")
	}
	return rows[0], nil
}

func SaveIdentity(ctx context.Context, identity DeviceInfo, privateKeyPEM, certificatePEM string) error {
	return exec(ctx, `INSERT INTO device_identity(id, device_id, public_key_pem, private_key_pem, certificate_pem, certificate_fingerprint, created_at, updated_at)
		VALUES(1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET device_id=excluded.device_id, public_key_pem=excluded.public_key_pem, private_key_pem=excluded.private_key_pem, certificate_pem=excluded.certificate_pem, certificate_fingerprint=excluded.certificate_fingerprint, updated_at=excluded.updated_at`,
		identity.DeviceID, identity.PublicKeyPEM, privateKeyPEM, certificatePEM, identity.CertificateFingerprint, nowString(), nowString())
}

func UpsertPeer(ctx context.Context, peer Peer) error {
	return exec(ctx, `INSERT INTO peers(device_id, nickname, avatar_path, avatar_hash, avatar_version, platform, os_version, ip, port, public_key_pem, certificate_fingerprint, relation, remark, last_seen, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE((SELECT relation FROM peers WHERE device_id=?), ?), COALESCE((SELECT remark FROM peers WHERE device_id=?), ''), ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET nickname=excluded.nickname, avatar_path=CASE WHEN excluded.avatar_path='' THEN peers.avatar_path ELSE excluded.avatar_path END, avatar_hash=CASE WHEN excluded.avatar_hash='' THEN peers.avatar_hash ELSE excluded.avatar_hash END, avatar_version=CASE WHEN excluded.avatar_hash='' THEN peers.avatar_version ELSE excluded.avatar_version END, platform=excluded.platform, os_version=excluded.os_version, ip=excluded.ip, port=excluded.port, public_key_pem=excluded.public_key_pem, certificate_fingerprint=excluded.certificate_fingerprint, last_seen=excluded.last_seen, updated_at=excluded.updated_at`,
		peer.DeviceID, peer.Nickname, peer.AvatarPath, peer.AvatarHash, peer.AvatarVersion, peer.Platform, peer.OSVersion, peer.IP, peer.Port, peer.PublicKeyPEM, peer.CertificateFingerprint, peer.DeviceID, peer.Relation, peer.DeviceID, peer.LastSeen, nowString(), nowString())
}

func SetPeerRelation(ctx context.Context, deviceID, relation string) error {
	return exec(ctx, `UPDATE peers SET relation=?, updated_at=? WHERE device_id=?`, relation, nowString(), deviceID)
}

func SetPeerRemark(ctx context.Context, deviceID, remark string) error {
	return exec(ctx, `UPDATE peers SET remark=?, updated_at=? WHERE device_id=?`, remark, nowString(), deviceID)
}

func SetPeerAvatar(ctx context.Context, deviceID, avatarPath, avatarHash string, avatarVersion int64) error {
	return exec(ctx, `UPDATE peers SET avatar_path=?, avatar_hash=?, avatar_version=?, updated_at=? WHERE device_id=?`, avatarPath, avatarHash, avatarVersion, nowString(), deviceID)
}

func ListPeers(ctx context.Context, relation string) ([]Peer, error) {
	sql := `SELECT device_id, nickname, avatar_path, avatar_hash, avatar_version, platform, os_version, ip, port, public_key_pem, certificate_fingerprint, relation, remark, last_seen, updated_at FROM peers`
	args := []any{}
	if relation != "" {
		sql += ` WHERE relation=?`
		args = append(args, relation)
	}
	sql += ` ORDER BY CASE WHEN relation='friend' THEN 0 ELSE 1 END, nickname COLLATE NOCASE`
	var rows []peerRow
	result, err := query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	peers := make([]Peer, 0, len(rows))
	for _, row := range rows {
		peers = append(peers, Peer{DeviceID: row.DeviceID, Nickname: row.Nickname, AvatarPath: row.AvatarPath, AvatarHash: row.AvatarHash, AvatarVersion: row.AvatarVersion, Platform: row.Platform, OSVersion: row.OSVersion, IP: row.IP, Port: row.Port, PublicKeyPEM: row.PublicKeyPEM, CertificateFingerprint: row.CertificateFingerprint, Relation: row.Relation, Remark: row.Remark, LastSeen: row.LastSeen, Online: recent(row.LastSeen), UpdatedAt: parseTime(row.UpdatedAt)})
	}
	return peers, nil
}

func SaveFriendRequest(ctx context.Context, request FriendRequest) error {
	return exec(ctx, `INSERT INTO friend_requests(request_id, device_id, nickname, message, status, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(request_id) DO UPDATE SET nickname=excluded.nickname, message=excluded.message, status=excluded.status, updated_at=excluded.updated_at`,
		request.RequestID, request.DeviceID, request.Nickname, request.Message, request.Status, request.CreatedAt, nowString())
}

func UpdateFriendRequest(ctx context.Context, requestID, status string) error {
	return exec(ctx, `UPDATE friend_requests SET status=?, updated_at=? WHERE request_id=?`, status, nowString(), requestID)
}

func ListFriendRequests(ctx context.Context, status string) ([]FriendRequest, error) {
	sql := `SELECT request_id, device_id, nickname, message, status, created_at, updated_at FROM friend_requests`
	args := []any{}
	if status != "" {
		sql += ` WHERE status=?`
		args = append(args, status)
	}
	sql += ` ORDER BY created_at DESC`
	var rows []requestRow
	result, err := query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	resultRows := make([]FriendRequest, 0, len(rows))
	for _, row := range rows {
		resultRows = append(resultRows, FriendRequest{RequestID: row.RequestID, DeviceID: row.DeviceID, Nickname: row.Nickname, Message: row.Message, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return resultRows, nil
}

func EnsureConversation(ctx context.Context, peerDeviceID string) (string, error) {
	id := "conv-" + peerDeviceID
	now := nowString()
	if err := exec(ctx, `INSERT INTO conversations(conversation_id, peer_device_id, created_at, updated_at) VALUES(?, ?, ?, ?) ON CONFLICT(peer_device_id) DO NOTHING`, id, peerDeviceID, now, now); err != nil {
		return "", err
	}
	return id, nil
}

func ListConversations(ctx context.Context) ([]Conversation, error) {
	var rows []conversationRow
	result, err := query(ctx, `SELECT conversation_id, peer_device_id, last_message, last_message_at, unread_count, pinned FROM conversations ORDER BY pinned DESC, last_message_at DESC`)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	conversations := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		conversations = append(conversations, Conversation{ConversationID: row.ConversationID, PeerDeviceID: row.PeerDeviceID, LastMessage: row.LastMessage, LastMessageAt: row.LastMessageAt, UnreadCount: row.UnreadCount, Pinned: row.Pinned != 0})
	}
	return conversations, nil
}

func ListMessages(ctx context.Context, conversationID string) ([]Message, error) {
	var rows []messageRow
	result, err := query(ctx, `SELECT message_id, conversation_id, sender_device_id, kind, content, status, created_at FROM messages WHERE conversation_id=? ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		message := Message{MessageID: row.MessageID, ConversationID: row.ConversationID, SenderDeviceID: row.SenderDeviceID, Kind: row.Kind, Content: row.Content, Status: row.Status, CreatedAt: row.CreatedAt}
		if row.Kind == "file" {
			var attachments []attachmentRow
			if attachmentResult, attachmentErr := query(ctx, `SELECT attachment_id, message_id, file_name, mime_type, file_size, sha256, local_path, status FROM attachments WHERE message_id=? LIMIT 1`, row.MessageID); attachmentErr == nil && attachmentResult.Structs(&attachments) == nil && len(attachments) > 0 {
				attachment := attachments[0]
				message.AttachmentID, message.AttachmentName, message.AttachmentSize, message.AttachmentMime, message.AttachmentStatus, message.AttachmentPath = attachment.AttachmentID, attachment.FileName, attachment.FileSize, attachment.MimeType, attachment.Status, attachment.LocalPath
			}
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func SaveMessage(ctx context.Context, message Message) error {
	if err := exec(ctx, `INSERT INTO messages(message_id, conversation_id, sender_device_id, kind, content, status, created_at) VALUES(?, ?, ?, ?, ?, ?, ?) ON CONFLICT(message_id) DO NOTHING`, message.MessageID, message.ConversationID, message.SenderDeviceID, message.Kind, message.Content, message.Status, message.CreatedAt); err != nil {
		return err
	}
	return exec(ctx, `UPDATE conversations SET last_message=?, last_message_at=?, updated_at=? WHERE conversation_id=?`, message.Content, message.CreatedAt, nowString(), message.ConversationID)
}

func UpdateMessageStatus(ctx context.Context, messageID, status string) error {
	return exec(ctx, `UPDATE messages SET status=? WHERE message_id=?`, status, messageID)
}

func MessageExists(ctx context.Context, messageID string) (bool, error) {
	var rows []struct {
		MessageID string `orm:"message_id"`
	}
	result, err := query(ctx, `SELECT message_id FROM messages WHERE message_id=? LIMIT 1`, messageID)
	if err != nil {
		return false, err
	}
	if err := result.Structs(&rows); err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func SaveAttachment(ctx context.Context, attachment Attachment) error {
	return exec(ctx, `INSERT INTO attachments(attachment_id, message_id, file_name, mime_type, file_size, sha256, local_path, status, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(attachment_id) DO UPDATE SET local_path=excluded.local_path, status=excluded.status`, attachment.AttachmentID, attachment.MessageID, attachment.FileName, attachment.MimeType, attachment.FileSize, attachment.SHA256, attachment.LocalPath, attachment.Status, nowString())
}

func ListAttachmentMigrationRows(ctx context.Context) ([]attachmentMigrationRow, error) {
	var rows []attachmentMigrationRow
	result, err := query(ctx, `SELECT a.attachment_id, a.message_id, a.file_name, a.local_path, a.file_size, a.sha256, a.status, c.peer_device_id
		FROM attachments a LEFT JOIN messages m ON m.message_id=a.message_id LEFT JOIN conversations c ON c.conversation_id=m.conversation_id ORDER BY a.created_at, a.attachment_id`)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func UpdateAttachmentLocalPath(ctx context.Context, attachmentID, localPath string) error {
	return exec(ctx, `UPDATE attachments SET local_path=? WHERE attachment_id=?`, localPath, attachmentID)
}

func AttachmentPeerDeviceID(ctx context.Context, attachmentID string) (string, error) {
	var rows []struct {
		PeerDeviceID string `orm:"peer_device_id"`
	}
	result, err := query(ctx, `SELECT c.peer_device_id FROM attachments a JOIN messages m ON m.message_id=a.message_id JOIN conversations c ON c.conversation_id=m.conversation_id WHERE a.attachment_id=?`, attachmentID)
	if err != nil {
		return "", err
	}
	if err := result.Structs(&rows); err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0].PeerDeviceID, nil
}

func SaveOutbox(ctx context.Context, itemID, peerDeviceID, kind, payload string) error {
	return exec(ctx, `INSERT INTO outbox(item_id, peer_device_id, kind, payload, attempts, next_attempt_at, created_at) VALUES(?, ?, ?, ?, 0, ?, ?) ON CONFLICT(item_id) DO UPDATE SET payload=excluded.payload, next_attempt_at=excluded.next_attempt_at`, itemID, peerDeviceID, kind, payload, nowString(), nowString())
}

func ListOutbox(ctx context.Context, peerDeviceID string) ([]outboxRow, error) {
	var rows []outboxRow
	result, err := query(ctx, `SELECT item_id, peer_device_id, payload, attempts, next_attempt_at FROM outbox WHERE peer_device_id=? AND next_attempt_at<=? ORDER BY created_at`, peerDeviceID, nowString())
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func MarkOutboxRetry(ctx context.Context, itemID string, attempts int) error {
	if attempts < 0 {
		attempts = 0
	}
	// Keep retries bounded but useful on a sleeping/offline laptop: 2, 4, 8...
	// minutes, capped at five minutes. The persisted deadline survives restart.
	delay := time.Duration(1<<minInt(attempts, 8)) * time.Second
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return exec(ctx, `UPDATE outbox SET attempts=?, next_attempt_at=? WHERE item_id=?`, attempts+1, time.Now().Add(delay).UTC().Format(time.RFC3339Nano), itemID)
}

func DeleteOutbox(ctx context.Context, itemID string) error {
	return exec(ctx, `DELETE FROM outbox WHERE item_id=?`, itemID)
}

func GetAttachment(ctx context.Context, id string) (Attachment, error) {
	var rows []attachmentRow
	result, err := query(ctx, `SELECT attachment_id, message_id, file_name, mime_type, file_size, sha256, local_path, status FROM attachments WHERE attachment_id=?`, id)
	if err != nil {
		return Attachment{}, err
	}
	if err := result.Structs(&rows); err != nil {
		return Attachment{}, err
	}
	if len(rows) == 0 {
		return Attachment{}, fmt.Errorf("attachment_not_found")
	}
	row := rows[0]
	return Attachment{AttachmentID: row.AttachmentID, MessageID: row.MessageID, FileName: row.FileName, MimeType: row.MimeType, FileSize: row.FileSize, SHA256: row.SHA256, LocalPath: row.LocalPath, Status: row.Status}, nil
}

func parseTime(value string) time.Time { t, _ := time.Parse(time.RFC3339Nano, value); return t }
func recent(value string) bool {
	t := parseTime(value)
	return !t.IsZero() && time.Since(t) < 45*time.Second
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
