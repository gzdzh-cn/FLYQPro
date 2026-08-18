package sys

import (
	"context"
	"strings"
	"time"

	v1 "dzhgo/internal/api/admin_v1"
	"dzhgo/internal/dao"
	"dzhgo/internal/model"
	"dzhgo/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gmlock"
	"github.com/gogf/gf/v2/os/gres"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/gzdzh-cn/dzhcore"
)

func init() {
	service.RegisterBaseSysAddonsService(NewsBaseSysAddonsService())
}

type sBaseSysAddonsService struct {
	*dzhcore.Service
}

func NewsBaseSysAddonsService() *sBaseSysAddonsService {
	return &sBaseSysAddonsService{
		&dzhcore.Service{
			Dao:   &dao.BaseSysAddons,
			Model: model.NewBaseSysAddons(),
			PageQueryOp: &dzhcore.QueryOp{
				KeyWordField: []string{"name", "remark"},
				AddOrderby:   g.MapStrStr{"`base_sys_addons`.`orderNum`": "ASC", "`base_sys_addons`.`createTime`": "DESC"},
				Where: func(ctx context.Context) []g.Array {
					var (
						r    = g.RequestFromCtx(ctx)
						rmap = r.GetMap()
					)
					condition := []g.Array{
						{"base_sys_addons.deleted_at IS NULL"},
						{"typeId = ?", rmap["typeId"], rmap["typeId"]},
						{"c.IsInstall = ?", true, rmap["type"]},
					}

					return condition
				},
				Select: "base_sys_addons.*,b.name as typeName,c.isInstall,c.isShow",
				Join: []*dzhcore.JoinOp{
					{
						Model:     model.NewBaseSysAddonsTypes(),
						Alias:     "b",
						Type:      "LeftJoin",
						Condition: "`base_sys_addons`.`typeId` = `b`.`id`",
					},
					{
						Model:     model.NewBaseSysMenu(),
						Alias:     "c",
						Type:      "LeftJoin",
						Condition: "`base_sys_addons`.`menuId` = `c`.`id`",
					},
				},
			},
		},
	}
}

// EnsureBuiltinAddons initializes builtin plugins only when their table state
// says initialization has not completed. A later process start only validates
// the recorded tables; it never silently recreates a missing table.
func (s *sBaseSysAddonsService) EnsureBuiltinAddons(ctx context.Context) error {
	manifests, err := dzhcore.LoadConfiguredAddonManifests()
	if err != nil {
		return err
	}

	for _, manifest := range manifests {
		if !manifest.Builtin {
			continue
		}
		if manifest.Title == "" || manifest.MenuID == "" {
			return gerror.Newf("内置插件 %q 的 plugin.json 必须配置 title 和 menuId", manifest.Name)
		}

		if err := s.ensureBuiltinAddonRegistration(ctx, manifest); err != nil {
			return err
		}

		if err := s.ensureAddonTableState(ctx, manifest, true, true); err != nil {
			return err
		}

		menu, err := dao.BaseSysMenu.Ctx(ctx).
			Unscoped().
			Fields("id,isInstall").
			Where("id", manifest.MenuID).
			One()
		if err != nil {
			return err
		}
		if menu.IsEmpty() {
			if err = replaceAddonMenus(ctx, manifest.Name, manifest.MenuID, true, 0); err != nil {
				return gerror.Wrapf(err, "内置插件 %s 菜单恢复失败", manifest.Name)
			}
		} else if gconv.Int(menu["isInstall"]) != 1 {
			// 仅恢复已有菜单状态，不重建菜单树，避免覆盖管理员对菜单的
			// 展示调整，也避免将“上架”误当成“重新安装”。
			if _, err = dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("id", manifest.MenuID).
				Data(g.Map{"isInstall": 1}).Update(); err != nil {
				return err
			}
		}
		if _, err = s.LineUpdateStatus(ctx, &v1.LineUpdateStatusReq{Id: manifest.MenuID, Active: true}); err != nil {
			return err
		}
		g.Log().Infof(ctx, "内置插件已确保安装并上架: %s (%s)", manifest.Title, manifest.Name)
	}
	return nil
}

