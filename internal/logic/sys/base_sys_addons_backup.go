package sys

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	v1 "dzhgo/internal/api/admin_v1"
	"dzhgo/internal/dao"
	"dzhgo/internal/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gres"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gzdzh-cn/dzhcore"
)

const (
	addonBackupDirectory                  = "addons"
	addonBackupRootDirectory              = "public"
	addonInsertBatchRows                  = 500
	addonInsertBatchBytes                 = 1024 * 1024
	addonRestoreTransactionStatementCount = 20
	addonBackupPreviewDefaultLimit        = int64(128 * 1024)
	addonBackupPreviewMaxLimit            = int64(512 * 1024)
	addonBackupTypeData                   = "data"
	addonBackupTypeStructure              = "structure"
	addonBackupMetaFile                   = "backup_meta.json"
	addonRestoreReaderBufferSize          = 256 * 1024
	addonRestoreMaxStatementBytes         = 64 * 1024 * 1024
	// Restoring one statement at a time is intentionally sequential, but the
	// transaction and progress intervals must be bounded so a large backup does
	// not spend most of its CPU on commits and SSE serialization.
	addonRestoreProgressInterval      = 500 * time.Millisecond
	addonRestoreProgressStatementStep = 500
	addonRestoreYieldInterval         = 50 * time.Millisecond
)

type addonBackupMeta struct {
	BackupType string `json:"backupType"`
}

var (
	addonTasks     sync.Map // map[string]*addonTask
	addonTaskLocks sync.Map // map[string]*addonTaskLock
	// Restores are deliberately serialized across plugins and full-site tasks.
	// Otherwise two large SQL imports can saturate the container even though
	// each individual task is sequential.
	addonRestoreSemaphore = make(chan struct{}, 1)
	addonTableRegexp      = regexp.MustCompile(`(?i)\b(?:CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?|DROP\s+TABLE(?:\s+IF\s+EXISTS)?|INSERT\s+INTO|DELETE\s+FROM)\s+` + "`?" + `([a-zA-Z0-9_]+)` + "`?")
)

type addonTask struct {
	mu          sync.RWMutex
	ID          string  `json:"taskId"`
	Operation   string  `json:"operation"`
	Scope       string  `json:"scope,omitempty"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"`
	Message     string  `json:"message"`
	Error       string  `json:"error,omitempty"`
	Errors      []g.Map `json:"errors,omitempty"`
	UpdatedAt   string  `json:"updatedAt"`
	cancel      context.CancelFunc
	subscribers map[chan g.Map]struct{}
}

// addonTaskLock serializes work on one plugin. A channel is used instead of a
// sync.Mutex so a task waiting for another backup/restore can still be stopped.
type addonTaskLock struct {
	semaphore chan struct{}
}

type addonTarget struct {
	MenuID string
	Name   string
}

// addonBackupTable describes a physical table belonging to a plugin backup.
// Shared tables are data-only: their schema is never dropped or recreated.
type addonBackupTable struct {
	Name          string
	Shared        bool
	OwnershipKey  string
	OwnershipName string
	InitIDs       []string
}

func newAddonTask(operation string) *addonTask {
	task := &addonTask{
		ID:          dzhcore.NodeSnowflake.Generate().String(),
		Operation:   operation,
		Status:      "pending",
		Progress:    0,
		Message:     "任务已创建",
		UpdatedAt:   time.Now().Format(time.DateTime),
		subscribers: make(map[chan g.Map]struct{}),
		Errors:      make([]g.Map, 0),
	}
	addonTasks.Store(task.ID, task)
	return task
}

func (t *addonTask) update(status string, progress int, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == "cancelling" && status == "running" {
		return
	}
	t.Status = status
	t.Progress = progress
	t.Message = message
	t.UpdatedAt = time.Now().Format(time.DateTime)
	t.publishLocked()
}

func (t *addonTask) setCancel(cancel context.CancelFunc) {
	t.mu.Lock()
	t.cancel = cancel
	cancelling := t.Status == "cancelling"
	t.mu.Unlock()
	if cancelling {
		cancel()
	}
}

func (t *addonTask) setScope(scope string) {
	t.mu.Lock()
	t.Scope = scope
	t.mu.Unlock()
}

func (t *addonTask) progressValue() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Progress
}

func (t *addonTask) completionMessage() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.Errors) > 0 {
		return fmt.Sprintf("恢复完成，但有 %d 张表失败", len(t.Errors))
	}
	return "恢复成功"
}

func (t *addonTask) requestCancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == "completed" || t.Status == "failed" || t.Status == "cancelled" {
		return gerror.New("任务已结束")
	}
	if t.Status == "cancelling" {
		return nil
	}
	t.Status = "cancelling"
	t.Message = "正在停止" + addonTaskOperationName(t.Operation) + "任务…"
	t.UpdatedAt = time.Now().Format(time.DateTime)
	t.publishLocked()
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

func acquireAddonTaskLock(ctx context.Context, addonName string) (release func(), err error) {
	value, _ := addonTaskLocks.LoadOrStore(addonName, &addonTaskLock{
		semaphore: make(chan struct{}, 1),
	})
	lock := value.(*addonTaskLock)
	select {
	case lock.semaphore <- struct{}{}:
		return func() { <-lock.semaphore }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func acquireAddonRestoreSlot(ctx context.Context) (release func(), err error) {
	select {
	case addonRestoreSemaphore <- struct{}{}:
		return func() { <-addonRestoreSemaphore }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func addonTaskOperationName(operation string) string {
	if operation == "restore" {
		return "恢复"
	}
	return "备份"
}

func (t *addonTask) fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = "failed"
	t.Error = err.Error()
	t.Message = "任务失败"
	t.UpdatedAt = time.Now().Format(time.DateTime)
	t.publishLocked()
}

func (t *addonTask) recordError(addonName, tableName, fileName string, err error) {
	if err == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	item := g.Map{
		"addonName": addonName,
		"tableName": tableName,
		"fileName":  fileName,
		"error":     err.Error(),
		"time":      time.Now().Format(time.DateTime),
	}
	t.Errors = append(t.Errors, item)
	t.Error = fmt.Sprintf("有 %d 张表恢复失败", len(t.Errors))
	t.UpdatedAt = time.Now().Format(time.DateTime)
	t.publishLocked()
}

func (t *addonTask) data() g.Map {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.dataLocked()
}

func (t *addonTask) dataLocked() g.Map {
	return g.Map{
		"taskId":    t.ID,
		"operation": t.Operation,
		"scope":     t.Scope,
		"status":    t.Status,
		"progress":  t.Progress,
		"message":   t.Message,
		"error":     t.Error,
		"errors":    t.Errors,
		"updatedAt": t.UpdatedAt,
	}
}

func (t *addonTask) publishLocked() {
	data := t.dataLocked()
	for subscriber := range t.subscribers {
		select {
		case subscriber <- data:
		default:
			// Keep only the newest state when progress updates arrive faster than
			// the SSE client can consume them. This also guarantees that a terminal
			// completed/failed state replaces a stale 100% running state.
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- data:
			default:
			}
		}
	}
}

// SubscribeAddonTask returns the latest task state followed by state-change
// notifications. The caller must invoke unsubscribe when its SSE connection closes.
func SubscribeAddonTask(taskID string) (initial g.Map, updates <-chan g.Map, unsubscribe func(), err error) {
	value, ok := addonTasks.Load(taskID)
	if !ok {
		err = gerror.New("任务不存在或已过期")
		return
	}
	task := value.(*addonTask)
	channel := make(chan g.Map, 1)
	task.mu.Lock()
	initial = task.dataLocked()
	task.subscribers[channel] = struct{}{}
	task.mu.Unlock()

	updates = channel
	unsubscribe = func() {
		task.mu.Lock()
		if _, exists := task.subscribers[channel]; exists {
			delete(task.subscribers, channel)
			close(channel)
		}
		task.mu.Unlock()
	}
	return
}

func (s *sBaseSysAddonsService) Backup(ctx context.Context, req *v1.AddonsBackupReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, req.Ids)
	if err != nil {
		return nil, err
	}
	if err = ensureAddonTargetsInstalled(ctx, targets); err != nil {
		return nil, err
	}
	backupType := normalizeAddonBackupType(req.BackupType)
	task := newAddonTask("backup")
	go runAddonTask(task, targets, func(taskCtx context.Context, target addonTarget, progress func(int, string)) error {
		return backupAddon(taskCtx, target, backupType, progress)
	})
	return g.Map{"taskId": task.ID}, nil
}

func (s *sBaseSysAddonsService) RestoreLatest(ctx context.Context, req *v1.AddonsRestoreLatestReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return nil, err
	}
	if err = ensureAddonTargetsInstalled(ctx, targets); err != nil {
		return nil, err
	}
	files, err := listAddonBackups(targets[0].Name)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, gerror.New("该插件暂无可恢复的备份")
	}
	task := newAddonTask("restore")
	go runAddonTask(task, targets, func(taskCtx context.Context, target addonTarget, progress func(int, string)) error {
		return restoreAddon(taskCtx, target, gconv.String(files[0]["backupName"]), nil, progress, func(tableName, fileName string, restoreErr error) {
			task.recordError(target.Name, tableName, fileName, restoreErr)
		})
	})
	return g.Map{"taskId": task.ID}, nil
}

