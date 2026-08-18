<template>
	<el-dialog
		v-model="visible"
		:title="isEdit ? '修改跟进记录' : '添加跟进记录'"
		width="70%"
		:close-on-click-modal="false"
		destroy-on-close
		@closed="onClosed"
	>
		<el-form :model="form" label-width="110px" label-position="right">
			<el-form-item label-width="0">
				<cl-editor-wang v-model="form.remark" :height="350" />
			</el-form-item>
			<div style="display: flex; gap: 16px;">
				<el-form-item label="下次跟进时间" required style="flex: 1;">
					<el-date-picker
						v-model="form.nextFollowupTime"
						type="datetime"
						:default-time="defaultTime"
						value-format="YYYY-MM-DD HH:mm"
						time-format="HH:mm"
						:disabled-date="disabledDate"
						placeholder="选择下次跟进时间"
						style="width: 100%"
					/>
				</el-form-item>
					<el-form-item label="跟进方式" style="flex: 1;">
						<el-cascader
							v-model="form.followType"
							:options="followOptions"
							:props="followProps"
							clearable
							placeholder="选择跟进方式"
							style="width: 100%"
						/>
				</el-form-item>
			</div>
		</el-form>
		<template #footer>
			<el-button @click="visible = false">取消</el-button>
			<el-button type="primary" :loading="saving" @click="handleSubmit">确定</el-button>
		</template>
	</el-dialog>
</template>

<script lang="ts" name="clues-follow-dialog" setup>
import { ref, reactive } from "vue";
import { useCool } from "/@/cool";
import { ElMessage } from "element-plus";
import { followOptions, followProps, normalizeFollowType } from "./followType";

const props = defineProps<{
	cluesId?: string | number;
}>();

const emit = defineEmits<{
	(e: "saved"): void;
}>();

const { service } = useCool();

const visible = ref(false);
const saving = ref(false);
const isEdit = ref(false);
const editId = ref<string | number | null>(null);

const defaultTime = new Date();

function disabledDate(time: { getTime: () => number }) {
	return time.getTime() < Date.now() - 8.64e7;
}

const form = reactive({
	remark: "",
	nextFollowupTime: "",
	followType: [] as string[]
});

function open(data?: any) {
	form.remark = "";
	form.nextFollowupTime = "";
	form.followType = [];
	isEdit.value = false;
	editId.value = null;

	if (data) {
		isEdit.value = true;
		editId.value = data.id || null;
		form.remark = data.remark || "";
		form.nextFollowupTime = data.nextFollowupTime || "";
		// 兼容接口的驼峰、下划线及数组字段，确保编辑时能回填下拉选项。
		const followType = data.followupType || data.followup_type || data.followType || data.follow_type || data.followTypeName || "";
		form.followType = normalizeFollowType(followType);
	}

	visible.value = true;
}

function onClosed() {
	form.remark = "";
	form.nextFollowupTime = "";
	form.followType = [];
	isEdit.value = false;
	editId.value = null;
}

async function handleSubmit() {
	const followTypeArr = form.followType;

	if (!form.nextFollowupTime) {
		ElMessage.error("请选择下次跟进时间");
		return;
	}
	if (!form.remark || form.remark.trim() === "" || form.remark === "<p><br></p>") {
		ElMessage.error("请填写跟进内容");
		return;
	}

	saving.value = true;
	try {
		if (isEdit.value) {
			// 编辑模式：调用 followUpdate
			const submitData: any = {
				id: editId.value,
				cluesId: props.cluesId,
				remark: form.remark
			};
			if (form.nextFollowupTime) {
				submitData.nextFollowupTime = form.nextFollowupTime;
			}
			if (followTypeArr.length > 0) {
				submitData.followType = followTypeArr;
			}
			await (service.customer_pro.clues as any).followUpdate(submitData);
			ElMessage.success("跟进记录修改成功");
		} else {
			// 新增模式：调用 followAdd
			const submitData: any = {
				cluesId: props.cluesId,
				remark: form.remark
			};
			if (form.nextFollowupTime) {
				submitData.nextFollowupTime = form.nextFollowupTime;
			}
			if (followTypeArr.length > 0) {
				submitData.followType = followTypeArr;
			}
			await (service.customer_pro.clues as any).followAdd(submitData);
			ElMessage.success("跟进记录添加成功");
		}

		visible.value = false;
		emit("saved");
	} catch (e: any) {
		ElMessage.error("保存失败：" + (e.message || ""));
	} finally {
		saving.value = false;
	}
}

defineExpose({ open });
</script>
