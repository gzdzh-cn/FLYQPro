// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	v1 "dzhgo/internal/api/admin_v1"
	baseCommon "dzhgo/internal/common"
	"dzhgo/internal/model"
	"dzhgo/internal/model/entity"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gzdzh-cn/dzhcore"
)

type (
	IBaseOpenService interface {
		// AdminEPS 获取eps
		AdminEPS(ctx g.Ctx) (result *g.Var, err error)
		// AdminEPS 获取eps
		AppEPS(ctx g.Ctx) (result *g.Var, err error)
		// 版本
		Versions(ctx context.Context, req *v1.VersionsReq) (data interface{}, err error)
		// 站点配置
		GetSetting(ctx context.Context, req *v1.GetSettingReq) (data interface{}, err error)
		// 服务器信息
		ServerInfo(ctx context.Context) (data interface{}, err error)
	}
	IBaseSysActionLogService interface {
		// 记录操作日志
		Record(ctx context.Context, userId string, name string, remark string) (data any, err error)
	}
	IBaseSysAddonsService interface {
		// EnsureBuiltinAddons makes builtin plugins installed and published during startup.
		EnsureBuiltinAddons(ctx context.Context) error
		// ValidateInstalledAddons checks installed plugin tables without creating them.
		ValidateInstalledAddons(ctx context.Context) error
		// RepairAddonRegistrationDuplicates hides historical duplicate plugin registrations.
		RepairAddonRegistrationDuplicates(ctx context.Context) error
		// Available 获取尚未添加到插件列表的插件
		Available(ctx context.Context, req *v1.AvailableReq) (data interface{}, err error)
		// 安装卸载插件
		InstallUpdateStatus(ctx context.Context, req *v1.InstallUpdateStatusReq) (data interface{}, err error)
		// 上下架插件
		LineUpdateStatus(ctx context.Context, req *v1.LineUpdateStatusReq) (data interface{}, err error)
		// Backup creates SQL backups for selected addons.
		Backup(ctx context.Context, req *v1.AddonsBackupReq) (data interface{}, err error)
		// RestoreLatest restores the most recent backup for an addon.
		RestoreLatest(ctx context.Context, req *v1.AddonsRestoreLatestReq) (data interface{}, err error)
		// BackupList lists an addon's backup files.
		BackupList(ctx context.Context, req *v1.AddonsBackupListReq) (data interface{}, err error)
		// BackupDetail lists the table SQL files in a backup batch.
		BackupDetail(ctx context.Context, req *v1.AddonsBackupDetailReq) (data interface{}, err error)
		// BackupPreview reads a bounded SQL fragment for fast preview.
		BackupPreview(ctx context.Context, req *v1.AddonsBackupPreviewReq) (data interface{}, err error)
		// BackupDownload resolves one backup table SQL file for streaming download.
		BackupDownload(ctx context.Context, req *v1.AddonsBackupDownloadReq) (path string, name string, err error)
		// DeleteBackups permanently removes selected local backup batches.
		DeleteBackups(ctx context.Context, req *v1.AddonsBackupDeleteReq) (data interface{}, err error)
		// Restore restores a selected backup file.
		Restore(ctx context.Context, req *v1.AddonsRestoreReq) (data interface{}, err error)
		// Task returns backup or restore progress.
		Task(ctx context.Context, req *v1.AddonsTaskReq) (data interface{}, err error)
		// CancelTask gracefully stops a running backup task.
		CancelTask(ctx context.Context, req *v1.AddonsTaskCancelReq) (data interface{}, err error)
		// SiteBackupTables lists every physical table in the current database.
		SiteBackupTables(ctx context.Context, req *v1.SiteBackupTablesReq) (data interface{}, err error)
		// SiteBackup creates a full-site backup for selected tables.
		SiteBackup(ctx context.Context, req *v1.SiteBackupReq) (data interface{}, err error)
		// SiteBackupList lists full-site backup batches.
		SiteBackupList(ctx context.Context, req *v1.SiteBackupListReq) (data interface{}, err error)
		// SiteBackupDetail lists table SQL files in a full-site backup batch.
		SiteBackupDetail(ctx context.Context, req *v1.SiteBackupDetailReq) (data interface{}, err error)
		// SiteBackupPreview reads a bounded SQL fragment.
		SiteBackupPreview(ctx context.Context, req *v1.SiteBackupPreviewReq) (data interface{}, err error)
		// SiteBackupDownload resolves one full-site table SQL file.
		SiteBackupDownload(ctx context.Context, req *v1.SiteBackupDownloadReq) (path string, name string, err error)
		// SiteBackupDelete deletes selected full-site backup batches.
		SiteBackupDelete(ctx context.Context, req *v1.SiteBackupDeleteReq) (data interface{}, err error)
		// SiteRestore restores selected table SQL files from a full-site backup.
		SiteRestore(ctx context.Context, req *v1.SiteRestoreReq) (data interface{}, err error)
		// EnsureSiteBackupMenu makes the full-site backup menu available on existing installations.
		EnsureSiteBackupMenu(ctx context.Context) error
	}
	IBaseSysAddonsTypesService interface {
		Show(ctx context.Context) (data interface{}, err error)
	}
	IBaseSysAnnouncementService interface {
		Test(ctx context.Context) (err error)
		// ServiceList 重写父方法，返回公告列表 + 未读数量
		ServiceList(ctx context.Context, req *dzhcore.ListReq) (data any, err error)
		// MarkRead 标记公告为已读
		MarkRead(ctx context.Context, announcementId string) (err error)
		// ModifyAfter 监听Update操作，当前端传read=1时标记已读
		ModifyAfter(ctx context.Context, method string, param g.MapStrAny) (err error)
		// MarkAllRead 标记所有公告为已读
		MarkAllRead(ctx context.Context) (err error)
	}
	IBaseSysConfService interface {
		// UpdateValue 更新配置值
		UpdateValue(cKey string, cValue string) error
		// GetValue 获取配置值
		GetValue(cKey string) string
	}
	IBaseSysDepartmentService interface {
		// GetByRoleIds 获取部门
		GetByRoleIds(roleIds []string) (res []uint)
		// Order 排序部门
		Order(ctx g.Ctx) (err error)
	}
	IBaseSysFeedbackService interface {
		ServiceAdd(ctx context.Context, req *dzhcore.AddReq) (data any, err error)
		// ServiceInfo 获取反馈信息
		ServiceInfo(ctx context.Context, req *dzhcore.InfoReq) (data any, err error)
	}
	IBaseSysLogService interface {
		// Record 记录日志
		Record(ctx g.Ctx)
		// Clear 清除日志
		Clear(isAll bool) (err error)
	}
	IBaseSysLoginService interface {
		// Login 登录
		Login(ctx context.Context, req *v1.BaseOpenLoginReq) (result *v1.TokenRes, err error)
		// Captcha 图形验证码
		Captcha(req *v1.BaseOpenCaptchaReq) (interface{}, error)
		// Logout 退出登录
		Logout(ctx context.Context) (err error)
		// RefreshToken 刷新token
		RefreshToken(ctx context.Context, token string) (result *v1.TokenRes, err error)
		// 根据用户生成前端需要的Token信息
		GenerateTokenByUser(ctx context.Context, user *model.BaseSysUser) (result *v1.TokenRes, err error)
	}
	IBaseSysMenuService interface {
		// ModifyAfter 修改后
		ModifyAfter(ctx context.Context, method string, param g.MapStrAny) (err error)
		// ServiceAdd 添加
		ServiceAdd(ctx context.Context, req *dzhcore.AddReq) (data interface{}, err error)
		// GetPerms 获取菜单的权限
		GetPerms(roleIds []string) []string
		// GetMenus 获取菜单
		GetMenus(roleIds []string) (result gdb.Result)
	}
	IBaseSysNoticeService interface {
		StartQueue()
		ServiceInfo(ctx context.Context, req *dzhcore.InfoReq) (data any, err error)
		// 更新阅读状态
		ServiceUpdate(ctx context.Context, req *dzhcore.UpdateReq) (data any, err error)
		// 删除用户消息
		ServiceDelete(ctx context.Context, req *dzhcore.DeleteReq) (data any, err error)
		// 一键已阅
		ServiceReadAll(ctx context.Context) (data any, err error)
		// NoticeAdd 给指定用户推送消息（保持接口兼容性）
		NoticeAdd(ctx context.Context, notice *entity.BaseSysNotice, userIdSlice *[]string) (data any, err error)
		// NoticeAddWithTarget 使用接口多态的消息推送
		NoticeAddWithTarget(ctx context.Context, notice *entity.BaseSysNotice, target baseCommon.NoticeTarget) (data any, err error)
		// NoticeAddToAllUsers 添加通知并推送给全部用户
		NoticeAddToAllUsers(ctx context.Context, notice *entity.BaseSysNotice) (data any, err error)
		// 消息通知处理（保持接口兼容性）
		NoticeDo(ctx context.Context, notice *entity.BaseSysNotice, userIdSlice *[]string) (data any, err error)
		// NoticeDoWithTarget 使用接口多态的消息处理
		NoticeDoWithTarget(ctx context.Context, notice *entity.BaseSysNotice, target baseCommon.NoticeTarget) (data any, err error)
		// 推送队列到 Redis
		NoticePushQueue(ctx context.Context, noticeId string, userId *string) (data any, err error)
		// 队列处理,把队列的数据插入到数据库
		NoticeQueueDo(ctx context.Context, noticeId string, userId *string) (data any, err error)
		// 启动 Redis 队列消费者
		StartRedisQueueConsumer()
		// 检查队列状态
		CheckQueueStatus(ctx context.Context) (data any, err error)
	}
	IBaseSysParamService interface {
		// HtmlByKey 根据配置参数key获取网页内容(富文本)
		HtmlByKey(key string) string
		// ModifyAfter 修改后
		ModifyAfter(ctx context.Context, method string, param g.MapStrAny) (err error)
		// DataByKey 根据配置参数key获取数据
		DataByKey(ctx context.Context, key string) (data string, err error)
	}
	IBaseSysPermsService interface {
		// permmenu 方法
		Permmenu(ctx context.Context, roleIds []string) (res interface{})
		// RefreshPerms refreshPerms(userId)
		RefreshPerms(ctx context.Context, userId string) (err error)
	}
	IBaseSysQuickMenuService interface {
		// ServiceList 重写父结构体的ServiceList方法，返回当前用户的快捷菜单
		ServiceList(ctx context.Context, req *dzhcore.ListReq) (data any, err error)
		// QuickMenuList 获取用户有权限的菜单列表（用于前端选择添加快捷菜单）
		QuickMenuList(ctx context.Context, roleIds []string) (data interface{}, err error)
		Test(ctx context.Context) (err error)
	}
	IBaseSysRoleService interface {
		// ModifyAfter modify after
		ModifyAfter(ctx context.Context, method string, param g.MapStrAny) (err error)
		// GetByUser get array  roleId by userId
		GetByUser(userId string) []string
		// ServiceInfo 方法重构
		ServiceInfo(ctx context.Context, req *dzhcore.InfoReq) (data interface{}, err error)
	}
	IBaseSysUserService interface {
		// Person 方法 返回不带密码的用户信息
		Person(userId string) (res interface{}, err error)
		ModifyBefore(ctx context.Context, method string, param g.MapStrAny) (err error)
		ModifyAfter(ctx context.Context, method string, param g.MapStrAny) (err error)
		// ServiceAdd 方法 添加用户
		ServiceAdd(ctx context.Context, req *dzhcore.AddReq) (data interface{}, err error)
		// ServiceInfo 方法 返回服务信息
		ServiceInfo(ctx g.Ctx, req *dzhcore.InfoReq) (data interface{}, err error)
		// ServiceUpdate 方法 更新用户信息
		ServiceUpdate(ctx context.Context, req *dzhcore.UpdateReq) (data interface{}, err error)
		// 删除用户缓存
		DeleteCache(ctx context.Context, userId string) (err error)
		// Move 移动用户部门
		Move(ctx g.Ctx) (err error)
		// Count 获取用户总数
		Count(ctx context.Context) (count int, err error)
		// OnlineCount 获取在线用户数
		OnlineCount(ctx context.Context) (count int, err error)
	}
)

