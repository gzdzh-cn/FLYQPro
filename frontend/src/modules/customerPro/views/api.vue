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
				<!-- <template #column-detail="{ scope }">
					<div style="padding: 0 30px">
						<p>分配列表: {{ scope.row?.allotServicesNames }}</p>
					</div>
				</template> -->
			</cl-table>
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

<script lang="ts" name="customer_pro-api" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ElMessage, ElMessageBox } from "element-plus";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "账号名称", prop: "name", required: true, component: { name: "el-input" } },
		{ label: "账户编号", prop: "account", component: { name: "el-input" } },
		{
			label: "接入平台",
			prop: "platform",
			component: {
				name: "el-select",
				options: [
					{
						label: "手动录入",
						value: 1
					},
					{
						label: "百度",
						value: 2
					},
					{
						label: "抖音",
						value: 3
					},
					{
						label: "53客服",
						value: 4
					},
					{
						label: "小红书",
						value: 5
					}
				]
			}
		},
		{ label: "token", prop: "token", component: { name: "el-input" } },
		() => {
			return {
				label: "所属项目",
				prop: "projectId",
				value: [],
				hook: {
					bind: ["split"],
					submit: ["join"]
				},
				component: {
					name: "el-select",
					options: [],
					props: {
						multiple: true,
						"multiple-limit": 3
					}
				}
			};
		},
		// {
		// 	label: "分配方式",
		// 	prop: "allotType",
		// 	value: 1,
		// 	component: {
		// 		name: "el-select",
		// 		options: [
		// 			{
		// 				label: "不分配",
		// 				value: 0
		// 			},
		// 			{
		// 				label: "按顺序分配",
		// 				value: 1
		// 			},
		// 			{
		// 				label: "随机分配",
		// 				value: 2
		// 			}
		// 		]
		// 	}
		// },
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		},
		{
			label: "状态",
			prop: "status",
			value: 1,
			component: {
				name: "el-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "激活",
					inactiveText: "关闭",
					inlinePrompt: true
				}
			}
		}
	],
	async onOpen() {
		const projectList = await service.customer_pro.project.list();
		if (projectList.length == 0) {
			ElMessageBox.alert("请先到左侧栏目：院校管理 >> 项目管理，添加项目", "警告", {
				confirmButtonText: "OK",
				callback: () => {
					Upsert.value?.close();
				}
			});
			return;
		}
		Upsert.value?.setOptions(
			"projectId",
			projectList.map((e) => {
				return {
					label: e.name,
					value: e.id
				};
			})
		);
		const kfList = await service.customer_pro.kf.list();
		if (kfList.length == 0) {
			ElMessageBox.alert(
				"请先到左侧栏目：院校管理 >> 项目管理 >> 客服列表，添加客服成员",
				"警告",
				{
					confirmButtonText: "OK",
					callback: () => {
						Upsert.value?.close();
					}
				}
			);
			return;
		}
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		// () => {
		// 	return {
		// 		label: "#",
		// 		type: "expand",
		// 		prop: "detail"
		// 	};
		// },
		{ label: "账号名称", prop: "name" },
		{ label: "账户编号", prop: "account" },
		{
			label: "接入平台",
			prop: "platform",
			dict: [
				{
					label: "手动录入",
					value: 1
				},
				{
					label: "百度",
					value: 2
				},
				{
					label: "抖音",
					value: 3
				},
				{
					label: "53客服",
					value: 4
				},
				{
					label: "小红书",
					value: 5
				}
			]
		},
		{ label: "所属项目", prop: "projectName" },
		// {
		// 	label: "分配方式",
		// 	prop: "allotType",
		// 	dict: [
		// 		{
		// 			label: "不分配",
		// 			value: 0
		// 		},
		// 		{
		// 			label: "按顺序分配",
		// 			value: 1
		// 		},
		// 		{
		// 			label: "随机分配",
		// 			value: 2
		// 		}
		// 	]
		// },
		// { label: "分配列表", prop: "allotServicesNames" },
		// { label: "下次分配", prop: "userName" },
		{
			label: "接收线索",
			prop: "status",
			component: {
				name: "cl-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "接收",
					inactiveText: "禁止",
					inlinePrompt: true
				}
			}
		},
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.api
	},
	(app) => {
		app.refresh();
	}
);
</script>
