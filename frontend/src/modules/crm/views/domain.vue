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
			<template #slot-orderCode="{ scope }">
				<el-select-v2
					v-model="scope.orderCode"
					filterable
					remote
					:remote-method="remoteMethod"
					clearable
					:options="options"
					:loading="loading"
					placeholder="请输入订单号"
				/>
			</template>
		</cl-upsert>
	</cl-crud>
</template>

<script lang="ts" name="crm-domain" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { useDict } from "/$/dict";
import { ref } from "vue";
import { dayjs } from "element-plus";

const { service } = useCool();
const { user } = useBase();
const { dict } = useDict();
const uidDisabled = ref(user.info.id == 1 ? false : true);
// dict.refresh();

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			label: "会员",
			prop: "uid",
			required: true,
			span: 12,
			value: user.info.id,
			hidden: () => {
				if (user.info.id == 1) {
					return false;
				} else {
					return true;
				}
			},
			component: { name: "el-select", options: [], props: {} }
		},
		{
			label: "项目订单",
			prop: "orderCode",
			// required: true,
			span: 12,
			component: { name: "slot-orderCode" }
		},

		{ label: "域名", prop: "domain", span: 12, component: { name: "el-input" } },
		{
			label: "注册时间",
			prop: "startDate",
			span: 12,
			value: dayjs().format("YYYY-MM-DD"),
			component: {
				name: "el-date-picker",
				props: { type: "date", valueFormat: "YYYY-MM-DD" }
			},
			group: "base"
		},
		{
			label: "到期时间",
			prop: "endDate",
			span: 12,
			value: dayjs().add(1, "year").format("YYYY-MM-DD"),
			component: {
				name: "el-date-picker",
				props: { type: "date", valueFormat: "YYYY-MM-DD" }
			},
			group: "base"
		},
		{
			label: "域名商",
			prop: "domainCompany",
			span: 12,
			component: { name: "el-select", options: dict.get("domainProvider") }
		},
		// { label: "域名所有者", prop: "domainOwer", span: 12, component: { name: "el-input" } },
		{
			label: "实名资料",
			prop: "realNameDataFiles",
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
		form.projectNum = data.orderCode.split("-")[0];
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
		{ label: "会员", prop: "userName", hidden: uidDisabled.value },
		{ label: "项目", prop: "orderCode" },
		{ label: "域名", prop: "domain" },
		{
			label: "域名商",
			prop: "domainCompany",
			dict: {
				text: true,
				separator: "",
				options: dict.get("domainProvider")
			}
		},
		{
			label: "注册时间",
			prop: "startDate",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{
			label: "续费时间",
			prop: "endDate",
			component: { name: "cl-date-text", props: { format: "YYYY-MM-DD" } }
		},
		{ type: "op", buttons: ["edit", "delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.crm.domain,
		async onRefresh(params, { render }) {
			const { list, pagination } = await service.crm.domain.page(params);
			render(list, pagination);
		}
	},
	(app) => {
		app.refresh();
	}
);

interface ListItem {
	value: string;
	label: string;
}

// 项目订单远程搜索
const options = ref<ListItem[]>([]);
const loading = ref(false);
const remoteMethod = async (keyWord: string) => {
	if (keyWord !== "") {
		loading.value = true;
		// 项目订单
		const orderList = await service.crm.order.list({ keyWord });
		loading.value = false;
		options.value = orderList.map((e) => {
			const r: any = doFilter(e.orderType);
			return {
				label: r?.label + "-" + e.orderCode,
				value: e.orderCode
			};
		});
	}
};
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
</script>
