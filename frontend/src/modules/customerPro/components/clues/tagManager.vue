<template>
	<!-- 主弹窗：标签管理 -->
	<el-dialog
		v-model="visible"
		title="标签管理"
		width="70%"
		append-to-body
		class="tag-manager-dialog"
		:close-on-click-modal="false"
	>
		<div class="tag-manager" v-loading="loading">
			<!-- 左侧：类型列表 -->
			<div class="left-panel">
				<div class="left-header">类型</div>
				<el-scrollbar class="left-list">
					<ul>
						<li
							v-for="t in types"
							:key="t.id"
							:class="{ 'is-active': activeTypeId === t.id }"
							@click="activeTypeId = t.id"
						>
							{{ t.name }}
						</li>
						<el-empty v-if="!types.length" :image-size="60" description="暂无类型" />
					</ul>
				</el-scrollbar>
				<div class="left-footer">
					<span class="edit-link" @click="openTypeEdit">编辑</span>
				</div>
			</div>

			<!-- 右侧：按类型分组 -->
			<div class="right-panel">
				<el-scrollbar>
					<div v-if="!types.length" class="empty-tip">
						<el-empty description="暂无类型" />
					</div>
					<div
						v-for="t in types"
						:key="t.id"
						class="group"
						:ref="(el) => setGroupRef(el, t.id)"
					>
						<div class="group-header">
							<span class="group-title">{{ t.name }}</span>
							<el-button v-if="!PROTECTED_TYPE_KEYS.has(t.key)" text size="small" @click="openTagEdit(t)">
								<el-icon><EditPen /></el-icon>
								<span class="edit-text">编辑</span>
							</el-button>
						</div>
						<div class="group-tags">
							<template v-if="(tagsByType[t.id] || []).length">
								<template v-for="tag in getRootTags(t.id)" :key="tag.id">
									<span
										class="tag-text"
										:class="{ 'is-selected': selectedTagIds.has(tag.id) }"
										@click="toggleTag(tag, t)"
									>
										{{ tag.name }}
									</span>
									<template v-for="child in getChildTags(t.id, tag.id)" :key="child.id">
										<span
											class="tag-text tag-text--child"
											:class="{ 'is-selected': selectedTagIds.has(child.id) }"
											@click="toggleTag(child, t)"
										>
											<el-icon class="child-arrow"><ArrowRight /></el-icon>
											{{ child.name }}
										</span>
									</template>
								</template>
							</template>
							<span v-else class="empty-text">暂无标签</span>
						</div>
					</div>
				</el-scrollbar>
			</div>
		</div>

		<template #footer>
			<el-button @click="visible = false">取消</el-button>
			<el-button type="primary" :loading="saving" @click="saveSelectedTags">保存</el-button>
		</template>
	</el-dialog>

	<!-- 子弹窗 1：编辑类型列表 -->
	<el-dialog
		v-model="typeEditVisible"
		title="编辑类型"
		width="520px"
		append-to-body
		class="tag-sub-edit-dialog"
		:close-on-click-modal="false"
	>
		<div class="edit-tip">
			<el-icon><InfoFilled /></el-icon>
			修改名称、新增或删除项，保存后将同步更新
			<span class="advanced-toggle" @click="toggleTypeAllExpanded">
				<el-icon class="setting-icon" :class="{ 'is-active': typeAllExpanded }"><Setting /></el-icon>
				<span>高级配置</span>
			</span>
		</div>

		<div class="edit-list">
			<el-scrollbar ref="typeScrollRef" max-height="380px">
				<div
					v-for="(item, idx) in typeItems"
					:key="idx"
					class="edit-item-wrap"
				>
					<div class="edit-item">
						<span class="idx">{{ idx + 1 }}</span>
						<el-input
							v-model="item.name"
							placeholder="请输入类型名称"
							clearable
							:disabled="item.isProtected"
						/>
						<el-icon
							class="expand-btn"
							:class="{ 'is-expanded': item.expanded }"
							@click="item.expanded = !item.expanded"
						>
							<ArrowRight />
						</el-icon>
						<el-button
							v-if="!item.isProtected"
							class="del-btn"
							link
							type="danger"
							@click="typeItems.splice(idx, 1)"
						>
							<el-icon><Delete /></el-icon>
						</el-button>
						<el-tooltip v-else content="系统内置类型，不可删除" placement="top">
							<el-icon class="lock-icon"><Lock /></el-icon>
						</el-tooltip>
					</div>
					<div v-show="item.expanded" class="edit-advanced">
						<div class="advanced-field">
							<span class="field-label">Key</span>
							<el-input
								v-model="item.key"
								placeholder="为空时自动生成"
								:disabled="item.keyLocked"
								clearable
							/>
						</div>
					</div>
				</div>

				<div v-if="!typeItems.length" class="empty-row">暂无数据</div>
			</el-scrollbar>

			<el-button
				class="add-btn"
				plain
				type="primary"
				@click="addType"
			>
				<el-icon><Plus /></el-icon> 添加类型
			</el-button>
		</div>

		<template #footer>
			<el-button @click="typeEditVisible = false">取消</el-button>
			<el-button type="primary" :loading="typeSaving" @click="saveTypes"
				>保存</el-button
			>
		</template>
	</el-dialog>

