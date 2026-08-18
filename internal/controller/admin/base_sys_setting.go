package admin

import (
	"context"

	v1 "dzhgo/internal/api/admin_v1"
	logic "dzhgo/internal/logic/sys"

	"github.com/gzdzh-cn/dzhcore"
)

type BaseSysSettingController struct {
	*dzhcore.ControllerSimple
}

func init() {
	dzhcore.AddControllerSimple(&BaseSysSettingController{
		ControllerSimple: &dzhcore.ControllerSimple{Prefix: "/admin/base/sys/setting"},
	})
}

func (c *BaseSysSettingController) KVList(ctx context.Context, req *v1.BaseSysSettingKVListReq) (res *dzhcore.BaseRes, err error) {
	data, err := logic.ListBaseSysSettingKV(ctx)
	if err != nil {
		return nil, err
	}
	return dzhcore.Ok(map[string]interface{}{"items": data}), nil
}

func (c *BaseSysSettingController) KVSave(ctx context.Context, req *v1.BaseSysSettingKVSaveReq) (res *dzhcore.BaseRes, err error) {
	if err = logic.SaveBaseSysSettingKV(ctx, req.Items); err != nil {
		return nil, err
	}
	return dzhcore.Ok(nil), nil
}
