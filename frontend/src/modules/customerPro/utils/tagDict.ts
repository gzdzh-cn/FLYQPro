/**
 * 线索标签字典公共模块
 * 供 clues.vue（列表标签列）和 info.vue（详情标签区）复用
 */

import { ref } from "vue";
import { useCool } from "/@/cool";

// ==================== 标签配色（30色循环） ====================
export const TAG_PALETTE = [
	{ bg: "#ecf5ff", border: "#409eff", color: "#409eff" },
	{ bg: "#f0f9eb", border: "#67c23a", color: "#67c23a" },
	{ bg: "#fdf6ec", border: "#e6a23c", color: "#e6a23c" },
	{ bg: "#fef0f0", border: "#f56c6c", color: "#f56c6c" },
	{ bg: "#f4f0ff", border: "#a06fff", color: "#a06fff" },
	{ bg: "#fff0f5", border: "#d9369f", color: "#d9369f" },
	{ bg: "#e6f7ff", border: "#1890ff", color: "#1890ff" },
	{ bg: "#f6ffed", border: "#52c41a", color: "#52c41a" },
	{ bg: "#fff7e6", border: "#fa8c16", color: "#fa8c16" },
	{ bg: "#fff1f0", border: "#f5222d", color: "#f5222d" },
	{ bg: "#f9f0ff", border: "#722ed1", color: "#722ed1" },
	{ bg: "#fff0f6", border: "#eb2f96", color: "#eb2f96" },
	{ bg: "#e8f5e9", border: "#4caf50", color: "#4caf50" },
	{ bg: "#fce4ec", border: "#e91e63", color: "#e91e63" },
	{ bg: "#fff3e0", border: "#ff9800", color: "#ff9800" },
	{ bg: "#e3f2fd", border: "#1e88e5", color: "#1e88e5" },
	{ bg: "#f1f8e9", border: "#7cb342", color: "#7cb342" },
	{ bg: "#f3e5f5", border: "#9c27b0", color: "#9c27b0" },
	{ bg: "#ffebee", border: "#c62828", color: "#c62828" },
	{ bg: "#e0f7fa", border: "#00acc1", color: "#00acc1" },
	{ bg: "#fff8e1", border: "#f9a825", color: "#f9a825" },
	{ bg: "#ede7f6", border: "#5e35b1", color: "#5e35b1" },
	{ bg: "#e8eaf6", border: "#3949ab", color: "#3949ab" },
	{ bg: "#fbe9e7", border: "#d84315", color: "#d84315" },
	{ bg: "#e0f2f1", border: "#00897b", color: "#00897b" },
	{ bg: "#f1f0ff", border: "#6366f1", color: "#6366f1" },
	{ bg: "#fefce8", border: "#ca8a04", color: "#ca8a04" },
	{ bg: "#f0fdf4", border: "#16a34a", color: "#16a34a" },
	{ bg: "#fdf2f8", border: "#db2777", color: "#db2777" },
	{ bg: "#ecfdf5", border: "#059669", color: "#059669" }
];

// ==================== 字段 → 字典类型 key 映射 ====================
export const FIELD_TO_TYPE_KEY: Record<string, string> = {
	level: "cluesLevel",
	sourceFrom: "sourceFrom",
	followupType: "followupType",
	householdType: "householdType",
	education: "education",
	customerStatus: "customerStatus"
};

// ==================== 数据库已知字段集合 ====================
// clues 表 + clues_attr 表中已有的字段名（JSON key 格式）
// 这些字段在数据库中有对应列，可以直接保存；不在其中的字段存入 labelJson
export const KNOWN_DB_FIELDS = new Set([
	// clues 表字段
	"id", "createTime", "updateTime", "deletedAt", "serialId", "name",
	"accountId", "guestId", "projectId", "servicesId", "servicesIds", "mobile", "wechat", "weixin",
	"sourceFrom", "keywords", "followupType", "lastFollowupTime", "level", "oceanTime", "allotTime",
	"remark", "orderNum", "status", "filterRemark", "filterGroupIds", "dtype", "schoolName",
	// clues_attr 表字段
	"cluesId", "schoolId", "gender", "emergencyMobile", "education", "majorsId", "majorsType",
	"degreeId", "graduatedSchool", "householdType", "householdAddress", "address", "email", "qq",
	"chatContent", "city", "ip", "guestIpInfo", "fromPage", "talkPage", "landPage", "se",
	"chatContentVersion", "labelJson"
]);

