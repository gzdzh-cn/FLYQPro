<template>
	<cl-crud ref="Crud">
		<!-- 负责人/参与人快捷筛选 -->
		<cl-row>
			<el-button-group class="owner-filter-group">
				<el-button size="small" :type="ownerFilter === '' ? 'primary' : 'default'"
					@click="toggleOwnerFilter('')">
					全部
				</el-button>
				<el-button size="small" :type="ownerFilter === 'owner' ? 'primary' : 'default'"
					@click="toggleOwnerFilter('owner')">
					我负责的
				</el-button>
				<el-button size="small" :type="ownerFilter === 'participant' ? 'primary' : 'default'"
					@click="toggleOwnerFilter('participant')">
					我参与的
				</el-button>
			</el-button-group>

			<el-button-group class="quick-filter-group">
				<el-button size="small" :type="quickFilter === '' ? 'primary' : 'default'"
					@click="toggleQuickFilter('')">
					全部
				</el-button>
				<el-button size="small" :type="quickFilter === 'today' ? 'primary' : 'default'"
					@click="toggleQuickFilter('today')">
					今日新增
				</el-button>
				<el-button size="small" :type="quickFilter === 'yesterday' ? 'primary' : 'default'"
					@click="toggleQuickFilter('yesterday')">
					昨日新增
				</el-button>
				<el-button size="small" :type="quickFilter === 'dealt' ? 'primary' : 'default'"
					@click="toggleQuickFilter('dealt')">
					已成交
				</el-button>
				<el-button size="small" :type="quickFilter === 'pushed' ? 'primary' : 'default'"
					@click="toggleQuickFilter('pushed')">
					已推出
				</el-button>
			</el-button-group>
		</cl-row>

		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn size="small" />
			<!-- 新增按钮 -->
			<el-button type="primary" size="small" @click="editInfoRef?.openAdd()">新增</el-button>
			<!-- 删除按钮 -->
			<cl-multi-delete-btn size="small" />

			<el-dropdown trigger="hover" @command="onMoreCommand">
				<el-button plain size="small">
					更多<el-icon class="el-icon--right">
						<ArrowDown />
					</el-icon>
				</el-button>
				<template #dropdown>
					<el-dropdown-menu>
						<el-dropdown-item command="excel" v-if="service.customer_pro.clues._permission.excel">
							导出Excel
						</el-dropdown-item>
						<el-dropdown-item command="distribute" :disabled="Table?.selection.length == 0"
							v-if="service.customer_pro.clues._permission.distribute">
							转交
							<el-tooltip content="请先勾选至少一条线索，再进行转交操作" placement="right">
								<el-icon style="margin-left: 4px; color: #909399;">
									<QuestionFilled />
								</el-icon>
							</el-tooltip>
						</el-dropdown-item>
					</el-dropdown-menu>
				</template>
			</el-dropdown>

			<el-button plain size="small" @click="columnDialogVisible = true">
				<el-icon style="margin-right: 4px;">
					<Setting />
				</el-icon>显示列
			</el-button>

			<!-- 排序按钮 -->
			<el-dropdown trigger="click" @command="onSortCommand">
				<el-button plain size="small">
					<el-icon style="margin-right: 4px;">
						<Rank />
					</el-icon>
					{{ sortFieldLabel ? sortFieldLabel + (sortOrder === 'ASC' ? ' ↑' : sortOrder === 'DESC' ? ' ↓' : '') : '排序' }}
				</el-button>
				<template #dropdown>
					<el-dropdown-menu class="sort-dropdown-menu">
						<el-dropdown-item class="sort-group-title" disabled>排序字段</el-dropdown-item>
						<el-dropdown-item v-for="item in sortFieldOptions" :key="item.value"
							:command="'field:' + item.value"
							:class="{ 'is-active': sortField === item.value }">
							{{ item.label }}
							<el-icon v-if="sortField === item.value" style="margin-left: auto;"><Check /></el-icon>
						</el-dropdown-item>
						<el-dropdown-item divided class="sort-group-title" disabled>排序方式</el-dropdown-item>
						<el-dropdown-item command="order:ASC"
							:class="{ 'is-active': sortOrder === 'ASC' }">
							正序 ↑
							<el-icon v-if="sortOrder === 'ASC'" style="margin-left: auto;"><Check /></el-icon>
						</el-dropdown-item>
						<el-dropdown-item command="order:DESC"
							:class="{ 'is-active': sortOrder === 'DESC' }">
							倒序 ↓
							<el-icon v-if="sortOrder === 'DESC'" style="margin-left: auto;"><Check /></el-icon>
						</el-dropdown-item>
						<el-dropdown-item command="order:default"
							:class="{ 'is-active': sortOrder === '' && sortField === '' }">
							默认
							<el-icon v-if="sortOrder === '' && sortField === ''" style="margin-left: auto;"><Check /></el-icon>
						</el-dropdown-item>
					</el-dropdown-menu>
				</template>
			</el-dropdown>

			<cl-flex1 />

			<el-button type="info" text bg size="small" v-show="searchStatus">
				正在搜索中
			</el-button>
			<el-button type="info" text bg size="small" :icon="Search" @click="advSearchVisible = true">
				搜索
			</el-button>
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="true" @header-dragend="onHeaderDragend" @row-dblclick="onRowDblclick">
				<template #column-status="{ scope }">
					<span style="color: #d83b01" v-if="scope.row.status == 0"> 未成交 </span>
					<span style="color: #00b294" v-if="scope.row.status == 1"> 已成交 </span>
				</template>
				<template #column-label="{ scope }">
					<div class="label-tags" v-if="getRowTags(scope.row).length">
						<el-tag v-for="(tag, idx) in getRowTags(scope.row)" :key="idx" effect="plain" size="small"
							:style="tag.style" :title="tag.typeName">{{ tag.text }}</el-tag>
					</div>
				</template>
				<template #column-mobile="{ scope }">
					<el-popover v-if="scope.row.mobile" trigger="hover" placement="bottom" :teleported="true"
						popper-class="mobile-popover" :width="'auto'">
						<template #reference>
							<span class="mobile-text" :title="scope.row.mobile">
								{{ formatMobile(scope.row.mobile) }}
							</span>
						</template>
						<div class="mobile-actions-inline">
							<el-icon :size="20" @click.stop="copyMobile(scope.row.mobile)">
								<CopyDocument />
							</el-icon>
							<span class="remote-call-anchor" @pointerdown.stop @click.stop="callMobile(scope.row, $event)">
								<el-icon :size="20" :class="{ 'is-calling': calling }"><Phone /></el-icon>
							</span>
						</div>
					</el-popover>
					<span v-else class="mobile-text">{{ formatMobile(scope.row.mobile) }}</span>
				</template>
				<template #slot-op="{ scope }">
					<div style="
							display: flex;
							flex-direction: row;
							flex-wrap: wrap;
							align-items: center;
							gap: 12px;
						">
						<!-- <el-button
							text
							bg
							type="primary"
							@click="edit(scope.row)"
							v-if="
								scope.row.status == 0 &&
								service.customer_pro.clues._permission.update
							"
							>审核</el-button
						> -->

						<el-button text bg type="info" @click="openFollow(scope.row)">跟进</el-button>
						<el-button text bg type="warning" @click="openTracks(scope.row)"
							v-if="service.customer_pro.clues._permission.getTrackList">轨迹</el-button>
						<el-button text bg type="success" @click="openOrderAdd(scope.row)" v-if="
							scope.row.status == 0 && service.customer_pro.order._permission.add
						">成交</el-button>
					</div>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination :pager-count="browser.isMini ? 3 : 7" :layout="browser.isMini
				? 'slot,total, prev, pager, next'
				: 'slot,total, sizes, prev, pager, next, jumper'
				" />
		</cl-row>

		<!-- 项目分配 -->
		<cl-form ref="distributeFormRef">
			<template #slot-projectId="{ scope }">
				<el-select v-model="scope.projectId" @change="projectChange">
					<el-option v-for="item in projectList" :key="item.value" :label="item.name" :value="item.id" />
				</el-select>
			</template>

			<template #slot-groupId="{ scope }">
				<el-select v-model="scope.groupId" @change="groupChange">
					<el-option v-for="item in groupList" :key="item.value" :label="item.name" :value="item.id" />
				</el-select>
			</template>

			<template #slot-servicesId="{ scope }">
				<el-select v-model="scope.servicesId">
					<el-option v-for="item in kfList" :key="item.value" :label="item.name" :value="item.userId" />
				</el-select>
			</template>
		</cl-form>

		<!-- 跟进弹窗 -->
		<cl-dialog title="跟进" v-model="visible">
			<sub-follow ref="FollowRef" :id="cluesId" :status="cluesStatus" dtype="clues" @cancel="cancel" />
		</cl-dialog>

		<!-- 轨迹弹窗 -->
		<cl-dialog title="轨迹" v-model="trackVisible">
			<sub-track ref="TrackRef" :id="cluesId" />
			<template #footer>
				<div class="dialog-footer">
					<el-button @click="trackVisible = false">关闭</el-button>
				</div>
			</template>
		</cl-dialog>

		<!-- 成交弹窗 -->
		<cl-form ref="OrderFormRef">
			<!-- 学校 -->
			<template #slot-schoolId="{ scope }">
				<el-select v-model="scope.schoolId" @change="schoolChange">
					<el-option v-for="item in schoolList" :key="item.value" :label="item.name" :value="item.id" />
				</el-select>
			</template>

			<!-- 专业 -->
			<template #slot-majorsId="{ scope }">
				<el-select v-model="scope.majorsId">
					<el-option v-for="item in majorsList" :key="item.value" :label="item.name" :value="item.id" />
				</el-select>
			</template>
		</cl-form>

		<!-- 高级搜索弹窗 -->
		<AdvSearchDialog v-model="advSearchVisible" :serviceGroup="serviceGroup" :tagTypeItems="tagTypeSearchItems"
			:showServiceGroup="true" :showServiceStatus="true" :showUpdateTime="true"
			@search="onAdvSearch" @reset="onAdvSearchReset" />

		<cl-dialog title="导出Excel" v-model="openExcel">
			<div class="exportBox">
				<el-button plain @click="toexcel(true)" style="margin-right: 10px">
					导出当前页
				</el-button>
				<el-button plain @click="toexcel(false)" style="margin-right: 10px"
					v-loading.fullscreen.lock="fullscreenLoading">
					导出全部
				</el-button>
			</div>

			<div class="excel-table">
				<excel-down ref="excelDownRef" :cluesStatus="cluesStatus" :isAdmin="isAdmin" :dtype="dtype" />
			</div>
		</cl-dialog>

		<cl-dialog title="生成随机数据" v-model="randomDialog">
			<el-form :model="randomForm" label-width="auto" style="max-width: 600px">
				<el-form-item label="生成数量">
					<el-input v-model="randomForm.randomNum" />
				</el-form-item>
				<el-form-item label="年月份">
					<el-date-picker v-model="randomForm.dateTime" type="month" placeholder="日期"
						value-format="YYYY-MM" />
				</el-form-item>
				<el-form-item>
					<el-button type="primary" @click="randomData()">提交</el-button>
				</el-form-item>
			</el-form>
		</cl-dialog>


		<!-- 订单详情 -->
		<el-drawer v-model="cluesOpen" title="" direction="rtl" :size="drawerSize" :with-header="false"
			class="clues-drawer">
			<clues-info ref="cluesInfoRef" :cluesId="cluesId" :key="callKey" style="margin-top: -20px" @toggleFullscreen="toggleDrawerSize"
				@close="onCluesInfoClose" @tagTypesChanged="onTagTypesChanged" @remote-call="openRemoteCall" />
		</el-drawer>

		<remote-call-dialog ref="remoteCallDialogRef" @follow-saved="onRemoteFollowSaved" />

		<Teleport to="body">
			<div v-if="simCardVisible" class="sim-card-popper" :style="simCardStyle" @pointerdown.stop @click.stop>
				<div class="sim-card-popper-header">
					<div class="sim-card-popper-title">选择拨出号码</div>
					<button type="button" class="sim-card-popper-close" title="关闭" @click.stop="closeSimCardPicker">
						<el-icon><Close /></el-icon>
					</button>
				</div>
				<div v-if="!simCardOptions.length" class="sim-card-empty">暂未获取到手机 SIM 信息，请刷新移动端连接</div>
				<div
					v-for="sim in simCardOptions"
					:key="sim.slotIndex"
					class="sim-card-option"
					:class="{ unavailable: !sim.available }"
				>
					<span class="sim-card-slot">{{ sim.slotLabel || `卡${sim.slotIndex + 1}` }}</span>
					<span class="sim-card-details">
						<span class="sim-card-number">{{ sim.numberMasked || "号码未识别" }}</span>
						<span class="sim-card-carrier">{{ sim.carrierName || "运营商未识别" }}</span>
					</span>
					<button
						type="button"
						class="sim-card-action"
						:disabled="!sim.available"
						@click.stop="selectSimCard(sim)"
					>
						拨打
					</button>
				</div>
			</div>
		</Teleport>

		<!-- 新增/编辑弹窗 -->
		<editInfo ref="editInfoRef" @saved="refresh" />

		<!-- 显示列弹窗 -->
		<el-dialog v-model="columnDialogVisible" title="显示列设置" width="500px" :close-on-click-modal="false"
			@open="openColumnDialog" class="column-dialog">
			<div class="column-setting-tip">
				<el-icon>
					<InfoFilled />
				</el-icon>
				拖拽左侧图标调整列顺序，开关控制列的显示与隐藏
			</div>
			<div class="column-dialog-body">
				<div class="column-setting-list">
					<draggable v-model="columnSettings" item-key="prop" handle=".drag-handle" animation="200">
						<template #item="{ element }">
							<div class="column-setting-item">
								<el-icon class="drag-handle">
									<Rank />
								</el-icon>
								<el-switch v-model="element.visible" />
								<span class="column-label">{{ element.label }}</span>
								<el-checkbox v-if="element.isTagType" v-model="element.searchable" size="small"
									style="margin-left: auto;" label="搜索" />
								<el-switch v-model="element.showOverflow" active-text="溢出隐藏" inactive-text=""
									size="small" style="margin-left: 8px;" />
							</div>
						</template>
					</draggable>
				</div>
			</div>
			<template #footer>
				<el-button @click="resetColumnSettings">恢复默认</el-button>
				<el-button @click="columnDialogVisible = false">取消</el-button>
				<el-button type="primary" @click="applyColumnSettings">保存</el-button>
			</template>
		</el-dialog>

	</cl-crud>
