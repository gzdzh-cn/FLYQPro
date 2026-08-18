import { useStore } from "./store";

export function useCrm() {
	return {
		...useStore()
	};
}
