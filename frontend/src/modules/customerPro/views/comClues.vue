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
		</cl-row>

		<cl-row>
			<cl-refresh-btn size="small" />
			<cl-multi-delete-btn size="small" />

			<el-dropdown trigger="hover" @command="onMoreCommand">
				<el-button plain size="small">
					更多<el-icon class="el-icon--right"><ArrowDown /></el-icon>
				</el-button>
				<template #dropdown>
					<el-dropdown-menu>
						<el-dropdown-item command="distribute"
							:disabled="Table?.selection.length == 0"
							v-if="service.customer_pro.comClues._permission.distribute">
							转交
							<el-tooltip content="请先勾选至少一条线索，再进行转交操作" placement="right">
								<el-icon style="margin-left: 4px; color: #909399;"><QuestionFilled /></el-icon>
							</el-tooltip>
						</el-dropdown-item>
					</el-dropdown-menu>
				</template>
			</el-dropdown>

			<el-button plain size="small" @click="columnDialogVisible = true">
				<el-icon style="margin-right: 4px;"><Setting /></el-icon>显示列
			</el-button>

			<!-- 排序按钮 -->
			<el-dropdown trigger="click" @command="onSortCommand" :hide-on-click="false">
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
							<el-icon :size="20" @click.stop="callMobile(scope.row.mobile)">
								<Phone />
							</el-icon>
						</div>
					</el-popover>
					<span v-else class="mobile-text">{{ formatMobile(scope.row.mobile) }}</span>
				</template>
				<template #slot-op="{ scope }">
					<div style="display:flex;flex-direction:row;flex-wrap:wrap;align-items:center;gap:12px;">
						<el-button text bg type="warning" @click="openTracks(scope.row)"
							v-if="service.customer_pro.comClues._permission.getTrackList">轨迹</el-button>
						<el-popconfirm title="确定认领该线索吗?" @confirm="claim(scope.row)">
							<template #reference>
								<el-button text bg type="info"
									v-permission="service.customer_pro.comClues._permission.claimClues">认领</el-button>
							</template>
						</el-popconfirm>
					</div>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
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

		<!-- 轨迹弹窗 -->
		<cl-dialog title="轨迹" v-model="trackVisible">
			<sub-track ref="TrackRef" :id="cluesId" />
			<template #footer>
				<div class="dialog-footer">
					<el-button @click="trackVisible = false">关闭</el-button>
				</div>
			</template>
		</cl-dialog>

		<!-- 高级搜索弹窗 -->
		<AdvSearchDialog v-model="advSearchVisible" :serviceGroup="serviceGroup" :tagTypeItems="tagTypeSearchItems"
			:showServiceGroup="false" :showServiceStatus="false" :showUpdateTime="true"
			@search="onAdvSearch" @reset="onAdvSearchReset" />

		<!-- 线索详情抽屉 -->
		<el-drawer v-model="cluesOpen" title="" direction="rtl" :size="drawerSize" :with-header="false"
			class="clues-drawer">
			<clues-info :cluesId="cluesId" mode="ocean" :key="callKey" style="margin-top: -20px" @toggleFullscreen="toggleDrawerSize"
				@close="onCluesInfoClose" @tagTypesChanged="onTagTypesChanged" />
		</el-drawer>

		<!-- 显示列弹窗 -->
		<el-dialog v-model="columnDialogVisible" title="显示列设置" width="500px" :close-on-click-modal="false"
			@open="openColumnDialog" class="column-dialog">
			<div class="column-setting-tip">
				<el-icon><InfoFilled /></el-icon>
				拖拽左侧图标调整列顺序，开关控制列的显示与隐藏
			</div>
			<div class="column-dialog-body">
				<div class="column-setting-list">
					<draggable v-model="columnSettings" item-key="prop" handle=".drag-handle" animation="200">
						<template #item="{ element }">
							<div class="column-setting-item">
								<el-icon class="drag-handle"><Rank /></el-icon>
								<el-switch v-model="element.visible" />
								<span class="column-label">{{ element.label }}</span>
								<el-checkbox v-if="element.isTagType" v-model="element.searchable"
									size="small" style="margin-left: auto;" label="搜索" />
								<el-switch v-model="element.showOverflow" active-text="溢出隐藏" inactive-text="" size="small" style="margin-left: 8px;" />
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

