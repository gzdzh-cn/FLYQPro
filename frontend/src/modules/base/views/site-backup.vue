<template>
	<div class="site-backup-page">
		<div class="site-backup-toolbar">
			<el-button @click="loadTables">刷新表列表</el-button>
			<el-dropdown
				:disabled="tableSelection.length === 0"
				@command="startBackup"
			>
				<el-button type="primary" :disabled="tableSelection.length === 0">
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
			<el-button type="success" @click="openBackupList">备份列表</el-button>
			<span class="selected-count">已选 {{ tableSelection.length }} 张表</span>
		</div>

		<!-- <div class="site-backup-section-title">备份数据库表</div> -->
		<el-alert
			title="请选择需要备份的数据库表，再从顶部下拉菜单选择备份数据或备份结构。"
			type="info"
			:closable="false"
			show-icon
		/>
		<el-table
			v-loading="tablesLoading"
			:data="tableList"
			stripe
			border
			@selection-change="selectTables"
		>
			<el-table-column type="selection" width="55" />
			<el-table-column label="序号" prop="orderNum" width="90" />
			<el-table-column label="数据库表名" prop="tableName" min-width="360" />
			<el-table-column label="容量" width="140">
				<template #default="{ row }">{{ formatSize(row.size) }}</template>
			</el-table-column>
		</el-table>
	</div>

	<el-dialog v-model="backupListVisible" title="全站备份列表" width="80%" destroy-on-close>
		<el-tabs v-model="backupTypeTab" @tab-change="clearBackupSelection">
			<el-tab-pane label="数据备份" name="data" />
			<el-tab-pane label="结构备份" name="structure" />
		</el-tabs>
		<el-table
			:key="backupTypeTab"
			:data="visibleBackupFiles"
			max-height="520"
			@selection-change="selectBackups"
			@row-click="openBackupDetail"
		>
			<el-table-column type="selection" width="55" />
			<el-table-column label="备份批次" prop="backupName" min-width="330" show-overflow-tooltip />
			<el-table-column label="表数量" prop="tableCount" width="90" />
			<el-table-column label="时间" prop="createTime" width="180" />
			<el-table-column label="大小" width="120">
				<template #default="{ row }">{{ formatSize(row.size) }}</template>
			</el-table-column>
			<el-table-column label="操作" width="150" fixed="right">
				<template #default="{ row }">
					<el-button type="primary" text @click.stop="openBackupDetail(row)">查看表 SQL</el-button>
					<el-button type="danger" text @click.stop="deleteBackups([row.backupName])">删除</el-button>
				</template>
			</el-table-column>
		</el-table>
		<template #footer>
			<el-button type="danger" :disabled="!backupSelection.length" @click="deleteSelectedBackups">
				删除选中{{ backupSelection.length ? ` (${backupSelection.length})` : "" }}
			</el-button>
			<el-button @click="backupListVisible = false">关闭</el-button>
			<el-button type="warning" :disabled="backupSelection.length !== 1" @click="restoreBackup">
				恢复全部表
			</el-button>
		</template>
	</el-dialog>

	<el-dialog v-model="backupDetailVisible" title="全站备份表 SQL" width="80%" destroy-on-close>
		<div class="backup-detail-tip">
			批次：{{ selectedBackupBatch?.backupName }}（{{ backupTypeLabel(selectedBackupBatch?.backupType) }}）
		</div>
		<el-table
			:data="backupTables"
			max-height="540"
			@selection-change="selectBackupTables"
			@row-click="viewBackupSql"
		>
			<el-table-column type="selection" width="55" />
			<el-table-column label="数据库表名" prop="tableName" min-width="330" />
			<el-table-column label="SQL 文件" prop="fileName" min-width="330" show-overflow-tooltip />
			<el-table-column label="大小" width="120">
				<template #default="{ row }">{{ formatSize(row.size) }}</template>
			</el-table-column>
			<el-table-column label="操作" width="150" fixed="right">
				<template #default="{ row }">
					<el-button type="primary" text @click.stop="viewBackupSql(row)">查看 SQL</el-button>
					<el-button type="warning" text @click.stop="restoreSingleTable(row)">恢复</el-button>
				</template>
			</el-table-column>
		</el-table>
		<template #footer>
			<el-button @click="backupDetailVisible = false">关闭</el-button>
			<el-button type="warning" :disabled="!backupTableSelection.length" @click="restoreSelectedTables">
				恢复选中表{{ backupTableSelection.length ? ` (${backupTableSelection.length})` : "" }}
			</el-button>
		</template>
	</el-dialog>

	<el-dialog v-model="backupSqlVisible" title="SQL 内容" width="80%">
		<div class="backup-sql-toolbar">
			已加载 {{ formatSize(currentSqlOffset) }} / {{ formatSize(currentSqlTotalSize) }}
		</div>
		<el-input v-model="currentSql" type="textarea" :rows="28" readonly placeholder="正在加载 SQL 片段..." />
		<div class="backup-sql-load-more">
			<el-button type="primary" :loading="currentSqlLoading" :disabled="!currentSqlHasMore" @click="loadMoreSql">
				{{ currentSqlHasMore ? "加载更多" : "已加载全部" }}
			</el-button>
		</div>
		<template #footer>
			<el-button type="primary" @click="downloadCurrentSql">下载完整 SQL</el-button>
			<el-button @click="backupSqlVisible = false">关闭</el-button>
		</template>
	</el-dialog>
