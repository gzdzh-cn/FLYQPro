<template>
	<el-dialog
		v-model="dialogVisible"
		title="搜索客户"
		width="800px"
		:close-on-click-modal="false"
		class="adv-search-dialog"
		@close="onClose"
	>
		<el-form :model="form" label-width="100px" label-position="right" class="adv-search-form">
			<!-- 关键字搜索（独立在顶部居中） -->
			<div class="keyword-row" v-if="showKeyWord">
				<span class="keyword-label">关键词</span>
				<el-input v-model="form.keyWord" placeholder="关键词" clearable size="large" style="max-width: 360px;" />
			</div>

			<el-collapse v-model="activeGroups">
				<!-- 第一组：联系方式 -->
				<el-collapse-item title="联系方式" name="contact">
					<div class="form-grid">
						<el-form-item label="手机号" v-if="showMobile">
							<el-input v-model="form.mobile" placeholder="请输入手机号" clearable />
						</el-form-item>
						<el-form-item label="微信" v-if="showWechat">
							<el-input v-model="form.wechat" placeholder="请输入微信号" clearable />
						</el-form-item>
					</div>
				</el-collapse-item>

				<!-- 第二组：客户信息 -->
				<el-collapse-item title="客户信息" name="customer">
					<div class="form-grid">
						<template v-for="item in resolvedTagItems" :key="item.prop">
							<el-form-item :label="item.label">
								<!-- 层级结构：el-tree-select -->
								<el-tree-select
									v-if="item.isHierarchical"
									v-model="form[item.prop]"
									:data="item.treeData"
									:props="{ value: 'id', label: 'label', children: 'children' }"
									check-strictly
									:render-after-expand="false"
									:multiple="item.isMulti"
									collapse-tags
									collapse-tags-tooltip
									clearable
									filterable
									default-expand-all
									placeholder="请选择"
								/>
								<!-- 普通下拉：el-select -->
								<el-select
									v-else
									v-model="form[item.prop]"
									:multiple="item.isMulti"
									clearable
									placeholder="请选择"
								>
									<el-option
										v-for="opt in item.options"
										:key="opt.value"
										:label="opt.label"
										:value="opt.value"
									/>
								</el-select>
							</el-form-item>
						</template>

						<el-form-item label="客服组" v-if="showServiceGroup">
							<el-select v-model="form.serviceGroupStatus" clearable placeholder="请选择">
								<el-option
									v-for="item in serviceGroupOptions"
									:key="item.value"
									:label="item.label"
									:value="item.value"
								/>
							</el-select>
						</el-form-item>
						<el-form-item label="客服分配状态" v-if="showServiceStatus">
							<el-select v-model="form.serviceStatus" clearable placeholder="请选择">
								<el-option label="未分配" :value="1" />
								<el-option label="已分配" :value="2" />
							</el-select>
						</el-form-item>
					</div>
				</el-collapse-item>

				<!-- 第三组：其他内容 -->
				<el-collapse-item title="其他内容" name="other">
					<div class="form-grid">
						<el-form-item label="53标识" v-if="showSerialId">
							<el-input v-model="form.guestId" placeholder="请输入53标识" clearable />
						</el-form-item>

						<el-form-item label="最后跟进时间" v-if="showLastFollowupTime">
							<el-date-picker
								v-model="form.lastFollowupTimeRange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="下次跟进时间" v-if="showNextFollowTime">
							<el-date-picker
								v-model="form.nextFollowTimeRange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="录入时间" v-if="showCreateTime">
							<el-date-picker
								v-model="form.datetimerange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="最后修改时间" v-if="showUpdateTime">
							<el-date-picker
								v-model="form.updatetimerange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="分配时间" v-if="showAllotTime">
							<el-date-picker
								v-model="form.allotTimeRange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="首次成交时间" v-if="showFirstDealTime">
							<el-date-picker
								v-model="form.firstDealTimeRange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="最后成交时间" v-if="showLastDealTime">
							<el-date-picker
								v-model="form.lastDealTimeRange"
								type="datetimerange"
								:shortcuts="shortcuts"
								start-placeholder="开始日期"
								end-placeholder="结束日期"
								value-format="YYYY-MM-DD HH:mm"
								time-format="HH:mm"
								style="width: 100%"
							/>
						</el-form-item>

						<el-form-item label="跟进次数" v-if="showFollowCount">
							<div style="display: flex; gap: 8px;">
								<el-select v-model="form.followCountOp" style="width: 120px; flex-shrink: 0;" placeholder="条件">
									<el-option label="大于" value="gt" />
									<el-option label="大于或等于" value="gte" />
									<el-option label="等于" value="eq" />
									<el-option label="小于" value="lt" />
									<el-option label="小于或等于" value="lte" />
								</el-select>
								<el-input-number
									v-model="form.followCountValue"
									:min="0"
									:controls="false"
									placeholder="次数"
									style="flex: 1;"
								/>
							</div>
						</el-form-item>

						<el-form-item label="成交次数" v-if="showDealCount">
							<div style="display: flex; gap: 8px;">
								<el-select v-model="form.dealCountOp" style="width: 120px; flex-shrink: 0;" placeholder="条件">
									<el-option label="大于" value="gt" />
									<el-option label="大于或等于" value="gte" />
									<el-option label="等于" value="eq" />
									<el-option label="小于" value="lt" />
									<el-option label="小于或等于" value="lte" />
								</el-select>
								<el-input-number
									v-model="form.dealCountValue"
									:min="0"
									:controls="false"
									placeholder="次数"
									style="flex: 1;"
								/>
							</div>
						</el-form-item>

						<el-form-item label="参与人数量" v-if="showParticipantCount">
							<div style="display: flex; gap: 8px;">
								<el-select v-model="form.participantCountOp" style="width: 120px; flex-shrink: 0;" placeholder="条件">
									<el-option label="大于" value="gt" />
									<el-option label="大于或等于" value="gte" />
									<el-option label="等于" value="eq" />
									<el-option label="小于" value="lt" />
									<el-option label="小于或等于" value="lte" />
								</el-select>
								<el-input-number
									v-model="form.participantCountValue"
									:min="0"
									:controls="false"
									placeholder="数量"
									style="flex: 1;"
								/>
							</div>
						</el-form-item>

						<el-form-item label="参与人" v-if="showParticipant">
							<div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap;">
								<div v-if="selectedParticipantNames.length" style="display: flex; flex-wrap: wrap; gap: 4px;">
									<el-tag v-for="name in selectedParticipantNames" :key="name" closable @close="removeParticipant(name)" size="small">{{ name }}</el-tag>
								</div>
								<el-button type="primary" link @click="openParticipantDialog">选择</el-button>
							</div>
						</el-form-item>
					</div>
				</el-collapse-item>
			</el-collapse>
		</el-form>

		<template #footer>
			<div class="adv-search-footer">
				<el-button type="primary" @click="onSearch">搜索</el-button>
				<el-button @click="onReset">重置</el-button>
				<el-button @click="onClose">关闭</el-button>
			</div>
		</template>
	</el-dialog>

	<!-- 参与人选择弹窗 -->
	<el-dialog
		v-model="participantSelectVisible"
		title="选择参与人"
		width="750px"
		append-to-body
		class="select-person-dialog"
		:close-on-click-modal="false"
	>
		<div class="select-person-layout">
			<div class="dept-panel">
				<div class="dept-header">部门</div>
				<el-scrollbar class="dept-list">
					<ul>
						<li :class="{ 'is-active': activeGroupId === null }" @click="activeGroupId = null">全部</li>
						<li v-for="g in groupList" :key="g.id" :class="{ 'is-active': activeGroupId === g.id }" @click="activeGroupId = g.id">{{ g.name }}</li>
					</ul>
				</el-scrollbar>
			</div>
			<div class="kf-panel">
				<div class="kf-header">客服人员</div>
				<el-scrollbar class="kf-list">
					<el-checkbox-group v-model="tempSelectedIds">
						<div v-for="kf in filteredKfList" :key="kf.userId" class="kf-item">
							<el-checkbox :label="kf.userId">
								<div class="kf-info">
									<el-avatar :size="22" :src="''"><el-icon><UserFilled /></el-icon></el-avatar>
									<span class="name">{{ kf.name }}</span>
								</div>
							</el-checkbox>
						</div>
					</el-checkbox-group>
					<el-empty v-if="!filteredKfList.length" :image-size="60" description="暂无客服人员" />
				</el-scrollbar>
			</div>
			<div class="selected-panel">
				<div class="selected-header">已选 ({{ tempSelectedKfList.length }})</div>
				<el-scrollbar class="selected-list">
					<div v-for="kf in tempSelectedKfList" :key="kf.userId" class="selected-item">
						<el-avatar :size="22" :src="''"><el-icon><UserFilled /></el-icon></el-avatar>
						<span class="name">{{ kf.name }}</span>
						<el-icon class="remove-icon" @click="removeTempSelected(kf.userId)"><Close /></el-icon>
					</div>
					<el-empty v-if="!tempSelectedKfList.length" :image-size="60" description="暂未选择" />
				</el-scrollbar>
			</div>
		</div>
		<template #footer>
			<el-button @click="participantSelectVisible = false">取消</el-button>
			<el-button type="primary" @click="confirmParticipantSelect">确定</el-button>
		</template>
	</el-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, watch, computed } from "vue";
