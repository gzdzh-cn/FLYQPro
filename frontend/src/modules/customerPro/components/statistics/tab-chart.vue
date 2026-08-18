<template>
	<div class="tab-chart">
		<div class="tab-chart__header">
			<ul class="tab-chart__tab">
				<li class="active">线索数</li>
				<!-- <li>访问量</li> -->
			</ul>

			<span class="tab-chart__year">上周线索-柱状图</span>
		</div>

		<div class="tab-chart__container">
			<v-chart :option="chartOption" autoresize />
		</div>
	</div>
</template>

<script lang="ts" setup>
import { computed, reactive, watch } from "vue";

const props = defineProps({
	data: Object
});

const weekData = computed(() => props.data);

const barWidth = 15;

const chartOption = reactive<any>({
	grid: {
		top: "20px",
		bottom: "30px",
		right: "10px",
		containLabel: true
	},
	xAxis: {
		type: "category",
		data: [],
		offset: 5,
		axisLine: {
			show: false
		},
		axisTick: {
			show: false
		}
	},
	yAxis: {
		type: "value",
		offset: 20,
		splitLine: {
			show: false
		},
		axisTick: {
			show: false
		},
		axisLine: {
			show: false
		}
	},
	tooltip: {
		trigger: "axis",
		formatter: (comp: any) => {
			const [serie] = comp;

			return `${serie.seriesName}：${serie.value}`;
		},
		axisPointer: {
			show: true,
			status: "shadow",
			z: -1,
			shadowStyle: {
				color: "#E6F7FF"
			},
			type: "shadow"
		},
		extraCssText: "width:120px; white-space:pre-wrap"
	},
	series: [
		{
			barWidth,
			name: "线索数量",
			type: "bar",
			data: [],
			itemStyle: {
				color: "#4165d7"
			}
		},
		{
			type: "bar",
			barWidth,
			xAxisIndex: 0,
			barGap: "-100%",
			data: [],
			itemStyle: {
				color: "#f1f1f9"
			},
			zlevel: -1
		}
	]
});

chartOption.xAxis.data = new Array(7).fill(1).map((e, i) => {
	let day = i + 1;
	switch (day) {
		case 1:
			return "周一";
		case 2:
			return "周二";
		case 3:
			return "周三";
		case 4:
			return "周四";
		case 5:
			return "周五";
		case 6:
			return "周六";
		case 7:
			return "周日";

		default:
			break;
	}

	return "周" + (i + 1);
});
// chartOption.series[0].data = new Array(7).fill(1).map(() => parseInt(String(Math.random() * 5000)));

chartOption.series[1].data = new Array(7).fill(5000);

console.log("props.data", props.data);

watch(
	() => props.data,
	(val) => {
		chartOption.series[0].data = props.data && props.data.map((e: { count: any }) => e.count);
		console.log("chartOption.series[0]", chartOption.series[0].data);
	},
	{
		immediate: true
	}
);
</script>

<style lang="scss" scoped>
.tab-chart {
	&__header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		height: 50px;
		padding: 0 20px;

		ul {
			li {
				list-style: none;
				float: left;
				margin-right: 20px;
				font-size: 15px;
				color: #dbdbdb;
				cursor: pointer;

				&.active {
					color: #000;
					font-weight: bold;
				}
			}
		}
	}

	&__year {
		font-size: 14px;
		position: relative;

		&::before {
			display: block;
			content: "";
			height: 8px;
			width: 8px;
			border-radius: 8px;
			background-color: #000;
			position: absolute;
			left: -15px;
			top: 4px;
		}
	}

	&__container {
		height: 300px;
		padding: 0 15px;

		.echarts {
			height: 100%;
			width: 100%;
		}
	}
}
</style>
