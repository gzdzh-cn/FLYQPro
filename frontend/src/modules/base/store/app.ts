import { defineStore } from "pinia";
import { reactive, ref } from "vue";
import { config, useBrowser } from "/@/cool";
import { deepMerge, storage } from "/@/cool/utils";

export const useAppStore = defineStore("app", function () {
	const { browser, onScreenChange } = useBrowser();

	// 基本信息
	const info = reactive({
		...config.app
	});

	// 是否折叠
	const isFold = ref(true);

	// 事件
	const events = reactive<{ [key: string]: any[] }>({
		hasToken: []
	});

	// 折叠
	function fold(v?: boolean) {
		if (v === undefined) {
			v = !isFold.value;
		}

		isFold.value = v;
	}

	// async function getSetting() {
	// 	// 延迟获取service，避免在store初始化时调用useCool
	// 	const { useCool } = await import("/@/cool");
	// 	const { service } = useCool();

	// 	const result = await service.base.open.getSetting();
	// 	if (result.siteName) {
	// 		info.name = result.siteName;
	// 	}
	// 	if (result.logo) {
	// 		info.logo = result.logo;
	// 	}
	// }

	// 设置基本信息
	async function set(data: any) {
		deepMerge(info, data);
		storage.set("__app__", info);
	}

	// 添加事件
	function addEvent(name: string, func: any) {
		if (func) {
			events[name].push(func);
		}
	}

	// 监听屏幕变化
	onScreenChange(() => {
		// isFold.value = browser.isMini;
	});

	return {
		info,
		isFold,
		fold,
		events,
		set,
		addEvent
		// getSetting
	};
});
