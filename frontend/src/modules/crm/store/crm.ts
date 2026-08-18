import { defineStore } from "pinia";
import { computed, reactive } from "vue";
import { service } from "/@/cool";

declare interface Data {
	[key: string]: Array<{ label: string; value: any }>;
}

export const useCrmStore = defineStore("crm", () => {
	// 对象数据
	const data = reactive<Data>({});

	// 获取
	function get() {
		return computed(() => data["userList"]);
	}

	// 刷新
	async function refresh() {
		const res = await service.base.sys.user.list();
		data.userList = res;
		return data;
	}

	return {
		data,
		get,
		refresh
	};
});
