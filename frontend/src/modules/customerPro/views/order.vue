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
			<cl-table ref="Table" :border="false">
				<template #column-detail="{ scope }">
					<div style="padding: 0 30px">
						<p v-if="scope.row?.userName">创建者: {{ scope.row?.userName }}</p>
						<p>接待人员: {{ scope.row?.receiver }}</p>
						<p>学方收定位金: {{ scope.row?.schoolPayment }}</p>
						<p>自收定位金: {{ scope.row?.teamsPayment }}</p>
						<p>支付编号: {{ scope.row?.serial }}</p>
						<p>备注: {{ scope.row?.remark }}</p>
					</div>
				</template>

				<template #column-auditStatus="{ scope }">
					<span style="color: #d83b01" v-if="scope.row.auditStatus == 1"> 待审核 </span>
					<span style="color: #00b294" v-if="scope.row.auditStatus == 2">
						审核通过
					</span>

					<span style="color: #ff8c00" v-if="scope.row.auditStatus == 3">
						审核驳回
					</span>
				</template>

				<template #slot-op="{ scope }">
					<el-button
						text
						bg
						type="primary"
						@click="edit(scope.row)"
						v-if="
							scope.row.auditStatus == 1 &&
							service.customer_pro.order._permission.update
						"
						>编辑</el-button
					>
					<el-button
						text
						bg
						type="primary"
						@click="edit(scope.row)"
						v-if="
							scope.row.auditStatus > 1 &&
							service.customer_pro.order._permission.info
						"
						>查看</el-button
					>
					<el-button
						text
						bg
						type="warning"
						@click="openTracks(scope.row)"
						v-if="service.customer_pro.clues._permission.getTrackList"
						>轨迹</el-button
					>

					<el-popover
						placement="right"
						:width="80"
						trigger="click"
						popper-class="dzh_popover"
						:visible="popoverVisible[scope.row.id]"
						:persistent="true"
						@show="showEvent"
						@hide="hideEvent"
					>
						<div class="more">
							<div class="more_btn">
								<el-popconfirm
									title="确定审核通过吗?"
									@confirm="verify(scope.row, 2)"
								>
									<template #reference>
										<el-button text bg type="warning">通过</el-button>
									</template>
								</el-popconfirm>
							</div>
							<div>
								<el-button text bg type="danger" @click="verify(scope.row, 3)"
									>驳回</el-button
								>
							</div>
						</div>

						<template #reference>
							<el-button
								text
								bg
								type="info"
								@click="doPopoverVisible(scope.row, scope.column)"
								:disabled="scope.row.auditStatus > 1"
								v-permission="service.customer_pro.order.permission.verify"
								>审核</el-button
							>
						</template>
					</el-popover>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert">
			<template #slot-schoolId="{ scope }">
				<!-- 学校 -->
				<el-select v-model="scope.schoolId" @change="schoolChange">
					<el-option
						v-for="item in schoolList"
						:key="item.value"
						:label="item.name"
						:value="item.id"
					/>
				</el-select>
			</template>

			<!-- 专业 -->
			<template #slot-majorsId="{ scope }">
				<el-select v-model="scope.majorsId">
					<el-option
						v-for="item in majorsList"
						:key="item.value"
						:label="item.name"
						:value="item.id"
					/>
				</el-select>
			</template>
		</cl-upsert>

		<!-- 轨迹弹窗 -->
		<cl-dialog title="轨迹" v-model="trackVisible">
			<sub-track ref="TrackRef" :id="cluesId" />
			<template #footer>
				<div class="dialog-footer">
					<el-button @click="trackVisible = false">关闭</el-button>
				</div>
			</template>
		</cl-dialog>

		<cl-form ref="RejectRef" />
	</cl-crud>
</template>

<script lang="ts" name="customer_pro-order" setup>
import { useCrud, useTable, useUpsert, useForm } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ref } from "vue";
import SubTrack from "../components/clues/subTrack.vue";
import { ElMessage } from "element-plus";

