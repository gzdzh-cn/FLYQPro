<template>
	<cl-crud ref="Crud">
		<cl-row style="">
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn style="margin-right: 10px" />

			<el-button type="info" @click="openSubOrder">查看子订单</el-button>

			<cl-flex1 />

			<el-button type="info" text bg :icon="Search" v-show="searchStatus">
				正在搜索中
			</el-button>
			<!-- 高级按钮 -->
			<cl-adv-btn />
			<!-- 关键字搜索 -->
			<cl-search-key />
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="false">
				<template #column-domain="{ scope }">
					<template v-if="scope.row?.domain !== ''">
						<el-tag v-for="(item, index) in scope.row?.domain?.split('\n')" :key="index" size="small"
							effect="plain">
							{{ item }}
						</el-tag>
					</template>
					<template v-if="scope.row?.subDomain !== ''">
						<el-tag v-for="(item, index) in scope.row?.subDomain?.split('\n')" :key="index" size="small"
							effect="plain">
							{{ item }}
						</el-tag>
					</template>
				</template>

				<template #column-state="{ scope }">
					<template v-if="scope.row?.state == 2">
						<el-button text bg :type="doFilterState(scope.row?.state)?.type">
							{{ doFilterState(scope.row?.state)?.label }}
						</el-button>
					</template>
					<el-popconfirm title="项目进度更新?" @confirm="clickState(scope.row)" v-if="scope.row?.state != 2">
						<template #reference>
							<el-button text bg :type="doFilterState(scope.row?.state)?.type">
								{{ doFilterState(scope.row?.state)?.label }}
							</el-button>
						</template>
					</el-popconfirm>
				</template>

				<template #column-endDate="{ scope }">
					<template v-if="scope.row.endDate">
						<span :class="{ textRed: !compareDate(scope.row.endDate, currentDate) }">
							{{ scope.row.endDate }}
						</span>
					</template>
				</template>

				<template #slot-subOp="{ scope }">
					<el-button text bg type="primary" @click="edit(scope.row)">编辑</el-button>
					<el-button text bg type="info" @click="doHistory(scope.row)">跟踪</el-button>
					<el-popover placement="right" :width="80" trigger="click" popper-class="dzh_popover">
						<template #reference>
							<el-button text bg type="success">更多</el-button>
						</template>
						<div class="more">
							<div class="more_btn" v-permission="service.crm.order.permission.add">
								<el-button text bg type="warning" @click="subList(scope.row)">追加</el-button>
							</div>
							<div v-permission="service.crm.finance.permission.page">
								<el-button text bg type="danger" @click="financeList(scope.row)">财务</el-button>
							</div>
						</div>
					</el-popover>
				</template>

				<template #column-detail="{ scope }">
					<div style="padding: 0 30px">
						<p>客户: {{ scope.row?.cName }}</p>
						<p>订单类型: {{ orderTypeObj[scope.row?.orderType] }}</p>
						<p>价格: {{ scope.row?.price }}</p>
						<p>渠道: {{ scope.row?.agentName }}</p>
						<p>客服: {{ userData[scope.row?.waiterId] }}</p>
						<p>技术员: {{ userData[scope.row?.technicianId] }}</p>
						<p>项目对接人: {{ userData[scope.row?.siteManagerId] }}</p>
					</div>
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
				<el-row :gutter="10">
					<el-col :span="18">
						<el-input v-model="newDomain" placeholder="不要添加http或https" />
					</el-col>
					<el-col>
						<el-button @click="addDomin(scope.domain)">添加</el-button>
					</el-col>
					<el-col>
						<el-button @click="resetDomin(scope.domain)">重置</el-button>
					</el-col>
				</el-row>
			</template>
			<template #slot-customerId="{ scope }">
				<el-select-v2 v-model="scope.customerId" filterable remote :remote-method="remoteMethodCustomer"
					clearable :options="optionsCustomerId" :loading="loading" placeholder="请输入名称模糊搜索" />
			</template>
			<template #slot-agentId="{ scope }">
				<el-select-v2 v-model="scope.agentId" filterable remote :remote-method="remoteMethodAgent" clearable
					:options="optionsAgentId" :loading="loading" placeholder="请输入名称模糊搜索" />
			</template>
		</cl-upsert>

		<!-- 高级搜索 -->
		<cl-adv-search ref="AdvSearch" />

		<!-- 子订单 -->
		<el-drawer v-model="drawerSubOrder" title="下级订单" direction="rtl" size="100%">
			<order-sub :orderInfo="orderInfo" :key="callKey"></order-sub>
		</el-drawer>

		<!-- 财务订单 -->
		<el-drawer v-model="drawerFinance" title="财务列表" direction="rtl" size="80%">
			<order-finance :orderInfo="orderInfo" :key="callKey" />
		</el-drawer>

		<!-- 历史跟踪 -->
		<el-drawer v-model="drawerDoHistory" title="跟踪处理" direction="rtl" size="80%">
			<order-dohistory :orderInfo="orderInfo" :key="callKey" />
		</el-drawer>
	</cl-crud>
