<template>
	<div class="sub-follow-list">
		<div v-if="!followList || followList.length === 0" class="list-empty">
			暂无跟进记录
		</div>
		<el-steps v-else :active="followList.length" direction="vertical">
			<el-step :icon="Edit" style="margin: 0 0 10px 0" v-for="(item, idx) in followList" :key="idx">
				<template #title>
					<div class="follow-title">
						<span class="follow-user">{{ item.userName || '' }}</span>
						<span class="follow-time" v-if="item.createTime">于 {{ formatFollowTime(item.createTime) }}</span>
					</div>
				</template>
				<template #description>
					<div class="follow-content">
						<div class="follow-tag-line">
							<p v-if="item.callId && (item.deviceModel || item.simSlotLabel)" class="follow-source-label">来自： {{ item.deviceModel || "手机" }} 直拨<span v-if="item.simSlotLabel"> · {{ item.simSlotLabel }}</span><span v-if="item.simNumberMasked"> · {{ item.simNumberMasked }}</span><span v-if="item.simCarrierName"> · {{ item.simCarrierName }}</span></p>
							<p v-if="item.followTypeName" class="follow-type-label">跟进方式：{{ item.followTypeName }}</p>
							<span v-if="item.isConnected === 0" class="follow-not-connected">电话未接通</span>
						</div>
						<div class="follow-desc">
							<div class="follow-remark-line">
								<span class="follow-remark-label">备注：</span>
								<div class="follow-remark" :class="{ 'is-overflow': isRemarkOverflow(idx) }" :ref="(el: any) => setRemarkRef(el, idx)" v-html="item.remark"></div>
							</div>
							<div v-if="item.audioUrl" class="follow-audio" @click.stop>
								<span class="audio-duration-label">呼出时长：{{ formatCallDuration(audioDuration[item.id as any] || 0) }}</span>
								<span class="audio-divider"></span>
								<el-button class="audio-action-btn audio-download-btn" text circle :loading="!!audioDownloading[item.id as any]" title="下载录音" @click="downloadAudio(item)">
									<el-icon><Download /></el-icon>
								</el-button>
								<el-button class="audio-action-btn audio-open-btn" text circle :loading="!!audioLoading[item.id as any]" title="播放录音" @click="openAudioPlayer(item)">
									<el-icon><VideoPause v-if="playingAudioId === item.id" /><VideoPlay v-else /></el-icon>
								</el-button>
							</div>
							<div class="follow-footer">
								<el-dropdown trigger="click" @command="(cmd: string) => handleCommand(cmd, item)" @hide="onDropdownHide">
									<el-button text class="more-btn" @click.stop @mouseenter="refreshMoreCursor">更多</el-button>
									<template #dropdown>
										<el-dropdown-menu>
											<el-dropdown-item v-if="!readonly" command="edit">修改</el-dropdown-item>
											<el-dropdown-item command="detail">详情</el-dropdown-item>
											<el-dropdown-item v-if="!readonly" command="delete" divided class="delete-item">删除</el-dropdown-item>
										</el-dropdown-menu>
									</template>
								</el-dropdown>
							</div>
						</div>
					</div>
				</template>
			</el-step>
		</el-steps>

		<!-- 修改/添加弹窗 -->
		<followDialog ref="followDialogRef" :cluesId="cluesId" @saved="onDialogSaved" />

		<!-- 详情弹窗 -->
		<followDetail ref="followDetailRef" @edit="onDetailEdit" />

		<!-- 录音播放器：固定在当前页面底部，播放按钮和进度控制集中在这里 -->
		<div v-if="playerVisible && activePlayerItem" class="audio-player-mask">
			<div class="audio-player-panel" @click.stop>
				<div class="audio-player-header">
					<div class="audio-player-title">录音播放</div>
					<div class="audio-player-name">{{ activePlayerItem.userName || "跟进录音" }}</div>
					<el-button text circle class="audio-player-close" title="关闭" @click="closeAudioPlayer">
						<el-icon><Close /></el-icon>
					</el-button>
				</div>
				<div class="audio-player-controls">
					<el-button text circle title="后退 5 秒" @click="skipAudio(-5)"><el-icon><DArrowLeft /></el-icon></el-button>
					<el-button class="audio-center-play" circle :loading="!!audioLoading[activePlayerKey]" title="播放/暂停" @click="toggleAudio(activePlayerItem)">
						<el-icon><VideoPause v-if="playingAudioId === activePlayerItem.id" /><VideoPlay v-else /></el-icon>
					</el-button>
					<el-button text circle title="前进 5 秒" @click="skipAudio(5)"><el-icon><DArrowRight /></el-icon></el-button>
					<input class="audio-player-progress" type="range" min="0" max="100" step="0.1" :value="audioProgress[activePlayerKey] || 0" @input="seekAudio(activePlayerItem.id, $event)" />
					<span class="audio-player-time">{{ formatAudioTime(audioCurrentTime[activePlayerKey] || 0) }} / {{ formatAudioTime(audioDuration[activePlayerKey] || 0) }}</span>
				</div>
				<div class="audio-player-footer">
					<el-button text class="audio-text-play" @click="toggleAudio(activePlayerItem)">{{ playingAudioId === activePlayerItem.id ? "暂停" : "播放" }}</el-button>
					<div class="audio-volume-control">
						<el-button text circle class="audio-volume-button" title="调整音量" @click="volumeVisible = !volumeVisible">
							<el-icon class="audio-volume-icon"><Mute /></el-icon>
						</el-button>
						<input v-if="volumeVisible" v-model.number="audioVolume" class="audio-volume-slider" type="range" min="0" max="1" step="0.01" aria-label="音量" @input="setAudioVolume" />
					</div>
				</div>
			</div>
		</div>
	</div>