<!-- 子弹窗 2：编辑某类型下的标签（树形结构，支持无限下级） -->
<el-dialog
	v-model="tagEditVisible"
	:title="`编辑标签 - ${tagEditingType?.name || ''}`"
	width="560px"
	append-to-body
	class="tag-sub-edit-dialog"
	:close-on-click-modal="false"
>
	<div class="edit-tip">
		<el-icon><InfoFilled /></el-icon>
		修改名称、新增或删除项，保存后将同步更新
		<span class="advanced-toggle" @click="toggleTagAllExpanded">
			<el-icon class="setting-icon" :class="{ 'is-active': tagAllExpanded }"><Setting /></el-icon>
			<span>高级配置</span>
		</span>
	</div>

	<div class="edit-list">
		<el-scrollbar ref="tagScrollRef" max-height="420px">
			<TagEditNode
				v-for="item in tagItems"
				:key="item._uid"
				:item="item"
				:depth="0"
				@add-child="onAddChild"
				@delete="onDeleteTagItem"
			/>
			<div v-if="!tagItems.length" class="empty-row">暂无数据</div>
		</el-scrollbar>

		<el-button
			class="add-btn"
			plain
			type="primary"
			@click="addTag"
		>
			<el-icon><Plus /></el-icon> 添加标签
		</el-button>
	</div>

	<template #footer>
		<el-button @click="tagEditVisible = false">取消</el-button>
		<el-button type="primary" :loading="tagSaving" @click="saveTags"
			>保存</el-button
		>
	</template>
</el-dialog>
</template>

<script lang="ts" name="clues-tag-manager" setup>
import { ref, reactive, watch, nextTick } from "vue";
import { ElMessage } from "element-plus";
import { EditPen, InfoFilled, Delete, Plus, ArrowRight, Setting, Lock } from "@element-plus/icons-vue";
import { useCool } from "/@/cool";
import { useDict } from "/$/dict";
import { parseLabelJson, toValueArray, KNOWN_DB_FIELDS } from "../../utils/tagDict";
import TagEditNode from "./tagEditNode.vue";

const { service } = useCool();
const { dict } = useDict();

// ===== 工具函数：生成随机字母串 =====
function genRandomKey(existingSet: Set<string>, len = 6): string {
	const chars = "abcdefghijklmnopqrstuvwxyz";
	for (let attempt = 0; attempt < 100; attempt++) {
		let key = "";
		for (let i = 0; i < len; i++) {
			key += chars.charAt(Math.floor(Math.random() * chars.length));
		}
		if (!existingSet.has(key)) return key;
	}
	// 极端情况：加长到8位
	let key = "";
	for (let i = 0; i < 8; i++) {
		key += chars.charAt(Math.floor(Math.random() * chars.length));
	}
	return key;
}

const emit = defineEmits<{
	(e: "saved"): void;
}>();

const props = defineProps<{
	cluesInfo?: any;
	cluesId?: string | number;
}>();

// ===== 主弹窗状态 =====
const visible = ref(false);
const loading = ref(false);
const saving = ref(false);

// 类型列表
const types = ref<any[]>([]);
// 按 typeId 分组的标签
const tagsByType = reactive<Record<string | number, any[]>>({});
// 当前选中的类型 id
const activeTypeId = ref<string | number | null>(null);

