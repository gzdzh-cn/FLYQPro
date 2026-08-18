import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { service } from "/@/cool";
import { storage } from "/@/cool/utils";

// 本地缓存
const data = storage.info();

export const useSettingStore = defineStore("setting", () => {
	// 对象数据
	const setting = ref(data.setting);

	// 获取
	async function get() {
		const result = await service.base.open.getSetting();
		set(result);
		return computed(() => setting.value);
	}

	function set(v: any) {
		setting.value = v;
		storage.set("setting", v);
	}

	// 刷新
	async function refresh() {
		get();
		return setting;
	}

	return {
		setting,
		get,
		set,
		refresh
	};
});