func (s *sBaseSysAddonsService) BackupList(ctx context.Context, req *v1.AddonsBackupListReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return nil, err
	}
	return listAddonBackups(targets[0].Name)
}

func (s *sBaseSysAddonsService) BackupDetail(ctx context.Context, req *v1.AddonsBackupDetailReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return nil, err
	}
	return addonBackupDetail(targets[0].Name, req.BackupName, req.FileName)
}

func (s *sBaseSysAddonsService) BackupPreview(ctx context.Context, req *v1.AddonsBackupPreviewReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return nil, err
	}
	return addonBackupPreview(targets[0].Name, req.BackupName, req.FileName, req.Offset, req.Limit)
}

func (s *sBaseSysAddonsService) BackupDownload(ctx context.Context, req *v1.AddonsBackupDownloadReq) (path string, name string, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return "", "", err
	}
	file, err := findAddonBackupFile(targets[0].Name, req.BackupName, req.FileName)
	if err != nil {
		return "", "", err
	}
	return file.Path, file.FileName, nil
}

func (s *sBaseSysAddonsService) DeleteBackups(ctx context.Context, req *v1.AddonsBackupDeleteReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return nil, err
	}
	addonName := targets[0].Name
	backupNames := append([]string{}, req.BackupNames...)
	if len(backupNames) == 0 {
		// The old client sent fileNames. Treat those values as batch names so
		// existing installations can still remove legacy single-file backups.
		backupNames = append(backupNames, req.FileNames...)
	}
	seen := make(map[string]struct{}, len(req.FileNames))
	cleanNames := make([]string, 0, len(backupNames))
	for _, backupName := range backupNames {
		if !isSafeBackupName(addonName, backupName) && !isSafeBackupFileName(addonName, backupName) {
			return nil, gerror.New("备份批次无效")
		}
		if _, exists := seen[backupName]; exists {
			continue
		}
		seen[backupName] = struct{}{}
		cleanNames = append(cleanNames, backupName)
	}
	if len(cleanNames) == 0 {
		return nil, gerror.New("请选择需要删除的备份批次")
	}

	release, err := acquireAddonTaskLock(ctx, addonName)
	if err != nil {
		return nil, err
	}
	defer release()
	for _, backupName := range cleanNames {
		if _, statErr := os.Stat(addonBackupPath(addonName, backupName)); statErr != nil {
			if os.IsNotExist(statErr) {
				return nil, gerror.Newf("备份批次不存在：%s", backupName)
			}
			return nil, statErr
		}
	}
	for _, backupName := range cleanNames {
		if removeErr := os.RemoveAll(addonBackupPath(addonName, backupName)); removeErr != nil {
			return nil, removeErr
		}
	}
	return g.Map{"deleted": len(cleanNames)}, nil
}

func (s *sBaseSysAddonsService) Restore(ctx context.Context, req *v1.AddonsRestoreReq) (data interface{}, err error) {
	targets, err := resolveAddonTargets(ctx, []string{req.Id})
	if err != nil {
		return nil, err
	}
	if err = ensureAddonTargetsInstalled(ctx, targets); err != nil {
		return nil, err
	}
	if !isSafeBackupName(targets[0].Name, req.BackupName) {
		return nil, gerror.New("备份批次无效")
	}
	if _, statErr := os.Stat(addonBackupPath(targets[0].Name, req.BackupName)); statErr != nil {
		return nil, gerror.New("备份批次不存在")
	}
	task := newAddonTask("restore")
	fileNames := append([]string{}, req.FileNames...)
	if len(fileNames) == 0 && req.FileName != "" {
		fileNames = append(fileNames, req.FileName)
	}
	go runAddonTask(task, targets, func(taskCtx context.Context, target addonTarget, progress func(int, string)) error {
		return restoreAddon(taskCtx, target, req.BackupName, fileNames, progress, func(tableName, fileName string, restoreErr error) {
			task.recordError(target.Name, tableName, fileName, restoreErr)
		})
	})
	return g.Map{"taskId": task.ID}, nil
}

func (s *sBaseSysAddonsService) Task(_ context.Context, req *v1.AddonsTaskReq) (data interface{}, err error) {
	value, ok := addonTasks.Load(req.TaskId)
	if !ok {
		return nil, gerror.New("任务不存在或已过期")
	}
	return value.(*addonTask).data(), nil
}

func (s *sBaseSysAddonsService) CancelTask(_ context.Context, req *v1.AddonsTaskCancelReq) (data interface{}, err error) {
	value, ok := addonTasks.Load(req.TaskId)
	if !ok {
		return nil, gerror.New("任务不存在或已过期")
	}
	if err = value.(*addonTask).requestCancel(); err != nil {
		return nil, err
	}
	return g.Map{"taskId": req.TaskId, "message": "已请求停止任务"}, nil
}