// ValidateInstalledAddons checks every installed addon before its code is
// started. It is intentionally read-only: a missing table is an installation
// failure, not a reason to rebuild production data at startup.
func (s *sBaseSysAddonsService) ValidateInstalledAddons(ctx context.Context) error {
	manifests, err := dzhcore.LoadConfiguredAddonManifests()
	if err != nil {
		return err
	}
	manifestByName := make(map[string]dzhcore.AddonManifest, len(manifests))
	for _, manifest := range manifests {
		manifestByName[manifest.Name] = manifest
	}
	installed, err := dao.BaseSysMenu.Ctx(ctx).Fields("addonsName").Where("isInstall", 1).Array()
	if err != nil {
		return err
	}
	for _, item := range installed {
		addonName := item.String()
		// base 是核心模块，资源位于 internal/resource，不属于 addons
		// 插件生命周期，因此没有也不需要 plugin.json 或插件表清单。
		if addonName == "" || addonName == "base" {
			continue
		}
		manifest, ok := manifestByName[addonName]
		if !ok {
			return gerror.Newf("已安装插件 %q 没有对应的 plugin.json", addonName)
		}
		if err := s.ensureAddonTableState(ctx, manifest, false, false); err != nil {
			return err
		}
	}
	return nil
}

// RepairAddonRegistrationDuplicates keeps one visible registration for each
// menu root and soft-deletes historical duplicate rows. Older uninstall code
// left a soft-deleted row and later versions could insert a second row, while
// some databases also contain duplicate active rows from concurrent requests.
// This repair is deliberately soft-delete-only so it is recoverable.
func (s *sBaseSysAddonsService) RepairAddonRegistrationDuplicates(ctx context.Context) error {
	rows, err := dao.BaseSysAddons.Ctx(ctx).
		Unscoped().
		Fields("id,menuId,addonsName,status,deleted_at").
		All()
	if err != nil {
		return err
	}

	type registration struct {
		id        string
		status    int
		deletedAt string
	}
	groups := make(map[string][]registration)
	for _, row := range rows {
		menuID := row["menuId"].String()
		addonName := row["addonsName"].String()
		key := ""
		if menuID != "" {
			key = "menu:" + menuID
		} else if addonName != "" {
			key = "name:" + addonName
		}
		if key == "" || row["id"].String() == "" {
			continue
		}
		groups[key] = append(groups[key], registration{
			id:        row["id"].String(),
			status:    gconv.Int(row["status"]),
			deletedAt: row["deleted_at"].String(),
		})
	}

	for key, candidates := range groups {
		if len(candidates) < 2 {
			continue
		}
		keep := candidates[0]
		for _, candidate := range candidates[1:] {
			if registrationPreferred(candidate, keep) {
				keep = candidate
			}
		}
		for _, candidate := range candidates {
			if candidate.id == keep.id {
				continue
			}
			if _, err = dao.BaseSysAddons.Ctx(ctx).Unscoped().Where("id", candidate.id).
				Data(g.Map{"status": 0, "deleted_at": time.Now()}).Update(); err != nil {
				return err
			}
		}
		g.Log().Warningf(ctx, "插件登记重复已整理: group=%s keepId=%s removed=%d", key, keep.id, len(candidates)-1)
	}
	return nil
}

func registrationPreferred(candidate, current struct {
	id        string
	status    int
	deletedAt string
}) bool {
	if (candidate.deletedAt == "") != (current.deletedAt == "") {
		return candidate.deletedAt == ""
	}
	if candidate.status != current.status {
		return candidate.status > current.status
	}
	// Snowflake IDs are monotonic in this project; keep the newest row when
	// both registrations have the same visibility and install state.
	return candidate.id > current.id
}

