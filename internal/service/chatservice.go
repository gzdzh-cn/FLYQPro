package service

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
	"helpfly/internal/chat"
	"helpfly/internal/platform/startup"
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
	return s.UpdateProfile(profile)
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
	profile.FileSavePath = path
	return s.UpdateProfile(profile)
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
func (s *ChatService) ListPeers() []chat.Peer         { return s.engine.Peers() }
func (s *ChatService) ScanPeers()                     { s.engine.Scan() }
func (s *ChatService) ListFriends() []chat.Peer {
	peers, _ := chat.ListPeers(gctx.New(), chat.PeerRelation)
	return peers
}
func (s *ChatService) ListFriendRequests() []chat.FriendRequest {
	requests, _ := chat.ListFriendRequests(gctx.New(), "pending")
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
func (s *ChatService) SendFile(deviceID, path string) (chat.Message, error) {
	return s.engine.SendFile(gctx.New(), deviceID, path)
}
func (s *ChatService) AcceptAttachment(attachmentID string) (chat.Attachment, error) {
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
	target := filepath.Join(profile.FileSavePath, filepath.Base(attachment.FileName))
	if err := os.Rename(attachment.LocalPath, target); err != nil {
		return attachment, err
	}
	attachment.LocalPath, attachment.Status = target, "saved"
	if err := chat.SaveAttachment(gctx.New(), attachment); err != nil {
		return attachment, err
	}
	return attachment, nil
}
func (s *ChatService) RejectAttachment(attachmentID string) error {
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