</template>

<script lang="ts" name="customer_pro-subFollowList" setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from "vue";
import { config, useCool } from "/@/cool";
import { useUserStore } from "/$/base/store/user";
import { dayjs, ElMessage, ElMessageBox } from "element-plus";
import { Close, DArrowLeft, DArrowRight, Download, Edit, Mute, VideoPause, VideoPlay } from "@element-plus/icons-vue";
import decodeAmr from "@audio/amr-decode";
import followDialog from "./followDialog.vue";
import followDetail from "./followDetail.vue";
import { formatFollowType } from "./followType";

interface FollowItem {
	id?: string | number;
	userName: string;
	createTime: string;
	remark: string;
	followTypeName: string;
	followupType?: string;
	nextFollowupTime?: string;
	audioUrl?: string;
	callId?: string;
	deviceModel?: string;
	simSlotIndex?: number;
	simSlotLabel?: string;
	simNumberMasked?: string;
	simCarrierName?: string;
	[key: string]: any;
}

const props = defineProps<{
	cluesId?: string | number;
	readonly?: boolean;
}>();

const { service } = useCool();
const user = useUserStore();
const followList = ref<FollowItem[]>([]);
const remarkRefs = ref<Record<number, HTMLElement>>({});
const overflowMap = ref<Record<number, boolean>>({});
const playingAudioId = ref<string | number | null>(null);
const audioProgress = ref<Record<string, number>>({});
const audioCurrentTime = ref<Record<string, number>>({});
const audioDuration = ref<Record<string, number>>({});
const audioLoading = ref<Record<string, boolean>>({});
const audioDownloading = ref<Record<string, boolean>>({});
const audioBuffers = ref<Record<string, AudioBuffer>>({});
const audioSources = ref<Record<string, AudioBufferSourceNode>>({});
const audioGainNodes = ref<Record<string, GainNode>>({});
const audioOffsets = ref<Record<string, number>>({});
const audioStartedAt = ref<Record<string, number>>({});
const audioContext = ref<AudioContext | null>(null);
const nativeAudios = ref<Record<string, HTMLAudioElement>>({});
const playerVisible = ref(false);
const activePlayerId = ref<string | number | null>(null);
const audioVolume = ref(1);
const volumeVisible = ref(false);
let progressTimer: number | null = null;

const activePlayerItem = computed(() => {
	if (activePlayerId.value === null) return null;
	return followList.value.find((item) => String(item.id) === String(activePlayerId.value)) || null;
});
const activePlayerKey = computed(() => (activePlayerId.value === null ? "" : String(activePlayerId.value)));

const followDialogRef = ref<InstanceType<typeof followDialog>>();
const followDetailRef = ref<InstanceType<typeof followDetail>>();

function formatFollowTime(timeStr: string): string {
	if (!timeStr) return "";
	const d = dayjs(timeStr);
	if (!d.isValid()) return timeStr;
	const weekdays = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
	const month = d.month() + 1;
	const date = d.date();
	const weekday = weekdays[d.day()];
	const hour = d.hour();
	const minute = d.minute();
	return `${month}月${date}日 (${weekday}) ${hour}点${minute}分`;
}

