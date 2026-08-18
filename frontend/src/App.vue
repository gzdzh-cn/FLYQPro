<template>
	<el-config-provider>
		<router-view />
	</el-config-provider>
</template>

<script lang="ts" setup>
import { ElConfigProvider } from "element-plus";

import { onMounted } from "vue";

onMounted(() => {});
</script>

<style lang="scss">
.dzh_popover {
	min-width: 80px !important;
}
.divider {
	width: 100%;
	height: 1px;
	margin: 15px 0;
	border-top: 1px dashed #e9e9e9;
}
</style>
