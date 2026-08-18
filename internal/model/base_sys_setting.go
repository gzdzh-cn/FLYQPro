package model

import "github.com/gzdzh-cn/dzhcore"

const TableNameBaseSysSetting = "base_sys_setting"

// BaseSysSetting stores one system setting per row.
type BaseSysSetting struct {
	*dzhcore.Model
	Key   string `gorm:"column:key;type:varchar(100);not null;uniqueIndex:uk_base_sys_setting_key" json:"key"`
	Value string `gorm:"column:value;type:text" json:"value"`
}

func (*BaseSysSetting) TableName() string { return TableNameBaseSysSetting }
func (*BaseSysSetting) GroupName() string { return "default" }

func NewBaseSysSetting() *BaseSysSetting {
	return &BaseSysSetting{Model: dzhcore.NewModel()}
}

func init() { dzhcore.AddModel(&BaseSysSetting{}) }
