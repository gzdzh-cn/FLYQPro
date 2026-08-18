package main

import (
	"embed"
	_ "embed"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	// _ "dzhgo/packed"
)

// Wails 使用 Go 的 `embed` 包将前端文件嵌入到二进制文件中。
// frontend/dist 文件夹中的所有文件都会被嵌入到二进制文件中，并
// 可供前端使用。
// 更多信息请参见 https://pkg.go.dev/embed。

//go:embed all:frontend/dist
var assets embed.FS

var ctx = gctx.New()

// main 函数作为应用程序的入口点。它初始化应用程序，创建窗口，
// 并启动一个每秒发出基于时间事件的 goroutine。随后运行应用程序，
// 并记录可能发生的任何错误。
func main() {

	// gres.Dump()
	gs := NewGreetService(ctx)
	gs.SetLogger()

	// 通过提供必要的选项创建一个新的 Wails 应用程序。
	// 变量 'Name' 和 'Description' 用于应用程序元数据。
	// 'Assets' 配置了资源服务器，'FS' 变量指向前端文件。
	// 'Bind' 是 Go 结构体实例的列表，前端可以访问这些实例的方法。
	// 'Mac' 选项用于在 macOS 上运行时定制应用程序。
	app := application.New(application.Options{
		Name:        "dzh3159",
		Description: "A demo of using raw HTML & CSS",
		Services: []application.Service{
			application.NewService(&GreetService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		// app.Logger.Info("Application started!!")
		g.Log().Info(ctx, "Application started!!")
	})

	// 使用必要的选项创建一个新窗口。
	// 'Title' 是窗口标题。
	// 'Mac' 选项用于在 macOS 上运行时定制窗口。
	// 'BackgroundColour' 是窗口的背景色。
	// 'URL' 是将加载到 webview 中的地址。
	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title: "Window 1",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
		Width:            1024,
		Height:           768,
	})

	env := app.Environment()
	g.Log().Infof(ctx, "env: %+v", env)

	gs.StartGfServer()

	// 运行应用程序。该操作会阻塞，直到应用程序退出。
	err := app.Run()

	// 如果运行应用程序时发生错误，则记录错误并退出。
	if err != nil {
		// app.Logger.Error("run app error: %s", err.Error())
		g.Log().Fatalf(ctx, "run app error: %s", err)
	}
}
