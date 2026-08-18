<template>
	<el-dialog
		v-model="visible"
		title="跟进记录详情"
		width="70%"
		:close-on-click-modal="false"
		destroy-on-close
		class="follow-detail-dialog"
	>
		<div class="detail-toolbar">
			<el-button type="primary" size="small" @click="handleEdit">
				<el-icon><EditPen /></el-icon> 修改
			</el-button>
			<el-button size="small" @click="handleExport">
				<el-icon><Download /></el-icon> 导出
			</el-button>
			<el-button size="small" @click="handleCall">
				<el-icon><Phone /></el-icon> 拨打电话
			</el-button>
		</div>
		<div class="detail-content" v-if="data">
			<div class="detail-row">
				<label>跟进人：</label>
				<span>{{ data.userName || '' }}</span>
			</div>
			<div v-if="data.callId && (data.deviceModel || data.simSlotLabel)" class="detail-row">
				<label>来自：</label>
				<span>{{ data.deviceModel || "手机" }} 直拨<span v-if="data.simSlotLabel"> · {{ data.simSlotLabel }}</span><span v-if="data.simNumberMasked"> · {{ data.simNumberMasked }}</span><span v-if="data.simCarrierName"> · {{ data.simCarrierName }}</span></span>
			</div>
			<div class="detail-row">
				<label>跟进方式：</label>
				<span>{{ data.followTypeName || '' }}</span>
			</div>
			<div class="detail-row">
				<label>跟进时间：</label>
				<span>{{ data.createTime || '' }}</span>
			</div>
			<div class="detail-row">
				<label>下次跟进时间：</label>
				<span>{{ data.nextFollowupTime || '未设置' }}</span>
			</div>
			<div class="detail-section">
				<label>跟进内容：</label>
				<div class="detail-remark" v-html="data.remark || ''"></div>
			</div>
		</div>
	</el-dialog>
</template>

<script lang="ts" name="clues-follow-detail" setup>
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { EditPen, Download, Phone } from "@element-plus/icons-vue";

const emit = defineEmits<{
	(e: "edit", data: any): void;
}>();

const visible = ref(false);
const data = ref<any>(null);

function open(item: any) {
	data.value = item;
	visible.value = true;
}

function handleEdit() {
	visible.value = false;
	emit("edit", data.value);
}

function handleExport() {
	ElMessage.info("功能正在开发");
}

function handleCall() {
	ElMessage.info("功能正在开发");
}

defineExpose({ open });
</script>

<style lang="scss" scoped>
.follow-detail-dialog {
	.detail-toolbar {
		display: flex;
		gap: 8px;
		margin-bottom: 20px;
		padding-bottom: 16px;
		border-bottom: 1px solid #f0f1f5;
	}

	.detail-content {
		.detail-row {
			display: flex;
			align-items: baseline;
			margin-bottom: 12px;
			font-size: 14px;

			label {
				color: #909399;
				width: 110px;
				flex-shrink: 0;
				text-align: right;
				margin-right: 12px;
			}

			span {
				color: #303133;
			}
		}

		.detail-section {
			margin-top: 16px;

			label {
				display: block;
				color: #909399;
				font-size: 14px;
				margin-bottom: 8px;
			}

			.detail-remark {
				background: #fafbfc;
				border: 1px solid #f0f1f5;
				border-radius: 8px;
				padding: 16px;
				font-size: 14px;
				color: #303133;
				line-height: 1.8;
				word-break: break-word;
				min-height: 80px;

				:deep(img) {
					max-width: 100%;
					height: auto;
				}

				:deep(p) {
					margin: 0;
				}
			}
		}
	}
}
</style>
