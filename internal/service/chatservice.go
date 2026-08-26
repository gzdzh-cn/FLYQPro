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
	"os/exec"
	"path/filepath"
	"runtime"
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
	if profile.SharedEnabled {
		root, err := chat.ValidateSharedRoot(profile.SharedRootPath)
		if err != nil {
			return chat.Profile{}, fmt.Errorf("共享目录不可用")
		}
		profile.SharedRootPath = root
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

func (s *ChatService) GetSharedFolderSettings() (chat.SharedFolderStatus, error) {
	profile, err := chat.GetProfile(gctx.New())
	if err != nil {
		return chat.SharedFolderStatus{}, err
	}
	status := chat.SharedFolderStatus{SharedFolderSettings: chat.SharedFolderSettings{Enabled: profile.SharedEnabled, RootPath: profile.SharedRootPath}, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if strings.TrimSpace(profile.SharedRootPath) == "" {
		status.Enabled = false
		return status, nil
	}
	root, rootErr := chat.ValidateSharedRoot(profile.SharedRootPath)
	if rootErr != nil {
		if profile.SharedEnabled {
			profile.SharedEnabled = false
			if saveErr := chat.SaveProfile(gctx.New(), profile); saveErr == nil {
				s.engine.UpdateProfile(profile)
			}
		}
		status.Enabled = false
		return status, nil
	}
	status.FileCount, status.FolderCount = sharedRootCounts(root)
	if available, availableErr := chat.AvailableDiskBytes(root); availableErr == nil && available >= 0 {
		status.AvailableBytes = uint64(available)
	}
	return status, nil
}

func sharedRootCounts(root string) (files, folders int) {
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			folders++
		} else {
			files++
		}
		return nil
	})
	return files, folders
}

func (s *ChatService) SetSharedFolder(path string, enabled bool) (chat.SharedFolderStatus, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return chat.SharedFolderStatus{}, fmt.Errorf("共享目录不能为空")
	}
	root, err := chat.ValidateSharedRoot(path)
	if err != nil {
		return chat.SharedFolderStatus{}, fmt.Errorf("共享目录不可用")
	}
	profile, err := chat.GetProfile(gctx.New())
	if err != nil {
		return chat.SharedFolderStatus{}, err
	}
	profile.SharedRootPath = root
	profile.SharedEnabled = enabled
	if err := chat.SaveProfile(gctx.New(), profile); err != nil {
		return chat.SharedFolderStatus{}, err
	}
	s.engine.UpdateProfile(profile)
	return s.GetSharedFolderSettings()
}

func (s *ChatService) SetSharedEnabled(enabled bool) (chat.SharedFolderStatus, error) {
	profile, err := chat.GetProfile(gctx.New())
	if err != nil {
		return chat.SharedFolderStatus{}, err
	}
	if enabled {
		if _, err := chat.ValidateSharedRoot(profile.SharedRootPath); err != nil {
			return chat.SharedFolderStatus{}, fmt.Errorf("请先选择有效的共享目录")
		}
	}
	profile.SharedEnabled = enabled
	if err := chat.SaveProfile(gctx.New(), profile); err != nil {
		return chat.SharedFolderStatus{}, err
	}
	s.engine.UpdateProfile(profile)
	return s.GetSharedFolderSettings()
}

func (s *ChatService) DisableSharedFolder() error {
	_, err := s.SetSharedEnabled(false)
	return err
}

func (s *ChatService) ListSharedEntries(relativePath string) ([]chat.SharedEntry, error) {
	profile := s.engine.Profile()
	return chat.ListSharedEntries(profile.SharedRootPath, relativePath)
}

func (s *ChatService) GetSharedEntryDetails(relativePath string) (chat.SharedEntry, error) {
	profile := s.engine.Profile()
	entry, _, err := chat.GetSharedEntry(profile.SharedRootPath, relativePath, true)
	return entry, err
}

func (s *ChatService) OpenSharedEntry(relativePath string) error {
	profile := s.engine.Profile()
	_, path, err := chat.GetSharedEntry(profile.SharedRootPath, relativePath, false)
	if err != nil {
		return err
	}
	return runAttachmentCommand("open", path)
}

