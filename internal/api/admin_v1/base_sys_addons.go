package admin_v1

import "github.com/gogf/gf/v2/frame/g"

// AvailableReq 获取插件资源目录中尚未添加到插件列表的插件。
type AvailableReq struct {
	g.Meta `path:"/available" method:"GET"`
}

// 安装卸载插件
type InstallUpdateStatusReq struct {
	g.Meta `path:"/installUpdateStatus" method:"POST"`
	Id     string `json:"id"`
	Active bool   `json:"active"`
}

// 上下架插件
type LineUpdateStatusReq struct {
	g.Meta `path:"/lineUpdateStatus" method:"POST"`
	Id     string `json:"id"`
	Active bool   `json:"active"`
}

// AddonsBackupReq creates one SQL backup for every selected addon.
type AddonsBackupReq struct {
	g.Meta     `path:"/backup" method:"POST"`
	Ids        []string `json:"ids" v:"required#请选择需要备份的插件"`
	BackupType string   `json:"backupType"`
}

// AddonsRestoreLatestReq restores the newest backup for one addon.
type AddonsRestoreLatestReq struct {
	g.Meta `path:"/restoreLatest" method:"POST"`
	Id     string `json:"id" v:"required#请选择插件"`
}

// AddonsBackupListReq lists backup batches for one addon.
type AddonsBackupListReq struct {
	g.Meta `path:"/backupList" method:"GET"`
	Id     string `json:"id" v:"required#请选择插件"`
}

// AddonsBackupDetailReq lists the table SQL files in one backup batch.
type AddonsBackupDetailReq struct {
	g.Meta     `path:"/backupDetail" method:"GET"`
	Id         string `json:"id" v:"required#请选择插件"`
	BackupName string `json:"backupName" v:"required#请选择备份"`
	FileName   string `json:"fileName"`
}

// AddonsBackupPreviewReq reads a small SQL fragment for fast preview.
type AddonsBackupPreviewReq struct {
	g.Meta     `path:"/backupPreview" method:"GET"`
	Id         string `json:"id" v:"required#请选择插件"`
	BackupName string `json:"backupName" v:"required#请选择备份"`
	FileName   string `json:"fileName" v:"required#请选择备份表"`
	Offset     int64  `json:"offset"`
	Limit      int64  `json:"limit"`
}

// AddonsBackupDownloadReq downloads one backup table SQL file as a stream.
type AddonsBackupDownloadReq struct {
	g.Meta     `path:"/backupDownload" method:"GET"`
	Id         string `json:"id" v:"required#请选择插件"`
	BackupName string `json:"backupName" v:"required#请选择备份"`
	FileName   string `json:"fileName" v:"required#请选择备份表"`
}

// AddonsBackupDeleteReq permanently removes selected local backup batches.
type AddonsBackupDeleteReq struct {
	g.Meta      `path:"/backupDelete" method:"POST"`
	Id          string   `json:"id" v:"required#请选择插件"`
	BackupNames []string `json:"backupNames"`
	FileNames   []string `json:"fileNames"` // 兼容旧版前端请求
}

// AddonsRestoreReq restores selected table SQL files, or all table SQL files when
// FileNames is empty.
type AddonsRestoreReq struct {
	g.Meta     `path:"/restore" method:"POST"`
	Id         string   `json:"id" v:"required#请选择插件"`
	BackupName string   `json:"backupName" v:"required#请选择备份"`
	FileNames  []string `json:"fileNames"`
	FileName   string   `json:"fileName"` // 兼容旧版前端的单表恢复请求
}

// AddonsTaskReq returns asynchronous backup or restore progress.
type AddonsTaskReq struct {
	g.Meta `path:"/task" method:"GET"`
	TaskId string `json:"taskId" v:"required#任务不存在"`
}

// AddonsTaskSseReq subscribes to an addon's backup or restore progress stream.
type AddonsTaskSseReq struct {
	g.Meta `path:"/taskSse" method:"GET"`
	TaskId string `json:"taskId" v:"required#任务不存在"`
}

// AddonsTaskCancelReq requests graceful cancellation of a running backup.
type AddonsTaskCancelReq struct {
	g.Meta `path:"/taskCancel" method:"POST"`
	TaskId string `json:"taskId" v:"required#任务不存在"`
}

// SiteBackupTablesReq lists every physical table in the current database.
type SiteBackupTablesReq struct {
	g.Meta `path:"/siteBackupTables" method:"GET"`
}

// SiteBackupReq creates one SQL file for every selected database table.
type SiteBackupReq struct {
	g.Meta     `path:"/siteBackup" method:"POST"`
	TableNames []string `json:"tableNames" v:"required#请选择数据库表"`
	BackupType string   `json:"backupType"`
}

// SiteBackupListReq lists full-site backup batches.
type SiteBackupListReq struct {
	g.Meta `path:"/siteBackupList" method:"GET"`
}

// SiteBackupDetailReq lists the table SQL files in one full-site batch.
type SiteBackupDetailReq struct {
	g.Meta     `path:"/siteBackupDetail" method:"GET"`
	BackupName string `json:"backupName" v:"required#请选择备份"`
}

// SiteBackupPreviewReq reads a bounded SQL fragment for fast preview.
type SiteBackupPreviewReq struct {
	g.Meta     `path:"/siteBackupPreview" method:"GET"`
	BackupName string `json:"backupName" v:"required#请选择备份"`
	FileName   string `json:"fileName" v:"required#请选择备份表"`
	Offset     int64  `json:"offset"`
	Limit      int64  `json:"limit"`
}

// SiteBackupDownloadReq downloads one full-site table SQL file.
type SiteBackupDownloadReq struct {
	g.Meta     `path:"/siteBackupDownload" method:"GET"`
	BackupName string `json:"backupName" v:"required#请选择备份"`
	FileName   string `json:"fileName" v:"required#请选择备份表"`
}

// SiteBackupDeleteReq permanently removes selected full-site backup batches.
type SiteBackupDeleteReq struct {
	g.Meta      `path:"/siteBackupDelete" method:"POST"`
	BackupNames []string `json:"backupNames"`
}

// SiteRestoreReq restores one or more table SQL files from a full-site batch.
type SiteRestoreReq struct {
	g.Meta     `path:"/siteRestore" method:"POST"`
	BackupName string   `json:"backupName" v:"required#请选择备份"`
	FileNames  []string `json:"fileNames"`
}
