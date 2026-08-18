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
		<cl-upsert ref="Upsert">
			<template #slot-memberId="{ scope }">
				<!-- 学校 -->
				<el-select v-model="scope.memberId" @change="memberIdChange">
					<el-option
						v-for="item in memberlList"
						:key="item.id"
						:label="item.memberName"
						:value="item.id"
					/>
				</el-select>
			</template>
		</cl-upsert>
	</cl-crud>
</template>

<script lang="ts" name="bank-bill" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useDict } from "/$/dict";
import { computed, onMounted, ref } from "vue";
import { addTypeToDictOptions, STATUS_TYPE_MAPPING } from "/@/dzh";
import { useBase } from "/$/base";

const { user } = useBase();
const { service } = useCool();
const { dict } = useDict();

const hidden = computed(() => {
	return !user.info?.roleIds.split(",").includes("1");
});

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "标题", prop: "title", component: { name: "el-input" } },
		{
			label: "会员",
			prop: "memberId",
			span: 12,
			required: true,
			component: {
				name: "slot-memberId"
			}
		},
		{
			label: "账单类型",
			prop: "billType",
			span: 12,
			required: true,
			component: { name: "el-select", options: dict.get("billType") as any }
		},
		{
			label: "收支类型",
			prop: "incomeType",
			span: 12,
			required: true,
			component: { name: "el-select", options: dict.get("incomeType") as any }
		},
		{
			label: "银行卡",
			prop: "cardId",
			span: 12,
			required: true,
			component: { name: "el-select", options: [] }
		},
		{
			label: "金额",
			prop: "money",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "剩余金额",
			prop: "remainingMoney",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "交易国家",
			prop: "country",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "记账币种",
			prop: "currency",
			span: 12,
			component: { name: "el-input" }
		},
		{
			label: "对方账户",
			prop: "otherAccount",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "对方户名",
			prop: "otherName",
			span: 12,
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "对方账户行别",
			prop: "otherBank",
			span: 12,
			required: true,
			component: { name: "el-input" }
		}
	],
	async onOpen() {
		getMemberIdlList();
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "租户", prop: "username", hidden: hidden.value },
		{ label: "会员", prop: "memberName" },
		{ label: "标题", prop: "title" },
		{ label: "账单类型", prop: "billType", dict: { options: dict.get("billType") as any } },
		() => {
			return {
				label: "收支类型",
				prop: "incomeType",
				dict: {
					options: incomeTypeOptions as any
				}
			};
		},
		{ label: "银行卡", prop: "bankNum" },
		{ label: "金额", prop: "money" },
		{ label: "剩余金额", prop: "remainingMoney" },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.bank.bill,
		async onRefresh(params, { render }) {
			const { list, pagination } = await service.bank.bill.page(params);
			render(list, pagination);
		}
	},
	(app) => {
		app.refresh();
	}
);

// 收支类型
const incomeTypeOptions = computed(() => {
	const options = dict.get("incomeType").value || [];
	const res = addTypeToDictOptions(options, STATUS_TYPE_MAPPING.INCOME_TYPE);
	return res;
});

const memberlList = ref<any>([]);
const getMemberIdlList = async () => {
	const memberList = await service.member.manage.list();
	memberlList.value = memberList;
	if (memberList && memberList.length > 0) {
		memberIdChange(memberList[0].id);
	}
};
const memberIdChange = async (v: any) => {
	Upsert.value?.setOptions("cardId", []);
	getCardList(v);
};

// 获取银行卡列表
const getCardList = async (id: string) => {
	const cardList = await service.bank.card.list({ memberId: id });
	if (cardList && cardList.length > 0) {
		Upsert.value?.setOptions(
			"cardId",
			cardList.map((e) => {
				return { label: e.bankNum, value: e.id };
			})
		);
	}
};

onMounted(() => {});

// 账单类型
</script>
