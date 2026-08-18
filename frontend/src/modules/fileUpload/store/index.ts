import { useConfigStore } from "./config";

export function useStore() {
	const config = useConfigStore();
	return {
		config
	};
}
