package sys

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	baseConfig "dzhgo/internal/config"
	"dzhgo/internal/dao"
	"dzhgo/internal/model"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gzdzh-cn/dzhcore/coreconfig"
	"github.com/gzdzh-cn/dzhcore/defineStruct"
)

const customerProSettingModule = "customer_pro"
const cloudStorageSettingModule = "cloud_storage"

// CloudStorageOssConfig is the database-backed OSS configuration used by
// application uploads. It intentionally matches dzhcore's OSS config shape so
// callers can use it without reading manifest/config.yaml.
type CloudStorageOssConfig = defineStruct.OssConfig

// GetCloudStorageOssConfig reads OSS settings from the base_sys_setting KV table.
func GetCloudStorageOssConfig(ctx context.Context) (CloudStorageOssConfig, error) {
	values := baseConfig.GetSettingValues(ctx,
		"ossEndpoint", "ossAccessKeyID", "ossSecretAccessKey", "ossBucketName",
		"ossUseSSL", "ossLocation",
	)
	if values == nil {
		return CloudStorageOssConfig{}, gerror.New("读取云存储配置失败")
	}
	return CloudStorageOssConfig{
		Endpoint:        values["ossEndpoint"],
		AccessKeyID:     values["ossAccessKeyID"],
		SecretAccessKey: values["ossSecretAccessKey"],
		BucketName:      values["ossBucketName"],
		UseSSL:          gconv.Bool(values["ossUseSSL"]),
		Location:        values["ossLocation"],
	}, nil
}

// GetCloudStorageOssAudioPath returns the OSS object prefix used for call
// recordings. The default keeps compatibility with existing objects under
// the audio/ prefix.
func GetCloudStorageOssAudioPath(ctx context.Context) (string, error) {
	audioPath := strings.TrimSpace(baseConfig.GetSettingValue(ctx, "ossAudioPath"))
	if audioPath == "" {
		return "audio", nil
	}

	audioPath = strings.ReplaceAll(audioPath, "\\", "/")
	audioPath = strings.Trim(audioPath, "/")
	if audioPath == "" {
		return "audio", nil
	}
	for _, segment := range strings.Split(audioPath, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", gerror.New("OSS录音目录不能包含空目录、.或..路径段")
		}
	}
	return audioPath, nil
}

// ApplyCloudStorageOssConfig refreshes dzhcore's runtime OSS settings before
// legacy file drivers are used. The source of truth remains the database.
func ApplyCloudStorageOssConfig(ctx context.Context) (CloudStorageOssConfig, error) {
	cfg, err := GetCloudStorageOssConfig(ctx)
	if err != nil {
		return CloudStorageOssConfig{}, err
	}
	coreconfig.Config.Core.File.Oss = cfg
	return cfg, nil
}

// SettingExtOption describes one option of a select setting.
type SettingExtOption struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

// SettingExtField is both the database initialization format and the data
// returned to the frontend. Value is stored in configJson and is omitted from
// the frontend schema because the module Config map contains current values.
type SettingExtField struct {
	Key         string             `json:"key"`
	Label       string             `json:"label"`
	Type        string             `json:"type"`
	Value       interface{}        `json:"value,omitempty"`
	Default     interface{}        `json:"default,omitempty"`
	Order       int                `json:"order,omitempty"`
	Required    bool               `json:"required,omitempty"`
	Placeholder string             `json:"placeholder,omitempty"`
	Description string             `json:"description,omitempty"`
	Options     []SettingExtOption `json:"options,omitempty"`
}

// settingExtConfig is stored in base_sys_setting_ext.configJson. The plugin's
// initjson file inserts this complete definition into the database; runtime
// code does not read the plugin resource file.
type settingExtConfig struct {
	Version  int               `json:"version"`
	Settings []SettingExtField `json:"settings"`
}

type SettingExtModule struct {
	Id         string                 `json:"id"`
	Module     string                 `json:"module"`
	ModuleName string                 `json:"moduleName"`
	Version    int                    `json:"version"`
	Settings   []SettingExtField      `json:"settings"`
	Config     map[string]interface{} `json:"config"`
}

type settingRow struct {
	ID         string
	ModuleName string
	ConfigJSON string
}