async function loadFollowList() {
	if (!props.cluesId) return;
	try {
		const res = await (service.customer_pro.clues as any).followList({ cluesId: props.cluesId });
		if (Array.isArray(res)) {
		followList.value = res.map((item: any) => ({
			id: item.id,
			userName: item.userName || item.nickName || "",
			createTime: item.createTime || "",
			remark: item.remark || "",
			followupType: item.followupType || item.followup_type || item.followType || item.follow_type || "",
			followTypeName: formatFollowType(item.followupType || item.followup_type || item.followType || item.follow_type) || item.followTypeName || "",
			nextFollowupTime: item.nextFollowupTime || "",
			audioUrl: item.audioUrl || item.audio_url || "",
			callId: item.callId || item.call_id || "",
			deviceModel: item.deviceModel || item.device_model || "",
			simSlotIndex: item.simSlotIndex ?? item.sim_slot_index ?? -1,
			simSlotLabel: item.simSlotLabel || item.sim_slot_label || "",
			simNumberMasked: item.simNumberMasked || item.sim_number_masked || "",
			simCarrierName: item.simCarrierName || item.sim_carrier_name || "",
			// 电话是否接通：0=未接通，其余=接通（由移动端自动跟进判定）
			isConnected: item.isConnected === 0 || item.isConnected === "0" ? 0 : 1
		})).reverse();
			void preloadAudioDurations(followList.value);
		}
		nextTick(checkOverflow);
	} catch (e) {
		console.error("获取跟进记录失败:", e);
	}
}

function formatAudioTime(seconds: number): string {
	if (!Number.isFinite(seconds) || seconds <= 0) return "00:00";
	const total = Math.floor(seconds);
	const minutes = Math.floor(total / 60);
	const remaining = total % 60;
	return `${String(minutes).padStart(2, "0")}:${String(remaining).padStart(2, "0")}`;
}

function formatCallDuration(seconds: number): string {
	if (!Number.isFinite(seconds) || seconds <= 0) return "读取中...";
	const total = Math.floor(seconds);
	const minutes = Math.floor(total / 60);
	const remaining = total % 60;
	if (minutes > 0) return `${minutes}分${remaining}秒`;
	return `${remaining}秒`;
}

function getAudioContext(): AudioContext {
	if (audioContext.value) return audioContext.value;
	const AudioContextCtor = window.AudioContext || (window as any).webkitAudioContext;
	if (!AudioContextCtor) throw new Error("当前浏览器不支持 Web Audio");
	audioContext.value = new AudioContextCtor();
	return audioContext.value;
}

function getAudioSourceUrl(item: FollowItem): string {
	if (!item.audioUrl || item.id === undefined || item.id === null) return item.audioUrl || "";
	// AWB 是 AMR-WB，Firefox/Chrome 以及当前前端 AMR-NB 解码器都不能稳定解码。
	// 播放和时长预读统一请求后端 FFmpeg 转成 MP3，OSS 原文件保持不变。
	if (getAudioExtension(item) === "awb") {
		return `${config.baseUrl}/admin/customer_pro/clues/audio?id=${encodeURIComponent(String(item.id))}&format=mp3`;
	}
	try {
		const source = new URL(item.audioUrl, window.location.origin);
		if (source.origin !== window.location.origin) {
			return `${config.baseUrl}/admin/customer_pro/clues/audio?id=${encodeURIComponent(String(item.id))}`;
		}
	} catch {
		// 交给 fetch 处理非法地址并返回明确错误
	}
	return item.audioUrl;
}

function getAudioDownloadUrl(item: FollowItem): string {
	return `${config.baseUrl}/admin/customer_pro/clues/audio?id=${encodeURIComponent(String(item.id))}&format=mp3&download=1`;
}

function getAudioDownloadName(item: FollowItem): string {
	try {
		const pathname = new URL(item.audioUrl || "", window.location.origin).pathname;
		const originalName = pathname.split("/").pop() || `call-${item.id}`;
		const dotIndex = originalName.lastIndexOf(".");
		return `${dotIndex > 0 ? originalName.slice(0, dotIndex) : originalName}.mp3`;
	} catch {
		return `call-${item.id}.mp3`;
	}
}

function getAudioExtension(item: FollowItem): string {
	try {
		return new URL(item.audioUrl || "", window.location.origin).pathname.split(".").pop()?.toLowerCase() || "";
	} catch {
		return item.audioUrl?.split("?")[0].split(".").pop()?.toLowerCase() || "";
	}
}

function isAmrAudio(item: FollowItem): boolean {
	return ["amr", "awb"].includes(getAudioExtension(item));
}

function startProgressTimer() {
	if (progressTimer !== null) return;
	progressTimer = window.setInterval(() => {
		if (playingAudioId.value !== null) {
			const key = String(playingAudioId.value);
			const ctx = audioContext.value;
			const duration = audioDuration.value[key] || 0;
			const startedAt = audioStartedAt.value[key];
			if (ctx && duration > 0 && startedAt !== undefined) {
				const current = Math.min(duration, audioOffsets.value[key] + (ctx.currentTime - startedAt));
				audioCurrentTime.value[key] = current;
				audioProgress.value[key] = (current / duration) * 100;
			}
		}
	}, 100);
}

function stopProgressTimer() {
	if (progressTimer !== null) {
		window.clearInterval(progressTimer);
		progressTimer = null;
	}
}