var (
	localBaseOpenService            IBaseOpenService
	localBaseSysActionLogService    IBaseSysActionLogService
	localBaseSysAddonsService       IBaseSysAddonsService
	localBaseSysAddonsTypesService  IBaseSysAddonsTypesService
	localBaseSysAnnouncementService IBaseSysAnnouncementService
	localBaseSysConfService         IBaseSysConfService
	localBaseSysDepartmentService   IBaseSysDepartmentService
	localBaseSysFeedbackService     IBaseSysFeedbackService
	localBaseSysLogService          IBaseSysLogService
	localBaseSysLoginService        IBaseSysLoginService
	localBaseSysMenuService         IBaseSysMenuService
	localBaseSysNoticeService       IBaseSysNoticeService
	localBaseSysParamService        IBaseSysParamService
	localBaseSysPermsService        IBaseSysPermsService
	localBaseSysQuickMenuService    IBaseSysQuickMenuService
	localBaseSysRoleService         IBaseSysRoleService
	localBaseSysUserService         IBaseSysUserService
)

func BaseOpenService() IBaseOpenService {
	if localBaseOpenService == nil {
		panic("implement not found for interface IBaseOpenService, forgot register?")
	}
	return localBaseOpenService
}

func RegisterBaseOpenService(i IBaseOpenService) {
	localBaseOpenService = i
}

