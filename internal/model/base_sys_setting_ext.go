package model

import "github.com/gzdzh-cn/dzhcore"

const TableNameBaseSysSettingExt = "base_sys_setting_ext"

// BaseSysSettingExt stores the configuration of one installed module.
// ConfigJson stays extensible so modules can own their fields independently.
type BaseSysSettingExt struct {
	*dzhcore.Model
	Module     string `gorm:"column:module;type:varchar(100);not null;uniqueIndex:uk_setting_ext_module" json:"module"`
	ModuleName string `gorm:"column:moduleName;type:varchar(100);not null" json:"moduleName"`
	ConfigJson string `gorm:"column:configJson;type:text;not null" json:"configJson"`
	Status     int    `gorm:"column:status;type:int;not null;default:1" json:"status"`
}

func (*BaseSysSettingExt) TableName() string { return TableNameBaseSysSettingExt }
func (*BaseSysSettingExt) GroupName() string { return "default" }

func NewBaseSysSettingExt() *BaseSysSettingExt {
	return &BaseSysSettingExt{Model: dzhcore.NewModel()}
}

func init() { dzhcore.AddModel(&BaseSysSettingExt{}) }