// ensureAddonTableState enforces the base_sys_init contract. When create is
// true it is used only by builtin first-start initialization; when false it
// only checks physical tables and supplements legacy init records.
func (s *sBaseSysAddonsService) ensureAddonTableState(ctx context.Context, manifest dzhcore.AddonManifest, create, seedNewTables bool) error {
	lockKey := "addon:lifecycle:" + manifest.Name
	if !gmlock.TryLock(lockKey) {
		return gerror.Newf("插件 %s 正在执行其他生命周期操作", manifest.Name)
	}
	defer gmlock.Unlock(lockKey)

	models := addonInitializationModels(manifest.Name, addonModels(manifest.Name))
	recorded, err := dzhcore.InitTables(ctx, manifest.Name, "default")
	if err != nil {
		return gerror.Wrapf(err, "读取插件 %s 初始化记录失败", manifest.Name)
	}
	recordedSet := make(map[string]struct{}, len(recorded))
	for _, tableName := range recorded {
		recordedSet[strings.ToLower(tableName)] = struct{}{}
	}
	if len(recordedSet) == 0 && !create {
		// 兼容旧版本已经建表但尚未写入 base_sys_init 的插件：只补登记，
		// 不执行初始化 JSON，避免把线上业务数据重复插入。
		for _, item := range addonModels(manifest.Name) {
			physical, checkErr := hasPhysicalTable(ctx, item.GroupName(), item.TableName())
			if checkErr != nil {
				return checkErr
			}
			if !physical {
				return gerror.Newf("插件 %s 已安装但表 %s 不存在，请执行修复或重新安装", manifest.Name, item.TableName())
			}
			if recordErr := dzhcore.RecordInitTable(ctx, manifest.Name, item.GroupName(), item.TableName()); recordErr != nil {
				return recordErr
			}
		}
		return nil
	}
	for _, item := range models {
		tableName := item.TableName()
		key := strings.ToLower(tableName)
		physical, hasTableErr := hasPhysicalTable(ctx, item.GroupName(), tableName)
		if hasTableErr != nil {
			return gerror.Wrapf(hasTableErr, "检查插件 %s 表 %s 失败", manifest.Name, tableName)
		}
		if _, initialized := recordedSet[key]; initialized {
			if !physical {
				return gerror.Newf("插件 %s 已记录初始化，但表 %s 不存在，请修复后再启动", manifest.Name, tableName)
			}
			continue
		}
		if !physical {
			if !create {
				return gerror.Newf("插件 %s 已安装但表 %s 不存在，请执行修复或重新安装", manifest.Name, tableName)
			}
			if err = dzhcore.CreateTable(item); err != nil {
				return gerror.Wrapf(err, "创建插件 %s 表 %s 失败", manifest.Name, tableName)
			}
			// 插件表由 GORM 动态创建；清理 GoFrame 的旧字段缓存，
			// 确保随后写入初始化数据时能读取到新表结构。
			if err = g.DB(item.GroupName()).GetCore().ClearTableFields(ctx, tableName); err != nil {
				return gerror.Wrapf(err, "刷新插件 %s 表 %s 字段缓存失败", manifest.Name, tableName)
			}
			physical = true
			if seedNewTables && hasAddonInitData(manifest.Name, tableName) {
				if err = dzhcore.FillInitData(ctx, manifest.Name, item); err != nil {
					return gerror.Wrapf(err, "初始化插件 %s 表 %s 失败", manifest.Name, tableName)
				}
			}
		}
		if physical {
			if err = dzhcore.RecordInitTable(ctx, manifest.Name, "default", tableName); err != nil {
				return gerror.Wrapf(err, "记录插件 %s 表 %s 初始化状态失败", manifest.Name, tableName)
			}
			recordedSet[key] = struct{}{}
		}
	}
	return nil
}

// hasPhysicalTable reads the database table list directly. The GoFrame table
// name cache may have been populated before a plugin table is created by GORM
// during the same startup, so GetCore().HasTable can return a stale result.
func hasPhysicalTable(ctx context.Context, group, tableName string) (bool, error) {
	tables, err := g.DB(group).Tables(ctx)
	if err != nil {
		return false, err
	}
	for _, table := range tables {
		if strings.EqualFold(table, tableName) {
			return true, nil
		}
	}
	return false, nil
}

func hasAddonInitData(addonName, tableName string) bool {
	path := "addons/" + addonName + "/resource/initjson/" + tableName + ".json"
	content, err := dzhcore.GetAddonResourceContent(path)
	return err == nil && len(content) > 0
}

