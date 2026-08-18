import { useStore } from "./store";

export function useFileUpload() {
	return {
		...useStore()
	};
}
