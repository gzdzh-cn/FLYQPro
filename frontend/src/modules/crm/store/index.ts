import { useCrmStore } from "./crm";

export function useStore() {
	const crm = useCrmStore();
	return {
		crm
	};
}