func (s *ChatService) RevealSharedEntry(relativePath string) error {
	profile := s.engine.Profile()
	_, path, err := chat.GetSharedEntry(profile.SharedRootPath, relativePath, false)
	if err != nil {
		return err
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Run()
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", "/select,"+path).Run()
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Run()
}

func (s *ChatService) OpenSharedDownload(targetPath string) error {
	root := filepath.Clean(chat.DefaultSharedDownloadDir())
	target := filepath.Clean(strings.TrimSpace(targetPath))
	if target == "" || !sharedServicePathWithin(root, target) {
		return fmt.Errorf("下载文件路径无效")
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return fmt.Errorf("下载文件不存在")
	}
	return runAttachmentCommand("open", target)
}

func (s *ChatService) RevealSharedDownload(targetPath string) error {
	root := filepath.Clean(chat.DefaultSharedDownloadDir())
	target := filepath.Clean(strings.TrimSpace(targetPath))
	if target == "" || !sharedServicePathWithin(root, target) {
		return fmt.Errorf("下载文件路径无效")
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("下载文件不存在")
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", target).Run()
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", "/select,"+target).Run()
	}
	return exec.Command("xdg-open", filepath.Dir(target)).Run()
}

func sharedServicePathWithin(root, target string) bool {
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	resolvedTarget, targetErr := filepath.EvalSymlinks(target)
	if rootErr != nil || targetErr != nil {
		return false
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedTarget)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *ChatService) CreateSharedFolder(relativePath, name string) (chat.SharedEntry, error) {
	profile := s.engine.Profile()
	return chat.CreateSharedFolder(profile.SharedRootPath, relativePath, name)
}

func (s *ChatService) RenameSharedEntry(relativePath, newName string) error {
	profile := s.engine.Profile()
	return chat.RenameSharedEntry(profile.SharedRootPath, relativePath, newName)
}

func (s *ChatService) MoveSharedEntry(relativePath, targetDirectory string) error {
	profile := s.engine.Profile()
	return chat.MoveSharedEntry(profile.SharedRootPath, relativePath, targetDirectory)
}

func (s *ChatService) CopySharedEntry(relativePath, targetDirectory string) error {
	profile := s.engine.Profile()
	return chat.CopySharedEntry(profile.SharedRootPath, relativePath, targetDirectory)
}

func (s *ChatService) DeleteSharedEntry(relativePath string) error {
	profile := s.engine.Profile()
	return chat.DeleteSharedEntry(profile.SharedRootPath, relativePath)
}

func (s *ChatService) ListFriendSharedEntries(deviceID, relativePath string) ([]chat.SharedEntry, error) {
	return s.engine.ListFriendSharedEntries(gctx.New(), strings.TrimSpace(deviceID), relativePath)
}

func (s *ChatService) DownloadFriendSharedEntry(deviceID, relativePath string) (chat.SharedTransfer, error) {
	targetRoot := chat.DefaultSharedDownloadDir()
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return chat.SharedTransfer{}, err
	}
	return s.engine.DownloadFriendSharedEntry(gctx.New(), strings.TrimSpace(deviceID), relativePath, filepath.Join(targetRoot, filepath.Base(filepath.FromSlash(relativePath))))
}

func (s *ChatService) SaveFriendSharedEntryAs(deviceID, relativePath string) (chat.SharedTransfer, error) {
	name := filepath.Base(filepath.FromSlash(relativePath))
	result, err := application.Get().Dialog.SaveFile().SetMessage("请选择共享文件保存位置").SetFilename(name).PromptForSingleSelection()
	if err != nil || strings.TrimSpace(result) == "" {
		return chat.SharedTransfer{}, err
	}
	return s.engine.DownloadFriendSharedEntry(gctx.New(), strings.TrimSpace(deviceID), relativePath, filepath.Clean(result))
}

func (s *ChatService) CancelSharedTransfer(transferID string) error {
	return s.engine.CancelSharedTransfer(transferID)
}

func (s *ChatService) GetFriendSharedEntryDetails(deviceID, relativePath string) (chat.SharedEntry, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." {
		clean = ""
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(clean)))
	if parent == "." {
		parent = ""
	}
	entries, err := s.ListFriendSharedEntries(deviceID, parent)
	if err != nil {
		return chat.SharedEntry{}, err
	}
	for _, entry := range entries {
		if entry.RelativePath == clean {
			return entry, nil
		}
	}
	return chat.SharedEntry{}, fmt.Errorf("共享文件不存在")
}