<script lang="ts" name="customer_pro-comClues" setup>
import { useCrud, useForm, useTable } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ElMessage } from "element-plus";
import { Search, CopyDocument, Phone, ArrowDown, QuestionFilled, Setting, Rank, InfoFilled, Check } from "@element-plus/icons-vue";
import { onMounted, ref, watch, h, nextTick, computed } from "vue";
import draggable from "vuedraggable";
import SubTrack from "../components/clues/subTrack.vue";
import CluesInfo from "../components/clues/info.vue";
import AdvSearchDialog from "../components/AdvSearchDialog.vue";
import { useDictMeta, resolveTags, KNOWN_DB_FIELDS, toValueArray, parseLabelJson, buildDictTree, hasDictHierarchy } from "../utils/tagDict";

const { service } = useCool();
const { browser } = useCool();
const cluesId = ref();
const projectList = ref();
const searchStatus = ref(false);
const searchData = ref();
const callKey = ref(0);

// ===== 标签类型映射 =====
const TYPE_KEY_TO_FIELD: Record<string, string> = {
	cluesLevel: "level", sourceFrom: "sourceFrom", source_from: "sourceFrom",
	followupType: "followupType", followup_type: "followupType",
	householdType: "householdType", household_type: "householdType", education: "education"
};
const MULTI_SELECT_KEYS = new Set(["cluesLevel"]);
const tagTypeSearchItems = ref<any[]>([]);

// ===== 动态列配置 =====
const SPECIAL_COLUMN_PROPS = new Set(["label", "mobile", "keywords"]);

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
	{ label: "线索状态", prop: "status", width: 100 },
	{ label: "公海时间", prop: "oceanTime", width: 160 },
	{ label: "负责人", prop: "servicesName", width: 120, showOverflowTooltip: true },
	{ label: "参与人", prop: "servicesNames", width: 160, showOverflowTooltip: true },
];

const dictTranslators = ref<Record<string, (val: any, row: any) => string>>({});

