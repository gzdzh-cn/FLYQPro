import { defineStore } from "pinia";
import { storage } from "/@/cool/utils";
import { ref } from "vue";

// 本地缓存
const data = storage.info();
export const useConfigStore = defineStore("config", function () {
	const refresh = ref<boolean>(data["refresh"] || false);

	const setConfig = (v: boolean) => {
		refresh.value = v;
		storage.set("refresh", v);
	};

	return {
		refresh,
		setConfig
	};
});
