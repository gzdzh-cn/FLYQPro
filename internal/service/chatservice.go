package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flyqpro/internal/chat"
	"flyqpro/internal/platform/startup"
	"flyqpro/internal/version"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ChatService struct{ engine *chat.Engine }

func NewChatService() *ChatService { return &ChatService{engine: chat.NewEngine()} }

func (s *ChatService) Start() error { return s.engine.Start(gctx.New()) }
func (s *ChatService) Stop()        { s.engine.Stop() }

func (s *ChatService) GetProfile() (chat.Profile, error) {
	profile, err := chat.GetProfile(gctx.New())
	if err != nil {
		return profile, err
	}
	return s.profileWithAvatar(profile), nil
}

func (s *ChatService) profileWithAvatar(profile chat.Profile) chat.Profile {
	if profile.AvatarPath == "" {
		return profile
	}
	data, err := os.ReadFile(profile.AvatarPath)
	if err != nil || len(data) > 5*1024*1024 {
		return profile
	}
	if profile.AvatarHash == "" {
		sum := sha256.Sum256(data)
		profile.AvatarHash = hex.EncodeToString(sum[:])
	}
	ext := strings.TrimPrefix(filepath.Ext(profile.AvatarPath), ".")
	if ext == "" {
		ext = "png"
	}
	profile.AvatarData = "data:image/" + ext + ";base64," + base64.StdEncoding.EncodeToString(data)
	return profile
}

func (s *ChatService) UpdateProfile(profile chat.Profile) (chat.Profile, error) {
	if strings.TrimSpace(profile.Nickname) == "" {
		return chat.Profile{}, fmt.Errorf("昵称不能为空")
	}
	if profile.Theme == "" {
		profile.Theme = "system"
	}
	if profile.FileSavePath == "" {
		profile.FileSavePath = chat.DefaultAttachmentDir()
	}
	if s.engine.IsAttachmentMigrationActive() {
		current := s.engine.Profile()
		if filepath.Clean(profile.FileSavePath) != filepath.Clean(current.FileSavePath) || profile.AutoSave != current.AutoSave {
			return chat.Profile{}, fmt.Errorf("附件迁移正在进行")
		}
	}
	if err := os.MkdirAll(profile.FileSavePath, 0o700); err != nil {
		return chat.Profile{}, err
	}
	if err := chat.SaveProfile(gctx.New(), profile); err != nil {
		return chat.Profile{}, err
	}
	s.engine.UpdateProfile(profile)
	return s.profileWithAvatar(profile), nil
}

func (s *ChatService) SetAvatar(sourcePath string) (chat.Profile, error) {
	info, err := os.Stat(sourcePath)
	if err != nil || info.IsDir() {
		return chat.Profile{}, fmt.Errorf("头像文件不存在")
	}
	if info.Size() > 5*1024*1024 {
		return chat.Profile{}, fmt.Errorf("头像不能超过 5 MB")
	}
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return chat.Profile{}, fmt.Errorf("头像仅支持 PNG、JPG、WEBP")
	}
	input, err := os.Open(sourcePath)
	if err != nil {
		return chat.Profile{}, err
	}
	defer input.Close()
	targetPath := filepath.Join(chat.AppDataDir(), "avatar"+ext)
	output, err := os.Create(targetPath)
	if err != nil {
		return chat.Profile{}, err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return chat.Profile{}, err
	}
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	profile.AvatarPath = targetPath
	sum := sha256.Sum256(inputBytesForAvatar(targetPath))
	profile.AvatarHash = hex.EncodeToString(sum[:])
	profile.AvatarVersion++
	return s.UpdateProfile(profile)
}