const { service } = useCool();
const RejectRef = useForm(); //审核驳回表单
const cluesId = ref(); //线索id
const popoverVisible = ref<{ [key: number]: boolean }>({}); //审核按钮弹出
const ids = ref();

// tip空白关闭
const showEvent = () => {
	document.addEventListener("click", hidePanel, false);
};
const hideEvent = () => {
	document.removeEventListener("click", hidePanel, false);
};
const hidePanel = (e: any) => {
	cancelPopver();
};

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
						label: "个人信息",
						value: "person"
					},
					{
						label: "收款信息",
						value: "financial"
					}
				]
			}
		},
		{
			label: "学生名称",
			prop: "name",
			span: 8,
			component: {
				name: "el-input"
			},
			required: true,
			group: "base"
		},
		{
			label: "学生电话",
			prop: "mobile",
			span: 8,
			component: {
				name: "el-input"
			},
			required: true,
			group: "base"
		},
		{
			label: "接待人员",
			prop: "receiver",
			span: 8,
			component: {
				name: "el-input"
			},
			required: true,
			group: "base"
		},

		{
			label: "身份证",
			prop: "idcardNumber",
			span: 8,
			component: {
				name: "el-input"
			},
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
					{
						label: "选择",
						value: "0"
					},
					{
						label: "男",
						value: "1"
					},
					{
						label: "女",
						value: "2"
					}
				]
			},
			required: true,
			group: "base"
		},
		{
			label: "意向院校",
			prop: "schoolId",
			span: 8,
			component: {
				name: "slot-schoolId",
				options: []
			},
			required: true,
			group: "base"
		},

		{
			label: "意向专业",
			prop: "majorsId",
			span: 8,
			component: {
				name: "slot-majorsId",
				options: []
			},
			required: true,
			group: "base"
		},
		{
			label: "报读类型",
			prop: "majorsType",
			span: 8,
			component: {
				name: "el-select",
				options: []
			},
			required: true,
			group: "base"
		},
		{
			label: "报读层次",
			prop: "degreeId",
			span: 8,
			component: {
				name: "el-select",
				options: []
			},
			required: true,
			group: "base"
		},

		{
			label: "通讯地址",
			prop: "address",
			span: 24,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "紧急联系人",
			prop: "emergencyContact",
			span: 12,
			component: {
				name: "el-input"
			},
			group: "person"
		},

		{
			label: "紧急联系人电话",
			prop: "emergencyMobile",
			props: {
				labelWidth: "130px"
			},
			span: 12,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "微信",
			prop: "wechat",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "民族",
			prop: "nation",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "籍贯",
			prop: "nativePlace",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},

		{
			label: "政治面貌",
			prop: "politicsStatus",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "户口性质",
			prop: "householdType",
			span: 8,
			component: {
				name: "el-select"
			},
			group: "person"
		},
		{
			label: "户口所在地",
			prop: "householdAddress",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},

		{
			label: "是否应届",
			prop: "freshman",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "当前学历",
			prop: "education",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "毕业学校",
			prop: "graduatedSchool",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},

		{
			label: "毕业时间",
			prop: "graduatedDate",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "person"
		},
		{
			label: "备注",
			prop: "remark",
			span: 24,
			component: {
				name: "el-input",
				props: {
					type: "textarea",
					rows: 4
				}
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
				props: {
					precision: 2,
					step: 0.1
				}
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
				props: {
					precision: 2,
					step: 0.1
				}
			},
			group: "financial"
		},
		{
			label: "支付编号",
			prop: "serial",
			span: 8,
			component: {
				name: "el-input"
			},
			group: "financial"
		},
		{
			label: "缴费凭证",
			prop: "voucher",
			span: 8,
			component: {
				name: "cl-upload"
			},
			group: "financial"
		}
	],
	async onOpen() {
		// 学校列表
		getSchoolList();

		// 报读类型
		const majorsTypeList = await service.customer_pro.readtypes.list();
		Upsert.value?.setOptions(
			"majorsType",
			majorsTypeList.map((e) => {
				return {
					label: e.name,
					value: e.id
				};
			})
		);

		// 报读层次
		const degreeList = await service.customer_pro.readdegree.list();
		Upsert.value?.setOptions(
			"degreeId",
			degreeList.map((e) => {
				return {
					label: e.name,
					value: e.id
				};
			})
		);

		// 户口性质
		Upsert.value?.setOptions("householdType", [
			{
				label: "城镇",
				value: "1"
			},
			{
				label: "农村",
				value: "2"
			}
		]);
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		() => {
			return {
				label: "#",
				type: "expand",
				prop: "detail"
			};
		},
		{ label: "学生名称", prop: "name" },
		{ label: "手机号码", prop: "mobile" },
		{ label: "微信号", prop: "wechat" },

		{ label: "专业", prop: "majorsName" },
		{ label: "专业类型", prop: "majorsTypeName" },
		{ label: "报读层次", prop: "degreeName" },
		{ label: "项目", prop: "projectName" },
		{ label: "客服", prop: "servicesName" },
		{
			label: "审核状态",
			prop: "auditStatus"
		},

		{ label: "创建时间", prop: "createTime" },
		{ type: "op", width: 300, buttons: ["slot-op"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.customer_pro.order,
		async onRefresh(params, { next, render }) {
			// 1 默认调用
			const { list, pagination } = await next(params);
			ids.value = list.map((e) => e.id);

			// 渲染数据
			render(list, pagination);
		}
	},
	(app) => {
		app.refresh();
	}
);

// 编辑
const edit = (row: any) => {
	if (row.auditStatus == 1) {
		Crud.value?.rowEdit(row);
	} else {
		Crud.value?.rowInfo(row);
	}
};

const refresh = () => {
	Crud.value?.refresh();
};

// 轨迹弹窗
const trackVisible = ref(false);
const openTracks = (row: any) => {
	cluesId.value = row.cluesId;
	trackVisible.value = true;
};

// 关闭审核按钮
const cancelPopver = (v?: number) => {
	for (let index = 0; index < ids.value.length; index++) {
		const id = ids.value[index];
		if (id != v) {
			if (popoverVisible.value[id]) {
				popoverVisible.value[id] = false;
			}
		}
	}
	hideEvent();
};

// 审核按钮弹出
const doPopoverVisible = (row: any, column: any) => {
	cancelPopver(row.id);
	popoverVisible.value[row.id] = !popoverVisible.value[row.id];
};

// 审查
const verify = (row: any, type: number) => {
	popoverVisible.value[row.id] = true;
	let action = true;
	if (type == 3) {
		action = false;
	}

	if (action) {
		orderUpdate({ id: row.id, auditStatus: type, cluesId: row.cluesId });
	} else {
		RejectRef.value?.open({
			title: "审核失败意见",
			items: [
				{
					label: "审核意见",
					prop: "auditNote",
					component: {
						name: "el-input",
						props: {
							type: "textarea",
							rows: 4
						}
					}
				}
			],
			on: {
				submit(data, { close }) {
					orderUpdate({
						id: row.id,
						auditStatus: type,
						auditNote: data.auditNote,
						cluesId: row.cluesId
					});
					close();
				}
			}
		});
	}
	cancelPopver(row.id);
};

// 更新订单
const orderUpdate = (form: any) => {
	service.customer_pro.order
		.verify({
			...form
		})
		.then((r) => {
			ElMessage.success("操作成功");
			refresh();
		})
		.catch((e) => {
			ElMessage.error(e.message);
		})
		.finally(() => {
			cancelPopver();
		});
};

// 学校列表
const schoolList = ref();
const majorsList = ref();

const getSchoolList = async () => {
	schoolList.value = await service.customer_pro.school.list();
	if (Upsert.value?.mode != "add") {
		getMajorList(schoolList.value[0].id);
	}
};

// 学校改变
const schoolChange = async (v: any) => {
	Upsert.value?.setForm("majorsId", null);
	getMajorList(v);
};

// 专业列表
const getMajorList = async (v: any) => {
	majorsList.value = await service.customer_pro.majors.list({ schoolId: v });
};
</script>

<style lang="scss">
.more_btn {
	margin-bottom: 5px;
}
</style>
