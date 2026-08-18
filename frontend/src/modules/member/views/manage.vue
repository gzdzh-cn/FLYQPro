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
				<template #column-levelName="{ scope }">
					<div class="levelName">
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

<script lang="ts" name="member" setup>
import { useCrud, useTable, useUpsert, useAdvSearch } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { computed, onMounted } from "vue";

const { user } = useBase();
const { service } = useCool();

const hidden = computed(() => {
	return !user.info?.roleIds.split(",").includes("1");
});

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
						label: "更多信息",
						value: "more"
					},
					{
						label: "其他信息",
						value: "other"
					}
				]
			}
		},
		// {
		// 	label: "租户标识",
		// 	prop: "userId",
		// 	span: 12,
		// 	hidden: hidden.value,
		// 	component: { name: "el-select", options: [] },
		// 	group: "base"
		// },
		{
			label: "会员账号",
			prop: "memberName",
			span: 12,
			required: true,
			component: { name: "el-input" },
			group: "base"
		},
		() => {
			return {
				label: "会员密码",
				prop: "password",
				span: 12,
				required: Upsert.value?.mode == "add" ? true : false,
				value: "",
				component: {
					name: "el-input",
					props: {
						type: "password",
						autocomplete: "new-password",
						showPassword: true
					}
				},
				group: "base"
			};
		},
		{
			label: "会员昵称",
			prop: "nickname",
			span: 12,
			component: { name: "el-input" },
			group: "base"
		},
		{
			label: "手机号码",
			prop: "mobile",
			span: 12,
			component: { name: "el-input" },
			group: "base"
		},

		{
			label: "状态",
			prop: "status",
			span: 12,
			value: 1,
			component: {
				name: "cl-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "开启",
					inactiveText: "关闭",
					inlinePrompt: true
				}
			},
			group: "base"
		},
		{
			label: "会员等级",
			prop: "level",
			span: 24,
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
			},
			group: "more"
		},
		{
			label: "性别",
			prop: "sex",
			span: 12,
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
			},
			group: "more"
		},
		{
			label: "qq",
			prop: "qq",
			span: 12,
			component: { name: "el-input" },
			group: "more"
		},
		{
			label: "微信",
			prop: "wx",
			span: 12,
			component: { name: "el-input" },
			group: "more"
		},
		{
			label: "email",
			prop: "email",
			span: 12,
			component: { name: "el-input" },
			group: "more"
		},
		{
			label: "微信二维码",
			prop: "wxImg",
			component: { name: "cl-upload" },
			group: "more"
		},
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } },
			group: "more"
		},
		{
			label: "类型",
			prop: "type",
			value: 1,
			component: {
				name: "el-radio-group",
				options: [
					{
						label: "公众号",
						value: 1
					},
					{
						label: "小程序",
						value: 2
					}
				]
			},
			group: "other"
		},
		{
			label: "头像",
			prop: "headimgurl",
			component: { name: "cl-upload", props: { listType: "text", limit: 1 } },
			group: "other"
		},
		{
			label: "订阅通知",
			prop: "notify",
			value: 0,
			span: 12,
			component: {
				name: "cl-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "开启",
					inactiveText: "关闭",
					inlinePrompt: true
				}
			},
			group: "other"
		}
	],
	async onOpen() {
		const userList = await service.base.sys.user.list();
		Upsert.value?.setOptions(
			"userId",
			userList.map((e: any) => {
				return { label: e.username || "", value: e.id || "" };
			})
		);
	},
	onSubmit(data, { next }) {
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
		{ label: "唯一码", prop: "id", formatter: (row) => row.id.slice(-6) },
		// { label: "租户", prop: "username", hidden: hidden.value },
		{ label: "会员账号", prop: "memberName" },
		{ label: "会员昵称", prop: "nickname" },
		{ label: "会员等级", prop: "levelName" },
		{ label: "手机号码", prop: "mobile" },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.member.manage
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

onMounted(() => {});
</script>

<style scoped lang="scss">
.levelName {
	display: flex;
	flex-direction: row;
	justify-content: center;
	gap: 10px;
}
</style>
