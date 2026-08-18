<template>
	<div class="app-process">
		<div class="app-process__icon" :class="{ active: homeActive }" @click="toHome">
			<el-icon size="15" color="#8d8989"><home-filled /></el-icon>
			<span>首页</span>
		</div>

		<!-- <div class="app-process__icon" @click="router.back">
			<el-icon :size="15"><arrow-left-bold /></el-icon>
			<span>后退</span>
		</div> -->

		<el-scrollbar :ref="setRefs('scroller')" class="app-process__scroller">
			<div
				v-for="(item, index) in process.list"
				:key="index"
				:ref="setRefs(`item-${index}`)"
				class="app-process__item"
				:class="{ active: item.active }"
				:data-index="index"
				@click="onTap(item, Number(index))"
				@contextmenu.stop.prevent="openCM($event, item)"
			>
				<span>{{ item.meta?.label || item.name || item.path }}</span>
				<el-icon @mousedown.stop="onDel(Number(index))">
					<close />
				</el-icon>
			</div>
		</el-scrollbar>
	</div>
</template>

<script lang="ts" setup>
import { watch, ref } from "vue";
import { last } from "lodash-es";
import { useCool } from "/@/cool";
import { Close, HomeFilled } from "@element-plus/icons-vue";
import { ContextMenu } from "@cool-vue/crud";
import { useBase } from "/$/base";
import { Process } from "/$/base/types";

const { refs, setRefs, route, router } = useCool();
const { process } = useBase();
const homeActive = ref(false);

// 跳首页
function toHome() {
	if (process.list.length > 0) {
		homeActive.value = true;
	}

	router.push("/");
}

// 跳转
function toPath() {
	const d = process.list.find((e) => e.active);

	if (!d) {
		const next = last(process.list);
		router.push(next ? next.fullPath : "/");
	}
}

// 移动到
function scrollTo(left: number) {
	refs.scroller.scrollTo({
		left,
		behavior: "smooth"
	});
}

// 调整滚动位置
function adScroll(index: number) {
	const el = refs[`item-${index}`];

	if (el) {
		scrollTo(el.offsetLeft + el.clientWidth - refs.scroller.clientWidth);
	}
}

// 选择
function onTap(item: Process.Item, index: number) {
	adScroll(index);
	router.push(item.fullPath);
}

// 删除
function onDel(index: number) {
	process.remove(index);
	toPath();
}

// 右键菜单
function openCM(e: any, item: Process.Item) {
	ContextMenu.open(e, {
		list: [
			{
				label: "关闭当前",
				hidden: item.fullPath !== route.path,
				callback(done) {
					onDel(process.list.findIndex((e) => e.fullPath == item.fullPath));
					done();
					toPath();
				}
			},
			{
				label: "关闭其他",
				callback(done) {
					process.set(process.list.filter((e) => e.fullPath == item.fullPath));
					done();
					toPath();
				}
			},
			{
				label: "关闭所有",
				callback(done) {
					process.clear();
					done();
					toPath();
				}
			}
		]
	});
}

watch(
	() => route.path,
	function (val) {
		if (val != "/") {
			homeActive.value = false;
		}
		adScroll(process.list.findIndex((e) => e.fullPath === val) || 0);
	}
);
</script>

<style lang="scss" scoped>
.app-process {
	display: flex;
	align-items: center;
	height: 5px;
	position: relative;
	margin-bottom: 10px;
	padding: 20px 10px;
	border: 1px #f0e8e8 solid;
	background-color: #fff;

	&__icon {
		position: relative;
		display: inline-flex;
		align-items: center;
		border-radius: 3px;
		height: 30px;
		line-height: 30px;
		padding: 0 10px;
		background-color: #fff;
		font-size: 14px;
		margin-right: 5px;
		color: #909399;
		cursor: pointer;
		border: 1px #f0e8e8 solid;

		span {
			font-size: 14px;
		}

		.el-icon {
			margin-right: 2px;
		}

		// &:hover {
		// 	background-color: #eee;
		// }
		&::before {
			content: "";
			position: absolute;
			left: 0;
			bottom: 0;
			width: 100%;
			height: 2px; /* 底边的高度 */
			background: var(--el-color-primary);
			transform: scaleX(0); /* 初始时隐藏 */
			transform-origin: 0 0; /* 从左上角开始变换 */
			transition: transform 0.3s ease-in-out; /* 过渡效果，可以根据需要调整 */
		}
		&:hover::before {
			transform: scaleX(1); /* 悬浮时展开 */
		}

		&.active {
			span {
				color: var(--color-primary);
				font-weight: bold;
				user-select: none;
			}

			i {
				width: auto;
				margin-left: 5px;
			}

			&:before {
				background-color: var(--color-primary);
			}
		}

		&.active::before {
			content: "";
			position: absolute;
			left: 0;
			bottom: 0;
			width: 100%;
			height: 2px; /* 底边的高度 */
			background: var(--el-color-primary);
			transform: scaleX(1); /* 初始时隐藏 */
			transform-origin: 0 0; /* 从左上角开始变换 */
			transition: transform 0.3s ease-in-out; /* 过渡效果，可以根据需要调整 */
		}
	}

	&__scroller {
		height: 32px;
		width: 100%;
		flex: 1;
		white-space: nowrap;
	}

	&__item {
		position: relative;
		display: inline-flex;
		align-items: center;
		border-radius: 3px;
		height: 30px;
		line-height: 30px;
		padding: 0 10px;
		background-color: #fff;
		font-size: 14px;
		margin-right: 5px;
		color: #909399;
		cursor: pointer;
		border: 1px #f0e8e8 solid;

		&:last-child {
			margin-right: 0;
		}

		i {
			font-size: 14px;
			width: 0;
			overflow: hidden;
			transition: all 0.3s;

			&:hover {
				color: #fff;
				background-color: var(--color-primary);
			}
		}

		.app-process__active {
			position: absolute;
			bottom: 0;
			left: 0;
			width: 0;
			height: 2px;
			opacity: 0;
			background: var(--el-color-primary);
		}
		&::before {
			content: "";
			position: absolute;
			left: 0;
			bottom: 0;
			width: 100%;
			height: 2px; /* 底边的高度 */
			background: var(--el-color-primary);
			transform: scaleX(0); /* 初始时隐藏 */
			transform-origin: 0 0; /* 从左上角开始变换 */
			transition: transform 0.3s ease-in-out; /* 过渡效果，可以根据需要调整 */
		}
		&:hover::before {
			transform: scaleX(1); /* 悬浮时展开 */
		}

		&:hover {
			&:not(.active) {
				background-color: #eee;
			}

			.el-icon-close {
				width: 14px;
				margin-left: 5px;
			}
		}

		&.active {
			span {
				color: var(--color-primary);
				font-weight: bold;
				user-select: none;
			}

			i {
				width: auto;
				margin-left: 5px;
			}

			&:before {
				background-color: var(--color-primary);
			}
		}

		&.active::before {
			content: "";
			position: absolute;
			left: 0;
			bottom: 0;
			width: 100%;
			height: 2px; /* 底边的高度 */
			background: var(--el-color-primary);
			transform: scaleX(1); /* 初始时隐藏 */
			transform-origin: 0 0; /* 从左上角开始变换 */
			transition: transform 0.3s ease-in-out; /* 过渡效果，可以根据需要调整 */
		}
	}
}
</style>
