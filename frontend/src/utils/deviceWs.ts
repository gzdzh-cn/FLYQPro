import { reactive } from "vue";
import { config } from "/@/cool/config";
import { useUserStore } from "/$/base/store/user";
import { ElMessageBox } from "element-plus";

// PC 端设备 WebSocket 客户端（type=pc）
// 负责：1) 维持与后端的 WS 连接；2) 接收 Android 在线/离线状态（由后端在移动端上线/下线时推送，
//       即「移动端发信号给 PC，PC 才看在线中」，PC 不主动探测）；3) 下发远程拨号指令；
//       4) 接收拨号结果回执；5) 处理被踢出事件。

export type ConnStatus = "disconnected" | "connecting" | "connected";

export interface SimCardInfo {
	slotIndex: number;
	slotLabel: string;
	numberMasked: string;
	carrierName: string;
	available: boolean;
}

interface DeviceState {
	connStatus: ConnStatus; // WS 连接状态
	androidOnline: boolean; // 同账号 Android 客户端是否在线（完全由后端推送信号驱动）
	deviceModel: string; // 在线移动端设备型号
	deviceId: string; // 在线移动端设备唯一码
	os: string; // 在线移动端系统：android / harmony / ios
	battery: number; // 手机电量百分比
	canRemoteCall: boolean; // 是否允许电脑控制手机外呼
	simCards: SimCardInfo[];
}

export const deviceState = reactive<DeviceState>({
	connStatus: "disconnected",
	androidOnline: false,
	deviceModel: "",
	deviceId: "",
	os: "",
	battery: -1,
	canRemoteCall: false,
	simCards: []
});

type DeviceEventType = "kick_out" | "dial_result" | "open" | "close";
type DeviceListener = (payload?: any) => void;

const listeners: Record<string, DeviceListener[]> = {};
let socket: WebSocket | null = null;
let reconnectTimer: any = null;
let heartbeatTimer: any = null;
let manualClose = false;

/** 订阅设备事件，返回取消订阅函数 */
export function onDeviceEvent(type: DeviceEventType, cb: DeviceListener) {
	(listeners[type] = listeners[type] || []).push(cb);
	return () => offDeviceEvent(type, cb);
}

/** 取消订阅设备事件 */
export function offDeviceEvent(type: DeviceEventType, cb: DeviceListener) {
	const arr = listeners[type];
	if (!arr) return;
	const idx = arr.indexOf(cb);
	if (idx >= 0) arr.splice(idx, 1);
}

function emit(type: DeviceEventType, payload?: any) {
	(listeners[type] || []).forEach((cb) => cb(payload));
}

function asBoolean(value: any): boolean {
	return value === true || value === 1 || value === "1" || value === "true";
}

function clearAndroidState(preserveDevice = false) {
	deviceState.androidOnline = false;
	deviceState.canRemoteCall = false;
	deviceState.simCards = [];
	if (!preserveDevice) {
		deviceState.deviceModel = "";
		deviceState.deviceId = "";
		deviceState.os = "";
		deviceState.battery = -1;
	}
}

