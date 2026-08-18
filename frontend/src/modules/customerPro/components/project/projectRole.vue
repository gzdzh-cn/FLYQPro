<template>
	<el-form ref="formRef" :inline="true" label-width="auto">
		<div
			style="width: 100%; display: inline-flex; flex-direction: row; align-items: self-start"
		>
			<el-form-item label="项目主管" prop="id" style="width: 90%">
				<el-select-v2
					v-model="projectRoleIds"
					remote
					:remote-method="remoteMethod"
					clearable
					:options="options"
					:loading="loading"
					placeholder="请输入项目主管"
					multiple
					:multipleLimit="multipleLimit"
					filterable
					style="width: 100%"
				/>
			</el-form-item>
			<el-button type="success" :loading="loading" @click="addProjectRole">添加</el-button>
		</div>
	</el-form>

	<div style="min-height: 110px; height: 400px">
		<cl-crud ref="Crud">
			<cl-row>
				<!-- 数据表格 -->
				<cl-table ref="Table">
					<template #column-status="{ scope }">
						<div
							style="
								display: flex;
								flex-direction: row;
								justify-content: center;
								align-items: center;
								gap: 10px;
							"
						>
							<el-switch
								v-model="scope.row.status"
								:active-value="1"
								:inactive-value="0"
								active-text="常规接收"
								inactive-text="常规禁止"
								inline-prompt
								style="width: 80px"
								@change="changeStatus(scope.row, 'status')"
							/>

							<el-switch
								v-model="scope.row.areaStatus"
								:active-value="1"
								:inactive-value="0"
								active-text="地区接收"
								inactive-text="地区禁止"
								inline-prompt
								style="width: 80px"
								@change="changeStatus(scope.row, 'areaStatus')"
							/>
						</div>
					</template>
				</cl-table>
			</cl-row>
		</cl-crud>
	</div>
</template>

<script lang="ts" name="projectRole" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { onMounted, PropType, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

interface ListItem {
	value: string;
	label: string;
}

interface ProjectRoleItem {
	id: string;
}

const props = defineProps({
	projectRoleItem: {
		type: Object as PropType<ProjectRoleItem>,
		default: () => {}
	}
});

const { service } = useCool();
const multipleLimit = ref(1);
const loading = ref(false);
const projectRoleIds = ref([]);
const options = ref<ListItem[]>([]);
const remoteMethod = async (keyWord?: string) => {
	if (keyWord !== "") {
		loading.value = true;
		const projectRoleList = await service.customer_pro.kf.getList({ keyWord });
		loading.value = false;
		options.value = projectRoleList.map((e: { name: any; id: any }) => {
			return {
				label: e.name,
				value: e.id
			};
		});
	}
};

// cl-table 配置
const Table = useTable({
	columns: [
		{ label: "姓名", prop: "name", width: 250 },
		{
			label: "接收推送",
			prop: "status",
			component: {
				name: "cl-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "常规接收",
					inactiveText: "关闭",
					inlinePrompt: true
				}
			}
		},
		{ type: "op", buttons: ["delete"] }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.customer_pro.kf,
	async onRefresh(params, { render }) {
		const list: any = await service.customer_pro.kf.list({
			role: 3,
			projectId: props.projectRoleItem.id
		});
		remoteMethod();
		// 渲染数据
		render(list);
		isTableInited.value = true; // 数据加载完后设置为 true
	},
	onDelete(selection) {
		ElMessageBox.confirm(`此操作将永久删除选中数据，是否继续？`, "提示", {
			type: "error"
		})
			.then(() => {
				// 继续执行删除请求
				service.customer_pro.project
					.delProjectRole({
						projectId: props.projectRoleItem.id,
						userIds: selection.map((e) => e.userId)
					})
					.then(() => {
						refresh();
						ElMessage.success("删除成功");
					})
					.catch((e) => {
						ElMessage.error(e.message);
					});

				return;
			})
			.catch((e) => null);
	}
});

const refresh = () => {
	Crud.value?.refresh();
};

// 添加项目主管
const addProjectRole = async () => {
	if (projectRoleIds.value.length == 0) {
		ElMessage.error("请选择项目主管");
		return;
	}

	loading.value = true;
	await service.customer_pro.project.addProjectRole({
		projectId: props.projectRoleItem.id,
		userIds: projectRoleIds.value
	});
	loading.value = false;
	refresh();
	projectRoleIds.value = [];
	ElMessage.success("添加成功");
};

interface Setting {
	projectMultiManger: boolean;
}

// 获取设置
const setting = ref<Setting>();
const getSetting = async () => {
	setting.value = await service.customer_pro.basic.info({ id: 1 });
	if (setting.value?.projectMultiManger) {
		multipleLimit.value = 0;
	}
};

// 标志位
const isTableInited = ref(false);
// 改变接收状态
const changeStatus = (row: any, type: string) => {
	if (!isTableInited.value) return; // 初始化未完成时不执行
	let status;
	if (type == "status") {
		status = row.status;
	}
	if (type == "areaStatus") {
		status = row.areaStatus;
	}
	service.customer_pro.kf
		.update({
			id: row.id,
			userId: row.userId,
			[type]: status
		})
		.then(() => {
			ElMessage.success("设置成功");
			isTableInited.value = false;
			Crud.value?.refresh();
		});
};

onMounted(() => {
	getSetting();
	refresh();
});
</script>
