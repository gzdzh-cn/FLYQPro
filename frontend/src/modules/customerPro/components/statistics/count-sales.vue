<template>
	<div class="count-sales">
		<div class="card">
			<div class="card__header">
				<span class="label">总线索数</span>
				<span class="value">{{ saleCount?.total }}</span>
			</div>

			<div class="card__container">
				<ul class="count-sales__cop">
					<li>
						<span>周同比</span>

						<div :class="saleCount?.weekRate > 0 ? 'rise' : 'fall'">
							<i class="el-icon-bottom-right"></i>
							<span>{{ saleCount?.weekRate }}%</span>
						</div>
					</li>

					<li>
						<span>日同比</span>

						<div :class="saleCount?.dayRate > 0 ? 'rise' : 'fall'">
							<i class="el-icon-top-right"></i>
							<span>{{ saleCount?.dayRate }}%</span>
						</div>
					</li>
				</ul>
			</div>

			<div class="card__footer">
				<span class="label">日线索数</span>
				<span class="value">{{ saleCount?.dayCount }}</span>
			</div>
		</div>
	</div>
</template>

<script lang="ts" setup>
import { computed, ref } from "vue";

const props = defineProps({
	data: Object
});

const saleCount = computed(() => props.data);
</script>

<style lang="scss" scoped>
.count-sales {
	&__cop {
		display: flex;
		justify-content: space-between;
		align-items: center;
		height: 50px;

		li {
			display: flex;
			list-style: none;
			flex: 1;
			color: #d8d8d8;

			.fall,
			.rise {
				display: flex;
				align-items: center;
				margin-left: 10px;
			}

			.rise {
				color: #13ae7c;
			}

			.fall {
				color: #f21e37;
			}
		}
	}
}
</style>