func (s *ChatService) ImportSharedFiles(relativePath string) ([]chat.SharedEntry, error) {
	profile := s.engine.Profile()
	if !profile.SharedEnabled {
		return nil, fmt.Errorf("%s", chat.SharedDisabledError)
	}
	root, err := chat.ValidateSharedRoot(profile.SharedRootPath)
	if err != nil {
		return nil, err
	}
	target, normalized, err := resolveSharedDirectory(root, relativePath)
	if err != nil {
		return nil, err
	}
	paths, err := application.Get().Dialog.OpenFile().SetTitle("选择要导入的文件").CanChooseFiles(true).CanChooseDirectories(false).PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	for _, source := range paths {
		if err := importSharedPath(source, target); err != nil {
			return nil, err
		}
	}
	return chat.ListSharedEntries(root, normalized)
}

func (s *ChatService) ImportSharedFolder(relativePath string) ([]chat.SharedEntry, error) {
	profile := s.engine.Profile()
	if !profile.SharedEnabled {
		return nil, fmt.Errorf("%s", chat.SharedDisabledError)
	}
	root, err := chat.ValidateSharedRoot(profile.SharedRootPath)
	if err != nil {
		return nil, err
	}
	target, normalized, err := resolveSharedDirectory(root, relativePath)
	if err != nil {
		return nil, err
	}
	paths, err := application.Get().Dialog.OpenFile().SetTitle("选择要导入的文件夹").CanChooseDirectories(true).CanChooseFiles(false).PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	for _, source := range paths {
		if err := importSharedPath(source, target); err != nil {
			return nil, err
		}
	}
	return chat.ListSharedEntries(root, normalized)
}

func resolveSharedDirectory(root, relative string) (string, string, error) {
	entries, err := chat.ListSharedEntries(root, relative)
	if err != nil {
		return "", "", err
	}
	_ = entries
	path, normalized, err := resolveSharedPathForService(root, relative)
	return path, normalized, err
}