import { UserFilled, Close } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { useCool } from "/@/cool";

const { service } = useCool();

interface TagTypeItem {
	label: string;
	prop: string;
	isHierarchical: boolean;
	isMulti: boolean;
	options: { label: string; value: any }[];
	treeData?: any[];
}

interface Props {
	modelValue: boolean;
	serviceGroup?: any[];
	tagTypeItems?: any[];
	showMobile?: boolean;
	showWechat?: boolean;
	showKeyWord?: boolean;
	showServiceGroup?: boolean;
	showServiceStatus?: boolean;
	showSerialId?: boolean;
	showLastFollowupTime?: boolean;
	showNextFollowTime?: boolean;
	showCreateTime?: boolean;
	showUpdateTime?: boolean;
	showAllotTime?: boolean;
	showFirstDealTime?: boolean;
	showLastDealTime?: boolean;
	showFollowCount?: boolean;
	showDealCount?: boolean;
	showParticipantCount?: boolean;
	showParticipant?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
	showMobile: true,
	showWechat: true,
	showKeyWord: true,
	showServiceGroup: true,
	showServiceStatus: true,
	showSerialId: true,
	showLastFollowupTime: true,
	showNextFollowTime: true,
	showCreateTime: true,
	showUpdateTime: true,
	showAllotTime: true,
	showFirstDealTime: true,
	showLastDealTime: true,
	showFollowCount: true,
	showDealCount: true,
	showParticipantCount: true,
	showParticipant: true,
});

