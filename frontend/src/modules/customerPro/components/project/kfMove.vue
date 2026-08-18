<template>
	<div class="dept-move">
		<cl-form ref="Form">
			<template #slot-projectId="{ scope }">
				<el-select v-model="scope.projectId" @change="projectChange">
					<el-option
						v-for="item in projectList"
						:key="item.value"
						:label="item.name"
						:value="item.id"
					/>
				</el-select>
			</template>

			<template #slot-groupId="{ scope }">
				<el-select v-model="scope.groupId" @change="groupChange">
					<el-option
						v-for="item in groupList"
						:key="item.value"
						:label="item.name"
						:value="item.id"
					/>
				</el-select>
			</template>
		</cl-form>
	</div>
</template>

<script lang="ts" name="kf-move" setup>
import { useCool } from "/@/cool";
import { ElMessage, ElMessageBox, MessageParamsWithType } from "element-plus";
import { useCrud, useForm } from "@cool-vue/crud";
import { ref } from "vue";

const { service } = useCool();
const Form = useForm();
const Crud = useCrud();

async function open(ids: any[]) {
	groupList.value = [];
	Form.value?.setForm("groupId", null);
	projectList.value = await service.customer_pro.project.list();

	Form.value?.open({
		title: "组别转移",
		width: "500px",
		props: {
			labelWidth: "80px"
		},
		items: [
			{
				label: "项目",
				prop: "projectId",
				component: {
					name: "slot-projectId"
				},
				required: true
			},
			{
				label: "客服组",
				prop: "groupId",
				component: {
					name: "slot-groupId"
				},
				required: true
			}
		],
		on: {
			submit(data, { done, close }) {
				if (!data.projectId) {
					ElMessage.warning("请选择项目");
					return done();
				}

				ElMessageBox.confirm("转移到新项目，是否继续？", "提示", {
					type: "warning"
				})
					.then(() => {
						service.customer_pro.project_group
							.move({
								...data,
								ids: ids
							})
							.then(() => {
								ElMessage.success("转移成功");
								Crud.value?.refresh();
								close();
							})
							.catch((err: { message: MessageParamsWithType }) => {
								ElMessage.error(err.message);
								done();
							});
					})
					.catch(() => {
						done();
					});
			}
		}
	});
}

const projectList = ref(); // 项目列表
// 项目id
const projectId = ref();
// 项目id改变
const projectChange = (v: any) => {
	groupList.value = [];
	Form.value?.setForm("groupId", null);
	Form.value?.setForm("servicesId", null);
	projectId.value = v;
	getGroupList(v);
};

// 组别id改变
const groupChange = (v: any) => {
	Form.value?.setForm("servicesId", null);
};

// 客服组列表
const groupList = ref();
const getGroupList = async (projectId: string) => {
	groupList.value = await service.customer_pro.project_group.list({ projectId });
};

defineExpose({
	open
});
</script>
