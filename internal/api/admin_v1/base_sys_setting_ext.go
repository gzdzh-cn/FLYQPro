package admin_v1

import "github.com/gogf/gf/v2/frame/g"

type SettingExtModulesReq struct {
	g.Meta `path:"/modules" method:"GET"`
}

type SettingExtSaveModuleReq struct {
	g.Meta `path:"/saveModule" method:"POST"`
	Module string                 `json:"module" v:"required#模块不能为空"`
	Config map[string]interface{} `json:"config"`
}
