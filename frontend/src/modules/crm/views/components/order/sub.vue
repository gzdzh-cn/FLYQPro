<template>
	<cl-view-group ref="ViewGroup" leftWidth="10%">
		<template #left>
			<sub-tree @refresh="refresh" :orderInfo="orderInfo" />
		</template>

		<template #right>
			<sub-order ref="subOrder" :orderInfo="orderInfo" />
		</template>
	</cl-view-group>
</template>

<script lang="ts" name="order-sub" setup>
import SubTree from "./subTree.vue";
import SubOrder from "./subOrder.vue";
import { useViewGroup } from "/$/base";
import { ref } from "vue";

const { ViewGroup } = useViewGroup();

const props = defineProps({
	orderInfo: Object
});

const subOrder = ref();
const refresh = (params: any) => {
	// params.id = props.orderInfo?.parentId;
	subOrder.value?.refresh(params);
};
</script>
