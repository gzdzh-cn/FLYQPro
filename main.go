package main

import (
	"embed"
	"log"
	"runtime"

	"flyqpro/internal/service"
	"flyqpro/internal/service/db"
	"flyqpro/internal/version"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
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
	dockService := dock.New()
	appBadgeService := service.NewAppBadgeService(dockService)
	backgroundType := application.BackgroundTypeSolid
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		// Let the themed page own the pixels behind its rounded corners. A solid
		// native background would otherwise show the startup dark colour there
		// when the user switches to the light theme.
		backgroundType = application.BackgroundTypeTransparent
	}
	app := application.New(application.Options{
		Name:        "飞秋Pro",
		Description: "版本：v" + version.AppVersion + "\n技术栈：Go、Wails v3、Vue 3、TypeScript、Arco Design、SQLite\n技术支持：广州大智汇信息科技有限公司",
		Services: []application.Service{
			application.NewService(chatService),
			application.NewService(dockService),
			application.NewService(appBadgeService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	app.RegisterService(application.NewService(service.NewImageViewerService(app, chatService)))
	app.RegisterService(application.NewService(service.NewSharedDriveWindowService(app, chatService)))
	app.RegisterService(application.NewServiceWithOptions(service.NewPreviewStreamService(chatService), application.ServiceOptions{Route: "/preview/"}))
	configureApplicationMenu(app)
	configureNativeApplicationName(app)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "flyqpro-main",
		Title:            "飞秋Pro",
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

// configureApplicationMenu replaces Wails' English About role on macOS with
// a localized menu item. The package/binary name remains FlyQPro so existing
// DMG names and installation paths do not change.
func configureApplicationMenu(app *application.App) {
	if runtime.GOOS != "darwin" {
		return
	}

	menu := application.NewMenu()
	appMenu := menu.AddSubmenu("飞秋Pro")
	appMenu.Add("关于飞秋Pro").OnClick(func(_ *application.Context) {
		app.Menu.ShowAbout()
	})
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	menu.AddRole(application.FileMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)
	app.Menu.Set(menu)
}