func BaseSysActionLogService() IBaseSysActionLogService {
	if localBaseSysActionLogService == nil {
		panic("implement not found for interface IBaseSysActionLogService, forgot register?")
	}
	return localBaseSysActionLogService
}

func RegisterBaseSysActionLogService(i IBaseSysActionLogService) {
	localBaseSysActionLogService = i
}

func BaseSysAddonsService() IBaseSysAddonsService {
	if localBaseSysAddonsService == nil {
		panic("implement not found for interface IBaseSysAddonsService, forgot register?")
	}
	return localBaseSysAddonsService
}

func RegisterBaseSysAddonsService(i IBaseSysAddonsService) {
	localBaseSysAddonsService = i
}

func BaseSysAddonsTypesService() IBaseSysAddonsTypesService {
	if localBaseSysAddonsTypesService == nil {
		panic("implement not found for interface IBaseSysAddonsTypesService, forgot register?")
	}
	return localBaseSysAddonsTypesService
}

func RegisterBaseSysAddonsTypesService(i IBaseSysAddonsTypesService) {
	localBaseSysAddonsTypesService = i
}

func BaseSysAnnouncementService() IBaseSysAnnouncementService {
	if localBaseSysAnnouncementService == nil {
		panic("implement not found for interface IBaseSysAnnouncementService, forgot register?")
	}
	return localBaseSysAnnouncementService
}