function pauseAudio(id: string | number) {
	const key = String(id);
	const nativeAudio = nativeAudios.value[key];
	if (nativeAudio) {
		nativeAudio.pause();
		const current = Number.isFinite(nativeAudio.currentTime) ? nativeAudio.currentTime : 0;
		audioOffsets.value[key] = current;
		audioCurrentTime.value[key] = current;
		if (audioDuration.value[key] > 0) {
			audioProgress.value[key] = (current / audioDuration.value[key]) * 100;
		}
	}
	const ctx = audioContext.value;
	const source = audioSources.value[key];
	if (ctx && source) {
		const startedAt = audioStartedAt.value[key];
		if (startedAt !== undefined) {
			audioOffsets.value[key] = Math.min(audioDuration.value[key] || 0, audioOffsets.value[key] + ctx.currentTime - startedAt);
		}
		source.onended = null;
		source.stop();
		delete audioSources.value[key];
		delete audioGainNodes.value[key];
	}
	playingAudioId.value = null;
	startProgressTimer();
	stopProgressTimer();
}

async function downloadAudio(item: FollowItem) {
	if (item.id === undefined || item.id === null) return;
	const key = String(item.id);
	audioDownloading.value[key] = true;
	try {
		const { data, contentType } = await fetchAudioData(item, getAudioDownloadUrl(item));
		const objectUrl = URL.createObjectURL(new Blob([data], { type: contentType || "audio/mpeg" }));
		const link = document.createElement("a");
		link.href = objectUrl;
		link.download = getAudioDownloadName(item);
		document.body.appendChild(link);
		link.click();
		link.remove();
		window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
	} catch (error: any) {
		ElMessage.error(`录音下载失败：${error?.message || "录音地址不可访问"}`);
	} finally {
		audioDownloading.value[key] = false;
	}
}

async function openAudioPlayer(item: FollowItem) {
	if (item.id === undefined || item.id === null) return;
	activePlayerId.value = item.id;
	playerVisible.value = true;
	if (playingAudioId.value !== item.id) await toggleAudio(item);
}

function closeAudioPlayer() {
	if (playingAudioId.value !== null) pauseAudio(playingAudioId.value);
	playerVisible.value = false;
	activePlayerId.value = null;
}

async function skipAudio(seconds: number) {
	const item = activePlayerItem.value;
	if (!item || item.id === undefined || item.id === null) return;
	const key = String(item.id);
	const duration = audioDuration.value[key] || 0;
	if (!duration) return;
	const current = audioCurrentTime.value[key] || audioOffsets.value[key] || 0;
	const next = Math.max(0, Math.min(duration, current + seconds));
	audioOffsets.value[key] = next;
	audioCurrentTime.value[key] = next;
	audioProgress.value[key] = (next / duration) * 100;
	const nativeAudio = nativeAudios.value[key];
	if (nativeAudio) {
		nativeAudio.currentTime = next;
		if (playingAudioId.value === item.id) await nativeAudio.play().catch(() => undefined);
		return;
	}
	if (playingAudioId.value === item.id) {
		pauseAudio(item.id);
		await toggleAudio(item);
	}
}

function setAudioVolume() {
	const value = Math.max(0, Math.min(1, Number(audioVolume.value)));
	audioVolume.value = value;
	Object.values(nativeAudios.value).forEach((audio) => {
		audio.volume = value;
	});
	Object.values(audioGainNodes.value).forEach((gain) => {
		gain.gain.value = value;
	});
}

async function fetchAudioData(item: FollowItem, sourceUrl = getAudioSourceUrl(item)): Promise<{ data: ArrayBuffer; contentType: string }> {
	if (!item.audioUrl) throw new Error("录音地址为空");
	const response = await fetch(sourceUrl, {
		credentials: "same-origin",
		// 避免浏览器继续使用此前接口返回的 JSON 错误缓存。
		cache: "no-store",
		headers: user.token ? { Authorization: user.token } : undefined
	});
	if (!response.ok) throw new Error(`录音请求失败（${response.status}）`);
	const data = await response.arrayBuffer();
	const bytes = new Uint8Array(data);
	const contentType = response.headers.get("content-type") || "";
	// 后端错误响应是 JSON，不能继续交给音频解码器，否则会误报成“M4A/AMR格式错误”。
	const header = new TextDecoder().decode(bytes.subarray(0, 32)).trimStart().toLowerCase();
	if (contentType.includes("json") || contentType.includes("text/html") || header.startsWith("<!doctype html") || header.startsWith("<html")) {
		let message = "录音接口返回错误";
		if (contentType.includes("text/html") || header.startsWith("<!doctype html") || header.startsWith("<html")) {
			message = "录音接口地址错误，请检查前端开发代理配置";
		}
		try {
			const payload = JSON.parse(new TextDecoder().decode(bytes));
			message = payload?.message || message;
		} catch {
			// 保留默认提示
		}
		throw new Error(message);
	}
	return { data, contentType };
}

