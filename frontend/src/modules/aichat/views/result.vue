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
			<cl-table ref="Table">
				<template #slot-review="{ scope }">
					<el-button type="warning" text bg @click="handleReview(scope.row)"
						>审核</el-button
					>
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

		<el-dialog v-model="isReviewDialogVisible" title="审核意见">
			<el-input
				type="textarea"
				v-model="reviewComment"
				placeholder="请输入审核意见"
			></el-input>
			<template #footer>
				<el-button @click="isReviewDialogVisible = false">取消</el-button>
				<el-button type="primary" @click="submitReview">提交</el-button>
			</template>
		</el-dialog>
	</cl-crud>
</template>

<script lang="ts" name="aichat-result" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ref } from "vue";
import dayjs from "dayjs";

const { service } = useCool();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		// { label: "用户ID", prop: "user_id", component: { name: "el-input" } },
		// { label: "资源标识", prop: "resource_id", component: { name: "el-input" } },
		{
			label: "审核状态",
			prop: "audit_status",
			span: 12,
			component: { name: "el-input" },
			required: true
		},
		{ label: "审核意见", prop: "audit_opinion", span: 12, component: { name: "el-input" } },
		{
			label: "审核时间",
			prop: "audit_time",
			span: 12,
			component: {
				name: "el-date-picker",
				props: { type: "datetime", valueFormat: "YYYY-MM-DD HH:mm:ss" }
			}
		},
		{ label: "内容", prop: "content", component: { name: "cl-editor-wang" }, required: true }
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "审核ID", prop: "id" },
		// { label: "用户ID", prop: "user_id" },
		// { label: "内容", prop: "content" },
		// { label: "资源标识", prop: "resource_id" },
		{ label: "审核状态", prop: "audit_status" },
		{ label: "审核意见", prop: "audit_opinion" },
		{ label: "审核时间", prop: "audit_time" },
		{ label: "创建时间", prop: "createTime" },
		{ label: "更新时间", prop: "updateTime" },
		{ type: "op", width: 320, buttons: ["slot-review", "info", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.aichat.result,
		async onRefresh(params, { next, render }) {
			const { list, pagination } = await next(params);
			render(list, pagination);
		}
	},
	(app) => {
		app.refresh();
	}
);

const isReviewDialogVisible = ref(false);
const id = ref("");
const reviewComment = ref("");

const handleReview = (row: any) => {
	reviewComment.value = "";
	id.value = row.id;
	isReviewDialogVisible.value = true;
};

const submitReview = async () => {
	await service.aichat.result.update({
		id: id.value,
		audit_opinion: reviewComment.value,
		audit_time: dayjs(new Date()).format("YYYY-MM-DD HH:mm:ss")
	});

	isReviewDialogVisible.value = false;
	await Crud.value?.refresh();
};
</script>
