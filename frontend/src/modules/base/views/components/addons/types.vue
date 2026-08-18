<template>
	<cl-crud ref="Crud">
		<div style="padding: 0 10px 0px 20px; display: flex; flex-wrap: wrap; row-gap: 10px">
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn style="margin-right: 10px" />
			<!-- 关键字搜索 -->
			<cl-search-key />
			<cl-flex1 />
		</div>

		<div class="divider"></div>

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
		<cl-upsert ref="Upsert" />
	</cl-crud>
</template>

<script lang="ts" name="addons-types" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";

const { service } = useCool();

const Upsert = useUpsert({
	items: [
		{ label: "标题", prop: "name", required: true, component: { name: "el-input" } },
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		},
		{
			label: "排序",
			prop: "orderNum",
			value: 99,
			component: { name: "el-input-number", props: { min: 0 } },
			required: true
		}
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "标题", prop: "name" },
		{ label: "备注", prop: "remark", showOverflowTooltip: true },
		{ label: "排序", prop: "orderNum" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.base.sys.addonsTypes
	},
	(app) => {
		app.refresh();
	}
);
</script>
