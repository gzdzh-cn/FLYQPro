package service

import (
	"fmt"
	"net/url"
	"path/filepath"
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
	app *application.App
	mu  sync.Mutex
}

func NewImageViewerService(app *application.App) *ImageViewerService {
	return &ImageViewerService{app: app}
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
	viewerURL := "/#/image-viewer?" + query.Encode()
	title := "图片预览"
	if name := strings.TrimSpace(message.AttachmentName); name != "" {
		title += " - " + name
	}

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
		Frameless:        false,
		URL:              viewerURL,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarDefault,
		},
	})
	window.Show()
	window.Focus()
	return nil
}
