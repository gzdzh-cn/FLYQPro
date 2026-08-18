<template>
	<div class="captcha" @click.stop="refresh">
		<div v-if="svg" class="svg" v-html="svg" />
		<img v-else class="base64" :src="base64" alt="" />
	</div>
</template>

<script lang="ts" setup>
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { useCool } from "/@/cool";

const emit = defineEmits(["update:modelValue", "change"]);

const { service } = useCool();

// base64
const base64 = ref("");

// svg
const svg = ref("");

function refresh() {
	// 清空旧内容，确保连续点击时始终触发可见刷新。
	base64.value = "";
	svg.value = "";

	service.base.open
		.captcha({
			height: 40,
			width: 150
		})
		.then(({ captchaId, data }: any) => {
			if (data.includes(";base64,")) {
				base64.value = data;
				svg.value = "";
			} else {
				svg.value = data;
				base64.value = "";
			}

			emit("update:modelValue", captchaId);
			emit("change", {
				base64,
				svg,
				captchaId
			});
		})
		.catch((err) => {
			ElMessage.error(err.message);
		});
}

onMounted(() => {
	refresh();
});

defineExpose({
	refresh
});
</script>

<style lang="scss" scoped>
.captcha {
	height: 40px;
	width: 150px;
	cursor: pointer;
	background-color: #999;

	.svg {
		height: 100%;
		position: relative;
	}

	.base64 {
		height: 66px;
		width: 100%;
	}
}
</style>
