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

<script lang="ts" name="order-finance" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { Money } from "@element-plus/icons-vue";
import { dayjs } from "element-plus";
import { useDict } from "../../../../dict";
import { useBase } from "/$/base";
import { filterHtml, subString } from "/@/dzh";
import { onMounted, ref, watch } from "vue";
const props = defineProps({
	orderInfo: Object
});

const { dict } = useDict();
const { user } = useBase();
const uidDisabled = ref(user.info?.id == 1 ? false : true);
const orderData = ref<any>({});
watch(
	() => props.orderInfo,
	(n?: any) => {
		refresh({ projectNum: n.projectNum });
	},
	{
		deep: true
	}
);

onMounted(() => {
	refresh();
});

const { service } = useCool();
// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "会员",
			prop: "uid",
			span: 12,
			required: true,
			hidden: () => {
				if (user.info.id == 1) {
					return false;
				} else {
					return true;
				}
			},
			component: {
				name: "el-select",
				props: {}
			}
		},
		{
			label: "项目订单",
			// span: 12,
			prop: "orderCode",
			component: {
				name: "el-select",
				options: []
			}
		},
		{
			label: "收款",
			prop: "receiveMoney",
			span: 12,
			component: {
				name: "el-input",
				props: {
					suffixIcon: Money
				}
			}
		},
		{
			label: "结算方式",
			prop: "paytype",
			span: 12,
			component: {
				name: "el-select",
				options: dict.get("paytype")
			}
		},
		{
			label: "结算日期",
			prop: "payDate",
			span: 12,
			value: dayjs().format("YYYY-MM-DD"),
			component: {
				name: "el-date-picker",
				props: {
					type: "date",
					valueFormat: "YYYY-MM-DD"
				}
			}
		},
		{
			label: "备注",
			prop: "remark",
			component: {
				name: "el-input",
				props: {
					type: "textarea",
					rows: 4
				}
			}
		}
	],
	async onOpen() {
		if (user.info.id == 1) {
			// 会员
			const userList = await service.base.sys.user.list();
			Upsert.value?.setOptions(
				"uid",
				userList.map((e) => {
					return {
						label: e.nickName || "",
						value: e.id || ""
					};
				})
			);
			if (Upsert.value?.mode == "add") {
				Upsert.value?.setForm("uid", user.info.id);
			}
		}

		// 项目订单
		const orderList = await service.crm.order.list({ projectNum: props.orderInfo?.projectNum });
		Upsert.value?.setOptions(
			"orderCode",
			orderList.map((e: any) => {
				orderData.value[e.orderCode] = e;
				const r: any = doFilter(e.orderType);
				return {
					label: r?.label + "-" + e.orderCode + "-" + subString(filterHtml(e.remark), 10),
					value: e.orderCode
				};
			})
		);

		if (Upsert.value?.mode == "add") {
			Upsert.value?.setTitle(`${props.orderInfo?.projectNum}-新增财务`);
		}
		if (Upsert.value?.mode == "update") {
			Upsert.value?.setTitle(`${props.orderInfo?.projectNum}-编辑财务`);
		}
	},
	onSubmit(data, { next }) {
		let form: any = {
			orderId: orderData.value[data.orderCode]?.id,
			orderType: orderData.value[data.orderCode]?.orderType,
			projectNum: props.orderInfo?.projectNum,
			incomeAndOut: data.receiveMoney
		};
		if (user.info?.id != 1) {
			form.uid = user.info.id;
		}
		next({
			...data,
			...form
		});
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "会员", prop: "userName", hidden: uidDisabled },
		{ label: "项目订单", prop: "orderCode" },
		{ label: "收款", prop: "receiveMoney" },
		{
			label: "收款方式",
			prop: "paytype",
			dict: {
				text: true,
				separator: "",
				options: dict.get("paytype")
			}
		},
		{
			label: "收款日期",
			prop: "payDate",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{
			label: "备注",
			prop: "remark",
			showOverflowTooltip: true,
			minWidth: 150
		},
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.crm.finance
});

const refresh = (projectNum?: any) => {
	const params = { projectNum: props.orderInfo?.projectNum | projectNum };
	Crud.value?.refresh(params);
};

// 取dict值
const doFilter = (v: any) => {
	const item1 = dict.get("orderType").value.filter((e) => e.value == v);
	if (item1.length > 0) {
		return item1[0];
	}
	const item2 = dict.get("subOrderType").value.filter((e) => e.value == v);
	if (item2.length > 0) {
		return item2[0];
	}
	return {};
};
</script>
