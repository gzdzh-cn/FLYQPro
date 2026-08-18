<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn />
			<cl-export-btn :columns="Table?.columns">导出留言列表</cl-export-btn>
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

<script lang="ts" name="cms-form" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "姓名", prop: "name", required: true, component: { name: "el-input" } },
		{ label: "号码", prop: "phone", required: true, component: { name: "el-input" } },
		{
			label: "题目",
			prop: "remark",
			component: { name: "cl-editor-wang", props: { type: "textarea", rows: 26 } },
			required: true
		}
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "id", prop: "id" },
		{ label: "创建时间", prop: "createTime" },
		{ label: "更新时间", prop: "updateTime" },
		{ label: "姓名", prop: "name" },
		{ label: "号码", prop: "phone" },
		{ label: "题目", prop: "remark", showOverflowTooltip: true },
		{ type: "op", buttons: ["info", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.dzhCms.form
	},
	(app) => {
		app.refresh();
	}
);
</script>
