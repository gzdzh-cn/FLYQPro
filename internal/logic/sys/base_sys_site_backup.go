package sys

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	v1 "dzhgo/internal/api/admin_v1"
	"dzhgo/internal/dao"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gzdzh-cn/dzhcore"
)

const siteBackupRootDirectory = "site-backups"

var (
	siteBackupNamePattern      = regexp.MustCompile(`^site_[0-9]{8}_[0-9]{6}_[0-9]{3}_[0-9]+$`)
	siteBackupTableFilePattern = regexp.MustCompile(`^[0-9]{4}_[0-9a-f]+_[0-9]+\.sql$`)
)

const siteBackupMenuID = "2048794052043739138"

var siteBackupMenuPermissions = []struct {
	ID    string
	Name  string
	Perms string
}{
	{ID: "2048794052043739139", Name: "查询数据库表", Perms: "base:sys:addons:siteBackupTables"},
	{ID: "2048794052043739140", Name: "备份数据库表", Perms: "base:sys:addons:siteBackup"},
	{ID: "2048794052043739141", Name: "备份列表", Perms: "base:sys:addons:siteBackupList,base:sys:addons:siteBackupDetail,base:sys:addons:siteBackupPreview,base:sys:addons:siteBackupDownload"},
	{ID: "2048794052043739142", Name: "删除备份", Perms: "base:sys:addons:siteBackupDelete"},
	{ID: "2048794052043739143", Name: "恢复备份", Perms: "base:sys:addons:siteRestore,base:sys:addons:taskSse,base:sys:addons:taskCancel"},
}

func (s *sBaseSysAddonsService) EnsureSiteBackupMenu(ctx context.Context) error {
	menu, err := dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("id", siteBackupMenuID).One()
	if err != nil {
		return err
	}
	data := g.Map{
		"id":         siteBackupMenuID,
		"parentId":   "1152921504606847010",
		"name":       "备份全站",
		"router":     "/base/site-backup",
		"type":       1,
		"icon":       "icon-cloud-upload",
		"orderNum":   2,
		"viewPath":   "modules/base/views/site-backup.vue",
		"keepAlive":  1,
		"isShow":     1,
		"isInstall":  1,
		"menuType":   "base",
		"addonsName": "base",
		"deleted_at": nil,
	}
	if menu.IsEmpty() {
		if _, err = dao.BaseSysMenu.Ctx(ctx).Data(data).Insert(); err != nil {
			return err
		}
	} else {
		delete(data, "id")
		if _, err = dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("id", siteBackupMenuID).Data(data).Update(); err != nil {
			return err
		}
	}
	for _, permission := range siteBackupMenuPermissions {
		permissionData := g.Map{
			"id":         permission.ID,
			"parentId":   siteBackupMenuID,
			"name":       permission.Name,
			"perms":      permission.Perms,
			"type":       2,
			"keepAlive":  1,
			"isShow":     1,
			"isInstall":  1,
			"menuType":   "base",
			"addonsName": "base",
			"deleted_at": nil,
		}
		permissionRow, queryErr := dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("id", permission.ID).One()
		if queryErr != nil {
			return queryErr
		}
		if permissionRow.IsEmpty() {
			if _, queryErr = dao.BaseSysMenu.Ctx(ctx).Data(permissionData).Insert(); queryErr != nil {
				return queryErr
			}
			continue
		}
		delete(permissionData, "id")
		if _, queryErr = dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("id", permission.ID).Data(permissionData).Update(); queryErr != nil {
			return queryErr
		}
	}
	return nil
}

func (s *sBaseSysAddonsService) SiteBackupTables(ctx context.Context, _ *v1.SiteBackupTablesReq) (data interface{}, err error) {
	tables, err := siteDatabaseTables(ctx)
	if err != nil {
		return nil, err
	}
	sizes, err := siteDatabaseTableSizes(ctx)
	if err != nil {
		return nil, err
	}
	result := make(g.List, 0, len(tables))
	for index, table := range tables {
		result = append(result, g.Map{
			"tableName": table,
			"orderNum":  index + 1,
			"size":      sizes[strings.ToLower(table)],
		})
	}
	return result, nil
}

func (s *sBaseSysAddonsService) SiteBackup(ctx context.Context, req *v1.SiteBackupReq) (data interface{}, err error) {
	tables, err := resolveSiteTables(ctx, req.TableNames)
	if err != nil {
		return nil, err
	}
	backupType := normalizeAddonBackupType(req.BackupType)
	task := newAddonTask("backup")
	task.setScope("site")
	go runSiteBackupTask(task, tables, backupType, nil)
	return g.Map{"taskId": task.ID}, nil
}

func (s *sBaseSysAddonsService) SiteBackupList(context.Context, *v1.SiteBackupListReq) (data interface{}, err error) {
	return listSiteBackups()
}