// InstalledModules returns only installed addons with a non-empty setting
// definition already stored in base_sys_setting_ext.
func InstalledModules(ctx context.Context) ([]*SettingExtModule, error) {
	if err := syncAddonMenuMetadata(ctx); err != nil {
		return nil, err
	}
	addons, err := dao.BaseSysAddons.Ctx(ctx).
		Fields("base_sys_addons.id,base_sys_addons.name,base_sys_addons.addonsName,base_sys_addons.orderNum,base_sys_menu.name AS menuName").
		LeftJoin("base_sys_menu", "base_sys_menu.id=base_sys_addons.menuId").
		Where("base_sys_menu.isInstall", 1).
		Order("base_sys_addons.orderNum ASC").
		All()
	if err != nil {
		return nil, err
	}

	result := make([]*SettingExtModule, 0, len(addons))
	seenModules := make(map[string]struct{}, len(addons))
	for _, addon := range addons {
		module := addon["addonsName"].String()
		if module == "" {
			continue
		}
		if _, exists := seenModules[module]; exists {
			continue
		}

		row, rowErr := getSettingRow(ctx, module)
		if rowErr != nil {
			return nil, rowErr
		}
		if row.ID == "" {
			// 没有 initjson 初始化记录的插件不显示设置标签。
			continue
		}
		stored, parseErr := decodeSettingConfig(row.ConfigJSON)
		if parseErr != nil {
			g.Log().Warningf(ctx, "解析插件配置失败 module=%s: %v", module, parseErr)
			continue
		}
		if len(stored.Settings) == 0 {
			continue
		}
		seenModules[module] = struct{}{}

		moduleName := row.ModuleName
		if moduleName == "" {
			moduleName = addon["name"].String()
		}
		if moduleName == "" {
			moduleName = addon["menuName"].String()
		}
		config := make(map[string]interface{}, len(stored.Settings))
		settings := make([]SettingExtField, len(stored.Settings))
		for index, field := range stored.Settings {
			config[field.Key] = field.Value
			field.Value = nil
			settings[index] = field
		}
		result = append(result, &SettingExtModule{
			Id:         row.ID,
			Module:     module,
			ModuleName: moduleName,
			Version:    stored.Version,
			Settings:   settings,
			Config:     config,
		})
	}
	return result, nil
}

// syncAddonMenuMetadata repairs data created before base_sys_addons.addonsName
// was introduced and keeps the plugin table and its root menu consistent.
func syncAddonMenuMetadata(ctx context.Context) error {
	addons, err := dao.BaseSysAddons.Ctx(ctx).
		Fields("id,name,menuId,addonsName,status").
		All()
	if err != nil {
		return err
	}

	for _, addon := range addons {
		menuID := addon["menuId"].String()
		if menuID == "" {
			continue
		}
		menu, menuErr := dao.BaseSysMenu.Ctx(ctx).
			Fields("id,name,addonsName").
			Where("id", menuID).
			One()
		if menuErr != nil {
			return menuErr
		}

		addonName := addon["addonsName"].String()
		if addonName == "" {
			addonName = menu["addonsName"].String()
		}
		if addonName == "" {
			addonName = inferAddonName(addon["name"].String(), menu["name"].String())
		}
		if addonName != "" && addon["addonsName"].String() != addonName {
			if _, err = dao.BaseSysAddons.Ctx(ctx).
				Where("id", addon["id"]).
				Data(g.Map{"addonsName": addonName}).
				Update(); err != nil {
				return err
			}
		}

		menuData := g.Map{"isShow": gconv.Int(addon["status"])}
		if addonName != "" && menu["addonsName"].String() != addonName {
			menuData["addonsName"] = addonName
		}
		if _, err = dao.BaseSysMenu.Ctx(ctx).
			Where("id", menuID).
			Data(menuData).
			Update(); err != nil {
			return err
		}
	}
	return nil
}

func inferAddonName(addonName, menuName string) string {
	name := addonName
	if name == "" {
		name = menuName
	}
	switch name {
	case "资源管理":
		return customerProSettingModule
	case "字典管理":
		return "dict"
	case "任务管理":
		return "task"
	case "crm管理", "CRM管理":
		return "crm"
	case "资源上传":
		return "file_upload"
	case "会员管理":
		return "member"
	default:
		return ""
	}
}