func runAddonTask(task *addonTask, targets []addonTarget, handle func(context.Context, addonTarget, func(int, string)) error) {
	ctx, cancel := context.WithCancel(gctx.New())
	task.setCancel(cancel)
	defer cancel()
	var releaseRestore func()
	if task.Operation == "restore" {
		var err error
		releaseRestore, err = acquireAddonRestoreSlot(ctx)
		if err != nil {
			task.update("cancelled", task.progressValue(), "恢复任务已停止")
			return
		}
		defer releaseRestore()
	}
	task.update("running", 1, "任务执行中")
	for index, target := range targets {
		if err := ctx.Err(); err != nil {
			task.update("cancelled", task.progressValue(), addonTaskOperationName(task.Operation)+"任务已停止")
			return
		}
		release, lockErr := acquireAddonTaskLock(ctx, target.Name)
		if lockErr != nil {
			task.update("cancelled", task.progressValue(), addonTaskOperationName(task.Operation)+"任务已停止")
			return
		}
		start := index * 100 / len(targets)
		end := (index + 1) * 100 / len(targets)
		progress := func(percent int, message string) {
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			task.update("running", start+(end-start)*percent/100, message)
		}
		err := handle(ctx, target, progress)
		release()
		// A database driver might wrap context.Canceled in its own error. The
		// task context is authoritative: a user-requested stop must finish as
		// cancelled rather than remain in the cancelling state or become failed.
		if ctx.Err() != nil {
			task.update("cancelled", task.progressValue(), addonTaskOperationName(task.Operation)+"任务已停止")
			return
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				task.update("cancelled", task.progressValue(), addonTaskOperationName(task.Operation)+"任务已停止")
				return
			}
			task.fail(err)
			return
		}
	}
	if task.Operation == "restore" {
		task.update("completed", 100, task.completionMessage())
		return
	}
	task.update("completed", 100, "任务完成")
}

func resolveAddonTargets(ctx context.Context, ids []string) ([]addonTarget, error) {
	if len(ids) == 0 {
		return nil, gerror.New("请选择插件")
	}
	seen := make(map[string]struct{}, len(ids))
	cleanIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				cleanIDs = append(cleanIDs, id)
			}
		}
	}
	addons, err := dao.BaseSysAddons.Ctx(ctx).Fields("menuId,name,addonsName").Where("menuId IN (?)", cleanIDs).All()
	if err != nil {
		return nil, err
	}
	if len(addons) != len(cleanIDs) {
		return nil, gerror.New("所选插件不存在")
	}
	targets := make([]addonTarget, 0, len(addons))
	for _, addon := range addons {
		addonName := addon["addonsName"].String()
		if addonName == "" {
			addonName = inferAddonName(addon["name"].String(), "")
		}
		if addonName == "" {
			return nil, gerror.Newf("插件 %s 未配置英文名称", addon["name"].String())
		}
		targets = append(targets, addonTarget{MenuID: addon["menuId"].String(), Name: addonName})
	}
	return targets, nil
}

func ensureAddonTargetsInstalled(ctx context.Context, targets []addonTarget) error {
	menuIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		menuIDs = append(menuIDs, target.MenuID)
	}
	count, err := dao.BaseSysMenu.Ctx(ctx).
		Where("id IN (?)", menuIDs).
		Where("isInstall", true).
		Count()
	if err != nil {
		return err
	}
	if count != len(targets) {
		return gerror.New("未安装的插件不支持备份")
	}
	return nil
}

func addonBackupRoot() string {
	return filepath.Join(addonBackupRootDirectory, addonBackupDirectory)
}

func addonBackupDirectoryPath(addonName string) string {
	return filepath.Join(addonBackupRoot(), addonName)
}

func addonBackupFilePath(addonName, fileName string) string {
	return filepath.Join(addonBackupDirectoryPath(addonName), fileName)
}

func addonBackupBatchPath(addonName, backupName string) string {
	return filepath.Join(addonBackupDirectoryPath(addonName), backupName)
}

func addonBackupTablePath(addonName, backupName, fileName string) string {
	return filepath.Join(addonBackupBatchPath(addonName, backupName), fileName)
}

func normalizeAddonBackupType(backupType string) string {
	if backupType == addonBackupTypeStructure {
		return addonBackupTypeStructure
	}
	return addonBackupTypeData
}

func addonBackupTypeLabel(backupType string) string {
	if normalizeAddonBackupType(backupType) == addonBackupTypeStructure {
		return "结构备份"
	}
	return "数据备份"
}

func readAddonBackupType(addonName, backupName string) string {
	metaContent, err := os.ReadFile(filepath.Join(addonBackupBatchPath(addonName, backupName), addonBackupMetaFile))
	if err != nil {
		// Old backup batches did not have metadata and contain schema plus data.
		return addonBackupTypeData
	}
	var meta addonBackupMeta
	if json.Unmarshal(metaContent, &meta) != nil {
		return addonBackupTypeData
	}
	return normalizeAddonBackupType(meta.BackupType)
}

func addonBackupPath(addonName, backupName string) string {
	if isSafeBackupName(addonName, backupName) {
		return addonBackupBatchPath(addonName, backupName)
	}
	return addonBackupFilePath(addonName, backupName)
}

func isSafeBackupFileName(addonName, fileName string) bool {
	if filepath.Base(fileName) != fileName {
		return false
	}
	return regexp.MustCompile("^" + regexp.QuoteMeta(addonName) + `_[0-9]{8}_[0-9]{6}_[0-9]{3}_[0-9]+\.sql$`).MatchString(fileName)
}

func isSafeBackupName(addonName, backupName string) bool {
	if filepath.Base(backupName) != backupName {
		return false
	}
	return regexp.MustCompile("^" + regexp.QuoteMeta(addonName) + `_[0-9]{8}_[0-9]{6}_[0-9]{3}_[0-9]+$`).MatchString(backupName)
}

func isSafeBackupTableFileName(fileName string) bool {
	if filepath.Base(fileName) != fileName {
		return false
	}
	return regexp.MustCompile(`^[0-9]{4}_[a-zA-Z0-9_]+_[0-9]+\.sql$`).MatchString(fileName)
}

type addonBackupFileRef struct {
	BackupName string
	FileName   string
	TableName  string
	Path       string
	Size       int64
	CreateTime string
}

func addonBackupTableName(fileName string) string {
	name := strings.TrimSuffix(fileName, ".sql")
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 {
		return name
	}
	name = parts[1]
	if index := strings.LastIndex(name, "_"); index > 0 {
		if _, err := fmt.Sscan(name[index+1:], new(int64)); err == nil {
			name = name[:index]
		}
	}
	return name
}

func listAddonBackupFiles(addonName, backupName string) ([]addonBackupFileRef, error) {
	if isSafeBackupFileName(addonName, backupName) {
		info, err := os.Stat(addonBackupFilePath(addonName, backupName))
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []addonBackupFileRef{{
			BackupName: backupName,
			FileName:   backupName,
			TableName:  "旧版备份（包含多个表）",
			Path:       addonBackupFilePath(addonName, backupName),
			Size:       info.Size(),
			CreateTime: info.ModTime().Format(time.DateTime),
		}}, nil
	}
	if !isSafeBackupName(addonName, backupName) {
		return nil, gerror.New("备份批次无效")
	}
	entries, err := os.ReadDir(addonBackupBatchPath(addonName, backupName))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]addonBackupFileRef, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isSafeBackupTableFileName(entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		files = append(files, addonBackupFileRef{
			BackupName: backupName,
			FileName:   entry.Name(),
			TableName:  addonBackupTableName(entry.Name()),
			Path:       addonBackupTablePath(addonName, backupName, entry.Name()),
			Size:       info.Size(),
			CreateTime: info.ModTime().Format(time.DateTime),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].FileName < files[j].FileName })
	return files, nil
}

