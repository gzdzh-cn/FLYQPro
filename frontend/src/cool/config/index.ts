import dev from "./dev";
import prod from "./prod";

// 是否开发模式
export const isDev = import.meta.env.MODE === "development";

// 配置
export const config = {
	eps: true,
	// 项目信息
	app: {
		name: "dzhgo管理系统",
		logo: "/logo.png",
		version: "v0.0.93",
		copyright: "管理系统 @2026",

		// 菜单
		menu: {
			// 是否分组显示
			isGroup: false,
			// 自定义菜单列表
			list: []
		},

		// 路由
		router: {
			// 模式
			mode: "hash", //history
			// 转场动画
			transition: "slide",
			// 首页组件
			home: () => import("/$/demo/views/home/index.vue")
		},

		// 字体图标库
		iconfont: []
	},

	// 忽略规则
	ignore: {
		// 不显示请求进度条
		NProgress: ["/base/comm/upload", "/base/comm/uploadMode", "/base/sys/notice/page"],
		// 页面不需要登录验证
		token: ["/login", "/401", "/403", "/404", "/500", "/502"]
	},

	// 调试
	test: {
		token: "",
		mock: false,
		eps: true
	},

	// 当前环境
	...(isDev ? dev : prod)
};

export * from "./proxy";
