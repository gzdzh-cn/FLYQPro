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

<script lang="ts" name="dzhchatgpt-order" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useDict } from "../../dict";

const { service } = useCool();
const { dict } = useDict();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "名称",
			prop: "name",
			required: true,
			component: { name: "el-input", props: { disabled: true } }
		},
		{
			label: "订单编号",
			prop: "tradeNo",
			component: { name: "el-input", props: { disabled: true } }
		},
		{
			label: "价格",
			prop: "price",
			component: { name: "el-input", props: { disabled: true } }
		},
		{
			label: "sku",
			prop: "sku",
			required: true,
			component: { name: "el-input", props: { disabled: true } }
		},
		{
			label: "会员",
			prop: "userId",
			required: true,
			component: { name: "el-select", props: { disabled: true } }
		},

		{
			label: "支付状态",
			prop: "payStatus",
			component: {
				name: "el-select",
				options: [
					{
						label: "未支付",
						value: 0
					},
					{
						label: "已支付",
						value: 1
					}
				],
				props: { disabled: true }
			}
		},
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		}
	],
	async onOpen() {
		const memberList = await service.cms.memberUser.list();
		Upsert.value?.setOptions(
			"userId",

			memberList.map((e) => {
				return {
					label: e.username,
					value: e.id
				};
			})
		);
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "会员", prop: "username" },
		{ label: "商品名称", prop: "name" },
		{ label: "订单编号", prop: "tradeNo" },
		{ label: "价格", prop: "price" },
		{ label: "sku", prop: "sku" },

		{
			label: "支付状态",
			prop: "payStatus",
			dict: [
				{
					label: "已支付",
					value: 1,
					type: "success"
				},
				{
					label: "待支付",
					value: 0,
					type: "danger"
				}
			]
		},
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.cms.financeOrder
	},
	(app) => {
		app.refresh();
	}
);
</script>