// 字典类型 key → 线索字段名的映射（与 editInfo/clues 保持一致）
const TYPE_KEY_TO_FIELD: Record<string, string> = {
	cluesLevel: "level",
	sourceFrom: "sourceFrom",
	source_from: "sourceFrom",
	followupType: "followupType",
	followup_type: "followupType",
	householdType: "householdType",
	household_type: "householdType",
	education: "education",
	customerStatus: "customerStatus",
	customer_status: "customerStatus"
};

// 禁止修改 key 的字典类型（值保存到 clues/attr 表对应字段）
const LOCKED_TYPE_KEYS = new Set([
	"cluesLevel", "education", "sourceFrom", "source_from",
	"householdType", "household_type", "followupType", "followup_type"
]);

// 禁止删除和编辑名称的字典类型（核心业务字段，不可修改）
const PROTECTED_TYPE_KEYS = new Set(["cluesLevel", "education", "sourceFrom", "householdType"]);

// 已选中的标签 id 集合（用户可点击切换）
const selectedTagIds = ref<Set<string | number>>(new Set());

// 多选类型 key 列表（线索等级支持多选，其他单选）
const MULTI_SELECT_KEYS = new Set(["cluesLevel"]);

// 根据线索数据初始化选中状态
function initSelectedTags() {
	const set = new Set<string | number>();
	const info = props.cluesInfo;
	if (!info) { selectedTagIds.value = set; return; }

	const labelJson = parseLabelJson(info.labelJson);

	types.value.forEach((type: any) => {
		const typeKey = type.key;
		if (!typeKey) return;
		// 优先从已知字段取值，其次从 labelJson 取值
		const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
		const raw = info[fieldName] ?? labelJson[typeKey];
		if (raw === undefined || raw === null || raw === "") return;

		const values = toValueArray(raw);
		const tags = tagsByType[type.id] || [];
		values.forEach((v) => {
			const matched = tags.find((t: any) => String(t.value) === String(v) || String(t.name) === String(v));
			if (matched) set.add(matched.id);
		});
	});

	selectedTagIds.value = set;
}

// 点击标签切换选中
function toggleTag(tag: any, type: any) {
	const isMulti = MULTI_SELECT_KEYS.has(type.key);

	if (selectedTagIds.value.has(tag.id)) {
		// 取消选中
		selectedTagIds.value.delete(tag.id);
	} else {
		if (!isMulti) {
			// 单选：先取消同类型其他标签
			const sameTypeTags = tagsByType[type.id] || [];
			sameTypeTags.forEach((t: any) => selectedTagIds.value.delete(t.id));
		}
		selectedTagIds.value.add(tag.id);
	}
	// 触发响应式更新
	selectedTagIds.value = new Set(selectedTagIds.value);
}

// 类型分组 DOM 引用
const groupRefs: Record<string | number, HTMLElement> = {};
function setGroupRef(el: any, id: string | number) {
	if (el) groupRefs[id] = el;
}

// 获取某类型下的顶级标签（无 parentId 的）
function getRootTags(typeId: string | number) {
	const tags = tagsByType[typeId] || [];
	return tags.filter((t: any) => !t.parentId || String(t.parentId) === "0" || String(t.parentId) === "");
}

// 获取某类型下某父标签的子标签
function getChildTags(typeId: string | number, parentId: string | number) {
	const tags = tagsByType[typeId] || [];
	return tags.filter((t: any) => String(t.parentId) === String(parentId));
}

// ===== 打开主弹窗 =====
async function open() {
	visible.value = true;
	await loadAll();
}