func inputBytesForAvatar(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

func (s *ChatService) ResetAvatar() (chat.Profile, error) {
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	if profile.AvatarPath != "" {
		if removeErr := os.Remove(profile.AvatarPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return profile, fmt.Errorf("删除头像失败: %w", removeErr)
		}
	}
	profile.AvatarPath = ""
	profile.AvatarData = ""
	profile.AvatarHash = ""
	profile.AvatarVersion++
	return s.UpdateProfile(profile)
}

func (s *ChatService) SetDiscoverable(value bool) (chat.Profile, error) {
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	profile.Discoverable = value
	return s.UpdateProfile(profile)
}
func (s *ChatService) SetAutoSave(value bool) (chat.Profile, error) {
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	profile.AutoSave = value
	return s.UpdateProfile(profile)
}
func (s *ChatService) SetTheme(theme string) (chat.Profile, error) {
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	profile.Theme = theme
	return s.UpdateProfile(profile)
}
func (s *ChatService) SetFileSavePath(path string) (chat.Profile, error) {
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	if filepath.Clean(strings.TrimSpace(path)) == filepath.Clean(profile.FileSavePath) {
		return profile, nil
	}
	if _, err := s.MigrateAttachmentStorage(path); err != nil {
		return profile, err
	}
	return s.GetProfile()
}

func (s *ChatService) DefaultAttachmentPath() string { return chat.DefaultAttachmentDir() }

func (s *ChatService) MigrateAttachmentStorage(targetRoot string) (chat.AttachmentMigrationResult, error) {
	if s.engine.IsAttachmentMigrationActive() {
		return chat.AttachmentMigrationResult{}, fmt.Errorf("附件迁移正在进行")
	}
	profile, err := chat.GetProfile(gctx.New())
	if err != nil {
		return chat.AttachmentMigrationResult{}, err
	}
	oldRoot := filepath.Clean(profile.FileSavePath)
	targetRoot = filepath.Clean(strings.TrimSpace(targetRoot))
	if targetRoot == "." || targetRoot == "" {
		return chat.AttachmentMigrationResult{}, fmt.Errorf("保存路径不能为空")
	}
	s.engine.SetAttachmentMigrationActive(true)
	var completion *chat.AttachmentMigrationProgress
	defer func() {
		s.engine.SetAttachmentMigrationActive(false)
		// Files received while the migration lock was active stayed in the
		// application temp directory. Once the final profile root is known,
		// archive only those eligible files into the new root.
		s.engine.ArchivePendingAttachments()
		if completion != nil {
			if app := application.Get(); app != nil {
				app.Event.Emit("chat:attachment-migration", *completion)
			}
		}
	}()
	report := func(progress chat.AttachmentMigrationProgress) {
		// The profile path is not switched until after the file loop and its
		// deferred temp-file archive complete. Emit completion only then so the
		// frontend remains blocked for the entire atomic operation.
		if progress.Phase == "completed" {
			completion = &progress
			return
		}
		if app := application.Get(); app != nil {
			app.Event.Emit("chat:attachment-migration", progress)
		}
	}
	result, err := chat.MigrateAttachments(gctx.New(), oldRoot, targetRoot, report)
	if err != nil {
		return result, err
	}
	profile.FileSavePath = targetRoot
	if err := chat.SaveProfile(gctx.New(), profile); err != nil {
		completion = &chat.AttachmentMigrationProgress{Phase: "failed", SourceRoot: oldRoot, TargetRoot: targetRoot, Current: result.Total, Total: result.Total, Migrated: result.Migrated, Skipped: result.Skipped, Failed: result.Failed + 1, Unclassified: result.Unclassified, ErrorMessage: err.Error()}
		return result, err
	}
	s.engine.UpdateProfile(profile)
	return result, nil
}

func (s *ChatService) SetLaunchAtStartup(value bool) (chat.Profile, error) {
	profile, err := s.GetProfile()
	if err != nil {
		return profile, err
	}
	executable, err := os.Executable()
	if err != nil {
		return profile, err
	}
	if err := startup.Set(value, executable); err != nil {
		return profile, err
	}
	profile.LaunchAtStartup = value
	return s.UpdateProfile(profile)
}

func (s *ChatService) GetDeviceInfo() chat.DeviceInfo { return s.engine.DeviceInfo() }
func (s *ChatService) GetAppVersion() string          { return version.AppVersion }
func (s *ChatService) ListPeers() []chat.Peer         { return s.engine.Peers() }
func (s *ChatService) ScanPeers()                     { s.engine.Scan() }
func (s *ChatService) ListFriends() []chat.Peer       { return s.engine.PeersByRelation(chat.PeerRelation) }
func (s *ChatService) ListFriendRequests() []chat.FriendRequest {
	// Return the full local request history. The frontend derives the pending
	// count separately so accepted/rejected requests remain visible without
	// being treated as actionable notifications.
	requests, _ := chat.ListFriendRequests(gctx.New(), "")
	return requests
}
func (s *ChatService) ListConversations() []chat.Conversation {
	conversations, _ := chat.ListConversations(gctx.New())
	return conversations
}
func (s *ChatService) ListMessages(conversationID string) []chat.Message {
	messages, _ := chat.ListMessages(gctx.New(), conversationID)
	return messages
}

func (s *ChatService) SendFriendRequest(deviceID, message string) (chat.FriendRequest, error) {
	return s.engine.SendFriendRequest(gctx.New(), deviceID, message)
}
func (s *ChatService) AcceptFriendRequest(requestID string) error {
	return s.engine.AcceptFriendRequest(gctx.New(), requestID)
}
func (s *ChatService) RejectFriendRequest(requestID string) error {
	return s.engine.RejectFriendRequest(gctx.New(), requestID)
}
func (s *ChatService) SendMessage(deviceID, content string) (chat.Message, error) {
	return s.engine.SendMessage(gctx.New(), deviceID, content)
}

func (s *ChatService) RetryMessage(messageID string) (chat.Message, error) {
	return s.engine.RetryMessage(gctx.New(), messageID)
}

func (s *ChatService) MarkConversationRead(deviceID string) error {
	return s.engine.MarkConversationRead(gctx.New(), deviceID)
}
func (s *ChatService) SendFile(deviceID, path string) (chat.Message, error) {
	return s.engine.SendFile(gctx.New(), deviceID, path)
}

func (s *ChatService) RetryAttachment(messageID string) (chat.Message, error) {
	return s.engine.RetryAttachment(gctx.New(), messageID)
}

func (s *ChatService) SendImage(deviceID, dataURL string) (chat.Message, error) {
	if !strings.HasPrefix(dataURL, "data:image/") {
		return chat.Message{}, fmt.Errorf("图片数据无效")
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 || !strings.Contains(parts[0], ";base64") {
		return chat.Message{}, fmt.Errorf("图片数据无效")
	}
	mimeType := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:")
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}[mimeType]
	if ext == "" {
		return chat.Message{}, fmt.Errorf("不支持的图片格式")
	}
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil || len(data) == 0 || len(data) > 100*1024*1024 {
		return chat.Message{}, fmt.Errorf("图片大小无效")
	}
	tempDir := filepath.Join(chat.AppDataDir(), "temp")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return chat.Message{}, err
	}
	path := filepath.Join(tempDir, "clipboard-"+fmt.Sprintf("%d", time.Now().UnixNano())+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return chat.Message{}, err
	}
	return s.engine.SendFile(gctx.New(), deviceID, path)
}