</template>

<script lang="ts" name="customer_pro-clues" setup>
import { useCrud, useForm, useTable } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ElMessage, ElLoading, ElIcon } from "element-plus";
import { Search, CopyDocument, Phone, ArrowDown, QuestionFilled, Setting, Rank, InfoFilled, Check, Close } from "@element-plus/icons-vue";
import { onMounted, onUnmounted, ref, watch, h, nextTick, RendererElement, RendererNode, VNode, VNodeArrayChildren, computed } from "vue";
import draggable from "vuedraggable";
import SubFollow from "../components/clues/subFollow.vue";
import SubTrack from "../components/clues/subTrack.vue";
import excelDown from "../components/clues/excelDown.vue";
import CluesInfo from "../components/clues/info.vue";
import editInfo from "../components/clues/editInfo.vue";
import AdvSearchDialog from "../components/AdvSearchDialog.vue";
import RemoteCallDialog from "../components/clues/remoteCallDialog.vue";
import { useBase } from "/$/base";
import { useDictMeta, resolveTags, KNOWN_DB_FIELDS, toValueArray, parseLabelJson, buildDictTree, hasDictHierarchy } from "../utils/tagDict";
import { deviceState, onDeviceEvent, type SimCardInfo } from "/@/utils/deviceWs";

const { service } = useCool();
const { user } = useBase();
const { browser } = useCool();
const FollowRef = ref(); //跟进
const cluesId = ref(); //线索id
const cluesStatus = ref(0); //线索状态
const projectList = ref(); // 项目列表
const searchData = ref(); //搜索条件
const serviceGroup = ref(); // 客服组
const openExcel = ref(false); //打开导出弹窗
const dtype = ref(0);
const callKey = ref(0);
const editInfoRef = ref<InstanceType<typeof editInfo>>();
const remoteCallDialogRef = ref<InstanceType<typeof RemoteCallDialog>>();
const cluesInfoRef = ref<any>();
const followRefreshTimers: number[] = [];
const simCardVisible = ref(false);
const simCardTarget = ref<any>(null);
const simCardStyle = ref<Record<string, string>>({});
const simCardOptions = computed(() => deviceState.simCards || []);

const onRemoteFollowSaved = (savedCluesId: string | number) => {
	if (!cluesOpen.value || String(cluesId.value || "") !== String(savedCluesId || "")) return;
	// 新一轮跟进刷新开始前清理上一轮定时器，避免连续拨号时旧任务刷新新线索。
	followRefreshTimers.splice(0).forEach((timer) => window.clearTimeout(timer));
	const refresh = () => cluesInfoRef.value?.refreshFollow?.();
	refresh();
	// 用户关闭手机跟进页后，未接通记录由 call_session / 异步任务稍后落库；
	// 接通录音还可能等待 OSS 合并。分阶段刷新覆盖两种后台时序。
	followRefreshTimers.push(window.setTimeout(refresh, 1200));
	followRefreshTimers.push(window.setTimeout(refresh, 4000));
	followRefreshTimers.push(window.setTimeout(refresh, 8000));
	followRefreshTimers.push(window.setTimeout(refresh, 15000));
};

// ===== 标签类型映射（与 tagManager / editInfo 保持一致）=====
const TYPE_KEY_TO_FIELD: Record<string, string> = {
	cluesLevel: "level",
	sourceFrom: "sourceFrom",
	source_from: "sourceFrom",
	followupType: "followupType",
	followup_type: "followupType",
	householdType: "householdType",
	household_type: "householdType",
	education: "education"
};
const MULTI_SELECT_KEYS = new Set(["cluesLevel"]);

// 动态标签类型搜索项
const tagTypeSearchItems = ref<any[]>([]);

// ===== 动态列配置 =====
// 有自定义模板的列（这些列不会被动态列覆盖）
const SPECIAL_COLUMN_PROPS = new Set(["label", "mobile", "keywords"]);

// editInfo 中的固定字段 → table 列配置
const EDIT_INFO_FIXED_COLUMNS: { label: string; prop: string; width?: number; showOverflowTooltip?: boolean }[] = [
	{ label: "53标识", prop: "guestId", width: 180 },
	{ label: "项目", prop: "projectId", width: 120 },
	{ label: "姓名", prop: "name", width: 120 },
	{ label: "关键字", prop: "keywords", width: 120 },
	{ label: "手机号", prop: "mobile", width: 200 },
	{ label: "微信号", prop: "wechat", width: 120 },
	{ label: "毕业院校", prop: "graduatedSchool", width: 120, showOverflowTooltip: true },
	{ label: "意向院校", prop: "schoolId", width: 120 },
	{ label: "意向专业", prop: "majorsId", width: 120 },
	{ label: "报读类型", prop: "majorsType", width: 120 },
	{ label: "报读层次", prop: "degreeId", width: 120 },
	{ label: "户籍地址", prop: "householdAddress", width: 160, showOverflowTooltip: true },
	{ label: "性别", prop: "gender", width: 80 },
	{ label: "紧急联系人电话", prop: "emergencyMobile", width: 140 },
	{ label: "已推学校", prop: "schoolName", width: 120, showOverflowTooltip: true },
	{ label: "备注", prop: "remark", width: 160, showOverflowTooltip: true },
	{ label: "负责人", prop: "servicesName", width: 120, showOverflowTooltip: true },
	{ label: "参与人", prop: "servicesNames", width: 160, showOverflowTooltip: true },
	{ label: "来源", prop: "sourceFrom", width: 100 },
	{ label: "跟进方式", prop: "followupType", width: 100 },
	{ label: "客户等级", prop: "cluesLevel", width: 100 },
	{ label: "户口性质", prop: "householdType", width: 100 },
	{ label: "学员阶段", prop: "education", width: 100 },
	{ label: "IP", prop: "ip", width: 140 },
	{ label: "IP归属地", prop: "guestIpInfo", width: 160, showOverflowTooltip: true },
	{ label: "录入时间", prop: "createTime", width: 160 },
	{ label: "首次跟进时间", prop: "firstFollowTime", width: 160 },
	{ label: "首次跟进人", prop: "firstFollower", width: 150 },
	{ label: "最后跟进时间", prop: "lastFollowupTime", width: 160 },
	{ label: "最后跟进人", prop: "lastFollowupName", width: 150 },
	{ label: "下次跟进时间", prop: "nextFollowTime", width: 160 },
	{ label: "最后跟进内容", prop: "lastFollowRemark", width: 200, showOverflowTooltip: true },
	{ label: "过滤原因", prop: "filterRemark", width: 200, showOverflowTooltip: true },
	{ label: "线索状态", prop: "status", width: 100 },
];