// 加载所有数据
async function loadAll() {
	loading.value = true;
	try {
		// 一次性加载所有字典项，仅保留属于 customer_pro 插件的项
		// （与 app 端 tagTypes/tagInfos 逻辑保持一致：addonsName = 'customer_pro'）
		const all: any[] = await service.dict.info.list({});
		const customerProInfos = (all || []).filter((item: any) => item.addonsName === "customer_pro");
		const typeIdsWithInfo = new Set(customerProInfos.map((item: any) => item.typeId));

		// 清空旧数据并按 typeId 分组
		Object.keys(tagsByType).forEach((k) => delete (tagsByType as any)[k]);
		customerProInfos.forEach((item: any) => {
			const tid = item.typeId;
			if (!tagsByType[tid]) tagsByType[tid] = [];
			tagsByType[tid].push(item);
		});

		// 类型列表：仅显示 isPublic=1 且属于 customer_pro 插件、且存在对应字典项的类型
		// （对齐 app 端 TagTypes：id IN (info WHERE addonsName='customer_pro') AND addonsName='customer_pro'）
		const typeList: any[] = await service.dict.type.list({
			order: "createTime",
			sort: "asc"
		});
		types.value = (typeList || []).filter(
			(t: any) =>
				String(t.isPublic) === "1" &&
				t.addonsName === "customer_pro" &&
				typeIdsWithInfo.has(t.id)
		);

		// 默认选中第一项
		if (types.value.length && !activeTypeId.value) {
			activeTypeId.value = types.value[0].id;
		}
	} catch (e: any) {
		ElMessage.error(e?.message || "加载失败");
	} finally {
		loading.value = false;
		initSelectedTags();
	}
}

// 切换选中类型时，滚动到对应分组
watch(activeTypeId, (id) => {
	if (id == null) return;
	nextTick(() => {
		const el = groupRefs[id];
		if (el && typeof el.scrollIntoView === "function") {
			el.scrollIntoView({ behavior: "smooth", block: "start" });
		}
	});
});

// ===== 类型批量编辑 =====
const typeEditVisible = ref(false);
const typeItems = ref<{ id?: any; name: string; key: string; keyLocked: boolean; isProtected: boolean; expanded: boolean }[]>([]);
const typeSaving = ref(false);
const typeScrollRef = ref<any>();

const typeAllExpanded = ref(false);

function toggleTypeAllExpanded() {
	typeAllExpanded.value = !typeAllExpanded.value;
	typeItems.value.forEach((item) => { item.expanded = typeAllExpanded.value; });
}

function openTypeEdit() {
	typeAllExpanded.value = false;
	typeItems.value = types.value.map((t) => ({
		id: t.id,
		name: t.name,
		key: t.key || "",
		keyLocked: LOCKED_TYPE_KEYS.has(t.key),
		isProtected: PROTECTED_TYPE_KEYS.has(t.key),
		expanded: false
	}));
	typeEditVisible.value = true;
}

// 添加一项后滚动到底部
function addType() {
	typeItems.value.push({ name: "", key: "", keyLocked: false, isProtected: false, expanded: typeAllExpanded.value });
	nextTick(() => {
		const wrap = typeScrollRef.value?.wrapRef as HTMLElement | undefined;
		if (wrap) {
			wrap.scrollTop = wrap.scrollHeight;
		}
	});
}

async function saveTypes() {
	// 过滤有效项（name 非空）
	const validItems = typeItems.value.filter((it) => (it.name || "").trim());
	const names = validItems.map((it) => (it.name || "").trim());

	// 校验重名
	const nameSet = new Set<string>();
	for (const n of names) {
		if (nameSet.has(n)) {
			ElMessage.error(`类型名称重复：${n}`);
			return;
		}
		nameSet.add(n);
	}

	typeSaving.value = true;
	try {
		const oldList = [...types.value];
		const oldMap = new Map(oldList.map((t) => [t.id, t]));

		// 收集已有 key 集合，用于去重
		const existingKeys = new Set(oldList.map((t) => t.key).filter(Boolean));

		// 删除：旧 id 不在新列表里（受保护类型不允许删除）
		const validIds = new Set(validItems.filter((it) => it.id).map((it) => it.id));
		const toDelete = oldList.filter((t) => !validIds.has(t.id) && !PROTECTED_TYPE_KEYS.has(t.key));
		if (toDelete.length) {
			await service.dict.type.delete({ ids: toDelete.map((t) => t.id) });
		}

		// 遍历有效项，新增或更新
		for (const item of validItems) {
			const name = (item.name || "").trim();
			const key = (item.key || "").trim() || genRandomKey(existingKeys);
			existingKeys.add(key);

			if (item.id && oldMap.has(item.id)) {
				// 已有项：受保护类型不允许修改名称和 key
				if (item.isProtected) {
					continue;
				}
				// 检查是否有变化
				const old = oldMap.get(item.id);
				if (old.name !== name || old.key !== key) {
					await service.dict.type.update({
						id: item.id,
						name,
						key
					});
				}
			} else {
				// 新增
				await service.dict.type.add({
					name,
					key,
					addonsName: "customer_pro",
					isPublic: 1
				});
			}
		}

		ElMessage.success("保存成功");
		typeEditVisible.value = false;
		await loadAll();
		// 刷新全局字典
		dict.refresh && dict.refresh();
		emit("saved");
	} catch (e: any) {
		ElMessage.error(e?.message || "保存失败");
	} finally {
		typeSaving.value = false;
	}
}