func (s *ChatService) GetAttachmentPreview(attachmentID string) (string, error) {
	attachment, err := chat.GetAttachment(gctx.New(), attachmentID)
	if err != nil || attachment.LocalPath == "" {
		return "", fmt.Errorf("图片预览不可用")
	}
	data, err := os.ReadFile(attachment.LocalPath)
	if err != nil || len(data) == 0 || len(data) > 20*1024*1024 {
		return "", fmt.Errorf("图片预览不可用")
	}
	mimeType := attachment.MimeType
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = mime.TypeByExtension(filepath.Ext(attachment.FileName))
	}
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("图片预览不可用")
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
func (s *ChatService) AcceptAttachment(attachmentID string) (chat.Attachment, error) {
	if s.engine.IsAttachmentMigrationActive() {
		return chat.Attachment{}, fmt.Errorf("附件迁移正在进行")
	}
	attachment, err := chat.GetAttachment(gctx.New(), attachmentID)
	if err != nil {
		return attachment, err
	}
	profile, err := chat.GetProfile(gctx.New())
	if err != nil {
		return attachment, err
	}
	if attachment.Status == "saved" {
		return attachment, nil
	}
	if err := os.MkdirAll(profile.FileSavePath, 0o700); err != nil {
		return attachment, err
	}
	peerDeviceID, _ := chat.AttachmentPeerDeviceID(gctx.New(), attachmentID)
	target, err := chat.AttachmentTargetPath(profile.FileSavePath, peerDeviceID, attachment.FileName)
	if err != nil {
		return attachment, err
	}
	return s.saveAttachmentToPath(attachment, target)
}

