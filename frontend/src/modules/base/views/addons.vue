<template>
	<cl-crud ref="Crud">
		<div
			style="
				padding: 10px 10px 0px 20px;
				display: flex;
				flex-wrap: wrap;
				row-gap: 10px;
				column-gap: 10px;
			"
		>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn />
			<el-dropdown
				:disabled="!canBackupSelected"
				@command="backupSelected"
			>
				<el-button type="primary" :disabled="!canBackupSelected">
					备份
					<el-icon class="el-icon--right"><ArrowDown /></el-icon>
				</el-button>
				<template #dropdown>
					<el-dropdown-menu>
						<el-dropdown-item command="data">备份数据</el-dropdown-item>
						<el-dropdown-item command="structure">备份结构</el-dropdown-item>
					</el-dropdown-menu>
				</template>
			</el-dropdown>
			<el-button type="warning" :disabled="!canRestoreLatest" @click="restoreLatest"
				>恢复最近备份</el-button
			>
			<!-- 关键字搜索 -->
			<cl-search-key style="margin-left: 10px" />
			<cl-flex1 />

			<el-button type="success" @click="typeOpen">插件类型</el-button>
		</div>

		<div class="divider"></div>

		<cl-row>
			<div class="fileter">
				<div class="filter-title">
					<span>分类：</span>
				</div>
				<div class="filter-name">
					<el-button
						@click="getList(null, null)"
						class="f-btn"
						:class="{ active: !activeIndex }"
						>全部</el-button
					>
					<el-button
						class="f-btn"
						:class="{ active: activeIndex == 'hasInstall' }"
						@click="getList(null, 'hasInstall')"
						>已安装</el-button
					>
					<el-button
						class="f-btn"
						:class="{ active: activeIndex == item.id }"
						v-for="(item, index) in typeList"
						:key="index"
						@click="getList(item.id)"
					>
						{{ item.name }}
					</el-button>
				</div>
			</div>
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="false">
				<template #slot-op="{ scope }">
					<div class="table-op">
						<el-button
							type="warning"
							text
							bg
							v-if="
								!scope.row.isInstall &&
								service.base.sys.addons._permission.installUpdateStatus
							"
							@click="installUpdateStatus(scope.row, true)"
							>安装</el-button
						>
						<el-button
							type="danger"
							text
							bg
							:disabled="Number(scope.row.isShow) === 1 || scope.row.isShow === true"
							:title="
								Number(scope.row.isShow) === 1 || scope.row.isShow === true
									? '请先下架插件后再卸载'
									: '卸载插件'
							"
							v-if="
								scope.row.isInstall &&
								service.base.sys.addons._permission.installUpdateStatus
							"
							@click="installUpdateStatus(scope.row, false)"
							>卸载</el-button
						>
						<el-button
							type="info"
							text
							bg
							@click="lineUpdateStatus(scope.row, true)"
							v-if="
								scope.row.isInstall &&
								!scope.row.isShow &&
								service.base.sys.addons._permission.lineUpdateStatus
							"
							>上架</el-button
						>
						<el-button
							type="info"
							text
							bg
							@click="lineUpdateStatus(scope.row, false)"
							v-if="
								scope.row.isInstall &&
								scope.row.isShow &&
								service.base.sys.addons._permission.lineUpdateStatus
							"
							>下架</el-button
						>
						<el-button type="primary" text bg @click="openBackupList(scope.row)"
							>备份列表</el-button
						>
					</div>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert" />

		<el-drawer v-model="typesShow" title="插件类型" direction="rtl" size="40%" append-to-body>
			<AddonsTypes />
		</el-drawer>

		<el-dialog v-model="backupListVisible" title="插件备份列表" width="80%">
			<el-tabs v-model="backupTypeTab" @tab-change="switchBackupTypeTab">
				<el-tab-pane label="数据备份" name="data" />
				<el-tab-pane label="结构备份" name="structure" />
			</el-tabs>
			<el-table
				:key="backupTypeTab"
				:data="visibleBackupFiles"
				max-height="420"
				@selection-change="selectBackupFiles"
				@row-click="openBackupDetail"
			>
				<el-table-column type="selection" width="48" />
				<el-table-column
					label="备份批次"
					prop="backupName"
					min-width="330"
					show-overflow-tooltip
				/>
				<el-table-column label="类型" width="100">
					<template #default="{ row }">{{ backupTypeLabel(row.backupType) }}</template>
				</el-table-column>
				<el-table-column label="表数量" prop="tableCount" width="90" />
				<el-table-column label="时间" prop="createTime" width="170" />
				<el-table-column label="大小" width="100">
					<template #default="{ row }">{{ formatSize(row.size) }}</template>
				</el-table-column>
				<el-table-column label="操作" width="135" fixed="right">
					<template #default="{ row }">
						<el-button type="primary" text @click.stop="openBackupDetail(row)">
							查看表 SQL
						</el-button>
						<el-button type="danger" text @click.stop="deleteBackup(row)"
							>删除</el-button
						>
					</template>
				</el-table-column>
			</el-table>
			<template #footer>
				<el-button
					type="danger"
					:disabled="backupSelection.length === 0"
					@click="deleteSelectedBackups"
				>
					删除选中{{ backupSelection.length ? ` (${backupSelection.length})` : "" }}
				</el-button>
				<el-button @click="backupListVisible = false">取消</el-button>
				<el-button
					type="warning"
					:disabled="!canRestoreSelectedBackup"
					@click="restoreSelected"
				>
					恢复全部表
				</el-button>
			</template>
		</el-dialog>

		<el-dialog v-model="backupDetailVisible" title="备份表 SQL" width="80%">
			<div class="backup-detail-tip">
				批次：{{ selectedBackupBatch?.backupName }}
				<span>（{{ backupTypeLabel(selectedBackupBatch?.backupType) }}）</span>
			</div>
			<el-table
				:data="backupTables"
				max-height="430"
				@selection-change="selectBackupTables"
				@row-click="viewBackupSql"
			>
				<el-table-column type="selection" width="48" />
				<el-table-column label="表名" prop="tableName" min-width="300" />
				<el-table-column
					label="文件名"
					prop="fileName"
					min-width="300"
					show-overflow-tooltip
				/>
				<el-table-column label="大小" width="100">
					<template #default="{ row }">{{ formatSize(row.size) }}</template>
				</el-table-column>
				<el-table-column label="操作" width="150" fixed="right">
					<template #default="{ row }">
						<el-button type="primary" text @click.stop="viewBackupSql(row)"
							>查看 SQL</el-button
						>
						<el-button type="warning" text @click.stop="restoreSingleTable(row)">
							恢复
						</el-button>
					</template>
				</el-table-column>
			</el-table>
			<template #footer>
				<el-button @click="backupDetailVisible = false">关闭</el-button>
				<el-button
					type="warning"
					:disabled="!canRestoreSelectedTables"
					@click="restoreSelectedTables"
				>
					恢复选中表{{
						backupTableSelection.length ? ` (${backupTableSelection.length})` : ""
					}}
				</el-button>
			</template>
		</el-dialog>

		<el-dialog v-model="backupSqlVisible" title="SQL 内容" width="80%">
			<div class="backup-sql-toolbar">
				<span>
					已加载 {{ formatSize(currentSqlOffset) }} /
					{{ formatSize(currentSqlTotalSize) }}
				</span>
			</div>
			<el-input
				v-model="currentSql"
				type="textarea"
				:rows="28"
				readonly
				placeholder="正在加载 SQL 片段..."
			/>
			<div class="backup-sql-load-more">
				<el-button
					type="primary"
					:loading="currentSqlLoading"
					:disabled="!currentSqlHasMore"
					@click="loadMoreSql"
				>
					{{ currentSqlHasMore ? "加载更多" : "已加载全部" }}
				</el-button>
			</div>
			<template #footer>
				<el-button type="primary" @click="downloadCurrentSql">下载完整 SQL</el-button>
				<el-button @click="backupSqlVisible = false">关闭</el-button>
			</template>
		</el-dialog>
	</cl-crud>