// 字典翻译映射（prop → 翻译函数）
const dictTranslators = ref<Record<string, (val: any, row: any) => string>>({});

// 加载字典翻译器
async function loadDictTranslators() {
	const translators: Record<string, (val: any, row: any) => string> = {};

	// 性别映射
	translators.gender = (val: any) => {
		const map: Record<string, string> = { "0": "保密", "1": "男", "2": "女" };
		return map[String(val)] || val || "";
	};

	// 客户等级映射（兜底，动态翻译器会覆盖）
	translators.cluesLevel = (val: any) => {
		const map: Record<string, string> = { "1": "A级", "2": "B级", "3": "C级", "4": "D级" };
		if (val === null || val === undefined || val === "") return "";
		const values = toValueArray(val);
		return values.map((v: string) => map[v] || v).filter(Boolean).join("、");
	};

	// 来源映射（兜底，动态翻译器会覆盖）
	translators.sourceFrom = (val: any) => {
		const map: Record<string, string> = { "1": "手动录入", "2": "百度", "3": "抖音", "4": "53客服", "5": "小红书" };
		if (val === null || val === undefined || val === "") return "";
		const str = String(val);
		if (str.includes(",")) {
			return str.split(",").map((v: string) => map[v.trim()] || v.trim()).filter(Boolean).join("、");
		}
		return map[str] || str;
	};

	// 跟进方式映射（兜底，动态翻译器会覆盖）
	// 级联结构：2→21/22/23, 3→31/32/33
	translators.followupType = (val: any) => {
		const map: Record<string, string> = {
			"1": "待跟进", "2": "电话访谈", "21": "电话-无人接听", "22": "电话-拒接",
			"23": "电话-已接通", "3": "微信沟通", "31": "微信-待通过", "32": "微信-拒绝通过",
			"33": "微信-已通过", "4": "视频参观", "5": "预约参观", "6": "已参观"
		};
		const childToParent: Record<string, string> = {
			"21": "2", "22": "2", "23": "2",
			"31": "3", "32": "3", "33": "3"
		};
		if (val === null || val === undefined || val === "") return "";
		const values = toValueArray(val);
		if (values.length === 1) return map[values[0]] || values[0];
		if (values.length === 2) {
			const second = values[1], first = values[0];
			if (childToParent[second] === first) {
				return (map[first] || first) + " / " + (map[second] || second);
			}
		}
		return values.map((v: string) => map[v] || v).filter(Boolean).join("、");
	};

	// 户口性质映射（兜底，动态翻译器会覆盖）
	translators.householdType = (val: any) => {
		const map: Record<string, string> = { "1": "城镇", "2": "农村" };
		if (val === null || val === undefined || val === "") return "";
		const values = toValueArray(val);
		return values.map((v: string) => map[v] || v).filter(Boolean).join("、");
	};

	// 学员阶段映射（兜底，动态翻译器会覆盖）
	translators.education = (val: any) => {
		const map: Record<string, string> = { "1": "未知", "2": "初中", "3": "高中/中专/中技", "4": "大专/高技", "5": "本科" };
		if (val === null || val === undefined || val === "") return "";
		const values = toValueArray(val);
		return values.map((v: string) => map[v] || v).filter(Boolean).join("、");
	};

	// 项目名称翻译
	try {
		const projectList = await service.customer_pro.project.list();
		const projectMap = new Map(projectList.map((p: any) => [String(p.id), p.name]));
		translators.projectId = (val: any) => projectMap.get(String(val)) || val || "";
	} catch { }

	// 意向院校翻译
	try {
		const schoolList = await service.customer_pro.school.list();
		const schoolMap = new Map(schoolList.map((s: any) => [String(s.id), s.name]));
		translators.schoolId = (val: any) => schoolMap.get(String(val)) || val || "";
	} catch { }

	// 意向专业翻译
	try {
		const majorsList = await service.customer_pro.majors.list({});
		const majorsMap = new Map(majorsList.map((m: any) => [String(m.id), m.name]));
		translators.majorsId = (val: any) => majorsMap.get(String(val)) || val || "";
	} catch { }

	// 报读类型翻译
	try {
		const readtypesList = await service.customer_pro.readtypes.list();
		const readtypesMap = new Map(readtypesList.map((r: any) => [String(r.id), r.name]));
		translators.majorsType = (val: any) => readtypesMap.get(String(val)) || val || "";
	} catch { }

	// 报读层次翻译
	try {
		const readdegreeList = await service.customer_pro.readdegree.list();
		const readdegreeMap = new Map(readdegreeList.map((r: any) => [String(r.id), r.name]));
		translators.degreeId = (val: any) => readdegreeMap.get(String(val)) || val || "";
	} catch { }

	// 动态字典类型翻译（从字典数据加载，覆盖硬编码兜底翻译器）
	try {
		const typeList: any[] = await service.dict.type.list({ order: "createTime", sort: "asc" });
		const publicTypes = (typeList || []).filter((t: any) => String(t.isPublic) === "1");
		const infoList: any[] = await service.dict.info.list({});
		const itemsByTypeId: Record<string, any[]> = {};
		(infoList || []).forEach((it: any) => {
			const tid = String(it.typeId);
			if (!itemsByTypeId[tid]) itemsByTypeId[tid] = [];
			itemsByTypeId[tid].push(it);
		});

		// 为每个字典类型创建翻译器（key 用 typeKey，覆盖硬编码兜底）
		publicTypes.forEach((t: any) => {
			const typeKey = t.key;
			if (!typeKey) return;
			const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
			const items = itemsByTypeId[String(t.id)] || [];

			// 如果没有字典项数据，保留硬编码兜底翻译器
			if (!items.length) return;

			translators[typeKey] = (val: any, row: any) => {
				// 优先使用传入的 val，为空时从 row 取值
				let raw = val;
				if (raw === undefined || raw === null || raw === "") {
					raw = row[fieldName];
				}
				if ((raw === undefined || raw === null || raw === "") && row.labelJson) {
					const labelJson = parseLabelJson(row.labelJson);
					raw = labelJson[typeKey];
				}
				if (!raw) return "";
				const values = toValueArray(raw);
				const names = values.map((v: string) => {
					const found = items.find((it: any) => String(it.value) === String(v) || String(it.name) === String(v));
					return found?.name || v;
				});
				return names.join("、");
			};
		});
	} catch (e) {
		console.error("加载字典翻译器失败:", e);
	}

	dictTranslators.value = translators;
}

// 动态标签类型列配置
const dynamicTagColumns = ref<{ label: string; prop: string; width?: number; isCustom?: boolean }[]>([]);

// 加载动态标签类型列
async function loadDynamicTagColumns() {
	try {
		const typeList: any[] = await service.dict.type.list({ order: "createTime", sort: "asc" });
		const publicTypes = (typeList || []).filter((t: any) => String(t.isPublic) === "1");
		if (!publicTypes.length) { dynamicTagColumns.value = []; return; }

		const columns: { label: string; prop: string; width?: number; isCustom?: boolean }[] = [];
		publicTypes.forEach((t: any) => {
			const typeKey = t.key;
			if (!typeKey) return;
			// 跳过已在 EDIT_INFO_FIXED_COLUMNS 中定义的字段（按 prop 和 label 去重）
			if (EDIT_INFO_FIXED_COLUMNS.some((c) => c.prop === typeKey || c.label === t.name)) return;
			const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
			columns.push({
				label: t.name,
				prop: typeKey,
				width: 120,
				isCustom: !KNOWN_DB_FIELDS.has(fieldName)
			});
		});
		dynamicTagColumns.value = columns;
	} catch (e) {
		console.error("加载动态标签列失败:", e);
		dynamicTagColumns.value = [];
	}
}

// 加载标签类型搜索项
async function loadTagTypeSearchItems(): Promise<any[]> {
	try {
		const typeList: any[] = await service.dict.type.list({ order: "createTime", sort: "asc" });
		const publicTypes = (typeList || []).filter((t: any) => String(t.isPublic) === "1");

		const infoList: any[] = await service.dict.info.list({});
		const itemsByTypeId: Record<string, any[]> = {};
		(infoList || []).forEach((it: any) => {
			const tid = String(it.typeId);
			if (!itemsByTypeId[tid]) itemsByTypeId[tid] = [];
			itemsByTypeId[tid].push(it);
		});

		// 从字典API加载的搜索项
		const items: any[] = publicTypes.map((t: any) => {
			const typeKey = t.key || "";
			const isMulti = MULTI_SELECT_KEYS.has(typeKey);
			const dictItems = itemsByTypeId[String(t.id)] || [];
			const isHierarchical = hasDictHierarchy(dictItems);

			if (isHierarchical) {
				// 有层级：使用 el-tree-select
				const treeData = buildDictTree(dictItems.filter((it: any) => it.value != null));
				return () => ({
					label: t.name,
					prop: typeKey + "Status",
					component: {
						name: "el-tree-select",
						props: {
							data: treeData,
							props: { value: "id", label: "label", children: "children" },
							checkStrictly: true,
							renderAfterExpand: false,
							...isMulti ? { multiple: true, collapseTags: true, collapseTagsTooltip: true } : {},
							clearable: true,
							filterable: true,
							defaultExpandAll: true
						}
					}
				});
			}

			return () => ({
				label: t.name,
				prop: typeKey + "Status",
				component: {
					name: "el-select",
					props: {
						clearable: true,
						multiple: isMulti
					},
					options: dictItems
						.filter((it: any) => it.value != null)
						.map((it: any) => ({ label: it.name, value: it.value }))
				}
			});
		});

		// 补充：EDIT_INFO_FIXED_COLUMNS 中属于标签类型但不在字典公开类型中的列
		// 这些列有硬编码的翻译映射，从中提取选项生成搜索项
		const dictTypeKeys = new Set(publicTypes.map((t: any) => t.key));
		const hardcodedOptions: Record<string, { label: string; options: { label: string; value: string }[] }> = {
			followupType: {
				label: "跟进方式",
				options: [
					{ label: "待跟进", value: "1" },
					{ label: "电话访谈", value: "2" },
					{ label: "电话-无人接听", value: "21" },
					{ label: "电话-拒接", value: "22" },
					{ label: "电话-已接通", value: "23" },
					{ label: "微信沟通", value: "3" },
					{ label: "微信-待通过", value: "31" },
					{ label: "微信-拒绝通过", value: "32" },
					{ label: "微信-已通过", value: "33" },
					{ label: "视频参观", value: "4" },
					{ label: "预约参观", value: "5" },
					{ label: "已参观", value: "6" }
				]
			}
		};

		for (const [typeKey, config] of Object.entries(hardcodedOptions)) {
			// 只在字典API中没有该类型时才补充
			if (!dictTypeKeys.has(typeKey)) {
				const isMulti = MULTI_SELECT_KEYS.has(typeKey);
				items.push(() => ({
					label: config.label,
					prop: typeKey + "Status",
					component: {
						name: "el-select",
						props: {
							clearable: true,
							multiple: isMulti
						},
						options: config.options
					}
				}));
			}
		}

		return items;
	} catch (e) {
		console.error("加载标签类型搜索项失败:", e);
		return [];
	}
}