// ==================== 字典数据状态（单例，全局只加载一次） ====================
const dictTypes = ref<{ id: any; name: string; key: string; idx: number }[]>([]);
const dictItemsByType = ref<Record<string, { value: string; name: string }[]>>({});
let loaded = false;

export function useDictMeta() {
	const { service } = useCool();

	async function loadDictMeta() {
		if (loaded) return;
		try {
			const [typeList, infoList] = await Promise.all([
				(service as any).dict.type.list({ order: "createTime", sort: "asc" }),
				(service as any).dict.info.list({})
			]);
			dictTypes.value = (typeList || []).map((t: any, i: number) => ({
				id: t.id,
				name: t.name,
				key: t.key || "",
				idx: i
			}));
			const map: Record<string, { value: string; name: string }[]> = {};
			(infoList || []).forEach((it: any) => {
				const tid = String(it.typeId);
				if (!map[tid]) map[tid] = [];
				map[tid].push({ value: String(it.value ?? it.name), name: it.name });
			});
			dictItemsByType.value = map;
			loaded = true;
		} catch (e) {
			console.error("加载字典数据失败:", e);
			dictTypes.value = [];
			dictItemsByType.value = {};
		}
	}

	/** 强制重新加载字典元数据（标签类型增删后调用） */
	function resetDictMeta() {
		loaded = false;
		dictTypes.value = [];
		dictItemsByType.value = {};
	}

	return { dictTypes, dictItemsByType, loadDictMeta, resetDictMeta };
}

// ==================== 工具函数 ====================

/** 判断字典项列表是否有层级关系（存在有效的 parentId） */
export function hasDictHierarchy(flatList: any[]): boolean {
	return flatList.some((it: any) => it.parentId && String(it.parentId) !== "0" && String(it.parentId) !== "");
}

/** 将扁平的字典项列表构建为树形结构（用于 el-tree-select） */
export function buildDictTree(flatList: any[]): any[] {
	const map = new Map<string, any>();
	const roots: any[] = [];

	// 先创建所有节点
	for (const it of flatList) {
		map.set(String(it.id), {
			id: it.value,       // el-tree-select 的 value 用 dict_info.value
			label: it.name,     // 显示名称
			rawId: it.id,       // 保留原始 id
			children: []
		});
	}

	// 构建树
	for (const it of flatList) {
		const pid = it.parentId ? String(it.parentId) : "";
		if (pid && map.has(pid)) {
			map.get(pid)!.children.push(map.get(String(it.id)));
		} else {
			roots.push(map.get(String(it.id)));
		}
	}

	// 清理空 children 数组（el-tree-select 叶子节点不需要 children）
	function cleanEmpty(nodes: any[]) {
		for (const n of nodes) {
			if (n.children.length === 0) {
				delete n.children;
			} else {
				cleanEmpty(n.children);
			}
		}
	}
	cleanEmpty(roots);

	return roots;
}

/** 解析字段值为字符串数组（支持单值/数组/JSON 字符串/逗号分隔） */
export function toValueArray(val: any): string[] {
	if (val === null || val === undefined || val === "") return [];
	if (Array.isArray(val)) return val.map(String).filter(Boolean);
	if (typeof val === "string") {
		// 先尝试 JSON 解析（处理 ["1","2"] 这类数组字符串）
		try {
			const parsed = JSON.parse(val);
			if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean);
			return [String(parsed)];
		} catch {
			// JSON 解析失败，再尝试逗号分隔（处理 "3,31" 这类值）
			if (val.includes(",")) {
				return val.split(",").map((v: string) => v.trim()).filter(Boolean);
			}
			return [val];
		}
	}
	return [String(val)];
}

