package main

import (
	"embed"
	"log"
	"runtime"

	"flyqpro/internal/service"
	"flyqpro/internal/service/db"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := db.Open(gctx.New()); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(gctx.New()); err != nil {
			log.Printf("关闭 SQLite 失败: %v", err)
		}
	}()

	chatService := service.NewChatService()
	backgroundType := application.BackgroundTypeSolid
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		// Let the themed page own the pixels behind its rounded corners. A solid
		// native background would otherwise show the startup dark colour there
		// when the user switches to the light theme.
		backgroundType = application.BackgroundTypeTransparent
	}
	app := application.New(application.Options{
		Name:        "FlyQPro",
		Description: "局域网点对点聊天工具",
		Services: []application.Service{
			application.NewService(chatService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "FlyQPro",
		Frameless:        runtime.GOOS == "darwin",
		Width:            1100,
		Height:           700,
		MinWidth:         980,
		MinHeight:        640,
		BackgroundType:   backgroundType,
		BackgroundColour: application.NewRGBA(15, 17, 21, 255),
		Windows: application.WindowsWindow{
			Theme:                  0,
			NonClientRegionSupport: true,
		},
		Mac: application.MacWindow{
			Backdrop:     application.MacBackdropTransparent,
			CornerType:   application.MacWindowCornerTypeRounded,
			CornerRadius: 16,
			TitleBar:     application.MacTitleBarHiddenInsetUnified,
		},
		MinimiseButtonState: application.ButtonEnabled,
		MaximiseButtonState: application.ButtonEnabled,
		CloseButtonState:    application.ButtonEnabled,
		URL:                 "/",
	})

	if err := chatService.Start(); err != nil {
		log.Fatal(err)
	}
	defer chatService.Stop()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