// 合并生成最终列配置
const allTableColumns = computed(() => {
	// 1. 有自定义模板的固定列（keywords, label, mobile）
	const specialCols: any[] = [
		{
			label: "关键词",
			prop: "keywords",
			width: 200,
			formatter(row: any) {
				return h(
					"span",
					{
						style: { color: row.keywords ? "var(--color-primary)" : undefined, cursor: "pointer", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", display: "block" },
						title: row.keywords || "无",
						onClick: () => doClues(row)
					},
					row.keywords || "无"
				);
			}
		},
		{ label: "标签", prop: "label", width: 260 },
		{
			label: "手机号",
			prop: "mobile",
			width: 200
		}
	];

	// 2. editInfo 固定字段列（排除有自定义模板的）
	const fixedCols = EDIT_INFO_FIXED_COLUMNS
		.filter((c) => !SPECIAL_COLUMN_PROPS.has(c.prop))
		.map((c) => {
			const col: any = { label: c.label, prop: c.prop };
			if (c.width) col.width = c.width;
			if (c.showOverflowTooltip) col.showOverflowTooltip = true;

			// 如果有字典翻译器，添加 formatter（通过 fieldName 映射取值）
			const translator = dictTranslators.value[c.prop];
			const fieldName = TYPE_KEY_TO_FIELD[c.prop];
			if (translator) {
				col.formatter = (row: any) => {
					const val = fieldName ? row[fieldName] : row[c.prop];
					return translator(val, row);
				};
			}

			// 53标识：点击打开线索详情，数字旁图标点击复制；空值显示"无"
			if (c.prop === "guestId") {
				col.formatter = (row: any) => {
					const id = row.guestId;
					return h(
						"span",
						{ style: { display: "flex", alignItems: "center", width: "100%", minWidth: "0" } },
						[
							h(
								"span",
								{
									style: {
										color: id ? "var(--color-primary)" : undefined,
										cursor: "pointer",
										flex: "1 1 auto",
										minWidth: "0",
										overflow: "hidden",
										textOverflow: "ellipsis",
										whiteSpace: "nowrap"
									},
									title: id || "无",
									onClick: () => doClues(row)
								},
								id || "无"
							),
							id
								? h(
										ElIcon,
										{
											size: 11,
											style: { flex: "0 0 auto", marginLeft: "4px", color: "#999", cursor: "pointer" },
											title: "复制53标识",
											onClick: (e: MouseEvent) => {
												e.stopPropagation();
												navigator.clipboard.writeText(id).then(() => ElMessage.success("已复制53标识"));
											}
										},
										{ default: () => h(CopyDocument) }
								  )
								: null
						]
					);
				};
			}

			// 线索状态翻译
			if (c.prop === "status") {
				col.formatter = (row: any) => {
					const s = row.status;
					if (s === 0 || s === "0") return "未成交";
					if (s === 1 || s === "1") return "已成交";
					return s ?? "";
				};
			}

			// 最后跟进内容：纯文本，去除HTML标签，超过200字省略
			if (c.prop === "lastFollowRemark") {
				col.formatter = (row: any) => {
					const val = row.lastFollowRemark;
					if (!val) return "";
					let str = String(val).replace(/<[^>]*>/g, "");
					return str.length > 200 ? str.substring(0, 200) + "..." : str;
				};
			}

			// 负责人：显示 servicesName
			if (c.prop === "servicesName") {
				col.formatter = (row: any) => {
					return row.servicesName || "";
				};
			}

			return col;
		});

	// 3. 动态标签列（不在固定列中的自定义标签类型）
	const tagCols = dynamicTagColumns.value.map((c) => {
		const col: any = { label: c.label, prop: c.prop, width: c.width || 120 };
		const fieldName = TYPE_KEY_TO_FIELD[c.prop] || c.prop;

		// 使用字典翻译器（通过 fieldName 映射取值）
		const translator = dictTranslators.value[c.prop];
		if (translator) {
			col.formatter = (row: any) => {
				const val = row[fieldName];
				if ((val === undefined || val === null || val === "") && row.labelJson) {
					const labelJson = parseLabelJson(row.labelJson);
					return translator(labelJson[c.prop], row);
				}
				return translator(val, row);
			};
		} else if (c.isCustom) {
			// 自定义字段从 labelJson 读取
			col.formatter = (row: any) => {
				const labelJson = parseLabelJson(row.labelJson);
				const raw = labelJson[c.prop];
				if (!raw) return "";
				return String(raw);
			};
		}

		return col;
	});

	// 去重：如果 tagCols 中的 prop 已在 specialCols 或 fixedCols 中存在，则跳过
	const existingProps = new Set([...specialCols, ...fixedCols].map((c: any) => c.prop));
	const dedupedTagCols = tagCols.filter((c: any) => !existingProps.has(c.prop));

	return [...specialCols, ...fixedCols, ...dedupedTagCols];
});

// cl-table 配置
const Table = useTable({
	columns: [],
	contextMenu: [
		"refresh",
		"check",
		(row: any) => ({
			label: "打开",
			callback(done: any) {
				doClues(row);
				done();
			}
		}),
		"delete",
		"order-asc",
		"order-desc"
	]
});

// ==================== 显示列设置 ====================
const COLUMN_CACHE_KEY = "clues_column_settings";
const COLUMN_PAGE_KEY = "clues_column_settings";
// 默认可见列（按顺序排列，guestId 排首位）
const DEFAULT_VISIBLE_COLUMNS = ["guestId", "keywords", "label", "mobile", "guestIpInfo", "createTime", "firstFollowTime", "lastFollowupTime", "nextFollowTime", "firstFollower", "lastFollowRemark"];

// ==================== 标签列渲染（使用公共模块） ====================
const { loadDictMeta, resetDictMeta: resetLocalDictMeta } = useDictMeta();

function getRowTags(row: any) {
	return resolveTags(row);
}

// 标签类型变更后的同步刷新（删除类型时调用）
async function onTagTypesChanged() {
	// 1. 重置字典元数据缓存，强制重新加载
	resetLocalDictMeta();
	await loadDictMeta();
	// 2. 重新加载翻译器和动态标签列
	await loadDictTranslators();
	await loadDynamicTagColumns();


	// 同步更新高级搜索的动态项
	updateAdvSearchItems();
	// 4. 刷新表格列（移除已删除类型的列，添加新增类型的列）
	await nextTick();
	await restoreColumnSettings();
}

interface ColumnSetting {
	prop: string;
	label: string;
	visible: boolean;
	width?: number;
	showOverflow?: boolean; // 是否溢出隐藏，默认 true（标签列除外）
	searchable?: boolean;   // 是否作为高级搜索条件（仅动态标签列有效）
	isTagType?: boolean;    // 是否为动态标签列（控制搜索开关是否显示）
}

const columnDialogVisible = ref(false);
const columnSettings = ref<ColumnSetting[]>([]);

// 从 allTableColumns 提取可配置的列（包含默认宽度）
function getOriginalColumns(): ColumnSetting[] {
	// 合并动态标签列 + TYPE_KEY_TO_FIELD 中的已知标签类型（如 cluesLevel、sourceFrom 等在 EDIT_INFO_FIXED_COLUMNS 中定义的标签列）
	const tagTypeProps = new Set([
		...dynamicTagColumns.value.map(c => c.prop),
		...Object.keys(TYPE_KEY_TO_FIELD)
	]);
	return allTableColumns.value.map((col: any) => ({
		prop: col.prop,
		label: col.label || col.prop,
		visible: true,
		width: col.width || undefined,
		showOverflow: col.prop !== "label", // 标签列默认不溢出隐藏，其他默认溢出隐藏
		isTagType: tagTypeProps.has(col.prop),
		searchable: tagTypeProps.has(col.prop) ? false : undefined
	}));
}

// 从 localStorage 缓存读取设置
function loadColumnSettingsFromCache(): ColumnSetting[] | null {
	try {
		const cached = localStorage.getItem(COLUMN_CACHE_KEY);
		if (cached) {
			return JSON.parse(cached);
		}
	} catch { }
	return null;
}

// 写入 localStorage 缓存
function saveColumnSettingsToCache(settings: ColumnSetting[]) {
	localStorage.setItem(COLUMN_CACHE_KEY, JSON.stringify(settings));
}

// 清除 localStorage 缓存
function removeColumnSettingsCache() {
	localStorage.removeItem(COLUMN_CACHE_KEY);
}

// 从后端加载设置
async function loadColumnSettingsFromServer(): Promise<ColumnSetting[] | null> {
	try {
		const res = await service.customer_pro.user_ui_config.getUiConfig({ pageKey: COLUMN_PAGE_KEY });
		if (res?.configData) {
			const parsed = JSON.parse(res.configData);
			console.log("[显示列] 后端有数据, 条数:", parsed.length);
			return parsed;
		}
		console.log("[显示列] 后端无数据");
	} catch (e) {
		console.error("[显示列] 从后端加载失败:", e);
	}
	return null;
}

// 保存设置到后端
async function saveColumnSettingsToServer(settings: ColumnSetting[]) {
	try {
		await service.customer_pro.user_ui_config.saveUiConfig({
			pageKey: COLUMN_PAGE_KEY,
			configData: JSON.stringify(settings)
		});
		console.log("[显示列] 已写入数据库, 条数:", settings.length);
	} catch (e) {
		console.error("[显示列] 保存到后端失败:", e);
	}
}

// 删除后端设置（恢复默认）
async function deleteColumnSettingsFromServer() {
	try {
		await service.customer_pro.user_ui_config.deleteUiConfig({ pageKey: COLUMN_PAGE_KEY });
	} catch (e) {
		console.error("删除显示列设置失败:", e);
	}
}

// 保存设置：同时写缓存和后端
async function saveColumnSettings(settings: ColumnSetting[]) {
	saveColumnSettingsToCache(settings);
	console.log("[显示列] 已写入localStorage缓存");
	await saveColumnSettingsToServer(settings);
}

// 根据 columnSettings 构建最终 table columns 并应用
function applyColumns(settings: ColumnSetting[]) {
	const settingsMap = new Map(settings.map((s) => [s.prop, s]));
	const sortedProps = settings.filter((s) => s.visible).map((s) => s.prop);

	// 从 allTableColumns 中按 prop 查找列配置
	const propColMap = new Map<string, any>();
	allTableColumns.value.forEach((col: any) => {
		if (col.prop) propColMap.set(col.prop, col);
	});

	const newColumns: any[] = [
		{ type: "selection" }
	];

	// 可见列按 settings 顺序排列，设置 orderNum 保证渲染顺序
	sortedProps.forEach((prop, index) => {
		const col = propColMap.get(prop);
		if (col) {
			const { hidden, orderNum, ...rest } = col;
			const s = settingsMap.get(prop);
			// 如果用户自定义了宽度，使用用户宽度
			if (s?.width) {
				rest.width = s.width;
			}
			// 根据 showOverflow 设置溢出隐藏样式
			// showOverflow 为 true（默认）时通过 className 添加 overflow-hidden 实现省略号
			// showOverflow 为 false 时不添加（如标签列需要完整显示）
			if (s?.showOverflow !== false) {
				rest.className = (rest.className ? rest.className + ' ' : '') + 'overflow-hidden';
			}
			newColumns.push({ ...rest, orderNum: index + 1 });
		}
	});

	// 隐藏的列也保留但标记 hidden
	settings
		.filter((s) => !s.visible)
		.forEach((s) => {
			const col = propColMap.get(s.prop);
			if (col) newColumns.push({ ...col, hidden: true });
		});

	// newColumns.push({ type: "op", width: 260, buttons: ["slot-op"] });

	Table.value?.setColumns(newColumns);
}

// 列宽拖拽调整后，保存宽度到设置
function onHeaderDragend(newWidth: number, oldWidth: number, column: any) {
	const prop = column.property;
	if (!prop) return;
	// 从当前缓存中找到对应列，更新宽度
	const cached = loadColumnSettingsFromCache();
	if (cached) {
		const item = cached.find((s) => s.prop === prop);
		if (item) {
			item.width = Math.round(newWidth);
			saveColumnSettingsToCache(cached);
			saveColumnSettingsToServer(cached);
			console.log("[显示列] 列宽调整:", prop, newWidth);
		}
	}
}

// 打开弹窗
function openColumnDialog() {
	const original = getOriginalColumns();
	const cached = loadColumnSettingsFromCache();

	if (cached && cached.length) {
		// 以缓存的顺序为准，合并新增的列到末尾
		const originalMap = new Map(original.map((s) => [s.prop, s]));
		const result: ColumnSetting[] = [];
		// 先按缓存顺序排列
		cached.forEach((c) => {
			const orig = originalMap.get(c.prop);
			if (orig) {
				// 合并：缓存值优先，但补充新增的 showOverflow 字段
				result.push({ ...orig, ...c, label: orig.label });
				originalMap.delete(c.prop);
			}
		});
		// 新增的列追加到末尾（默认隐藏）
		originalMap.forEach((orig) => {
			result.push({ ...orig, visible: false });
		});
		columnSettings.value = result;
	} else {
		columnSettings.value = original.map((col) => ({ ...col }));
	}
	columnDialogVisible.value = true;
}

// 应用设置：更新缓存+后端+表格
async function applyColumnSettings() {
	console.log("[显示列] 保存设置, 可见列数:", columnSettings.value.filter(s => s.visible).length, "总列数:", columnSettings.value.length);
	await saveColumnSettings(columnSettings.value);
	applyColumns(columnSettings.value);
	updateAdvSearchItems();
	columnDialogVisible.value = false;
}

// 恢复默认：与第一次打开一致，只显示默认列，guestId 排首位
async function resetColumnSettings() {
	console.log("[显示列] 恢复默认");
	removeColumnSettingsCache();
	const original = getOriginalColumns();
	const defaultVisible = new Set(DEFAULT_VISIBLE_COLUMNS);
	const orderedVisible = DEFAULT_VISIBLE_COLUMNS
		.map((prop) => original.find((c) => c.prop === prop))
		.filter(Boolean)
		.map((c) => ({ ...c!, visible: true }));
	const visibleProps = new Set(DEFAULT_VISIBLE_COLUMNS);
	const hiddenCols = original
		.filter((c) => !visibleProps.has(c.prop))
		.map((c) => ({ ...c, visible: false }));
	columnSettings.value = [...orderedVisible, ...hiddenCols];
	await saveColumnSettings(columnSettings.value);
	applyColumns(columnSettings.value);
	updateAdvSearchItems();
}

// 页面加载时恢复列设置
// 没有缓存：后端无数据→写入默认到后端+缓存；后端有数据→读后端写入缓存
// 有缓存：直接使用，仅合并新增/删除列时才写入
async function restoreColumnSettings() {
	const original = getOriginalColumns();
	console.log("[显示列] restoreColumnSettings, 可用列数:", original.length);

	// 构建默认列配置
	const buildDefaultSettings = (): ColumnSetting[] => {
		const defaultVisible = new Set(DEFAULT_VISIBLE_COLUMNS);
		const orderedVisible = DEFAULT_VISIBLE_COLUMNS
			.map((prop) => original.find((c) => c.prop === prop))
			.filter(Boolean)
			.map((c) => ({ ...c!, visible: true }));
		const visibleProps = new Set(DEFAULT_VISIBLE_COLUMNS);
		const hiddenCols = original
			.filter((c) => !visibleProps.has(c.prop))
			.map((c) => ({ ...c, visible: false }));
		return [...orderedVisible, ...hiddenCols];
	};

	let cached = loadColumnSettingsFromCache();
	let needWriteServer = false;
	console.log("[显示列] localStorage缓存:", cached ? `有${cached.length}条` : "无");

	if (!cached || !cached.length) {
		// console.log("[显示列] 无缓存");
		// 没有缓存，从后端加载
		const serverData = await loadColumnSettingsFromServer();
		if (serverData && serverData.length) {
			// 后端有数据：读后端数据写入缓存
			cached = serverData;
			saveColumnSettingsToCache(cached);
			console.log("[显示列] 无缓存- 后端有数据，已写入缓存");
		} else {
			// 后端没有数据：写入初始默认数据到后端+缓存
			cached = buildDefaultSettings();
			saveColumnSettingsToCache(cached);
			await saveColumnSettingsToServer(cached);
			console.log("[显示列] 无缓存- 后端无数据，已写入默认配置到缓存和后端");
		}
	}

	// 合并新增列（自定义标签类型列默认隐藏）+ 移除已删除列 + 补充缺失字段
	const originalMap = new Map(original.map((s) => [s.prop, s]));
	const cachedProps = new Set(cached.map((s) => s.prop));
	const newSettings = [...cached];
	let hasChanged = false;
	originalMap.forEach((orig, prop) => {
		if (!cachedProps.has(prop)) {
			newSettings.push({ ...orig, visible: false });
			hasChanged = true;
		} else {
			const existing = newSettings.find((s) => s.prop === prop);
			if (existing) {
				// 补充缺失的 showOverflow 字段
				if (existing.showOverflow === undefined) {
					existing.showOverflow = orig.showOverflow;
					hasChanged = true;
				}
				// 补充缺失的 isTagType / searchable 字段
				if (existing.isTagType === undefined && orig.isTagType !== undefined) {
					existing.isTagType = orig.isTagType;
					hasChanged = true;
				}
				if (existing.searchable === undefined && orig.searchable !== undefined) {
					existing.searchable = orig.searchable;
					hasChanged = true;
				}
			}
		}
	});
	const filtered = newSettings.filter((s) => originalMap.has(s.prop));
	if (filtered.length !== newSettings.length) {
		hasChanged = true;
	}

	if (hasChanged) {
		saveColumnSettingsToCache(filtered);
		await saveColumnSettingsToServer(filtered);
		console.log("[显示列] 列配置有变更，已同步缓存和后端");
	}
	columnSettings.value = filtered;
	applyColumns(filtered);
	updateAdvSearchItems();
}

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.clues,
		async onRefresh(params: any, { next, render }: any) {
			searchData.value = params;
			// 快捷筛选：已成交时覆盖默认 status，已推出由后端 school_pushed 处理
			if (params.quickFilter === "dealt") {
				params.status = 1;
			} else {
				params.status = 0;
			}
			params.dtype = 0;

			// 注入自定义排序参数（cl-crud 框架的 paramsReplace 可能丢失 order/sort）
			if (sortField.value && sortOrder.value) {
				params.order = sortField.value;
				params.sort = sortOrder.value;
			}

			const { list, pagination } = await next(params);
			render(list, pagination);
		}
	},
	async (app) => {
		app.refresh();
	}
);

