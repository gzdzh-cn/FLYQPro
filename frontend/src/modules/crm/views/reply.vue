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
		<cl-upsert ref="Upsert">
			<template #slot-historyId="{ scope }">
				<!-- 下拉远程搜索 -->
				<el-select-v2
					v-model="scope.historyId"
					filterable
					remote
					:remote-method="remoteMethodHistoryId"
					@change="checkHistoryId"
					clearable
					:options="options"
					:loading="loading"
					placeholder="请输入跟踪订单"
					:disabled="Upsert?.mode == 'add' ? false : true"
				/>
			</template>
		</cl-upsert>
	</cl-crud>
</template>

<script lang="ts" name="crm-reply" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
// import { useDict } from "/$/dict";
import { ref } from "vue";

const { service } = useCool();
const { user } = useBase();
// const { dict } = useDict();

const uidDisabled = ref(user.info?.id == 1 ? false : true);
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
		{ label: "标题", prop: "title", required: true, component: { name: "el-input" } },
		{
			label: "跟踪订单",
			// span: 12,
			prop: "historyId",
			component: {
				name: "slot-historyId"
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

		// 跟踪订单
		if (Upsert.value?.mode == "update") {
			await remoteMethodHistoryId(data.orderCode);
		}
	},
	async onSubmit(data, { next }) {
		let form: any = {
			orderId: orderData.value[data.historyId].orderId,
			projectNum: orderData.value[data.historyId].projectNum,
			orderCode: orderData.value[data.historyId].orderCode
		};
		console.log("form", form);

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
const Crud = useCrud(
	{
		service: service.crm.reply
	},
	(app) => {
		app.refresh();
	}
);

interface ListItem {
	value: string;
	label: string;
}

const orderData = ref<any>({});
// 跟踪订单远程搜索
const options = ref<ListItem[]>([]);
const loading = ref(false);
const remoteMethodHistoryId = async (keyWord: string) => {
	if (keyWord !== "") {
		loading.value = true;
		// 跟踪订单
		const dohistoryList = await service.crm.dohistory.list({ keyWord });

		orderData.value = {};
		loading.value = false;
		options.value = dohistoryList.map((e) => {
			orderData.value[e.id] = e;
			return {
				label: e.title + "-" + e.orderCode,
				value: e.id
			};
		});
		console.log("form", Upsert.value?.getForm());
	}
};

const historyId = ref();
// 选中
const checkHistoryId = (v: any) => {
	historyId.value = v;
};
</script>
