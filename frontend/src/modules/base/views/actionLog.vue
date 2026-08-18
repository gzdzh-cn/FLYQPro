<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<!-- <cl-add-btn /> -->
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

<script lang="ts" name="base-sys-actionLog" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { ref } from "vue";

const { service } = useCool();
const { user } = useBase();
const isAdmin = ref(user.info.roleIds?.split(",").includes("1") || false);
// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "操作名称", prop: "name", required: true, component: { name: "el-input" } },
		{
			label: "操作备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		}
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		// { label: "id", prop: "id" },
		{ prop: "userName", label: "用户" },
		{ label: "操作类型", prop: "name" },
		{
			label: "操作备注",
			prop: "remark",
			showOverflowTooltip: true,
			minWidth: 200,
			formatter: (row: any) => {
				// 过滤 html 标签
				return row.remark.replace(/<[^>]*>?/g, "");
			}
		},
		{ label: "操作时间", prop: "createTime" },
		{ type: "op", buttons: ["info", "delete"], hidden: !isAdmin.value }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.base.sys.actionLog
	},
	(app) => {
		app.refresh();
	}
);
</script>