func resolveSharedPathForService(root, relative string) (string, string, error) {
	// Reusing the public listing validates the path without exposing the
	// unexported resolver outside the chat package; the final Stat is still
	// checked before copying.
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))
	if clean == "." {
		clean = ""
	}
	if strings.Contains(relative, `\`) || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("%s", chat.SharedPathInvalidError)
	}
	root, err := chat.ValidateSharedRoot(root)
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(root, clean)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%s", chat.SharedPathInvalidError)
	}
	return path, filepath.ToSlash(clean), nil
}

func importSharedPath(source, targetDirectory string) error {
	info, err := os.Stat(source)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("导入文件不可用")
	}
	target := filepath.Join(targetDirectory, filepath.Base(source))
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("目标名称已存在")
	}
	if info.IsDir() {
		return copyDirectoryForImport(source, target)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func copyDirectoryForImport(source, target string) error {
	if err := os.Mkdir(target, 0o700); err != nil {
		return err
	}
	items, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := importSharedPath(filepath.Join(source, item.Name()), target); err != nil {
			return err
		}
	}
	return nil
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

func (s *ChatService) SendMessageWithMetadata(deviceID, content, quoteMessageID, quoteContent, forwardedFrom string) (chat.Message, error) {
	return s.engine.SendMessageWithMetadata(gctx.New(), deviceID, content, quoteMessageID, quoteContent, forwardedFrom)
}

func (s *ChatService) SetMessageFavorite(messageID string, favorite bool) error {
	message, err := chat.GetMessage(gctx.New(), messageID)
	if err != nil {
		return err
	}
	return chat.UpdateMessageLocalState(gctx.New(), messageID, favorite, message.DeletedAt)
}

func (s *ChatService) DeleteMessage(messageID string) error {
	if strings.TrimSpace(messageID) == "" {
		return fmt.Errorf("消息 ID 不能为空")
	}
	return chat.DeleteMessageRecord(gctx.New(), messageID)
}

func (s *ChatService) attachmentFile(attachmentID string) (chat.Attachment, os.FileInfo, error) {
	attachment, err := chat.GetAttachment(gctx.New(), attachmentID)
	if err != nil {
		return attachment, nil, fmt.Errorf("附件不存在")
	}
	path := strings.TrimSpace(attachment.LocalPath)
	if path == "" {
		return attachment, nil, fmt.Errorf("附件尚未保存在本机")
	}
	if attachment.Status != "sent" && attachment.Status != "saved" {
		return attachment, nil, fmt.Errorf("附件尚未接收完成")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return attachment, nil, fmt.Errorf("本地附件不存在")
	}
	return attachment, info, nil
}

func (s *ChatService) OpenAttachment(attachmentID string) error {
	attachment, _, err := s.attachmentFile(attachmentID)
	if err != nil {
		return err
	}
	return runAttachmentCommand("open", attachment.LocalPath)
}

func (s *ChatService) RevealAttachment(attachmentID string) error {
	attachment, _, err := s.attachmentFile(attachmentID)
	if err != nil {
		return err
	}
	path := filepath.Clean(attachment.LocalPath)
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Run()
	case "windows":
		return exec.Command("explorer.exe", "/select,"+path).Run()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Run()
	}
}

func runAttachmentCommand(action, path string) error {
	path = filepath.Clean(path)
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Run()
	case "windows":
		return exec.Command("explorer.exe", path).Run()
	default:
		return exec.Command("xdg-open", path).Run()
	}
}

func (s *ChatService) SaveAttachmentCopy(attachmentID string) error {
	attachment, _, err := s.attachmentFile(attachmentID)
	if err != nil {
		return err
	}
	result, err := application.Get().Dialog.SaveFile().SetMessage("请选择附件保存位置").SetFilename(attachment.FileName).PromptForSingleSelection()
	if err != nil {
		return err
	}
	if strings.TrimSpace(result) == "" {
		return nil
	}
	input, err := os.Open(attachment.LocalPath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(filepath.Clean(result))
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func (s *ChatService) GetAttachmentDetails(attachmentID string) (chat.AttachmentDetails, error) {
	attachment, _, err := s.attachmentFile(attachmentID)
	if err != nil {
		return chat.AttachmentDetails{}, err
	}
	message, err := chat.GetMessage(gctx.New(), attachment.MessageID)
	if err != nil {
		return chat.AttachmentDetails{}, err
	}
	return chat.AttachmentDetails{AttachmentID: attachment.AttachmentID, FileName: attachment.FileName, MimeType: attachment.MimeType, FileSize: attachment.FileSize, SHA256: attachment.SHA256, Status: attachment.Status, CreatedAt: message.CreatedAt, LocalPath: attachment.LocalPath}, nil
}

func (s *ChatService) MarkConversationRead(deviceID string) error {
	return s.engine.MarkConversationRead(gctx.New(), deviceID)
}
func (s *ChatService) requireFriend(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("好友设备 ID 不能为空")
	}
	peers, err := chat.ListPeers(gctx.New(), chat.PeerRelation)
	if err != nil {
		return err
	}
	for _, peer := range peers {
		if peer.DeviceID == deviceID {
			return nil
		}
	}
	return fmt.Errorf("不是好友")
}
func (s *ChatService) MarkConversationUnread(deviceID string) error {
	if err := s.requireFriend(deviceID); err != nil {
		return err
	}
	conversationID, err := chat.EnsureConversation(gctx.New(), deviceID)
	if err != nil {
		return err
	}
	return chat.MarkConversationUnread(gctx.New(), conversationID)
}
func (s *ChatService) SetConversationPinned(deviceID string, pinned bool) error {
	if err := s.requireFriend(deviceID); err != nil {
		return err
	}
	conversationID, err := chat.EnsureConversation(gctx.New(), deviceID)
	if err != nil {
		return err
	}
	return chat.SetConversationPinned(gctx.New(), conversationID, pinned)
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
	if err != nil {
		return "", fmt.Errorf("图片预览不可用")
	}
	// Prefer the sender-provided/generated thumbnail even when the original
	// file is local. This keeps previews instant and avoids loading a 25 MB
	// image into the WebView just to render a chat bubble.
	if attachment.ThumbnailData != "" && attachment.ThumbnailMime != "" {
		return "data:" + attachment.ThumbnailMime + ";base64," + attachment.ThumbnailData, nil
	}
	if attachment.LocalPath == "" {
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

// GetAttachmentThumbnail is intentionally separate from GetAttachmentImage.
// Chat bubbles should never load a large local original just to render a
// preview, while the image viewer must use the original when it is available.
func (s *ChatService) GetAttachmentThumbnail(attachmentID string) (string, error) {
	attachment, err := chat.GetAttachment(gctx.New(), attachmentID)
	if err != nil || attachment.ThumbnailData == "" || attachment.ThumbnailMime == "" {
		return "", fmt.Errorf("图片缩略图不可用")
	}
	return "data:" + attachment.ThumbnailMime + ";base64," + attachment.ThumbnailData, nil
}

func (s *ChatService) GetAttachmentImage(attachmentID string) (string, error) {
	attachment, info, err := s.attachmentFile(attachmentID)
	if err != nil {
		return "", err
	}
	mimeType := attachment.MimeType
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = mime.TypeByExtension(filepath.Ext(attachment.FileName))
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("附件不是图片")
	}
	if info.Size() <= 0 || info.Size() > 100*1024*1024 {
		return "", fmt.Errorf("图片大小不支持预览")
	}
	data, err := os.ReadFile(attachment.LocalPath)
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("原图读取失败")
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
	return s.engine.AcceptIncomingAttachment(gctx.New(), attachmentID, target)
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
	if attachment.Status != "pending" {
		return attachment, fmt.Errorf("附件当前不可保存")
	}
	result, err := application.Get().Dialog.SaveFile().SetMessage("请选择附件保存位置").SetFilename(attachment.FileName).PromptForSingleSelection()
	if err != nil {
		return attachment, err
	}
	if strings.TrimSpace(result) == "" {
		return attachment, nil
	}
	return s.engine.AcceptIncomingAttachment(gctx.New(), attachmentID, filepath.Clean(result))
}

func (s *ChatService) RejectAttachment(attachmentID string) error {
	return s.engine.RejectIncomingAttachment(attachmentID)
}

func (s *ChatService) CancelAttachment(attachmentID string) error {
	if s.engine.IsAttachmentMigrationActive() {
		return fmt.Errorf("附件迁移正在进行")
	}
	return s.engine.CancelAttachment(attachmentID)
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

func (s *ChatService) HideFriendAndClearLocalData(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if err := s.requireFriend(deviceID); err != nil {
		return err
	}
	conversations, err := chat.ListConversations(gctx.New())
	if err != nil {
		return err
	}
	pinned := false
	for _, conversation := range conversations {
		if conversation.PeerDeviceID == deviceID {
			pinned = conversation.Pinned
			break
		}
	}
	if _, err := s.ClearConversation(deviceID); err != nil {
		return err
	}
	conversationID, err := chat.EnsureConversation(gctx.New(), deviceID)
	if err != nil {
		return err
	}
	if pinned {
		if err := chat.SetConversationPinned(gctx.New(), conversationID, true); err != nil {
			return err
		}
	}
	return s.engine.SetPeerVisibleInFriends(gctx.New(), deviceID, false)
}

func (s *ChatService) RemoveFriendAndClearLocalData(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if err := s.requireFriend(deviceID); err != nil {
		return err
	}
	if _, err := s.ClearConversation(deviceID); err != nil {
		return err
	}
	if err := chat.DeletePeerAndFriendRecords(gctx.New(), deviceID); err != nil {
		return err
	}
	s.engine.RemovePeerFromMemory(deviceID)
	return nil
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

func (s *ChatService) PickSharedDirectory() (string, error) {
	dialog := application.Get().Dialog.OpenFile().
		SetTitle("选择共享目录").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true)
	if window, ok := application.Get().Window.GetByName(sharedDriveWindowName); ok {
		dialog.AttachToWindow(window)
	}
	if current := strings.TrimSpace(s.engine.Profile().SharedRootPath); current != "" {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			dialog.SetDirectory(current)
		}
	}
	result, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("选择共享目录失败: %w", err)
	}
	return strings.TrimSpace(result), nil
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
