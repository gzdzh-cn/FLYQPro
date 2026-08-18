<template>
	<a-menu v-if="app.info.menu.isGroup" />

	<div class="app-topbar">
		<div class="app-topbar__collapse" :class="{
			unfold: !app.isFold
		}" @click="app.fold()">
			<el-icon>
				<operation />
			</el-icon>
		</div>

		<!-- 路由导航 -->
		<route-nav />

		<div class="flex1"></div>

		<!-- 全局插件任务进度：固定在设备状态左侧。 -->
		<div
			v-if="addonsTask.id"
			class="app-topbar__addon-task"
			:class="`is-${addonsTask.status}`"
			@click="addonsTask.visible = true"
		>
			<span class="dot"></span>
			<span>
				{{
					addonsTask.scope === "site"
						? addonsTask.operation === "backup"
							? "全站备份"
							: "全站恢复"
						: addonsTask.operation === "backup"
						? "插件备份"
						: "插件恢复"
				}}
			</span>
			<span>
				{{
					addonsTask.status === "completed"
						? "已完成"
						: addonsTask.status === "failed"
						? "失败"
						: addonsTask.status === "cancelled"
						? "已停止"
						: `${addonsTask.progress}%`
				}}
			</span>
		</div>

		<!-- 设备在线状态（同账号 Android 客户端） -->
		<li class="app-topbar__device" :class="dotClass" :title="deviceTip" @click="showDeviceInfo">
			<span class="dot"></span>
			<span class="text">{{ deviceText }}</span>
		</li>

		<!-- 通知置于设备状态右侧。 -->
		<ul class="app-topbar__tools">
			<li>
				<cl-notice ref="noticeRef" />
			</li>
		</ul>

		<!-- 设备信息弹窗 -->
		<el-dialog v-model="deviceDialogVisible" title="移动端设备信息" width="380px" append-to-body>
			<div v-if="deviceState.androidOnline" class="device-info">
				<div class="device-info__row">
					<span class="label">手机系统</span>
					<span class="value">{{ osText }}</span>
				</div>
				<div class="device-info__row">
					<span class="label">手机型号</span>
					<span class="value">{{ deviceState.deviceModel || "未知" }}</span>
				</div>
				<div class="device-info__row">
					<span class="label">设备唯一码</span>
					<span class="value">{{ deviceState.deviceId || "未知" }}</span>
				</div>
			</div>
			<div v-else class="device-info__offline">当前移动端设备离线，无法获取设备信息</div>
		</el-dialog>

		<el-dialog
			v-model="addonsTask.visible"
			:title="
				addonsTask.scope === 'site'
					? addonsTask.operation === 'backup'
						? '全站备份进度'
						: '全站恢复进度'
					: addonsTask.operation === 'backup'
					? '插件备份进度'
					: '插件恢复进度'
			"
			width="460px"
			:close-on-click-modal="false"
			:close-on-press-escape="false"
			append-to-body
		>
			<el-progress
				:percentage="addonsTask.progress"
				:status="addonsTask.status === 'failed' ? 'exception' : undefined"
			/>
			<p class="addon-task-message">{{ addonsTask.message }}</p>
			<p v-if="addonsTask.error" class="addon-task-error">{{ addonsTask.error }}</p>
			<div v-if="addonsTask.errors.length" class="addon-task-errors">
				<div class="addon-task-errors__title">失败记录</div>
				<div v-for="item in addonsTask.errors" :key="`${item.fileName}-${item.time}`">
					{{ item.tableName }}：{{ item.error }}
				</div>
			</div>
			<template #footer>
				<el-button
					v-if="!addonsTask.isFinished()"
					type="danger"
					:loading="addonsTask.status === 'cancelling'"
					@click="cancelAddonTask"
					>停止{{ addonsTask.operation === "restore" ? "恢复" : "备份" }}</el-button
				>
				<el-button @click="addonsTask.visible = false">
					{{ addonsTask.isFinished() ? "关闭" : "最小化" }}
				</el-button>
			</template>
		</el-dialog>

		<!-- 用户信息 -->
		<div class="app-topbar__user" v-if="user.info">
			<el-dropdown trigger="click" hide-on-click @command="onCommand">
				<span class="el-dropdown-link">
					<img class="avatar" :src="user.info.headImg" />
					<span class="name">{{ user.info.nickName }}</span>
				</span>

				<template #dropdown>
					<el-dropdown-menu>
						<el-dropdown-item command="my">
							<i class="cl-iconfont cl-icon-user"></i>
							<span>个人中心</span>
						</el-dropdown-item>
						<el-dropdown-item command="exit">
							<i class="cl-iconfont cl-icon-exit"></i>
							<span>退出</span>
						</el-dropdown-item>
					</el-dropdown-menu>
				</template>
			</el-dropdown>
		</div>
	</div>
</template>

<script lang="ts" name="topbar" setup>
import { useBase } from "/$/base";
import { useCool } from "/@/cool";
import RouteNav from "./route-nav.vue";
import AMenu from "./amenu.vue";
import { Operation } from "@element-plus/icons-vue";
import { computed, onMounted, ref } from "vue";
import { deviceState } from "/@/utils/deviceWs";
import { ElDialog, ElMessageBox } from "element-plus";

const { router, service } = useCool();
const { user, app, addonsTask } = useBase();
const noticeRef = ref<any>(null);

