import { defineStore } from "pinia";
import { ref } from "vue";
import { config } from "/@/cool";
import { useUserStore } from "./user";

const storageKey = "base-addons-active-task";
const apiPrefix = "/admin/base/sys/addons";

export const useAddonsTaskStore = defineStore("addons-task", () => {
	const id = ref("");
	const operation = ref("");
	const scope = ref("addon");
	const status = ref("");
	const progress = ref(0);
	const message = ref("");
	const error = ref("");
	const errors = ref<any[]>([]);
	const visible = ref(false);
	let streamAbort: AbortController | undefined;
	let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	let completionTimer: ReturnType<typeof setTimeout> | undefined;

	function isFinished() {
		return ["completed", "failed", "cancelled"].includes(status.value);
	}

	function applyEvent(task: any) {
		operation.value = task.operation || operation.value;
		scope.value = task.scope || scope.value;
		status.value = task.status || status.value;
		progress.value = Number(task.progress || 0);
		message.value = task.message || "任务执行中";
		error.value = task.error || "";
		errors.value = task.errors || [];
		if (isFinished()) {
			sessionStorage.removeItem(storageKey);
		}
		if (
			status.value === "completed" &&
			progress.value >= 100 &&
			(operation.value === "backup" ||
				(operation.value === "restore" && errors.value.length === 0))
		) {
			scheduleCompletedDismiss(task.taskId || id.value);
		}
	}

	function scheduleCompletedDismiss(taskId: string) {
		if (completionTimer) clearTimeout(completionTimer);
		completionTimer = setTimeout(() => {
			if (id.value !== taskId) {
				return;
			}
			id.value = "";
			visible.value = false;
			completionTimer = undefined;
		}, 5000);
	}

	async function connect(taskId: string) {
		if (reconnectTimer) clearTimeout(reconnectTimer);
		streamAbort?.abort();
		streamAbort = new AbortController();
		const { token } = useUserStore();
		try {
			const response = await fetch(
				`${config.baseUrl}${apiPrefix}/taskSse?taskId=${encodeURIComponent(taskId)}`,
				{
					headers: { Accept: "text/event-stream", Authorization: token || "" },
					signal: streamAbort.signal
				}
			);
			const contentType = response.headers.get("content-type") || "";
			if (!response.ok || !response.body || !contentType.includes("text/event-stream")) {
				sessionStorage.removeItem(storageKey);
				id.value = "";
				throw new Error("进度服务连接失败");
			}
			const reader = response.body.getReader();
			const decoder = new TextDecoder();
			let buffer = "";
			while (true) {
				const { done, value } = await reader.read();
				if (done) break;
				buffer += decoder.decode(value, { stream: true });
				const events = buffer.split("\n\n");
				buffer = events.pop() || "";
				events.forEach((event) => {
					const dataLine = event.split("\n").find((line) => line.startsWith("data: "));
					if (dataLine) {
						try {
							applyEvent(JSON.parse(dataLine.slice(6)));
						} catch {
							// Ignore heartbeat and incomplete events.
						}
					}
				});
			}
			if (isFinished()) return;
		} catch (e: any) {
			if (e?.name === "AbortError") return;
			error.value = e?.message || "进度服务暂时断开，正在重连";
		}
		if (id.value === taskId && !isFinished()) {
			reconnectTimer = setTimeout(() => connect(taskId), 1500);
		}
	}

	function start(taskId: string, taskOperation: string, taskScope = "addon") {
		if (completionTimer) clearTimeout(completionTimer);
		id.value = taskId;
		operation.value = taskOperation;
		scope.value = taskScope;
		status.value = "pending";
		progress.value = 0;
		message.value = "任务已创建";
		error.value = "";
		errors.value = [];
		visible.value = true;
		sessionStorage.setItem(storageKey, taskId);
		connect(taskId);
	}

	function init() {
		const taskId = sessionStorage.getItem(storageKey);
		if (taskId && !id.value) {
			id.value = taskId;
			message.value = "正在恢复任务进度…";
			connect(taskId);
		}
	}

	async function cancel() {
		if (!id.value || status.value === "cancelling" || isFinished()) return;
		const { token } = useUserStore();
		const response = await fetch(`${config.baseUrl}${apiPrefix}/taskCancel`, {
			method: "POST",
			headers: { "Content-Type": "application/json", Authorization: token || "" },
			body: JSON.stringify({ taskId: id.value })
		});
		if (!response.ok) throw new Error("停止任务请求失败");
		const result = await response.json().catch(() => null);
		if (result?.code && result.code !== 1000) {
			throw new Error(result.message || "停止任务请求失败");
		}
		status.value = "cancelling";
		message.value = `正在停止${operation.value === "restore" ? "恢复" : "备份"}任务…`;
	}

	return {
		id,
		operation,
		scope,
		status,
		progress,
		message,
		error,
		errors,
		visible,
		isFinished,
		start,
		init,
		cancel
	};
});