// SaveAttachmentAs lets a user choose the final path for a pending attachment.
// The temporary download is moved only after the user confirms the destination.
func (s *ChatService) SaveAttachmentAs(attachmentID string) (chat.Attachment, error) {
	if s.engine.IsAttachmentMigrationActive() {
		return chat.Attachment{}, fmt.Errorf("附件迁移正在进行")
	}
	attachment, err := chat.GetAttachment(gctx.New(), attachmentID)
	if err != nil {
		return attachment, err
	}
	if attachment.Status == "saved" {
		return attachment, nil
	}
	if attachment.Status != "pending" || strings.TrimSpace(attachment.LocalPath) == "" {
		return attachment, fmt.Errorf("附件当前不可保存")
	}
	result, err := application.Get().Dialog.SaveFile().SetMessage("请选择附件保存位置").SetFilename(attachment.FileName).PromptForSingleSelection()
	if err != nil {
		return attachment, err
	}
	if strings.TrimSpace(result) == "" {
		return attachment, nil
	}
	return s.saveAttachmentToPath(attachment, filepath.Clean(result))
}

func (s *ChatService) saveAttachmentToPath(attachment chat.Attachment, target string) (chat.Attachment, error) {
	oldPath := attachment.LocalPath
	if strings.TrimSpace(oldPath) == "" {
		return attachment, fmt.Errorf("附件暂存文件不存在")
	}
	if strings.TrimSpace(target) == "" {
		return attachment, fmt.Errorf("附件保存路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return attachment, err
	}
	if err := os.Rename(oldPath, target); err != nil {
		return attachment, err
	}
	attachment.LocalPath, attachment.Status = target, "saved"
	if err := chat.SaveAttachment(gctx.New(), attachment); err != nil {
		_ = os.Rename(target, oldPath)
		return attachment, err
	}
	return attachment, nil
}

func (s *ChatService) RejectAttachment(attachmentID string) error {
	if s.engine.IsAttachmentMigrationActive() {
		return fmt.Errorf("附件迁移正在进行")
	}
	attachment, err := chat.GetAttachment(gctx.New(), attachmentID)
	if err != nil {
		return err
	}
	if attachment.LocalPath != "" {
		_ = os.Remove(attachment.LocalPath)
	}
	attachment.Status = "rejected"
	return chat.SaveAttachment(gctx.New(), attachment)
}
func (s *ChatService) SetPeerRemark(deviceID, remark string) error {
	return chat.SetPeerRemark(gctx.New(), deviceID, remark)
}
func (s *ChatService) EnsureConversation(deviceID string) (string, error) {
	return chat.EnsureConversation(gctx.New(), deviceID)
}

type conversationFileMove struct {
	source string
	target string
}

func (s *ChatService) ClearConversation(deviceID string) (chat.ClearConversationResult, error) {
	var result chat.ClearConversationResult
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return result, fmt.Errorf("好友设备 ID 不能为空")
	}
	if deviceID == s.engine.DeviceInfo().DeviceID {
		return result, fmt.Errorf("不能清除本机聊天记录")
	}
	if s.engine.IsAttachmentMigrationActive() {
		return result, fmt.Errorf("附件迁移正在进行")
	}

	ctx := gctx.New()
	s.engine.CancelIncomingForPeer(deviceID)
	attachments, err := chat.ListConversationAttachments(ctx, deviceID)
	if err != nil {
		return result, err
	}
	profile, err := chat.GetProfile(ctx)
	if err != nil {
		return result, err
	}
	roots := []string{profile.FileSavePath, chat.DefaultAttachmentDir(), filepath.Join(chat.AppDataDir(), "temp")}
	trashRoot := filepath.Join(chat.AppDataDir(), "temp", "chat-clear", fmt.Sprintf("%d", time.Now().UnixNano()))
	moves := make([]conversationFileMove, 0, len(attachments))
	seen := make(map[string]struct{}, len(attachments))
	rollback := func() {
		for index := len(moves) - 1; index >= 0; index-- {
			move := moves[index]
			if _, statErr := os.Stat(move.target); statErr == nil {
				_ = os.MkdirAll(filepath.Dir(move.source), 0o700)
				_ = os.Rename(move.target, move.source)
			}
		}
		_ = os.RemoveAll(trashRoot)
	}

	for _, item := range attachments {
		path := strings.TrimSpace(item.LocalPath)
		if path == "" {
			continue
		}
		// The sender's path is the user's original file, not an application
		// copy. Never remove it as part of clearing local chat history.
		if item.SenderDeviceID == s.engine.DeviceInfo().DeviceID {
			result.SkippedExternalFiles++
			continue
		}
		managed := false
		for _, root := range roots {
			if chat.IsPathWithin(path, root) {
				managed = true
				break
			}
		}
		if !managed {
			result.SkippedExternalFiles++
			continue
		}
		cleanPath, absErr := filepath.Abs(path)
		if absErr != nil {
			rollback()
			return result, absErr
		}
		if _, already := seen[cleanPath]; already {
			continue
		}
		seen[cleanPath] = struct{}{}
		if _, statErr := os.Stat(cleanPath); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			rollback()
			return result, statErr
		}
		if err := os.MkdirAll(trashRoot, 0o700); err != nil {
			rollback()
			return result, err
		}
		target := filepath.Join(trashRoot, fmt.Sprintf("%04d-%s", len(moves), filepath.Base(cleanPath)))
		if err := os.Rename(cleanPath, target); err != nil {
			rollback()
			return result, fmt.Errorf("暂存附件失败: %w", err)
		}
		moves = append(moves, conversationFileMove{source: cleanPath, target: target})
		result.DeletedFiles++
	}

	deletedMessages, deletedAttachments, err := chat.DeleteConversationRecords(ctx, deviceID)
	if err != nil {
		rollback()
		return chat.ClearConversationResult{}, err
	}
	result.DeletedMessages = deletedMessages
	result.DeletedAttachments = deletedAttachments
	_ = os.RemoveAll(trashRoot)
	return result, nil
}