// 设备在线状态：仅由移动端是否发来在线信号（androidOnline）决定。
// PC 端 WS 连接过程不对外暴露连接中等中间态，连不上即一直显示「离线中」。
const dotClass = computed(() => {
	return deviceState.androidOnline ? "is-online" : "is-offline";
});
const deviceText = computed(() => {
	return deviceState.androidOnline ? "客户端已上线" : "离线中";
});
const osText = computed(() => {
	switch (deviceState.os) {
		case "harmony":
			return "鸿蒙 (HarmonyOS)";
		case "ios":
			return "iOS";
		case "android":
			return "Android";
		default:
			return deviceState.os ? deviceState.os : "未知";
	}
});
const deviceTip = "同账号 Android 客户端在线状态，点击查看设备详情";

// 设备信息弹窗
const deviceDialogVisible = ref(false);
function showDeviceInfo() {
	deviceDialogVisible.value = true;
}

async function cancelAddonTask() {
	const isRestore = addonsTask.operation === "restore";
	try {
		await ElMessageBox.confirm(
			isRestore
				? "停止后，已执行的恢复数据不会自动回滚，可能保留部分恢复结果，是否继续？"
				: "停止后会删除本次未完成的备份文件，是否继续？",
			isRestore ? "停止恢复" : "停止备份",
			{ type: "warning" }
		);
		await addonsTask.cancel();
	} catch {
		// User cancelled the stop action.
	}
}
// 跳转
async function onCommand(name: string) {
	switch (name) {
		case "my":
			router.push("/my/info");
			break;
		case "exit":
			await service.base.comm.logout();
			user.logout();
			break;
	}
}

onMounted(() => {
	// noticeRef.value.refresh();
	// 当前 Wails 版本未启用 customer_pro 后端插件，不建立对应的 App WebSocket。
	addonsTask.init();
});
</script>

<style lang="scss" scoped>
.app-topbar {
	display: flex;
	align-items: center;
	height: 50px;
	padding: 0 10px;
	background-color: #fff;
	//margin-bottom: 10px;

	&__collapse {
		display: flex;
		justify-content: center;
		align-items: center;
		height: 40px;
		width: 40px;
		cursor: pointer;
		transform: rotateY(180deg);

		&.unfold {
			transform: rotateY(0);
		}

		i {
			font-size: 20px;
		}
	}

	.flex1 {
		flex: 1;
	}

	&__tools {
		display: flex;
		margin-right: 20px;

		&>li {
			display: flex;
			justify-content: center;
			align-items: center;
			list-style: none;
			height: 45px;
			min-width: 45px;
			border-radius: 3px;
			cursor: pointer;
			margin-left: 10px;

			&:hover {
				background-color: rgba(0, 0, 0, 0.1);
			}
		}
	}

	&__addon-task {
		display: flex;
		align-items: center;
		gap: 5px;
		height: 30px;
		padding: 0 12px;
		margin-right: 8px;
		border-radius: 16px;
		background: var(--el-color-primary-light-9);
		color: var(--el-color-primary);
		font-size: 12px;
		white-space: nowrap;
		cursor: pointer;

		.dot {
			width: 7px;
			height: 7px;
			border-radius: 50%;
			background: currentcolor;
		}

		&.is-completed {
			background: var(--el-color-success-light-9);
			color: var(--el-color-success);
		}

		&.is-failed {
			background: var(--el-color-danger-light-9);
			color: var(--el-color-danger);
		}

		&.is-cancelled,
		&.is-cancelling {
			background: var(--el-color-warning-light-9);
			color: var(--el-color-warning);
		}
	}

	&__user {
		margin-right: 10px;
		cursor: pointer;

		.el-dropdown-link {
			display: flex;
			align-items: center;
		}

		.avatar {
			height: 22px;
			width: 22px;
			border-radius: 22px;
		}

		.name {
			white-space: nowrap;
			margin-left: 15px;
		}

		.el-icon-arrow-down {
			margin-left: 10px;
		}
	}

	// 设备在线状态指示器
	&__device {
		display: flex;
		align-items: center;
		height: 45px;
		margin-right: 16px;
		padding: 0 12px;
		border-radius: 22px;
		background: rgba(0, 0, 0, 0.04);
		cursor: pointer;
		user-select: none;

		.dot {
			width: 8px;
			height: 8px;
			border-radius: 50%;
			margin-right: 8px;
			transition: background-color 0.3s;
		}

		.text {
			font-size: 13px;
			color: #606266;
			white-space: nowrap;
		}

		&.is-online .dot {
			background-color: #19be6b;
			box-shadow: 0 0 0 3px rgba(25, 190, 107, 0.18);
		}

		&.is-offline .dot {
			background-color: #c0c4cc;
		}

		&.is-connecting .dot {
			background-color: #e6a23c;
			animation: device-breathe 1.2s ease-in-out infinite;
		}

		@media (max-width: 768px) {
			padding: 0 8px;

			.text {
				display: none;
			}
		}
	}
}

@keyframes device-breathe {
	0%,
	100% {
		opacity: 1;
	}
	50% {
		opacity: 0.35;
	}
}

.device-info {
	&__row {
		display: flex;
		align-items: center;
		padding: 10px 0;
		border-bottom: 1px solid #f0f0f0;

		.label {
			width: 96px;
			color: #909399;
			font-size: 13px;
			flex-shrink: 0;
		}

		.value {
			flex: 1;
			color: #303133;
			font-size: 13px;
			word-break: break-all;
		}
	}

	&__offline {
		padding: 12px 0;
		color: #909399;
		font-size: 13px;
		text-align: center;
	}
}

.addon-task-message {
	margin: 18px 0 0;
	color: var(--el-text-color-regular);
	text-align: left;
	white-space: pre-line;
	line-height: 1.8;
	word-break: break-all;
}
.addon-task-error {
	margin: 10px 0 0;
	color: var(--el-color-danger);
	word-break: break-all;
}
</style>
