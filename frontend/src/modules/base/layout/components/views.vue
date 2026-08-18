<template>
	<div class="app-views">
		<router-view v-slot="{ Component }">
			<div style="height: 96%">
				<el-scrollbar>
					<transition :name="app.info.router.transition">
						<keep-alive :include="caches">
							<component :is="Component" :key="$route.path" />
						</keep-alive>
					</transition>
				</el-scrollbar>
			</div>
			<div class="preload__footer">
				<a v-text="copyright"> </a>
			</div>
		</router-view>

		<!-- 右侧固定浮动图标 -->
		<div class="floating-icon" @click="handleIconClick">
			<el-icon size="20">
				<chat-dot-round />
			</el-icon>
		</div>

		<!-- 通知弹窗 -->
		<cl-feedback ref="feedbackRef" />
	</div>
</template>

<script lang="ts" setup>
import { computed, ref } from "vue";
import { useBase } from "/$/base";
import { ChatDotRound } from "@element-plus/icons-vue";

const { process, app } = useBase();

const copyright = computed(() => app.info.copyright);

// 通知弹窗引用
const feedbackRef = ref();

// 生成缓存名称的函数
function generateCacheName(item: any) {
	return item.path.substring(1, item.path.length).replace(/\//g, "-");
}

// 缓存列表
const caches = computed(() => {
	const cacheList = process.list
		.filter((e) => e.meta?.keepAlive)
		.map((e) => generateCacheName(e));

	return cacheList;
});

// 处理图标点击事件
const handleIconClick = () => {
	// 调用 cl-notice 组件的 open 方法
	if (feedbackRef.value) {
		feedbackRef.value.open();
	}
};
</script>

<style lang="scss" scoped>
.app-views {
	flex: 1;
	overflow: hidden;
	margin: 0px 20px;
	// margin-bottom: 20px;
	height: 100%;
	width: calc(100% - 30px);
	box-sizing: border-box;
	border-radius: 3px;
	position: relative;

	:deep(.el-scrollbar__view) {
		height: 100%;
	}

	.slide-enter-active {
		position: absolute;
		top: 0;
		width: 100%;
		transition: all 0.4s ease-in-out 0.2s;
	}

	.slide-leave-active {
		position: absolute;
		top: 0;
		width: 100%;
		transition: all 0.4s ease-in-out;
	}

	.slide-enter-to {
		transform: translate3d(0, 0, 0);
		opacity: 1;
	}

	.slide-enter-from {
		transform: translate3d(-5%, 0, 0);
		opacity: 0;
	}

	.slide-leave-to {
		transform: translate3d(5%, 0, 0);
		opacity: 0;
	}

	.slide-leave-from {
		transform: translate3d(0, 0, 0);
		opacity: 1;
	}

	// 右侧固定浮动图标样式
	.floating-icon {
		position: fixed;
		right: 10px;
		bottom: 80px;
		width: 50px;
		height: 50px;
		background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		color: white;
		cursor: pointer;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		transition: all 0.3s ease;
		z-index: 1000;

		&:hover {
			transform: translateY(-2px);
			box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
		}

		&:active {
			transform: translateY(0);
		}
	}
}
</style>