// 刷新
const refresh = (params?: any) => {
	Crud.value?.refresh(params);
};

const randomDialog = ref(false);
const randomForm = ref<{
	randomNum: number;
	dateTime: string;
}>({
	randomNum: 1000,
	dateTime: ""
});
// 生成随机数据
const randomData = async () => {
	await service.customer_pro.clues.randomData(randomForm.value);
	randomDialog.value = false;
	ElMessage.success("数据生成中，请稍等10分钟再刷新页面");
};

// 迁移数据
// const migrateData = async () => {
// 	const info = await service.customer_pro.config.info({ id: 1 });
// 	service.customer_pro.config
// 		.migrateData()
// 		.then(() => {
// 			ElMessage.success("迁移执行中，请稍等5分钟再刷新页面");
// 		})
// 		.catch((e) => {
// 			ElMessage.error(e.message);
// 		});
// };

//清除数据
// const clearData = async () => {
// 	service.customer_pro.config.clearTable().then(() => {
// 		ElMessage.success("清除数据完成");
// 		refresh();
// 	});
// };

// 导出
// const onExportData = async (params: any) => {
// 	return Table.value?.selection;
// };

// 切换类型
// const handleClick = (tab: TabsPaneContext) => {
// 	refresh({ followType: tab.index });
// };

