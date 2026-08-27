package chat

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"flyqpro/internal/service/db"
	"github.com/gogf/gf/v2/database/gdb"
)

type profileRow struct {
	Nickname               string `orm:"nickname"`
	AvatarPath             string `orm:"avatar_path"`
	AvatarHash             string `orm:"avatar_hash"`
	AvatarVersion          int64  `orm:"avatar_version"`
	Discoverable           int    `orm:"discoverable"`
	AutoSave               int    `orm:"auto_save"`
	FileSavePath           string `orm:"file_save_path"`
	SharedRootPath         string `orm:"shared_root_path"`
	SharedEnabled          int    `orm:"shared_enabled"`
	SharedDriveMultiWindow int    `orm:"shared_drive_multi_window"`
	Theme                  string `orm:"theme"`
	LaunchAtStartup        int    `orm:"launch_at_startup"`
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
	ProtocolName           string `orm:"protocol_name"`
	ProtocolMajor          int    `orm:"protocol_major"`
	DiscoveryMagic         string `orm:"discovery_magic"`
	Capabilities           string `orm:"capabilities"`
	DiscoveryVisible       int    `orm:"discovery_visible"`
	VisibleInFriends       int    `orm:"visible_in_friends"`
	RelationshipVersion    string `orm:"relationship_version"`
	LastSeen               string `orm:"last_seen"`
	UpdatedAt              string `orm:"updated_at"`
}

type requestRow struct {
	RequestID  string `orm:"request_id"`
	DeviceID   string `orm:"device_id"`
	Nickname   string `orm:"nickname"`
	Message    string `orm:"message"`
	Status     string `orm:"status"`
	Direction  string `orm:"direction"`
	CreatedAt  string `orm:"created_at"`
	AcceptedAt string `orm:"accepted_at"`
	UpdatedAt  string `orm:"updated_at"`
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
	IsFavorite     int    `orm:"is_favorite"`
	DeletedAt      string `orm:"deleted_at"`
	QuoteMessageID string `orm:"quote_message_id"`
	QuoteContent   string `orm:"quote_content"`
	ForwardedFrom  string `orm:"forwarded_from"`
}

