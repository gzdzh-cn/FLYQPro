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
			<cl-table ref="Table" :border="false">
				<template #column-state="{ scope }">
					<el-popconfirm title="跟踪状态更新?" @confirm="clickState(scope.row)">
						<template #reference>
							<el-button text bg :type="doFilterState(scope.row.state).type">
								{{ doFilterState(scope.row.state).label }}
							</el-button>
						</template>
					</el-popconfirm>
				</template>

				<template #slot-reply="{ scope }">
					<el-button text bg type="warning" @click="clickReply(scope.row)">
						反馈
					</el-button>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert" />
		<el-dialog v-model="dialogReply" title="反馈列表" :fullscreen="true">
			<order-history-reply
				:orderInfo="orderInfo"
				:historyInfo="historyInfo"
				:key="callKey"
				style="height: 80vh"
			></order-history-reply>
		</el-dialog>
	</cl-crud>
</template>

<script lang="ts" name="order-doHistory" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { useDict } from "../../../../dict";
import { reactive, ref, watch } from "vue";
import { ElNotification } from "element-plus";
import OrderHistoryReply from "./history/reply.vue";
import { filterHtml, subString } from "/@/dzh";

const props = defineProps({
	orderInfo: Object
});
watch(
	() => props.orderInfo,
	(n?: any) => {
		Crud.value?.refresh({ orderId: n.id });
	},
	{
		deep: true
	}
);

const { service } = useCool();
const { user } = useBase();
const { dict } = useDict();
const isAdmin = ref(user.info?.id == 1 ? true : false);

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		() => {
			return {
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
			};
		},
		() => {
			return {
				label: "标题",
				prop: "title",
				required: true,
				component: {
					name: "el-input",
					props: {
						disabled: !isAdmin.value && Upsert.value?.mode == "update"
					}
				}
			};
		},
		() => {
			return {
				label: "项目订单",
				span: 12,
				prop: "orderCode",
				component: {
					name: "el-select",
					options: [],
					props: {
						disabled: !isAdmin.value && Upsert.value?.mode == "update"
					}
				}
			};
		},
		() => {
			return {
				label: "跟踪状态",
				prop: "state",
				span: 12,
				hidden: () => {
					return Upsert.value?.mode == "add";
				},
				component: {
					name: "el-select",
					options: stateList,
					props: {
						disabled: !isAdmin.value && Upsert.value?.mode == "update"
					}
				}
			};
		},
		() => {
			return {
				label: "图片上传",
				prop: "imgs",
				component: {
					name: "cl-upload",
					props: {
						disabled: !isAdmin.value && Upsert.value?.mode == "update",
						type: "image",
						multiple: true,
						showFileList: true,
						draggable: true,
						rename: false
					}
				}
			};
		},
		() => {
			return {
				label: "文件上传",
				prop: "files",
				component: {
					name: "cl-upload",
					props: {
						disabled: !isAdmin.value && Upsert.value?.mode == "update",
						type: "file",
						multiple: true,
						showFileList: true,
						draggable: true,
						rename: false
					}
				}
			};
		},
		() => {
			return {
				label: "备注",
				prop: "remark",
				component: {
					name: "el-input",
					props: {
						disabled: !isAdmin.value && Upsert.value?.mode == "update",
						type: "textarea",
						rows: 4
					}
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

		// 项目订单
		const orderList = await service.crm.order.list({ projectNum: props.orderInfo?.projectNum });
		Upsert.value?.setOptions(
			"orderCode",
			orderList.map((e) => {
				const r: any = doFilter(e.orderType);
				return {
					label: r?.label + "-" + e.orderCode + "-" + subString(filterHtml(e.remark), 10),
					value: e.orderCode
				};
			})
		);
		if (Upsert.value?.mode == "add") {
			Upsert.value?.setTitle(`${props.orderInfo?.projectNum}-新增跟踪`);
		}
		if (Upsert.value?.mode == "update") {
			Upsert.value?.setTitle(`${props.orderInfo?.projectNum}-编辑跟踪`);
		}
	},
	onSubmit(data, { next }) {
		let form: any = {
			orderId: props.orderInfo?.id,
			projectNum: props.orderInfo?.projectNum
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
		{ label: "会员", prop: "userName" },
		{ label: "标题", prop: "title", showOverflowTooltip: true },
		{ label: "项目订单", prop: "orderCode" },
		{
			label: "跟踪状态",
			prop: "state"
		},
		{
			label: "备注",
			prop: "remark",
			showOverflowTooltip: true,
			minWidth: 150
		},
		{
			label: "创建时间",
			prop: "createTime",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{ type: "op", buttons: ["edit", "slot-reply"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.crm.dohistory,
		async onRefresh(params, { render }) {
			params.orderId = props.orderInfo?.id;
			const { list, pagination } = await service.crm.dohistory.page(params);
			render(list, pagination);
		}
	},
	(app) => {
		app.refresh();
	}
);

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

// 取项目进度值
const doFilterState = (v: number) => {
	const data: any = stateList.filter((e) => e.value == v);
	return data[0];
};

// 点击跟踪状态
const clickState = (row: any) => {
	if (row.state == 2) {
		return ElNotification({
			title: "提醒",
			message: "不可再更新",
			type: "warning"
		});
	}
	if (row.state < 2) row.state++;
	service.crm.dohistory
		.update({
			...row
		})
		.then(() => {
			ElNotification({
				title: "成功",
				message: "状态更新",
				type: "success"
			});
		});
};

// 强制子组件视图重载
const callKey = ref(0);
const historyInfo = ref();

// 反馈
const dialogReply = ref(false);
const clickReply = (row: any) => {
	dialogReply.value = true;
	historyInfo.value = row;
	callKey.value++;
};
</script>
