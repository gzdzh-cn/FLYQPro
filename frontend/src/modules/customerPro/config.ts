import { ModuleConfig } from "/@/cool";
import { useBase } from "/$/base";
// import { useSettingStore } from "/$/base/store/setting";

export default (): ModuleConfig => {
	return {
		async onLoad() {
			const logo2 = "/customer_pro/logo2.png";
			const logo3 = "/demo.png";
			// 获取域名，如果域名是a，就是赋值为客户管理，否则赋值为默认标题
			if (
				window.location.hostname.includes("lingnan.gzlingnan.com")
			) {
				document.title = "学益教育";
				document.querySelector("link[rel='icon']").href = logo2;

				const { app } = useBase();
				// 更新 app info 中的 logo
				await app.set({
					logo: logo2,
					name: "学益教育",
					copyright: "学益教育 © 2026"
				});
			}

			if (
				window.location.hostname.includes("weichuang.gzlingnan.com") ||
				window.location.hostname.includes("127.0.0.1")
			) {
				document.title = "炜创科技";
				document.querySelector("link[rel='icon']").href = logo3;

				const { app } = useBase();
				// 更新 app info 中的 logo
				await app.set({
					logo: logo3,
					name: "炜创科技",
					copyright: "炜创科技 © 2026"
				});
			}
		}
	};
};
