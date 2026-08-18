<template>
	<div class="sub-track">
		<div v-if="!trackList || trackList.length === 0" class="track-empty">
			暂无轨迹记录
		</div>
		<el-steps :active="trackList.length" direction="vertical" v-else>
			<el-step
				:icon="Flag"
				style="margin: 0 0 10px 0"
				v-for="(item, index) in trackList"
				:key="index"
			>
				<template #title>
					<h4>{{ item.typeName }}</h4>
				</template>
				<template #description>
					<div>
						<p style="font-weight: 400">
							<span v-html="item.remark"></span>
						</p>
						<p style="font-weight: 200">
							{{ item.createTime }}「{{ item.nickName ? item.nickName : "-" }}」
						</p>
					</div>
				</template>
			</el-step>
		</el-steps>
	</div>
</template>

<script lang="ts" name="customeer_pro-subTrack" setup>
import { useCool } from "/@/cool";
import { Flag } from "@element-plus/icons-vue";
import { onMounted, ref, watch } from "vue";

const props = defineProps<{
	id?: string;
}>();

const { service } = useCool();

const trackList = ref();
const getTrack = async () => {
	if (!props.id) return;
	trackList.value = (await service.customer_pro.clues.getTrackList({ cluesId: props.id })).reverse();
};

onMounted(() => {
	getTrack();
});

watch(() => props.id, () => {
	getTrack();
});

defineExpose({ refresh: getTrack });
</script>

<style lang="scss" scoped>
.sub-track {
	height: 100%;
	overflow-y: auto;
	padding: 0 16px 16px;

	.track-empty {
		text-align: center;
		padding: 40px;
		color: #999;
	}

	:deep(.el-step) {
		flex-basis: auto !important;
	}

	:deep(.el-step__main) {
		padding-bottom: 4px;
	}

	:deep(.el-step__head) {
		padding-right: 10px;
	}

	:deep(.el-step__description) {
		color: #333;
		p {
			color: #333;
		}
	}

	:deep(.el-step__title) {
		color: #333;
	}
}
</style>
