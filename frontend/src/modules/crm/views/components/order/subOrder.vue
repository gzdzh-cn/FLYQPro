<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn v-if="props.orderInfo?.id" />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn />
			<cl-flex1 />

			<el-button type="info" text bg :icon="Search" v-show="searchStatus">
				正在搜索中
			</el-button>
			<cl-adv-btn />
			<!-- 关键字搜索 -->
			<cl-search-key />
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="false">
				<template #column-detail="{ scope }">
					<div style="padding: 20px 100px">
						<h3>订单明细：</h3>
						<br />
						<p v-if="scope.row?.orderType == 'subRenewal'">
							到期时间：
							<cl-date-text v-model="scope.row.endDate" format="YYYY-MM-DD" />
						</p>
						<p>备注: {{ scope.row?.remark }}</p>
					</div>
				</template>

				<template #column-orderCode="{ scope }">
					{{ scope.row.orderCode }}
					<span style="color: crimson">
						{{
							scope.row.orderType == "subExtend"
								? ":" + getStateName(scope.row.state)
								: ""
						}}
					</span>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert">
			<template #slot-newDomain="{ scope }">
				<cl-row>
					<cl-col :span="18">
						<el-input v-model="newDomain" placeholder="不要添加http或https" />
					</cl-col>
					<cl-col>
						<el-button @click="addDomin(scope.domain)">添加</el-button>
					</cl-col>
					<cl-col>
						<el-button @click="resetDomin(scope.domain)">重置</el-button>
					</cl-col>
				</cl-row>
			</template>
		</cl-upsert>

		<cl-adv-search ref="AdvSearch" />
	</cl-crud>
</template>

<script lang="ts" name="sub-order" setup>
import { useCrud, useTable, useUpsert, useAdvSearch } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { reactive, ref, watch, onMounted } from "vue";

import { Money, Search } from "@element-plus/icons-vue";
import { dayjs } from "element-plus";
import { useDict } from "/$/dict";

const { service } = useCool();
const { user } = useBase();
const { dict } = useDict();
const uidDisabled = ref(user.info.id == 1 ? false : true);
const userMapList = ref();
const userList = ref();
const searchStatus = ref(false); // 搜索状态

const props = defineProps({
	orderInfo: Object
});

