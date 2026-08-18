<template>
	<div v-if="minimized" class="remote-call-minimized" @click="restore">
		<el-icon><Phone /></el-icon>
		<span>{{ clue?.keywords || clue?.name || "远程通话" }}</span>
		<span class="minimized-state">{{ stateText }}</span>
	</div>

	<el-dialog
		v-model="visible"
		width="900px"
		class="remote-call-dialog"
		:close-on-click-modal="false"
		:close-on-press-escape="false"
		:show-close="false"
		append-to-body
		@closed="onClosed"
	>
		<template #header>
			<div class="remote-call-titlebar">
				<div class="remote-call-device">
					<span>{{ deviceState.deviceModel || "手机客户端" }}</span>
					<span v-if="deviceState.battery >= 0" class="battery">电量 {{ deviceState.battery }}%</span>
					<span class="mobile-connection" :class="{ online: mobileConnected }">
						<i />{{ mobileConnected ? "移动端已连接" : "移动端未连接" }}
					</span>
				</div>
				<div class="remote-call-window-actions">
					<el-button text circle title="最小化" @click="minimize"><el-icon><Minus /></el-icon></el-button>
					<el-button text circle title="关闭" @click="close"><el-icon><Close /></el-icon></el-button>
				</div>
			</div>
		</template>

		<div class="remote-call-shell">
			<!-- 上部：线索信息与拨号状态，占整体约 1/5 -->
			<div class="remote-call-summary">
				<div class="remote-call-clue">
					<el-avatar :size="44" :src="clue?.headImg || clue?.avatar || '/customerPro/usreicon_80.png'" shape="square" />
					<div class="remote-call-clue-text">
						<div class="keyword">{{ clue?.keywords || clue?.name || "未命名线索" }}</div>
						<div class="mobile">{{ clue?.mobile || "暂无手机号" }}</div>
						<div v-if="selectedSim" class="selected-sim">拨出线路：{{ selectedSim.slotLabel || `卡${selectedSim.slotIndex + 1}` }} · {{ selectedSim.numberMasked || "号码未识别" }} · {{ selectedSim.carrierName || "运营商未识别" }}</div>
					</div>
				</div>
				<div class="remote-call-status">
					<div class="status-line" :class="{ connected: status === 'connected', ended: ended }">
						<el-icon><PhoneFilled /></el-icon>
						<span>{{ stateText }}</span>
					</div>
					<el-button class="hangup-button" type="danger" circle :loading="hangupLoading" :disabled="ended" title="挂机" @click="hangup">
						<el-icon><Phone /></el-icon>
					</el-button>
				</div>
			</div>

			<!-- 下部：左侧跟进记录，右侧填写跟进，占整体约 4/5 -->
			<div class="remote-call-body">
				<div class="remote-follow-history">
					<div class="section-title">全部跟进记录</div>
					<div v-if="loadingHistory" class="history-empty">加载中...</div>
					<div v-else-if="!followList.length" class="history-empty">暂无跟进记录</div>
					<div v-else class="history-scroll">
						<div v-for="item in followList" :key="item.id" class="history-item">
							<div class="history-meta">
								<span>{{ item.userName || item.nickName || "未命名跟进人" }}</span>
								<span>{{ item.createTime || "" }}</span>
							</div>
							<div v-if="item.callId && (item.deviceModel || item.simSlotLabel)" class="history-source">
								来自：{{ item.deviceModel || "手机" }} 直拨<span v-if="item.simSlotLabel"> · {{ item.simSlotLabel }}</span><span v-if="item.simNumberMasked"> · {{ item.simNumberMasked }}</span><span v-if="item.simCarrierName"> · {{ item.simCarrierName }}</span>
							</div>
							<div v-if="item.followTypeName || item.followupType" class="history-type">
								{{ item.followTypeName || formatFollowType(item.followupType) }}
							</div>
							<div class="history-flags">
								<span :class="['history-flag', isConnectedRecord(item) ? 'is-connected' : 'is-missed']">
									{{ isConnectedRecord(item) ? "已接通" : "未接通" }}
								</span>
								<span :class="['history-flag', hasRecording(item) ? 'has-recording' : 'no-recording']">
									{{ hasRecording(item) ? "有录音" : "无录音" }}
								</span>
							</div>
							<div class="history-remark" v-html="item.remark || '无备注'" />
						</div>
					</div>
				</div>

				<div class="remote-follow-form">
					<div class="section-title">填写跟进记录</div>
					<el-form label-position="top" class="follow-form">
						<el-form-item label="跟进内容" required>
							<cl-editor-wang :key="editorKey" ref="editorRef" v-model="form.remark" :height="150" />
						</el-form-item>
						<div class="form-row">
							<el-form-item label="跟进方式" class="form-item-half">
								<el-cascader v-model="form.followType" :options="followOptions" :props="followProps" clearable placeholder="选择跟进方式" style="width: 100%" />
							</el-form-item>
							<el-form-item label="下次跟进时间" required class="form-item-half">
								<el-date-picker v-model="form.nextFollowupTime" type="datetime" value-format="YYYY-MM-DD HH:mm" time-format="HH:mm" placeholder="选择跟进时间" style="width: 100%" />
							</el-form-item>
						</div>
						<div class="submit-row">
							<el-button v-if="!ended" class="call-in-progress-button" disabled>电话中</el-button>
							<el-button v-else type="primary" :loading="saving" @click="submitFollow">提交</el-button>
						</div>
					</el-form>
				</div>
			</div>
		</div>
	</el-dialog>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { Close, Minus, Phone, PhoneFilled } from "@element-plus/icons-vue";