func (s *ChatService) PickFile() string {
	result, err := application.Get().Dialog.OpenFile().SetTitle("选择要发送的文件").CanChooseFiles(true).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return result
}

func (s *ChatService) PickDirectory() string {
	result, err := application.Get().Dialog.OpenFile().SetTitle("选择文件保存目录").CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return result
}

func (s *ChatService) NetworkStatus() chat.NetworkStatus { return s.engine.NetworkStatus() }

func (s *ChatService) RunNetworkDiagnostic() chat.DiagnosticResult {
	status := s.engine.NetworkStatus()
	items := []chat.DiagnosticItem{
		{Name: "可用网卡", Status: statusOf(len(status.Interfaces) > 0), Detail: strings.Join(status.Interfaces, ", "), Advice: "请连接 Wi-Fi 或有线网络。"},
		{Name: "局域网 IP", Status: statusOf(len(status.LocalIPs) > 0), Detail: strings.Join(status.LocalIPs, ", "), Advice: "请检查系统网络配置。"},
		{Name: "UDP 发现端口", Status: statusOf(status.DiscoveryPort > 0), Detail: fmt.Sprintf("UDP %d", status.DiscoveryPort), Advice: "如果没有设备响应，可能被防火墙或 AP 隔离阻止。"},
		{Name: "TCP 发现端口", Status: statusOf(status.DiscoveryPort > 0), Detail: fmt.Sprintf("TCP %d", status.DiscoveryPort), Advice: "请允许应用通过系统防火墙接收 TCP 连接。"},
		{Name: "发现响应处理", Status: statusOf(status.LastError == ""), Detail: discoverySendDetail(status.LastError), Advice: "请检查设备身份数据或本地数据库状态。"},
		{Name: "TCP/TLS 聊天端口", Status: statusOf(status.ChatPort > 0), Detail: fmt.Sprintf("TCP %d", status.ChatPort), Advice: "请允许应用通过系统防火墙。"},
		{Name: "已发现设备", Status: statusOf(status.PeerCount > 0), Detail: fmt.Sprintf("%d 台，在线 %d 台", status.PeerCount, status.OnlineCount), Advice: "请确认双方处于同一局域网且已开启允许被发现。"},
	}
	resultStatus := "normal"
	for _, item := range items {
		if item.Status == "error" {
			resultStatus = "error"
			break
		}
		if item.Status == "warning" {
			resultStatus = "warning"
		}
	}
	return chat.DiagnosticResult{Status: resultStatus, Items: items, CreatedAt: status.LastScanAt}
}

func discoverySendDetail(lastError string) string {
	if lastError != "" {
		return "发送失败: " + lastError
	}
	return "广播、UDP 单播和 TCP 探测已发送"
}

func statusOf(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

func (s *ChatService) ClearApplicationData() error {
	return fmt.Errorf("请在设置页面确认后执行，当前版本暂不提供自动删除身份数据")
}