func (s *sBaseSysAddonsService) SiteBackupDetail(_ context.Context, req *v1.SiteBackupDetailReq) (data interface{}, err error) {
	return siteBackupDetail(req.BackupName)
}

func (s *sBaseSysAddonsService) SiteBackupPreview(_ context.Context, req *v1.SiteBackupPreviewReq) (data interface{}, err error) {
	file, err := findSiteBackupFile(req.BackupName, req.FileName)
	if err != nil {
		return nil, err
	}
	return previewBackupFile(file, req.Offset, req.Limit)
}

func (s *sBaseSysAddonsService) SiteBackupDownload(_ context.Context, req *v1.SiteBackupDownloadReq) (path string, name string, err error) {
	file, err := findSiteBackupFile(req.BackupName, req.FileName)
	if err != nil {
		return "", "", err
	}
	return file.Path, file.FileName, nil
}

func (s *sBaseSysAddonsService) SiteBackupDelete(ctx context.Context, req *v1.SiteBackupDeleteReq) (data interface{}, err error) {
	cleanNames, err := cleanSiteBackupNames(req.BackupNames)
	if err != nil {
		return nil, err
	}
	release, err := acquireAddonTaskLock(ctx, "__site_backup__")
	if err != nil {
		return nil, err
	}
	defer release()
	for _, backupName := range cleanNames {
		if _, statErr := os.Stat(siteBackupBatchPath(backupName)); statErr != nil {
			if os.IsNotExist(statErr) {
				return nil, gerror.Newf("全站备份批次不存在：%s", backupName)
			}
			return nil, statErr
		}
	}
	for _, backupName := range cleanNames {
		if removeErr := os.RemoveAll(siteBackupBatchPath(backupName)); removeErr != nil {
			return nil, removeErr
		}
	}
	return g.Map{"deleted": len(cleanNames)}, nil
}

func (s *sBaseSysAddonsService) SiteRestore(ctx context.Context, req *v1.SiteRestoreReq) (data interface{}, err error) {
	if !isSafeSiteBackupName(req.BackupName) {
		return nil, gerror.New("全站备份批次无效")
	}
	if _, statErr := os.Stat(siteBackupBatchPath(req.BackupName)); statErr != nil {
		return nil, gerror.New("全站备份批次不存在")
	}
	files := append([]string{}, req.FileNames...)
	task := newAddonTask("restore")
	task.setScope("site")
	go runSiteRestoreTask(task, req.BackupName, files)
	return g.Map{"taskId": task.ID}, nil
}

func siteDatabaseTables(ctx context.Context) ([]string, error) {
	tables, err := g.DB().Tables(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(tables, func(i, j int) bool {
		return strings.ToLower(tables[i]) < strings.ToLower(tables[j])
	})
	return tables, nil
}

func siteDatabaseTableSizes(ctx context.Context) (map[string]int64, error) {
	sizes := make(map[string]int64)
	if !strings.EqualFold(g.DB().GetConfig().Type, "mysql") {
		return sizes, nil
	}
	result, err := g.DB().Query(ctx, `
		SELECT TABLE_NAME AS tableName,
		       COALESCE(DATA_LENGTH, 0) + COALESCE(INDEX_LENGTH, 0) AS tableSize
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?`, g.DB().GetConfig().Name)
	if err != nil {
		return nil, err
	}
	for _, row := range result {
		sizes[strings.ToLower(row["tableName"].String())] = row["tableSize"].Int64()
	}
	return sizes, nil
}

func resolveSiteTables(ctx context.Context, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, gerror.New("请选择数据库表")
	}
	available, err := siteDatabaseTables(ctx)
	if err != nil {
		return nil, err
	}
	lookup := make(map[string]string, len(available))
	for _, table := range available {
		lookup[strings.ToLower(table)] = table
	}
	seen := make(map[string]struct{}, len(requested))
	result := make([]string, 0, len(requested))
	for _, requestedTable := range requested {
		actual, ok := lookup[strings.ToLower(strings.TrimSpace(requestedTable))]
		if !ok {
			return nil, gerror.Newf("数据库表不存在：%s", requestedTable)
		}
		key := strings.ToLower(actual)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, actual)
	}
	return result, nil
}

func siteBackupRootPath() string {
	return filepath.Join(addonBackupRootDirectory, siteBackupRootDirectory)
}

func siteBackupBatchPath(backupName string) string {
	return filepath.Join(siteBackupRootPath(), backupName)
}

func siteBackupTablePath(backupName, fileName string) string {
	return filepath.Join(siteBackupBatchPath(backupName), fileName)
}

func isSafeSiteBackupName(backupName string) bool {
	return filepath.Base(backupName) == backupName && siteBackupNamePattern.MatchString(backupName)
}