func RegisterBaseSysAnnouncementService(i IBaseSysAnnouncementService) {
	localBaseSysAnnouncementService = i
}

func BaseSysConfService() IBaseSysConfService {
	if localBaseSysConfService == nil {
		panic("implement not found for interface IBaseSysConfService, forgot register?")
	}
	return localBaseSysConfService
}

func RegisterBaseSysConfService(i IBaseSysConfService) {
	localBaseSysConfService = i
}

func BaseSysDepartmentService() IBaseSysDepartmentService {
	if localBaseSysDepartmentService == nil {
		panic("implement not found for interface IBaseSysDepartmentService, forgot register?")
	}
	return localBaseSysDepartmentService
}

func RegisterBaseSysDepartmentService(i IBaseSysDepartmentService) {
	localBaseSysDepartmentService = i
}

func BaseSysFeedbackService() IBaseSysFeedbackService {
	if localBaseSysFeedbackService == nil {
		panic("implement not found for interface IBaseSysFeedbackService, forgot register?")
	}
	return localBaseSysFeedbackService
}

func RegisterBaseSysFeedbackService(i IBaseSysFeedbackService) {
	localBaseSysFeedbackService = i
}

func BaseSysLogService() IBaseSysLogService {
	if localBaseSysLogService == nil {
		panic("implement not found for interface IBaseSysLogService, forgot register?")
	}
	return localBaseSysLogService
}