watch(
	() => props.orderInfo?.id,
	async (n?: any) => {
		refresh({ id: n });
	},
	{
		deep: true
	}
);

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
			label: "订单类型",
			prop: "orderType",
			required: true,
			span: 12,
			component: {
				name: "el-select",
				options: dict.get("subOrderType"),
				props: {
					onChange: (v) => {
						orderType.value = v;
						if (v == "subRenewal") {
							Upsert.value?.setForm(
								"endDate",
								dayjs(props.orderInfo?.endDate).add(1, "year").format("YYYY-MM-DD")
							);
						} else {
							Upsert.value?.setForm(
								"endDate",
								dayjs().add(1, "year").format("YYYY-MM-DD")
							);
						}
					}
				}
			}
		},
		{
			label: "域名",
			prop: "domain",
			hidden: ({ scope }) => scope.orderType != "subDomain",
			component: {
				name: "el-input",
				disabled: () => {
					return Upsert.value?.mode !== "update";
				}
			}
		},

		{
			label: "域名归属我司",
			prop: "isSelf",
			span: 12,
			hidden: ({ scope }) => scope.orderType != "subDomain",
			value: 1,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } },
			group: "other"
		},
		{
			label: "是否备案",
			prop: "isIcp",
			span: 12,
			hidden: ({ scope }) => scope.orderType != "subDomain",
			value: 0,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } },
			group: "other"
		},
		{
			label: "注册时间",
			prop: "startDate",
			span: 12,
			hidden: ({ scope }) => {
				return scope.orderType !== "subDomain";
			},
			value: dayjs().format("YYYY-MM-DD"),
			component: {
				name: "el-date-picker",
				props: { type: "date", valueFormat: "YYYY-MM-DD" }
			},
			group: "base"
		},
		{
			label: "到期时间",
			prop: "endDate",
			required: true,
			span: 12,
			hidden: ({ scope }) => scope.orderType == "subExtend" || scope.orderType == "subRecord",
			component: {
				name: "el-date-picker",
				props: {
					type: "date",
					valueFormat: "YYYY-MM-DD"
				}
			}
		},

		{
			label: "价格",
			prop: "price",
			required: true,
			span: 12,
			component: {
				name: "el-input",
				props: {
					suffixIcon: Money
				}
			}
		},
		{
			label: "返佣",
			prop: "rebate",
			span: 12,
			value: 0,
			component: {
				name: "el-input",
				props: {
					suffixIcon: Money
				}
			}
		},
		{
			label: "空间信息",
			prop: "vpsInfo",
			hidden: ({ scope }) => scope.orderType !== "subVps",
			component: {
				name: "el-input",
				props: {
					type: "textarea",
					rows: 4
				}
			}
		},

		// {
		// 	label: "域名商",
		// 	prop: "domainCompany",
		// 	span: 12,
		// 	hidden: ({ scope }) => {
		// 		return scope.orderType !== "subDomain";
		// 	},
		// 	component: { name: "el-select", options: dict.get("domainProvider") }
		// },
		// { label: "域名所有者", prop: "domainOwer", span: 12, component: { name: "el-input" } },
		{
			label: "实名资料",
			prop: "realNameDataFiles",
			hidden: ({ scope }) => {
				return scope.orderType !== "subDomain";
			},
			component: {
				name: "cl-upload",
				props: {
					type: "file",
					multiple: true,
					showFileList: true,
					draggable: true,
					rename: false
				}
			}
		},
		{
			label: "其他资料",
			prop: "requirementFiles",
			hidden: ({ scope }) => {
				return scope.orderType !== "subRecord" && scope.orderType !== "subExtend";
			},
			component: {
				name: "cl-upload",
				props: {
					type: "file",
					multiple: true,
					showFileList: true,
					draggable: true,
					rename: false
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
		},
		() => {
			return {
				label: "项目进度",
				prop: "state",
				span: 12,
				hidden: ({ scope }) => {
					return Upsert.value?.mode == "add" || scope.orderType !== "subExtend";
				},
				component: {
					name: "el-select",
					options: stateList
				}
			};
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
			if (Upsert.value?.mode == "add") {
				Upsert.value?.setForm("uid", user.info.id);
			}
		}

		if (Upsert.value?.mode == "add" && props.orderInfo?.projectNum) {
			Upsert.value?.setTitle(`${props.orderInfo?.projectNum}-新增子订单`);

			if (orderType.value) {
				Upsert.value?.setForm("orderType", orderType.value);
			}
		}
		if (Upsert.value?.mode == "update" && props.orderInfo?.projectNum) {
			Upsert.value?.setTitle(`${props.orderInfo?.projectNum}-编辑子订单`);
		}
		if (orderType.value == "subRenewal") {
			Upsert.value?.setForm(
				"endDate",
				dayjs(props.orderInfo?.endDate).add(1, "year").format("YYYY-MM-DD")
			);
		}
		if (orderType.value == "subDomain" || orderType.value == "subVps") {
			Upsert.value?.setForm("endDate", dayjs().add(1, "year").format("YYYY-MM-DD"));
		}
	},
	onSubmit(data, { next }) {
		let form: any = {
			parentId: props.orderInfo?.id,
			customerId: props.orderInfo?.customerId,
			domain: filteredStr.value ? filteredStr.value : data.domain
		};

		if (user.info?.id != 1) {
			form.uid = user.info.id;
		}

		if (data?.domain) {
			data.domain = data?.domain.trim();
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
		() => {
			return {
				label: "#",
				type: "expand",
				prop: "detail"
			};
		},
		{ label: "会员", prop: "userName", hidden: uidDisabled },
		{
			label: "订单类型",
			prop: "orderType",
			dict: {
				text: true,
				separator: "",
				options: dict.get("subOrderType")
			}
		},
		{ label: "项目订单", prop: "orderCode", width: 300 },
		{ label: "价格", prop: "price" },
		{
			label: "到期时间",
			prop: "endDate",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{
			label: "操作时间",
			prop: "createTime",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},

		{ type: "op", buttons: ["edit", "delete"], width: 160 }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.crm.order,
	async onRefresh(params, { render }) {
		const { list, pagination } = await service.crm.order.page(params);
		// 渲染数据
		render(list, pagination);
	}
});
const stateList = reactive([
	{
		label: "待处理",
		type: "danger",
		value: 0
	},
	{
		label: "处理中",
		type: "warning",
		value: 1
	},
	{
		label: "已完成",
		type: "success",
		value: 2
	}
]);

// 当前订单类型
const orderType = ref();
const refresh = async (params: any) => {
	params.id = props.orderInfo?.id;
	orderType.value = params.orderType;
	params.sub = true;
	Crud.value?.refresh(params);
};

const cloneDomain = ref();
// 新增域名
const newDomain = ref();
const filteredStr = ref();
const addDomin = (v: any) => {
	let str = "";
	str = newDomain.value;
	if (v != "") {
		str = newDomain.value + "\n" + v;
	}

	filteredStr.value = str
		.trim()
		.replace(/^https?:\/\//, "")
		.replace(/\/$/, "");

	Upsert.value?.setForm("domain", filteredStr.value);
	newDomain.value = "";
	filteredStr.value = "";
};

// 重置
const resetDomin = (v: any) => {
	Upsert.value?.setForm("domain", cloneDomain.value);
	filteredStr.value = cloneDomain.value;
};

// 显示项目状态名称
const getStateName = (v: any) => {
	return stateList.find((item: any) => item.value === v)?.label;
};

// 高级搜索
const AdvSearch = useAdvSearch({
	items: [
		() => {
			return {
				label: "下单者",
				prop: "uid",
				hidden: () => {
					return user.info?.id != 1;
				},
				component: {
					name: "cl-select",
					props: {
						clearable: true,
						options: [...userMapList.value]
					}
				}
			};
		},
		() => {
			return {
				label: "排序",
				prop: "orderBy",
				component: {
					name: "cl-select",
					props: {
						clearable: true,
						options: [
							{
								label: "最近更新",
								value: "updateTimeDesc"
							}
						]
					}
				}
			};
		},
		{
			label: "项目进度",
			prop: "state",
			component: {
				name: "el-select",
				props: {
					clearable: true
				},
				options: stateList
			}
		},
		{
			label: "到期区间",
			prop: "month",
			component: {
				name: "cl-select",
				props: {
					options: [
						{
							label: "当月",
							value: "0"
						},
						{
							label: "未来一个月",
							value: "1"
						},
						{
							label: "未来二个月",
							value: "2"
						},
						{
							label: "未来三个月",
							value: "3"
						},
						{
							label: "未来半年",
							value: "6"
						},
						{
							label: "未来一年",
							value: "12"
						}
					]
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
					valueFormat: "YYYY-MM-DD"
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

// 后台用户列表：客服，技术员，对接人字段使用
const getUser = (userList: any) => {
	return userList.map((e: { nickName: any; id: any }) => {
		return {
			label: e.nickName || "",
			value: e.id || ""
		};
	});
};

onMounted(async () => {
	// 会员
	userList.value = await service.base.sys.user.list();

	userMapList.value = getUser(userList.value);
});

defineExpose({
	refresh
});
</script>