func siteBackupFileName(index int, tableName string) string {
	encodedTable := hex.EncodeToString([]byte(tableName))
	return fmt.Sprintf("%04d_%s_%s.sql", index, encodedTable, dzhcore.NodeSnowflake.Generate().String())
}

func isSafeSiteBackupFileName(fileName string) bool {
	return filepath.Base(fileName) == fileName && siteBackupTableFilePattern.MatchString(fileName)
}

func siteBackupTableName(fileName string) string {
	name := strings.TrimSuffix(fileName, ".sql")
	parts := strings.Split(name, "_")
	if len(parts) < 3 {
		return name
	}
	encoded := strings.Join(parts[1:len(parts)-1], "_")
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return encoded
	}
	return string(decoded)
}

func runSiteBackupTask(task *addonTask, tables []string, backupType string, _ func(string, string, error)) {
	ctx, cancel := context.WithCancel(gctxForSiteTask())
	task.setCancel(cancel)
	defer cancel()
	task.update("running", 1, "全站备份执行中")
	release, err := acquireAddonTaskLock(ctx, "__site_backup__")
	if err != nil {
		task.update("cancelled", task.progressValue(), "全站备份任务已停止")
		return
	}
	err = backupSiteTables(ctx, tables, backupType, func(percent int, message string) {
		task.update("running", percent, message)
	})
	release()
	if ctx.Err() != nil {
		task.update("cancelled", task.progressValue(), "全站备份任务已停止")
		return
	}
	if err != nil {
		task.fail(err)
		return
	}
	task.update("completed", 100, "全站备份完成")
}

func runSiteRestoreTask(task *addonTask, backupName string, fileNames []string) {
	ctx, cancel := context.WithCancel(gctxForSiteTask())
	task.setCancel(cancel)
	defer cancel()
	releaseRestore, err := acquireAddonRestoreSlot(ctx)
	if err != nil {
		task.update("cancelled", task.progressValue(), "恢复任务已停止")
		return
	}
	defer releaseRestore()
	task.update("running", 1, "全站恢复执行中")
	release, err := acquireAddonTaskLock(ctx, "__site_backup__")
	if err != nil {
		task.update("cancelled", task.progressValue(), "全站恢复任务已停止")
		return
	}
	err = restoreSiteTables(ctx, task, backupName, fileNames, func(percent int, message string) {
		task.update("running", percent, message)
	})
	release()
	if ctx.Err() != nil {
		task.update("cancelled", task.progressValue(), "全站恢复任务已停止")
		return
	}
	if err != nil {
		task.fail(err)
		return
	}
	task.update("completed", 100, task.completionMessage())
}

func gctxForSiteTask() context.Context {
	return context.Background()
}

func backupSiteTables(ctx context.Context, tables []string, backupType string, progress func(int, string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(siteBackupRootPath(), 0755); err != nil {
		return err
	}
	backupName := fmt.Sprintf("site_%s_%s", time.Now().Format("20060102_150405_000"), dzhcore.NodeSnowflake.Generate().String())
	backupDir := siteBackupBatchPath(backupName)
	if err := os.Mkdir(backupDir, 0755); err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = os.RemoveAll(backupDir)
		}
	}()
	meta, err := jsonMarshalBackupMeta(backupType)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(backupDir, addonBackupMetaFile), meta, 0644); err != nil {
		return err
	}
	for index, tableName := range tables {
		if err = ctx.Err(); err != nil {
			return err
		}
		progress(index*100/len(tables), fmt.Sprintf("正在备份表：%s", tableName))
		fileName := siteBackupFileName(index+1, tableName)
		file, openErr := os.OpenFile(siteBackupTablePath(backupName, fileName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if openErr != nil {
			return openErr
		}
		writer := newBufferedWriter(file)
		_, _ = fmt.Fprintf(writer, "-- DZH3136 full-site backup table\n-- table: %s\n-- generated: %s\n\n", tableName, time.Now().Format(time.RFC3339))
		_, _ = writer.WriteString("SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS=0;\n\n")
		writeErr := writeAddonTableBackup(ctx, writer, addonBackupTable{Name: tableName}, backupType == addonBackupTypeData)
		_, _ = writer.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")
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
	completed = true
	progress(100, fmt.Sprintf("全站备份已保存（%s，%d 张表）", addonBackupTypeLabel(backupType), len(tables)))
	return nil
}

func restoreSiteTables(ctx context.Context, task *addonTask, backupName string, fileNames []string, progress func(int, string)) error {
	files, err := listSiteBackupFiles(backupName)
	if err != nil {
		return err
	}
	selectedNames := make(map[string]struct{}, len(fileNames))
	for _, fileName := range fileNames {
		selectedNames[fileName] = struct{}{}
	}
	selected := make([]addonBackupFileRef, 0, len(files))
	for _, file := range files {
		if len(selectedNames) == 0 {
			selected = append(selected, file)
		} else if _, ok := selectedNames[file.FileName]; ok {
			selected = append(selected, file)
		}
	}
	if len(selected) == 0 {
		return gerror.New("备份表文件不存在")
	}
	tables, err := siteDatabaseTables(ctx)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		allowed[strings.ToLower(table)] = struct{}{}
	}
	for _, file := range selected {
		// 允许恢复备份后已经被删除的表，SQL 中会负责重新创建它。
		allowed[strings.ToLower(file.TableName)] = struct{}{}
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
		if err = restoreSQLFile(ctx, file.Path, file.FileName, file.TableName, allowed, tableProgress); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			task.recordError("全站", file.TableName, file.FileName, err)
		}
	}
	return nil
}