// 编辑
// const edit = (row: any) => {
// 	if (row.status == 0) {
// 		Crud.value?.rowEdit(row);
// 	} else {
// 		Crud.value?.rowInfo(row);
// 	}
// };

// 轨迹弹窗
const trackVisible = ref(false);
const openTracks = (row: any) => {
	cluesId.value = row.id;
	trackVisible.value = true;
};

// 跟进弹窗
const visible = ref(false);
const openFollow = (row: any) => {
	cluesId.value = row.id;
	cluesStatus.value = row.status;
	visible.value = true;
};

const cluesOpen = ref(false);
const drawerSize = ref("80%");
const doClues = (row: any) => {
	cluesId.value = row.id;
	callKey.value++;
	drawerSize.value = "80%";
	cluesOpen.value = true;
};

// 表格双击打开线索详情
const onRowDblclick = (row: any) => {
	doClues(row);
};

const onCluesInfoClose = () => {
	cluesOpen.value = false;
	refresh();
};

const toggleDrawerSize = () => {
	drawerSize.value = drawerSize.value === "80%" ? "100%" : "80%";
};

// 格式化手机号
const formatMobile = (phone: string) => {
	if (!phone) return phone;
	return phone.replace(/(\d{4})(?=\d)/g, "$1 ");
};

// 复制手机号
const copyMobile = (mobile: string) => {
	if (!mobile) return;
	navigator.clipboard.writeText(mobile).then(() => {
		ElMessage.success("已复制到剪贴板");
	});
};

// 远程拨号中状态
const calling = ref(false);
// 拨号回执监听注销函数（在 onMounted 注册一次，避免每次点击重复注册导致多次提示）
let offDialResult: (() => void) | null = null;

// 打开远程通话窗口并由窗口统一发起拨号。
const openRemoteCall = (row: any, event?: MouseEvent) => {
	// 详情页和表格按钮都把原始点击事件传入，卡片始终锚定在触发按钮附近。
	if (!deviceState.simCards.length) {
		ElMessage.warning("暂未获取到手机 SIM 信息，请刷新移动端连接");
		return;
	}
	callMobile(row, event);
};

// 拨打电话（远程控制同账号 Android 客户端自动拨号）
const openSimCardPicker = (row: any, event?: MouseEvent) => {
	const mobile = row?.mobile;
	if (!mobile) return;
	if (calling.value) return;
	if (deviceState.connStatus !== "connected" || !deviceState.androidOnline || !deviceState.canRemoteCall) {
		ElMessage.warning("客户端未连接或未开启电脑控制手机外呼");
		return;
	}
	if (!simCardOptions.value.length) {
		ElMessage.warning("暂未获取到手机 SIM 信息，请刷新移动端连接");
		return;
	}
	simCardTarget.value = row;
	simCardVisible.value = true;
	const anchor = (event?.currentTarget || event?.target) as HTMLElement | null;
	const rect = anchor?.getBoundingClientRect?.() || null;
	const width = 270;
	const estimatedHeight = 76 + simCardOptions.value.length * 68;
	// 没有坐标时也不要居中显示，使用页面左上操作区作为明确的兜底锚点。
	const left = rect ? Math.min(Math.max(8, rect.left), window.innerWidth - width - 8) : 16;
	const below = rect ? rect.bottom + 6 : 72;
	const opensUp = below + estimatedHeight > window.innerHeight - 8;
	const top = opensUp && rect ? Math.max(8, rect.top - estimatedHeight - 6) : below;
	simCardStyle.value = { left: `${left}px`, top: `${top}px`, width: `${width}px` };
};

const isSameSimCardTarget = (row: any) => {
	if (!simCardVisible.value || !simCardTarget.value) return false;
	if (simCardTarget.value === row) return true;
	const currentId = String(simCardTarget.value?.id || "");
	const nextId = String(row?.id || "");
	return Boolean(currentId && nextId && currentId === nextId);
};

// 表格/详情中的拨打图标是开关：同一条线索第一次打开、第二次关闭；
// 点击另一条线索则替换卡片锚点并重新定位。
const callMobile = (row: any, event?: MouseEvent) => {
	if (!calling.value && isSameSimCardTarget(row)) {
		closeSimCardPicker();
		return;
	}
	openSimCardPicker(row, event);
};

const selectSimCard = (sim: SimCardInfo) => {
	const row = simCardTarget.value;
	simCardVisible.value = false;
	simCardTarget.value = null;
	if (!row || !sim.available) return;
	calling.value = true;
	remoteCallDialogRef.value?.open(row, sim);
	ElMessage.info("正在呼叫，请查看手机");
};

const closeSimCardPicker = () => {
	simCardVisible.value = false;
	simCardTarget.value = null;
	// Element Plus 按钮点击后会保留 focus 状态，详情页的 success 按钮会因此
	// 一直显示高亮。关闭卡片时主动释放焦点，恢复按钮默认颜色。
	const active = document.activeElement as HTMLElement | null;
	if (active?.closest(".detail-call-anchor")) active.blur();
};

// 使用捕获阶段监听，避免详情抽屉、表格或其他组件阻止冒泡后，
// 页面空白区域无法关闭 SIM 卡片。卡片和拨打图标内部点击需要保留给自身处理。
const handleSimCardDocumentClick = (event: MouseEvent) => {
	if (!simCardVisible.value) return;
	const target = event.target as HTMLElement | null;
	if (target?.closest(".sim-card-popper, .remote-call-anchor, .detail-call-anchor")) return;
	closeSimCardPicker();
};

// 保存跟进
// const followSave = () => {
// 	FollowRef.value.sub();
// };

// 取消
const cancel = () => {
	visible.value = false;
	refresh();
};

// 保存到公海
// const pushCommonClause = () => {
// 	FollowRef.value.pushCommonClause();
// 	refresh();
// };

// 线索成交
const OrderFormRef = useForm(); //成交表单
const openOrderAdd = async (row: any) => {
	cluesId.value = row.id;
	const cluesOne = await service.customer_pro.clues.info({ id: cluesId.value });
	OrderFormRef.value?.open({
		title: "线索成交",
		items: [
			{
				type: "tabs",
				props: {
					labels: [
						{
							label: "基础信息",
							value: "base"
						},
						{
							label: "个人信息",
							value: "person"
						},
						{
							label: "收款信息",
							value: "financial"
						}
					]
				}
			},
			{
				label: "学生名称",
				prop: "name",
				span: 8,
				component: {
					name: "el-input"
				},
				required: true,
				group: "base"
			},
			{
				label: "学生电话",
				prop: "mobile",
				span: 8,
				component: {
					name: "el-input"
				},
				required: true,
				group: "base"
			},
			{
				label: "接待人员",
				prop: "receiver",
				span: 8,
				component: {
					name: "el-input"
				},
				required: true,
				group: "base"
			},

			{
				label: "身份证",
				prop: "idcardNumber",
				span: 8,
				component: {
					name: "el-input"
				},
				required: true,
				group: "base"
			},
			{
				label: "性别",
				prop: "gender",
				span: 8,
				component: {
					name: "el-select",
					options: [
						{
							label: "保密",
							value: "0"
						},
						{
							label: "男",
							value: "1"
						},
						{
							label: "女",
							value: "2"
						}
					]
				},
				required: true,
				group: "base"
			},
			{
				label: "意向院校",
				prop: "schoolId",
				span: 8,
				component: { name: "slot-schoolId" },
				group: "base"
			},
			{
				label: "意向专业",
				prop: "majorsId",
				span: 8,
				component: { name: "slot-majorsId" },
				group: "base"
			},
			{
				label: "报读类型",
				prop: "majorsType",
				span: 8,
				component: {
					name: "el-select",
					options: []
				},
				required: true,
				group: "base"
			},
			{
				label: "报读层次",
				prop: "degreeId",
				span: 8,
				component: {
					name: "el-select",
					options: []
				},
				required: true,
				group: "base"
			},

			{
				label: "通讯地址",
				prop: "address",
				span: 24,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "紧急联系人",
				prop: "emergencyContact",
				span: 12,
				component: {
					name: "el-input"
				},
				group: "person"
			},

			{
				label: "紧急联系人电话",
				prop: "emergencyMobile",
				props: {
					labelWidth: "130px"
				},
				span: 12,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "微信",
				prop: "wechat",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "民族",
				prop: "nation",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "籍贯",
				prop: "nativePlace",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},

			{
				label: "政治面貌",
				prop: "politicsStatus",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "户口性质",
				prop: "householdType",
				span: 8,
				component: {
					name: "el-select"
				},
				group: "person"
			},
			{
				label: "户口所在地",
				prop: "householdAddress",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},

			{
				label: "是否应届",
				prop: "freshman",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "当前学历",
				prop: "education",
				span: 8,
				component: {
					name: "el-select"
				},
				group: "person"
			},
			{
				label: "毕业学校",
				prop: "graduatedSchool",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},

			{
				label: "毕业时间",
				prop: "graduatedDate",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "person"
			},
			{
				label: "备注",
				prop: "remark",
				span: 24,
				component: {
					name: "el-input",
					props: {
						type: "textarea",
						rows: 4
					}
				},
				group: "base"
			},

			{
				label: "校方收定位金",
				prop: "schoolPayment",
				span: 8,
				value: 0.0,
				component: {
					name: "el-input-number",
					props: {
						precision: 2,
						step: 0.1
					}
				},
				group: "financial"
			},
			{
				label: "自收定位金",
				prop: "teamsPayment",
				span: 8,
				value: 0.0,
				component: {
					name: "el-input-number",
					props: {
						precision: 2,
						step: 0.1
					}
				},
				group: "financial"
			},
			{
				label: "支付编号",
				prop: "serial",
				span: 8,
				component: {
					name: "el-input"
				},
				group: "financial"
			},
			{
				label: "缴费凭证",
				prop: "voucher",
				span: 8,
				component: {
					name: "cl-upload"
				},
				group: "financial"
			}
		],
		form: {
			...cluesOne
		},
		on: {
			async open() {
				// 学校列表
				getSchoolList();

				// 报读类型
				const majorsTypeList = await service.customer_pro.readtypes.list();
				OrderFormRef.value?.setOptions(
					"majorsType",
					majorsTypeList.map((e) => {
						return {
							label: e.name,
							value: e.id
						};
					})
				);

				// 报读层次
				const degreeList = await service.customer_pro.readdegree.list();
				OrderFormRef.value?.setOptions(
					"degreeId",
					degreeList.map((e) => {
						return {
							label: e.name,
							value: e.id
						};
					})
				);

				// 户口性质
				OrderFormRef.value?.setOptions("householdType", [
					{
						label: "城镇",
						value: "1"
					},
					{
						label: "农村",
						value: "2"
					}
				]);

				// 学历
				OrderFormRef.value?.setOptions("education", [
					{
						label: "未知",
						value: "1"
					},
					{
						label: "初中",
						value: "2"
					},
					{
						label: "高中/中专/中技",
						value: "3"
					},
					{
						label: "大专/高技",
						value: "4"
					},
					{
						label: "本科",
						value: "5"
					}
				]);
			},
			submit(data: { cluesId: any; servicesId: any; projectId: any; }, { close, done }: any) {
				data.cluesId = cluesId.value;
				data.servicesId = row.servicesId;
				data.projectId = row.projectId;
				service.customer_pro.order
					.add({ ...data })
					.then((r: any) => {
						done();
						close();
						refresh();
					})
					.catch((err: any) => {
						done();
						ElMessage.error(err.message);
						close();
					});
			}
		}
	});
};