func listAddonBackups(addonName string) (g.List, error) {
	dir := addonBackupDirectoryPath(addonName)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return g.List{}, nil
	}
	if err != nil {
		return nil, err
	}
	list := make(g.List, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !isSafeBackupName(addonName, entry.Name()) {
			continue
		}
		if !entry.IsDir() && !isSafeBackupFileName(addonName, entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		backupName := entry.Name()
		files, filesErr := listAddonBackupFiles(addonName, backupName)
		if filesErr != nil {
			return nil, filesErr
		}
		size := info.Size()
		backupType := addonBackupTypeData
		if entry.IsDir() {
			size = 0
			for _, file := range files {
				size += file.Size
			}
			backupType = readAddonBackupType(addonName, backupName)
		}
		list = append(list, g.Map{
			"backupName": backupName,
			"backupType": backupType,
			"fileName":   backupName, // 兼容旧版前端展示
			"size":       size,
			"tableCount": len(files),
			"createTime": info.ModTime().Format(time.DateTime),
			"legacy":     !entry.IsDir(),
		})
	}
	sort.Slice(list, func(i, j int) bool { return gconv.String(list[i]["backupName"]) > gconv.String(list[j]["backupName"]) })
	return list, nil
}

func addonBackupDetail(addonName, backupName, fileName string) (g.Map, error) {
	files, err := listAddonBackupFiles(addonName, backupName)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, gerror.New("备份批次不存在或为空")
	}
	tables := make(g.List, 0, len(files))
	for _, file := range files {
		item := g.Map{
			"fileName":   file.FileName,
			"tableName":  file.TableName,
			"size":       file.Size,
			"createTime": file.CreateTime,
		}
		tables = append(tables, item)
	}
	if fileName != "" {
		found := false
		for _, file := range files {
			found = found || file.FileName == fileName
		}
		if !found {
			return nil, gerror.New("备份表文件不存在")
		}
	}
	return g.Map{
		"backupName": backupName,
		"backupType": readAddonBackupType(addonName, backupName),
		"tables":     tables,
	}, nil
}

func findAddonBackupFile(addonName, backupName, fileName string) (addonBackupFileRef, error) {
	files, err := listAddonBackupFiles(addonName, backupName)
	if err != nil {
		return addonBackupFileRef{}, err
	}
	for _, file := range files {
		if file.FileName == fileName {
			return file, nil
		}
	}
	return addonBackupFileRef{}, gerror.New("备份表文件不存在")
}

func addonBackupPreview(addonName, backupName, fileName string, offset, limit int64) (g.Map, error) {
	file, err := findAddonBackupFile(addonName, backupName, fileName)
	if err != nil {
		return nil, err
	}
	if offset < 0 || offset > file.Size {
		return nil, gerror.New("预览位置无效")
	}
	if limit <= 0 {
		limit = addonBackupPreviewDefaultLimit
	}
	if limit > addonBackupPreviewMaxLimit {
		limit = addonBackupPreviewMaxLimit
	}

	previewFile, err := os.Open(file.Path)
	if err != nil {
		return nil, err
	}
	defer previewFile.Close()
	if _, err = previewFile.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	content := make([]byte, limit)
	readSize, readErr := previewFile.Read(content)
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}
	nextOffset := offset + int64(readSize)
	return g.Map{
		"fileName":   file.FileName,
		"content":    string(content[:readSize]),
		"offset":     offset,
		"nextOffset": nextOffset,
		"totalSize":  file.Size,
		"hasMore":    nextOffset < file.Size,
	}, nil
}

func addonModels(addonName string) []dzhcore.IModel {
	models := make([]dzhcore.IModel, 0)
	for _, item := range dzhcore.RegisteredModels() {
		if isAddonTable(addonName, item.TableName()) {
			models = append(models, item)
		}
	}
	return models
}

func isAddonTable(addonName, tableName string) bool {
	return strings.HasPrefix(normalizeAddonIdentifier(tableName), "addons"+normalizeAddonIdentifier(addonName))
}

func normalizeAddonIdentifier(value string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(value))
}

func isSharedAddonDataTable(tableName string) bool {
	switch tableName {
	case model.TableNameBaseSysMenu, model.TableNameBaseSysSettingExt, "addons_dict_info", "addons_dict_type", "addons_task_info":
		return true
	default:
		return false
	}
}

func addonExistingTables(ctx context.Context, addonName string) ([]string, error) {
	expected := addonModels(addonName)
	actual, err := g.DB().Tables(ctx)
	if err != nil {
		return nil, err
	}
	exists := make(map[string]string, len(actual))
	for _, table := range actual {
		exists[strings.ToLower(table)] = table
	}
	tables := make([]string, 0, len(expected))
	for _, item := range expected {
		if isSharedAddonDataTable(item.TableName()) {
			continue
		}
		if table, ok := exists[strings.ToLower(item.TableName())]; ok {
			tables = append(tables, table)
		}
	}
	sort.Strings(tables)
	return tables, nil
}

func addonBackupTables(ctx context.Context, addonName string) ([]addonBackupTable, error) {
	ownedTables, err := addonExistingTables(ctx, addonName)
	if err != nil {
		return nil, err
	}
	actual, err := g.DB().Tables(ctx)
	if err != nil {
		return nil, err
	}
	exists := make(map[string]string, len(actual))
	for _, table := range actual {
		exists[strings.ToLower(table)] = table
	}
	tables := make([]addonBackupTable, 0, len(ownedTables)+5)
	for _, table := range ownedTables {
		tables = append(tables, addonBackupTable{Name: table})
	}
	for _, definition := range addonSharedBackupTables(addonName) {
		if table, ok := exists[strings.ToLower(definition.Name)]; ok {
			definition.Name = table
			tables = append(tables, definition)
		}
	}
	return tables, nil
}

func addonSharedBackupTables(addonName string) []addonBackupTable {
	dictOwner := addonName
	if addonName == "dict" {
		// The dictionary addon historically owns the built-in "base" dictionaries.
		dictOwner = "base"
	}
	return []addonBackupTable{
		{Name: model.TableNameBaseSysMenu, Shared: true, OwnershipKey: "addonsName", OwnershipName: addonName},
		{Name: model.TableNameBaseSysSettingExt, Shared: true, OwnershipKey: "module", OwnershipName: addonName},
		{Name: "addons_dict_type", Shared: true, OwnershipKey: "addonsName", OwnershipName: dictOwner, InitIDs: addonInitRecordIDs(addonName, "addons_dict_type")},
		{Name: "addons_dict_info", Shared: true, OwnershipKey: "addonsName", OwnershipName: dictOwner, InitIDs: addonInitRecordIDs(addonName, "addons_dict_info")},
		{Name: "addons_task_info", Shared: true, OwnershipKey: "addonsName", OwnershipName: addonName, InitIDs: addonInitRecordIDs(addonName, "addons_task_info")},
	}
}