func (s *sBaseSysAddonsService) ensureBuiltinAddonRegistration(ctx context.Context, manifest dzhcore.AddonManifest) error {
	addon, err := dao.BaseSysAddons.Ctx(ctx).
		Unscoped().
		Where("addonsName = ? OR menuId = ?", manifest.Name, manifest.MenuID).
		Order("id ASC").
		One()
	if err != nil {
		return err
	}

	if addon.IsEmpty() {
		typeID, err := dao.BaseSysAddonsTypes.Ctx(ctx).Fields("id").Order("orderNum ASC,id ASC").Value("id")
		if err != nil {
			return err
		}
		data := g.Map{
			"id":         dzhcore.CreateSnowflakeId(),
			"name":       manifest.Title,
			"addonsName": manifest.Name,
			"menuId":     manifest.MenuID,
			"typeId":     typeID.String(),
			"isCustom":   0,
			"status":     1,
			"orderNum":   99,
		}
		if _, err = dao.BaseSysAddons.Ctx(ctx).Data(data).Insert(); err != nil {
			return err
		}
		return nil
	}

	data := g.Map{
		"name":       manifest.Title,
		"addonsName": manifest.Name,
		"menuId":     manifest.MenuID,
		"isCustom":   0,
		"status":     1,
	}
	if addon["deleted_at"].String() != "" {
		data["deleted_at"] = nil
	}
	if _, err = dao.BaseSysAddons.Ctx(ctx).Unscoped().Where("id", addon["id"]).Data(data).Update(); err != nil {
		return err
	}
	return nil
}

// Available returns external plugins from plugin.json that have not been
// registered in base_sys_addons yet. The manifest is the source of truth for
// the plugin list. Menu resources are validated only when installation starts,
// so one incomplete optional plugin cannot make the whole dropdown empty.
func (s *sBaseSysAddonsService) Available(ctx context.Context, _ *v1.AvailableReq) (data interface{}, err error) {
	manifests, err := dzhcore.LoadConfiguredAddonManifests()
	if err != nil {
		return nil, err
	}

	existing, err := dao.BaseSysAddons.Ctx(ctx).
		Unscoped().
		Fields("menuId,addonsName,deleted_at").
		All()
	if err != nil {
		return nil, err
	}

	existingMenuIDs := make(map[string]struct{}, len(existing))
	existingAddonNames := make(map[string]struct{}, len(existing))
	for _, addon := range existing {
		// 历史版本可能留下软删除登记。它仍然代表该插件已经登记，
		// 不能再次通过“添加插件”插入第二条记录。
		if menuID := addon["menuId"].String(); menuID != "" {
			existingMenuIDs[menuID] = struct{}{}
		}
		if addonName := addon["addonsName"].String(); addonName != "" {
			existingAddonNames[addonName] = struct{}{}
		}
	}

	available := make([]g.Map, 0, len(manifests))
	availableNames := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		// 内置插件在首次启动时自动登记，不应出现在“添加插件”的下拉框。
		if manifest.Builtin || manifest.Name == "" || manifest.MenuID == "" {
			continue
		}
		if _, exists := existingMenuIDs[manifest.MenuID]; exists {
			continue
		}
		if _, exists := existingAddonNames[manifest.Name]; exists {
			continue
		}

		available = append(available, g.Map{
			"id":         manifest.MenuID,
			"name":       manifest.Title,
			"addonsName": manifest.Name,
			"builtin":    manifest.Builtin,
		})
		availableNames = append(availableNames, manifest.Name)
	}
	g.Log().Infof(ctx, "插件添加下拉: 清单=%d，已登记插件=%d，可添加外置插件=%v", len(manifests), len(existing), availableNames)

	return available, nil
}