</template>

<script lang="ts" name="crm-order" setup>
import { useCrud, useTable, useUpsert, useAdvSearch } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { markRaw, reactive, ref } from "vue";
import { ElDrawer, dayjs } from "element-plus";
import OrderSub from "./components/order/sub.vue";
import OrderFinance from "./components/order/finance.vue";
import OrderDohistory from "./components/order/doHistory.vue";
import { useDict } from "../../dict";
import { Money, Search } from "@element-plus/icons-vue";
import { ElNotification } from "element-plus";
import { compareDate } from "/@/dzh";

const { dict } = useDict();
// const { crm } = useCrm();
const { service } = useCool();
const { user } = useBase();
const uidDisabled = ref(user.info?.id == 1 ? false : true);
const newDomain = ref();
const cloneDomain = ref();
const userMapList = ref();
// 新增域名
const filteredStr = ref();
const userList = ref();
// 强制子组件视图重载
const callKey = ref(0);
const orderInfo = ref();
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
const userData = ref<any>({});

const orderTypeObj: any = ref({});
const nowTime = new Date();
const currentDate = ref(dayjs(nowTime).format("YYYY-MM-DD"));
const isExtend = ref(false);
const searchStatus = ref(false); // 搜索状态
// 展开按钮
const toggleExtendBt = () => {
	isExtend.value = !isExtend.value;
};


// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			type: "tabs",
			props: {
				labels: [
					{
						label: "基础信息",
						value: "base"
					},
					{
						label: "其他信息",
						value: "other"
					},
					{
						label: "富文本信息",
						value: "text"
					},
					{
						label: "人员信息",
						value: "personnel"
					},
					{
						label: "财务信息",
						value: "finance"
					}
				]
			}
		},
		{
			label: "下单者",
			prop: "uid",
			span: 12,
			required: true,
			value: user.info.id,
			hidden: () => {
				if (user.info.id == 1) {
					return false;
				} else {
					return true;
				}
			},
			component: {
				name: "el-select",
				props: {
					clearable: true
				}
			},
			group: "base"
		},
		() => {
			return {
				label: "项目订单",
				span: 12,
				prop: "orderCode",
				hidden: () => {
					return Upsert.value?.mode == "add";
				},
				component: {
					name: "el-input",
					props: {
						disabled: Upsert.value?.mode != "add" ? true : false
					}
				},
				group: "base"
			}
		},
		{
			label: "域名归属我司",
			prop: "isSelf",
			span: 6,
			value: 0,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } },
			group: "base"
		},
		{
			label: "是否备案",
			prop: "isIcp",
			span: 6,
			value: 0,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } },
			group: "base"
		},
		() => {
			return {
				label: "域名",
				prop: "domain",
				span: 24,
				component: {
					name: "el-input",
					props: {
						type: "textarea",
						rows: 4,
						disabled: Upsert.value?.mode != "add" ? true : false
					}
				},
				group: "base"
			};
		},
		{
			label: "新增域名",
			prop: "newDomain",
			hidden: () => {
				return Upsert.value?.mode == "add";
			},
			component: {
				name: "slot-newDomain",
				disabled: () => {
					return Upsert.value?.mode !== "update";
				}
			},
			group: "base"
		},
		{
			label: "客户",
			prop: "customerId",
			component: { name: "slot-customerId" },
			group: "base"
		},
		{
			label: "项目名称",
			prop: "siteName",
			required: true,
			component: { name: "el-input" },
			group: "base"
		},
		() => {
			return {
				label: "项目进度",
				prop: "state",
				span: 12,
				hidden: () => {
					return Upsert.value?.mode == "add";
				},
				component: {
					name: "el-select",
					options: stateList
				},
				group: "base"
			};
		},
		{
			label: "订单类型",
			prop: "orderType",
			span: 12,
			required: true,
			component: {
				name: "el-select",
				options: dict.get("orderType")
			},
			group: "base"
		},
		{
			label: "主订单到期",
			prop: "mainEndDate",
			span: 12,
			hidden: () => {
				return Upsert.value?.mode == "add";
			},
			component: {
				name: "el-input",
				props: {
					disabled: true
				}
			},
			group: "base"
		},
		{
			label: "续费时间",
			prop: "endDate",
			span: 12,
			value: dayjs().add(1, "year").format("YYYY-MM-DD"),
			component: {
				name: "el-date-picker",
				props: { type: "date", valueFormat: "YYYY-MM-DD" }
			},
			group: "base"
		},
		{
			label: "需求上传",
			prop: "requirementFiles",
			component: {
				name: "cl-upload",
				props: {
					type: "file",
					multiple: true,
					showFileList: true,
					draggable: true,
					// rename: false
				}
			},
			group: "base"
		},

		// other
		{
			label: "模板地址",
			prop: "siteUrl",
			component: { name: "el-input" },
			group: "other"
		},
		{
			label: "交单日期",
			prop: "completeDate",
			span: 12,
			value: dayjs().add(7, "day").format("YYYY-MM-DD"),
			component: {
				name: "el-date-picker",
				props: { type: "date", valueFormat: "YYYY-MM-DD" }
			},
			group: "other"
		},
		{
			label: "联系人",
			prop: "contactor",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{
			label: "手机",
			prop: "phone",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{
			label: "公司座机",
			prop: "tel",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{
			label: "传真",
			prop: "zipCode",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{
			label: "Email",
			prop: "email",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{ label: "QQ", prop: "qq", span: 12, component: { name: "el-input" }, group: "other" },
		{ label: "地址", prop: "addess", component: { name: "el-input" }, group: "other" },

		// other

		{
			label: "备注",
			prop: "remark",
			component: {
				name: "cl-editor-quill",
				props: {
					height: 200
				}
			},
			group: "text"
		},
		{
			label: "项目信息",
			prop: "projectInfo",
			component: {
				name: "cl-editor-quill",
				props: {
					height: 200
				}
			},
			group: "text"
		},
		// personnel

		{
			label: "渠道",
			prop: "agentId",
			component: { name: "slot-agentId", options: [], props: { clearable: true } },
			group: "personnel"
		},
		{
			label: "客服",
			prop: "waiterId",
			span: 12,
			component: { name: "el-select", options: [], props: { clearable: true } },
			group: "personnel"
		},
		{
			label: "技术员",
			prop: "technicianId",
			span: 12,
			component: { name: "el-select", options: [], props: { clearable: true } },
			group: "personnel"
		},
		{
			label: "项目对接人",
			prop: "siteManagerId",
			span: 12,
			required: true,
			component: { name: "el-select", options: [], props: { clearable: true } },
			group: "personnel"
		},

		// personnel
		// finance
		{
			label: "发票上传",
			prop: "invoiceImgs",
			component: {
				name: "cl-upload",
				props: {
					type: "file",
					multiple: true,
					showFileList: true,
					draggable: true,
					// rename: false
				}
			},
			group: "finance"
		},
		{
			label: "合同上传",
			prop: "contractFiles",
			component: {
				name: "cl-upload",
				props: {
					type: "file",
					multiple: true,
					showFileList: true,
					draggable: true,
					// rename: false
				}
			},
			group: "finance"
		},
		() => {
			return {
				label: "项目总价",
				prop: "price",
				span: 12,
				required: true,
				component: {
					name: "el-input",
					props: {
						suffixIcon: markRaw(Money)
					}
				},
				group: "finance"
			}
		},
		{
			label: "续费价格",
			prop: "renewal",
			span: 12,
			required: true,
			component: {
				name: "el-input",
				props: {
					suffixIcon: markRaw(Money)
				}
			},
			group: "finance"
		},
		{
			label: "返佣",
			prop: "rebate",
			span: 12,
			required: true,
			value: 0,
			component: {
				name: "el-input",
				props: {
					suffixIcon: markRaw(Money)
				}
			},
			group: "finance"
		}
	],
	async onOpen(data) {
		newDomain.value = "";
		filteredStr.value = "";
		cloneDomain.value = "";

		if (Upsert.value?.mode != "add") {
			cloneDomain.value = data.domain;
		}

		if (user.info.id == 1) {
			Upsert.value?.setOptions(
				"uid",
				userList.value.map((e: { nickName: any; id: any }) => {
					return {
						label: e.nickName || "",
						value: e.id || ""
					};
				})
			);
		}

		// 远程搜索字段初始化
		if (Upsert.value?.mode == "update") {
			await remoteMethodCustomer(data.customerId);
			await remoteMethodAgent(data.agentId);
		}

		// 客服
		Upsert.value?.setOptions(
			"waiterId",
			userList.value.map((e: { nickName: any; id: any }) => {
				return {
					label: e.nickName || "",
					value: e.id || ""
				};
			})
		);
		// 技术
		Upsert.value?.setOptions(
			"technicianId",
			userList.value.map((e: { nickName: any; id: any }) => {
				return {
					label: e.nickName || "",
					value: e.id || ""
				};
			})
		);
		// 项目对接人
		Upsert.value?.setOptions(
			"siteManagerId",
			userList.value.map((e: { nickName: any; id: any }) => {
				return {
					label: e.nickName || "",
					value: e.id || ""
				};
			})
		);
	},
	onSubmit(data, { next }) {
		let form: any = {
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
		() => {
			return {
				label: "下单者",
				prop: "userName",
				hidden: uidDisabled.value
			};
		},
		{ label: "项目订单", prop: "orderCode" },
		{
			label: "域名",
			prop: "domain",
			minWidth: 150
		},
		{ label: "网站名称", prop: "siteName", minWidth: 200 },
		{
			label: "项目进度",
			prop: "state"
		},
		// { label: "价格", prop: "price" },
		{
			label: "续费时间",
			prop: "endDate"
		},
		{
			label: "创建时间",
			prop: "createTime",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{ type: "op", buttons: ["slot-subOp"], width: 260 }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.crm.order,
		async onRefresh(params, { render }) {
			const { list, pagination } = await service.crm.order.page(params);
			// 渲染数据
			render(list, pagination);
			// 整理订单类型
			orderTypeObj.value = doFilterOrderType();
			// 会员
			userList.value = await service.base.sys.user.list();

			userData.value = userFilter(userList.value);
			userMapList.value = getUser(userList.value);
		}
	},
	(app) => {
		app.refresh({ departmentId: user.info?.departmentId });
	}
);

// 后台用户列表：客服，技术员，对接人字段使用
const getUser = (userList: any) => {
	return userList.map((e: { nickName: any; id: any }) => {
		return {
			label: e.nickName || "",
			value: e.id || ""
		};
	});
};

// 整理会员数据，展开信息用到
const userFilter = (list: any) => {
	let userData: any = {};
	list.forEach((e: { id: any; name: any }) => {
		userData[e.id] = e.name;
	});
	return userData;
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
			label: "订单类型",
			prop: "orderType",
			component: {
				name: "el-select",
				props: {
					clearable: true
				},
				options: dict.get("orderType")
			}
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

const drawerSubOrder = ref(false);
// 子订单
const subList = (row: any) => {
	drawerSubOrder.value = true;
	orderInfo.value = row;
	callKey.value++;
};

const edit = (row: any) => {
	Crud.value?.rowEdit(row);
};

const drawerFinance = ref(false);
// 财务列表
const financeList = (row: any) => {
	drawerFinance.value = true;
	orderInfo.value = row;
	callKey.value++;
};

// 跟踪处理
const drawerDoHistory = ref(false);
const doHistory = (row: any) => {
	drawerDoHistory.value = true;
	orderInfo.value = row;
	callKey.value++;
};

// 取项目进度值
const doFilterState = (v: number) => {
	if (stateList.length) {
		const data: any = stateList.filter((e) => e.value == v);
		if (data.length) {
			return data[0];
		}
	}
};

// 项目订单类型
const doFilterOrderType = () => {
	let orderTypeObj_: any = [];
	dict.get("orderType").value.forEach((ele: any) => {
		orderTypeObj_[ele?.value] = ele?.label;
	});
	return orderTypeObj_;
};

// 点击项目进度
const clickState = (row: any) => {
	if (row.state == 2) {
		return false;
	}
	if (row.state < 2) row.state++;
	service.crm.order
		.update({
			...row
		})
		.then(() => {
			ElNotification({
				title: "成功",
				message: "进度更新",
				type: "success"
			});
		});
};

const loading = ref(false);
interface ListItem {
	value: string;
	label: string;
}
// 客户远程搜索
const optionsCustomerId = ref<ListItem[]>([]);
const remoteMethodCustomer = async (keyWord: string) => {
	if (keyWord && keyWord !== "") {
		loading.value = true;
		const customerList = await service.crm.customer.list({ keyWord });
		loading.value = false;
		optionsCustomerId.value = customerList.map((e) => {
			return {
				label: e.company ? e.company + ": " + e.customerName : "" + e.customerName,
				value: e.id
			};
		});
	}
};

// 渠道远程搜索
const optionsAgentId = ref<ListItem[]>([]);
const remoteMethodAgent = async (keyWord: string) => {
	if (keyWord && keyWord !== "") {
		loading.value = true;
		const customerList = await service.crm.customer.list({ keyWord });
		loading.value = false;
		optionsAgentId.value = customerList.map((e) => {
			return {
				label: e.company ? e.company + ": " + e.customerName : "" + e.customerName,
				value: e.id
			};
		});
	}
};

// 追加域名
const addDomin = (v: any) => {
	let str = "";
	str = newDomain.value;
	if (v) {
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

// 查看全部子订单
const openSubOrder = () => {
	drawerSubOrder.value = true;
	orderInfo.value = {};
	callKey.value++;
};
</script>

<style lang="scss">
.more_btn {
	margin-bottom: 5px;
}

.textRed {
	color: red;
}
</style>
