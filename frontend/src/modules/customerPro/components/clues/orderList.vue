<template>
	<cl-crud ref="Crud">
		<cl-row>
			<cl-refresh-btn size="small" />
			<cl-add-btn size="small" />
			<cl-multi-delete-btn size="small" />
			<cl-flex1 />
			<cl-search-key size="small" />
		</cl-row>

		<cl-row>
			<cl-table ref="Table" :border="false" empty-text="暂无订单数据">
				<template #column-auditStatus="{ scope }">
					<span style="color: #d83b01" v-if="scope.row.auditStatus == 1">待审核</span>
					<span style="color: #00b294" v-else-if="scope.row.auditStatus == 2">审核通过</span>
					<span style="color: #ff8c00" v-else-if="scope.row.auditStatus == 3">审核驳回</span>
					<span v-else>-</span>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert">
			<template #slot-schoolId="{ scope }">
				<el-select v-model="scope.schoolId" @change="schoolChange">
					<el-option v-for="item in schoolList" :key="item.id" :label="item.name" :value="item.id" />
				</el-select>
			</template>

			<template #slot-majorsId="{ scope }">
				<el-select v-model="scope.majorsId">
					<el-option v-for="item in majorsList" :key="item.id" :label="item.name" :value="item.id" />
				</el-select>
			</template>
		</cl-upsert>
	</cl-crud>
</template>

<script lang="ts" name="clues-order-list" setup>
import { ref, watch, nextTick } from "vue";
import { useCool } from "/@/cool";
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";

const props = defineProps<{
	cluesId?: string | number;
}>();

const emit = defineEmits(["success"]);

const { service } = useCool();

// 学校/专业列表
const schoolList = ref<any[]>([]);
const majorsList = ref<any[]>([]);

const getSchoolList = async () => {
	schoolList.value = await service.customer_pro.school.list();
	if (schoolList.value?.[0]?.id) {
		getMajorList(schoolList.value[0].id);
	}
};

const schoolChange = async (v: any) => {
	majorsList.value = [];
	Upsert.value?.setForm("majorsId", null);
	getMajorList(v);
};