</template>

<script lang="ts" name="base-site-backup" setup>
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { ArrowDown } from "@element-plus/icons-vue";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";

const { service } = useCool();
const { addonsTask } = useBase();

const tableList = ref<any[]>([]);
const tableSelection = ref<any[]>([]);
const tablesLoading = ref(false);
const backupListVisible = ref(false);
const backupFiles = ref<any[]>([]);
const backupSelection = ref<any[]>([]);
const backupTypeTab = ref<"data" | "structure">("data");
const backupDetailVisible = ref(false);
const selectedBackupBatch = ref<any>();
const backupTables = ref<any[]>([]);
const backupTableSelection = ref<any[]>([]);
const backupSqlVisible = ref(false);
const currentSql = ref("");
const currentSqlFile = ref("");
const currentSqlOffset = ref(0);
const currentSqlTotalSize = ref(0);
const currentSqlHasMore = ref(false);
const currentSqlLoading = ref(false);

const visibleBackupFiles = computed(() =>
	backupFiles.value.filter((item) => item.backupType === backupTypeTab.value)
);

const request = (url: string, method: string, options: any = {}) =>
	service.base.sys.addons.request({ url, method, ...options });

const loadTables = async () => {
	tablesLoading.value = true;
	try {
		tableList.value = (await request("/siteBackupTables", "GET")) || [];
	} catch (error: any) {
		ElMessage.error(error?.message || "获取数据库表失败");
	} finally {
		tablesLoading.value = false;
	}
};

const selectTables = (rows: any[]) => {
	tableSelection.value = rows;
};

const startBackup = async (backupType = "data") => {
	if (!tableSelection.value.length) {
		ElMessage.warning("请先勾选需要备份的数据库表");
		return;
	}
	if (addonsTask.id && !addonsTask.isFinished()) {
		ElMessage.warning("已有备份或恢复任务正在执行，请先等待任务完成");
		return;
	}
	const type = backupType === "structure" ? "structure" : "data";
	try {
		await ElMessageBox.confirm(
			`将备份选中的 ${tableSelection.value.length} 张表的${type === "structure" ? "结构" : "数据"}，是否继续？`,
			"全站备份确认",
			{ type: "warning", confirmButtonText: "开始备份", cancelButtonText: "取消" }
		);
		const result = await request("/siteBackup", "POST", {
			data: { tableNames: tableSelection.value.map((item) => item.tableName), backupType: type }
		});
		if (!result?.taskId) throw new Error("任务创建失败");
		addonsTask.start(result.taskId, "backup", "site");
	} catch (error: any) {
		if (error !== "cancel" && error !== "close") ElMessage.error(error?.message || "创建备份任务失败");
	}
};

const openBackupList = async () => {
	backupSelection.value = [];
	backupTypeTab.value = "data";
	try {
		backupFiles.value = (await request("/siteBackupList", "GET")) || [];
		backupListVisible.value = true;
	} catch (error: any) {
		ElMessage.error(error?.message || "获取备份列表失败");
	}
};

const clearBackupSelection = () => {
	backupSelection.value = [];
};

const selectBackups = (rows: any[]) => {
	backupSelection.value = rows;
};

const openBackupDetail = async (row: any, _column?: any, event?: MouseEvent) => {
	if ((event?.target as HTMLElement | null)?.closest?.(".el-checkbox")) return;
	selectedBackupBatch.value = row;
	backupTableSelection.value = [];
	try {
		const result = await request("/siteBackupDetail", "GET", { params: { backupName: row.backupName } });
		backupTables.value = result?.tables || [];
		backupDetailVisible.value = true;
	} catch (error: any) {
		ElMessage.error(error?.message || "获取备份表失败");
	}
};

const selectBackupTables = (rows: any[]) => {
	backupTableSelection.value = rows;
};