const emit = defineEmits<{
	(e: "update:modelValue", value: boolean): void;
	(e: "search", params: Record<string, any>): void;
	(e: "reset"): void;
}>();

const dialogVisible = computed({
	get: () => props.modelValue,
	set: (val: boolean) => emit("update:modelValue", val),
});

const activeGroups = ref(["contact", "customer", "other"]);

// 客服组选项
const serviceGroupOptions = computed(() => props.serviceGroup || []);

// 解析后的标签搜索项
const resolvedTagItems = computed<TagTypeItem[]>(() => {
	if (!props.tagTypeItems) return [];
	return props.tagTypeItems.map((itemFn: any) => {
		const item = typeof itemFn === "function" ? itemFn() : itemFn;
		const isHierarchical = item.component?.name === "el-tree-select";
		const isMulti = item.component?.props?.multiple || false;
		return {
			label: item.label || "",
			prop: item.prop || "",
			isHierarchical,
			isMulti,
			options: item.component?.options || [],
			treeData: item.component?.props?.data || [],
		};
	});
});

// 表单数据
const form = reactive<Record<string, any>>({
	keyWord: "",
	mobile: "",
	wechat: "",
	guestId: "",
	serviceGroupStatus: "",
	serviceStatus: "",
	datetimerange: null,
	updatetimerange: null,
	lastFollowupTimeRange: null,
	nextFollowTimeRange: null,
	allotTimeRange: null,
	firstDealTimeRange: null,
	lastDealTimeRange: null,
	followCountOp: "",
	followCountValue: undefined as number | undefined,
	dealCountOp: "",
	dealCountValue: undefined as number | undefined,
	participantCountOp: "",
	participantCountValue: undefined as number | undefined,
	participantIds: [] as string[],
});

