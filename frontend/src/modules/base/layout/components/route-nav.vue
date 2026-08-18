<template>
	<div class="route-nav">
		<p v-if="browser.isMini" class="title">{{ lastName }}</p>

		<p class="home" v-if="!lastName">控制台面板</p>

		<template v-if="!browser.isMini">
			<el-breadcrumb>
				<el-breadcrumb-item v-for="(item, index) in list" :key="index">{{
					item.meta?.label || item.name
				}}</el-breadcrumb-item>
			</el-breadcrumb>
		</template>
	</div>
</template>

<script lang="ts" name="route-nav" setup>
import { computed } from "vue";
import { flattenDeep, last } from "lodash-es";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
// import { ArrowRight } from "@element-plus/icons-vue";

const { route, browser } = useCool();
const { menu } = useBase();

// 数据列表
const list = computed(() => {
	function deep(item: any) {
		if (route.path === "/") {
			return false;
		}

		if (item.path == route.path) {
			return item;
		} else {
			if (item.children) {
				const ret = item.children.map(deep).find(Boolean);

				if (ret) {
					return [item, ret];
				} else {
					return false;
				}
			} else {
				return false;
			}
		}
	}

	return flattenDeep(menu.group.map(deep).filter(Boolean));
});

// 最后一个节点名称
const lastName = computed(() => last(list.value)?.name);
</script>

<style lang="scss" scoped>
.route-nav {
	white-space: nowrap;

	:deep(.el-breadcrumb) {
		margin: 0 10px;

		.el-breadcrumb__inner {
			font-size: 13px;
		}
	}
	.home {
		font-size: 14px;
		margin-left: 5px;
		color: #606266;
	}
	.title {
		font-size: 15px;
		font-weight: 500;
		margin-left: 5px;
	}
}
</style>