func SaveModule(ctx context.Context, module string, config map[string]interface{}) error {
	if module == cloudStorageSettingModule {
		return gerror.New("云存储配置已迁移到 base_sys_setting KV 表")
	}
	installed, err := dao.BaseSysAddons.Ctx(ctx).
		Fields("base_sys_addons.id").
		LeftJoin("base_sys_menu", "base_sys_menu.id=base_sys_addons.menuId").
		Where("base_sys_addons.addonsName", module).
		Where("base_sys_menu.isInstall", 1).
		One()
	if err != nil {
		return err
	}
	if installed.IsEmpty() {
		return gerror.New("插件尚未安装，不能保存配置")
	}

	row, err := getSettingRow(ctx, module)
	if err != nil {
		return err
	}
	if row.ID == "" {
		return gerror.New("插件没有初始化配置参数")
	}
	stored, err := decodeSettingConfig(row.ConfigJSON)
	if err != nil {
		return err
	}
	fields := make(map[string]*SettingExtField, len(stored.Settings))
	for index := range stored.Settings {
		fields[stored.Settings[index].Key] = &stored.Settings[index]
	}
	for key, value := range config {
		field, ok := fields[key]
		if !ok {
			return gerror.Newf("不允许保存未声明的配置参数: %s", key)
		}
		if !validSettingValue(*field, value) {
			return gerror.Newf("配置参数类型不正确: %s", displaySettingName(*field))
		}
		field.Value = value
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	_, err = g.DB().Model(model.TableNameBaseSysSettingExt).Ctx(ctx).
		Where("module", module).
		Data(g.Map{"configJson": string(raw), "status": 1}).
		Update()
	return err
}

// UploadToOssEnabled reads the persisted value. The plugin initjson provides
// the default row; this function does not read manifest/config.yaml.
func UploadToOssEnabled(ctx context.Context) bool {
	row, err := getSettingRow(ctx, customerProSettingModule)
	if err != nil || row.ID == "" {
		return false
	}
	stored, err := decodeSettingConfig(row.ConfigJSON)
	if err != nil {
		return false
	}
	for _, field := range stored.Settings {
		if field.Key == "uploadToOss" {
			return gconv.Bool(field.Value)
		}
	}
	return false
}

func getSettingRow(ctx context.Context, module string) (settingRow, error) {
	row, err := g.DB().Model(model.TableNameBaseSysSettingExt).Ctx(ctx).
		Where("module", module).
		One()
	if err != nil || row.IsEmpty() {
		return settingRow{}, err
	}
	return settingRow{
		ID:         gconv.String(row["id"]),
		ModuleName: gconv.String(row["moduleName"]),
		ConfigJSON: gconv.String(row["configJson"]),
	}, nil
}

func decodeSettingConfig(raw string) (*settingExtConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return &settingExtConfig{}, nil
	}
	var config settingExtConfig
	if err := json.Unmarshal([]byte(raw), &config); err == nil && len(config.Settings) > 0 {
		return &config, nil
	}

	// 兼容旧版本直接保存 {"key": value} 的数据格式。
	var legacy map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil, err
	}
	config.Settings = make([]SettingExtField, 0, len(legacy))
	for key, value := range legacy {
		fieldType := "text"
		switch value.(type) {
		case bool:
			fieldType = "switch"
		case float64:
			fieldType = "number"
		}
		config.Settings = append(config.Settings, SettingExtField{Key: key, Label: key, Type: fieldType, Value: value})
	}
	return &config, nil
}

func validSettingValue(field SettingExtField, value interface{}) bool {
	switch field.Type {
	case "switch":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64, uint, uint32, uint64, json.Number:
			return true
		default:
			return false
		}
	case "select":
		for _, option := range field.Options {
			if fmt.Sprint(option.Value) == fmt.Sprint(value) {
				return true
			}
		}
		return false
	default:
		_, ok := value.(string)
		return ok
	}
}

func displaySettingName(field SettingExtField) string {
	if field.Label != "" {
		return field.Label
	}
	return field.Key
}
