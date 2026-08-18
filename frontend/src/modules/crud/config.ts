import { Merge, ModuleConfig } from "/@/cool";
import Crud from "@cool-vue/crud";
import "@cool-vue/crud/dist/index.css";

// 自定义
// import { Crud } from "/~/crud/src";
// import "/~/crud/src/static/index.scss";

export default (): Merge<ModuleConfig, CrudOptions> => {
	return {
		options: {
			dict: {
				sort: {
					prop: "order",
					order: "sort"
				}
			}
		},
		install: Crud.install
	};
};