// ===== 标签批量编辑（树形结构） =====
interface TagEditItem {
	_uid: number;          // 唯一标识（用于 v-for key）
	id?: any;              // 数据库 id（已有项）
	name: string;
	value: string;
	orderNum: number;
	parentId: string;      // 父级 id（数据库 id 或空字符串表示顶级）
	expanded: boolean;     // 高级配置展开
	collapsed: boolean;    // 子级折叠
	children: TagEditItem[];
}

let tagUidCounter = 0;
function newTagItem(partial: Partial<TagEditItem> = {}): TagEditItem {
	return {
		_uid: ++tagUidCounter,
		name: "",
		value: "",
		orderNum: 1,
		parentId: "",
		expanded: !!partial.expanded,
		collapsed: false,
		children: [],
		...partial
	};
}

// 将扁平的标签列表构建为树形结构
function buildTagTree(flatList: any[]): TagEditItem[] {
	const map = new Map<string, TagEditItem>();
	const roots: TagEditItem[] = [];

	// 先创建所有节点
	for (const t of flatList) {
		map.set(String(t.id), newTagItem({
			id: t.id,
			name: t.name || "",
			value: t.value || "",
			orderNum: t.orderNum || 1,
			parentId: t.parentId ? String(t.parentId) : "",
			expanded: false,
			children: []
		}));
	}

	// 构建树
	for (const t of flatList) {
		const node = map.get(String(t.id))!;
		const pid = t.parentId ? String(t.parentId) : "";
		if (pid && map.has(pid)) {
			map.get(pid)!.children.push(node);
		} else {
			roots.push(node);
		}
	}

	return roots;
}

// 将树形结构扁平化（用于保存）
function flattenTagTree(items: TagEditItem[], parentId: string = ""): TagEditItem[] {
	const result: TagEditItem[] = [];
	for (const item of items) {
		result.push({ ...item, parentId, children: [] });
		if (item.children.length) {
			result.push(...flattenTagTree(item.children, String(item.id || item._uid)));
		}
	}
	return result;
}

const tagEditVisible = ref(false);
const tagItems = ref<TagEditItem[]>([]);
const tagSaving = ref(false);
const tagEditingType = ref<any>(null);
const tagScrollRef = ref<any>();

const tagAllExpanded = ref(false);

function toggleTagAllExpanded() {
	tagAllExpanded.value = !tagAllExpanded.value;
	function setExpanded(items: TagEditItem[]) {
		for (const item of items) {
			item.expanded = tagAllExpanded.value;
			if (item.children.length) setExpanded(item.children);
		}
	}
	setExpanded(tagItems.value);
}

function openTagEdit(type: any) {
	tagAllExpanded.value = false;
	tagEditingType.value = type;
	const list = tagsByType[type.id] || [];
	tagItems.value = buildTagTree(list);
	tagEditVisible.value = true;
}

// 添加顶级标签
function addTag() {
	tagItems.value.push(newTagItem({ expanded: tagAllExpanded.value }));
	nextTick(() => {
		const wrap = tagScrollRef.value?.wrapRef as HTMLElement | undefined;
		if (wrap) wrap.scrollTop = wrap.scrollHeight;
	});
}

// 添加子级标签
function onAddChild(parentItem: TagEditItem) {
	if (!parentItem.children) parentItem.children = [];
	parentItem.children.push(newTagItem({ expanded: tagAllExpanded.value }));
	parentItem.collapsed = false;
}

