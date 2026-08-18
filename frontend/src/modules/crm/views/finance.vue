<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<el-button type="primary" @click="expenditureOpen('in')">新增收入</el-button>
			<el-button type="danger" @click="expenditureOpen('out')">新增支出</el-button>
			<!-- 删除按钮 -->
			<cl-multi-delete-btn style="margin-right: 10px" />

			<cl-flex1 />

			<!-- <el-button plain @click="toggleExtendBt" style="margin-right: 10px">
				<el-icon><DArrowLeft /></el-icon> {{ isExtend ? "收起" : "展开" }}
			</el-button> -->

			<!-- <el-button test bg @click="search(3)">近三个月</el-button>
			<el-button test bg @click="search(0)">当月</el-button> -->

			<!-- <el-button test bg @click="doClear">清除条件</el-button> -->

			<el-button type="info" text bg :icon="Search" v-show="searchStatus">
				正在搜索中
			</el-button>
			<!-- 按钮 -->
			<cl-adv-btn />
			<!-- 关键字搜索 -->
			<cl-search-key />
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table
				ref="Table"
				:border="false"
				:cellClassName="className"
				:headerCellClassName="className"
				:span-method="arraySpanMethod"
				:summary-method="getSummaries"
				show-summary
				@selectionChange="selectionChange"
			>
				<template #column-incomeAndOut="{ scope }">
					<div style="text-align: left" v-html="doIncomeAndOutFilter(scope.row)"></div>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<!-- <cl-pagination /> -->
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert">
			<template #slot-orderCode="{ scope }">
				<el-select-v2
					v-model="scope.orderCode"
					filterable
					remote
					:remote-method="remoteMethod"
					clearable
					:options="options"
					:loading="loading"
					placeholder="请输入订单号"
				/>
			</template>
		</cl-upsert>
		<!-- 弹窗 -->
		<cl-adv-search ref="AdvSearch" />
	</cl-crud>
</template>

<script lang="ts" name="crm-finance" setup>
import { useCrud, useTable, useUpsert, useAdvSearch } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useDict } from "../../dict";
import { TableColumnCtx, dayjs } from "element-plus";
import { Money, Search } from "@element-plus/icons-vue";
import { onMounted, ref } from "vue";
import { useBase } from "/$/base";

const { user } = useBase();
const uidDisabled = ref(user.info.id == 1 ? false : true);
const { dict } = useDict();
const { service } = useCool();
const type = ref("in");
const isExtend = ref(false);
// 展开按钮
const toggleExtendBt = () => {
	isExtend.value = !isExtend.value;
};
const searchStatus = ref(false); // 搜索状态

onMounted(() => {
	getUser();
});

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "下单者",
			prop: "uid",
			required: true,
			value: user.info.id,
			hidden: () => {
				if (user.info.id == 1) {
					return false;
				} else {
					return true;
				}
			},
			component: { name: "el-select", options: [], props: {} }
		},
		{
			label: "项目订单",
			span: 12,
			prop: "orderCode",
			component: {
				name: "slot-orderCode"
			}
		},
		() => {
			return {
				label: "收款",
				prop: "receiveMoney",
				span: 12,
				hidden: ({ scope }) => {
					if (Upsert.value?.mode == "add") {
						return type.value !== "in";
					} else {
						return scope.type !== "in";
					}
				},
				component: {
					name: "el-input-number",
					props: {
						suffixIcon: Money,
						min: 0
					}
				}
			};
		},
		() => {
			return {
				label: "支出",
				prop: "expenditureMoney",
				span: 12,
				hidden: ({ scope }) => {
					if (Upsert.value?.mode == "add") {
						return type.value !== "out";
					} else {
						return scope.type !== "out";
					}
				},
				component: {
					name: "el-input-number",
					props: {
						suffixIcon: Money,
						min: 0
					}
				}
			};
		},
		{
			label: "种类",
			prop: "species",
			span: 12,
			value: "company",
			component: {
				name: "el-select",
				options: dict.get("species")
			}
		},
		{
			label: "结算方式",
			prop: "paytype",
			span: 12,
			required: true,
			component: {
				name: "el-select",
				options: dict.get("paytype")
			}
		},
		{
			label: "结算日期",
			prop: "payDate",
			span: 12,
			required: true,
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
	async onOpen(data) {
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
		}
		// 项目订单
		if (Upsert.value?.mode == "update") {
			await remoteMethod(data.orderCode);
		}

		if (Upsert.value?.mode == "add") {
			if (type.value == "in") {
				Upsert.value?.setTitle("新增收入");
			}
			if (type.value == "out") {
				Upsert.value?.setTitle("新增支出");
			}
		}

		if (Upsert.value?.mode == "update") {
			type.value = data.type;
			if (data.type == "in") {
				Upsert.value?.setTitle("编辑收入");
			}
			if (data.type == "out") {
				Upsert.value?.setTitle("编辑支出");
			}
		}
	},
	onSubmit(data, { next }) {
		let form: any = {
			projectNum: data.orderCode?.split("-")[0],
			type: type.value,
			incomeAndOut: data.receiveMoney,
			orderId: orderData.value[data.orderCode]?.id,
			orderType: orderData.value[data.orderCode]?.orderType
		};

		if (data.expenditureMoney) {
			form.incomeAndOut = "-" + data.expenditureMoney;
		}

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
		{ label: "下单者", prop: "userName", hidden: uidDisabled },
		{ label: "项目订单", prop: "orderCode" },
		{
			label: "收支",
			prop: "incomeAndOut"
		},
		{
			label: "种类",
			prop: "species",
			dict: {
				text: true,
				separator: "",
				options: dict.get("species")
			}
		},
		{
			label: "结算方式",
			prop: "paytype",
			dict: {
				text: true,
				separator: "",
				options: dict.get("paytype")
			}
		},
		{
			label: "结算日期",
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
	service: service.crm.finance,
	async onRefresh(params, { next, render }) {
		let list;
		if (clear.value) {
			clear.value = false;
			list = await service.crm.finance.list({ size: 10000, month: 1 });
		} else {
			list = await service.crm.finance.list(params);
		}
		render(list);
	}
});

