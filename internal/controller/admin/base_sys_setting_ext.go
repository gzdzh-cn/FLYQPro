package admin

import (
	"context"

	v1 "dzhgo/internal/api/admin_v1"
	logic "dzhgo/internal/logic/sys"

	"github.com/gzdzh-cn/dzhcore"
)

type BaseSysSettingExtController struct {
	*dzhcore.ControllerSimple
}

func init() {
	dzhcore.AddControllerSimple(&BaseSysSettingExtController{
		ControllerSimple: &dzhcore.ControllerSimple{Prefix: "/admin/base/sys/settingExt"},
	})
}

func (c *BaseSysSettingExtController) Modules(ctx context.Context, req *v1.SettingExtModulesReq) (res *dzhcore.BaseRes, err error) {
	data, err := logic.InstalledModules(ctx)
	if err != nil {
		return nil, err
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysSettingExtController) SaveModule(ctx context.Context, req *v1.SettingExtSaveModuleReq) (res *dzhcore.BaseRes, err error) {
	if err = logic.SaveModule(ctx, req.Module, req.Config); err != nil {
		return nil, err
	}
	return dzhcore.Ok(nil), nil
}