// ModifyBefore keeps the English addon identifier when older records or the
// legacy edit form do not send addonsName.
func (s *sBaseSysAddonsService) ModifyBefore(ctx context.Context, method string, param g.MapStrAny) error {
	if method != "Add" && method != "Update" {
		return nil
	}
	if method == "Add" {
		param["isCustom"] = 1
		// 添加插件只登记插件信息，不代表已经安装或上架。
		param["status"] = 0
		for _, field := range []string{"menuId", "addonsName"} {
			value := gconv.String(param[field])
			if value == "" {
				continue
			}
			addon, err := dao.BaseSysAddons.Ctx(ctx).
				Unscoped().
				Fields("deleted_at").
				Where(field, value).
				One()
			if err != nil {
				return err
			}
			if !addon.IsEmpty() {
				return gerror.New("插件列表已存在该插件，不能重复添加")
			}
		}
	}
	if method == "Update" && gconv.String(param["id"]) != "" {
		if addon, err := dao.BaseSysAddons.Ctx(ctx).
			Fields("isCustom").
			Where("id", param["id"]).
			One(); err == nil && gconv.Int(addon["isCustom"]) == 1 {
			return gerror.New("自定义添加的插件不支持编辑")
		}
	}
	if gconv.String(param["addonsName"]) != "" {
		return nil
	}

	menuID := gconv.String(param["menuId"])
	if menuID == "" && method == "Update" {
		if addon, err := dao.BaseSysAddons.Ctx(ctx).
			Fields("menuId,addonsName").
			Where("id", param["id"]).
			One(); err == nil {
			if addon["addonsName"].String() != "" {
				param["addonsName"] = addon["addonsName"].String()
				return nil
			}
			menuID = addon["menuId"].String()
		}
	}
	if menuID == "" {
		return nil
	}

	menu, err := dao.BaseSysMenu.Ctx(ctx).
		Fields("name,addonsName").
		Where("id", menuID).
		One()
	if err != nil || menu.IsEmpty() {
		return nil
	}
	addonName := menu["addonsName"].String()
	if addonName == "" {
		addonName = inferAddonName("", menu["name"].String())
	}
	if addonName != "" {
		param["addonsName"] = addonName
	}
	return nil
}

// ModifyAfter resets the packed menu tree when an addon is newly registered.
// Registration and installation are separate operations: the addon must stay
// unavailable until the administrator explicitly clicks "安装".
func (s *sBaseSysAddonsService) ModifyAfter(ctx context.Context, method string, param g.MapStrAny) error {
	if method != "Add" {
		return nil
	}

	menuID := gconv.String(param["menuId"])
	if menuID == "" {
		return nil
	}
	ids, err := collectAddonMenuIDs(dao.BaseSysMenu.Ctx(ctx), menuID)
	if err != nil || len(ids) == 0 {
		return err
	}
	_, err = dao.BaseSysMenu.Ctx(ctx).
		Unscoped().
		Where("id IN (?)", ids).
		Data(g.Map{"isInstall": 0, "isShow": 0}).
		Update()
	return err
}