// 删除标签（递归查找并移除）
function onDeleteTagItem(item: TagEditItem) {
	function removeFromList(list: TagEditItem[]): boolean {
		const idx = list.findIndex((i) => i._uid === item._uid);
		if (idx >= 0) {
			list.splice(idx, 1);
			return true;
		}
		for (const i of list) {
			if (i.children && removeFromList(i.children)) return true;
		}
		return false;
	}
	removeFromList(tagItems.value);
}

async function saveTags() {
	if (!tagEditingType.value) return;

	// 扁平化并过滤有效项
	const flatItems = flattenTagTree(tagItems.value);
	const validItems = flatItems.filter((it) => (it.name || "").trim());
	const names = validItems.map((it) => (it.name || "").trim());

	// 校验重名
	const nameSet = new Set<string>();
	for (const n of names) {
		if (nameSet.has(n)) {
			ElMessage.error(`标签名称重复：${n}`);
			return;
		}
		nameSet.add(n);
	}

	tagSaving.value = true;
	try {
		const typeId = tagEditingType.value.id;
		const oldList = tagsByType[typeId] || [];
		const oldMap = new Map(oldList.map((t: any) => [t.id, t]));

		// 收集已有 value 集合，用于去重
		const existingValues = new Set(oldList.map((t: any) => t.value).filter(Boolean));

		// 删除：旧 id 不在新列表里
		const validIds = new Set(validItems.filter((it) => it.id).map((it) => it.id));
		const toDelete = oldList.filter((t: any) => !validIds.has(t.id));
		if (toDelete.length) {
			await service.dict.info.delete({ ids: toDelete.map((t: any) => t.id) });
		}

		// 遍历有效项，新增或更新
		// 新增项的 id 暂时用 _uid 占位，新增后需要获取真实 id 作为子级的 parentId
		const uidToRealId = new Map<number, string>();

		for (const item of validItems) {
			const name = (item.name || "").trim();
			const value = (item.value || "").trim() || genRandomKey(existingValues);
			existingValues.add(value);

			// 解析 parentId：可能是数据库 id，也可能是新增项的 _uid
			let resolvedParentId = item.parentId || "";
			if (resolvedParentId && uidToRealId.has(Number(resolvedParentId))) {
				resolvedParentId = uidToRealId.get(Number(resolvedParentId))!;
			}

			if (item.id && oldMap.has(item.id)) {
				// 已有项：检查是否有变化
				const old = oldMap.get(item.id);
				if (old.name !== name || old.value !== value || old.orderNum !== item.orderNum || String(old.parentId || "") !== resolvedParentId) {
					await service.dict.info.update({
						id: item.id,
						name,
						value,
						orderNum: item.orderNum,
						parentId: resolvedParentId || undefined
					});
				}
				uidToRealId.set(item._uid, String(item.id));
			} else {
				// 新增
				const res = await service.dict.info.add({
					name,
					value,
					typeId,
					orderNum: item.orderNum || 1,
					addonsName: "customer_pro",
					parentId: resolvedParentId || undefined
				});
				// 记录新增项的真实 id
				if (res) {
					const realId = res.id || res;
					uidToRealId.set(item._uid, String(realId));
				}
			}
		}

		ElMessage.success("保存成功");
		tagEditVisible.value = false;
		await loadAll();
		dict.refresh && dict.refresh();
		emit("saved");
	} catch (e: any) {
		ElMessage.error(e?.message || "保存失败");
	} finally {
		tagSaving.value = false;
	}
}

