package service

import (
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"

	"flyqpro/internal/chat"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	mainWindowName        = "flyqpro-main"
	sharedDriveWindowName = "flyqpro-shared-drive"
)

// SharedDriveWindowService owns the non-modal shared-drive window. The file
// APIs remain on ChatService so the window can use the same authenticated
// store and event stream as the main chat window.
type SharedDriveWindowService struct {
	app  *application.App
	chat *ChatService
	mu   sync.Mutex
}

func NewSharedDriveWindowService(app *application.App, chatService *ChatService) *SharedDriveWindowService {
	return &SharedDriveWindowService{app: app, chat: chatService}
}

func (s *SharedDriveWindowService) OpenSharedDrive() error {
	return s.open("owner", "")
}

func (s *SharedDriveWindowService) OpenFriendSharedDrive(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("好友设备 ID 不能为空")
	}
	friends, err := chat.ListPeers(gctx.New(), chat.PeerRelation)
	if err != nil {
		return err
	}
	for _, peer := range friends {
		if peer.DeviceID == deviceID {
			if !peer.Online {
				return fmt.Errorf("好友不在线，暂不支持打开共享盘")
			}
			return s.open("friend", deviceID)
		}
	}
	return fmt.Errorf("不是好友，无法访问共享盘")
}

func (s *SharedDriveWindowService) open(mode, deviceID string) error {
	if s.app == nil {
		return fmt.Errorf("共享窗口尚未初始化")
	}
	query := url.Values{}
	query.Set("mode", mode)
	if deviceID != "" {
		query.Set("deviceId", deviceID)
	}
	windowURL := "/#/shared-drive?" + query.Encode()

	s.mu.Lock()
	defer s.mu.Unlock()
	title := "共享"
	if mode == "friend" {
		title = "好友共享"
	}
	if existing, ok := s.app.Window.GetByName(sharedDriveWindowName); ok {
		existing.SetURL(windowURL)
		existing.SetTitle(title)
		existing.ForceReload()
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
		Name:             sharedDriveWindowName,
		Title:            title,
		Width:            1100,
		Height:           700,
		MinWidth:         980,
		MinHeight:        640,
		InitialPosition:  application.WindowXY,
		X:                x,
		Y:                y,
		Frameless:        runtime.GOOS == "darwin",
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGBA(245, 245, 245, 255),
		Windows:          application.WindowsWindow{NonClientRegionSupport: true},
		Mac: application.MacWindow{
			Backdrop:     application.MacBackdropTransparent,
			CornerType:   application.MacWindowCornerTypeRounded,
			CornerRadius: 18,
			TitleBar:     application.MacTitleBarHiddenInsetUnified,
		},
		URL: windowURL,
	})
	window.Show()
	window.Focus()
	return nil
}