</template>

<script lang="ts" name="base-addons" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { ElMessage, ElMessageBox } from "element-plus";
import AddonsTypes from "./components/addons/types.vue";
import { computed, ref, watch } from "vue";
import { ArrowDown } from "@element-plus/icons-vue";

const { service } = useCool();
const { menu, addonsTask } = useBase();
console.log("perm", service.base.sys.addons._permission);

const typeList = ref<any>([]);
const addonsList = ref<any>([]);
const backupListVisible = ref(false);
const backupFiles = ref<any[]>([]);
const backupAddon = ref<any>();
const backupSelection = ref<any[]>([]);
const backupTypeTab = ref<"data" | "structure">("data");
const backupDetailVisible = ref(false);
const backupTables = ref<any[]>([]);
const backupTableSelection = ref<any[]>([]);
const selectedBackupBatch = ref<any>();
const backupSqlVisible = ref(false);
const currentSql = ref("");
const currentSqlFile = ref("");
const currentSqlOffset = ref(0);
const currentSqlTotalSize = ref(0);
const currentSqlHasMore = ref(false);
const currentSqlLoading = ref(false);

const getValue = () => {
	const map = {
		name: "张三"
	};

	function setV(obj: any) {
		obj.name = "李四";
	}

	setV(map);

	console.log("map", map);
};

