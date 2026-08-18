<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />

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
		<cl-upsert ref="Upsert" />
	</cl-crud>
</template>

<script lang="ts" name="customer_pro-wxMessage" setup>
import { h } from "vue";
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import dayjs from "dayjs";
import { done } from "nprogress";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "公众号", prop: "mp_id", span: 12, component: { name: "el-input" } },
		{ label: "标题", prop: "title", span: 12, component: { name: "el-input" } },
		{ label: "用户", prop: "userId", span: 12, component: { name: "el-select" } },
		{ label: "接收用户", prop: "touser", span: 12, component: { name: "el-input" } },
		{
			label: "错误码",
			prop: "errcode",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "错误提示",
			prop: "errmsg",
			component: {
				name: "el-input",
				props: {
					type: "textarea",
					rows: 4
				}
			}
		},
		{
			label: "通知内容",
			prop: "remark",
			component: {
				name: "el-input",
				props: {
					type: "textarea",
					rows: 10
				}
			}
		},
		// {
		// 	label: "内容",
		// 	prop: "content",
		// 	component: {
		// 		name: "el-input",
		// 		props: {
		// 			type: "textarea",
		// 			rows: 10
		// 		}
		// 	}
		// }
	],
	async onOpen() {
		const userList = await service.base.sys.user.list();
		Upsert.value?.setOptions(
			"userId",
			userList.map((e) => {
				return {
					label: e.name,
					value: e.id
				};
			})
		);
	},
	onInfo(data, { done }) {
		service.customer_pro.wxMessage.update({
			id: data.id,
			receipt_time: dayjs().format("YYYY-MM-DD HH:mm:ss")
		});
		done(data);
	},
	onClosed() {
		Crud.value?.refresh();
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
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
		{ label: "标题", prop: "title" },
		{ label: "用户", prop: "userName" },
		// { label: "接收用户", prop: "touser" },
		// { label: "内容", prop: "content", width: 400, showOverflowTooltip: true },
		// { label: "错误码", prop: "errcode" },
		// { label: "错误提示", prop: "errmsg", width: 200, showOverflowTooltip: true },
		{ label: "阅读时间", prop: "receipt_time" },
		{
			label: "状态",
			prop: "status",
			formatter(row) {
				if (row.status === -1) {
					return h("span", { style: { color: "#F56C6C" } }, "失败");
				}
				return row.status === 1 ? "成功" : row.status;
			}
		},
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["info"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.wxMessage
	},
	(app) => {
		app.refresh();
	}
);
</script>
