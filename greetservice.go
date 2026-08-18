package main

import (
	"context"
	"dzhgo/internal/cmd"
	"encoding/base64"
	"errors"
	"os"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/gzdzh-cn/dzhcore/utility/env"
	"github.com/gzdzh-cn/dzhcore/utility/util"
)

type GreetService struct {
	ctx context.Context
}

func NewGreetService(ctx context.Context) *GreetService {
	gs := &GreetService{
		ctx: ctx,
	}

	return gs
}

func (gs *GreetService) Greet(name string) string {
	return "Hello " + name + "!"
}
func (gs *GreetService) Shutdown(ctx context.Context) {
	g.Log().Info(ctx, "GreetService Shutdown")
}

func (gs *GreetService) SetLogger() {
	type Config struct {
		IsProd      bool
		AppName     string
		IsDesktop   bool
		ConfigMap   g.Map
		defaultPath string
	}
	config := &Config{}
	config.defaultPath = env.GetCfgWithDefault(ctx, "core.gfLogger.path", g.NewVar("path")).String()
	config.IsProd = env.GetCfgWithDefault(ctx, "core.isProd", g.NewVar(false)).Bool()
	config.AppName = env.GetCfgWithDefault(ctx, "core.appName", g.NewVar("dzhgo")).String()
	config.IsDesktop = env.GetCfgWithDefault(ctx, "core.isDesktop", g.NewVar(false)).Bool()
	logPath := util.NewToolUtil().GetLoggerPath(config.IsProd, config.AppName, config.IsDesktop, config.defaultPath)
	config.ConfigMap = g.Map{
		"path":     logPath,
		"file":     env.GetCfgWithDefault(ctx, "core.gfLogger.file", g.NewVar("{Y-m-d}.log")).String(),
		"level":    env.GetCfgWithDefault(ctx, "core.gfLogger.level", g.NewVar("debug")).String(),
		"stdout":   env.GetCfgWithDefault(ctx, "core.gfLogger.stdout", g.NewVar(true)).Bool(),
		"flags":    env.GetCfgWithDefault(ctx, "core.gfLogger.flags", g.NewVar(44)).Int(),
		"stStatus": env.GetCfgWithDefault(ctx, "core.gfLogger.stStatus", g.NewVar(1)).Int(),
		"stSkip":   env.GetCfgWithDefault(ctx, "core.gfLogger.stSkip", g.NewVar(0)).Int(),
	}
	g.Log().SetConfigWithMap(config.ConfigMap)
}

func (gs *GreetService) StartGfServer() {
	// 启动 goframe 服务
	go cmd.Main.Run(ctx)
}

func (gs *GreetService) UploadFile(file string) string {
	return "dzh"
}

// GetLocalImage 读取本地图片并返回 base64 编码
func (gs *GreetService) GetLocalImage(filePath string) (string, error) {
	// 检查文件是否存在
	if !gfile.Exists(filePath) {
		return "", errors.New("文件不存在: " + filePath)
	}

	// 读取文件内容
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 获取文件扩展名来确定 MIME 类型
	ext := gfile.Ext(filePath)
	var mimeType string
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	default:
		mimeType = "image/jpeg" // 默认
	}

	// 将文件内容转换为 base64
	base64Data := base64.StdEncoding.EncodeToString(fileContent)

	// 返回 data URL 格式
	return "data:" + mimeType + ";base64," + base64Data, nil
}