getValue();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "标题",
			prop: "name",
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "",
			prop: "addonsName",
			hidden: true,
			component: { name: "el-input" }
		},
		{
			label: "插件",
			prop: "menuId",
			required: true,
			component: {
				name: "el-select",
				options: [],
				props: {
					onChange: (v) => {
						const addon = addonsList.value.find((item: any) => item.id == v);
						if (addon) {
							Upsert.value?.setForm("name", addon.name);
							if (addon.addonsName) {
								Upsert.value?.setForm("addonsName", addon.addonsName);
							}
						}
					}
				}
			}
		},
		{
			label: "类别",
			prop: "typeId",
			required: true,
			component: {
				name: "el-select",
				options: [],
				props: {}
			}
		},
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		},
		{
			label: "排序",
			prop: "orderNum",
			value: 99,
			required: true,
			component: { name: "el-input-number", props: { min: 0 } }
		}
	],
	async onOpen(data) {
		// 插件：从资源目录读取全部插件，再排除已经存在于插件列表的插件。
		service.base.sys.addons
			.request({ url: "/available", method: "GET" })
			.then((res) => {
				const availableAddons = res || [];
				addonsList.value = availableAddons;
				Upsert.value?.setOptions(
					"menuId",
					availableAddons.map((e) => ({
						label: e.name || e.addonsName || "",
						value: e.id
					}))
				);
				const current = availableAddons.find((item: any) => item.id == data?.menuId);
				if (current?.addonsName) {
					Upsert.value?.setForm("addonsName", current.addonsName);
				}
			})
			.catch((error) => {
				addonsList.value = [];
				Upsert.value?.setOptions("menuId", []);
				ElMessage.error(error?.message || "读取可添加插件失败");
			});

		// 类别
		Upsert.value?.setOptions(
			"typeId",
			typeList.value.map((e) => {
				return {
					label: e.name || "",
					value: e.id
				};
			})
		);
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "标题", prop: "name" },
		{ label: "类别", prop: "typeName" },
		{ label: "描述", prop: "remark", showOverflowTooltip: true },
		{ label: "排序", prop: "orderNum" },
		{ type: "op", buttons: ["slot-op"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.base.sys.addons,
		async onRefresh(params, { render }) {
			const { list, pagination } = await service.base.sys.addons.page(params);
			// 渲染数据
			render(list, pagination);

			typeList.value = await service.base.sys.addonsTypes.list();
		}
	},
	(app) => {
		app.refresh({ typeId: typeId.value, type: type.value });
	}
);

// 打开插件类型
const typesShow = ref(false);
const typeOpen = () => {
	typesShow.value = true;
};

//获取列表
const activeIndex = ref();
const typeId = ref();
const type = ref();
const getList = (tid: any, t?: any) => {
	if (t) {
		activeIndex.value = "hasInstall";
		typeId.value = null;
		type.value = t;
	}
	if (tid) {
		activeIndex.value = tid;
		typeId.value = tid;
		type.value = null;
	}
	if (!t && !tid) {
		activeIndex.value = null;
		typeId.value = null;
		type.value = null;
	}

	Crud.value?.refresh({ typeId: tid, type: t });
};

