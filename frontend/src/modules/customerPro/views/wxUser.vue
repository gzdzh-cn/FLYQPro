<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />

			<!-- 删除按钮 -->
			<cl-multi-delete-btn />
			<cl-flex1 />
			<!-- 关键字搜索 -->
			<cl-search-key />
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="false">
				<template #column-subscribe="{ scope }">
					<template v-if="scope.row._subscribeChecked">
						<el-tag v-if="scope.row.subscribe === 1" type="success" size="small">已关注</el-tag>
						<el-tag v-else type="info" size="small">未关注</el-tag>
					</template>
					<el-button v-else type="primary" link size="small" :loading="scope.row._subscribeLoading"
						@click="checkSubscribe(scope.row)">
						查询
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
	</cl-crud>
</template>

<script lang="ts" name="customer_pro-wxUser" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ElMessage } from "element-plus";

const { service } = useCool();

// 查询关注状态
const checkSubscribe = async (row: any) => {
	row._subscribeLoading = true;
	try {
		const res = await service.customer_pro.wxUser.getWxUserInfo({ userId: row.user_id });
		row.subscribe = res.subscribe === 1 ? 1 : 0;
		row._subscribeChecked = true;
		ElMessage.success(row.subscribe === 1 ? "已关注" : "未关注");
	} catch (e: any) {
		ElMessage.error("查询失败");
	} finally {
		row._subscribeLoading = false;
	}
};

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
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
			required: true
		},
		{
			label: "头像",
			prop: "headimgurl",
			component: { name: "cl-upload", props: { listType: "text", limit: 1 } },
			required: true
		},
		{ label: "用户", prop: "userName", component: { name: "el-input" } },
		{ label: "微信昵称", prop: "nickname", required: true, component: { name: "el-input" } },
		{ label: "openid", prop: "openid", component: { name: "el-input" } }
		// {
		// 	label: "备注",
		// 	prop: "remark",
		// 	component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		// }
		// {
		// 	label: "状态",
		// 	prop: "status",
		// 	value: 1,
		// 	component: {
		// 		name: "cl-switch",
		// 		props: {
		// 			activeValue: 1,
		// 			inactiveValue: 0,
		// 			activeText: "开启",
		// 			inactiveText: "关闭",
		// 			inlinePrompt: true
		// 		}
		// 	}
		// }
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		// { label: "id", prop: "id" },
		{
			label: "公众号",
			prop: "mp_id",
			formatter(row) {
				const map: Record<number, string> = {
					1: "领南教育",
					2: "同伈服务",
					3: "炜创科技"
				};
				return map[row.mp_id] || row.mp_id;
			}
		},
		{
			label: "类型",
			prop: "type",
			dict: [
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
		{ label: "头像", prop: "headimgurl", component: { name: "cl-image" } },
		{ label: "用户", prop: "userName" },
		{ label: "微信昵称", prop: "nickname" },
		{ label: "关注公众号", prop: "subscribe" },

		// { label: "unionid", prop: "unionid" },
		{ label: "openid", prop: "openid" },

		// { label: "备注", prop: "remark", showOverflowTooltip: true },

		// {
		// 	label: "状态",
		// 	prop: "status",
		// 	component: {
		// 		name: "cl-switch",
		// 		props: {
		// 			activeValue: 1,
		// 			inactiveValue: 0,
		// 			activeText: "开启",
		// 			inactiveText: "关闭",
		// 			inlinePrompt: true
		// 		}
		// 	}
		// },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["info", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.wxUser
	},
	(app) => {
		app.refresh();
	}
);
</script>