function getWsBase(): string {
	let base = (config as any).host as string;
	if (!base) {
		base = window.location.origin;
	}
	const proto = base.startsWith("https") ? "wss" : "ws";
	const host = base.replace(/^https?:\/\//, "");
	return `${proto}://${host}`;
}

function startHeartbeat() {
	stopHeartbeat();
	heartbeatTimer = setInterval(() => {
		if (socket && socket.readyState === WebSocket.OPEN) {
			socket.send(JSON.stringify({ type: "heartbeat" }));
		}
	}, 30000);
}

function stopHeartbeat() {
	if (heartbeatTimer) {
		clearInterval(heartbeatTimer);
		heartbeatTimer = null;
	}
}

function scheduleReconnect() {
	if (manualClose) return;
	if (reconnectTimer) return;
	// 后台静默重连：UI 不暴露「重连中」，连不上即一直显示「离线中」。
	// 间隔拉长，避免后端未就绪时频繁重试打日志。
	reconnectTimer = setTimeout(() => {
		reconnectTimer = null;
		connectDeviceWs();
	}, 10000);
}

function handleMessage(raw: string) {
	let msg: any;
	try {
		msg = JSON.parse(raw);
	} catch {
		return;
	}
	switch (msg.type) {
		case "online":
			// 移动端上线：记录设备型号/唯一码/系统（信号来自后端推送）
			deviceState.androidOnline = asBoolean(msg.canRemoteCall);
			if (msg.deviceModel) deviceState.deviceModel = msg.deviceModel;
			if (msg.deviceId) deviceState.deviceId = msg.deviceId;
			if (msg.os) deviceState.os = msg.os;
			deviceState.battery = Number.isFinite(Number(msg.battery)) ? Number(msg.battery) : -1;
			deviceState.canRemoteCall = asBoolean(msg.canRemoteCall);
			deviceState.simCards = Array.isArray(msg.simCards) ? msg.simCards : [];
			if (!deviceState.canRemoteCall) clearAndroidState();
			break;
		case "offline":
			// 通话时 Wi-Fi/蜂窝数据切换可能造成瞬断。在线能力置为 false，
			// 但保留设备型号供正在进行的通话窗口展示。
			clearAndroidState(true);
			break;
		case "online_status":
			// 兼容保留：后端推送的在线状态快照
			const remoteAvailable = asBoolean(msg.online) && asBoolean(msg.canRemoteCall);
			deviceState.androidOnline = remoteAvailable;
			deviceState.canRemoteCall = asBoolean(msg.canRemoteCall);
			if (remoteAvailable) {
				deviceState.deviceModel = msg.deviceModel || "";
				deviceState.deviceId = msg.deviceId || "";
				deviceState.os = msg.os || "";
				deviceState.battery = Number.isFinite(Number(msg.battery)) ? Number(msg.battery) : -1;
				deviceState.simCards = Array.isArray(msg.simCards) ? msg.simCards : [];
			} else {
				clearAndroidState(true);
			}
			break;
		case "kick_out":
			emit("kick_out", msg);
			ElMessageBox.alert(msg.content || "账号已在其他客户端登录", "提示", {
				confirmButtonText: "重新登录",
				type: "warning",
				showClose: false
			}).then(() => {
				const user = useUserStore();
				user.logout();
			});
			break;
		case "dial_result":
			emit("dial_result", msg);
			break;
	}
}

/** 建立 WS 连接 */
export function connectDeviceWs() {
	const user = useUserStore();
	const token = user.token;
	if (!token) return;
	if (socket && socket.readyState === WebSocket.CONNECTING) {
		return;
	}
	if (socket && socket.readyState === WebSocket.OPEN) {
		// 连接已存在（如 SPA 内刷新未重连），直接补发一次在线检查兜底，
		// 不等 onopen，确保「每次刷新都主动检查客户端是否在线」。
		socket.send(JSON.stringify({ type: "query_online" }));
		return;
	}
	manualClose = false;
	const base = getWsBase();
	const url = `${base}/app/customer_pro/im/ws?token=${encodeURIComponent(token)}&type=pc`;
	deviceState.connStatus = "connecting";

	socket = new WebSocket(url);

	socket.onopen = () => {
		deviceState.connStatus = "connected";
		startHeartbeat();
		// 兜底：连接建立后主动询问一次同账号移动端当前在线状态，
		// 弥补「PC 先上线、移动端后上线」的时序空窗。移动端上线时
		// 仍由后端主动推送 online 信号（主路径）。
		socket?.send(JSON.stringify({ type: "query_online" }));
		emit("open");
	};

	socket.onmessage = (ev) => {
		handleMessage(ev.data as string);
	};

	socket.onclose = () => {
		deviceState.connStatus = "disconnected";
		clearAndroidState();
		stopHeartbeat();
		socket = null;
		emit("close");
		scheduleReconnect();
	};

	socket.onerror = () => {
		socket?.close();
	};
}

/** 主动断开（退出登录时调用） */
export function disconnectDeviceWs() {
	manualClose = true;
	stopHeartbeat();
	if (reconnectTimer) {
		clearTimeout(reconnectTimer);
		reconnectTimer = null;
	}
	if (socket) {
		socket.close();
		socket = null;
	}
}

/** 下发远程拨号指令，成功返回 callId，失败返回 false。 */
export function sendDial(mobile: string, cluesId: string, simSlotIndex: number): string | false {
	if (!socket || socket.readyState !== WebSocket.OPEN) {
		return false;
	}
	// 生成通话会话ID，贯穿 PC -> Android -> 后端，用于录音与跟进合并
	const callId =
		typeof crypto !== "undefined" && crypto.randomUUID
			? crypto.randomUUID()
			: `call_${Date.now()}_${Math.random().toString(36).slice(2)}`;
	socket.send(JSON.stringify({ type: "dial", mobile, cluesId, callId, simSlotIndex }));
	return callId;
}

/** 让当前已连接的 Android 客户端远程挂机。 */
export function sendHangup(callId: string, cluesId: string): boolean {
	if (!socket || socket.readyState !== WebSocket.OPEN) return false;
	socket.send(JSON.stringify({ type: "call_control", status: "hangup", callId, cluesId }));
	return true;
}

/** PC 关闭远程通话窗口时，同步关闭 Android 当前通话的跟进填写页。 */
export function sendCloseFollow(callId: string, cluesId: string): boolean {
	if (!socket || socket.readyState !== WebSocket.OPEN) return false;
	socket.send(JSON.stringify({ type: "call_control", status: "close_follow", callId, cluesId }));
	return true;
}
