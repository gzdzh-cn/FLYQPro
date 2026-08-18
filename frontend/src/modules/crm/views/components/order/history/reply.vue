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
			<div style="width: 100%; height: 100%">
				<!-- 数据表格 -->
				<cl-table ref="Table" />
			</div>
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

<script lang="ts" name="history-reply" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { onMounted, reactive, ref } from "vue";

const props = defineProps({
	orderInfo: Object,
	historyInfo: Object
});

const { service } = useCool();
const { user } = useBase();

const uidDisabled = ref(user.info?.id == 1 ? false : true);

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "会员",
			prop: "uid",

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
			label: "标题",
			prop: "title",

			required: true,
			component: {
				name: "el-input"
			}
		},
		{
			label: "图片上传",
			prop: "imgs",
			component: {
				name: "cl-upload",
				props: {
					type: "image",
					multiple: true,
					showFileList: true,
					draggable: true,
					rename: false
				}
			}
		},
		{
			label: "文件上传",
			prop: "files",
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
			if (Upsert.value?.mode == "add") {
				Upsert.value?.setForm("uid", user.info.id);
			}
		}
	},
	onSubmit(data, { next }) {
		let form: any = {
			orderId: props.orderInfo?.id,
			projectNum: props.orderInfo?.projectNum,
			orderCode: props.orderInfo?.orderCode,
			historyId: props.historyInfo?.id
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
		{ label: "会员", prop: "userName", hidden: uidDisabled },
		{ label: "标题", prop: "title" },
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
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.crm.reply,
	async onRefresh(params, { render }) {
		params.historyId = props.historyInfo?.id;
		const { list, pagination } = await service.crm.reply.page(params);
		render(list, pagination);
	}
});

onMounted(async () => {
	Crud.value?.refresh();
});
</script>

<style lang="scss">
.form-reply {
	width: 100%;
	border: 1px solid #ece6e6;
	padding-top: 30px;
}
.form-reply-content {
	width: 100%;
	height: 250px;
	.form-title {
		width: 500px;
	}
	.form-remark {
		width: 80%;
	}
	.imgs-upload {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		align-items: center;
	}
}
.sub-reply {
	height: 60px;
	display: flex;
	flex-direction: row;
	justify-content: flex-start;
	align-items: center;
}
</style>
