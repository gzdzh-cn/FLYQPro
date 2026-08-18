<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn />
			<cl-flex1 />
			<!-- 关键字搜索 -->
			<cl-search-key />
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="false" />
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert">
			<template #slot-fields="{ scope }">
				<el-button size="large" :icon="Plus" style="width: 100%" @click="btn = false"
					v-show="btn">添加</el-button>

				<div v-show="!btn">
					<el-row :gutter="10" v-for="(item, index) in scope.fields" :key="index">
						<el-col :span="9"><el-input placeholder="属性名" v-model="item.name" /></el-col>
						<el-col :span="1">
							<div style="text-align: center">=</div>
						</el-col>

						<el-col :span="9"><el-input placeholder="属性值" v-model="item.value" /></el-col>
						<el-col :span="5">
							<el-button type="primary" size="small" :icon="Minus" circle
								@click="editAttr('reduce', index)" />
							<el-button type="primary" size="small" :icon="Plus" circle @click="editAttr('add')" />
						</el-col>
					</el-row>
				</div>
			</template>
		</cl-upsert>
	</cl-crud>
</template>

<script lang="ts" name="customer_pro-wxTemplate" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { Plus, Minus } from "@element-plus/icons-vue";
import { h, ref } from "vue";
import { forEach, min } from "lodash";
import { ElMessage } from "element-plus";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "类型",
			prop: "type",
			span: 12,
			required: true,
			component: {
				name: "el-select",
				options: [
					{
						label: "线索自动分配",
						value: 1
					},
					{
						label: "线索手动分配",
						value: 2
					},
					{
						label: "线索跟进",
						value: 3
					},
					{
						label: "订单成交",
						value: 4
					}
				]
			}
		},
		{
			label: "名称",
			prop: "name",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "公众号",
			prop: "mp_id",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "模板ID",
			prop: "template_id",
			required: true,
			component: { name: "el-input" }
		},
		// {
		// 	label: "模板字段",
		// 	prop: "fields",
		// 	hook: {
		// 		bind: (value, { form }) => {
		// 			if (value) {
		// 				btn.value = false;
		// 				return JSON.parse(value);
		// 			}
		// 		}
		// 	},
		// 	rules: {
		// 		required: true,
		// 		validator(rule, value, callback) {
		// 			let fields = value;
		// 			const indicesToRemove = new Set(); // 要删除的下标
		// 			for (let index = 0; index < fields.length; index++) {
		// 				const ele = fields[index];
		// 				if (ele.name == "" && ele.value == "") {
		// 					indicesToRemove.add(index);
		// 				}
		// 			}
		// 			fields = fields.filter((_: any, index: number) => !indicesToRemove.has(index));
		// 			if (fields.length == 0) {
		// 				callback(new Error("不能为空"));
		// 			} else {
		// 				callback();
		// 			}
		// 		}
		// 	},
		// 	component: { name: "slot-fields" }
		// },
		{
			label: "状态",
			prop: "status",
			value: 1,
			component: {
				name: "cl-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "正常",
					inactiveText: "禁用",
					inlinePrompt: true
				}
			}
		}
	],
	onOpened(data) {
		if (Upsert.value?.mode == "add") {
			data.fields = rowForm.value;
		}
	},
	onSubmit(data, { next, close, done }) {
		console.log("data", data);

		// Upsert.value?.setForm("fields", rowForm.value);
		// let fields = data.fields;
		// const indicesToRemove = new Set(); // 要删除的下标
		// for (let index = 0; index < fields.length; index++) {
		// 	const ele = fields[index];
		// 	if (ele.name == "" && ele.value == "") {
		// 		indicesToRemove.add(index);
		// 	}
		// }

		// fields = fields.filter((_: any, index: number) => !indicesToRemove.has(index));
		next(data);
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{
			label: "类型",
			prop: "type",
			dict: [
				{
					label: "线索自动分配",
					value: 1
				},
				{
					label: "线索手动分配",
					value: 2
				},
				{
					label: "线索跟进",
					value: 3
				},
				{
					label: "订单成交",
					value: 4
				}
			]
		},
		{
			label: "公众号",
			prop: "mp_id",
			formatter(row) {
				const map: Record<number, string> = {
					1: "领南教育",
					2: "同伈服务",
					3: "炜创科技"
				};
				return map[row.mp_id] || row.mp_id;
			}
		},
		{ label: "模板名称", prop: "name" },
		{ label: "模板ID", prop: "template_id" },
		{
			label: "状态",
			prop: "status",
			formatter(row) {
				if (row.status === 0) {
					return h("span", { style: { color: "#d83b01" } }, "禁止");
				}
				if (row.status === 1) {
					return h("span", { style: { color: "#00b294" } }, "正常");
				}
				return row.status;
			}
		},
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.wxTemplate
	},
	(app) => {
		app.refresh();
	}
);

interface Row {
	name: string;
	value: string;
}
// 添加表单按钮
const btn = ref(true);
const rowForm = ref<Row[]>([{ name: "", value: "" }]);
// 增加一行
const editAttr = (str: string, index?: number) => {
	if (str == "add") {
		rowForm.value.push({ name: "", value: "" });
	}
	if (str == "reduce" && (index || index == 0)) {
		if (rowForm.value.length == 1) {
			btn.value = true;
			return;
		}
		rowForm.value.splice(index, 1);
	}
	Upsert.value?.setForm("fields", rowForm.value);
	console.log(rowForm.value);
};
</script>

<style>
.el-row {
	margin-bottom: 20px;
}
</style>
