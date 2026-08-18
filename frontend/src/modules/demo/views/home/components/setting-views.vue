<template>
	<div class="setting-container">
		<main class="main-content">
			<!-- 欢迎区域 -->
			<div class="welcome-section">
				<div class="welcome-text">
					<h2>{{ greeting }}，{{ username }} 👋</h2>
				</div>
				<div class="welcome-date">
					<el-icon :size="18">
						<Calendar />
					</el-icon>
					<span>{{ currentDate }}</span>
				</div>
			</div>

			<!-- 统计卡片 -->
			<el-row v-if="isAdmin" :gutter="20" class="stat-row">
				<el-col :xs="12" :sm="6">
					<div class="stat-card" style="border-top: 3px solid #409eff">
						<div class="stat-icon" style="background: rgba(64,158,255,0.1)">
							<el-icon :size="24" color="#409eff">
								<User />
							</el-icon>
						</div>
						<div class="stat-info">
							<span class="stat-value">{{ statData.userCount }}</span>
							<span class="stat-title">用户总数</span>
						</div>
					</div>
				</el-col>
				<el-col :xs="12" :sm="6">
					<div class="stat-card" style="border-top: 3px solid #67c23a">
						<div class="stat-icon" style="background: rgba(103,194,58,0.1)">
							<el-icon :size="24" color="#67c23a">
								<Connection />
							</el-icon>
						</div>
						<div class="stat-info">
							<span class="stat-value">{{ statData.onlineCount }}</span>
							<span class="stat-title">在线人数</span>
						</div>
					</div>
				</el-col>
				<el-col :xs="12" :sm="6">
					<div class="stat-card" style="border-top: 3px solid #e6a23c">
						<div class="stat-icon" style="background: rgba(230,162,60,0.1)">
							<el-icon :size="24" color="#e6a23c">
								<ChatDotRound />
							</el-icon>
						</div>
						<div class="stat-info">
							<span class="stat-value">{{ statData.todayClues }}</span>
							<span class="stat-title">今日订单</span>
						</div>
					</div>
				</el-col>
				<el-col :xs="12" :sm="6">
					<div class="stat-card" style="border-top: 3px solid #f56c6c">
						<div class="stat-icon" style="background: rgba(245,108,108,0.1)">
							<el-icon :size="24" color="#f56c6c">
								<Clock />
							</el-icon>
						</div>
						<div class="stat-info">
							<span class="stat-value">{{ statData.pendingFollow }}</span>
							<span class="stat-title">待跟进</span>
						</div>
					</div>
				</el-col>
			</el-row>

			<!-- 信息面板 -->
			<el-row :gutter="20" class="info-row">
				<!-- 系统信息 -->
				<el-col :xs="24" :lg="14">
					<el-card class="info-card" shadow="never">
						<template #header>
							<div class="card-header">
								<span><el-icon>
										<Setting />
									</el-icon> 系统信息</span>
							</div>
						</template>
						<div class="info-list">
							<div class="info-item">
								<span class="info-label">系统版本</span>
								<span class="info-value">{{ serverInfo?.dzhVersion || '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">网站名称</span>
								<span class="info-value">{{ '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">网站域名/IP</span>
								<span class="info-value">{{ serverInfo?.hostUrl || '-' }} [ {{ serverInfo?.sourceIp ||
									'-' }} ]</span>
							</div>
							<div class="info-item">
								<span class="info-label">版权所有</span>
								<span class="info-value">盗版必究</span>
							</div>
							<div class="info-item">
								<span class="info-label">服务器系统</span>
								<span class="info-value">{{ serverInfo?.goHostOs || '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">服务器架构</span>
								<span class="info-value">{{ serverInfo?.goHostArch || '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">Go 版本</span>
								<span class="info-value">{{ serverInfo?.goVersion || '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">GoFrame 版本</span>
								<span class="info-value">{{ serverInfo?.gfVersion || '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">数据库版本</span>
								<span class="info-value">{{ serverInfo?.dBVersion || '-' }}</span>
							</div>
							<div class="info-item">
								<span class="info-label">许可证</span>
								<span class="info-value" style="color: #67c23a">企业版</span>
							</div>
						</div>
					</el-card>
				</el-col>

				<!-- 快捷操作 -->
				<el-col :xs="24" :lg="10">
					<el-card class="info-card" shadow="never">
						<template #header>
							<div class="card-header">
								<span><el-icon>
										<Operation />
									</el-icon> 快捷操作</span>
							</div>
						</template>
						<div class="quick-actions">
							<div class="action-item" v-for="(item, idx) in quickActions" :key="item.menuId || idx"
								@click="handleAction(item)">
								<div class="action-icon" :style="{ background: getIconBg(item.icon) }">
									<el-icon :size="22" :color="getIconColor(item.icon)">
										<component :is="getIconComp(item.icon)" />
									</el-icon>
								</div>
								<span class="action-label">{{ item.name }}</span>
								<el-icon class="action-delete" :size="14"
									@click.stop="handleDeleteQuickAction(item, idx)">
									<Close />
								</el-icon>
							</div>
							<!-- 添加按钮 -->
							<div class="action-item action-add" @click="showAddDialog">
								<div class="action-icon" style="background: #f5f7fa">
									<el-icon :size="22" color="#909399">
										<Plus />
									</el-icon>
								</div>
								<span class="action-label" style="color: #909399">添加</span>
							</div>
						</div>
					</el-card>
				</el-col>
			</el-row>

			<!-- 系统公告 - 底部占满宽度 -->
			<el-card class="info-card notice-card" shadow="never">
				<template #header>
					<div class="card-header">
						<span><el-icon>
								<Bell />
							</el-icon> 系统公告</span>
						<el-tag v-if="noticeCount > 0" size="small" type="danger">{{ noticeCount }} 条未读</el-tag>
					</div>
				</template>
				<div class="notice-list" v-if="notices.length > 0">
					<div class="notice-item" v-for="notice in notices" :key="notice.id"
						@click="handleNoticeClick(notice)">
						<div class="notice-row">
							<span class="notice-dot" v-if="!notice.isRead"></span>
							<span class="notice-text">{{ notice.title }}：{{ notice.content }}</span>
						</div>
						<span class="notice-time">{{ notice.createTime }}</span>
					</div>
				</div>
				<div v-else class="empty-notice">暂无公告</div>
			</el-card>
		</main>

		<!-- 添加快捷操作弹窗 -->
		<el-dialog v-model="addDialogVisible" title="添加快捷操作" width="520px" :close-on-click-modal="false">
			<el-form :model="addForm" label-width="80px" ref="addFormRef">
				<el-form-item label="菜单" prop="menuId"
					:rules="[{ required: true, message: '请选择菜单', trigger: 'change' }]">
					<el-select v-model="addForm.menuId" placeholder="请选择菜单" filterable style="width: 100%"
						@change="handleMenuSelect">
						<el-option v-for="menu in menuList" :key="menu.id || menu.router" :label="menu.name"
							:value="menu.id || menu.router" />
					</el-select>
				</el-form-item>
				<el-form-item label="图标" prop="icon">
					<div class="icon-picker">
						<div class="icon-option" v-for="icon in iconOptions" :key="icon.name"
							:class="{ active: addForm.icon === icon.name }" @click="addForm.icon = icon.name"
							:title="icon.name">
							<el-icon :size="18">
								<component :is="icon.comp" />
							</el-icon>
						</div>
					</div>
				</el-form-item>
			</el-form>
			<template #footer>
				<el-button @click="addDialogVisible = false">取消</el-button>
				<el-button type="primary" @click="handleAddQuickAction" :loading="addLoading">确定</el-button>
			</template>
		</el-dialog>

		<!-- 公告详情弹窗 -->
		<el-dialog v-model="noticeDialogVisible" :title="currentNotice?.title || '公告详情'" width="560px">
			<div v-if="currentNotice" class="notice-detail">
				<div class="notice-detail-content">{{ currentNotice.content }}</div>
				<div class="notice-detail-time">{{ currentNotice.createTime }}</div>
			</div>
		</el-dialog>
	</div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted, markRaw } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
	Calendar,
	Setting,
	Operation,
	Bell,
	User,
	Connection,
	ChatDotRound,
	Clock,
	Plus,
	Close,
	// 图标选择器可用的图标
	Document,
	FolderOpened,
	Promotion,
	UserFilled,
	Goods,
	DataLine,
	Tickets,
	Monitor,
	Menu as MenuIcon,
	HomeFilled,
	Setting as SettingIcon,
	Message,
	Notification,
	Phone,
	ChatLineSquare,
	Star,
	Flag,
	Location,
	Link,
	Download,
	Upload,
	Search,
	Edit,
	View,
	Key,
	Lock,
	Unlock,
	PieChart,
	TrendCharts,
	Opportunity,
	Management,
	DataBoard,
	DataAnalysis,
	Files,
	Paperclip,
	Stamp,
	Tools,
	MuteNotification,
	Comment
} from "@element-plus/icons-vue";
import { router, useCool } from "/@/cool";
import { useBase } from "/$/base";

const { service } = useCool();
const { user } = useBase();
const isAdmin = computed(() => user.info.roleIds?.split(",").includes("1") || false);

const username = ref(sessionStorage.getItem("username") || "admin");
const serverInfo = ref<any>({});
const notices = ref<any[]>([]);
const noticeCount = ref(0);
const noticeDialogVisible = ref(false);
const currentNotice = ref<any>(null);

// 统计数据
const statData = ref({
	userCount: 0,
	onlineCount: 0,
	todayClues: 0,
	pendingFollow: 0
});

// 欢迎语
const greeting = computed(() => {
	const h = new Date().getHours();
	if (h < 6) return "凌晨好";
	if (h < 12) return "上午好";
	if (h < 14) return "中午好";
	if (h < 18) return "下午好";
	return "晚上好";
});

// 当前日期
const currentDate = computed(() => {
	const d = new Date();
	const weekMap = ["日", "一", "二", "三", "四", "五", "六"];
	return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 星期${weekMap[d.getDay()]}`;
});

// ==================== 图标相关 ====================

// 可选图标列表
const iconOptions = [
	{ name: "User", comp: markRaw(User), color: "#409eff" },
	{ name: "UserFilled", comp: markRaw(UserFilled), color: "#409eff" },
	{ name: "Goods", comp: markRaw(Goods), color: "#67c23a" },
	{ name: "Document", comp: markRaw(Document), color: "#e6a23c" },
	{ name: "FolderOpened", comp: markRaw(FolderOpened), color: "#f56c6c" },
	{ name: "ChatDotRound", comp: markRaw(ChatDotRound), color: "#909399" },
	{ name: "ChatLineSquare", comp: markRaw(ChatLineSquare), color: "#909399" },
	{ name: "Comment", comp: markRaw(Comment), color: "#909399" },
	{ name: "Promotion", comp: markRaw(Promotion), color: "#b37feb" },
	{ name: "DataLine", comp: markRaw(DataLine), color: "#67c23a" },
	{ name: "DataAnalysis", comp: markRaw(DataAnalysis), color: "#e6a23c" },
	{ name: "DataBoard", comp: markRaw(DataBoard), color: "#409eff" },
	{ name: "PieChart", comp: markRaw(PieChart), color: "#e6a23c" },
	{ name: "TrendCharts", comp: markRaw(TrendCharts), color: "#67c23a" },
	{ name: "Tickets", comp: markRaw(Tickets), color: "#e6a23c" },
	{ name: "Monitor", comp: markRaw(Monitor), color: "#909399" },
	{ name: "Management", comp: markRaw(Management), color: "#409eff" },
	{ name: "Opportunity", comp: markRaw(Opportunity), color: "#f56c6c" },
	{ name: "Message", comp: markRaw(Message), color: "#409eff" },
	{ name: "Notification", comp: markRaw(Notification), color: "#e6a23c" },
	{ name: "Phone", comp: markRaw(Phone), color: "#67c23a" },
	{ name: "Star", comp: markRaw(Star), color: "#e6a23c" },
	{ name: "Flag", comp: markRaw(Flag), color: "#f56c6c" },
	{ name: "Location", comp: markRaw(Location), color: "#409eff" },
	{ name: "Link", comp: markRaw(Link), color: "#409eff" },
	{ name: "Download", comp: markRaw(Download), color: "#67c23a" },
	{ name: "Upload", comp: markRaw(Upload), color: "#e6a23c" },
	{ name: "Search", comp: markRaw(Search), color: "#909399" },
	{ name: "Edit", comp: markRaw(Edit), color: "#409eff" },
	{ name: "View", comp: markRaw(View), color: "#909399" },
	{ name: "Key", comp: markRaw(Key), color: "#e6a23c" },
	{ name: "Lock", comp: markRaw(Lock), color: "#f56c6c" },
	{ name: "Unlock", comp: markRaw(Unlock), color: "#67c23a" },
	{ name: "Files", comp: markRaw(Files), color: "#909399" },
	{ name: "Stamp", comp: markRaw(Stamp), color: "#f56c6c" },
	{ name: "Tools", comp: markRaw(Tools), color: "#909399" },
	{ name: "HomeFilled", comp: markRaw(HomeFilled), color: "#409eff" },
	{ name: "Setting", comp: markRaw(SettingIcon), color: "#909399" },
	{ name: "Menu", comp: markRaw(MenuIcon), color: "#409eff" },
	{ name: "Paperclip", comp: markRaw(Paperclip), color: "#909399" },
	{ name: "MuteNotification", comp: markRaw(MuteNotification), color: "#f56c6c" },
	{ name: "Clock", comp: markRaw(Clock), color: "#f56c6c" },
	{ name: "Connection", comp: markRaw(Connection), color: "#67c23a" }
];

// 图标名称到组件的映射
const iconMap: Record<string, any> = {};
iconOptions.forEach((item) => {
	iconMap[item.name] = item.comp;
});

// 根据图标名称获取组件
const getIconComp = (iconName: string) => {
	return iconMap[iconName] || markRaw(Goods);
};

// 根据图标名称获取颜色
const getIconColor = (iconName: string) => {
	const found = iconOptions.find((item) => item.name === iconName);
	return found?.color || "#409eff";
};

// 根据图标名称获取背景色
const getIconBg = (iconName: string) => {
	const color = getIconColor(iconName);
	// 将 hex 转 rgb 用于背景透明度
	const r = parseInt(color.slice(1, 3), 16);
	const g = parseInt(color.slice(3, 5), 16);
	const b = parseInt(color.slice(5, 7), 16);
	return `rgba(${r},${g},${b},0.1)`;
};

// ==================== 快捷操作 ====================

const quickActions = ref<any[]>([]);
const addDialogVisible = ref(false);
const addLoading = ref(false);
const addFormRef = ref();
const menuList = ref<any[]>([]);

const addForm = ref({
	menuId: "",
	name: "",
	icon: "Goods"
});

// 显示添加弹窗
const showAddDialog = async () => {
	addForm.value = { menuId: "", name: "", icon: "Goods" };
	addDialogVisible.value = true;
	// 获取菜单列表
	try {
		const res: any = await service.base.sys.quickMenu.quickMenuList();
		menuList.value = Array.isArray(res) ? res : [];
	} catch (e) {
		console.error("获取菜单列表失败", e);
		menuList.value = [];
	}
};

// 选中菜单时自动填充名称
const handleMenuSelect = (val: string) => {
	const menu = menuList.value.find((m: any) => (m.id || m.router) === val);
	if (menu) {
		addForm.value.name = menu.name || "";
	}
};

// 提交添加快捷操作
const handleAddQuickAction = async () => {
	try {
		await addFormRef.value?.validate();
	} catch {
		return;
	}
	addLoading.value = true;
	try {
		await service.base.sys.quickMenu.add({
			menuId: addForm.value.menuId,
			name: addForm.value.name,
			icon: addForm.value.icon
		});
		ElMessage.success("添加成功");
		addDialogVisible.value = false;
		getQuickActions();
	} catch (e: any) {
		ElMessage.error("添加失败");
	} finally {
		addLoading.value = false;
	}
};

// 删除快捷操作
const handleDeleteQuickAction = async (item: any, index: number) => {
	try {
		await ElMessageBox.confirm(`确定删除快捷操作「${item.name}」？`, "提示", {
			type: "warning"
		});
		try {
			await service.base.sys.quickMenu.delete({ ids: [item.id] });
			ElMessage.success("删除成功");
			getQuickActions();
		} catch (e) {
			ElMessage.error("删除失败");
		}
	} catch {
		// 取消删除
	}
};

// 获取快捷操作列表
const getQuickActions = async () => {
	try {
		const res: any = await service.base.sys.quickMenu.list();
		quickActions.value = Array.isArray(res) ? res : [];
	} catch (e) {
		console.error("获取快捷操作失败", e);
		quickActions.value = [];
	}
};

// 快捷操作点击
const handleAction = (item: any) => {
	if (item.router) {
		router.push(item.router);
	} else {
		ElMessage.info(`${item.name}功能开发中...`);
	}
};

// ==================== 数据获取 ====================

// 获取服务器信息
const getServerInfo = async () => {
	try {
		serverInfo.value = await service.base.open.serverInfo();
	} catch (e) {
		console.error("获取服务器信息失败", e);
	}
};

// 获取统计数据
const getStatData = async () => {
	try {
		const [userRes, onlineRes]: any[] = await Promise.all([
			service.base.sys.user.count(),
			(service.base.sys.user as any).onlineCount()
		]);
		statData.value.userCount = userRes?.count ?? 0;
		statData.value.onlineCount = onlineRes?.count ?? 0;
	} catch (e) {
		console.error("获取统计数据失败", e);
	}
};

// 获取公告列表
const getNotices = async () => {
	try {
		const res: any = await service.base.sys.announcement.list();
		notices.value = res.list || [];
		noticeCount.value = res.unreadCount || 0;
	} catch (e) {
		console.error("获取公告失败", e);
		notices.value = [];
	}
};

// 点击公告
const handleNoticeClick = async (notice: any) => {
	currentNotice.value = notice;
	noticeDialogVisible.value = true;
	// 标记已读
	if (!notice.isRead) {
		try {
			await service.base.sys.announcement.update({
				id: notice.id,
				read: 1
			});
			notice.isRead = 1;
			noticeCount.value = Math.max(0, noticeCount.value - 1);
		} catch (e) {
			console.error("标记已读失败", e);
		}
	}
};

onMounted(() => {
	getServerInfo();
	if (isAdmin.value) {
		getStatData();
	}
	getNotices();
	getQuickActions();
});
</script>

<style lang="scss" scoped>
.setting-container {
	// 容器样式
}

.main-content {
	max-width: 1400px;
	margin: 0 auto;
	padding: 10px 0;
}

/* 欢迎区域 */
.welcome-section {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 24px;
	padding: 28px 32px;
	background: linear-gradient(135deg, #409eff 0%, #337ecc 100%);
	border-radius: 12px;
	color: #fff;
}

.welcome-text h2 {
	font-size: 22px;
	font-weight: 600;
	margin-bottom: 6px;
}

.welcome-date {
	display: flex;
	align-items: center;
	gap: 6px;
	font-size: 14px;
	opacity: 0.85;
}

/* 统计卡片 */
.stat-row {
	margin-bottom: 20px;
}

.stat-card {
	background: #fff;
	border-radius: 12px;
	padding: 22px 20px;
	display: flex;
	align-items: center;
	gap: 16px;
	position: relative;
	transition: transform 0.2s, box-shadow 0.2s;
	cursor: default;
}

.stat-card:hover {
	transform: translateY(-2px);
	box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
}

.stat-icon {
	width: 52px;
	height: 52px;
	border-radius: 12px;
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
}

.stat-info {
	display: flex;
	flex-direction: column;
	flex: 1;
}

.stat-value {
	font-size: 24px;
	font-weight: 700;
	color: #303133;
	line-height: 1.2;
}

.stat-title {
	font-size: 13px;
	color: #909399;
	margin-top: 4px;
}

/* 信息卡片 */
.info-row .el-col {
	margin-bottom: 20px;
}

.info-card {
	border-radius: 12px;
	border: none;
}

.info-card :deep(.el-card__header) {
	padding: 16px 20px;
	border-bottom: 1px solid #f0f0f0;
}

.info-card :deep(.el-card__body) {
	padding: 20px;
}

.card-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	font-size: 15px;
	font-weight: 600;
	color: #303133;
}

.card-header .el-icon {
	margin-right: 6px;
	vertical-align: middle;
}

/* 系统信息列表 */
.info-list {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 0;
}

.info-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 12px 0;
	border-bottom: 1px solid #f5f5f5;
}

.info-item:nth-last-child(-n + 2) {
	border-bottom: none;
}

.info-item:nth-child(odd) {
	padding-right: 20px;
}

.info-item:nth-child(even) {
	padding-left: 20px;
	border-left: 1px solid #f5f5f5;
}

.info-label {
	font-size: 13px;
	color: #909399;
}

.info-value {
	font-size: 13px;
	font-weight: 500;
}

/* 快捷操作 */
.quick-actions {
	display: grid;
	grid-template-columns: repeat(3, 1fr);
	gap: 16px;
}

.action-item {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 10px;
	padding: 18px 8px;
	border-radius: 10px;
	cursor: pointer;
	transition: all 0.2s;
	position: relative;
}

.action-item:hover {
	background: #f5f7fa;
	transform: translateY(-2px);
}

.action-item:hover .action-delete {
	opacity: 1;
}

.action-icon {
	width: 48px;
	height: 48px;
	border-radius: 12px;
	display: flex;
	align-items: center;
	justify-content: center;
}

.action-label {
	font-size: 13px;
	color: #606266;
	font-weight: 500;
}

.action-delete {
	position: absolute;
	top: 4px;
	right: 4px;
	opacity: 0;
	color: #f56c6c;
	cursor: pointer;
	transition: opacity 0.2s;
}

.action-delete:hover {
	color: #f56c6c;
	transform: scale(1.2);
}

.action-add {
	border: 1px dashed #dcdfe6;
}

.action-add:hover {
	border-color: #409eff;
}

/* 图标选择器 */
.icon-picker {
	display: grid;
	grid-template-columns: repeat(8, 1fr);
	gap: 8px;
	max-height: 200px;
	overflow-y: auto;
	padding: 4px;
}

.icon-option {
	width: 36px;
	height: 36px;
	display: flex;
	align-items: center;
	justify-content: center;
	border: 1px solid #ebeef5;
	border-radius: 6px;
	cursor: pointer;
	transition: all 0.2s;
}

.icon-option:hover {
	border-color: #409eff;
	background: rgba(64, 158, 255, 0.05);
}

.icon-option.active {
	border-color: #409eff;
	background: rgba(64, 158, 255, 0.1);
	box-shadow: 0 0 0 1px #409eff;
}

/* 公告卡片 */
.notice-card {
	margin-top: 20px;
	width: 100%;
}

.notice-list {
	display: flex;
	flex-direction: column;
	gap: 14px;
}

.notice-item {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 10px;
	font-size: 13px;
	padding: 8px 10px;
	border-radius: 6px;
	cursor: pointer;
	transition: background 0.2s;
}

.notice-item:hover {
	background: #f5f7fa;
}

.notice-row {
	display: flex;
	align-items: center;
	gap: 8px;
	flex: 1;
	min-width: 0;
}

.notice-dot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	background: #f56c6c;
	flex-shrink: 0;
}

.notice-text {
	flex: 1;
	color: #606266;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.notice-time {
	font-size: 12px;
	color: #c0c4cc;
	white-space: nowrap;
}

.empty-notice {
	text-align: center;
	color: #909399;
	padding: 20px;
}

/* 公告详情弹窗 */
.notice-detail-content {
	font-size: 14px;
	line-height: 1.8;
	color: #303133;
	white-space: pre-wrap;
}

.notice-detail-time {
	margin-top: 16px;
	font-size: 12px;
	color: #c0c4cc;
	text-align: right;
}

/* 响应式 */
@media (max-width: 768px) {
	.welcome-section {
		flex-direction: column;
		align-items: flex-start;
		gap: 10px;
	}

	.info-list {
		grid-template-columns: 1fr;
	}

	.info-item:nth-child(even) {
		padding-left: 0;
		border-left: none;
	}

	.info-item:nth-child(odd) {
		padding-right: 0;
	}

	.quick-actions {
		grid-template-columns: repeat(3, 1fr);
	}

	.icon-picker {
		grid-template-columns: repeat(6, 1fr);
	}
}
</style>
