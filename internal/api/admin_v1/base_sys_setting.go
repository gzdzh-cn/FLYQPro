package admin_v1

import "github.com/gogf/gf/v2/frame/g"

type BaseSysSettingKVItem struct {
	Key   string `json:"key" v:"required#配置键不能为空"`
	Value string `json:"value"`
}

type BaseSysSettingKVListReq struct {
	g.Meta `path:"/kv/list" method:"GET"`
}

type BaseSysSettingKVSaveReq struct {
	g.Meta `path:"/kv/save" method:"POST"`
	Items  []BaseSysSettingKVItem `json:"items"`
}
