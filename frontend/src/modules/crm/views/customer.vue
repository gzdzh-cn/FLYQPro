<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn style="margin-right: 10px" />

			<cl-flex1 />
			<!-- 关键字搜索 -->
			<cl-search-key />

			<!-- 导出按钮 -->
			<!-- <cl-export-btn :columns="Table?.columns" />
			<cl-import-btn
				tips="请按照模版填写信息"
				:on-submit="onImportSubmit"
				:on-config="onImportConfig"
			/> -->
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

<script lang="ts" name="crm-customer" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { useDict } from "../../dict";
import { ElMessage } from "element-plus";

const { service } = useCool();
const { user } = useBase();
const { dict } = useDict();

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
					}
				]
			}
		},
		{
			label: "会员",
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
			component: { name: "el-select", options: [], props: {} },
			group: "base"
		},
		{
			label: "是否公共库",
			prop: "isCommon",
			span: 12,
			value: 0,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } },
			group: "base"
		},
		{
			label: "客户名称",
			prop: "customerName",
			required: true,
			span: 12,
			component: { name: "el-input" },
			group: "base"
		},
		{
			label: "公司名称",
			prop: "company",
			span: 12,
			component: { name: "el-input" },
			group: "base"
		},
		{
			label: "来源",
			prop: "source",
			span: 12,
			component: {
				name: "el-select",
				options: dict.get("source")
			},
			group: "base"
		},
		{
			label: "咨询品类",
			prop: "consultation",
			span: 12,
			component: {
				name: "el-select",
				options: dict.get("orderType")
			},
			group: "base"
		},

		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } },
			group: "base"
		},

		{
			label: "公司网址",
			prop: "companyDomain",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{ label: "电话", prop: "phone", span: 12, component: { name: "el-input" }, group: "other" },
		{ label: "微信", prop: "wx", span: 12, component: { name: "el-input" }, group: "other" },
		{ label: "QQ", prop: "qq", span: 12, component: { name: "el-input" }, group: "other" },
		{
			label: "Email",
			prop: "email",
			span: 12,
			component: { name: "el-input" },
			group: "other"
		},
		{
			label: "公司地址",
			prop: "address",
			span: 12,
			component: { name: "el-input" },
			group: "other"
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
		}
	},
	onSubmit(data, { next }) {
		let form: any = {};
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
		{ label: "客户名称", prop: "customerName" },
		{ label: "公司名称", prop: "company" },
		{
			label: "咨询品类",
			prop: "consultation",
			dict: {
				text: true,
				separator: "",
				options: dict.get("orderType")
			}
		},
		{
			label: "创建时间",
			prop: "createTime",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.crm.customer
	},
	(app) => {
		app.refresh();
	}
);

// const onImportConfig = (Form: ClForm.Ref) => {
// 	return [
// 		{
// 			label: "选择公司",
// 			prop: "companyId",
// 			component: {
// 				name: "el-select",
// 				options: [
// 					{
// 						label: "COOL",
// 						value: 1
// 					},
// 					{
// 						label: "神仙",
// 						value: 2
// 					}
// 				]
// 			}
// 		}
// 	];
// };

// const onImportSubmit = (data: any, { next, done, setProgress }: any) => {
// 	service.test
// 		.add(data)
// 		.then(() => {
// 			ElMessage.success("导入成功");
// 			close();
// 		})
// 		.catch((err) => {
// 			ElMessage.error(err.message);
// 			done();
// 		});
// };
</script>