async function loadDictTranslators() {
	const translators: Record<string, (val: any, row: any) => string> = {};

	translators.gender = (val: any) => {
		const map: Record<string, string> = { "0": "保密", "1": "男", "2": "女" };
		return map[String(val)] || val || "";
	};
	translators.cluesLevel = (val: any) => {
		const map: Record<string, string> = { "1": "A级", "2": "B级", "3": "C级", "4": "D级" };
		if (val === null || val === undefined || val === "") return "";
		return toValueArray(val).map((v: string) => map[v] || v).filter(Boolean).join("、");
	};
	translators.sourceFrom = (val: any) => {
		const map: Record<string, string> = { "1": "手动录入", "2": "百度", "3": "抖音", "4": "53客服", "5": "小红书" };
		if (val === null || val === undefined || val === "") return "";
		const str = String(val);
		if (str.includes(",")) return str.split(",").map((v: string) => map[v.trim()] || v.trim()).filter(Boolean).join("、");
		return map[str] || str;
	};
	translators.followupType = (val: any) => {
		const map: Record<string, string> = {
			"1": "待跟进", "2": "电话访谈", "21": "电话-无人接听", "22": "电话-拒接",
			"23": "电话-已接通", "3": "微信沟通", "31": "微信-待通过", "32": "微信-拒绝通过",
			"33": "微信-已通过", "4": "视频参观", "5": "预约参观", "6": "已参观"
		};
		const childToParent: Record<string, string> = { "21": "2", "22": "2", "23": "2", "31": "3", "32": "3", "33": "3" };
		if (val === null || val === undefined || val === "") return "";
		const values = toValueArray(val);
		if (values.length === 1) return map[values[0]] || values[0];
		if (values.length === 2) {
			const [first, second] = values;
			if (childToParent[second] === first) return (map[first] || first) + " / " + (map[second] || second);
		}
		return values.map((v: string) => map[v] || v).filter(Boolean).join("、");
	};
	translators.householdType = (val: any) => {
		const map: Record<string, string> = { "1": "城镇", "2": "农村" };
		if (val === null || val === undefined || val === "") return "";
		return toValueArray(val).map((v: string) => map[v] || v).filter(Boolean).join("、");
	};
	translators.education = (val: any) => {
		const map: Record<string, string> = { "1": "未知", "2": "初中", "3": "高中/中专/中技", "4": "大专/高技", "5": "本科" };
		if (val === null || val === undefined || val === "") return "";
		return toValueArray(val).map((v: string) => map[v] || v).filter(Boolean).join("、");
	};

	try {
		const projectList = await service.customer_pro.project.list();
		const projectMap = new Map(projectList.map((p: any) => [String(p.id), p.name]));
		translators.projectId = (val: any) => projectMap.get(String(val)) || val || "";
	} catch {}
	try {
		const schoolList = await service.customer_pro.school.list();
		const schoolMap = new Map(schoolList.map((s: any) => [String(s.id), s.name]));
		translators.schoolId = (val: any) => schoolMap.get(String(val)) || val || "";
	} catch {}
	try {
		const majorsList = await service.customer_pro.majors.list({});
		const majorsMap = new Map(majorsList.map((m: any) => [String(m.id), m.name]));
		translators.majorsId = (val: any) => majorsMap.get(String(val)) || val || "";
	} catch {}
	try {
		const readtypesList = await service.customer_pro.readtypes.list();
		const readtypesMap = new Map(readtypesList.map((r: any) => [String(r.id), r.name]));
		translators.majorsType = (val: any) => readtypesMap.get(String(val)) || val || "";
	} catch {}
	try {
		const readdegreeList = await service.customer_pro.readdegree.list();
		const readdegreeMap = new Map(readdegreeList.map((r: any) => [String(r.id), r.name]));
		translators.degreeId = (val: any) => readdegreeMap.get(String(val)) || val || "";
	} catch {}

	// 动态字典类型翻译
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
		publicTypes.forEach((t: any) => {
			const typeKey = t.key;
			if (!typeKey) return;
			const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
			const items = itemsByTypeId[String(t.id)] || [];
			if (!items.length) return;
			translators[typeKey] = (val: any, row: any) => {
				let raw = val;
				if (raw === undefined || raw === null || raw === "") raw = row[fieldName];
				if ((raw === undefined || raw === null || raw === "") && row.labelJson) {
					raw = parseLabelJson(row.labelJson)[typeKey];
				}
				if (!raw) return "";
				return toValueArray(raw).map((v: string) => {
					const found = items.find((it: any) => String(it.value) === String(v) || String(it.name) === String(v));
					return found?.name || v;
				}).join("、");
			};
		});
	} catch (e) {
		console.error("加载字典翻译器失败:", e);
	}

	dictTranslators.value = translators;
}

const dynamicTagColumns = ref<{ label: string; prop: string; width?: number; isCustom?: boolean }[]>([]);