async function loadAudioBuffer(item: FollowItem): Promise<AudioBuffer> {
	const key = String(item.id);
	if (audioBuffers.value[key]) return audioBuffers.value[key];
	const { data } = await fetchAudioData(item);
	const bytes = new Uint8Array(data);
	const header = new TextDecoder().decode(bytes.subarray(0, 9));
	const ctx = getAudioContext();
	let buffer: AudioBuffer;
	if (header.startsWith("#!AMR-WB")) {
		throw new Error("AMR-WB 需要后端转换为 MP3");
	} else if (header.startsWith("#!AMR")) {
		const decoded = await decodeAmr(bytes);
		if (!decoded.channelData.length || !decoded.channelData[0]?.length) {
			throw new Error("AMR 文件解码失败");
		}
		buffer = ctx.createBuffer(decoded.channelData.length, decoded.channelData[0].length, decoded.sampleRate);
		decoded.channelData.forEach((channel, index) => buffer.copyToChannel(channel, index));
	} else {
		try {
			// M4A/AAC、MP3、WAV 等格式交给浏览器原生解码。
			buffer = await ctx.decodeAudioData(data.slice(0));
		} catch {
			throw new Error(`不支持的录音格式（${header.replace(/[\r\n]/g, " ").slice(0, 16)}）`);
		}
	}
	audioBuffers.value[key] = buffer;
	audioDuration.value[key] = buffer.duration;
	return buffer;
}

function getNativeAudioUrls(item: FollowItem): string[] {
	const originalUrl = item.audioUrl || "";
	const proxyUrl = getAudioSourceUrl(item);
	// 播放保持原来的逻辑：OSS地址可直接播放时优先使用原地址，
	// 否则使用后端代理；下载转换不会影响播放链路。
	if (originalUrl && originalUrl !== proxyUrl && /^https?:\/\//i.test(originalUrl)) {
		return [originalUrl, proxyUrl];
	}
	return [proxyUrl];
}

function waitForNativeAudioMetadata(audio: HTMLAudioElement): Promise<void> {
	if (audio.readyState >= HTMLMediaElement.HAVE_METADATA && Number.isFinite(audio.duration) && audio.duration > 0) {
		return Promise.resolve();
	}
	return new Promise((resolve, reject) => {
		let timer: number | null = window.setTimeout(() => {
			cleanup();
			reject(new Error("浏览器无法读取录音时长"));
		}, 15000);
		const cleanup = () => {
			if (timer !== null) window.clearTimeout(timer);
			audio.removeEventListener("loadedmetadata", onLoaded);
			audio.removeEventListener("error", onError);
		};
		const onLoaded = () => {
			cleanup();
			if (Number.isFinite(audio.duration) && audio.duration > 0) resolve();
			else reject(new Error("录音时长读取失败"));
		};
		const onError = () => {
			cleanup();
			reject(new Error("浏览器无法解码该录音"));
		};
		audio.addEventListener("loadedmetadata", onLoaded, { once: true });
		audio.addEventListener("error", onError, { once: true });
	});
}

function bindNativeAudioEvents(item: FollowItem, audio: HTMLAudioElement) {
	const key = String(item.id);
	audio.onloadedmetadata = () => {
		audioDuration.value[key] = Number.isFinite(audio.duration) ? audio.duration : 0;
	};
	audio.ontimeupdate = () => {
		const current = Number.isFinite(audio.currentTime) ? audio.currentTime : 0;
		audioCurrentTime.value[key] = current;
		const duration = audioDuration.value[key] || audio.duration || 0;
		if (duration > 0) audioProgress.value[key] = (current / duration) * 100;
	};
	audio.onended = () => {
		audioOffsets.value[key] = 0;
		audioCurrentTime.value[key] = audioDuration.value[key] || audio.duration || 0;
		audioProgress.value[key] = 100;
		if (playingAudioId.value === item.id) playingAudioId.value = null;
	};
}

async function createNativeAudio(item: FollowItem): Promise<HTMLAudioElement> {
	const key = String(item.id);
	if (nativeAudios.value[key]) return nativeAudios.value[key];
	let lastError: unknown;
	for (const sourceUrl of getNativeAudioUrls(item)) {
		if (!sourceUrl) continue;
		const candidate = new Audio();
		candidate.preload = "metadata";
		candidate.volume = audioVolume.value;
		candidate.src = sourceUrl;
		candidate.load();
		try {
			await waitForNativeAudioMetadata(candidate);
			bindNativeAudioEvents(item, candidate);
			nativeAudios.value[key] = candidate;
			audioDuration.value[key] = candidate.duration;
			return candidate;
		} catch (error) {
			lastError = error;
			candidate.pause();
			candidate.removeAttribute("src");
			candidate.load();
		}
	}
	throw lastError || new Error("浏览器无法解码该录音");
}