func addonInitRecordIDs(addonName, tableName string) []string {
	path := "addons/" + addonName + "/resource/initjson/" + tableName + ".json"
	jsonData, err := gjson.LoadContent(gres.GetContent(path))
	if err != nil || jsonData.Var().IsEmpty() {
		return nil
	}
	var rows []g.Map
	if err = jsonData.Scan(&rows); err != nil {
		return nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := gconv.String(row["id"]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func backupAddon(ctx context.Context, target addonTarget, backupType string, progress func(int, string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tables, err := addonBackupTables(ctx, target.Name)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return gerror.Newf("插件 %s 没有可备份的数据表", target.Name)
	}
	if err = os.MkdirAll(addonBackupDirectoryPath(target.Name), 0755); err != nil {
		return err
	}
	backupName := fmt.Sprintf("%s_%s_%s", target.Name, time.Now().Format("20060102_150405_000"), dzhcore.NodeSnowflake.Generate().String())
	backupDir := addonBackupBatchPath(target.Name, backupName)
	if err = os.Mkdir(backupDir, 0755); err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = os.RemoveAll(backupDir)
		}
	}()
	meta, marshalErr := json.Marshal(addonBackupMeta{BackupType: backupType})
	if marshalErr != nil {
		return marshalErr
	}
	if err = os.WriteFile(filepath.Join(backupDir, addonBackupMetaFile), meta, 0644); err != nil {
		return err
	}

	for index, table := range tables {
		if err = ctx.Err(); err != nil {
			return err
		}
		progress(index*100/len(tables), fmt.Sprintf("正在备份 %s", table.Name))
		fileName := fmt.Sprintf("%04d_%s_%s.sql", index+1, table.Name, dzhcore.NodeSnowflake.Generate().String())
		filePath := addonBackupTablePath(target.Name, backupName, fileName)
		file, openErr := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if openErr != nil {
			return openErr
		}
		writer := bufio.NewWriter(file)
		_, _ = fmt.Fprintf(writer, "-- DZH3136 addon backup table\n-- addon: %s\n-- table: %s\n-- generated: %s\n\n", target.Name, table.Name, time.Now().Format(time.RFC3339))
		if strings.EqualFold(g.DB().GetConfig().Type, "mysql") {
			_, _ = writer.WriteString("SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS=0;\n\n")
		}
		writeErr := writeAddonTableBackup(ctx, writer, table, backupType == addonBackupTypeData)
		if strings.EqualFold(g.DB().GetConfig().Type, "mysql") {
			_, _ = writer.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")
		}
		if flushErr := writer.Flush(); writeErr == nil {
			writeErr = flushErr
		}
		closeErr := file.Close()
		if writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			return writeErr
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	completed = true
	progress(100, fmt.Sprintf("备份已保存：%s（%s，%d 张表）", backupName, addonBackupTypeLabel(backupType), len(tables)))
	return nil
}

func writeAddonTableBackup(ctx context.Context, writer *bufio.Writer, backupTable addonBackupTable, includeData bool) error {
	db := g.DB()
	table := backupTable.Name
	quotedTable := db.GetCore().QuoteWord(table)
	if !backupTable.Shared {
		createSQL, err := addonCreateSQL(ctx, table)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(writer, "DROP TABLE IF EXISTS %s;\n%s;\n", quotedTable, strings.TrimSuffix(strings.TrimSpace(createSQL), ";"))
	} else if includeData {
		if err := writeSharedTableDelete(ctx, writer, backupTable); err != nil {
			return err
		}
	} else {
		_, _ = fmt.Fprintf(writer, "-- 结构备份：共享表 %s 的表结构由系统维护，本文件不包含数据。\n", table)
		return nil
	}
	if !includeData {
		return nil
	}

	fields, err := db.TableFields(ctx, table)
	if err != nil {
		return err
	}
	ordered := make([]*gdb.TableField, 0, len(fields))
	for _, field := range fields {
		ordered = append(ordered, field)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Index < ordered[j].Index })
	fieldNames := make([]string, 0, len(ordered))
	for _, field := range ordered {
		fieldNames = append(fieldNames, db.GetCore().QuoteWord(field.Name))
	}
	rows, err := addonBackupRows(ctx, backupTable)
	if err != nil {
		return err
	}
	insertValues := make([]string, 0, addonInsertBatchRows)
	insertBytes := 0
	flushInsert := func() error {
		if len(insertValues) == 0 {
			return nil
		}
		_, writeErr := fmt.Fprintf(
			writer,
			"INSERT INTO %s (%s) VALUES %s;\n",
			quotedTable,
			strings.Join(fieldNames, ", "),
			strings.Join(insertValues, ", "),
		)
		insertValues = insertValues[:0]
		insertBytes = 0
		return writeErr
	}
	for index, row := range rows {
		if index%64 == 0 {
			if err = ctx.Err(); err != nil {
				return err
			}
		}
		values := make([]string, 0, len(ordered))
		for _, field := range ordered {
			value := row[field.Name]
			if value == nil || value.Val() == nil {
				values = append(values, "NULL")
				continue
			}
			values = append(values, addonSQLLiteral(value.Val()))
		}
		rowValues := "(" + strings.Join(values, ", ") + ")"
		if len(insertValues) > 0 &&
			(len(insertValues) >= addonInsertBatchRows || insertBytes+len(rowValues) > addonInsertBatchBytes) {
			if err = flushInsert(); err != nil {
				return err
			}
		}
		insertValues = append(insertValues, rowValues)
		insertBytes += len(rowValues)
	}
	if err = flushInsert(); err != nil {
		return err
	}
	_, _ = writer.WriteString("\n")
	return nil
}

func addonBackupRows(ctx context.Context, backupTable addonBackupTable) (gdb.Result, error) {
	query := g.DB().Model(backupTable.Name).Ctx(ctx).Unscoped()
	if !backupTable.Shared {
		return query.All()
	}
	if len(backupTable.InitIDs) == 0 {
		return query.Where(backupTable.OwnershipKey, backupTable.OwnershipName).All()
	}
	return query.Where(backupTable.OwnershipKey+" = ? OR id IN (?)", backupTable.OwnershipName, backupTable.InitIDs).All()
}

func writeSharedTableDelete(_ context.Context, writer *bufio.Writer, backupTable addonBackupTable) error {
	db := g.DB()
	quotedTable := db.GetCore().QuoteWord(backupTable.Name)
	quotedKey := db.GetCore().QuoteWord(backupTable.OwnershipKey)
	_, err := fmt.Fprintf(writer, "DELETE FROM %s WHERE %s = %s;\n", quotedTable, quotedKey, addonSQLLiteral(backupTable.OwnershipName))
	if err != nil || len(backupTable.InitIDs) == 0 {
		return err
	}
	ids := make([]string, 0, len(backupTable.InitIDs))
	for _, id := range backupTable.InitIDs {
		ids = append(ids, addonSQLLiteral(id))
	}
	_, err = fmt.Fprintf(writer, "DELETE FROM %s WHERE %s IN (%s);\n", quotedTable, db.GetCore().QuoteWord("id"), strings.Join(ids, ", "))
	return err
}

func addonCreateSQL(ctx context.Context, table string) (string, error) {
	db := g.DB()
	if strings.EqualFold(db.GetConfig().Type, "sqlite") {
		result, err := db.Query(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table)
		if err != nil || len(result) == 0 || result[0]["sql"].IsEmpty() {
			return "", gerror.Newf("读取表 %s 结构失败", table)
		}
		return result[0]["sql"].String(), nil
	}
	result, err := db.Query(ctx, "SHOW CREATE TABLE "+db.GetCore().QuoteWord(table))
	if err != nil || len(result) == 0 {
		return "", gerror.Newf("读取表 %s 结构失败", table)
	}
	for key, value := range result[0] {
		if strings.EqualFold(key, "Create Table") || strings.Contains(strings.ToLower(key), "create") {
			return value.String(), nil
		}
	}
	return "", gerror.Newf("读取表 %s 结构失败", table)
}

func addonSQLLiteral(value any) string {
	switch item := value.(type) {
	case bool:
		if item {
			return "1"
		}
		return "0"
	case []byte:
		return "X'" + hex.EncodeToString(item) + "'"
	case time.Time:
		return "'" + item.Format("2006-01-02 15:04:05.999999") + "'"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(item)
	default:
		text := fmt.Sprint(value)
		text = strings.ReplaceAll(text, "\\", "\\\\")
		text = strings.ReplaceAll(text, "'", "''")
		text = strings.ReplaceAll(text, "\x00", "\\0")
		return "'" + text + "'"
	}
}

func restoreAddon(ctx context.Context, target addonTarget, backupName string, fileNames []string, progress func(int, string), recordError func(tableName, fileName string, err error)) error {
	files, err := listAddonBackupFiles(target.Name, backupName)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return gerror.New("备份批次中没有可恢复的数据表")
	}
	selected := make([]addonBackupFileRef, 0, len(files))
	selectedNames := make(map[string]struct{}, len(fileNames))
	for _, fileName := range fileNames {
		selectedNames[fileName] = struct{}{}
	}
	for _, file := range files {
		if len(selectedNames) == 0 {
			selected = append(selected, file)
			continue
		}
		if _, ok := selectedNames[file.FileName]; ok {
			selected = append(selected, file)
		}
	}
	if len(selected) == 0 {
		return gerror.New("备份表文件不存在")
	}
	for index, file := range selected {
		if err = ctx.Err(); err != nil {
			return err
		}
		start := index * 100 / len(selected)
		end := (index + 1) * 100 / len(selected)
		tableProgress := func(percent int, message string) {
			progress(start+(end-start)*percent/100, message)
		}
		progress(start, addonRestorePreparingMessage(file.TableName, file.FileName))
		if err = restoreAddonFile(ctx, target, file.Path, file.FileName, file.TableName, tableProgress); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil || errors.Is(err, context.Canceled) {
				if ctxErr != nil {
					return ctxErr
				}
				return err
			}
			if recordError != nil {
				recordError(file.TableName, file.FileName, err)
			}
			continue
		}
	}
	return nil
}