// ===== 保存选中标签到线索 =====
async function saveSelectedTags() {
	if (!props.cluesId) {
		ElMessage.warning("缺少线索ID");
		return;
	}

	// 构建更新数据：遍历所有类型，收集选中的标签值
	const updateData: Record<string, any> = { id: props.cluesId };
	const customLabels: Record<string, any> = {};

	types.value.forEach((type: any) => {
		const typeKey = type.key;
		if (!typeKey) return;

		const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
		const tags = tagsByType[type.id] || [];
		const isMulti = MULTI_SELECT_KEYS.has(typeKey);
		const selectedTags = tags.filter((t: any) => selectedTagIds.value.has(t.id));

		let value: any;
		if (isMulti) {
			const values = selectedTags.map((t: any) => t.value || t.name);
			value = values.length ? values : "";
		} else {
			value = selectedTags.length ? (selectedTags[0].value || selectedTags[0].name) : "";
		}

		if (KNOWN_DB_FIELDS.has(fieldName)) {
			// 已知字段：多选转 JSON 字符串（后端 do 结构体 any 类型传数组会导致 ORM 写入异常）
			updateData[fieldName] = (isMulti && Array.isArray(value) && value.length) ? JSON.stringify(value) : value;
		} else if (value !== "" && value !== null && value !== undefined) {
			// 自定义字段：存入 labelJson
			customLabels[typeKey] = value;
		}
	});

	// 合并自定义标签到 labelJson（统一处理新增和清空）
	const existingLabelJson = parseLabelJson(props.cluesInfo?.labelJson);
	const merged = { ...existingLabelJson, ...customLabels };
	// 清除本次处理中被清空的 key
	types.value.forEach((type: any) => {
		const typeKey = type.key;
		if (!typeKey) return;
		if (!KNOWN_DB_FIELDS.has(TYPE_KEY_TO_FIELD[typeKey] || typeKey) && !customLabels[typeKey]) {
			delete merged[typeKey];
		}
	});
	if (Object.keys(merged).length) {
		updateData.labelJson = JSON.stringify(merged);
	} else if (Object.keys(existingLabelJson).length) {
		updateData.labelJson = "";
	}

	saving.value = true;
	try {
		await service.customer_pro.clues.update(updateData);
		ElMessage.success("保存成功");
		visible.value = false;
		emit("saved");
	} catch (e: any) {
		ElMessage.error(e?.message || "保存失败");
	} finally {
		saving.value = false;
	}
}

defineExpose({ open });
</script>

<style lang="scss">
.tag-manager-dialog {
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

	.tag-manager {
		display: flex;
		height: 560px;

		// 左侧：占 30%
		.left-panel {
			width: 200px;
			min-width: 200px;
			border-right: 1px solid #f0f1f5;
			display: flex;
			flex-direction: column;
			background: #fafbfd;

			.left-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
				letter-spacing: 0.5px;
			}

			.left-list {
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
						position: relative;

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

			.left-footer {
				padding: 12px 14px;
				border-top: 1px solid #f0f1f5;
				background: #fff;
				text-align: center;

				.edit-link {
					font-size: 13px;
					color: var(--color-primary);
					cursor: pointer;

					&:hover {
						text-decoration: underline;
					}
				}
			}
		}

		// 右侧：占 70%
		.right-panel {
			flex: 1;
			overflow: hidden;
			padding: 6px 4px 6px 0;
			background: #fff;

			.empty-tip {
				padding: 80px 0;
			}

			.group {
				margin: 14px 18px;
				padding: 14px 16px;
				background: #fafbfd;
				border: 1px solid #f0f1f5;
				border-radius: 8px;
				transition: box-shadow 0.2s;

				&:hover {
					box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
					border-color: #e4e7ed;
				}

				.group-header {
					display: flex;
					align-items: center;
					justify-content: space-between;
					margin-bottom: 10px;

					.group-title {
						display: flex;
						align-items: center;
						font-size: 14px;
						font-weight: 600;
						color: #1f2329;

						&::before {
							content: "";
							display: inline-block;
							width: 3px;
							height: 14px;
							background: var(--color-primary);
							border-radius: 2px;
							margin-right: 8px;
						}
					}

					.el-button {
						padding: 4px 10px;
						height: 26px;
						font-size: 12px;
						color: var(--color-primary);

						.edit-text {
							margin-left: 4px;
						}

						&:hover {
							background: rgba(64, 158, 255, 0.08);
						}
					}
				}

				.group-tags {
					display: flex;
					flex-wrap: wrap;
					align-items: center;
					gap: 6px 10px;
					padding: 4px 0 2px;
					font-size: 13px;
					line-height: 1.8;

					.tag-text {
						display: inline-block;
						padding: 3px 10px;
						color: #4e5969;
						background: #fff;
						border: 1px solid #e4e7ed;
						border-radius: 4px;
						white-space: nowrap;
						transition: all 0.18s;
						cursor: pointer;
						user-select: none;

						&:hover {
							color: var(--color-primary);
							border-color: var(--color-primary);
							background: #f0f7ff;
						}

						&.is-selected {
							color: #fff;
							background: var(--color-primary);
							border-color: var(--color-primary);
							box-shadow: 0 2px 6px rgba(64, 158, 255, 0.3);
						}

						&.tag-text--child {
							padding-left: 6px;
							color: #6b7785;
							background: #f9fafb;
							border-color: #e8eaed;
							font-size: 12px;

							.child-arrow {
								font-size: 10px;
								margin-right: 2px;
								color: #c0c4cc;
							}

							&:hover {
								color: var(--color-primary);
								border-color: var(--color-primary);
								background: #f0f7ff;

								.child-arrow {
									color: var(--color-primary);
								}
							}

							&.is-selected {
								color: #fff;
								background: var(--color-primary);
								border-color: var(--color-primary);

								.child-arrow {
									color: #fff;
								}
							}
						}
					}

					.empty-text {
						color: #a8abb2;
						font-size: 12px;
						font-style: italic;
					}
				}
			}
		}
	}
}