async function preloadNativeAudioMetadata(item: FollowItem) {
	if (!item.audioUrl || item.id === undefined || item.id === null || isAmrAudio(item)) return;
	try {
		await createNativeAudio(item);
	} catch {
		// 预加载失败不打断跟进列表，点击播放时仍会再次尝试并提示具体错误。
	}
}

async function preloadAudioDurations(items: FollowItem[]) {
	const queue = items.filter((item) => item.audioUrl);
	let nextIndex = 0;
	const worker = async () => {
		while (nextIndex < queue.length) {
			const item = queue[nextIndex++];
			if (isAmrAudio(item)) {
				try {
					// AMR 在前端解码；AWB 由后端转 MP3 后解码，提前处理可直接显示时长。
					await loadAudioBuffer(item);
				} catch {
					// 预加载失败不打断列表加载。
				}
			} else {
				await preloadNativeAudioMetadata(item);
			}
		}
	};
	// 限制并发，避免进入详情时同时向 OSS 发起大量音频元数据请求。
	await Promise.all(Array.from({ length: Math.min(3, queue.length) }, () => worker()));
}

async function toggleNativeAudio(item: FollowItem) {
	if (item.id === undefined || item.id === null) return;
	const key = String(item.id);
	if (playingAudioId.value === item.id) {
		pauseAudio(item.id);
		return;
	}
	if (playingAudioId.value !== null) pauseAudio(playingAudioId.value);
	audioLoading.value[key] = true;
	try {
		const audio = await createNativeAudio(item);
		if (audio.readyState < HTMLMediaElement.HAVE_METADATA) {
			await waitForNativeAudioMetadata(audio);
		}
		const duration = audioDuration.value[key] || audio.duration || 0;
		if (duration <= 0) throw new Error("录音时长读取失败");
		audioDuration.value[key] = duration;
		const offset = audioOffsets.value[key] || 0;
		audio.volume = audioVolume.value;
		audio.currentTime = offset >= duration ? 0 : offset;
		await audio.play();
		playingAudioId.value = item.id;
	} catch (error: any) {
		ElMessage.error(`录音播放失败：${error?.message || "浏览器无法解码该录音"}`);
	} finally {
		audioLoading.value[key] = false;
	}
}

async function toggleAudio(item: FollowItem) {
	if (item.id === undefined || item.id === null) return;
	const key = String(item.id);
	if (playingAudioId.value === item.id) {
		pauseAudio(item.id);
		return;
	}
	if (!isAmrAudio(item)) {
		await toggleNativeAudio(item);
		return;
	}
	if (playingAudioId.value !== null) pauseAudio(playingAudioId.value);
	audioLoading.value[key] = true;
	try {
		const ctx = getAudioContext();
		await ctx.resume();
		const buffer = await loadAudioBuffer(item);
		const offset = audioOffsets.value[key] || 0;
		const source = ctx.createBufferSource();
		source.buffer = buffer;
		const gain = ctx.createGain();
		gain.gain.value = audioVolume.value;
		source.connect(gain);
		gain.connect(ctx.destination);
		source.onended = () => {
			if (playingAudioId.value === item.id) {
				audioOffsets.value[key] = 0;
				audioCurrentTime.value[key] = buffer.duration;
				audioProgress.value[key] = 100;
				playingAudioId.value = null;
				stopProgressTimer();
			}
		};
		source.start(0, offset >= buffer.duration ? 0 : offset);
		audioSources.value[key] = source;
		audioGainNodes.value[key] = gain;
		audioOffsets.value[key] = offset >= buffer.duration ? 0 : offset;
		audioStartedAt.value[key] = ctx.currentTime;
		playingAudioId.value = item.id;
		startProgressTimer();
	} catch (error: any) {
		ElMessage.error(`录音播放失败：${error?.message || "录音地址不可访问"}`);
	} finally {
		audioLoading.value[key] = false;
	}
}

function seekAudio(id: string | number | undefined, event: Event) {
	if (id === undefined || id === null) return;
	const key = String(id);
	const duration = audioDuration.value[key] || 0;
	if (!duration) return;
	const value = Number((event.target as HTMLInputElement).value);
	const offset = (value / 100) * duration;
	audioOffsets.value[key] = offset;
	audioCurrentTime.value[key] = offset;
	audioProgress.value[key] = value;
	const nativeAudio = nativeAudios.value[key];
	if (nativeAudio) {
		nativeAudio.currentTime = offset;
		if (playingAudioId.value === id) nativeAudio.play().catch(() => undefined);
		return;
	}
	if (playingAudioId.value === id) {
		const item = followList.value.find((entry) => String(entry.id) === key);
		if (item) {
			pauseAudio(id);
			toggleAudio(item);
		}
	}
}


