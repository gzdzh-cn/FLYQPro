<template>
	<cl-crud ref="Crud">
		<cl-row>
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<!-- <cl-add-btn /> -->
			<!-- 删除按钮 -->
			<cl-multi-delete-btn />
			<cl-flex1 />
			<!-- 关键字搜索 -->
			<!-- <cl-search-key /> -->
		</cl-row>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" style="height: 500px" />
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<!-- <cl-upsert ref="Upsert" /> -->
	</cl-crud>
</template>

<script lang="ts" name="excel_down" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { onMounted, ref } from "vue";
import { isEmpty } from "lodash";

const props = defineProps({
	cluesStatus: {
		type: Number,
		default: 0
	},
	isAdmin: {
		type: Boolean,
		default: false
	},
	dtype: {
		type: Number,
		default: 0
	}
});
defineExpose({
	refresh,
	stopInterval
});

const { service } = useCool();

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "创建时间", prop: "createTime" },
		{
			label: "操作人",
			prop: "userName",
			hidden: !props.isAdmin
		},
		{ label: "生成时间", prop: "taskTime" },
		{
			label: "状态",
			prop: "taskTime",
			formatter(row) {
				return row.status == 1 ? "生成中" : "生成完成";
			}
		},
		{
			type: "op",
			buttons: [
				{
					label: "下载",
					onClick(e: { scope: any }) {
						download(e.scope.row.downLoad);
					}
				},
				"delete"
			]
		}
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.customer_pro.clues_excel,
	async onRefresh(params, { render }) {
		params.clues_status = props.cluesStatus;
		params.dtype = props.dtype;
		const { list, pagination } = await service.customer_pro.clues_excel.page(params);
		const itemAllComplete = list?.every((item) => item.status == 0);

		if (itemAllComplete || isEmpty(list)) {
			clearInterval(task.value);
		}
		render(list, pagination);
	}
});
// 刷新
const task = ref();
function refresh() {
	Crud.value?.refresh();
	task.value = setInterval(() => {
		Crud.value?.refresh();
	}, 5000);
}

function stopInterval() {
	clearInterval(task.value);
}

const download = (downLoadUrl: string) => {
	console.log("downLoadUrl", downLoadUrl);

	const link = document.createElement("a");
	link.href = downLoadUrl;
	link.download = ""; // 可以设置文件名，留空则按服务器提供的文件名
	// 将标签添加到页面并触发点击
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
};

onMounted(async () => {
	console.log("excelDown");

	refresh();
});
</script>
