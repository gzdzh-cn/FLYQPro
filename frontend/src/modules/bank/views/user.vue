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

<script lang="ts" name="bank-user" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { computed } from "vue";

const { user } = useBase();

const hidden = computed(() => {
	return !user.info?.roleIds.split(",").includes("1");
});

const { service } = useCool();

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
			label: "绿色能量",
			prop: "greenPower",
			span: 12,
			value: 0,
			required: true,
			component: { name: "el-input" }
		},

		{
			label: "银行卡数量",
			prop: "bankCardNum",
			span: 12,
			value: 0,
			component: { name: "el-input", props: { disabled: true } },
			required: true
		},
		{
			label: "优惠卷数量",
			prop: "discountNum",
			span: 12,
			value: 0,
			component: { name: "el-input", props: { min: 0 } },
			required: true
		},
		{
			label: "积分数量",
			prop: "scoreNum",
			span: 12,
			value: 0,
			component: { name: "el-input", props: { min: 0 } },
			required: true
		},
		{
			label: "立减金数量",
			prop: "reduceNum",
			span: 12,
			value: 0,
			component: { name: "el-input", props: { min: 0 } },
			required: true
		},
		{
			label: "工业银行豆",
			prop: "industrialBankBean",
			span: 12,
			value: 0,
			component: { name: "el-input", props: { min: 0 } }
		},
		{
			label: "电子收藏数量",
			prop: "digitalCollectionNum",
			span: 12,
			value: 0,
			component: { name: "el-input", props: { min: 0 } }
		},
		{
			label: "电子注册地址",
			prop: "registerAddress",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "注册方式",
			prop: "registerType",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "上次登录",
			prop: "lastLogin",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "定位信息",
			prop: "position",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "国籍",
			prop: "nationality",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "职业",
			prop: "occupation",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "证件号码",
			prop: "certNo",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "证件类型",
			prop: "certType",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "证件有效期",
			prop: "certValidity",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "邀请码",
			prop: "inviteCode",
			span: 12,
			component: { name: "el-input", props: { disabled: true } }
		}
	],
	async onOpen() {
		const memberList = await service.member.manage.list();
		Upsert.value?.setOptions(
			"memberId",
			memberList.map((e) => {
				return { label: e.memberName, value: e.id };
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
		{ label: "绿色能量", prop: "greenPower" },
		{ label: "银行卡数量", prop: "bankCardNum" },
		{ label: "优惠卷数量", prop: "discountNum" },
		{ label: "积分数量", prop: "scoreNum" },
		{ label: "立减金数量", prop: "reduceNum" },
		{ label: "邀请码", prop: "inviteCode" },

		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.bank.user
	},
	(app) => {
		app.refresh();
	}
);
</script>