// 安装卸载插件
func (s *sBaseSysAddonsService) InstallUpdateStatus(ctx context.Context, req *v1.InstallUpdateStatusReq) (data interface{}, err error) {
	addon, err := dao.BaseSysAddons.Ctx(ctx).
		Unscoped().
		Where("menuId", req.Id).
		Fields("id,name,addonsName,status").
		One()
	if err != nil || addon.IsEmpty() {
		if err != nil {
			g.Log().Error(ctx, err.Error())
		}
		err = gerror.New("插件不存在")
		return
	}
	if !req.Active && gconv.Int(addon["status"]) == 1 {
		err = gerror.New("请先下架插件后再卸载")
		return
	}

	addonName := addon["addonsName"].String()
	if addonName == "" {
		addonName = inferAddonName(addon["name"].String(), "")
	}
	if addonName == "" {
		if menu, menuErr := dao.BaseSysMenu.Ctx(ctx).
			Fields("name,addonsName").
			Where("id", req.Id).
			One(); menuErr == nil {
			addonName = menu["addonsName"].String()
			if addonName == "" {
				addonName = inferAddonName("", menu["name"].String())
			}
		}
	}
	// Older addon records may have been created by a resource whose
	// addonsName field was missing. The packed menu resource is still the
	// authoritative source for rebuilding the menu, so recover the addon
	// directory from the root menu id before failing the install.
	if addonName == "" {
		addonName = resolvePackedAddonName(req.Id)
		if addonName != "" {
			g.Log().Warningf(ctx, "插件英文名称为空，已从菜单资源恢复: menuId=%s addonsName=%s", req.Id, addonName)
		}
	}
	if addonName == "" {
		err = gerror.New("插件英文名称未配置，无法重建菜单")
		return
	}
	manifest, found, manifestErr := dzhcore.FindAddonManifest(addonName)
	if manifestErr != nil {
		err = manifestErr
		return
	}
	if !found {
		err = gerror.Newf("插件 %s 未配置 plugin.json，不能执行生命周期操作", addonName)
		return
	}
	if !req.Active && found && manifest.Builtin {
		err = gerror.New("内置插件不允许卸载，请使用下架操作")
		return
	}
	lockKey := "addon:lifecycle:" + addonName
	if !gmlock.TryLock(lockKey) {
		err = gerror.Newf("插件 %s 正在执行其他生命周期操作", addonName)
		return
	}
	defer gmlock.Unlock(lockKey)
	if req.Active && manifest.Builtin {
		// 内置插件的安装动作只负责恢复菜单，不重复创建表或导入种子数据。
		menu, menuErr := dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("id", req.Id).One()
		if menuErr != nil {
			return nil, menuErr
		}
		if menu.IsEmpty() {
			if err = replaceAddonMenus(ctx, addonName, req.Id, true, gconv.Int(addon["status"])); err != nil {
				return nil, gerror.Wrap(err, "内置插件菜单恢复失败")
			}
		} else if _, err = dao.BaseSysMenu.Ctx(ctx).Unscoped().Where("addonsName", addonName).
			Data(g.Map{"isInstall": 1}).Update(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if req.Active {
		if err = recreateAddonDataTables(ctx, addonName); err != nil {
			g.Log().Error(ctx, err.Error())
			err = gerror.New("插件数据初始化失败")
			return
		}
	} else {
		if err = dropAddonDataTables(ctx, addonName); err != nil {
			g.Log().Error(ctx, err.Error())
			err = gerror.New("插件数据表删除失败")
			return
		}
	}
	// 安装和上架是两个独立动作。安装完成后插件必须保持下架状态，
	// 只有管理员明确点击“上架”时，LineUpdateStatus 才能显示菜单。
	if err = replaceAddonMenus(ctx, addonName, req.Id, req.Active, 0); err != nil {
		g.Log().Error(ctx, err.Error())
		if req.Active {
			// DDL 不能完整回滚。菜单重建失败时清理本次未完成的安装，
			// 保证下一次安装不会把半成品误判为已安装。
			if cleanupErr := dropAddonDataTables(ctx, addonName); cleanupErr != nil {
				g.Log().Errorf(ctx, "清理插件 %s 半安装数据失败: %v", addonName, cleanupErr)
			}
		}
		err = gerror.New("菜单处理失败")
		return
	}
	if req.Active {
		// 兼容旧数据：历史上部分插件记录可能已经是 status=1，
		// 安装时统一纠正为未上架，避免数据库状态和菜单状态不一致。
		if _, err = dao.BaseSysAddons.Ctx(ctx).
			Where("id", addon["id"]).
			Data(g.Map{"status": 0, "deleted_at": nil}).
			Update(); err != nil {
			g.Log().Error(ctx, err.Error())
			err = gerror.New("插件安装状态保存失败")
			return
		}
	}

	if addon["addonsName"].String() != addonName {
		_, err = dao.BaseSysAddons.Ctx(ctx).
			Where("id", addon["id"]).
			Data(g.Map{"addonsName": addonName}).
			Update()
	}
	if err != nil {
		g.Log().Error(ctx, err.Error())
		err = gerror.New("操作失败")
		return
	}
	if !req.Active {
		// 卸载只清理业务表和菜单，插件登记记录保留在插件列表中，
		// 这样管理员可以看到“未安装”状态并再次安装。不要软删除
		// base_sys_addons，否则通用列表查询会把插件隐藏掉。
		if _, err = dao.BaseSysAddons.Ctx(ctx).Unscoped().Where("id", addon["id"]).
			Data(g.Map{"status": 0, "deleted_at": nil, "addonsName": addonName}).Update(); err != nil {
			g.Log().Error(ctx, err.Error())
			err = gerror.New("插件卸载状态保存失败")
			return
		}
	}
	if err = deduplicateAddonRegistrations(ctx, addonName, req.Id, addon["id"].String()); err != nil {
		g.Log().Error(ctx, err.Error())
		return nil, gerror.New("插件登记去重失败")
	}
	return
}

// deduplicateAddonRegistrations keeps the registration being operated on and
// hides older duplicate rows by soft deletion. This is needed for databases
// created before menuId/addonsName was treated as an idempotent pair.
func deduplicateAddonRegistrations(ctx context.Context, addonName, menuID, keepID string) error {
	if addonName == "" && menuID == "" {
		return nil
	}
	var rows, byName gdb.Result
	var err error
	if menuID != "" {
		rows, err = dao.BaseSysAddons.Ctx(ctx).Unscoped().Fields("id").Where("menuId", menuID).All()
		if err != nil {
			return err
		}
	}
	if addonName != "" {
		byName, err = dao.BaseSysAddons.Ctx(ctx).Unscoped().Fields("id").Where("addonsName", addonName).All()
		if err != nil {
			return err
		}
	}
	ids := make(map[string]struct{}, len(rows)+len(byName))
	for _, row := range append(rows, byName...) {
		id := row["id"].String()
		if id != "" && id != keepID {
			ids[id] = struct{}{}
		}
	}
	for id := range ids {
		if _, err = dao.BaseSysAddons.Ctx(ctx).Unscoped().Where("id", id).
			Data(g.Map{"status": 0, "deleted_at": time.Now()}).Update(); err != nil {
			return err
		}
	}
	return nil
}

// resolvePackedAddonName finds the addon directory that owns a menu root.
// This keeps installation compatible with old base_sys_addons/base_sys_menu
// rows where addonsName was not persisted, without hard-coding every plugin.
func resolvePackedAddonName(rootID string) string {
	if rootID == "" {
		return ""
	}

	for _, file := range gres.ScanDirFile("addons", "base_sys_menu.json", true) {
		jsonData, err := gjson.LoadContent(file.Content())
		if err != nil {
			continue
		}

		var menus []g.Map
		if err = jsonData.Scan(&menus); err != nil {
			continue
		}
		for _, menu := range menus {
			if gconv.String(menu["id"]) != rootID ||
				gconv.Int(menu["type"]) != 0 ||
				gconv.String(menu["parentId"]) != "" {
				continue
			}

			if addonName := gconv.String(menu["addonsName"]); addonName != "" {
				return addonName
			}
			parts := strings.Split(file.Name(), "/")
			if len(parts) > 1 && parts[0] == "addons" {
				return parts[1]
			}
		}
	}
	return ""
}

// replaceAddonMenus deletes an addon's current menu tree and, when installing,
// rebuilds it from the packed addon initialization data.
func replaceAddonMenus(ctx context.Context, addonName, rootID string, install bool, isShow int) error {
	return dao.BaseSysMenu.Ctx(ctx).Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		menuModel := tx.Model(model.TableNameBaseSysMenu).Ctx(ctx)
		ids, err := collectAddonMenuIDs(menuModel, rootID)
		if err != nil {
			return err
		}
		// Restored menu backups may contain addon menus whose parent is a
		// shared/base menu, so they are not reachable from rootID. Include all
		// rows owned by this addon before rebuilding the packed menu tree; this
		// prevents duplicate primary-key errors during reinstall.
		addonRows, addonRowsErr := menuModel.Clone().Unscoped().
			Fields("id").
			Where("addonsName", addonName).
			All()
		if addonRowsErr != nil {
			return addonRowsErr
		}
		seenIDs := make(map[string]struct{}, len(ids)+len(addonRows))
		for _, id := range ids {
			seenIDs[id] = struct{}{}
		}
		for _, row := range addonRows {
			id := row["id"].String()
			if id != "" {
				if _, exists := seenIDs[id]; !exists {
					ids = append(ids, id)
					seenIDs[id] = struct{}{}
				}
			}
		}
		if len(ids) > 0 {
			if _, err = tx.Model(model.TableNameBaseSysRoleMenu).
				Ctx(ctx).
				Unscoped().
				Where("menuId IN (?)", ids).
				Delete(); err != nil {
				return err
			}
			if _, err = menuModel.Clone().Unscoped().Where("id IN (?)", ids).Delete(); err != nil {
				return err
			}
		}
		if !install {
			return nil
		}

		path := "addons/" + addonName + "/resource/initjson/base_sys_menu.json"
		content, err := dzhcore.GetAddonResourceContent(path)
		if err != nil {
			return gerror.Newf("插件菜单资源不存在: %s", path)
		}
		jsonData, loadErr := gjson.LoadContent(content)
		if loadErr != nil {
			return loadErr
		}
		if jsonData.Var().IsEmpty() {
			return gerror.Newf("插件菜单资源不存在: %s", path)
		}
		if _, err = menuModel.Data(jsonData).Insert(); err != nil {
			return err
		}

		ids, err = collectAddonMenuIDs(menuModel, rootID)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return gerror.New("插件菜单资源中未找到根菜单")
		}
		if _, err = menuModel.
			Where("id IN (?)", ids).
			Data(g.Map{"isInstall": 1, "addonsName": addonName}).
			Update(); err != nil {
			return err
		}
		_, err = menuModel.
			Where("id", rootID).
			Data(g.Map{"isShow": isShow, "isInstall": 1, "addonsName": addonName}).
			Update()
		return err
	})
}

