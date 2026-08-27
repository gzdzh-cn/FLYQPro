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
	if s.chatService == nil {
		return fmt.Errorf("共享预览服务尚未初始化")
	}
	profile := s.chatService.engine.Profile()
	entry, _, err := chat.GetSharedEntry(profile.SharedRootPath, relativePath, false)
	if err != nil {
		return err
	}
	if entry.IsDirectory || (!isImageMime(entry.MimeType, entry.Name) && !isPDFMime(entry.MimeType, entry.Name)) {
		return fmt.Errorf("该文件类型不支持在线预览")
	}
	query := url.Values{}
	query.Set("source", "shared-owner")
	query.Set("relativePath", filepath.ToSlash(relativePath))
	return s.openViewer(query, "共享文件预览")
}

func (s *ImageViewerService) OpenFriendSharedPreview(deviceID, relativePath string) error {
	if s.chatService == nil {
		return fmt.Errorf("共享预览服务尚未初始化")
	}
	entry, err := s.chatService.GetFriendSharedEntryDetails(deviceID, relativePath)
	if err != nil {
		return err
	}
	if entry.IsDirectory || (!isImageMime(entry.MimeType, entry.Name) && !isPDFMime(entry.MimeType, entry.Name)) {
		return fmt.Errorf("该文件类型不支持在线预览")
	}
	query := url.Values{}
	query.Set("source", "shared-friend")
	query.Set("deviceId", strings.TrimSpace(deviceID))
	query.Set("relativePath", filepath.ToSlash(relativePath))
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

	window := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             imageViewerWindowName,
		Title:            title,
		Width:            1100,
		Height:           760,
		MinWidth:         720,
		MinHeight:        480,
		InitialPosition:  application.WindowCentered,
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