type attachmentRow struct {
	AttachmentID  string `orm:"attachment_id"`
	MessageID     string `orm:"message_id"`
	FileName      string `orm:"file_name"`
	MimeType      string `orm:"mime_type"`
	FileSize      int64  `orm:"file_size"`
	SHA256        string `orm:"sha256"`
	ThumbnailData string `orm:"thumbnail_data"`
	ThumbnailMime string `orm:"thumbnail_mime"`
	LocalPath     string `orm:"local_path"`
	Status        string `orm:"status"`
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

type ConversationAttachment struct {
	Attachment
	SenderDeviceID string `json:"senderDeviceId"`
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
	result, err := query(ctx, `SELECT nickname, avatar_path, avatar_hash, avatar_version, discoverable, auto_save, file_save_path, shared_root_path, shared_enabled, shared_drive_multi_window, theme, launch_at_startup FROM profiles WHERE id = 1`)
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
	profile := Profile{Nickname: row.Nickname, AvatarPath: row.AvatarPath, AvatarHash: row.AvatarHash, AvatarVersion: row.AvatarVersion, Discoverable: row.Discoverable != 0, AutoSave: row.AutoSave != 0, FileSavePath: row.FileSavePath, SharedRootPath: row.SharedRootPath, SharedEnabled: row.SharedEnabled != 0, SharedDriveMultiWindow: row.SharedDriveMultiWindow != 0, Theme: row.Theme, LaunchAtStartup: row.LaunchAtStartup != 0}
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
	if migratedPath := migrateLegacyAttachmentPath(profile.FileSavePath); migratedPath != profile.FileSavePath {
		profile.FileSavePath = migratedPath
		if err := SaveProfile(ctx, profile); err != nil {
			return Profile{}, err
		}
	}
	return profile, nil
}

func migrateLegacyAttachmentPath(path string) string {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || cleanPath == "" {
		return path
	}
	for _, legacyRoot := range []string{
		filepath.Join(legacyAppDataDir(), "attachments"),
		filepath.Join(legacyLanChatDataDir(), "attachments"),
	} {
		relative, err := filepath.Rel(legacyRoot, cleanPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		return filepath.Join(DefaultAttachmentDir(), relative)
	}
	return path
}

func SaveProfile(ctx context.Context, profile Profile) error {
	return exec(ctx, `UPDATE profiles SET nickname=?, avatar_path=?, avatar_hash=?, avatar_version=?, discoverable=?, auto_save=?, file_save_path=?, shared_root_path=?, shared_enabled=?, shared_drive_multi_window=?, theme=?, launch_at_startup=?, updated_at=? WHERE id=1`,
		strings.TrimSpace(profile.Nickname), profile.AvatarPath, profile.AvatarHash, profile.AvatarVersion, boolInt(profile.Discoverable), boolInt(profile.AutoSave), profile.FileSavePath, profile.SharedRootPath, boolInt(profile.SharedEnabled), boolInt(profile.SharedDriveMultiWindow), profile.Theme, boolInt(profile.LaunchAtStartup), nowString())
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
	visibleInFriends := peer.VisibleInFriends
	if peer.Relation == PeerRelation {
		visibleInFriends = true
	}
	return exec(ctx, `INSERT INTO peers(device_id, nickname, avatar_path, avatar_hash, avatar_version, platform, os_version, ip, port, public_key_pem, certificate_fingerprint, relation, remark, protocol_name, protocol_major, discovery_magic, capabilities, discovery_visible, visible_in_friends, relationship_version, last_seen, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE((SELECT relation FROM peers WHERE device_id=?), ?), COALESCE((SELECT remark FROM peers WHERE device_id=?), ''), ?, ?, ?, ?, ?, COALESCE((SELECT visible_in_friends FROM peers WHERE device_id=?), ?), COALESCE((SELECT relationship_version FROM peers WHERE device_id=?), ?), ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET nickname=excluded.nickname, avatar_path=CASE WHEN excluded.avatar_path='' THEN peers.avatar_path ELSE excluded.avatar_path END, avatar_hash=CASE WHEN excluded.avatar_hash='' THEN peers.avatar_hash ELSE excluded.avatar_hash END, avatar_version=CASE WHEN excluded.avatar_hash='' THEN peers.avatar_version ELSE excluded.avatar_version END, platform=excluded.platform, os_version=excluded.os_version, ip=excluded.ip, port=excluded.port, public_key_pem=excluded.public_key_pem, certificate_fingerprint=excluded.certificate_fingerprint, protocol_name=CASE WHEN excluded.protocol_name='' THEN peers.protocol_name ELSE excluded.protocol_name END, protocol_major=CASE WHEN excluded.protocol_major=0 THEN peers.protocol_major ELSE excluded.protocol_major END, discovery_magic=CASE WHEN excluded.discovery_magic='' THEN peers.discovery_magic ELSE excluded.discovery_magic END, capabilities=CASE WHEN excluded.capabilities='' THEN peers.capabilities ELSE excluded.capabilities END, discovery_visible=excluded.discovery_visible, relationship_version=CASE WHEN excluded.relationship_version='' THEN peers.relationship_version ELSE excluded.relationship_version END, last_seen=excluded.last_seen, updated_at=excluded.updated_at`,
		peer.DeviceID, peer.Nickname, peer.AvatarPath, peer.AvatarHash, peer.AvatarVersion, peer.Platform, peer.OSVersion, peer.IP, peer.Port, peer.PublicKeyPEM, peer.CertificateFingerprint, peer.DeviceID, peer.Relation, peer.DeviceID, peer.ProtocolName, peer.ProtocolMajor, peer.DiscoveryMagic, strings.Join(peer.Capabilities, ","), boolInt(peer.DiscoveryVisible), peer.DeviceID, boolInt(visibleInFriends), peer.DeviceID, peer.RelationshipVersion, peer.LastSeen, nowString(), nowString())
}

func SetPeerDiscoveryVisible(ctx context.Context, deviceID string, visible bool) error {
	return exec(ctx, `UPDATE peers SET discovery_visible=?, updated_at=? WHERE device_id=?`, boolInt(visible), nowString(), deviceID)
}

func SetPeerVisibleInFriends(ctx context.Context, deviceID string, visible bool) error {
	return exec(ctx, `UPDATE peers SET visible_in_friends=?, updated_at=? WHERE device_id=?`, boolInt(visible), nowString(), deviceID)
}

func SetPeerRelation(ctx context.Context, deviceID, relation string) error {
	return exec(ctx, `UPDATE peers SET relation=?, updated_at=? WHERE device_id=?`, relation, nowString(), deviceID)
}

func SetPeerRelationshipVersion(ctx context.Context, deviceID, version string) error {
	return exec(ctx, `UPDATE peers SET relationship_version=?, updated_at=? WHERE device_id=?`, version, nowString(), deviceID)
}

func SetPeerRemark(ctx context.Context, deviceID, remark string) error {
	return exec(ctx, `UPDATE peers SET remark=?, updated_at=? WHERE device_id=?`, remark, nowString(), deviceID)
}

func SetPeerProtocol(ctx context.Context, deviceID string, dialect ProtocolDialect, capabilities []string) error {
	return exec(ctx, `UPDATE peers SET protocol_name=?, protocol_major=?, discovery_magic=?, capabilities=?, updated_at=? WHERE device_id=?`, dialect.Name, dialect.Major, dialect.Magic, strings.Join(capabilities, ","), nowString(), deviceID)
}

func SetPeerAvatar(ctx context.Context, deviceID, avatarPath, avatarHash string, avatarVersion int64) error {
	return exec(ctx, `UPDATE peers SET avatar_path=?, avatar_hash=?, avatar_version=?, updated_at=? WHERE device_id=?`, avatarPath, avatarHash, avatarVersion, nowString(), deviceID)
}

func ListPeers(ctx context.Context, relation string) ([]Peer, error) {
	sql := `SELECT device_id, nickname, avatar_path, avatar_hash, avatar_version, platform, os_version, ip, port, public_key_pem, certificate_fingerprint, relation, remark, protocol_name, protocol_major, discovery_magic, capabilities, discovery_visible, visible_in_friends, relationship_version, last_seen, updated_at FROM peers`
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
		peer := Peer{DeviceID: row.DeviceID, Nickname: row.Nickname, AvatarPath: row.AvatarPath, AvatarHash: row.AvatarHash, AvatarVersion: row.AvatarVersion, Platform: row.Platform, OSVersion: row.OSVersion, IP: row.IP, Port: row.Port, PublicKeyPEM: row.PublicKeyPEM, CertificateFingerprint: row.CertificateFingerprint, Relation: row.Relation, Remark: row.Remark, ProtocolName: row.ProtocolName, ProtocolMajor: row.ProtocolMajor, DiscoveryMagic: row.DiscoveryMagic, Capabilities: splitCapabilities(row.Capabilities), DiscoveryVisible: row.DiscoveryVisible != 0, VisibleInFriends: row.VisibleInFriends != 0, RelationshipVersion: row.RelationshipVersion, LastSeen: row.LastSeen, Online: recent(row.LastSeen), UpdatedAt: parseTime(row.UpdatedAt)}
		// Older builds persisted a full contact removal directly as
		// relation=removed. Normalize it to the current retained-contact
		// state so it can show "不是好友". Do not overwrite the saved local
		// visibility flag: the user may have hidden this retained row already.
		if peer.Relation == "removed" {
			peer.Relation = DiscoveredState
			peer.FriendshipState = "removed"
		}
		// A removal tombstone belongs to a peer that has already been
		// downgraded from friend.  Do not let a stale tombstone override an
		// active friend row (or its explicit visible_in_friends=false hide
		// flag).  The latter can happen when an older removal notification is
		// delivered after the local "hide friend" operation.
		if peer.Relation != PeerRelation {
			if removed, removalErr := IsFriendRemoved(ctx, row.DeviceID); removalErr == nil && removed {
				peer.Relation = DiscoveredState
				peer.FriendshipState = "removed"
			}
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

func splitCapabilities(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func SaveFriendRequest(ctx context.Context, request FriendRequest) error {
	return exec(ctx, `INSERT INTO friend_requests(request_id, device_id, nickname, message, status, direction, created_at, accepted_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(request_id) DO UPDATE SET nickname=excluded.nickname, message=excluded.message, status=CASE WHEN friend_requests.status IN ('accepted', 'rejected') THEN friend_requests.status ELSE excluded.status END, direction=CASE WHEN excluded.direction != '' THEN excluded.direction ELSE friend_requests.direction END, accepted_at=CASE WHEN excluded.accepted_at != '' THEN excluded.accepted_at ELSE friend_requests.accepted_at END, updated_at=excluded.updated_at`,
		request.RequestID, request.DeviceID, request.Nickname, request.Message, request.Status, request.Direction, request.CreatedAt, request.AcceptedAt, nowString())
}

func UpdateFriendRequest(ctx context.Context, requestID, status string) error {
	now := nowString()
	if status == "accepted" {
		return exec(ctx, `UPDATE friend_requests SET status=?, accepted_at=CASE WHEN accepted_at='' THEN ? ELSE accepted_at END, updated_at=? WHERE request_id=?`, status, now, now, requestID)
	}
	return exec(ctx, `UPDATE friend_requests SET status=?, updated_at=? WHERE request_id=?`, status, now, requestID)
}

func UpdateFriendRequestAccepted(ctx context.Context, requestID, acceptedAt string) error {
	if acceptedAt == "" {
		acceptedAt = nowString()
	}
	return exec(ctx, `UPDATE friend_requests SET status='accepted', accepted_at=?, updated_at=? WHERE request_id=?`, acceptedAt, nowString(), requestID)
}

func UpdateFriendRequestsForDevice(ctx context.Context, deviceID, status, acceptedAt string) error {
	now := nowString()
	if status == "accepted" {
		if acceptedAt == "" {
			acceptedAt = now
		}
		return exec(ctx, `UPDATE friend_requests SET status=?, accepted_at=CASE WHEN accepted_at='' THEN ? ELSE accepted_at END, updated_at=? WHERE device_id=? AND status IN ('pending', 'sent', 'queued')`, status, acceptedAt, now, deviceID)
	}
	return exec(ctx, `UPDATE friend_requests SET status=?, updated_at=? WHERE device_id=? AND status IN ('pending', 'sent', 'queued')`, status, now, deviceID)
}

func listFriendRequestRows(ctx context.Context, status string) ([]FriendRequest, error) {
	sql := `SELECT request_id, device_id, nickname, message, status, direction, created_at, accepted_at, updated_at FROM friend_requests`
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
		resultRows = append(resultRows, FriendRequest{RequestID: row.RequestID, DeviceID: row.DeviceID, Nickname: row.Nickname, Message: row.Message, Status: row.Status, Direction: row.Direction, CreatedAt: row.CreatedAt, AcceptedAt: row.AcceptedAt, UpdatedAt: row.UpdatedAt})
	}
	return resultRows, nil
}

func requestTimeBefore(left, right string) bool {
	if left == "" {
		return false
	}
	if right == "" {
		return true
	}
	return parseTime(left).Before(parseTime(right))
}

func requestTimeAfter(left, right string) bool {
	if left == "" {
		return false
	}
	if right == "" {
		return true
	}
	return parseTime(left).After(parseTime(right))
}

func ListFriendRequests(ctx context.Context, status string) ([]FriendRequest, error) {
	rows, err := listFriendRequestRows(ctx, "")
	if err != nil {
		return nil, err
	}
	// The database keeps the complete lifecycle for audit/history, but the
	// friends UI must have exactly one row per device. Pick the newest request
	// cycle, so a new pending request replaces an old accepted one visually.
	latestByDevice := make(map[string]FriendRequest)
	activeByDevice := make(map[string]FriendRequest)
	for _, request := range rows {
		if isActiveFriendRequest(request.Status) {
			current, exists := activeByDevice[request.DeviceID]
			if !exists || requestIsNewer(request, current) {
				activeByDevice[request.DeviceID] = request
			}
			continue
		}
		current, exists := latestByDevice[request.DeviceID]
		if !exists || requestIsNewer(request, current) {
			latestByDevice[request.DeviceID] = request
		}
	}
	for deviceID, request := range activeByDevice {
		latestByDevice[deviceID] = request
	}
	result := make([]FriendRequest, 0, len(latestByDevice))
	for _, request := range latestByDevice {
		if status == "" || request.Status == status {
			result = append(result, request)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return requestTimeAfter(result[i].UpdatedAt, result[j].UpdatedAt)
	})
	return result, nil
}

func requestIsNewer(candidate, current FriendRequest) bool {
	candidateCreated := parseTime(candidate.CreatedAt)
	currentCreated := parseTime(current.CreatedAt)
	if candidateCreated.After(currentCreated) {
		return true
	}
	if candidateCreated.Before(currentCreated) {
		return false
	}
	return requestTimeAfter(candidate.UpdatedAt, current.UpdatedAt)
}

// SupersedeActiveFriendRequests marks older in-flight requests as historical
// when a new request for the same device is created. Requests are identified
// by request_id; this helper never changes terminal records such as accepted.
func SupersedeActiveFriendRequests(ctx context.Context, deviceID, direction string) error {
	if direction == "" {
		return exec(ctx, `UPDATE friend_requests SET status='superseded', updated_at=? WHERE device_id=? AND status IN ('queued', 'sent', 'pending')`, nowString(), deviceID)
	}
	return exec(ctx, `UPDATE friend_requests SET status='superseded', updated_at=? WHERE device_id=? AND direction=? AND status IN ('queued', 'sent', 'pending')`, nowString(), deviceID, direction)
}

func ClearFriendRequestHistory(ctx context.Context) error {
	database := db.DB()
	if database == nil {
		return fmt.Errorf("数据库尚未初始化")
	}
	_, err := database.Exec(ctx, `DELETE FROM friend_requests`)
	return err
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
	result, err := query(ctx, `SELECT message_id, conversation_id, sender_device_id, kind, content, status, created_at, is_favorite, deleted_at, quote_message_id, quote_content, forwarded_from FROM messages WHERE conversation_id=? AND deleted_at='' ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		message := messageFromRow(row)
		if row.Kind == "file" {
			var attachments []attachmentRow
			if attachmentResult, attachmentErr := query(ctx, `SELECT attachment_id, message_id, file_name, mime_type, file_size, sha256, thumbnail_data, thumbnail_mime, local_path, status FROM attachments WHERE message_id=? LIMIT 1`, row.MessageID); attachmentErr == nil && attachmentResult.Structs(&attachments) == nil && len(attachments) > 0 {
				attachment := attachments[0]
				message.AttachmentID, message.AttachmentName, message.AttachmentSize, message.AttachmentMime, message.AttachmentStatus, message.AttachmentPath = attachment.AttachmentID, attachment.FileName, attachment.FileSize, attachment.MimeType, attachment.Status, attachment.LocalPath
				message.AttachmentThumbnail, message.AttachmentThumbnailMime = attachment.ThumbnailData, attachment.ThumbnailMime
			}
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func ListConversationAttachments(ctx context.Context, peerDeviceID string) ([]ConversationAttachment, error) {
	var rows []struct {
		AttachmentID   string `orm:"attachment_id"`
		MessageID      string `orm:"message_id"`
		FileName       string `orm:"file_name"`
		MimeType       string `orm:"mime_type"`
		FileSize       int64  `orm:"file_size"`
		SHA256         string `orm:"sha256"`
		ThumbnailData  string `orm:"thumbnail_data"`
		ThumbnailMime  string `orm:"thumbnail_mime"`
		LocalPath      string `orm:"local_path"`
		Status         string `orm:"status"`
		SenderDeviceID string `orm:"sender_device_id"`
	}
	result, err := query(ctx, `SELECT a.attachment_id, a.message_id, a.file_name, a.mime_type, a.file_size, a.sha256, a.thumbnail_data, a.thumbnail_mime, a.local_path, a.status, m.sender_device_id
		FROM attachments a JOIN messages m ON m.message_id=a.message_id JOIN conversations c ON c.conversation_id=m.conversation_id
		WHERE c.peer_device_id=? ORDER BY a.created_at, a.attachment_id`, peerDeviceID)
	if err != nil {
		return nil, err
	}
	if err := result.Structs(&rows); err != nil {
		return nil, err
	}
	attachments := make([]ConversationAttachment, 0, len(rows))
	for _, row := range rows {
		attachments = append(attachments, ConversationAttachment{
			Attachment:     Attachment{AttachmentID: row.AttachmentID, MessageID: row.MessageID, FileName: row.FileName, MimeType: row.MimeType, FileSize: row.FileSize, SHA256: row.SHA256, ThumbnailData: row.ThumbnailData, ThumbnailMime: row.ThumbnailMime, LocalPath: row.LocalPath, Status: row.Status},
			SenderDeviceID: row.SenderDeviceID,
		})
	}
	return attachments, nil
}

func DeleteConversationRecords(ctx context.Context, peerDeviceID string) (int, int, error) {
	database := db.DB()
	if database == nil {
		return 0, 0, fmt.Errorf("数据库尚未初始化")
	}
	var messageRows []struct {
		Count int `orm:"count"`
	}
	result, err := query(ctx, `SELECT COUNT(*) AS count FROM messages m JOIN conversations c ON c.conversation_id=m.conversation_id WHERE c.peer_device_id=?`, peerDeviceID)
	if err != nil {
		return 0, 0, err
	}
	if err := result.Structs(&messageRows); err != nil {
		return 0, 0, err
	}
	var attachmentRows []struct {
		Count int `orm:"count"`
	}
	result, err = query(ctx, `SELECT COUNT(*) AS count FROM attachments a JOIN messages m ON m.message_id=a.message_id JOIN conversations c ON c.conversation_id=m.conversation_id WHERE c.peer_device_id=?`, peerDeviceID)
	if err != nil {
		return 0, 0, err
	}
	if err := result.Structs(&attachmentRows); err != nil {
		return 0, 0, err
	}
	if err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`DELETE FROM attachments WHERE message_id IN (SELECT m.message_id FROM messages m JOIN conversations c ON c.conversation_id=m.conversation_id WHERE c.peer_device_id=?)`, peerDeviceID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id IN (SELECT conversation_id FROM conversations WHERE peer_device_id=?)`, peerDeviceID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM conversations WHERE peer_device_id=?`, peerDeviceID)
		return err
	}); err != nil {
		return 0, 0, err
	}
	messageCount, attachmentCount := 0, 0
	if len(messageRows) > 0 {
		messageCount = messageRows[0].Count
	}
	if len(attachmentRows) > 0 {
		attachmentCount = attachmentRows[0].Count
	}
	return messageCount, attachmentCount, nil
}

func DeletePeerAndFriendRecords(ctx context.Context, deviceID string) error {
	database := db.DB()
	if database == nil {
		return fmt.Errorf("数据库尚未初始化")
	}
	var identity struct {
		RelationshipVersion    string `orm:"relationship_version"`
		PublicKeyPEM           string `orm:"public_key_pem"`
		CertificateFingerprint string `orm:"certificate_fingerprint"`
	}
	if rows, err := query(ctx, `SELECT relationship_version, public_key_pem, certificate_fingerprint FROM peers WHERE device_id=? LIMIT 1`, deviceID); err == nil {
		var values []struct {
			RelationshipVersion    string `orm:"relationship_version"`
			PublicKeyPEM           string `orm:"public_key_pem"`
			CertificateFingerprint string `orm:"certificate_fingerprint"`
		}
		if rows.Structs(&values) == nil && len(values) > 0 {
			identity.RelationshipVersion = values[0].RelationshipVersion
			identity.PublicKeyPEM = values[0].PublicKeyPEM
			identity.CertificateFingerprint = values[0].CertificateFingerprint
		}
	}
	if identity.RelationshipVersion == "" {
		identity.RelationshipVersion = newID()
	}
	return database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`INSERT INTO friend_removals(device_id, removed_at, relationship_version, public_key_pem, certificate_fingerprint) VALUES(?, ?, ?, ?, ?) ON CONFLICT(device_id) DO UPDATE SET removed_at=excluded.removed_at, relationship_version=excluded.relationship_version, public_key_pem=CASE WHEN excluded.public_key_pem='' THEN friend_removals.public_key_pem ELSE excluded.public_key_pem END, certificate_fingerprint=CASE WHEN excluded.certificate_fingerprint='' THEN friend_removals.certificate_fingerprint ELSE excluded.certificate_fingerprint END`, deviceID, nowString(), identity.RelationshipVersion, identity.PublicKeyPEM, identity.CertificateFingerprint); err != nil {
			return err
		}
		// Keep the request history so a later re-add can create a new request_id
		// and be shown independently of the old accepted/rejected request. Only
		// in-flight requests are superseded by the relationship removal.
		if _, err := tx.Exec(`UPDATE friend_requests SET status='superseded', updated_at=? WHERE device_id=? AND status IN ('queued','sent','pending')`, nowString(), deviceID); err != nil {
			return err
		}
		// Keep the peer row and its relationship to the local conversation. The
		// tombstone makes it unusable as a friend, while the retained row lets
		// the friends list show the existing chat as “不是好友”.
		_, err := tx.Exec(`UPDATE peers SET relation='discovered', discovery_visible=0, visible_in_friends=1, updated_at=? WHERE device_id=?`, nowString(), deviceID)
		return err
	})
}

func IsFriendRemoved(ctx context.Context, deviceID string) (bool, error) {
	// Some in-memory protocol tests construct an Engine without opening the
	// database. In a running application the database is always initialized;
	// preserve the engine's in-memory behavior for those callers.
	if db.DB() == nil {
		return false, nil
	}
	rows, err := query(ctx, `SELECT device_id FROM friend_removals WHERE device_id=? LIMIT 1`, deviceID)
	if err != nil {
		return false, err
	}
	var values []struct {
		DeviceID string `orm:"device_id"`
	}
	if err := rows.Structs(&values); err != nil {
		return false, err
	}
	return len(values) > 0, nil
}

func MarkFriendRemoved(ctx context.Context, deviceID string) error {
	return MarkFriendRemovedWithVersion(ctx, deviceID, "", "", "")
}

func MarkFriendRemovedWithVersion(ctx context.Context, deviceID, version, publicKey, certificateFingerprint string) error {
	if version == "" {
		version = newID()
	}
	return exec(ctx, `INSERT INTO friend_removals(device_id, removed_at, relationship_version, public_key_pem, certificate_fingerprint) VALUES(?, ?, ?, ?, ?) ON CONFLICT(device_id) DO UPDATE SET removed_at=excluded.removed_at, relationship_version=excluded.relationship_version, public_key_pem=CASE WHEN excluded.public_key_pem='' THEN friend_removals.public_key_pem ELSE excluded.public_key_pem END, certificate_fingerprint=CASE WHEN excluded.certificate_fingerprint='' THEN friend_removals.certificate_fingerprint ELSE excluded.certificate_fingerprint END`, deviceID, nowString(), version, publicKey, certificateFingerprint)
}

func FriendRemovalInfo(ctx context.Context, deviceID string) (version, publicKey, certificateFingerprint, removedAt string, err error) {
	rows, err := query(ctx, `SELECT relationship_version, public_key_pem, certificate_fingerprint, removed_at FROM friend_removals WHERE device_id=? LIMIT 1`, deviceID)
	if err != nil {
		return "", "", "", "", err
	}
	var values []struct {
		RelationshipVersion    string `orm:"relationship_version"`
		PublicKeyPEM           string `orm:"public_key_pem"`
		CertificateFingerprint string `orm:"certificate_fingerprint"`
		RemovedAt              string `orm:"removed_at"`
	}
	if err := rows.Structs(&values); err != nil || len(values) == 0 {
		if err != nil {
			return "", "", "", "", err
		}
		return "", "", "", "", fmt.Errorf("friend_removal_not_found")
	}
	return values[0].RelationshipVersion, values[0].PublicKeyPEM, values[0].CertificateFingerprint, values[0].RemovedAt, nil
}

func ClearFriendRemoval(ctx context.Context, deviceID string) error {
	return exec(ctx, `DELETE FROM friend_removals WHERE device_id=?`, deviceID)
}

func GetMessage(ctx context.Context, messageID string) (Message, error) {
	var rows []messageRow
	result, err := query(ctx, `SELECT message_id, conversation_id, sender_device_id, kind, content, status, created_at, is_favorite, deleted_at, quote_message_id, quote_content, forwarded_from FROM messages WHERE message_id=? AND deleted_at='' LIMIT 1`, messageID)
	if err != nil {
		return Message{}, err
	}
	if err := result.Structs(&rows); err != nil {
		return Message{}, err
	}
	if len(rows) == 0 {
		return Message{}, fmt.Errorf("message_not_found")
	}
	row := rows[0]
	message := messageFromRow(row)
	if row.Kind == "file" {
		var attachments []attachmentRow
		if attachmentResult, attachmentErr := query(ctx, `SELECT attachment_id, message_id, file_name, mime_type, file_size, sha256, thumbnail_data, thumbnail_mime, local_path, status FROM attachments WHERE message_id=? LIMIT 1`, row.MessageID); attachmentErr == nil && attachmentResult.Structs(&attachments) == nil && len(attachments) > 0 {
			attachment := attachments[0]
			message.AttachmentID, message.AttachmentName, message.AttachmentSize, message.AttachmentMime, message.AttachmentStatus, message.AttachmentPath = attachment.AttachmentID, attachment.FileName, attachment.FileSize, attachment.MimeType, attachment.Status, attachment.LocalPath
			message.AttachmentThumbnail, message.AttachmentThumbnailMime = attachment.ThumbnailData, attachment.ThumbnailMime
		}
	}
	return message, nil
}

func SaveMessage(ctx context.Context, message Message) error {
	if err := exec(ctx, `INSERT INTO messages(message_id, conversation_id, sender_device_id, kind, content, status, created_at, is_favorite, deleted_at, quote_message_id, quote_content, forwarded_from) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(message_id) DO UPDATE SET content=excluded.content, status=excluded.status, is_favorite=excluded.is_favorite, deleted_at=excluded.deleted_at, quote_message_id=excluded.quote_message_id, quote_content=excluded.quote_content, forwarded_from=excluded.forwarded_from`, message.MessageID, message.ConversationID, message.SenderDeviceID, message.Kind, message.Content, message.Status, message.CreatedAt, boolInt(message.IsFavorite), message.DeletedAt, message.QuoteMessageID, message.QuoteContent, message.ForwardedFrom); err != nil {
		return err
	}
	return exec(ctx, `UPDATE conversations SET last_message=?, last_message_at=?, updated_at=? WHERE conversation_id=?`, message.Content, message.CreatedAt, nowString(), message.ConversationID)
}

func messageFromRow(row messageRow) Message {
	return Message{MessageID: row.MessageID, ConversationID: row.ConversationID, SenderDeviceID: row.SenderDeviceID, Kind: row.Kind, Content: row.Content, Status: row.Status, CreatedAt: row.CreatedAt, IsFavorite: row.IsFavorite != 0, DeletedAt: row.DeletedAt, QuoteMessageID: row.QuoteMessageID, QuoteContent: row.QuoteContent, ForwardedFrom: row.ForwardedFrom}
}

func UpdateMessageLocalState(ctx context.Context, messageID string, favorite bool, deletedAt string) error {
	return exec(ctx, `UPDATE messages SET is_favorite=?, deleted_at=? WHERE message_id=?`, boolInt(favorite), deletedAt, messageID)
}

func DeleteMessageRecord(ctx context.Context, messageID string) error {
	return exec(ctx, `DELETE FROM messages WHERE message_id=?`, messageID)
}

func IncrementConversationUnread(ctx context.Context, conversationID string) error {
	return exec(ctx, `UPDATE conversations SET unread_count=unread_count+1, updated_at=? WHERE conversation_id=?`, nowString(), conversationID)
}

func ClearConversationUnread(ctx context.Context, conversationID string) error {
	return exec(ctx, `UPDATE conversations SET unread_count=0, updated_at=? WHERE conversation_id=?`, nowString(), conversationID)
}

func MarkConversationUnread(ctx context.Context, conversationID string) error {
	return exec(ctx, `UPDATE conversations SET unread_count=CASE WHEN unread_count < 1 THEN 1 ELSE unread_count END, updated_at=? WHERE conversation_id=?`, nowString(), conversationID)
}

func SetConversationPinned(ctx context.Context, conversationID string, pinned bool) error {
	return exec(ctx, `UPDATE conversations SET pinned=?, updated_at=? WHERE conversation_id=?`, boolInt(pinned), nowString(), conversationID)
}

func UpdateMessageStatus(ctx context.Context, messageID, status string) error {
	return exec(ctx, `UPDATE messages SET status=? WHERE message_id=?`, status, messageID)
}

func RecoverSendingMessages(ctx context.Context, senderDeviceID string) error {
	// There is no automatic retry path. Keep interrupted local sends visible as
	// failed so the user can explicitly retry them from the conversation.
	if err := exec(ctx, `UPDATE messages SET status='failed' WHERE sender_device_id=? AND status='sending'`, senderDeviceID); err != nil {
		return err
	}
	return exec(ctx, `UPDATE attachments SET status='failed' WHERE status='sending' AND message_id IN (SELECT message_id FROM messages WHERE sender_device_id=?)`, senderDeviceID)
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
	return exec(ctx, `INSERT INTO attachments(attachment_id, message_id, file_name, mime_type, file_size, sha256, thumbnail_data, thumbnail_mime, local_path, status, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(attachment_id) DO UPDATE SET thumbnail_data=CASE WHEN excluded.thumbnail_data != '' THEN excluded.thumbnail_data ELSE attachments.thumbnail_data END, thumbnail_mime=CASE WHEN excluded.thumbnail_mime != '' THEN excluded.thumbnail_mime ELSE attachments.thumbnail_mime END, local_path=excluded.local_path, status=excluded.status`, attachment.AttachmentID, attachment.MessageID, attachment.FileName, attachment.MimeType, attachment.FileSize, attachment.SHA256, attachment.ThumbnailData, attachment.ThumbnailMime, attachment.LocalPath, attachment.Status, nowString())
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

func UpdateAttachmentThumbnail(ctx context.Context, attachmentID, thumbnailData, thumbnailMime string) error {
	return exec(ctx, `UPDATE attachments SET thumbnail_data=?, thumbnail_mime=? WHERE attachment_id=?`, thumbnailData, thumbnailMime, attachmentID)
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

func GetAttachment(ctx context.Context, id string) (Attachment, error) {
	var rows []attachmentRow
	result, err := query(ctx, `SELECT attachment_id, message_id, file_name, mime_type, file_size, sha256, thumbnail_data, thumbnail_mime, local_path, status FROM attachments WHERE attachment_id=?`, id)
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
	return Attachment{AttachmentID: row.AttachmentID, MessageID: row.MessageID, FileName: row.FileName, MimeType: row.MimeType, FileSize: row.FileSize, SHA256: row.SHA256, ThumbnailData: row.ThumbnailData, ThumbnailMime: row.ThumbnailMime, LocalPath: row.LocalPath, Status: row.Status}, nil
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