// 安装卸载
const installUpdateStatus = async (item: any, active: boolean) => {
	if (!active && (Number(item.isShow) === 1 || item.isShow === true)) {
		ElMessage.warning("请先下架插件后再卸载");
		return;
	}
	if (!active) {
		try {
			await ElMessageBox.confirm(
				"卸载会删除该插件的全部数据表。建议先执行备份，是否继续？",
				"卸载确认",
				{ type: "warning" }
			);
		} catch {
			return;
		}
	} else {
		try {
			await ElMessageBox.confirm(
				"安装将创建该插件的数据表并初始化菜单数据，是否继续？",
				"安装确认",
				{ type: "warning", confirmButtonText: "确认安装", cancelButtonText: "取消" }
			);
		} catch {
			return;
		}
	}
	service.base.sys.addons
		.installUpdateStatus({ id: item.menuId, active })
		.then(async () => {
			Crud.value?.refresh();
			await menu.get();
			ElMessage.success(active ? "安装成功" : "卸载成功");
		})
		.catch((e) => {
			ElMessage.error(e.message);
		});
};

// 上下架
const lineUpdateStatus = (item: any, active: boolean) => {
	if (!item.isInstall) {
		ElMessage.warning("请先安装插件后再上架");
		return;
	}
	service.base.sys.addons
		.lineUpdateStatus({ id: item.menuId, active })
		.then(async () => {
			Crud.value?.refresh();
			await menu.get();
			ElMessage.success(active ? "上架成功" : "下架成功");
		})
		.catch((e) => {
			ElMessage.error(e.message);
		});
};

const selectedAddonIds = () => Table.value?.selection.map((item: any) => item.menuId) || [];
const isAddonInstalled = (item: any) => item?.isInstall === true || Number(item?.isInstall) === 1;
const canBackupSelected = computed(() => {
	const selected = Table.value?.selection || [];
	return selected.length > 0 && selected.every(isAddonInstalled);
});
const canRestoreLatest = computed(() => {
	const selected = Table.value?.selection || [];
	return selected.length === 1 && isAddonInstalled(selected[0]);
});
const canRestoreSelectedBackup = computed(
	() => backupSelection.value.length === 1 && isAddonInstalled(backupAddon.value)
);
const canRestoreSelectedTables = computed(
	() => backupTableSelection.value.length > 0 && isAddonInstalled(backupAddon.value)
);

watch(
	() => addonsTask.status,
	(status, previousStatus) => {
		if (status === "completed" && previousStatus !== "completed") {
			Crud.value?.refresh();
		}
	}
);

const beginTask = async (url: string, data: any, operation: string) => {
	if (addonsTask.id && !addonsTask.isFinished()) {
		ElMessage.warning("已有备份或恢复任务正在执行，请先在右上角查看进度");
		return;
	}
	try {
		const result = await service.base.sys.addons.request({ url, method: "POST", data });
		const taskId = result?.taskId;
		if (!taskId) {
			throw new Error("任务创建失败");
		}
		addonsTask.start(taskId, operation);
	} catch (error: any) {
		ElMessage.error(error?.message || "操作失败");
	}
};

const backupSelected = async (backupType = "data") => {
	const ids = selectedAddonIds();
	if (!canBackupSelected.value) {
		ElMessage.warning("未安装的插件不支持备份，请仅选择已安装插件");
		return;
	}
	if (!ids.length) {
		return;
	}
	const type = backupType === "structure" ? "structure" : "data";
	const typeLabel = type === "structure" ? "结构" : "数据";
	try {
		await ElMessageBox.confirm(
			`将备份选中的 ${ids.length} 个插件的${typeLabel}，是否继续？`,
			"备份确认",
			{ type: "warning", confirmButtonText: "开始备份", cancelButtonText: "取消" }
		);
		beginTask("/backup", { ids, backupType: type }, "backup");
	} catch {
		// 用户取消备份。
	}
};

const backupTypeLabel = (backupType: string) =>
	backupType === "structure" ? "结构备份" : "数据备份";

const visibleBackupFiles = computed(() => {
	return backupFiles.value.filter(
		(item) => backupTypeLabel(item.backupType) === backupTypeLabel(backupTypeTab.value)
	);
});

const switchBackupTypeTab = () => {
	backupSelection.value = [];
};

const restoreLatest = async () => {
	const ids = selectedAddonIds();
	if (!canRestoreLatest.value || ids.length !== 1) {
		ElMessage.warning("仅已安装的单个插件支持恢复备份");
		return;
	}
	try {
		await ElMessageBox.confirm("将使用该插件最近一次备份覆盖当前数据，是否继续？", "恢复确认", {
			type: "warning"
		});
		beginTask("/restoreLatest", { id: ids[0] }, "restore");
	} catch {
		// 用户取消恢复。
	}
};

