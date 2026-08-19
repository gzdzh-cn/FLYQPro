package main

import (
	"embed"
	"log"

	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
	"helpfly/internal/service"
	"helpfly/internal/service/db"
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
	app := application.New(application.Options{
		Name:        "POPChat",
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
			Title:            "POPChat",
		Width:            1280,
		Height:           800,
		MinWidth:         1100,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(255, 255, 255),
		Windows:          application.WindowsWindow{Theme: 0},
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropNormal,
			TitleBar: application.MacTitleBarDefault,
		},
		URL: "/",
	})

	if err := chatService.Start(); err != nil {
		log.Fatal(err)
	}
	defer chatService.Stop()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
