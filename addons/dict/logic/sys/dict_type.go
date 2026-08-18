package sys

import (
	"context"

	"dzhgo/addons/dict/dao"
	"dzhgo/addons/dict/model"
	"dzhgo/addons/dict/model/entity"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gzdzh-cn/dzhcore"
)

func init() {
	// DictType 不需要 service 注册，controller 直接使用 NewsDictTypeService() 返回的实例
}

type sDictTypeService struct {
	*dzhcore.Service
}

func NewsDictTypeService() *sDictTypeService {
	return &sDictTypeService{
		Service: &dzhcore.Service{
			Dao:   &dao.AddonsDictType,
			Model: model.NewDictType(),
			ListQueryOp: &dzhcore.QueryOp{
				KeyWordField: []string{"name"},
			},
		},
	}
}

// 受保护的字典类型 key，禁止删除和修改名称
var protectedTypeKeys = map[string]string{
	"cluesLevel":   "线索等级",
	"education":    "学员阶段",
	"sourceFrom":   "学员来源",
	"householdType": "户口性质",
}

// ModifyBefore 增删改前的钩子：保护核心字典类型
func (s *sDictTypeService) ModifyBefore(ctx context.Context, method string, param g.MapStrAny) (err error) {
	switch method {
	case "Update":
		// 更新时，检查是否为受保护类型，禁止修改名称和 key
		id := gconv.String(param["id"])
		if id == "" {
			return nil
		}
		var dictType *entity.AddonsDictType
		err = dao.AddonsDictType.Ctx(ctx).Where("id", id).Scan(&dictType)
		if err != nil || dictType == nil {
			return nil
		}
		if label, ok := protectedTypeKeys[dictType.Key]; ok {
			newName := gconv.String(param["name"])
			newKey := gconv.String(param["key"])
			if newName != "" && newName != dictType.Name {
				return gerror.Newf("「%s」为系统内置类型，禁止修改名称", label)
			}
			if newKey != "" && newKey != dictType.Key {
				return gerror.Newf("「%s」为系统内置类型，禁止修改Key", label)
			}
		}

	case "Delete":
		// 删除时，检查是否为受保护类型
		ids, ok := param["ids"]
		if !ok {
			return nil
		}
		idSlice := gconv.SliceAny(ids)
		for _, id := range idSlice {
			var dictType *entity.AddonsDictType
			err = dao.AddonsDictType.Ctx(ctx).Where("id", gconv.String(id)).Scan(&dictType)
			if err != nil || dictType == nil {
				continue
			}
			if label, ok := protectedTypeKeys[dictType.Key]; ok {
				return gerror.Newf("「%s」为系统内置类型，禁止删除", label)
			}
		}
	}
	return nil
}
