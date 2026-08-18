package admin

import (
	"context"
	v1 "dzhgo/internal/api/admin_v1"
	logic "dzhgo/internal/logic/sys"
	"dzhgo/internal/service"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gzdzh-cn/dzhcore"
)

type BaseSysAddonsController struct {
	*dzhcore.Controller
}

// Page repairs historical duplicate registrations before querying the plugin
// list. The repair only soft-deletes redundant rows and keeps one visible row.
func (c *BaseSysAddonsController) Page(ctx context.Context, req *dzhcore.PageReq) (res *dzhcore.BaseRes, err error) {
	if err = service.BaseSysAddonsService().RepairAddonRegistrationDuplicates(ctx); err != nil {
		g.Log().Errorf(ctx, "插件登记去重失败: %v", err)
		return nil, err
	}
	return c.Controller.Page(ctx, req)
}

func init() {
	var baseSysAddonsController = &BaseSysAddonsController{
		&dzhcore.Controller{
			Prefix:  "/admin/base/sys/addons",
			Api:     []string{"Add", "Delete", "Update", "Info", "List", "Page"},
			Service: logic.NewsBaseSysAddonsService(),
		},
	}
	// 注册路由
	dzhcore.AddController(baseSysAddonsController)

}

// Available 获取尚未添加到插件列表的插件
func (c *BaseSysAddonsController) Available(ctx context.Context, req *v1.AvailableReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().Available(ctx, req)
	if err != nil {
		return
	}
	res = dzhcore.Ok(data)
	return
}

// 安装卸载插件
func (c *BaseSysAddonsController) InstallUpdateStatus(ctx context.Context, req *v1.InstallUpdateStatusReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().InstallUpdateStatus(ctx, req)
	if err != nil {
		return
	}
	res = dzhcore.Ok(data)
	return
}

// 上下架插件
func (c *BaseSysAddonsController) LineUpdateStatus(ctx context.Context, req *v1.LineUpdateStatusReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().LineUpdateStatus(ctx, req)
	if err != nil {
		return
	}
	res = dzhcore.Ok(data)
	return
}

func (c *BaseSysAddonsController) Backup(ctx context.Context, req *v1.AddonsBackupReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().Backup(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) RestoreLatest(ctx context.Context, req *v1.AddonsRestoreLatestReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().RestoreLatest(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) BackupList(ctx context.Context, req *v1.AddonsBackupListReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().BackupList(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) BackupDetail(ctx context.Context, req *v1.AddonsBackupDetailReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().BackupDetail(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) BackupPreview(ctx context.Context, req *v1.AddonsBackupPreviewReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().BackupPreview(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) BackupDownload(ctx context.Context, req *v1.AddonsBackupDownloadReq) (res *dzhcore.BaseRes, err error) {
	path, name, err := service.BaseSysAddonsService().BackupDownload(ctx, req)
	if err != nil {
		return nil, err
	}
	g.RequestFromCtx(ctx).Response.ServeFileDownload(path, name)
	return nil, nil
}

func (c *BaseSysAddonsController) BackupDelete(ctx context.Context, req *v1.AddonsBackupDeleteReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().DeleteBackups(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) Restore(ctx context.Context, req *v1.AddonsRestoreReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().Restore(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) Task(ctx context.Context, req *v1.AddonsTaskReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().Task(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) TaskCancel(ctx context.Context, req *v1.AddonsTaskCancelReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().CancelTask(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteBackupTables(ctx context.Context, req *v1.SiteBackupTablesReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteBackupTables(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteBackup(ctx context.Context, req *v1.SiteBackupReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteBackup(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteBackupList(ctx context.Context, req *v1.SiteBackupListReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteBackupList(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteBackupDetail(ctx context.Context, req *v1.SiteBackupDetailReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteBackupDetail(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteBackupPreview(ctx context.Context, req *v1.SiteBackupPreviewReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteBackupPreview(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteBackupDownload(ctx context.Context, req *v1.SiteBackupDownloadReq) (res *dzhcore.BaseRes, err error) {
	path, name, err := service.BaseSysAddonsService().SiteBackupDownload(ctx, req)
	if err != nil {
		return nil, err
	}
	g.RequestFromCtx(ctx).Response.ServeFileDownload(path, name)
	return nil, nil
}

func (c *BaseSysAddonsController) SiteBackupDelete(ctx context.Context, req *v1.SiteBackupDeleteReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteBackupDelete(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

func (c *BaseSysAddonsController) SiteRestore(ctx context.Context, req *v1.SiteRestoreReq) (res *dzhcore.BaseRes, err error) {
	data, err := service.BaseSysAddonsService().SiteRestore(ctx, req)
	if err != nil {
		return
	}
	return dzhcore.Ok(data), nil
}

// TaskSse streams backup and restore progress. It writes SSE directly so the
// normal JSON response wrapper does not buffer the progress events.
func (c *BaseSysAddonsController) TaskSse(ctx context.Context, req *v1.AddonsTaskSseReq) (res *dzhcore.BaseRes, err error) {
	initial, updates, unsubscribe, err := logic.SubscribeAddonTask(req.TaskId)
	if err != nil {
		return nil, err
	}
	defer unsubscribe()

	r := g.RequestFromCtx(ctx)
	rw := r.Response.RawWriter()
	flusher, ok := rw.(http.Flusher)
	if !ok {
		r.Response.WriteStatusExit(http.StatusInternalServerError)
		return
	}
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("X-Accel-Buffering", "no")

	writeEvent := func(data interface{}) error {
		payload, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			return marshalErr
		}
		_, writeErr := fmt.Fprintf(rw, "event: progress\ndata: %s\n\n", payload)
		if writeErr == nil {
			flusher.Flush()
		}
		return writeErr
	}
	if err = writeEvent(initial); err != nil {
		return nil, nil
	}
	if isFinishedAddonTask(initial) {
		return nil, nil
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case update, ok := <-updates:
			if !ok {
				return nil, nil
			}
			if err = writeEvent(update); err != nil {
				return nil, nil
			}
			if isFinishedAddonTask(update) {
				return nil, nil
			}
		case <-heartbeat.C:
			if _, err = fmt.Fprint(rw, ": heartbeat\n\n"); err != nil {
				return nil, nil
			}
			flusher.Flush()
		}
	}
}

func isFinishedAddonTask(task map[string]interface{}) bool {
	status, _ := task["status"].(string)
	return status == "completed" || status == "failed" || status == "cancelled"
}