func addonRestorePreparingMessage(tableName, fileName string) string {
	return fmt.Sprintf("当前恢复表：%s\n当前恢复 SQL：%s\n执行进度：准备中", tableName, fileName)
}

func addonRestoreProgressMessage(tableName, fileName string, current, total int) string {
	return fmt.Sprintf("当前恢复表：%s\n当前恢复 SQL：%s\n执行进度：%d/%d 条", tableName, fileName, current, total)
}

func restoreAddonFile(ctx context.Context, target addonTarget, filePath, fileName, tableName string, progress func(int, string)) error {
	allowed := make(map[string]struct{})
	for _, item := range addonModels(target.Name) {
		allowed[strings.ToLower(item.TableName())] = struct{}{}
	}
	for _, item := range addonSharedBackupTables(target.Name) {
		allowed[strings.ToLower(item.Name)] = struct{}{}
	}
	return restoreSQLFile(ctx, filePath, fileName, tableName, allowed, progress)
}

func restoreSQLFile(ctx context.Context, filePath, fileName, tableName string, allowed map[string]struct{}, progress func(int, string)) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	fileSize := fileInfo.Size()
	reader := bufio.NewReaderSize(file, addonRestoreReaderBufferSize)

	// Keep all restore statements on one dedicated connection. The normal
	// connection pool may choose different connections for individual Exec calls;
	// setting the charset only once on a pooled connection would then be unreliable.
	db, err := g.DB().GetCore().Master()
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if strings.EqualFold(g.DB().GetConfig().Type, "mysql") {
		if _, err = conn.ExecContext(ctx, "SET NAMES utf8mb4"); err != nil {
			return err
		}
	}

	// The parser and executor deliberately use bounded state. A backup file can
	// be many gigabytes, but the process only keeps the current SQL statement and
	// the current INSERT chunk in memory.
	type restoreDataBatchItem struct {
		sql         string
		sourceCount int
	}
	dataBatch := make([]restoreDataBatchItem, 0, addonRestoreTransactionStatementCount)
	dataBatchBytes := 0
	parsedStatements := 0
	committedStatements := 0
	lastProgressAt := time.Time{}
	lastProgressStatement := -1
	readOffset := func() int64 {
		position, seekErr := file.Seek(0, io.SeekCurrent)
		if seekErr != nil {
			return 0
		}
		position -= int64(reader.Buffered())
		if position < 0 {
			return 0
		}
		if position > fileSize {
			return fileSize
		}
		return position
	}
	emitProgress := func(force bool) {
		offset := readOffset()
		percent := 0
		if fileSize > 0 {
			percent = int(offset * 100 / fileSize)
			if percent > 100 {
				percent = 100
			}
		}
		now := time.Now()
		if !force && percent < 100 &&
			!lastProgressAt.IsZero() &&
			now.Sub(lastProgressAt) < addonRestoreProgressInterval &&
			parsedStatements-lastProgressStatement < addonRestoreProgressStatementStep {
			return
		}
		progress(percent, addonRestoreStreamProgressMessage(
			tableName,
			fileName,
			parsedStatements,
			committedStatements,
			percent,
		))
		lastProgressAt = now
		lastProgressStatement = parsedStatements
	}
	emitProgress(true)

	flushDataBatch := func() error {
		if len(dataBatch) == 0 {
			return nil
		}
		tx, txErr := conn.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		batchSourceCount := 0
		for _, item := range dataBatch {
			if ctxErr := ctx.Err(); ctxErr != nil {
				_ = tx.Rollback()
				return ctxErr
			}
			if _, execErr := tx.ExecContext(ctx, item.sql); execErr != nil {
				_ = tx.Rollback()
				return execErr
			}
			batchSourceCount += item.sourceCount
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		committedStatements += batchSourceCount
		dataBatch = dataBatch[:0]
		dataBatchBytes = 0
		emitProgress(false)
		return restoreYield(ctx)
	}
	appendDataStatement := func(statement string, sourceCount int) error {
		dataBatch = append(dataBatch, restoreDataBatchItem{
			sql:         statement,
			sourceCount: sourceCount,
		})
		dataBatchBytes += len(statement)
		if len(dataBatch) >= addonRestoreTransactionStatementCount || dataBatchBytes >= addonInsertBatchBytes {
			return flushDataBatch()
		}
		return nil
	}

	var (
		pendingInsertPrefix     string
		pendingInsertPrefixKey  string
		pendingInsertValues     []string
		pendingInsertRows       int
		pendingInsertValueBytes int
		pendingInsertStatements int
	)
	flushPendingInsert := func() error {
		if len(pendingInsertValues) == 0 {
			return nil
		}
		statement := pendingInsertPrefix + " " + strings.Join(pendingInsertValues, ", ")
		err = appendDataStatement(statement, pendingInsertStatements)
		pendingInsertPrefix = ""
		pendingInsertPrefixKey = ""
		pendingInsertValues = nil
		pendingInsertRows = 0
		pendingInsertValueBytes = 0
		pendingInsertStatements = 0
		return err
	}

	streamErr := streamAddonSQLStatements(ctx, reader, func(statement string) error {
		parsedStatements++
		if err = ctx.Err(); err != nil {
			return err
		}
		clean := cleanAddonBackupStatement(statement)
		if !isAllowedAddonBackupStatement(statement) {
			return gerror.New("备份文件包含不允许执行的 SQL 语句")
		}
		for _, match := range addonTableRegexp.FindAllStringSubmatch(clean, -1) {
			if _, ok := allowed[strings.ToLower(match[1])]; !ok {
				return gerror.Newf("备份文件包含非该插件的数据表：%s", match[1])
			}
		}

		if isAddonBackupDataStatement(statement) {
			insertPrefix, insertValues, ok := splitAddonInsertStatement(statement)
			if ok {
				insertRows := addonInsertValuesCount(insertValues)
				if insertRows > 0 {
					insertPrefixKey := strings.ToUpper(strings.Join(strings.Fields(insertPrefix), " "))
					if len(pendingInsertValues) > 0 &&
						(pendingInsertPrefixKey != insertPrefixKey ||
							pendingInsertRows+insertRows > addonInsertBatchRows ||
							pendingInsertValueBytes+len(insertValues) > addonInsertBatchBytes) {
						if err = flushPendingInsert(); err != nil {
							return err
						}
					}
					if len(pendingInsertValues) == 0 {
						pendingInsertPrefix = insertPrefix
						pendingInsertPrefixKey = insertPrefixKey
					}
					pendingInsertValues = append(pendingInsertValues, insertValues)
					pendingInsertRows += insertRows
					pendingInsertValueBytes += len(insertValues)
					pendingInsertStatements++
					if pendingInsertRows >= addonInsertBatchRows || pendingInsertValueBytes >= addonInsertBatchBytes {
						return flushPendingInsert()
					}
					emitProgress(false)
					return nil
				}
			}
			if err = flushPendingInsert(); err != nil {
				return err
			}
			if err = appendDataStatement(statement, 1); err != nil {
				return err
			}
			emitProgress(false)
			return nil
		}

		if err = flushPendingInsert(); err != nil {
			return err
		}
		if err = flushDataBatch(); err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, statement); err != nil {
			return err
		}
		committedStatements++
		emitProgress(false)
		return nil
	})
	if streamErr != nil {
		return streamErr
	}
	if parsedStatements == 0 {
		return gerror.New("备份文件中没有可恢复的数据")
	}
	if err = flushPendingInsert(); err != nil {
		return err
	}
	if err = flushDataBatch(); err != nil {
		return err
	}
	progress(100, addonRestoreStreamProgressMessage(
		tableName,
		fileName,
		parsedStatements,
		committedStatements,
		100,
	))
	return nil
}

