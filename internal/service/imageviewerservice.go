package service

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"flyqpro/internal/chat"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const imageViewerWindowName = "flyqpro-image-viewer"

// ImageViewerService owns the optional native image viewer window. The main
// chat window only passes database identifiers; the viewer reads the image
// through the existing bound services after it has loaded.
type ImageViewerService struct {
	app         *application.App
	chatService *ChatService
	mu          sync.Mutex
}

func NewImageViewerService(app *application.App, chatService *ChatService) *ImageViewerService {
	return &ImageViewerService{app: app, chatService: chatService}
}

func isImageMime(mimeType, fileName string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".avif", ".bmp", ".gif", ".heic", ".heif", ".jpeg", ".jpg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func isVideoMime(mimeType, fileName string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "video/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".3gp", ".avi", ".flv", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".ogv", ".ts", ".webm", ".wmv":
		return true
	default:
		return false
	}
}

func (s *ImageViewerService) OpenImageViewer(conversationID string, messageID string) error {
	conversationID = strings.TrimSpace(conversationID)
	messageID = strings.TrimSpace(messageID)
	if conversationID == "" || messageID == "" {
		return fmt.Errorf("图片预览参数不能为空")
	}

	message, err := chat.GetMessage(gctx.New(), messageID)
	if err != nil {
		return fmt.Errorf("消息不存在")
	}
	if message.ConversationID != conversationID {
		return fmt.Errorf("图片不属于当前会话")
	}
	if message.Kind != "file" || message.AttachmentID == "" || !isImageMime(message.AttachmentMime, message.AttachmentName) {
		return fmt.Errorf("消息不是图片附件")
	}

	query := url.Values{}
	query.Set("conversationId", conversationID)
	query.Set("messageId", messageID)
	return s.openViewer(query, "图片预览")
}

func (s *ImageViewerService) OpenSharedPreview(relativePath string) error {
	return s.openSharedPreviewWithMetadata(relativePath, "", 0, "")
}

// OpenSharedPreviewFast accepts the metadata already present in the shared
// list. It lets the viewer address the same remote thumbnail cache entry
// instead of starting a second path-only lookup.
func (s *ImageViewerService) OpenSharedPreviewFast(relativePath, entryID string, fileSize int64, modifiedAt string) error {
	return s.openSharedPreviewWithMetadata(relativePath, entryID, fileSize, modifiedAt)
}

func (s *ImageViewerService) openSharedPreviewWithMetadata(relativePath, entryID string, fileSize int64, modifiedAt string) error {
	if s.chatService == nil {
		return fmt.Errorf("共享预览服务尚未初始化")
	}
	profile := s.chatService.engine.Profile()
	entry, _, err := chat.GetSharedEntry(profile.SharedRootPath, relativePath, false)
	if err != nil {
		return err
	}
	if entry.IsDirectory || (!isImageMime(entry.MimeType, entry.Name) && !isPDFMime(entry.MimeType, entry.Name) && !isVideoMime(entry.MimeType, entry.Name)) {
		return fmt.Errorf("该文件类型不支持在线预览")
	}
	query := url.Values{}
	query.Set("source", "shared-owner")
	query.Set("relativePath", filepath.ToSlash(relativePath))
	if entryID != "" {
		query.Set("entryId", entryID)
	}
	if fileSize > 0 {
		query.Set("fileSize", fmt.Sprintf("%d", fileSize))
	}
	if modifiedAt != "" {
		query.Set("modifiedAt", modifiedAt)
	}
	if isPDFMime(entry.MimeType, entry.Name) {
		query.Set("previewType", "pdf")
	} else if isVideoMime(entry.MimeType, entry.Name) {
		query.Set("previewType", "video")
	} else {
		query.Set("previewType", "image")
	}
	return s.openViewer(query, "共享文件预览")
}

func (s *ImageViewerService) OpenFriendSharedPreview(deviceID, relativePath string) error {
	return s.openFriendSharedPreviewWithMetadata(deviceID, relativePath, "", 0, "")
}