onBeforeUnmount(() => {
	if (playingAudioId.value !== null) pauseAudio(playingAudioId.value);
	Object.values(nativeAudios.value).forEach((audio) => audio.pause());
	if (audioContext.value) audioContext.value.close();
});

function setRemarkRef(el: any, idx: number) {
	if (el) remarkRefs.value[idx] = el;
}

function checkOverflow() {
	const map: Record<number, boolean> = {};
	Object.keys(remarkRefs.value).forEach((key) => {
		const idx = Number(key);
		const el = remarkRefs.value[idx];
		if (el) {
			map[idx] = el.scrollHeight > el.clientHeight;
		}
	});
	overflowMap.value = map;
}

function isRemarkOverflow(idx: number): boolean {
	return !!overflowMap.value[idx];
}

function handleCommand(cmd: string, item: FollowItem) {
	switch (cmd) {
		case "edit":
			followDialogRef.value?.open(item);
			break;
		case "detail":
			followDetailRef.value?.open(item);
			break;
		case "delete":
			handleDelete(item);
			break;
	}
}

async function handleDelete(item: FollowItem) {
	try {
		await ElMessageBox.confirm("确定要删除该跟进记录吗？删除后不可恢复。", "提示", {
			confirmButtonText: "确定",
			cancelButtonText: "取消",
			type: "warning"
		});
		await (service.customer_pro.clues as any).followDelete({
			id: item.id,
			cluesId: props.cluesId
		});
		ElMessage.success("跟进记录已删除");
		loadFollowList();
	} catch {
		// 用户取消
	}
}

function onDropdownHide() {
	// 修复 aria-hidden 警告：下拉菜单隐藏时主动移除焦点
	if (document.activeElement instanceof HTMLElement) {
		document.activeElement.blur();
	}
}

function refreshMoreCursor(event: MouseEvent) {
	const element = event.currentTarget as HTMLElement | null;
	if (!element) return;
	element.style.setProperty("cursor", "pointer", "important");
	requestAnimationFrame(() => {
		if (element.isConnected) element.style.setProperty("cursor", "pointer", "important");
	});
}

function onDialogSaved() {
	loadFollowList();
}

function onDetailEdit(data: any) {
	followDialogRef.value?.open(data);
}

onMounted(() => {
	loadFollowList();
});

watch(() => props.cluesId, () => {
	loadFollowList();
});

function openFollowDialog() {
	followDialogRef.value?.open();
}

defineExpose({ refresh: loadFollowList, followList, openFollowDialog });
</script>

