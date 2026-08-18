<template>
	<!-- 新增/编辑个人信息弹窗 -->
	<el-dialog v-model="editDialogVisible" :title="isAdd ? '新增线索' : '编辑个人信息'" width="70%" append-to-body
		:close-on-click-modal="false">
		<div class="edit-form-scroll">
			<cl-form ref="FormEdit" :inner="true">
				<template #slot-schoolId="{ scope }">
					<el-select v-model="scope.schoolId" @change="handleSchoolChange">
						<el-option v-for="item in schoolOptions" :key="item.id" :label="item.name" :value="item.id" />
					</el-select>
				</template>
				<template #slot-majorsId="{ scope }">
					<el-select v-model="scope.majorsId">
						<el-option v-for="item in majorsOptions" :key="item.id" :label="item.name" :value="item.id" />
					</el-select>
				</template>
			</cl-form>
		</div>
		<template #footer>
			<el-button @click="editDialogVisible = false">取消</el-button>
			<el-button type="primary" @click="handleEditSubmit">保存</el-button>
		</template>
	</el-dialog>
</template>

<script lang="ts" name="clues-edit-info" setup>
import { ref, nextTick } from "vue";
import { useCool } from "/@/cool";
import { useForm } from "@cool-vue/crud";
import { ElMessage } from "element-plus";
import { parseLabelJson, toValueArray, KNOWN_DB_FIELDS, buildDictTree, hasDictHierarchy } from "/@/modules/customerPro/utils/tagDict";

const props = defineProps<{
	cluesId?: string | number;
}>();

const emit = defineEmits<{
	(e: "saved"): void;
}>();

const { service } = useCool();

// ===== 字典类型 key → 线索字段名的映射（与 tagManager 中 TYPE_KEY_TO_FIELD 对应）=====
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

// 多选类型 key 列表
const MULTI_SELECT_KEYS = new Set(["cluesLevel"]);

// 标签类型元数据（typeKey → { fieldName, isMulti, isCustom }）
const tagTypeMeta = ref<Record<string, { fieldName: string; isMulti: boolean; isCustom: boolean }>>({});

