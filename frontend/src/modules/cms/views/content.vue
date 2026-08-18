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

<script lang="ts" name="cms-content" setup>
import { useViewGroup } from "/@/modules/base/hooks";
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import Tree from "./components/content/tree.vue";

const { service } = useCool();
const { ViewGroup } = useViewGroup();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "名称", prop: "name", required: true, component: { name: "el-input" } },
		{ label: "图片", prop: "img", component: { name: "cl-upload" } },
		{
			label: "点击量",
			prop: "click",
			value: 0,
			component: {
				name: "el-input-number",
				props: { min: 0 }
			}
		},
		{
			label: "内容",
			prop: "content",
			component: { name: "cl-editor-wang" }
		},

		{
			label: "状态",
			prop: "status",
			value: 1,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } }
		}
	],

	onSubmit(data, { next }) {
		next({
			...data,
			typeId: ViewGroup.value?.selected?.id
		});
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "名称", prop: "name" },
		{ label: "点击量", prop: "click" },
		{ label: "排序", prop: "orderNum" },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.cms.content
});

// 刷新列表
function refresh(params?: any) {
	Crud.value?.refresh(params);
}
</script>