const deleteBackups = async (backupNames: string[]) => {
	if (!backupNames.length) return;
	try {
		await ElMessageBox.confirm(
			`将永久删除选中的 ${backupNames.length} 个全站备份批次，删除后无法恢复，是否继续？`,
			"删除备份确认",
			{ type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
		);
		await request("/siteBackupDelete", "POST", { data: { backupNames } });
		backupSelection.value = [];
		backupFiles.value = (await request("/siteBackupList", "GET")) || [];
		ElMessage.success("备份已删除");
	} catch (error: any) {
		if (error !== "cancel" && error !== "close") ElMessage.error(error?.message || "删除备份失败");
	}
};

const deleteSelectedBackups = () => deleteBackups(backupSelection.value.map((item) => item.backupName));

const startRestore = async (fileNames: string[] = []) => {
	if (!selectedBackupBatch.value) return;
	if (addonsTask.id && !addonsTask.isFinished()) {
		ElMessage.warning("已有备份或恢复任务正在执行，请先等待任务完成");
		return;
	}
	try {
		await ElMessageBox.confirm(
			fileNames.length
				? `将按顺序恢复选中的 ${fileNames.length} 张表，失败的表会记录并继续，是否继续？`
				: "将按顺序恢复该批次的全部表，失败的表会记录并继续，是否继续？",
			"全站恢复确认",
			{ type: "warning", confirmButtonText: "开始恢复", cancelButtonText: "取消" }
		);
		const result = await request("/siteRestore", "POST", {
			data: { backupName: selectedBackupBatch.value.backupName, fileNames }
		});
		if (!result?.taskId) throw new Error("任务创建失败");
		backupDetailVisible.value = false;
		backupListVisible.value = false;
		addonsTask.start(result.taskId, "restore", "site");
	} catch (error: any) {
		if (error !== "cancel" && error !== "close") ElMessage.error(error?.message || "创建恢复任务失败");
	}
};

const restoreBackup = () => {
	if (backupSelection.value.length === 1) {
		selectedBackupBatch.value = backupSelection.value[0];
		startRestore();
	}
};

const restoreSelectedTables = () => startRestore(backupTableSelection.value.map((item) => item.fileName));

const restoreSingleTable = (row: any) => startRestore([row.fileName]);

const backupTypeLabel = (type: string) => (type === "structure" ? "结构备份" : "数据备份");

const loadSqlPreview = async (append = false) => {
	if (!selectedBackupBatch.value || !currentSqlFile.value) return;
	currentSqlLoading.value = true;
	try {
		const result = await request("/siteBackupPreview", "GET", {
			params: {
				backupName: selectedBackupBatch.value.backupName,
				fileName: currentSqlFile.value,
				offset: append ? currentSqlOffset.value : 0,
				limit: 128 * 1024
			}
		});
		currentSql.value = append ? currentSql.value + (result?.content || "") : result?.content || "";
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
	if ((event?.target as HTMLElement | null)?.closest?.(".el-checkbox")) return;
	currentSqlFile.value = row.fileName;
	currentSql.value = "";
	currentSqlOffset.value = 0;
	currentSqlTotalSize.value = Number(row.size || 0);
	currentSqlHasMore.value = false;
	backupSqlVisible.value = true;
	await loadSqlPreview();
};

const loadMoreSql = () => {
	if (!currentSqlLoading.value && currentSqlHasMore.value) loadSqlPreview(true);
};

const downloadCurrentSql = async () => {
	if (!selectedBackupBatch.value || !currentSqlFile.value) return;
	try {
		const blob = await request("/siteBackupDownload", "GET", {
			params: { backupName: selectedBackupBatch.value.backupName, fileName: currentSqlFile.value },
			responseType: "blob"
		});
		const url = URL.createObjectURL(blob);
		const link = document.createElement("a");
		link.href = url;
		link.download = currentSqlFile.value;
		link.click();
		window.setTimeout(() => URL.revokeObjectURL(url), 1000);
	} catch (error: any) {
		ElMessage.error(error?.message || "下载 SQL 失败");
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

onMounted(loadTables);
</script>

<style lang="scss" scoped>
.site-backup-page {
	padding: 16px 20px;
}

.site-backup-toolbar {
	display: flex;
	align-items: center;
	gap: 10px;
	margin-bottom: 16px;
}

.selected-count {
	color: var(--el-text-color-secondary);
	font-size: 13px;
}

.site-backup-section-title {
	margin: 18px 0 10px;
	font-size: 16px;
	font-weight: 600;
}

.backup-detail-tip,
.backup-sql-toolbar {
	margin-bottom: 12px;
	color: var(--el-text-color-secondary);
	font-size: 13px;
}

.backup-sql-load-more {
	display: flex;
	justify-content: center;
	padding: 12px 0 2px;
}
</style>