// 加载标签类型（isPublic=1 的字典类型），返回表单项配置
async function loadTagTypeFormItems(): Promise<any[]> {
	try {
		const typeList: any[] = await service.dict.type.list({ order: "createTime", sort: "asc" });
		const publicTypes = (typeList || []).filter((t: any) => String(t.isPublic) === "1");

		if (!publicTypes.length) return [];

		// 加载所有字典项
		const infoList: any[] = await service.dict.info.list({});
		const itemsByTypeId: Record<string, any[]> = {};
		(infoList || []).forEach((it: any) => {
			const tid = String(it.typeId);
			if (!itemsByTypeId[tid]) itemsByTypeId[tid] = [];
			itemsByTypeId[tid].push(it);
		});

		const formItems: any[] = [];
		const meta: Record<string, { fieldName: string; isMulti: boolean; isCustom: boolean }> = {};

		publicTypes.forEach((t: any) => {
			const typeKey = t.key;
			if (!typeKey) return;

			const fieldName = TYPE_KEY_TO_FIELD[typeKey] || typeKey;
			const isMulti = MULTI_SELECT_KEYS.has(typeKey);
			const isCustom = !KNOWN_DB_FIELDS.has(fieldName);
			const dictItems = itemsByTypeId[String(t.id)] || [];
			const isHierarchical = hasDictHierarchy(dictItems);

			meta[typeKey] = { fieldName, isMulti, isCustom };

			if (isHierarchical) {
				// 有层级：使用 el-tree-select
				const treeData = buildDictTree(dictItems.filter((it: any) => it.value != null));
				formItems.push({
					label: t.name,
					prop: typeKey,
					span: 12,
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
			} else {
				// 无层级：使用 el-select
				formItems.push({
					label: t.name,
					prop: typeKey,
					span: 12,
					component: {
						name: "el-select",
						options: dictItems
							.filter((it: any) => it.value != null)
							.map((it: any) => ({ label: it.name, value: it.value })),
						props: isMulti ? { multiple: true, clearable: true, filterable: true } : { clearable: true, filterable: true }
					}
				});
			}
		});

		tagTypeMeta.value = meta;
		return formItems;
	} catch (e) {
		console.error("加载标签类型失败:", e);
		return [];
	}
}

// 将表单数据中的标签类型字段做映射转换：typeKey → fieldName，自定义字段提取到 labelJson
function processTagTypeFields(data: any, existingLabelJson?: any) {
	const customLabels: Record<string, any> = {};

	Object.entries(tagTypeMeta.value).forEach(([typeKey, meta]) => {
		const val = data[typeKey];
		if (val === undefined) return;
		// 删除 typeKey 字段（避免发送给后端无法识别的字段）
		delete data[typeKey];

		if (meta.isCustom) {
			// 自定义字段：提取到 customLabels，不写入 data
			if (val !== null && val !== "" && !(Array.isArray(val) && val.length === 0)) {
				customLabels[typeKey] = val;
			}
		} else {
			// 已知字段：映射到 fieldName
			let mappedVal = val;
			// 空数组时发送空字符串
			if (Array.isArray(mappedVal) && mappedVal.length === 0) {
				mappedVal = "";
			}
			// 多选：数组转 JSON 字符串
			if (meta.isMulti && Array.isArray(mappedVal) && mappedVal.length) {
				mappedVal = JSON.stringify(mappedVal);
			}
			data[meta.fieldName] = mappedVal;
		}
	});

	// 合并自定义标签到 labelJson
	if (Object.keys(customLabels).length > 0) {
		const existing = parseLabelJson(existingLabelJson);
		const merged = { ...existing, ...customLabels };
		data.labelJson = JSON.stringify(merged);
	} else {
		// 清空已有的自定义标签 key
		const existing = parseLabelJson(existingLabelJson);
		let changed = false;
		Object.entries(tagTypeMeta.value).forEach(([typeKey, meta]) => {
			if (!meta.isCustom) return;
			if (existing[typeKey] !== undefined) {
				delete existing[typeKey];
				changed = true;
			}
		});
		if (changed) {
			data.labelJson = Object.keys(existing).length ? JSON.stringify(existing) : "";
		}
	}
}

const editDialogVisible = ref(false);
const FormEdit = useForm();
const isAdd = ref(false);

// 构建表单项（新增和编辑共用，新增时隐藏部分字段）
function buildFormItems(tagTypeItems: any[]) {
	const baseItems: any[] = [];

	if (!isAdd.value) {
		baseItems.push(
			{
				label: "53标识",
				prop: "guestId",
				span: 12,
				component: { name: "el-input", props: { disabled: true } }
			},
			{
				label: "IP",
				prop: "ip",
				span: 12,
				component: { name: "el-input", props: { disabled: true } }
			},
			{
				label: "IP归属地",
				prop: "guestIpInfo",
				span: 12,
				component: { name: "el-input", props: { disabled: true } }
			}
		);
	}

	baseItems.push(
		{
			label: "项目",
			prop: "projectId",
			span: 12,
			required: true,
			component: { name: "el-select", props: isAdd.value ? {} : { disabled: true } }
		},
		{
			label: "姓名",
			prop: "name",
			span: 12,
			component: { name: "el-input" }
		},
		{ label: "关键字", prop: "keywords", span: 12, component: { name: "el-input" } },
		{ label: "手机号", prop: "mobile", span: 12, component: { name: "el-input" } },
		{ label: "微信号", prop: "wechat", span: 12, component: { name: "el-input" } },
		{
			label: "毕业院校",
			prop: "graduatedSchool",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "意向院校",
			prop: "schoolId",
			span: 12,
			component: { name: "slot-schoolId" }
		},
		{
			label: "意向专业",
			prop: "majorsId",
			span: 12,
			component: { name: "slot-majorsId" }
		},
		{ label: "报读类型", prop: "majorsType", span: 12, component: { name: "el-select" } },
		{ label: "报读层次", prop: "degreeId", span: 12, component: { name: "el-select" } },
		 
		{
			label: "户籍地址",
			prop: "householdAddress",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "性别",
			prop: "gender",
			span: 12,
			component: { name: "el-select" }
		},
		{
			label: "紧急联系人电话",
			prop: "emergencyMobile",
			span: 12,
			component: { name: "el-input" }
		}
	);

	if (!isAdd.value) {
		baseItems.push({
			label: "已推学校",
			prop: "schoolName",
			span: 12,
			component: { name: "el-input" }
		});
	}

	// 动态标签类型字段
	baseItems.push(...tagTypeItems);

	baseItems.push({
		label: "备注",
		prop: "remark",
		component: { name: "el-input", props: { type: "textarea", rows: 4 } }
	});

	return baseItems;
}

// 打开新增弹窗
const openAdd = async () => {
	isAdd.value = true;
	try {
		const tagTypeItems = await loadTagTypeFormItems();

		editDialogVisible.value = true;
		await nextTick();

		FormEdit.value?.open({
			title: "新增线索",
			width: "980px",
			props: {
				labelWidth: "120px"
			},
			items: buildFormItems(tagTypeItems),
			form: {},
			on: {
				async open() {
					getSchoolList();
					await loadSelectOptions();
				},
				close(done: any) {
					done();
					editDialogVisible.value = false;
				},
				submit(data: any, { close, done }: any) {
					// 处理标签类型字段映射：typeKey→fieldName，自定义字段→labelJson
					processTagTypeFields(data);
					// 手动新增：来源固定为 1（手动录入）
					data.sourceFrom = "1";
					service.customer_pro.clues
						.add(data)
						.then(() => {
							ElMessage.success("新增成功");
							done();
							close();
							editDialogVisible.value = false;
							emit("saved");
						})
						.catch((e: any) => {
							console.error("新增失败:", e);
							ElMessage.error(e?.message || "新增失败");
							done();
						});
				}
			},
			op: {
				hidden: true
			}
		});
	} catch (e) {
		console.error("打开新增弹窗失败:", e);
	}
};

// 打开编辑弹窗
const open = async () => {
	if (!props.cluesId) return;
	isAdd.value = false;
	try {
		const item = await service.customer_pro.clues.info({ id: props.cluesId });

		// 加载动态标签类型表单项
		const tagTypeItems = await loadTagTypeFormItems();

		// 将数据库字段值映射到 typeKey（表单 prop 用 typeKey）
		const labelJson = parseLabelJson(item.labelJson);
		Object.entries(tagTypeMeta.value).forEach(([typeKey, meta]) => {
			let raw: any;
			if (meta.isCustom) {
				// 自定义字段从 labelJson 取值
				raw = labelJson[typeKey];
			} else {
				// 已知字段从数据库字段取值
				raw = item[meta.fieldName];
			}
			if (raw === undefined || raw === null || raw === "") return;

			if (meta.isMulti) {
				const arr = toValueArray(raw);
				item[typeKey] = arr.length ? arr : [];
			} else {
				const arr = toValueArray(raw);
				item[typeKey] = arr.length ? arr[0] : "";
			}
		});

		editDialogVisible.value = true;
		await nextTick();

		FormEdit.value?.open({
			title: "编辑个人信息",
			width: "980px",
			props: {
				labelWidth: "120px"
			},
			items: buildFormItems(tagTypeItems),
			form: {
				...item
			},
			on: {
				async open() {
					getSchoolList();
					await loadSelectOptions();
				},
				close(done: any) {
					done();
					editDialogVisible.value = false;
				},
				submit(data: any, { close, done }: any) {
					// 处理标签类型字段映射：typeKey→fieldName，自定义字段→labelJson
					processTagTypeFields(data, item.labelJson);
					service.customer_pro.clues
						.update(data)
						.then(() => {
							ElMessage.success("保存成功");
							done();
							close();
							editDialogVisible.value = false;
							emit("saved");
						})
						.catch((e: any) => {
							console.error("保存失败:", e);
							ElMessage.error(e?.message || "保存失败");
							done();
						});
				}
			},
			op: {
				hidden: true
			}
		});
	} catch (e) {
		console.error("获取线索信息失败:", e);
	}
};

// 加载下拉选项（项目、报读类型、报读层次、性别、户口类型）
async function loadSelectOptions() {
	// 项目
	const projectList = await service.customer_pro.project.list();
	FormEdit.value?.setOptions(
		"projectId",
		projectList
			.map((e: any) => ({ label: e.name, value: e.id }))
			.filter((item: any) => item.value != null)
	);

	// 报读类型
	const majorsTypeList = await service.customer_pro.readtypes.list();
	FormEdit.value?.setOptions(
		"majorsType",
		majorsTypeList
			.map((e: any) => ({ label: e.name, value: e.id }))
			.filter((item: any) => item.value != null)
	);

	// 报读层次
	const degreeList = await service.customer_pro.readdegree.list();
	FormEdit.value?.setOptions(
		"degreeId",
		degreeList
			.map((e: any) => ({ label: e.name, value: e.id }))
			.filter((item: any) => item.value != null)
	);

	// 性别
	FormEdit.value?.setOptions("gender", [
		{ label: "保密", value: "0" },
		{ label: "男", value: "1" },
		{ label: "女", value: "2" }
	]);


}

// 学校列表
const schoolOptions = ref<any[]>([]);
const majorsOptions = ref<any[]>([]);
const getSchoolList = async () => {
	schoolOptions.value = await service.customer_pro.school.list();
	schoolOptions.value[0]?.id && getMajorList(schoolOptions.value[0]?.id);
};
// 学校改变
const handleSchoolChange = (v: any) => {
	majorsOptions.value = [];
	FormEdit.value?.setForm("majorsId", null);
	getMajorList(v);
};
// 专业列表
const getMajorList = async (v: any) => {
	majorsOptions.value = await service.customer_pro.majors.list({ schoolId: v });
};

// 提交编辑
const handleEditSubmit = () => {
	FormEdit.value?.submit();
};

defineExpose({ open, openAdd });
</script>

<style lang="scss">
/* 编辑弹窗 */
.edit-form-scroll {
	max-height: 60vh;
	overflow-y: auto;
	overflow-x: hidden;
	padding: 0 10px;

	.cl-form {
		width: 100%;

		.cl-form__container {
			width: 100%;

			.el-form {
				width: 100%;
			}

			.el-form-item__label {
				display: flex;
				align-items: center;
				justify-content: flex-end;
				width: 120px !important;
				flex: 0 0 120px !important;
				text-align: right;
			}
		}

		.cl-form__footer {
			display: none;
		}
	}
}
</style>