<style lang="scss" scoped>
.sub-follow-list {
	height: 100%;
	overflow-y: auto;
	overflow-x: hidden;
	padding: 0 16px 16px;

	.list-empty {
		text-align: center;
		padding: 40px;
		color: #999;
	}

	:deep(.el-step) {
		flex-basis: auto !important;
		max-width: 100%;
	}

	:deep(.el-step__main) {
		padding-bottom: 4px;
		flex: 1;
		min-width: 0;
	}

	:deep(.el-step__description) {
		width: 100%;
	}

	:deep(.el-step__head) {
		padding-right: 10px;
	}

	.follow-title {
		display: flex;
		align-items: baseline;
		gap: 6px;

		.follow-user {
			font-size: 14px;
			font-weight: 500;
			color: #303133;
		}

		.follow-time {
			font-size: 12px;
			font-weight: 400;
			color: #909399;
		}
	}

	.follow-content {
		min-width: 0;
		overflow: hidden;

		.follow-type-label {
			font-size: 13px;
			font-weight: 400;
			color: #606266;
			margin-bottom: 6px;
		}

		.follow-tag-line {
			display: block;
			margin-bottom: 6px;

			.follow-type-label {
				display: block;
				margin-bottom: 4px;
			}

			.follow-source-label {
				display: block;
				font-size: 13px;
				font-weight: 400;
				color: #606266;
				margin: 0 0 4px;
			}
		}

		.follow-not-connected {
			display: inline-block;
			font-size: 12px;
			line-height: 20px;
			padding: 0 8px;
			border-radius: 4px;
			color: #f56c6c;
			background: #fef0f0;
			border: 1px solid #fbc4c4;
			margin-top: 4px;
		}
	}

	.follow-desc {
		background: transparent;
		padding: 0;
		width: 100%;
		box-sizing: border-box;

		.follow-remark-line {
			display: flex;
			align-items: flex-start;
			min-width: 0;
		}

		.follow-remark-label {
			flex: 0 0 auto;
			font-size: 13px;
			line-height: 1.6;
			color: #606266;
		}

		.follow-remark {
			flex: 1;
			min-width: 0;
			font-size: 13px;
			font-weight: 400;
			color: #303133;
			word-break: break-word;
			max-height: 120px;
			overflow: hidden;
			position: relative;

			&.is-overflow::after {
				content: '';
				position: absolute;
				bottom: 0;
				left: 0;
				right: 0;
				height: 30px;
				background: linear-gradient(transparent, rgba(255, 255, 255, 0.94));
				pointer-events: none;
			}

			:deep(img) {
				max-width: 100%;
				height: auto;
			}

			:deep(p) {
				margin: 0;
				line-height: 1.6;
			}
		}

		.follow-audio {
			display: flex;
			align-items: center;
			gap: 8px;
			margin-top: 8px;
			padding: 5px 9px;
			width: 50%;
			box-sizing: border-box;
			background: #f7f8fa;
			border: 1px solid #ebeef5;
			border-radius: 6px;
			min-height: 34px;

			.audio-duration-label {
				flex: 1;
				min-width: 0;
				font-size: 13px;
				color: #606266;
				white-space: nowrap;
			}

			.audio-divider {
				width: 1px;
				height: 20px;
				background: #e4e7ed;
			}

			.audio-action-btn {
				width: 28px;
				height: 28px;
				margin: 0;
				font-size: 18px;
				padding: 0;
			}

			.audio-download-btn {
				color: #ff666f;
			}

			.audio-open-btn {
				color: #18b99a;
			}

			.el-icon {
				vertical-align: middle;
			}
		}

		.follow-footer {
			display: flex;
			justify-content: flex-end;
			margin-top: 8px;
			cursor: pointer !important;

			:deep(.el-dropdown),
			:deep(.el-dropdown .el-tooltip__trigger),
			:deep(.el-dropdown > *) {
				cursor: pointer !important;
			}

			.more-btn {
				border: 0;
				background: transparent;
				font-size: 12px;
				color: #909399;
				display: inline-block;
				font-weight: 400;
				cursor: pointer !important;
				user-select: none;
				padding: 2px 8px;
				border-radius: 4px;
				transition: all 0.2s;

				&:hover {
					color: var(--color-primary);
					background: rgba(64, 158, 255, 0.06);
				}
			}
		}
	}

	@media (max-width: 768px) {
		.follow-audio {
			width: 100%;
		}
	}

	.audio-player-mask {
		position: absolute;
		z-index: 2000;
		left: 0;
		right: 0;
		top: 0;
		bottom: 0;
		display: flex;
		justify-content: center;
		align-items: center;
		padding: 20px;
		background: rgba(0, 0, 0, 0.38);
		pointer-events: auto;
	}

	.audio-player-panel {
		width: min(600px, 100%);
		padding: 14px 18px 12px;
		border: 1px solid #dcdfe6;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.98);
		box-shadow: 0 8px 30px rgba(0, 0, 0, 0.16);
		pointer-events: auto;
	}

	.audio-player-header,
	.audio-player-footer {
		display: flex;
		align-items: center;
	}

	.audio-player-header {
		gap: 10px;
		margin-bottom: 10px;
	}

	.audio-player-title {
		font-size: 14px;
		font-weight: 600;
		color: #303133;
	}

	.audio-player-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		color: #909399;
		font-size: 13px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.audio-player-close {
		margin-left: auto;
		color: #909399;
	}

	.audio-player-controls {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.audio-player-controls > .el-button {
		flex: 0 0 auto;
		margin: 0;
		font-size: 20px;
		color: #606266;
	}

	.audio-player-controls .audio-center-play {
		width: 34px;
		height: 34px;
		color: #fff;
		background: #18b99a;
		border-color: #18b99a;
	}

	.audio-player-progress {
		flex: 1;
		min-width: 80px;
		margin: 0 8px;
		accent-color: #18b99a;
		cursor: pointer;
	}

	.audio-player-time {
		flex: 0 0 92px;
		font-size: 12px;
		color: #909399;
		text-align: right;
		font-variant-numeric: tabular-nums;
	}

	.audio-player-footer {
		justify-content: space-between;
		margin-top: 8px;
	}

	.audio-text-play {
		margin-left: -8px;
		padding: 4px 8px;
		color: #606266;
	}

	.audio-volume-control {
		display: flex;
		align-items: center;
		gap: 7px;
	}

	.audio-volume-icon {
		font-size: 18px;
		color: #606266;
	}

	.audio-volume-button {
		width: 30px;
		height: 30px;
		margin: 0;
		padding: 0;
	}

	.audio-volume-slider {
		width: 86px;
		accent-color: #18b99a;
		cursor: pointer;
	}

	:deep(.delete-item) {
		color: #f56c6c;
	}
}
</style>

<style lang="scss">
/* 全局样式：dropdown-menu teleport 到 body，scoped 无法穿透 */
.el-dropdown-menu .delete-item {
	color: #f56c6c !important;

	&:hover {
		background-color: #fef0f0 !important;
		color: #f56c6c !important;
	}
}
</style>
