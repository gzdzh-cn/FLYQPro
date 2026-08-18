import { ModuleConfig } from "/@/cool";
import { useCrm } from "./index";

export default (): ModuleConfig => {
	return {
		onLoad({ hasToken }) {
			const { crm } = useCrm();

			hasToken(() => {
				// crm.refresh();
			});
		}
	};
};