func collectAddonMenuIDs(menuModel *gdb.Model, rootID string) ([]string, error) {
	rootValue, err := menuModel.Clone().Unscoped().Where("id", rootID).Value("id")
	if err != nil {
		return nil, err
	}
	if rootValue.IsEmpty() {
		return nil, nil
	}

	rows, err := menuModel.Clone().Unscoped().Fields("id,parentId").All()
	if err != nil {
		return nil, err
	}
	children := make(map[string][]string)
	for _, row := range rows {
		children[row["parentId"].String()] = append(children[row["parentId"].String()], row["id"].String())
	}
	ids := make([]string, 0)
	queue := []string{rootID}
	seen := map[string]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		queue = append(queue, children[id]...)
	}
	return ids, nil
}

// 上下架插件
func (s *sBaseSysAddonsService) LineUpdateStatus(ctx context.Context, req *v1.LineUpdateStatusReq) (data interface{}, err error) {
	addon, err := dao.BaseSysAddons.Ctx(ctx).
		Where("menuId", req.Id).
		Fields("id,addonsName").
		One()
	if err != nil {
		g.Log().Error(ctx, err.Error())
		err = gerror.New("操作失败")
		return
	}
	menu, menuErr := dao.BaseSysMenu.Ctx(ctx).
		Fields("isInstall,addonsName").
		Where("id", req.Id).
		One()
	if menuErr != nil {
		g.Log().Error(ctx, menuErr.Error())
		err = gerror.New("插件菜单不存在，请先安装插件")
		return
	}
	if menu.IsEmpty() || gconv.Int(menu["isInstall"]) != 1 {
		err = gerror.New("请先安装插件后再上架")
		return
	}

	addonName := ""
	if !addon.IsEmpty() {
		addonName = addon["addonsName"].String()
		addonUpdate := g.Map{"status": gconv.Int(req.Active)}
		if addonName == "" {
			menu, menuErr := dao.BaseSysMenu.Ctx(ctx).
				Fields("name,addonsName").
				Where("id", req.Id).
				One()
			if menuErr == nil {
				addonName = menu["addonsName"].String()
				if addonName == "" {
					addonName = inferAddonName("", menu["name"].String())
				}
				if addonName != "" {
					addonUpdate["addonsName"] = addonName
				}
			}
		}
		if _, err = dao.BaseSysAddons.Ctx(ctx).
			Where("id", addon["id"]).
			Data(addonUpdate).
			Update(); err != nil {
			g.Log().Error(ctx, err.Error())
			err = gerror.New("操作失败")
			return
		}
	}

	menuUpdate := g.Map{"isShow": req.Active}
	if addonName != "" {
		menuUpdate["addonsName"] = addonName
	}
	_, err = dao.BaseSysMenu.Ctx(ctx).
		Where("id", req.Id).
		Data(menuUpdate).
		Update()
	if err != nil {
		g.Log().Error(ctx, err.Error())
		err = gerror.New("操作失败")
		return
	}

	return
}