import { useCool } from "/@/cool";
import { deviceState, onDeviceEvent, sendCloseFollow, sendDial, sendHangup, type SimCardInfo } from "/@/utils/deviceWs";
import { followOptions, followProps, formatFollowType } from "./followType";

const { service } = useCool();
const visible = ref(false);
const minimized = ref(false);
const clue = ref<any>(null);
const callId = ref("");
const selectedSim = ref<SimCardInfo | null>(null);
const status = ref("dialing");
const hangupLoading = ref(false);
const saving = ref(false);
const loadingHistory = ref(false);
const followList = ref<any[]>([]);
const form = reactive({ remark: "", followType: [] as string[], nextFollowupTime: "" });
const editorRef = ref<any>();
const editorKey = ref(0);
let hangupTimer: ReturnType<typeof setTimeout> | null = null;
const emit = defineEmits<{
	(e: "follow-saved", cluesId: string | number): void;
}>();

const ended = computed(() => status.value === "ended");
const mobileConnected = computed(
	() => deviceState.connStatus === "connected" && deviceState.androidOnline && deviceState.canRemoteCall
);
const stateText = computed(() => {
	if (status.value === "ringing") return "响铃中";
	if (status.value === "connected") return "通话中";
	if (status.value === "hangup_pending") return "等待手机响应";
	if (status.value === "ending") return "正在挂机";
	if (status.value === "ended") return "已挂机";
	return "正在拨号";
});

const offDialResult = onDeviceEvent("dial_result", (msg: any) => {
	// 弹窗最小化后仍要处理回执，否则移动端已经挂机，恢复弹窗时仍会一直显示“正在挂机”。
	if (!callId.value || msg?.callId !== callId.value) return;
	if (msg.status === "follow_submitted") {
		if (clue.value?.id) emit("follow-saved", clue.value.id);
		clearHangupTimer();
		hangupLoading.value = false;
		visible.value = false;
		minimized.value = false;
		ElMessage.success(msg.content || "手机端跟进已提交");
		return;
	}
	if (msg.status === "follow_closed") {
		// 未接通且用户直接关闭填写页时，后端仍会异步生成一条系统通话记录。
		// 通知线索详情启动分阶段刷新，不能只关闭 PC 弹窗。
		if (clue.value?.id) emit("follow-saved", clue.value.id);
		clearHangupTimer();
		hangupLoading.value = false;
		visible.value = false;
		minimized.value = false;
		return;
	}
	if (msg.status === "failed" || msg.status === "offline" || msg.status === "sim_unavailable") {
		ElMessage.error(msg.content || "手机拨号失败");
		clearHangupTimer();
		hangupLoading.value = false;
		// 挂机失败时保持通话状态，避免 PC 端误显示为可提交的“已挂机”。
		if (["ending", "hangup_pending"].includes(status.value)) status.value = "connected";
		if (msg.status === "sim_unavailable") status.value = "ended";
		return;
	}
	if (["dialing", "ringing", "connected", "hangup_pending", "hangup_requested", "ended"].includes(msg.status)) {
		status.value = msg.status;
		if (msg.status === "hangup_pending") {
			status.value = "hangup_pending";
			hangupLoading.value = true;
		} else if (msg.status === "hangup_requested") {
			status.value = "ending";
			hangupLoading.value = true;
		} else if (msg.status === "ended") {
			clearHangupTimer();
			hangupLoading.value = false;
		}
	}
});

function clearHangupTimer() {
	if (hangupTimer) {
		clearTimeout(hangupTimer);
		hangupTimer = null;
	}
}

