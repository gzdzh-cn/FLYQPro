package config

import (
	"context"
	"dzhgo/internal/defineType"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// sBaseConfig 配置
type sBaseConfig struct {
	WxConfig *defineType.WxConfig
}

// 公众号信息
func GetWxCf(ctx context.Context) (data *defineType.WxConfig) {
	values := GetSettingValues(ctx, "wxAppId", "wxSecret")
	if values == nil {
		return nil
	}
	return &defineType.WxConfig{
		Appid:     values["wxAppId"],
		Secret:    values["wxSecret"],
		GrantType: "authorization_code",
	}
}

// GetSettingValues reads selected values from the base_sys_setting key/value table.
func GetSettingValues(ctx context.Context, keys ...string) map[string]string {
	values := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return values
	}
	rows, err := g.DB().Model("base_sys_setting").Ctx(ctx).
		Fields("`key`,`value`").
		WhereIn("`key`", keys).
		Where("deleted_at IS NULL").
		All()
	if err == nil {
		for _, row := range rows {
			values[strings.TrimSpace(row["key"].String())] = row["value"].String()
		}
		return values
	}

	// 兼容旧版 SQLite 宽表：旧数据库将站点配置直接存放在
	// base_sys_setting 的各个列中，没有新版 KV 表所需的 key/value 列。
	// 不修改或重建用户数据库，按请求的配置键读取旧表中的同名列。
	legacyRow, legacyErr := g.DB().Model("base_sys_setting").Ctx(ctx).
		Fields("*").
		Where("deleted_at IS NULL").
		OrderAsc("updateTime").
		One()
	if legacyErr != nil {
		g.Log().Error(ctx, err)
		g.Log().Error(ctx, legacyErr)
		return nil
	}
	for _, key := range keys {
		for column, value := range legacyRow {
			if strings.EqualFold(column, key) {
				values[key] = value.String()
				break
			}
		}
	}
	return values
}

func GetSettingValue(ctx context.Context, key string) string {
	return GetSettingValues(ctx, key)[key]
}

func GetAutoConfig() (data *defineType.AutoPhone) {
	return &defineType.AutoPhone{
		RequestUrl: "https://api.weixin.qq.com/wxa/business/getuserphonenumber",
	}
}

func GetAccessToken() (data *defineType.AccessToken) {
	return &defineType.AccessToken{
		RequestUrl: "https://api.weixin.qq.com/cgi-bin/token",
		Appid:      "wx7a3c7f891ab07e34",
		Secret:     "51cdfc9e7570c5ac19581f7795fb27c0",
		GrantType:  "client_credential",
	}
}

func NewBaseConfig() *sBaseConfig {

	return &sBaseConfig{
		WxConfig: GetWxCf(ctx),
	}

}

//var BaseConfig = NewBaseConfig()
