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

<script lang="ts" name="bank-payee" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useDict } from "/$/dict";
import { useBase } from "/$/base";
import { computed } from "vue";

const { user } = useBase();
const { service } = useCool();
const { dict } = useDict();

const hidden = computed(() => {
	return !user.info?.roleIds.split(",").includes("1");
});

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		// {
		// 	label: "租户",
		// 	prop: "userId",
		// 	span: 12,
		// 	hidden: hidden.value,
		// 	required: true,
		// 	component: { name: "el-select" }
		// },
		{
			label: "会员",
			prop: "memberId",
			span: 12,
			required: true,
			component: { name: "el-select" }
		},
		{
			label: "卡种类",
			prop: "cardKindId",
			span: 12,
			required: true,
			component: { name: "el-select", options: dict.get("cardKind") as any }
		},
		{
			label: "卡户名",
			prop: "bankUser",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "银行名称",
			prop: "bankName",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "银行卡号",
			prop: "bankNum",
			span: 12,
			component: { name: "el-input" },
			required: true
		},
		{
			label: "银行卡icon",
			prop: "cardIcon",
			component: { name: "cl-upload" },
			required: true
		}
	],
	async onOpen() {
		const memberList = await service.member.manage.list();
		Upsert.value?.setOptions(
			"memberId",
			memberList.map((e) => {
				return { label: e.memberName || "", value: e.id || "" };
			})
		);

		// const userList = await service.base.sys.user.list();
		// Upsert.value?.setOptions(
		// 	"userId",
		// 	userList.map((e: any) => {
		// 		return { label: e.username || "", value: e.id || "" };
		// 	})
		// );
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "租户", prop: "username", hidden: hidden.value },
		{ label: "会员", prop: "memberName" },
		{ label: "卡种类", prop: "cardKindId", dict: { options: dict.get("cardKind") as any } },
		{ label: "卡户名", prop: "bankUser" },
		{ label: "银行名称", prop: "bankName" },
		{ label: "银行卡号", prop: "bankNum" },
		{
			label: "银行卡icon",
			prop: "cardIcon",
			component: { name: "cl-image", props: { size: 60 } }
		},
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.bank.payee
	},
	(app) => {
		app.refresh();
	}
);
</script>