async function loadHistory() {
	if (!clue.value?.id) return;
	loadingHistory.value = true;
	try {
		const data = await (service.customer_pro.clues as any).followList({ cluesId: clue.value.id });
		followList.value = Array.isArray(data) ? data.slice().reverse() : [];
	} catch (e) {
		followList.value = [];
	} finally {
		loadingHistory.value = false;
	}
}

function isConnectedRecord(item: any) {
	const value = item?.isConnected ?? item?.is_connected;
	return value !== 0 && value !== "0";
}

function hasRecording(item: any) {
	return Boolean(item?.audioUrl || item?.audio_url);
}

function open(item: any, sim: SimCardInfo) {
	if (!item?.mobile) {
		ElMessage.warning("该线索没有手机号");
		return;
	}
	if (deviceState.connStatus !== "connected" || !deviceState.androidOnline || !deviceState.canRemoteCall) {
		ElMessage.warning("客户端未连接或未开启电脑控制手机外呼");
		return;
	}
	if (!sim || !sim.available) {
		ElMessage.warning("所选 SIM 卡已不可用，请重新选择");
		return;
	}
	const newCallId = sendDial(String(item.mobile), String(item.id || ""), sim.slotIndex);
	if (!newCallId) {
		ElMessage.warning("客户端未连接");
		return;
	}
	clue.value = item;
	selectedSim.value = sim;
	callId.value = newCallId;
	status.value = "dialing";
	hangupLoading.value = false;
	clearHangupTimer();
	saving.value = false;
	form.remark = "";
	editorKey.value += 1;
	form.followType = [];
	form.nextFollowupTime = "";
	minimized.value = false;
	visible.value = true;
	void loadHistory();
}

function hangup() {
	if (ended.value || !callId.value) return;
	if (!sendHangup(callId.value, String(clue.value?.id || ""))) {
		ElMessage.warning("客户端连接已断开");
		return;
	}
	hangupLoading.value = true;
	status.value = "ending";
	clearHangupTimer();
	// Android 通话时可能短暂失去数据网络，后端会保存挂机指令并在手机重连后补发。
	// 五分钟内保持等待状态，不再把网络瞬断误报成“未开启电脑控制”。
	hangupTimer = setTimeout(() => {
		hangupTimer = null;
		if (!["ending", "hangup_pending"].includes(status.value)) return;
		hangupLoading.value = false;
		status.value = "connected";
		ElMessage.error("挂机指令等待超时，请检查手机网络或在手机端确认通话状态");
	}, 5 * 60 * 1000);
}

async function submitFollow() {
	const currentRemark = String(editorRef.value?.getHtml?.() ?? form.remark ?? "");
	const plainRemark = currentRemark
		.replace(/<br\s*\/?>/gi, "")
		.replace(/<[^>]+>/g, "")
		.replace(/&nbsp;|\u00a0/gi, "")
		.trim();
	const hasMedia = /<(img|video|audio|iframe)\b/i.test(currentRemark);
	if (!plainRemark && !hasMedia) {
		ElMessage.error("请填写跟进内容");
		return;
	}
	form.remark = currentRemark;
	if (!form.nextFollowupTime) {
		ElMessage.error("请选择下次跟进时间");
		return;
	}
	saving.value = true;
	try {
		await (service.customer_pro.clues as any).followAdd({
			cluesId: clue.value.id,
			remark: currentRemark,
			followType: form.followType,
			nextFollowupTime: form.nextFollowupTime,
			callId: callId.value,
			waitForRecording: 1
		});
		ElMessage.success("跟进记录已提交");
		emit("follow-saved", clue.value.id);
		sendCloseFollow(callId.value, String(clue.value?.id || ""));
		visible.value = false;
		minimized.value = false;
	} catch (e: any) {
		ElMessage.error(e?.message || "跟进记录提交失败");
	} finally {
		saving.value = false;
	}
}

function minimize() {
	minimized.value = true;
	visible.value = false;
}

function restore() {
	minimized.value = false;
	visible.value = true;
}

function close() {
	if (callId.value) {
		sendCloseFollow(callId.value, String(clue.value?.id || ""));
	}
	visible.value = false;
	minimized.value = false;
}

function onClosed() {
	if (!ended.value) {
		// 关闭窗口不挂机，通话仍由手机继续执行；最小化可继续恢复窗口。
		return;
	}
	minimized.value = false;
}

onBeforeUnmount(() => {
	clearHangupTimer();
	offDialResult();
});

defineExpose({ open });
</script>

