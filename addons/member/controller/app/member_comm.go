package app

import (
	"context"
	v1 "dzhgo/addons/member/api/app_v1"
	"dzhgo/addons/member/common"
	"dzhgo/addons/member/dao"
	"dzhgo/addons/member/service"

	"github.com/gogf/gf/v2/crypto/gmd5"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gzdzh-cn/dzhcore"
)

type MemberCommController struct {
	*dzhcore.ControllerSimple
}

func init() {
	var memberCommController = &MemberCommController{
		&dzhcore.ControllerSimple{
			Prefix: "/app/member/comm",
		},
	}
	// 注册路由
	dzhcore.AddControllerSimple(memberCommController)
}

// Person 方法 返回不带密码的用户信息
func (c *MemberCommController) Person(ctx context.Context, req *v1.PersonReq) (res *dzhcore.BaseRes, err error) {

	member := common.GetMember(ctx)
	data, err := service.MemberManageService().Person(ctx, member.MemberId)
	if err != nil {
		return
	}
	res = dzhcore.Ok(data)
	return
}

// MemberUpdate 更新个人信息
func (c *MemberCommController) MemberUpdate(ctx context.Context, req *v1.MemberUpdateReq) (res *dzhcore.BaseRes, err error) {
	member := common.GetMember(ctx)
	if member == nil {
		return nil, gerror.New("登录失效")
	}
	uid := member.MemberId

	updateData := g.Map{}
	if req.Name != "" {
		updateData["memberName"] = req.Name
	}
	if req.NickName != "" {
		updateData["nickname"] = req.NickName
	}
	if req.Password != "" {
		md5password, _ := gmd5.Encrypt(req.Password)
		updateData["password"] = md5password
		updateData["passwordV"] = gdb.Raw("passwordV+1")
	}

	if len(updateData) == 0 {
		err = gerror.New("请至少修改一项信息")
		return
	}

	_, err = dao.AddonsMemberManage.Ctx(ctx).Where("id", uid).Data(updateData).Update()
	if err != nil {
		g.Log().Error(ctx, "更新个人信息失败", err)
		err = gerror.New("更新失败")
		return
	}

	res = dzhcore.Ok(g.Map{"success": true})
	return
}