// 分配表单打开
const distributeFormRef = useForm(); //分配表单
const openDistribute = async () => {
	groupList.value = [];
	distributeFormRef.value?.setForm("groupId", null);
	kfList.value = [];
	distributeFormRef.value?.setForm("servicesId", null);

	projectList.value = await service.customer_pro.project.list();
	const ids = Crud.value?.selection.map((e: any) => {
		return e.id;
	});

	// const item: any = [];
	distributeFormRef.value?.open({
		title: `转交`,
		items: [
			{
				label: "项目",
				prop: "projectId",
				component: {
					name: "slot-projectId"
				},
				required: true
			},
			{
				label: "客服组",
				prop: "groupId",
				component: {
					name: "slot-groupId"
				},
				required: true
			},
			{
				label: "接收人",
				prop: "servicesId",
				component: {
					name: "slot-servicesId"
				},
				required: true
			}
		],

		on: {
			async open() { },
			submit(data: { servicesId: any; }, { done, close }: any) {
				service.customer_pro.clues
					.distribute({
						ids: ids,
						servicesId: data.servicesId
					})
					.then(() => {
						ElMessage.success("转交完成");
						close();
						refresh();
					})
					.catch((err) => {
						ElMessage.error(err.message);
						done();
					});
			}
		}
	});
};

// 项目id
const projectId = ref();
// 项目id改变
const projectChange = (v: any) => {
	groupList.value = [];
	distributeFormRef.value?.setForm("groupId", null);
	kfList.value = [];
	distributeFormRef.value?.setForm("servicesId", null);
	projectId.value = v;
	getGroupList(v);
};

// 组别id改变
const groupChange = (v: any) => {
	kfList.value = [];
	distributeFormRef.value?.setForm("servicesId", null);
	getKfList(v, projectId.value);
};

// 客服组列表
const groupList = ref();
const getGroupList = async (projectId: string) => {
	groupList.value = await service.customer_pro.project_group.list({ projectId });
};

// 客服人员列表
const kfList = ref();
const getKfList = async (groupId: string, projectId: string) => {
	kfList.value = await service.customer_pro.kf.list({ groupId, projectId });
};

// 学校列表
const schoolList = ref();
const majorsList = ref();
const getSchoolList = async () => {
	schoolList.value = await service.customer_pro.school.list();
	if (schoolList.value?.[0]?.id) {
		getMajorList(schoolList.value[0].id);
	}
};

// 学校改变
const schoolChange = async (v: any) => {
	majorsList.value = [];
	OrderFormRef.value?.setForm("majorsId", null);
	getMajorList(v);
};

// 专业列表
const getMajorList = async (v: any) => {
	majorsList.value = await service.customer_pro.majors.list({ schoolId: v });
};

const searchStatus = ref(false); // 搜索状态

// 高级搜索弹窗
const advSearchVisible = ref(false);

// 高级搜索回调
function onAdvSearch(params: any) {
	searchStatus.value = Object.values(params).some((value: any) => value != null && value !== "");
	// 先清除旧的搜索条件，避免清除单个条件后旧值残留
	if (Crud.value?.params) {
		const keepKeys = ["page", "size", "status", "dtype"];
		for (const key of Object.keys(Crud.value.params)) {
			if (!keepKeys.includes(key)) {
				delete Crud.value.params[key];
			}
		}
	}
	Crud.value?.refresh(params);
}

// 高级搜索重置（只清除查询条件，不刷新table）
function onAdvSearchReset() {
	// 清除 crud.params 中的搜索字段，只保留分页参数
	if (Crud.value?.params) {
		const keepKeys = ["page", "size", "status", "dtype"];
		for (const key of Object.keys(Crud.value.params)) {
			if (!keepKeys.includes(key)) {
				delete Crud.value.params[key];
			}
		}
	}
	// 重置快捷筛选 UI 状态
	quickFilter.value = "";
}

// 更新高级搜索的动态标签类型搜索项
async function updateAdvSearchItems() {
	tagTypeSearchItems.value = await loadTagTypeSearchItems();
}

const userInfo = ref();
// 是否是管理员
const isAdmin = ref(false);

// 负责人/参与人快捷筛选
const ownerFilter = ref<"owner" | "participant" | "">("");

function toggleOwnerFilter(type: "owner" | "participant" | "") {
	ownerFilter.value = type;
	// 带上筛选参数刷新列表
	Crud.value?.refresh({ ownerFilter: ownerFilter.value, quickFilter: quickFilter.value });
}

// 快捷筛选标签页
const quickFilter = ref<"" | "today" | "yesterday" | "dealt" | "pushed">("");

function toggleQuickFilter(type: "" | "today" | "yesterday" | "dealt" | "pushed") {
	quickFilter.value = type;
	Crud.value?.refresh({ ownerFilter: ownerFilter.value, quickFilter: quickFilter.value });
}

// 排序功能
const SORT_STORAGE_KEY = "clues_sort_state";
const SORT_PAGE_KEY = "clues_sort_state";
const sortFieldOptions = [
	{ label: "编号", value: "serialId" },
	{ label: "数据来源", value: "source_from" },
	{ label: "类型", value: "followup_type" },
	{ label: "最后跟进时间", value: "last_followup_time" },
	{ label: "下次跟进时间", value: "nextFollowTime" },
	{ label: "录入时间", value: "createTime" },
	{ label: "最后修改时间", value: "updateTime" },
	{ label: "获得时间", value: "allot_time" },
	{ label: "跟进次数", value: "followCount" },
	{ label: "成交次数", value: "dealCount" },
	{ label: "首次成交时间", value: "firstDealTime" },
	{ label: "最后成交时间", value: "lastDealTime" }
];

// 从 localStorage 恢复排序状态（同步，用于初始化）
const savedSortState = (() => {
	try {
		const saved = localStorage.getItem(SORT_STORAGE_KEY);
		if (saved) return JSON.parse(saved);
	} catch {}
	return null;
})();

const sortField = ref<string>(savedSortState?.field || "");
const sortOrder = ref<string>(savedSortState?.order || "");

// 从后端加载排序设置
async function loadSortStateFromServer(): Promise<{ field: string; order: string } | null> {
	try {
		const res = await service.customer_pro.user_ui_config.getUiConfig({ pageKey: SORT_PAGE_KEY });
		if (res?.configData) {
			return JSON.parse(res.configData);
		}
	} catch (e) {
		console.error("[排序] 从后端加载失败:", e);
	}
	return null;
}

// 保存排序设置到后端
async function saveSortStateToServer(field: string, order: string) {
	try {
		await service.customer_pro.user_ui_config.saveUiConfig({
			pageKey: SORT_PAGE_KEY,
			configData: JSON.stringify({ field, order })
		});
	} catch (e) {
		console.error("[排序] 保存到后端失败:", e);
	}
}

// 恢复排序设置（优先使用后端数据，回退到 localStorage）
async function restoreSortState() {
	const cached = savedSortState;
	if (cached?.field) {
		// localStorage 有数据，直接使用（同时同步到后端）
		await saveSortStateToServer(cached.field, cached.order);
		return;
	}
	// localStorage 无数据，从后端加载
	const serverData = await loadSortStateFromServer();
	if (serverData?.field) {
		sortField.value = serverData.field;
		sortOrder.value = serverData.order;
		localStorage.setItem(SORT_STORAGE_KEY, JSON.stringify({ field: serverData.field, order: serverData.order }));
	}
}

const sortFieldLabel = computed(() => {
	const opt = sortFieldOptions.find(o => o.value === sortField.value);
	return opt?.label || "";
});

function onSortCommand(command: string) {
	const [type, value] = command.split(":");
	if (type === "field") {
		if (sortField.value === value) {
			// 再次点击同一字段，切换排序方向
			if (sortOrder.value === "ASC") {
				sortOrder.value = "DESC";
			} else if (sortOrder.value === "DESC") {
				// 第三次点击：清除排序
				sortField.value = "";
				sortOrder.value = "";
			} else {
				sortOrder.value = "DESC";
			}
		} else {
			sortField.value = value;
			sortOrder.value = "DESC";
		}
	} else if (type === "order") {
		if (value === "default") {
			sortField.value = "";
			sortOrder.value = "";
		} else {
			if (!sortField.value) {
				// 未选择字段时默认选编号
				sortField.value = "serialId";
			}
			sortOrder.value = value;
		}
	}
	// 持久化排序状态（localStorage + 后端）
	localStorage.setItem(SORT_STORAGE_KEY, JSON.stringify({ field: sortField.value, order: sortOrder.value }));
	saveSortStateToServer(sortField.value, sortOrder.value);
	// 刷新列表（清除排序时需显式删除 crud.params 中残留的 order/sort）
	const params: any = {};
	if (sortField.value && sortOrder.value) {
		params.order = sortField.value;
		params.sort = sortOrder.value;
	} else {
		params.order = undefined;
		params.sort = undefined;
	}
	Crud.value?.refresh(params);
}
const getUserInfo = async () => {
	userInfo.value = await service.customer_pro.comm.userInfo();
	isAdmin.value = user.info.roleIds?.split(",").includes("1");
};