func listSiteBackups() (g.List, error) {
	entries, err := os.ReadDir(siteBackupRootPath())
	if os.IsNotExist(err) {
		return g.List{}, nil
	}
	if err != nil {
		return nil, err
	}
	list := make(g.List, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !isSafeSiteBackupName(entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		files, filesErr := listSiteBackupFiles(entry.Name())
		if filesErr != nil {
			return nil, filesErr
		}
		size := int64(0)
		for _, file := range files {
			size += file.Size
		}
		list = append(list, g.Map{
			"backupName": entry.Name(),
			"backupType": readSiteBackupType(entry.Name()),
			"size":       size,
			"tableCount": len(files),
			"createTime": info.ModTime().Format(time.DateTime),
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return gconv.String(list[i]["backupName"]) > gconv.String(list[j]["backupName"])
	})
	return list, nil
}

func listSiteBackupFiles(backupName string) ([]addonBackupFileRef, error) {
	if !isSafeSiteBackupName(backupName) {
		return nil, gerror.New("全站备份批次无效")
	}
	entries, err := os.ReadDir(siteBackupBatchPath(backupName))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]addonBackupFileRef, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isSafeSiteBackupFileName(entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		files = append(files, addonBackupFileRef{
			BackupName: backupName,
			FileName:   entry.Name(),
			TableName:  siteBackupTableName(entry.Name()),
			Path:       siteBackupTablePath(backupName, entry.Name()),
			Size:       info.Size(),
			CreateTime: info.ModTime().Format(time.DateTime),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].FileName < files[j].FileName })
	return files, nil
}

func siteBackupDetail(backupName string) (g.Map, error) {
	files, err := listSiteBackupFiles(backupName)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, gerror.New("全站备份批次不存在或为空")
	}
	tables := make(g.List, 0, len(files))
	for _, file := range files {
		tables = append(tables, g.Map{
			"fileName":   file.FileName,
			"tableName":  file.TableName,
			"size":       file.Size,
			"createTime": file.CreateTime,
		})
	}
	return g.Map{
		"backupName": backupName,
		"backupType": readSiteBackupType(backupName),
		"tables":     tables,
	}, nil
}

func findSiteBackupFile(backupName, fileName string) (addonBackupFileRef, error) {
	if !isSafeSiteBackupFileName(fileName) {
		return addonBackupFileRef{}, gerror.New("全站备份表文件名无效")
	}
	files, err := listSiteBackupFiles(backupName)
	if err != nil {
		return addonBackupFileRef{}, err
	}
	for _, file := range files {
		if file.FileName == fileName {
			return file, nil
		}
	}
	return addonBackupFileRef{}, gerror.New("全站备份表文件不存在")
}

func readSiteBackupType(backupName string) string {
	content, err := os.ReadFile(filepath.Join(siteBackupBatchPath(backupName), addonBackupMetaFile))
	if err != nil {
		return addonBackupTypeData
	}
	var meta addonBackupMeta
	if json.Unmarshal(content, &meta) != nil {
		return addonBackupTypeData
	}
	return normalizeAddonBackupType(meta.BackupType)
}

func cleanSiteBackupNames(names []string) ([]string, error) {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		if !isSafeSiteBackupName(name) {
			return nil, gerror.New("全站备份批次无效")
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	if len(result) == 0 {
		return nil, gerror.New("请选择需要删除的备份批次")
	}
	return result, nil
}

func jsonMarshalBackupMeta(backupType string) ([]byte, error) {
	return json.Marshal(addonBackupMeta{BackupType: normalizeAddonBackupType(backupType)})
}

func newBufferedWriter(file *os.File) *bufio.Writer {
	return bufio.NewWriterSize(file, addonInsertBatchBytes)
}

func previewBackupFile(file addonBackupFileRef, offset, limit int64) (g.Map, error) {
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