// 时间快捷选项
const shortcuts = [
	{
		text: "当天",
		value: () => {
			const end = new Date();
			end.setDate(end.getDate() + 1);
			end.setHours(0, 0, 0, 0);
			const start = new Date();
			start.setHours(0, 0, 0, 0);
			return [start, end];
		},
	},
	{
		text: "昨天",
		value: () => {
			const end = new Date();
			end.setHours(0, 0, 0, 0);
			const start = new Date();
			start.setDate(start.getDate() - 1);
			start.setHours(0, 0, 0, 0);
			return [start, end];
		},
	},
	{
		text: "最近一周",
		value: () => {
			const end = new Date();
			end.setDate(end.getDate() + 1);
			end.setHours(0, 0, 0, 0);
			const start = new Date();
			start.setDate(start.getDate() - 7);
			start.setHours(0, 0, 0, 0);
			return [start, end];
		},
	},
	{
		text: "最近一个月",
		value: () => {
			const end = new Date();
			end.setDate(end.getDate() + 1);
			end.setHours(0, 0, 0, 0);
			const start = new Date();
			start.setMonth(start.getMonth() - 1);
			start.setHours(0, 0, 0, 0);
			return [start, end];
		},
	},
	{
		text: "最近三个月",
		value: () => {
			const end = new Date();
			end.setDate(end.getDate() + 1);
			end.setHours(0, 0, 0, 0);
			const start = new Date();
			start.setMonth(start.getMonth() - 3);
			start.setHours(0, 0, 0, 0);
			return [start, end];
		},
	},
];

// 重置表单
function resetForm() {
	form.keyWord = "";
	form.mobile = "";
	form.wechat = "";
	form.guestId = "";
	form.serviceGroupStatus = "";
	form.serviceStatus = "";
	form.datetimerange = null;
	form.updatetimerange = null;
	form.lastFollowupTimeRange = null;
	form.nextFollowTimeRange = null;
	form.allotTimeRange = null;
	form.firstDealTimeRange = null;
	form.lastDealTimeRange = null;
	form.followCountOp = "";
	form.followCountValue = undefined;
	form.dealCountOp = "";
	form.dealCountValue = undefined;
	form.participantCountOp = "";
	form.participantCountValue = undefined;
	form.participantIds = [];
	selectedParticipantList.value = [];

	// 清空动态标签字段
	for (const key in form) {
		if (key.endsWith("Status")) {
			form[key] = Array.isArray(form[key]) ? [] : "";
		}
	}
}

// 搜索
function onSearch() {
	// 校验数值字段必须大于0
	const numberFields: { key: string; opKey: string; label: string }[] = [
		{ key: "followCountValue", opKey: "followCountOp", label: "跟进次数" },
		{ key: "dealCountValue", opKey: "dealCountOp", label: "成交次数" },
		{ key: "participantCountValue", opKey: "participantCountOp", label: "参与人数量" },
	];
	for (const field of numberFields) {
		const val = (form as any)[field.key];
		const op = (form as any)[field.opKey];
		// 选择了操作符但未填值，或值不大于0
		if (op && (val === "" || val === null || val === undefined)) {
			ElMessage.warning(`请填写${field.label}`);
			return;
		}
		if (val !== "" && val !== null && val !== undefined && val <= 0) {
			ElMessage.warning(`${field.label}必须大于0`);
			return;
		}
	}

	const params: Record<string, any> = {};

	// 收集非空字段
	for (const [key, value] of Object.entries(form)) {
		if (value === "" || value === null || value === undefined) continue;
		if (Array.isArray(value) && value.length === 0) continue;
		// 文本字段去除所有空格（手机号等中间可能有空格）
		if (typeof value === "string") {
			const trimmed = value.replace(/\s+/g, "");
			if (trimmed === "") continue;
			params[key] = trimmed;
		} else {
			params[key] = value;
		}
	}

	// 清空动态标签空数组
	for (const key in params) {
		if (key.endsWith("Status") && Array.isArray(params[key]) && params[key].length === 0) {
			delete params[key];
		}
	}

	// 无搜索条件时也走 search 逻辑
	emit("search", params);
	dialogVisible.value = false;
}

