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
			<!-- 按钮 -->
			<cl-adv-btn />
			<!-- 关键字搜索 -->
			<cl-search-key />
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table">
				<template #column-nickName="{ scope }">
					<div class="nickName">
						<div>
							{{ scope.row.nickName }}
						</div>
						<el-button type="warning" plain>{{ scope.row.levelName }}</el-button>
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
		<cl-upsert ref="Upsert" />

		<!-- 弹窗 -->
		<cl-adv-search ref="AdvSearch" />
	</cl-crud>
</template>

<script lang="ts" name="member-user" setup>
import { useCrud, useTable, useUpsert, useAdvSearch } from "@cool-vue/crud";
import { useCool } from "/@/cool";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "会员账号", prop: "username", required: true, component: { name: "el-input" } },
		() => {
			return {
				label: "会员密码",
				prop: "password",
				required: Upsert.value?.mode == "add" ? true : false,
				value: "",
				component: { name: "el-input", props: { type: "password" } }
			};
		},
		{ label: "会员昵称", prop: "nickName", component: { name: "el-input" } },
		{
			label: "会员等级",
			prop: "level",
			value: 1,
			component: {
				name: "el-select",
				options: [
					{
						label: "普通会员",
						value: 1
					},
					{
						label: "vip会员",
						value: 2
					},
					{
						label: "至尊会员",
						value: 3
					}
				]
			}
		},
		{
			label: "性别",
			prop: "sex",
			value: 0,
			component: {
				name: "el-select",
				options: [
					{
						label: "选择",
						value: 0
					},
					{
						label: "男",
						value: 1
					},
					{
						label: "女",
						value: 2
					}
				]
			}
		},
		{ label: "手机号码", prop: "mobile", component: { name: "el-input" } },
		{ label: "qq", prop: "qq", component: { name: "el-input" } },
		{ label: "微信", prop: "wx", component: { name: "el-input" } },
		{ label: "微信二维码", prop: "wxImg", component: { name: "cl-upload" } },
		{ label: "email", prop: "email", component: { name: "el-input" } },
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		},
		{
			label: "状态",
			prop: "status",
			value: 1,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } }
		}
	],
	onSubmit(data, { done, close, next }) {
		data.levelName = data.level == 1 ? "普通会员" : data.level == 2 ? "vip会员" : "至尊会员";
		next({
			...data
		});
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "id", prop: "id" },
		{ label: "会员账号", prop: "username" },
		{ label: "会员昵称", prop: "nickName" },
		{ label: "手机号码", prop: "mobile" },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.cms.memberUser
	},
	(app) => {
		app.refresh();
	}
);

const AdvSearch = useAdvSearch({
	items: [
		{
			label: "会员等级",
			prop: "level",
			component: {
				name: "el-select",
				props: {
					clearable: true
				},
				options: [
					{
						label: "普通会员",
						value: 1
					},
					{
						label: "vip会员",
						value: 2
					},
					{
						label: "至尊会员",
						value: 3
					}
				]
			}
		}
	]
});
</script>

<style scoped lang="scss">
.nickName {
	display: flex;
	flex-direction: row;
	justify-content: center;
	gap: 10px;
}
</style>