async function loadDynamicTagColumns() {
	try {
		const typeList: any[] = await service.dict.type.list({ order: "createTime", sort: "asc" });
		const publicTypes = (typeList || []).filter((t: any) => String(t.isPublic) === "1");
		if (!publicTypes.length) { dynamicTagColumns.value = []; return; }
		const columns: { label: string; prop: string; width?: number; isCustom?: boolean }[] = [];
		publicTypes.forEach((t: any) => {
			const typeKey = t.key;
			if (!typeKey) return;
			if (EDIT_INFO_FIXED_COLUMNS.some((c) => c.prop === typeKey || c.label === t.name)) return;
			const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
			columns.push({ label: t.name, prop: typeKey, width: 120, isCustom: !KNOWN_DB_FIELDS.has(fieldName) });
		});
		dynamicTagColumns.value = columns;
	} catch (e) {
		console.error("加载动态标签列失败:", e);
		dynamicTagColumns.value = [];
	}
}

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
					props: { clearable: true, multiple: isMulti },
					options: dictItems.filter((it: any) => it.value != null).map((it: any) => ({ label: it.name, value: it.value }))
				}
			});
		});

		// 补充：EDIT_INFO_FIXED_COLUMNS 中属于标签类型但不在字典公开类型中的列
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
			if (!dictTypeKeys.has(typeKey)) {
				const isMulti = MULTI_SELECT_KEYS.has(typeKey);
				items.push(() => ({
					label: config.label,
					prop: typeKey + "Status",
					component: {
						name: "el-select",
						props: { clearable: true, multiple: isMulti },
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

const allTableColumns = computed(() => {
	const specialCols: any[] = [
		{
			label: "关键词", prop: "keywords", width: 200,
			formatter(row: any) {
				return h("span", {
					style: { color: row.keywords ? "var(--color-primary)" : undefined, cursor: "pointer", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", display: "block" },
					title: row.keywords || "无",
					onClick: () => doClues(row)
				}, row.keywords || "无");
			}
		},
		{ label: "标签", prop: "label", width: 260 },
		{ label: "手机号", prop: "mobile", width: 200 }
	];

	const fixedCols = EDIT_INFO_FIXED_COLUMNS
		.filter((c) => !SPECIAL_COLUMN_PROPS.has(c.prop))
		.map((c) => {
			const col: any = { label: c.label, prop: c.prop };
			if (c.width) col.width = c.width;
			if (c.showOverflowTooltip) col.showOverflowTooltip = true;
			const translator = dictTranslators.value[c.prop];
			const fieldName = TYPE_KEY_TO_FIELD[c.prop];
			if (translator) {
				col.formatter = (row: any) => translator(fieldName ? row[fieldName] : row[c.prop], row);
			}
			if (c.prop === "guestId") {
				col.formatter = (row: any) => {
					return h("span", { style: { color: row.guestId ? "var(--color-primary)" : undefined, cursor: "pointer", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", display: "block" }, title: row.guestId || "无", onClick: () => doClues(row) }, row.guestId || "无");
				};
			}
			if (c.prop === "status") {
				col.formatter = (row: any) => {
					const s = row.status;
					if (s === 0 || s === "0") return "未成交";
					if (s === 1 || s === "1") return "已成交";
					return s ?? "";
				};
			}
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

	const tagCols = dynamicTagColumns.value.map((c) => {
		const col: any = { label: c.label, prop: c.prop, width: c.width || 120 };
		const fieldName = TYPE_KEY_TO_FIELD[c.prop] || c.prop;
		const translator = dictTranslators.value[c.prop];
		if (translator) {
			col.formatter = (row: any) => {
				const val = row[fieldName];
				if ((val === undefined || val === null || val === "") && row.labelJson) {
					return translator(parseLabelJson(row.labelJson)[c.prop], row);
				}
				return translator(val, row);
			};
		} else if (c.isCustom) {
			col.formatter = (row: any) => {
				const raw = parseLabelJson(row.labelJson)[c.prop];
				return raw ? String(raw) : "";
			};
		}
		return col;
	});

	const existingProps = new Set([...specialCols, ...fixedCols].map((c: any) => c.prop));
	const dedupedTagCols = tagCols.filter((c: any) => !existingProps.has(c.prop));
	return [...specialCols, ...fixedCols, ...dedupedTagCols];
});

const Table = useTable({ columns: [], contextMenu: ["refresh", "check", (row: any) => ({ label: "打开", callback(done: any) { doClues(row); done(); } }), "delete", "order-asc", "order-desc"] });

// ==================== 显示列设置 ====================
const COLUMN_CACHE_KEY = "comClues_column_settings";
const COLUMN_PAGE_KEY = "comClues_column_settings";
const DEFAULT_VISIBLE_COLUMNS = ["guestId", "keywords", "label", "mobile", "guestIpInfo", "createTime", "oceanTime", "lastFollowupTime", "lastFollowRemark"];

const { loadDictMeta, resetDictMeta: resetLocalDictMeta } = useDictMeta();

function getRowTags(row: any) { return resolveTags(row); }

async function onTagTypesChanged() {
	resetLocalDictMeta();
	await loadDictMeta();
	await loadDictTranslators();
	await loadDynamicTagColumns();

	// 同步更新高级搜索的动态项
	updateAdvSearchItems();
	await nextTick();
	await restoreColumnSettings();
}

// 是否溢出隐藏，默认 true（标签列除外）
// searchable: 是否作为高级搜索条件（仅动态标签列有效）
// isTagType: 是否为动态标签列（控制搜索开关是否显示）
interface ColumnSetting { prop: string; label: string; visible: boolean; width?: number; showOverflow?: boolean; searchable?: boolean; isTagType?: boolean; }
const columnDialogVisible = ref(false);
const columnSettings = ref<ColumnSetting[]>([]);

function getOriginalColumns(): ColumnSetting[] {
	const tagTypeProps = new Set([
		...dynamicTagColumns.value.map(c => c.prop),
		...Object.keys(TYPE_KEY_TO_FIELD)
	]);
	return allTableColumns.value.map((col: any) => ({ prop: col.prop, label: col.label || col.prop, visible: true, width: col.width || undefined, showOverflow: col.prop !== "label", isTagType: tagTypeProps.has(col.prop), searchable: tagTypeProps.has(col.prop) ? false : undefined }));
}
function loadColumnSettingsFromCache(): ColumnSetting[] | null {
	try { const cached = localStorage.getItem(COLUMN_CACHE_KEY); if (cached) return JSON.parse(cached); } catch {}
	return null;
}
function saveColumnSettingsToCache(settings: ColumnSetting[]) { localStorage.setItem(COLUMN_CACHE_KEY, JSON.stringify(settings)); }
function removeColumnSettingsCache() { localStorage.removeItem(COLUMN_CACHE_KEY); }
async function loadColumnSettingsFromServer(): Promise<ColumnSetting[] | null> {
	try {
		const res = await service.customer_pro.user_ui_config.getUiConfig({ pageKey: COLUMN_PAGE_KEY });
		if (res?.configData) return JSON.parse(res.configData);
	} catch (e) { console.error("[显示列] 从后端加载失败:", e); }
	return null;
}
async function saveColumnSettingsToServer(settings: ColumnSetting[]) {
	try { await service.customer_pro.user_ui_config.saveUiConfig({ pageKey: COLUMN_PAGE_KEY, configData: JSON.stringify(settings) }); }
	catch (e) { console.error("[显示列] 保存到后端失败:", e); }
}
async function saveColumnSettings(settings: ColumnSetting[]) { saveColumnSettingsToCache(settings); await saveColumnSettingsToServer(settings); }

function applyColumns(settings: ColumnSetting[]) {
	const settingsMap = new Map(settings.map((s) => [s.prop, s]));
	const sortedProps = settings.filter((s) => s.visible).map((s) => s.prop);
	const propColMap = new Map<string, any>();
	allTableColumns.value.forEach((col: any) => { if (col.prop) propColMap.set(col.prop, col); });
	const newColumns: any[] = [{ type: "selection" }];
	sortedProps.forEach((prop, index) => {
		const col = propColMap.get(prop);
		if (col) {
			const { hidden, orderNum, ...rest } = col;
			const s = settingsMap.get(prop);
			if (s?.width) rest.width = s.width;
			// 根据 showOverflow 设置溢出隐藏样式
			if (s?.showOverflow !== false) {
				rest.className = (rest.className ? rest.className + ' ' : '') + 'overflow-hidden';
			}
			newColumns.push({ ...rest, orderNum: index + 1 });
		}
	});
	settings.filter((s) => !s.visible).forEach((s) => {
		const col = propColMap.get(s.prop);
		if (col) newColumns.push({ ...col, hidden: true });
	});
	Table.value?.setColumns(newColumns);
}

function onHeaderDragend(newWidth: number, oldWidth: number, column: any) {
	const prop = column.property;
	if (!prop) return;
	const cached = loadColumnSettingsFromCache();
	if (cached) {
		const item = cached.find((s) => s.prop === prop);
		if (item) { item.width = Math.round(newWidth); saveColumnSettingsToCache(cached); saveColumnSettingsToServer(cached); }
	}
}

function openColumnDialog() {
	const original = getOriginalColumns();
	const cached = loadColumnSettingsFromCache();
	if (cached && cached.length) {
		const originalMap = new Map(original.map((s) => [s.prop, s]));
		const result: ColumnSetting[] = [];
		cached.forEach((c) => { const orig = originalMap.get(c.prop); if (orig) { result.push({ ...orig, ...c, label: orig.label }); originalMap.delete(c.prop); } });
		originalMap.forEach((orig) => { result.push({ ...orig, visible: false }); });
		columnSettings.value = result;
	} else {
		columnSettings.value = original.map((col) => ({ ...col }));
	}
}

async function applyColumnSettings() { await saveColumnSettings(columnSettings.value); applyColumns(columnSettings.value); updateAdvSearchItems(); columnDialogVisible.value = false; }

async function resetColumnSettings() {
	removeColumnSettingsCache();
	const original = getOriginalColumns();
	const defaultVisible = new Set(DEFAULT_VISIBLE_COLUMNS);
	const orderedVisible = DEFAULT_VISIBLE_COLUMNS.map((prop) => original.find((c) => c.prop === prop)).filter(Boolean).map((c) => ({ ...c!, visible: true }));
	const hiddenCols = original.filter((c) => !defaultVisible.has(c.prop)).map((c) => ({ ...c, visible: false }));
	columnSettings.value = [...orderedVisible, ...hiddenCols];
	await saveColumnSettings(columnSettings.value);
	applyColumns(columnSettings.value);
	updateAdvSearchItems();
}

async function restoreColumnSettings() {
	const original = getOriginalColumns();
	let cached = loadColumnSettingsFromCache();
	if (!cached || !cached.length) {
		const serverData = await loadColumnSettingsFromServer();
		if (serverData && serverData.length) { cached = serverData; saveColumnSettingsToCache(cached); }
	}
	if (!cached || !cached.length) {
		const defaultVisible = new Set(DEFAULT_VISIBLE_COLUMNS);
		const orderedVisible = DEFAULT_VISIBLE_COLUMNS.map((prop) => original.find((c) => c.prop === prop)).filter(Boolean).map((c) => ({ ...c!, visible: true }));
		const hiddenCols = original.filter((c) => !defaultVisible.has(c.prop)).map((c) => ({ ...c, visible: false }));
		cached = [...orderedVisible, ...hiddenCols];
	}
	const originalMap = new Map(original.map((s) => [s.prop, s]));
	const cachedProps = new Set(cached.map((s) => s.prop));
	const newSettings = [...cached];
	originalMap.forEach((orig, prop) => {
		if (!cachedProps.has(prop)) {
			newSettings.push({ ...orig, visible: false });
		} else {
			const existing = newSettings.find((s) => s.prop === prop);
			if (existing) {
				if (existing.showOverflow === undefined) {
					existing.showOverflow = orig.showOverflow;
				}
				if (existing.isTagType === undefined && orig.isTagType !== undefined) {
					existing.isTagType = orig.isTagType;
				}
				if (existing.searchable === undefined && orig.searchable !== undefined) {
					existing.searchable = orig.searchable;
				}
			}
		}
	});
	const filtered = newSettings.filter((s) => originalMap.has(s.prop));
	saveColumnSettingsToCache(filtered);
	await saveColumnSettingsToServer(filtered);
	columnSettings.value = filtered;
	applyColumns(filtered);
	updateAdvSearchItems();
}

// cl-crud 配置（公海：status=2, oceanTime=true）
const Crud = useCrud(
	{
		service: service.customer_pro.comClues,
		async onRefresh(params: any, { next, render }: any) {
			searchData.value = params;
			params.oceanTime = true;
			params.status = 2;
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

const refresh = (params?: any) => { Crud.value?.refresh(params); };

// 负责人/参与人快捷筛选
const ownerFilter = ref<"owner" | "participant" | "">("");

function toggleOwnerFilter(type: "owner" | "participant" | "") {
	ownerFilter.value = type;
	Crud.value?.refresh({ ownerFilter: ownerFilter.value });
}

// 轨迹弹窗
const trackVisible = ref(false);
const openTracks = (row: any) => { cluesId.value = row.id; trackVisible.value = true; };

// 线索详情抽屉
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
const onCluesInfoClose = () => { cluesOpen.value = false; refresh(); };
const toggleDrawerSize = () => { drawerSize.value = drawerSize.value === "80%" ? "100%" : "80%"; };

// 认领
const claim = (row: any) => {
	service.customer_pro.clues
		.claimClues({ cluesId: row.id })
		.then(() => {
			ElMessage.success("认领成功");
			refresh();
		})
		.catch((e) => {
			ElMessage.error(e.message);
		});
};

// 格式化手机号
const formatMobile = (phone: string) => { if (!phone) return phone; return phone.replace(/(\d{4})(?=\d)/g, "$1 "); };
const copyMobile = (mobile: string) => { if (!mobile) return; navigator.clipboard.writeText(mobile).then(() => { ElMessage.success("已复制到剪贴板"); }); };
const callMobile = (mobile: string) => { if (!mobile) return; ElMessage.warning("功能正在开发"); };

// 分配表单
const distributeFormRef = useForm();
const openDistribute = async () => {
	groupList.value = [];
	distributeFormRef.value?.setForm("groupId", null);
	kfList.value = [];
	distributeFormRef.value?.setForm("servicesId", null);
	projectList.value = await service.customer_pro.project.list();
	const ids = Crud.value?.selection.map((e: any) => e.id);
	distributeFormRef.value?.open({
		title: `转交`,
		items: [
			{ label: "项目", prop: "projectId", component: { name: "slot-projectId" }, required: true },
			{ label: "客服组", prop: "groupId", component: { name: "slot-groupId" }, required: true },
			{ label: "接收人", prop: "servicesId", component: { name: "slot-servicesId" }, required: true }
		],
		on: {
			async open() { },
			submit(data: { servicesId: any }, { done, close }: any) {
				service.customer_pro.clues
					.distribute({ ids, servicesId: data.servicesId })
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

const projectId = ref();
const projectChange = (v: any) => {
	groupList.value = [];
	distributeFormRef.value?.setForm("groupId", null);
	kfList.value = [];
	distributeFormRef.value?.setForm("servicesId", null);
	projectId.value = v;
	getGroupList(v);
};
const groupChange = (v: any) => {
	kfList.value = [];
	distributeFormRef.value?.setForm("servicesId", null);
	getKfList(v, projectId.value);
};
const groupList = ref();
const getGroupList = async (projectId: string) => { groupList.value = await service.customer_pro.project_group.list({ projectId }); };
const kfList = ref();
const getKfList = async (groupId: string, projectId: string) => { kfList.value = await service.customer_pro.kf.list({ groupId, projectId }); };

const serviceGroup = ref();
const getServiceGroup = async () => {
	const list = await service.customer_pro.project_group.list();
	serviceGroup.value = list.map((item: any) => ({ label: item.name, value: item.id }));
};

const onMoreCommand = (command: string) => {
	if (command === "distribute") openDistribute();
};

// 高级搜索弹窗
const advSearchVisible = ref(false);

// 高级搜索回调
function onAdvSearch(params: any) {
	searchStatus.value = Object.values(params).some((value: any) => value != null && value !== "");
	// 先清除旧的搜索条件，避免清除单个条件后旧值残留
	if (Crud.value?.params) {
		const keepKeys = ["page", "size", "status", "dtype", "oceanTime"];
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
	if (Crud.value?.params) {
		const keepKeys = ["page", "size", "status", "dtype", "oceanTime"];
		for (const key of Object.keys(Crud.value.params)) {
			if (!keepKeys.includes(key)) {
				delete Crud.value.params[key];
			}
		}
	}
}

// 更新高级搜索的动态标签类型搜索项
async function updateAdvSearchItems() {
	tagTypeSearchItems.value = await loadTagTypeSearchItems();
}

// 排序功能
const SORT_STORAGE_KEY = "comClues_sort_state";
const SORT_PAGE_KEY = "comClues_sort_state";
const sortFieldOptions = [
	{ label: "编号", value: "serialId" },
	{ label: "数据来源", value: "source_from" },
	{ label: "类型", value: "followup_type" },
	{ label: "最后跟进时间", value: "last_followup_time" },
	{ label: "下次跟进时间", value: "nextFollowTime" },
	{ label: "录入时间", value: "createTime" },
	{ label: "最后修改时间", value: "updateTime" },
	{ label: "获得时间", value: "allot_time" },
	{ label: "公海时间", value: "ocean_time" },
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
			if (sortOrder.value === "ASC") {
				sortOrder.value = "DESC";
			} else if (sortOrder.value === "DESC") {
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

onMounted(async () => {
	await getServiceGroup();
	await loadDictTranslators();
	await loadDynamicTagColumns();
	await loadDictMeta();
	await nextTick();
	await restoreColumnSettings();
	// 恢复排序设置（从后端同步）
	await restoreSortState();
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
</style>
