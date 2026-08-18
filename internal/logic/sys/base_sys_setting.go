package sys

import (
	"context"
	"strings"

	"dzhgo/internal/api/admin_v1"
	"dzhgo/internal/dao"
	"dzhgo/internal/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gzdzh-cn/dzhcore"
)

type BaseSysSettingKV struct {
	Id    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func ListBaseSysSettingKV(ctx context.Context) ([]*BaseSysSettingKV, error) {
	rows, err := dao.BaseSysSetting.Ctx(ctx).
		Fields("id,`key`,`value`").
		Where("deleted_at IS NULL").
		OrderAsc("`key`").
		All()
	if err != nil {
		return nil, err
	}
	items := make([]*BaseSysSettingKV, 0, len(rows))
	for _, row := range rows {
		items = append(items, &BaseSysSettingKV{
			Id:    row["id"].String(),
			Key:   row["key"].String(),
			Value: row["value"].String(),
		})
	}
	return items, nil
}

func GetBaseSysSettingValue(ctx context.Context, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", gerror.New("配置键不能为空")
	}
	row, err := dao.BaseSysSetting.Ctx(ctx).
		Fields("`value`").
		Where("`key` = ?", key).
		Where("deleted_at IS NULL").
		One()
	if err != nil {
		return "", err
	}
	if row.IsEmpty() {
		return "", nil
	}
	return row["value"].String(), nil
}

func SaveBaseSysSettingKV(ctx context.Context, items []admin_v1.BaseSysSettingKVItem) error {
	if len(items) == 0 {
		return gerror.New("配置列表不能为空")
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return gerror.New("配置键不能为空")
		}
		if len(key) > 100 {
			return gerror.Newf("配置键长度不能超过100: %s", key)
		}
		if _, exists := seen[key]; exists {
			return gerror.Newf("配置键重复: %s", key)
		}
		seen[key] = struct{}{}
	}

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now := gtime.Now()
		for _, item := range items {
			key := strings.TrimSpace(item.Key)
			setting, err := tx.Model(model.TableNameBaseSysSetting).
				Fields("id").
				Where("`key` = ?", key).
				One()
			if err != nil {
				return err
			}
			if !setting.IsEmpty() {
				_, err = tx.Model(model.TableNameBaseSysSetting).
					Where("id = ?", setting["id"]).
					Data(g.Map{"value": item.Value, "updateTime": now, "deleted_at": nil}).
					Update()
				if err != nil {
					return err
				}
				continue
			}
			_, err = tx.Model(model.TableNameBaseSysSetting).Data(g.Map{
				"id":         dzhcore.NodeSnowflake.Generate().String(),
				"createTime": now,
				"updateTime": now,
				"key":        key,
				"value":      item.Value,
			}).Insert()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func SettingInt(value string, defaultValue int) int {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return gconv.Int(value)
}