<style lang="scss" scoped>
:global(.remote-call-dialog .el-dialog__header) { margin: 0 !important; padding: 6px 12px !important; border-bottom: 1px solid #eef0f3; }
:global(.remote-call-dialog .el-dialog__body) { padding: 0 12px 12px !important; }
/* 使用自定义标题栏按钮，隐藏 Element Plus 默认的右上角关闭按钮，避免出现两个关闭按钮。 */
:global(.remote-call-dialog .el-dialog__headerbtn) { display: none !important; }
.remote-call-titlebar { display: flex; align-items: center; justify-content: space-between; }
.remote-call-device { display: flex; gap: 14px; align-items: center; font-size: 14px; font-weight: 600; color: #303133; }
.remote-call-device .battery { font-size: 12px; font-weight: 400; color: #67c23a; }
.mobile-connection { display: inline-flex; align-items: center; gap: 5px; font-size: 12px; font-weight: 400; color: #909399; }
.mobile-connection i { width: 7px; height: 7px; border-radius: 50%; background: #c0c4cc; }
.mobile-connection.online { color: #16a34a; }
.mobile-connection.online i { background: #22c55e; box-shadow: 0 0 0 3px #dcfce7; }
.remote-call-window-actions { display: flex; gap: 2px; }
.remote-call-shell { height: 650px; display: grid; grid-template-rows: auto minmax(0, 1fr); min-height: 0; }
.remote-call-summary { display: flex; align-items: center; justify-content: space-between; gap: 18px; border-bottom: 1px solid #edf0f4; padding: 4px 0 6px; min-height: 52px; }
.remote-call-clue, .remote-call-status { display: flex; align-items: center; gap: 10px; }
.remote-call-clue-text .keyword { font-size: 16px; font-weight: 600; color: #303133; }
.remote-call-clue-text .mobile { margin-top: 4px; color: #606266; font-size: 14px; }
.selected-sim { margin-top: 4px; color: #409eff; font-size: 12px; white-space: nowrap; }
.remote-call-status { gap: 22px; }
.status-line { display: flex; align-items: center; gap: 8px; color: #e6a23c; font-size: 15px; }
.status-line.connected { color: #409eff; }.status-line.ended { color: #909399; }
.hangup-button { background: #f56c6c; border-color: #f56c6c; }
.remote-call-body { display: grid; grid-template-columns: 42% 58%; min-height: 0; }
.remote-follow-history, .remote-follow-form { min-width: 0; min-height: 0; padding: 12px 14px; }
.remote-follow-history { border-right: 1px solid #edf0f4; display: flex; flex-direction: column; }
.remote-follow-form { overflow: auto; }
.section-title { color: #303133; font-weight: 600; margin-bottom: 8px; }
.history-scroll { overflow: auto; flex: 1; padding-right: 4px; }
.history-item { padding: 10px 0; border-bottom: 1px solid #f0f2f5; }
.history-meta { display: flex; justify-content: space-between; gap: 8px; color: #606266; font-size: 12px; }
.history-type { margin-top: 6px; color: #409eff; font-size: 12px; }
.history-source { margin-top: 6px; color: #909399; font-size: 12px; line-height: 1.4; }
.history-flags { display: flex; gap: 6px; margin-top: 7px; }
.history-flag { padding: 2px 7px; border-radius: 9px; font-size: 11px; line-height: 1.35; }
.history-flag.is-connected { color: #15803d; background: #dcfce7; }
.history-flag.is-missed { color: #b45309; background: #fef3c7; }
.history-flag.has-recording { color: #1d4ed8; background: #dbeafe; }
.history-flag.no-recording { color: #6b7280; background: #f1f5f9; }
.history-remark { margin-top: 6px; color: #303133; line-height: 1.6; word-break: break-word; font-size: 13px; }
.history-empty { color: #a0a4ad; text-align: center; padding: 36px 8px; }
.follow-form :deep(.el-form-item) { margin-bottom: 8px; }
.form-row { display: flex; gap: 12px; }.form-item-half { flex: 1; min-width: 0; }
.submit-row { display: flex; justify-content: flex-end; padding-top: 0; }
.call-in-progress-button { width: 100%; color: #8dbce8; background: #edf6ff; border-color: #d9ecff; }
.remote-call-minimized { position: fixed; right: 28px; bottom: 22px; z-index: 3000; display: flex; align-items: center; gap: 8px; padding: 10px 14px; border: 1px solid #dcdfe6; border-radius: 22px; background: #fff; box-shadow: 0 4px 18px #0002; cursor: pointer; color: #303133; }
.remote-call-minimized .minimized-state { color: #409eff; font-size: 12px; }
@media (max-width: 900px) { .remote-call-dialog :deep(.el-dialog) { width: 94% !important; } .remote-call-shell { height: 70vh; } }
</style>
