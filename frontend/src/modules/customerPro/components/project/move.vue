<template>
	<div class="dept-move">
		<cl-form ref="Form" />
	</div>
</template>

<script lang="ts" name="project-move" setup>
import { useCool } from "/@/cool";
import { ElMessage, ElMessageBox, MessageParamsWithType } from "element-plus";
import { useCrud, useForm } from "@cool-vue/crud";
import DeptSelect from "./select.vue";

const { service } = useCool();
const Form = useForm();
const Crud = useCrud();

async function open(ids: any[]) {
	Form.value?.open({
		title: "组别转移",
		width: "500px",
		props: {
			labelWidth: "80px"
		},
		items: [
			{
				label: "选择项目",
				prop: "projectId",
				component: {
					vm: DeptSelect
				}
			}
		],
		on: {
			submit(data, { done, close }) {
				if (!data.projectId) {
					ElMessage.warning("请选择项目");
					return done();
				}

				ElMessageBox.confirm("转移到新项目，组内客服列表也会一起迁移，是否继续？", "提示", {
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

defineExpose({
	open
});
</script>