.tag-sub-edit-dialog {
	border-radius: 10px;

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
		padding: 16px 20px !important;
	}

	.el-dialog__footer {
		padding: 12px 20px;
		border-top: 1px solid #f0f1f5;
	}

	.edit-tip {
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

		.advanced-toggle {
			margin-left: auto;
			display: inline-flex;
			align-items: center;
			gap: 4px;
			cursor: pointer;
			font-size: 12px;
			color: #909399;
			user-select: none;
			transition: color 0.2s;

			&:hover {
				color: var(--color-primary);
			}

			.setting-icon {
				font-size: 16px;
				transition: transform 0.3s ease, color 0.2s;

				&.is-active {
					transform: rotate(90deg);
					color: var(--color-primary);
				}
			}
		}
	}

	.edit-list {
		.el-scrollbar {
			padding-right: 4px;
		}

		.edit-item-wrap {
			margin-bottom: 8px;
		}

		.edit-item {
			display: flex;
			align-items: center;
			gap: 8px;
			padding: 4px;
			border-radius: 6px;
			transition: background 0.18s;

			&:hover {
				background: #fafbfd;
			}

			.idx {
				display: inline-flex;
				align-items: center;
				justify-content: center;
				width: 24px;
				height: 24px;
				font-size: 12px;
				font-weight: 500;
				color: #909399;
				background: #f0f2f5;
				border-radius: 50%;
				flex-shrink: 0;
			}

			.el-input {
				flex: 1;

				.el-input__wrapper {
					border-radius: 6px;
				}
			}

			.expand-btn {
				flex-shrink: 0;
				font-size: 14px;
				color: #c0c4cc;
				cursor: pointer;
				transition: transform 0.2s ease, color 0.2s;

				&:hover {
					color: var(--color-primary);
				}

				&.is-expanded {
					transform: rotate(90deg);
					color: var(--color-primary);
				}
			}

			.del-btn {
				flex-shrink: 0;
				padding: 6px;

				.el-icon {
					font-size: 16px;
				}
			}

			.lock-icon {
				flex-shrink: 0;
				font-size: 14px;
				color: #c0c4cc;
				cursor: not-allowed;
			}
		}

		.edit-advanced {
			margin: 4px 0 4px 32px;
			padding: 10px 12px;
			background: #f7f8fa;
			border-left: 3px solid var(--color-primary);
			border-radius: 0 6px 6px 0;

			.advanced-field {
				display: flex;
				align-items: flex-start;
				gap: 8px;
				margin-bottom: 8px;

				&:last-child {
					margin-bottom: 0;
				}

				.field-label {
					display: inline-flex;
					align-items: center;
					flex-shrink: 0;
					width: 36px;
					height: 32px;
					font-size: 12px;
					color: #909399;
					justify-content: flex-end;
				}

				.el-input,
				.el-input-number {
					flex: 1;
				}

				.el-input-number {
					width: 120px;
				}
			}
		}

		.empty-row {
			padding: 30px 0;
			text-align: center;
			color: #a8abb2;
			font-size: 13px;
		}

		.add-btn {
			width: 100%;
			margin-top: 8px;
			height: 36px;
			border-radius: 6px;
			border-style: dashed;
			font-size: 13px;

			.el-icon {
				margin-right: 4px;
			}
		}
	}
}
</style>
