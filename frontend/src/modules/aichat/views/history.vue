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

<script lang="ts" name="aichat-history" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	props: {
		labelWidth: "130px"
	},
	items: [
		{
			label: "资源类型",
			prop: "resource_type",
			span: 12,
			component: { name: "el-select", options: [] },
			required: true
		},
		{
			label: "违法信息类型",
			prop: "illegal_type",
			span: 12,
			component: { name: "el-select", options: [] },
			required: true
		},

		{
			label: "内容",
			prop: "content",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } },
			required: true
		}
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "资源类型", prop: "resource_type", dict: [] },
		{ label: "违法信息类型", prop: "illegal_type", dict: [] },
		{ label: "内容", prop: "content" },
		{ label: "处理状态", prop: "process_status" },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", width: 320, buttons: ["edit", "info", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.aichat.history
	},
	(app) => {
		app.refresh();
	}
);
</script>