const openBackupList = async (item: any) => {
	backupAddon.value = item;
	backupSelection.value = [];
	backupTypeTab.value = "data";
	backupDetailVisible.value = false;
	try {
		await loadBackupFiles();
		backupListVisible.value = true;
	} catch (error: any) {
		ElMessage.error(error?.message || "获取备份列表失败");
	}
};

const openBackupDetail = async (row: any, _column?: any, event?: MouseEvent) => {
	const target = event?.target as HTMLElement | null;
	if (target?.closest?.(".el-checkbox")) {
		return;
	}
	selectedBackupBatch.value = row;
	backupTableSelection.value = [];
	try {
		const result = await service.base.sys.addons.request({
			url: "/backupDetail",
			method: "GET",
			params: { id: backupAddon.value.menuId, backupName: row.backupName }
		});
		backupTables.value = result?.tables || [];
		backupDetailVisible.value = true;
	} catch (error: any) {
		ElMessage.error(error?.message || "获取备份表失败");
	}
};

const loadBackupFiles = async () => {
	if (!backupAddon.value) {
		return;
	}
	backupFiles.value =
		(await service.base.sys.addons.request({
			url: "/backupList",
			method: "GET",
			params: { id: backupAddon.value.menuId }
		})) || [];
};

const selectBackupFiles = (rows: any[]) => {
	// 只有左侧复选框才会改变批次选中状态；点击行仅查看详情。
	backupSelection.value = rows;
};

const selectBackupTables = (rows: any[]) => {
	backupTableSelection.value = rows;
};

const deleteBackups = async (backupNames: string[]) => {
	if (!backupAddon.value || backupNames.length === 0) {
		return;
	}
	try {
		await ElMessageBox.confirm(
			`将永久删除选中的 ${backupNames.length} 个备份批次及其中的全部表 SQL，删除后无法恢复，是否继续？`,
			"删除备份确认",
			{ type: "warning", confirmButtonText: "删除本地文件", cancelButtonText: "取消" }
		);
		await service.base.sys.addons.request({
			url: "/backupDelete",
			method: "POST",
			data: { id: backupAddon.value.menuId, backupNames }
		});
		backupSelection.value = [];
		await loadBackupFiles();
		ElMessage.success("本地备份文件已删除");
	} catch (error: any) {
		if (error !== "cancel" && error !== "close") {
			ElMessage.error(error?.message || "删除备份失败");
		}
	}
};

const deleteBackup = (row: any) => {
	deleteBackups([row.backupName]);
};

const deleteSelectedBackups = () => {
	deleteBackups(backupSelection.value.map((item) => item.backupName));
};

const restoreSelected = async () => {
	if (!canRestoreSelectedBackup.value) {
		ElMessage.warning("请先安装插件后再恢复备份");
		return;
	}
	const selectedBatch = backupSelection.value[0];
	try {
		await ElMessageBox.confirm(
			"将按备份表顺序恢复该批次的全部表，失败的表会记录并继续，是否继续？",
			"恢复确认",
			{
				type: "warning"
			}
		);
		backupListVisible.value = false;
		backupDetailVisible.value = false;
		beginTask(
			"/restore",
			{ id: backupAddon.value.menuId, backupName: selectedBatch.backupName },
			"restore"
		);
	} catch {
		// 用户取消恢复。
	}
};

const restoreSelectedTables = async () => {
	if (!canRestoreSelectedTables.value || !selectedBackupBatch.value) {
		ElMessage.warning("请先勾选需要恢复的表");
		return;
	}
	try {
		await ElMessageBox.confirm(
			`将按顺序恢复选中的 ${backupTableSelection.value.length} 张表，失败的表会记录并继续，是否继续？`,
			"恢复确认",
			{ type: "warning" }
		);
		backupDetailVisible.value = false;
		backupListVisible.value = false;
		beginTask(
			"/restore",
			{
				id: backupAddon.value.menuId,
				backupName: selectedBackupBatch.value.backupName,
				fileNames: backupTableSelection.value.map((item) => item.fileName)
			},
			"restore"
		);
	} catch {
		// 用户取消恢复。
	}
};