// 重置
function onReset() {
	resetForm();
	emit("reset");
}

// 关闭（只关闭弹窗，不改变搜索状态，不刷新列表）
function onClose() {
	dialogVisible.value = false;
}

// 监听 tagTypeItems 变化，初始化动态标签字段
watch(
	() => props.tagTypeItems,
	(items) => {
		if (!items) return;
		for (const itemFn of items) {
			const item = typeof itemFn === "function" ? itemFn() : itemFn;
			if (item.prop && !(item.prop in form)) {
				form[item.prop] = item.component?.props?.multiple ? [] : "";
			}
		}
	},
	{ immediate: true, deep: true }
);

// ===== 参与人选择相关 =====
const selectedParticipantList = ref<{ userId: string; name: string }[]>([]);
const selectedParticipantNames = computed(() => selectedParticipantList.value.map((p) => p.name));

// 参与人选择弹窗
const participantSelectVisible = ref(false);
const allKfList = ref<any[]>([]);
const groupList = ref<any[]>([]);
const activeGroupId = ref<string | null>(null);
const tempSelectedIds = ref<string[]>([]);

const filteredKfList = computed(() => {
	if (activeGroupId.value === null) return allKfList.value;
	return allKfList.value.filter((kf: any) => kf.groupId === activeGroupId.value);
});

const tempSelectedKfList = computed(() => {
	return tempSelectedIds.value
		.map((uid) => allKfList.value.find((kf: any) => kf.userId === uid))
		.filter(Boolean) as any[];
});

function removeTempSelected(userId: string) {
	const idx = tempSelectedIds.value.indexOf(userId);
	if (idx > -1) tempSelectedIds.value.splice(idx, 1);
}

async function openParticipantDialog() {
	// 加载客服列表
	try {
		const list = await service.customer_pro.kf.list({});
		allKfList.value = list || [];
	} catch {
		allKfList.value = [];
	}
	try {
		const list = await service.customer_pro.project_group.list({});
		groupList.value = list || [];
	} catch {
		groupList.value = [];
	}
	tempSelectedIds.value = selectedParticipantList.value.map((p) => p.userId);
	activeGroupId.value = null;
	participantSelectVisible.value = true;
}

function confirmParticipantSelect() {
	selectedParticipantList.value = tempSelectedIds.value
		.map((uid) => {
			const kf = allKfList.value.find((k: any) => k.userId === uid);
			return kf ? { userId: kf.userId, name: kf.name } : null;
		})
		.filter(Boolean) as { userId: string; name: string }[];
	form.participantIds = selectedParticipantList.value.map((p) => p.userId);
	participantSelectVisible.value = false;
}

function removeParticipant(name: string) {
	selectedParticipantList.value = selectedParticipantList.value.filter((p) => p.name !== name);
	form.participantIds = selectedParticipantList.value.map((p) => p.userId);
}
</script>

<style lang="scss" scoped>
.form-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 0 16px;
}

.keyword-row {
	display: flex;
	justify-content: center;
	align-items: center;
	gap: 8px;
	margin-bottom: 16px;
}

.keyword-label {
	font-size: 14px;
	font-weight: 600;
	color: var(--el-text-color-regular);
	white-space: nowrap;
}