func addonRestoreStreamProgressMessage(tableName, fileName string, parsed, committed, percent int) string {
	return fmt.Sprintf(
		"当前恢复表：%s\n当前恢复 SQL：%s\n执行进度：已读取 %d 条，已执行 %d 条，文件进度：%d%%",
		tableName,
		fileName,
		parsed,
		committed,
		percent,
	)
}

func restoreYield(ctx context.Context) error {
	timer := time.NewTimer(addonRestoreYieldInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// streamAddonSQLStatements parses one SQL statement at a time. It is
// intentionally byte-based: quote markers are ASCII and the original UTF-8
// bytes are copied unchanged to the database.
func streamAddonSQLStatements(ctx context.Context, reader *bufio.Reader, handle func(string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var builder strings.Builder
	var quote byte
	var escaped bool
	processChunk := func(chunk []byte) error {
		for _, char := range chunk {
			builder.WriteByte(char)
			if builder.Len() > addonRestoreMaxStatementBytes {
				return gerror.Newf("单条 SQL 超过 %dMB，拒绝继续恢复", addonRestoreMaxStatementBytes/(1024*1024))
			}
			if quote != 0 {
				if quote == '\'' && char == '\\' && !escaped {
					escaped = true
					continue
				}
				if char == quote && !escaped {
					quote = 0
				}
				escaped = false
				continue
			}
			switch char {
			case '\'', '"', '`':
				quote = char
			case ';':
				statement := strings.TrimSpace(builder.String())
				statement = strings.TrimSpace(strings.TrimSuffix(statement, ";"))
				if statement != "" {
					if err := handle(statement); err != nil {
						return err
					}
				}
				builder.Reset()
			}
		}
		return nil
	}
	for {
		chunk, readErr := reader.ReadSlice('\n')
		if len(chunk) > 0 {
			if err := processChunk(chunk); err != nil {
				return err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, bufio.ErrBufferFull) {
				continue
			}
			if errors.Is(readErr, io.EOF) {
				if statement := strings.TrimSpace(builder.String()); statement != "" {
					if err := handle(statement); err != nil {
						return err
					}
				}
				return nil
			}
			return readErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func isAddonBackupDataStatement(statement string) bool {
	clean := cleanAddonBackupStatement(statement)
	return strings.HasPrefix(clean, "INSERT INTO") || strings.HasPrefix(clean, "DELETE FROM")
}

func isAllowedAddonBackupStatement(statement string) bool {
	clean := cleanAddonBackupStatement(statement)
	return strings.HasPrefix(clean, "SET NAMES ") ||
		strings.HasPrefix(clean, "SET FOREIGN_KEY_CHECKS=") ||
		strings.HasPrefix(clean, "DROP TABLE") ||
		strings.HasPrefix(clean, "CREATE TABLE") ||
		strings.HasPrefix(clean, "INSERT INTO") ||
		strings.HasPrefix(clean, "DELETE FROM")
}

func cleanAddonBackupStatement(statement string) string {
	lines := strings.Split(statement, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			filtered = append(filtered, line)
		}
	}
	return strings.ToUpper(strings.TrimSpace(strings.Join(filtered, "\n")))
}

// combineAddonInsertStatements upgrades old backups that stored one INSERT per
// row into the same bounded multi-row INSERT batches produced by new backups.
func combineAddonInsertStatements(statements []string) []string {
	result, _ := combineAddonInsertStatementsContext(context.Background(), statements)
	return result
}

func combineAddonInsertStatementsContext(ctx context.Context, statements []string) ([]string, error) {
	result := make([]string, 0, len(statements))
	var (
		prefix     string
		prefixKey  string
		values     []string
		valueCount int
		valueBytes int
	)
	flush := func() {
		if len(values) == 0 {
			return
		}
		result = append(result, prefix+" "+strings.Join(values, ", "))
		prefix = ""
		prefixKey = ""
		values = values[:0]
		valueCount = 0
		valueBytes = 0
	}
	for index, statement := range statements {
		if index%32 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		insertPrefix, insertValues, ok := splitAddonInsertStatement(statement)
		if !ok {
			flush()
			result = append(result, statement)
			continue
		}
		insertCount := addonInsertValuesCount(insertValues)
		if insertCount == 0 {
			flush()
			result = append(result, statement)
			continue
		}
		key := strings.ToUpper(strings.Join(strings.Fields(insertPrefix), " "))
		if len(values) > 0 &&
			(prefixKey != key ||
				valueCount+insertCount > addonInsertBatchRows ||
				valueBytes+len(insertValues) > addonInsertBatchBytes) {
			flush()
		}
		if len(values) == 0 {
			prefix = insertPrefix
			prefixKey = key
		}
		values = append(values, insertValues)
		valueCount += insertCount
		valueBytes += len(insertValues)
	}
	flush()
	return result, nil
}

func splitAddonInsertStatement(statement string) (prefix, values string, ok bool) {
	statement = strings.TrimSpace(statement)
	if !strings.HasPrefix(strings.ToUpper(statement), "INSERT INTO") {
		return "", "", false
	}
	var (
		quote   rune
		escaped bool
		depth   int
	)
	for index, char := range statement {
		if quote != 0 {
			if quote == '\'' && char == '\\' && !escaped {
				escaped = true
				continue
			}
			if char == quote && !escaped {
				quote = 0
			}
			escaped = false
			continue
		}
		if depth == 0 && index+len("VALUES") <= len(statement) &&
			strings.EqualFold(statement[index:index+len("VALUES")], "VALUES") &&
			(index == 0 || !isAddonSQLIdentifierRune(rune(statement[index-1]))) &&
			(index+len("VALUES") == len(statement) || !isAddonSQLIdentifierRune(rune(statement[index+len("VALUES")]))) {
			return strings.TrimSpace(statement[:index+len("VALUES")]), strings.TrimSpace(statement[index+len("VALUES"):]), true
		}
		switch char {
		case '\'', '"', 96:
			quote = char
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return "", "", false
}

func addonInsertValuesCount(values string) int {
	var (
		quote   rune
		escaped bool
		depth   int
		count   int
	)
	for _, char := range values {
		if quote != 0 {
			if quote == '\'' && char == '\\' && !escaped {
				escaped = true
				continue
			}
			if char == quote && !escaped {
				quote = 0
			}
			escaped = false
			continue
		}
		switch char {
		case '\'', '"', 96:
			quote = char
		case '(':
			if depth == 0 {
				count++
			}
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return count
}

func isAddonSQLIdentifierRune(char rune) bool {
	return char == '_' || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
}

func splitAddonSQL(content string) []string {
	statements, _ := splitAddonSQLContext(context.Background(), content)
	return statements
}

func splitAddonSQLContext(ctx context.Context, content string) ([]string, error) {
	statements := make([]string, 0)
	var builder strings.Builder
	var quote rune
	var escaped bool
	for index, char := range content {
		if index%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if quote != 0 {
			builder.WriteRune(char)
			if quote == '\'' && char == '\\' && !escaped {
				escaped = true
				continue
			}
			if char == quote && !escaped {
				quote = 0
			}
			escaped = false
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
			builder.WriteRune(char)
		case ';':
			if statement := strings.TrimSpace(builder.String()); statement != "" {
				statements = append(statements, statement)
			}
			builder.Reset()
		default:
			builder.WriteRune(char)
		}
	}
	if statement := strings.TrimSpace(builder.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}

// dropAddonDataTables removes dedicated tables and only this addon's rows from
// shared tables. It never drops a shared dictionary, task, menu or setting table.
func dropAddonDataTables(ctx context.Context, addonName string) error {
	tables, err := addonExistingTables(ctx, addonName)
	if err != nil {
		return err
	}
	for _, table := range tables {
		if _, err = g.DB().Exec(ctx, "DROP TABLE IF EXISTS "+g.DB().GetCore().QuoteWord(table)); err != nil {
			return err
		}
	}
	if err = deleteAddonSharedData(ctx, addonName); err != nil {
		return err
	}
	_, err = g.DB().Model(model.TableNameBaseSysInit).Ctx(ctx).Where("module", addonName).Delete()
	return err
}

// recreateAddonDataTables creates the registered addon tables again and uses
// the normal initjson mechanism to import that addon's initial records.
func recreateAddonDataTables(ctx context.Context, addonName string) error {
	models := addonModels(addonName)
	for _, item := range models {
		exists, err := g.DB(item.GroupName()).GetCore().HasTable(item.TableName())
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err = dzhcore.CreateTable(item); err != nil {
			return err
		}
	}
	_, err := g.DB().Model(model.TableNameBaseSysInit).Ctx(ctx).Where("module", addonName).Delete()
	if err != nil {
		return err
	}
	for _, item := range addonInitializationModels(addonName, models) {
		if hasAddonInitData(addonName, item.TableName()) {
			if err = dzhcore.FillInitData(ctx, addonName, item); err != nil {
				return err
			}
		}
		if err = dzhcore.RecordInitTable(ctx, addonName, item.GroupName(), item.TableName()); err != nil {
			return err
		}
	}
	return assignAddonOwnershipToInitRows(ctx, addonName)
}

func addonInitializationModels(addonName string, models []dzhcore.IModel) []dzhcore.IModel {
	result := make([]dzhcore.IModel, 0, len(models)+4)
	seen := make(map[string]struct{})
	for _, item := range models {
		result = append(result, item)
		seen[item.TableName()] = struct{}{}
	}
	for _, definition := range addonSharedBackupTables(addonName) {
		if definition.Name == model.TableNameBaseSysMenu || len(gres.GetContent("addons/"+addonName+"/resource/initjson/"+definition.Name+".json")) == 0 {
			continue
		}
		if _, ok := seen[definition.Name]; ok {
			continue
		}
		for _, item := range dzhcore.RegisteredModels() {
			if item.TableName() == definition.Name {
				result = append(result, item)
				seen[definition.Name] = struct{}{}
				break
			}
		}
	}
	return result
}

func deleteAddonSharedData(ctx context.Context, addonName string) error {
	for _, definition := range addonSharedBackupTables(addonName) {
		hasTable, err := g.DB().GetCore().HasTable(definition.Name)
		if err != nil {
			return err
		}
		if !hasTable {
			// 兼容旧版本未创建过的共享表；没有表就没有需要清理的数据。
			continue
		}
		query := g.DB().Model(definition.Name).Ctx(ctx).Unscoped()
		// 共享表在不同版本的系统中字段并不完全一致。例如旧版
		// addons_task_info 可能没有 addonsName。卸载外置插件时，
		// 不能因为这个可选清理字段不存在而阻断业务表删除。
		fields, fieldErr := g.DB().GetCore().TableFields(ctx, definition.Name)
		if fieldErr != nil {
			return fieldErr
		}
		if hasTableField(fields, definition.OwnershipKey) {
			if _, err := query.Clone().Where(definition.OwnershipKey, definition.OwnershipName).Delete(); err != nil {
				return err
			}
		}
		if len(definition.InitIDs) > 0 {
			if _, err := query.Clone().Where("id IN (?)", definition.InitIDs).Delete(); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasTableField(fields map[string]*gdb.TableField, name string) bool {
	for fieldName := range fields {
		if strings.EqualFold(fieldName, name) {
			return true
		}
	}
	return false
}

// assignAddonOwnershipToInitRows upgrades legacy initjson records that were
// created before addonsName existed (CRM dictionaries and scheduled tasks).
func assignAddonOwnershipToInitRows(ctx context.Context, addonName string) error {
	for _, definition := range addonSharedBackupTables(addonName) {
		if definition.OwnershipKey != "addonsName" || len(definition.InitIDs) == 0 {
			continue
		}
		if _, err := g.DB().Model(definition.Name).Ctx(ctx).
			Where("id IN (?)", definition.InitIDs).
			Data(definition.OwnershipKey, definition.OwnershipName).
			Update(); err != nil {
			return err
		}
	}
	return nil
}
