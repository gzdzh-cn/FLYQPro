package internal

import (
	"dzhgo/internal/model"
	"dzhgo/internal/service"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gzdzh-cn/dzhcore"
	"github.com/gzdzh-cn/dzhcore/coreconfig"

	_ "dzhgo/internal/controller"
	_ "dzhgo/internal/funcs"
	_ "dzhgo/internal/logic"
	_ "dzhgo/internal/middleware"
	_ "dzhgo/internal/packed"
)

var (
	ctx = gctx.GetInitCtx()
)

func init() {

}

func NewInit() {
	g.Log().Debug(ctx, "------------ base init start ...")
	g.Log().Debugf(ctx, "base version:%v", Version)

	dzhcore.FillInitData(ctx, "base", &model.BaseSysMenu{})
	if err := service.BaseSysAddonsService().EnsureSiteBackupMenu(ctx); err != nil {
		panic(err)
	}
	dzhcore.FillInitData(ctx, "base", &model.BaseSysUser{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysUserRole{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysRole{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysRoleMenu{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysDepartment{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysRoleDepartment{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysSetting{})
	dzhcore.FillInitData(ctx, "base", &model.BaseSysAddonsTypes{})

	// 内置插件由 plugin.json 声明，启动时确保已安装并上架。
	if err := service.BaseSysAddonsService().EnsureBuiltinAddons(ctx); err != nil {
		panic(err)
	}
	if err := service.BaseSysAddonsService().ValidateInstalledAddons(ctx); err != nil {
		panic(err)
	}

	if dzhcore.IsRedisMode && coreconfig.Config.Core.Notice.Enable {
		g.Log().Info(ctx, "Redis队列消费者开始启动")
		// 启动队列消费者
		service.BaseSysNoticeService().StartQueue()
	}

	g.Log().Debug(ctx, "------------ base init end ...")
}