// OpenFriendSharedPreviewFast carries the shared-entry identity to the
// viewer. The old method remains for older frontend bundles.
func (s *ImageViewerService) OpenFriendSharedPreviewFast(deviceID, relativePath, entryID string, fileSize int64, modifiedAt string) error {
	return s.openFriendSharedPreviewWithMetadata(deviceID, relativePath, entryID, fileSize, modifiedAt)
}

func (s *ImageViewerService) openFriendSharedPreviewWithMetadata(deviceID, relativePath, entryID string, fileSize int64, modifiedAt string) error {
	if s.chatService == nil {
		return fmt.Errorf("共享预览服务尚未初始化")
	}
	deviceID = strings.TrimSpace(deviceID)
	friends, err := chat.ListPeers(gctx.New(), chat.PeerRelation)
	if err != nil {
		return err
	}
	knownFriend := false
	for _, peer := range friends {
		if peer.DeviceID == deviceID {
			knownFriend = true
			if !peer.Online {
				return fmt.Errorf("好友不在线，暂不支持打开共享盘")
			}
			break
		}
	}
	if !knownFriend {
		return fmt.Errorf("不是好友，无法访问共享盘")
	}
	if !isImageMime("", relativePath) && !isPDFMime("", relativePath) && !isVideoMime("", relativePath) {
		return fmt.Errorf("该文件类型不支持在线预览")
	}
	query := url.Values{}
	query.Set("source", "shared-friend")
	query.Set("deviceId", deviceID)
	query.Set("relativePath", filepath.ToSlash(relativePath))
	if entryID != "" {
		query.Set("entryId", entryID)
	}
	if fileSize > 0 {
		query.Set("fileSize", fmt.Sprintf("%d", fileSize))
	}
	if modifiedAt != "" {
		query.Set("modifiedAt", modifiedAt)
	}
	if isPDFMime("", relativePath) {
		query.Set("previewType", "pdf")
	} else if isVideoMime("", relativePath) {
		query.Set("previewType", "video")
	} else {
		query.Set("previewType", "image")
	}
	return s.openViewer(query, "好友共享预览")
}

func isPDFMime(mimeType, fileName string) bool {
	return strings.EqualFold(strings.TrimSpace(mimeType), "application/pdf") || strings.EqualFold(filepath.Ext(fileName), ".pdf")
}

func (s *ImageViewerService) openViewer(query url.Values, title string) error {
	viewerURL := "/#/image-viewer?" + query.Encode()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return fmt.Errorf("图片查看器尚未初始化")
	}
	if existing, ok := s.app.Window.GetByName(imageViewerWindowName); ok {
		existing.SetTitle(title)
		existing.SetURL(viewerURL)
		existing.Show()
		existing.Focus()
		return nil
	}

	x, y := 40, 40
	if mainWindow, ok := s.app.Window.GetByName(mainWindowName); ok {
		mainX, mainY := mainWindow.Position()
		mainWidth, mainHeight := mainWindow.Size()
		if mainWidth > 0 && mainHeight > 0 {
			x, y = mainX+40, mainY+40
		}
	}
	window := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             imageViewerWindowName,
		Title:            title,
		Width:            1100,
		Height:           760,
		MinWidth:         720,
		MinHeight:        480,
		InitialPosition:  application.WindowXY,
		X:                x,
		Y:                y,
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGBA(15, 17, 21, 255),
		// Both desktop platforms use a fully custom title bar. macOS draws its
		// traffic-light controls in the viewer page so no native edge can leak
		// through above the themed header.
		Frameless: runtime.GOOS == "windows" || runtime.GOOS == "darwin",
		URL:       viewerURL,
		Windows: application.WindowsWindow{
			NonClientRegionSupport: true,
		},
		Mac: application.MacWindow{
			CornerType: application.MacWindowCornerTypeSquare,
		},
	})
	window.Show()
	window.Focus()
	return nil
}