/** 通过字典类型 key 查找 value 对应的 name */
export function dictLabel(typeKey: string, val: any): string {
	const type = dictTypes.value.find((t) => t.key === typeKey);
	if (!type) return "";
	const items = dictItemsByType.value[String(type.id)] || [];
	const found = items.find((it) => it.value === String(val));
	return found?.name || "";
}

// ==================== 标签类型 ====================
export interface TagItem {
	text: string;
	typeName?: string;
	effect?: "plain" | "dark" | "light";
	style: string;
}

// ==================== 从数据行/线索对象解析标签列表 ====================

/** 解析 labelJson 字段（自定义标签数据，存储在 clues_attr.label_json） */
export function parseLabelJson(raw: any): Record<string, any> {
	if (!raw) return {};
	if (typeof raw === "string") {
		try {
			const parsed = JSON.parse(raw);
			if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) return parsed;
		} catch { /* ignore */ }
		return {};
	}
	if (typeof raw === "object" && !Array.isArray(raw)) return raw;
	return {};
}

export function resolveTags(data: Record<string, any>): TagItem[] {
	const tags: TagItem[] = [];
	if (!dictTypes.value.length) return tags;

	const findTypeByKey = (key: string) => dictTypes.value.find((t) => t.key === key);

	const processField = (fieldName: string, type: { id: any; name: string; key: string }) => {
		const raw = data[fieldName];
		if (raw === undefined || raw === null || raw === "") return;
		const values = toValueArray(raw);
		if (!values.length) return;

		const items = dictItemsByType.value[String(type.id)] || [];
		values.forEach((v) => {
			const matched = items.find((it) => it.value === v || it.name === v);
			if (matched) {
				const c = TAG_PALETTE[tags.length % TAG_PALETTE.length];
				tags.push({
					text: matched.name,
					typeName: type.name,
					effect: "plain",
					style: `background:${c.bg};border-color:${c.border};color:${c.color}`
				});
			}
		});
	};

	// 解析 labelJson（自定义标签数据）
	const labelJson = parseLabelJson(data.labelJson);
	// 已处理的字典类型 key 集合（避免重复）
	const processedKeys = new Set<string>();

	// 1. 显式映射的字段
	Object.keys(FIELD_TO_TYPE_KEY).forEach((fieldName) => {
		const typeKey = FIELD_TO_TYPE_KEY[fieldName];
		const t = findTypeByKey(typeKey);
		if (t) {
			processField(fieldName, t);
			processedKeys.add(typeKey);
		}
	});

	// 2. 自动兜底：字典类型 key 直接等于数据字段名（clues/clues_attr 表中的固定列）
	dictTypes.value.forEach((t) => {
		if (!t.key) return;
		if (processedKeys.has(t.key)) return;
		const candidates = [t.key, t.key.charAt(0).toLowerCase() + t.key.slice(1)];
		for (const k of candidates) {
			if (data[k] !== undefined && data[k] !== null && data[k] !== "") {
				processField(k, t);
				processedKeys.add(t.key);
				break;
			}
		}
	});

	// 3. 从 labelJson 中解析自定义标签（不在 clues/clues_attr 固定列中的类型）
	dictTypes.value.forEach((t) => {
		if (!t.key) return;
		if (processedKeys.has(t.key)) return;
		const raw = labelJson[t.key];
		if (raw === undefined || raw === null || raw === "") return;

		const values = toValueArray(raw);
		if (!values.length) return;

		const items = dictItemsByType.value[String(t.id)] || [];
		values.forEach((v) => {
			const matched = items.find((it) => it.value === v || it.name === v);
			if (matched) {
				const c = TAG_PALETTE[tags.length % TAG_PALETTE.length];
				tags.push({
					text: matched.name,
					typeName: t.name,
					effect: "plain",
					style: `background:${c.bg};border-color:${c.border};color:${c.color}`
				});
			}
		});
	});

	return tags;
}
