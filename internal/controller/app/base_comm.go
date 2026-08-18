package app

import (
	"context"
	v1 "dzhgo/internal/api/app_v1"
	"dzhgo/internal/config"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gzdzh-cn/dzhcore"
)

type BaseCommController struct {
	*dzhcore.ControllerSimple
}

func init() {
	var baseCommController = &BaseCommController{
		&dzhcore.ControllerSimple{
			Prefix: "/app/base/comm",
		},
	}
	// 注册路由
	dzhcore.RegisterControllerSimple(baseCommController)
}

// 获取系统设置信息（官网、客服电话等）
func (c *BaseCommController) SettingInfo(ctx context.Context, req *v1.BaseCommSettingInfoReq) (res *dzhcore.BaseRes, err error) {
	values := config.GetSettingValues(ctx, "mobile", "websiteUrl", "siteName", "logo", "copyright")
	if values == nil {
		return dzhcore.Ok(nil), nil
	}
	data := g.Map{
		"mobile":     values["mobile"],
		"websiteUrl": values["websiteUrl"],
		"siteName":   values["siteName"],
		"logo":       values["logo"],
		"copyright":  values["copyright"],
	}
	return dzhcore.Ok(data), nil
}