.adv-search-footer {
	display: flex;
	justify-content: center;
	gap: 12px;
}
</style>

<style lang="scss">
.adv-search-dialog {
	.el-dialog__body {
		padding-top: 10px;
		padding-bottom: 0;
		max-height: 60vh !important;
		overflow-y: auto !important;
	}

	.el-collapse {
		border: none;
	}

	.el-collapse-item__header {
		font-weight: 600;
		font-size: 14px;
		background: transparent;
		border-bottom: 1px solid var(--el-border-color-lighter);
	}

	.el-collapse-item__wrap {
		border-bottom: none;
	}

	.el-collapse-item__content {
		padding-bottom: 8px;
	}

	.adv-search-form .el-form-item {
		margin-bottom: 12px;
	}

	.adv-search-form .el-form-item__content {
		flex: 1;
		min-width: 0;
	}

	.el-date-editor {
		width: 100% !important;
	}
}

.select-person-dialog {
	border-radius: 12px;
	overflow: hidden;

	.el-dialog__header {
		padding: 16px 20px;
		margin: 0;
		border-bottom: 1px solid #f0f1f5;
		background: linear-gradient(180deg, #fafbfd 0%, #ffffff 100%);

		.el-dialog__title {
			font-size: 15px;
			font-weight: 600;
			color: #1f2329;
		}
	}

	.el-dialog__body {
		padding: 0 !important;
	}

	.el-dialog__footer {
		padding: 12px 20px;
		border-top: 1px solid #f0f1f5;
	}

	.select-person-layout {
		display: flex;
		height: 460px;

		.dept-panel {
			width: 180px;
			min-width: 180px;
			border-right: 1px solid #f0f1f5;
			display: flex;
			flex-direction: column;
			background: #fafbfd;

			.dept-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
			}

			.dept-list {
				flex: 1;

				ul {
					padding: 10px 8px;
					margin: 0;
					list-style: none;

					li {
						display: flex;
						align-items: center;
						padding: 9px 14px;
						margin-bottom: 4px;
						font-size: 13px;
						color: #4e5969;
						border-radius: 6px;
						cursor: pointer;
						transition: all 0.18s ease;

						&::before {
							content: "";
							display: inline-block;
							width: 4px;
							height: 4px;
							border-radius: 50%;
							background: #c0c4cc;
							margin-right: 10px;
							transition: all 0.18s;
						}

						&:hover {
							background: #f0f4ff;
							color: var(--color-primary);

							&::before {
								background: var(--color-primary);
							}
						}

						&.is-active {
							background: var(--color-primary);
							color: #fff;
							font-weight: 500;
							box-shadow: 0 2px 8px rgba(64, 158, 255, 0.25);

							&::before {
								background: #fff;
							}
						}
					}
				}
			}
		}

		.kf-panel {
			flex: 1;
			border-right: 1px solid #f0f1f5;
			display: flex;
			flex-direction: column;
			background: #fff;

			.kf-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
			}

			.kf-list {
				flex: 1;
				padding: 8px 12px;

				.kf-item {
					padding: 6px 0;

					.kf-info {
						display: inline-flex;
						align-items: center;
						gap: 6px;

						.name {
							font-size: 13px;
							color: #333;
						}
					}
				}
			}
		}

		.selected-panel {
			width: 200px;
			min-width: 200px;
			display: flex;
			flex-direction: column;
			background: #fafbfd;

			.selected-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
			}

			.selected-list {
				flex: 1;
				padding: 8px 12px;

				.selected-item {
					display: flex;
					align-items: center;
					gap: 6px;
					padding: 8px 6px;
					border-bottom: 1px solid #f0f1f5;

					.name {
						flex: 1;
						font-size: 13px;
						color: #333;
						overflow: hidden;
						text-overflow: ellipsis;
						white-space: nowrap;
					}

					.remove-icon {
						flex-shrink: 0;
						cursor: pointer;
						color: #909399;
						font-size: 14px;
						transition: color 0.2s;

						&:hover {
							color: #f56c6c;
						}
					}
				}
			}
		}
	}
}
</style>