// 客服组
const getServiceGroup = async () => {
	const list = await service.customer_pro.project_group.list();
	serviceGroup.value = list.map((item) => {
		return {
			label: item.name,
			value: item.id
		};
	});
};

// 打开弹窗
const onMoreCommand = (command: string) => {
	if (command === "excel") {
		openExcelDialog();
	} else if (command === "distribute") {
		openDistribute();
	}
};

const openExcelDialog = () => {
	openExcel.value = true;
};

const fullscreenLoading = ref(false);
const excelDownRef = ref();
// 导出excel
const toexcel = async (isCurrent: boolean) => {
	const loading = ElLoading.service({
		lock: true,
		text: "Loading",
		background: "rgba(0, 0, 0, 0.7)"
	});

	const result = await service.customer_pro.clues.excel({
		...searchData.value,
		// keyWord: keyWord.value,
		isCurrentPage: isCurrent
	});

	loading.close();

	if (result.status == 1) {
		ElMessage.warning(result.msg);
	} else {
		ElMessage.success("导出任务已启动，请等待完成");
		excelDownRef.value.refresh();
	}

	return;
};
watch(
	() => openExcel.value,
	(val) => {
		if (!val) {
			console.log("停止任务");
			excelDownRef.value.stopInterval();
		}
	}
);
onMounted(async () => {
	await getUserInfo();
	await getServiceGroup();
	// 加载字典翻译器和动态标签列
	await loadDictTranslators();
	await loadDynamicTagColumns();

	// 加载字典数据（用于标签列渲染）
	await loadDictMeta();
	// 恢复显示列设置
	await nextTick();
	await restoreColumnSettings();
	// 恢复排序设置（从后端同步）
	await restoreSortState();

	// 常驻注册拨号结果回执监听（只注册一次，避免重复提示）
	document.addEventListener("click", handleSimCardDocumentClick, true);
	offDialResult = onDeviceEvent("dial_result", (msg: any) => {
		// 仅处理当前拨号周期的回执，忽略非拨号期的残留消息
		if (!calling.value) return;
		calling.value = false;
		// 不接通等由移动端自动处理（自动打开跟进页并兜底提交），
		// PC 端只保留点击时的「正在呼叫」提示，回执不再弹提示。
		if (msg?.status === "offline") {
			ElMessage.warning("客户端未连接，请先登录 App");
		}
		// 其余状态（成功/未接通）保持静默，交由移动端自动跟进
	});
});

// 超时保护：30s 内未收到回执则复位
let dialTimeout: any = null;
watch(calling, (v) => {
	clearTimeout(dialTimeout);
	if (v) {
		dialTimeout = setTimeout(() => {
			if (calling.value) {
				calling.value = false;
				ElMessage.warning("呼叫超时，请重试");
			}
		}, 30000);
	}
});

onUnmounted(() => {
	offDialResult?.();
	offDialResult = null;
	clearTimeout(dialTimeout);
	document.removeEventListener("click", handleSimCardDocumentClick, true);
	followRefreshTimers.splice(0).forEach((timer) => window.clearTimeout(timer));
});
</script>

<style lang="scss" scoped>
.owner-filter-group {
	margin-right: 8px;

	.el-button {
		font-size: 12px;
		padding: 5px 12px;
	}
}

.quick-filter-group {
	margin-left: 8px;

	.el-button {
		font-size: 12px;
		padding: 5px 12px;
	}
}

.dialog-footer {
	display: inline-flex;
	flex-direction: row;
	gap: 10px;
}


:deep(.el-pagination) {
	width: 100%;
	justify-content: end;
}

.el-button+.el-button {
	margin-left: 0;
}

.exportBox {
	display: flex;
	flex-direction: row;
	justify-content: center;
	align-items: center;
}

.excel-table {
	height: 600px;
}

.column-setting-tip {
	display: flex;
	align-items: center;
	gap: 6px;
	font-size: 12px;
	color: #606266;
	margin-bottom: 14px;
	padding: 8px 12px;
	background: #ecf5ff;
	border: 1px solid #d9ecff;
	border-radius: 6px;

	.el-icon {
		color: var(--color-primary);
		font-size: 14px;
	}
}

.column-dialog-body {
	max-height: 60vh;
	overflow-y: auto;
	overflow-x: hidden;
}

:deep(.column-dialog .el-dialog__body) {
	padding-bottom: 0;
}

:deep(.column-dialog .el-dialog__footer) {
	border-top: 1px solid #f0f1f5;
	padding-top: 16px;
}

.column-setting-list {
	.column-setting-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 12px;
		border-bottom: 1px solid #f0f1f5;
		transition: background 0.15s;

		&:hover {
			background: #f5f7fa;
		}

		.drag-handle {
			cursor: grab;
			color: #c0c4cc;
			font-size: 16px;
			flex-shrink: 0;

			&:active {
				cursor: grabbing;
			}
		}

		.column-label {
			flex: 1;
			font-size: 13px;
			color: #333;
		}
	}
}

:deep(.clues-drawer .el-drawer__body) {
	padding: 0;
}

:deep(.mobile-cell) {
	position: relative;
	display: inline-block;
}

:deep(.mobile-text) {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	cursor: pointer;
}

:deep(.mobile-actions-inline) {
	display: inline-flex;
	gap: 16px;
	align-items: center;
	justify-content: center;
	cursor: pointer;
}

:deep(.remote-call-anchor) {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 32px;
	height: 32px;
	cursor: pointer;
}

:deep(.mobile-actions-inline .el-icon) {
	color: var(--color-primary);
	font-size: 18px;
	width: 32px;
	height: 32px;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	border-radius: 4px;
	transition: background-color 0.2s;
}

:deep(.mobile-actions-inline .el-icon:hover) {
	background-color: var(--el-fill-color-light);
}

:deep(.mobile-actions-inline .el-icon.is-calling) {
	color: #19be6b;
	animation: calling-rotate 1s linear infinite;
}

@keyframes calling-rotate {
	from {
		transform: rotate(0deg);
	}
	to {
		transform: rotate(360deg);
	}
}

/* 溢出隐藏列：内容超长时显示省略号 */
:deep(.el-table .overflow-hidden .cell) {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

:deep(.label-tags) {
	display: flex;
	flex-wrap: wrap;
	gap: 4px;
	align-items: center;

	.el-tag {
		border-radius: 4px;
	}
}
</style>

<style>
/* el-popover teleported 到 body，必须用全局样式 */
.mobile-popover.el-popper {
	padding: 4px !important;
	min-width: 0 !important;
	width: auto !important;
	max-width: none !important;
}

.mobile-popover.el-popper .mobile-actions-inline {
	display: flex !important;
	gap: 8px;
	align-items: center;
	justify-content: center;
	cursor: pointer;
	width: fit-content;
	margin: 0 auto;
}

.mobile-popover.el-popper .mobile-actions-inline .el-icon {
	color: var(--color-primary);
	font-size: 18px;
	width: 28px;
	height: 28px;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: 4px;
	transition: background-color 0.2s;
}

.mobile-popover.el-popper .mobile-actions-inline .el-icon:hover {
	background-color: var(--el-fill-color-light);
}

.sim-card-popper {
	position: fixed;
	z-index: 4000;
	padding: 10px;
	border: 1px solid var(--el-border-color-light);
	border-radius: 10px;
	background: var(--el-bg-color-overlay);
	box-shadow: 0 8px 24px rgb(0 0 0 / 16%);
}

.sim-card-popper-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	min-height: 28px;
}

.sim-card-popper-title {
	padding: 2px 4px 8px;
	color: var(--el-text-color-primary);
	font-size: 13px;
	font-weight: 600;
}

.sim-card-popper-close {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 28px;
	height: 28px;
	margin: -2px -2px 4px 8px;
	padding: 0;
	border: 0;
	border-radius: 5px;
	background: transparent;
	color: var(--el-text-color-secondary);
	cursor: pointer;
}

.sim-card-popper-close:hover {
	background: var(--el-fill-color-light);
	color: var(--el-text-color-primary);
}

.sim-card-empty {
	padding: 10px 6px;
	color: var(--el-text-color-secondary);
	font-size: 12px;
	line-height: 1.5;
}

.sim-card-option {
	display: grid;
	grid-template-columns: 42px 1fr auto;
	column-gap: 8px;
	align-items: center;
	width: 100%;
	min-height: 54px;
	padding: 7px 8px;
	border-radius: 7px;
	background: transparent;
	color: var(--el-text-color-primary);
	text-align: left;
}

.sim-card-option:hover { background: var(--el-fill-color-light); }
.sim-card-option.unavailable { color: var(--el-text-color-disabled); }
.sim-card-slot { font-size: 13px; font-weight: 600; }
.sim-card-details {
	display: flex;
	min-width: 0;
	flex-direction: column;
	line-height: 1.35;
}
.sim-card-number { font-size: 13px; }
.sim-card-carrier { color: var(--el-text-color-secondary); font-size: 12px; }
.sim-card-action {
	border: 0;
	min-width: 52px;
	margin-right: 6px;
	padding: 5px 11px;
	border-radius: 5px;
	background: var(--el-color-primary);
	color: #fff;
	font-size: 12px;
	line-height: 1.2;
	text-align: center;
	white-space: nowrap;
	cursor: pointer;
}
.sim-card-action:disabled,
.sim-card-option.unavailable .sim-card-action {
	background: var(--el-fill-color-darker);
	color: var(--el-text-color-disabled);
	cursor: not-allowed;
}

.sort-dropdown-menu .sort-group-title {
	color: var(--el-text-color-secondary);
	font-size: 12px;
	font-weight: 600;
	padding: 4px 16px 2px;
	cursor: default;
}

.sort-dropdown-menu .el-dropdown-menu__item {
	display: flex;
	align-items: center;
	justify-content: space-between;
	min-width: 160px;
}

.sort-dropdown-menu .el-dropdown-menu__item.is-active {
	color: var(--el-color-primary);
	font-weight: 500;
}
</style>
