import { useAppStore } from "./app";
import { useMenuStore } from "./menu";
import { useProcessStore } from "./process";
import { useUserStore } from "./user";
import { useSettingStore } from "./setting";
import { useAddonsTaskStore } from "./addons-task";

export function useStore() {
	const app = useAppStore();
	const menu = useMenuStore();
	const process = useProcessStore();
	const user = useUserStore();
	const setting = useSettingStore();
	const addonsTask = useAddonsTaskStore();

	return {
		app,
		menu,
		process,
		user,
		setting,
		addonsTask
	};
}
