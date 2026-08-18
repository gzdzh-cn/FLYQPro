import { createPinia } from "pinia";
import mitt from "mitt";
import VueECharts from "vue-echarts";
import { App } from "vue";
import { createModule } from "./module";
import { createEps } from "./eps";
import { router } from "../router";
import ElementPlus from "element-plus";
import "element-plus/theme-chalk/src/index.scss";
import { Loading } from "../utils";
import zhCn from "element-plus/es/locale/lang/zh-cn";

export async function bootstrap(app: App) {
	// pinia
	app.use(createPinia());

	// element-plus
	app.use(ElementPlus, {
		locale: zhCn
	});

	// mitt
	app.provide("mitt", mitt());

	// charts
	app.component("v-chart", VueECharts);

	// 路由
	app.use(router);

	// 模块
	const { eventLoop } = createModule(app);

	// eps
	await createEps();

	// 加载
	Loading.set([eventLoop()]);
}
