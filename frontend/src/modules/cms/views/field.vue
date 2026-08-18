<template>
	<cl-view-group ref="ViewGroup">
		<template #left>
			<Tree @refresh="refresh" />
		</template>
		<template #right>
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
					<cl-table ref="Table" />
				</cl-row>

				<cl-row>
					<cl-flex1 />
					<!-- 分页控件 -->
					<cl-pagination />
				</cl-row>

				<!-- 新增、编辑 -->
				<cl-upsert ref="Upsert" />
			</cl-crud>
		</template>
	</cl-view-group>
</template>

<script lang="ts" name="cms-field" setup>
import { useViewGroup } from "/@/modules/base/hooks";
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import Tree from "./components/field/tree.vue";

const { service } = useCool();
const { ViewGroup } = useViewGroup();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "标题", prop: "name", required: true, component: { name: "el-input" } },
		{
			label: "字段类型",
			prop: "fieldTypeId",
			required: true,
			component: {
				name: "el-select",
				options: []
			}
		},

		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		},
		{
			label: "状态",
			prop: "status",
			value: 1,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } }
		}
	],
	onOpen() {
		service.cms.fieldType.list().then((res) => {
			Upsert.value?.setOptions(
				"fieldTypeId",
				res.map((e) => {
					return {
						label: e.name,
						value: e.id
					};
				})
			);
		});
	},
	onSubmit(data, { next }) {
		next({
			...data,
			modelTypeId: ViewGroup.value?.selected?.id
		});
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "标题", prop: "name" },
		{ label: "备注", prop: "remark" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.cms.field
});

// 刷新列表
function refresh(params?: any) {
	Crud.value?.refresh(params);
}
</script>