func RegisterBaseSysLogService(i IBaseSysLogService) {
	localBaseSysLogService = i
}

func BaseSysLoginService() IBaseSysLoginService {
	if localBaseSysLoginService == nil {
		panic("implement not found for interface IBaseSysLoginService, forgot register?")
	}
	return localBaseSysLoginService
}

func RegisterBaseSysLoginService(i IBaseSysLoginService) {
	localBaseSysLoginService = i
}

func BaseSysMenuService() IBaseSysMenuService {
	if localBaseSysMenuService == nil {
		panic("implement not found for interface IBaseSysMenuService, forgot register?")
	}
	return localBaseSysMenuService
}

func RegisterBaseSysMenuService(i IBaseSysMenuService) {
	localBaseSysMenuService = i
}

func BaseSysNoticeService() IBaseSysNoticeService {
	if localBaseSysNoticeService == nil {
		panic("implement not found for interface IBaseSysNoticeService, forgot register?")
	}
	return localBaseSysNoticeService
}

func RegisterBaseSysNoticeService(i IBaseSysNoticeService) {
	localBaseSysNoticeService = i
}

func BaseSysParamService() IBaseSysParamService {
	if localBaseSysParamService == nil {
		panic("implement not found for interface IBaseSysParamService, forgot register?")
	}
	return localBaseSysParamService
}

func RegisterBaseSysParamService(i IBaseSysParamService) {
	localBaseSysParamService = i
}

func BaseSysPermsService() IBaseSysPermsService {
	if localBaseSysPermsService == nil {
		panic("implement not found for interface IBaseSysPermsService, forgot register?")
	}
	return localBaseSysPermsService
}

func RegisterBaseSysPermsService(i IBaseSysPermsService) {
	localBaseSysPermsService = i
}

func BaseSysQuickMenuService() IBaseSysQuickMenuService {
	if localBaseSysQuickMenuService == nil {
		panic("implement not found for interface IBaseSysQuickMenuService, forgot register?")
	}
	return localBaseSysQuickMenuService
}

func RegisterBaseSysQuickMenuService(i IBaseSysQuickMenuService) {
	localBaseSysQuickMenuService = i
}

func BaseSysRoleService() IBaseSysRoleService {
	if localBaseSysRoleService == nil {
		panic("implement not found for interface IBaseSysRoleService, forgot register?")
	}
	return localBaseSysRoleService
}

func RegisterBaseSysRoleService(i IBaseSysRoleService) {
	localBaseSysRoleService = i
}

func BaseSysUserService() IBaseSysUserService {
	if localBaseSysUserService == nil {
		panic("implement not found for interface IBaseSysUserService, forgot register?")
	}
	return localBaseSysUserService
}

func RegisterBaseSysUserService(i IBaseSysUserService) {
	localBaseSysUserService = i
}