const getMajorList = async (v: any) => {
	majorsList.value = await service.customer_pro.majors.list({ schoolId: v });
};

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{
			type: "tabs",
			props: {
				labels: [
					{ label: "基础信息", value: "base" },
					{ label: "个人信息", value: "person" },
					{ label: "收款信息", value: "financial" }
				]
			}
		},
		{
			label: "学生名称",
			prop: "name",
			span: 8,
			component: { name: "el-input" },
			required: true,
			group: "base"
		},
		{
			label: "学生电话",
			prop: "mobile",
			span: 8,
			component: { name: "el-input" },
			required: true,
			group: "base"
		},
		{
			label: "接待人员",
			prop: "receiver",
			span: 8,
			component: { name: "el-input" },
			required: true,
			group: "base"
		},
		{
			label: "身份证",
			prop: "idcardNumber",
			span: 8,
			component: { name: "el-input" },
			required: true,
			group: "base"
		},
		{
			label: "性别",
			prop: "gender",
			span: 8,
			component: {
				name: "el-select",
				options: [
					{ label: "保密", value: "0" },
					{ label: "男", value: "1" },
					{ label: "女", value: "2" }
				]
			},
			required: true,
			group: "base"
		},
		{
			label: "意向院校",
			prop: "schoolId",
			span: 8,
			component: { name: "slot-schoolId" },
			group: "base"
		},
		{
			label: "意向专业",
			prop: "majorsId",
			span: 8,
			component: { name: "slot-majorsId" },
			group: "base"
		},
		{
			label: "报读类型",
			prop: "majorsType",
			span: 8,
			component: { name: "el-select", options: [] },
			required: true,
			group: "base"
		},
		{
			label: "报读层次",
			prop: "degreeId",
			span: 8,
			component: { name: "el-select", options: [] },
			required: true,
			group: "base"
		},
		{
			label: "通讯地址",
			prop: "address",
			span: 24,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "紧急联系人",
			prop: "emergencyContact",
			span: 12,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "紧急联系人电话",
			prop: "emergencyMobile",
			props: { labelWidth: "130px" },
			span: 12,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "微信",
			prop: "wechat",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "民族",
			prop: "nation",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "籍贯",
			prop: "nativePlace",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "政治面貌",
			prop: "politicsStatus",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "户口性质",
			prop: "householdType",
			span: 8,
			component: { name: "el-select" },
			group: "person"
		},
		{
			label: "户口所在地",
			prop: "householdAddress",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "是否应届",
			prop: "freshman",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "当前学历",
			prop: "education",
			span: 8,
			component: { name: "el-select" },
			group: "person"
		},
		{
			label: "毕业学校",
			prop: "graduatedSchool",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "毕业时间",
			prop: "graduatedDate",
			span: 8,
			component: { name: "el-input" },
			group: "person"
		},
		{
			label: "备注",
			prop: "remark",
			span: 24,
			component: {
				name: "el-input",
				props: { type: "textarea", rows: 4 }
			},
			group: "base"
		},
		{
			label: "校方收定位金",
			prop: "schoolPayment",
			span: 8,
			value: 0.0,
			component: {
				name: "el-input-number",
				props: { precision: 2, step: 0.1 }
			},
			group: "financial"
		},
		{
			label: "自收定位金",
			prop: "teamsPayment",
			span: 8,
			value: 0.0,
			component: {
				name: "el-input-number",
				props: { precision: 2, step: 0.1 }
			},
			group: "financial"
		},
		{
			label: "支付编号",
			prop: "serial",
			span: 8,
			component: { name: "el-input" },
			group: "financial"
		},
		{
			label: "缴费凭证",
			prop: "voucher",
			span: 8,
			component: { name: "cl-upload" },
			group: "financial"
		}
	],
	async onOpen() {
		getSchoolList();

		const majorsTypeList = await service.customer_pro.readtypes.list();
		Upsert.value?.setOptions(
			"majorsType",
			majorsTypeList.map((e: any) => ({ label: e.name, value: e.id }))
		);

		const degreeList = await service.customer_pro.readdegree.list();
		Upsert.value?.setOptions(
			"degreeId",
			degreeList.map((e: any) => ({ label: e.name, value: e.id }))
		);

		Upsert.value?.setOptions("householdType", [
			{ label: "城镇", value: "1" },
			{ label: "农村", value: "2" }
		]);

		Upsert.value?.setOptions("education", [
			{ label: "未知", value: "1" },
			{ label: "初中", value: "2" },
			{ label: "高中/中专/中技", value: "3" },
			{ label: "大专/高技", value: "4" },
			{ label: "本科", value: "5" }
		]);
	},
	onSubmit(data, { next }) {
		next({
			...data,
			cluesId: props.cluesId
		});
		// 新增/编辑成功后通知父组件刷新
		nextTick(() => {
			emit("success");
		});
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "学生名称", prop: "name" },
		{ label: "手机号码", prop: "mobile" },
		{ label: "意向专业", prop: "majorsName" },
		{ label: "报读类型", prop: "majorsTypeName" },
		{ label: "报读层次", prop: "degreeName" },
		{ label: "项目", prop: "projectName" },
		{ label: "接待人员", prop: "receiver" },
		{ label: "审核状态", prop: "auditStatus" },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", width: 120, buttons: [
			{ label: "编辑", type: "primary", text: true, size: "small", onClick: ({ scope }) => { Crud.value?.rowEdit(scope.row); } },
			{ label: "删除", type: "danger", text: true, size: "small", onClick: ({ scope }) => { Crud.value?.rowDelete(scope.row); } }
		] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.order,
		async onRefresh(params, { next, render }) {
			// 注入线索ID筛选
			params.clues_id = props.cluesId;
			const { list, pagination } = await next(params);
			render(list, pagination);
		}
	},
	(app) => {
		// 不在这里自动 refresh，等 cluesId 就绪后手动触发
	}
);

// 监听 cluesId 变化，有值时刷新
watch(
	() => props.cluesId,
	(val) => {
		if (val) {
			nextTick(() => {
				Crud.value?.refresh();
			});
		}
	},
	{ immediate: true }
);

// 新增订单行
const addRow = () => {
	Crud.value?.rowAdd();
};

// 暴露方法
const refresh = () => {
	nextTick(() => {
		Crud.value?.refresh();
	});
};

defineExpose({ refresh, addRow });
</script>

<style lang="scss" scoped>
:deep(.cl-crud) {
	height: 100%;
}
</style>