onMounted(() => {
	refresh({ size: 10000, month: 1 });
});

const refresh = (params: any) => {
	Crud.value?.refresh(params);
};

//
const userMapList = ref();
const getUser = async () => {
	const userList = await service.base.sys.user.list();
	const mapList = userList.map((e) => {
		return {
			label: e.nickName || "",
			value: e.id || ""
		};
	});

	userMapList.value = mapList;
	return userMapList;
};

// 高级搜索
const AdvSearch = useAdvSearch({
	items: [
		() => {
			return {
				label: "下单者",
				prop: "uid",
				component: {
					name: "cl-select",
					props: {
						options: [...userMapList.value]
					}
				}
			};
		},
		{
			label: "月份",
			prop: "month",
			component: {
				name: "cl-select",
				props: {
					options: [
						{
							label: "近一个月",
							value: "1"
						},
						{
							label: "近二个月",
							value: "2"
						},
						{
							label: "近三个月",
							value: "3"
						},
						{
							label: "近半年",
							value: "6"
						},
						{
							label: "近一年",
							value: "12"
						}
					]
				}
			}
		},
		{
			label: "种类",
			prop: "species",
			component: {
				name: "cl-select",
				props: {
					options: dict.get("species")
				}
			}
		},
		{
			label: "收支类型",
			prop: "type",
			component: {
				name: "cl-select",
				props: {
					options: dict.get("type")
				}
			}
		},
		{
			label: "结算方式",
			prop: "paytype",
			component: {
				name: "cl-select",
				props: {
					options: dict.get("paytype")
				}
			}
		},
		{
			label: "时间",
			prop: "dateTime",
			component: {
				name: "el-date-picker",
				props: {
					type: "daterange",
					startPlaceholder: "开始日期",
					endPlaceholder: "结束日期",
					value: "YYYY-MM-DD",
					valueFormat: "YYYY-MM-DD",
					disabledDate: (date: any) => {
						return date.getTime() > Date.now();
					}
				}
			}
		}
	],
	onSearch(data, { next, close }) {
		next(data);
		searchStatus.value = false;
		searchStatus.value = Object.values(data).some((value) => {
			if (value || value === 0) return true;
		});
	}
});

// 搜索
// const search = (n?: number) => {
// 	Crud.value?.refresh({ month: n });
// };

// 清除搜索条件
const clear = ref(false);
const doClear = () => {
	clear.value = true;
	Crud.value?.refresh();
};

// 表格竖列改颜色
const className = ({ column }: any) => {
	if (column.property == "expenditureMoney") {
		return "expenditureMoney";
	}
	if (column.property == "receiveMoney") {
		return "receiveMoney";
	}
};

// 新增
const expenditureOpen = (v: any) => {
	type.value = v;
	Upsert.value?.add();
};

// 整理远程数据，提交时用到
const orderData = ref<any>({});

interface ListItem {
	value: string;
	label: string;
}
// 项目订单远程搜索
const options = ref<ListItem[]>([]);
const loading = ref(false);
const remoteMethod = async (keyWord: string) => {
	if (keyWord !== "") {
		loading.value = true;
		// 项目订单
		const orderList = await service.crm.order.list({ keyWord });

		loading.value = false;
		options.value = orderList.map((e) => {
			orderData.value[e.orderCode] = e;
			const r: any = doFilter(e.orderType);
			return {
				label: r?.label + "-" + e.orderCode,
				value: e.orderCode
			};
		});
	}
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

const multipleSelection = ref();
// 选中触发
const selectionChange = (val: any) => {
	multipleSelection.value = val;
};

interface SpanMethodProps {
	row: any;
	column: TableColumnCtx<any>;
	rowIndex: number;
	columnIndex: number;
}
// 合并行和列
const arraySpanMethod = ({ row, column, rowIndex, columnIndex }: SpanMethodProps) => {
	if (rowIndex === Number(Table.value?.data.length)) {
		if (columnIndex === 0) {
			return [1, 2];
		} else if (columnIndex === 1) {
			return [0, 0];
		}
	}
};

// 合计
const getSummaries = (param: any) => {
	const { columns, data } = param;
	return [
		"",
		"合计",
		"",
		`￥ ${data
			.reduce((a: any, b: any) => parseFloat(a + Number(b.incomeAndOut)), 0)
			.toFixed(2)}`
	];
};

// 收支
const doIncomeAndOutFilter = (v: any) => {
	let type = "in";
	if (v?.expenditureMoney) {
		type = "out";
	}
	let htmlIncomeAndOut: any;
	if (v?.type == "in") {
		htmlIncomeAndOut = `<span style="color:#f74848"> 收 : ${v?.incomeAndOut} </span> 元`;
	} else {
		htmlIncomeAndOut = `<span style="color:#4165d7"> 支 : ${v?.incomeAndOut}  </span>  元`;
	}
	return htmlIncomeAndOut;
};
</script>

<style lang="scss">
.expenditureMoney {
	color: #f74848 !important;
}

.receiveMoney {
	color: #3981c1 !important;
}
</style>