const loadSqlPreview = async (append = false) => {
	if (!backupAddon.value || !selectedBackupBatch.value || !currentSqlFile.value) {
		return;
	}
	currentSqlLoading.value = true;
	try {
		const result = await service.base.sys.addons.request({
			url: "/backupPreview",
			method: "GET",
			params: {
				id: backupAddon.value.menuId,
				backupName: selectedBackupBatch.value.backupName,
				fileName: currentSqlFile.value,
				offset: append ? currentSqlOffset.value : 0,
				limit: 128 * 1024
			}
		});
		const content = result?.content || "";
		currentSql.value = append ? currentSql.value + content : content;
		currentSqlOffset.value = Number(result?.nextOffset || 0);
		currentSqlTotalSize.value = Number(result?.totalSize || 0);
		currentSqlHasMore.value = Boolean(result?.hasMore);
	} catch (error: any) {
		ElMessage.error(error?.message || "获取 SQL 片段失败");
	} finally {
		currentSqlLoading.value = false;
	}
};

const viewBackupSql = async (row: any, _column?: any, event?: MouseEvent) => {
	const target = event?.target as HTMLElement | null;
	if (target?.closest?.(".el-checkbox")) {
		return;
	}
	currentSqlFile.value = row.fileName;
	currentSql.value = "";
	currentSqlOffset.value = 0;
	currentSqlTotalSize.value = Number(row.size || 0);
	currentSqlHasMore.value = false;
	backupSqlVisible.value = true;
	await loadSqlPreview();
};

const loadMoreSql = async () => {
	if (!currentSqlLoading.value && currentSqlHasMore.value) {
		await loadSqlPreview(true);
	}
};

const downloadCurrentSql = async () => {
	if (!backupAddon.value || !selectedBackupBatch.value || !currentSqlFile.value) {
		return;
	}
	try {
		const blob = await service.base.sys.addons.request({
			url: "/backupDownload",
			method: "GET",
			params: {
				id: backupAddon.value.menuId,
				backupName: selectedBackupBatch.value.backupName,
				fileName: currentSqlFile.value
			},
			responseType: "blob"
		});
		const url = URL.createObjectURL(blob);
		const link = document.createElement("a");
		link.href = url;
		link.download = currentSqlFile.value;
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		window.setTimeout(() => URL.revokeObjectURL(url), 1000);
	} catch (error: any) {
		ElMessage.error(error?.message || "下载 SQL 失败");
	}
};

const restoreSingleTable = async (row: any) => {
	if (!isAddonInstalled(backupAddon.value)) {
		ElMessage.warning("请先安装插件后再恢复备份");
		return;
	}
	try {
		await ElMessageBox.confirm(
			`将只恢复表「${row.tableName}」，会覆盖该表当前数据，是否继续？`,
			"恢复确认",
			{ type: "warning" }
		);
		backupDetailVisible.value = false;
		backupListVisible.value = false;
		beginTask(
			"/restore",
			{
				id: backupAddon.value.menuId,
				backupName: selectedBackupBatch.value.backupName,
				fileName: row.fileName
			},
			"restore"
		);
	} catch {
		// 用户取消恢复。
	}
};

const formatSize = (size: number) => {
	if (!size || size < 0) return "0 B";
	const units = ["B", "KB", "MB", "GB", "TB", "PB"];
	let value = size;
	let unitIndex = 0;
	while (value >= 1024 && unitIndex < units.length - 1) {
		value /= 1024;
		unitIndex += 1;
	}
	return `${unitIndex === 0 ? value : value.toFixed(1)} ${units[unitIndex]}`;
};
</script>

<style lang="scss" scoped>
.f-btn:hover {
	color: #fff;
	background-color: var(--el-color-info-light-5);
	border-color: var(--el-color-info-light-5);
}
.active {
	color: #fff;
	background-color: var(--el-color-info-light-5);
}
.backup-detail-tip {
	margin-bottom: 12px;
	color: var(--el-text-color-secondary);
	font-size: 13px;
}
.backup-sql-toolbar {
	display: flex;
	align-items: center;
	justify-content: flex-start;
	margin-bottom: 8px;
	color: var(--el-text-color-secondary);
	font-size: 13px;
}
.backup-sql-load-more {
	display: flex;
	justify-content: center;
	padding: 12px 0 2px;
}
.fileter {
	display: flex;
	align-items: center;
	padding: 0 30px;
	.filter-title {
		width: 50px;
		span {
			color: #333;
			font-weight: 400;
			font-size: 14px;
		}
	}
	.filter-name {
		span {
			color: #666;
			font-size: 14px;
		}
	}
	.active {
		span {
			color: var(--el-color-primary);
			font-weight: 400;
			font-size: 14px;
		}
	}
}
</style>
