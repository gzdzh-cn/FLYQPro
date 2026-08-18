import { defineComponent, h, watch } from "vue";
import { useStore } from "../../store";
import { Menu } from "../../types";
import { useCool } from "/@/cool";

export default defineComponent({
	name: "b-menu",

	setup() {
		const { router, route, browser } = useCool();
		const { menu, app } = useStore();

		// 检查当前路由是否在菜单的子菜单中
		function hasActiveChild(item: Menu.Item): boolean {
			if (!item.children) return false;
			return item.children.some((child) => {
				if (child.path === route.path) return true;
				if (child.children) return hasActiveChild(child);
				return false;
			});
		}

		// 页面跳转
		function toView(url: string) {
			if (url != route.path) {
				router.push(url);
			}

			// 小屏下点击收起左侧菜单
			if (browser.isMini) {
				app.fold(true);
			}
		}

		// 初始化当前选中一级菜单
		function initActiveIndex() {
			if (!app.isFold || browser.isMini) return;
			const idx = menu.list.findIndex((e) => hasActiveChild(e));
			if (idx >= 0) {
				menu.foldActiveIndex = idx;
			}
		}

		// 监听路由变化，更新 foldActiveIndex
		watch(() => route.path, initActiveIndex, { immediate: true });

		// 渲染一级菜单（缩进模式下）
		function renderLevel1() {
			return menu.list
				.filter((e) => e.isShow)
				.map((e, index) => {
					const isActive = menu.foldActiveIndex === index;
					return h(
						"div",
						{
							key: e.id,
							class: ["menu-level1-item", isActive ? "is-active" : ""],
							onClick: () => {
								menu.foldActiveIndex = index;
							}
						},
						[
							h(<cl-svg name={e.icon} />),
							h("span", { class: "menu-label" }, e.name)
						]
					);
				});
		}

		// 原始递归渲染菜单（非缩进模式下）
		function renderMenu() {
			function deep(list: Menu.Item[], index: number) {
				return list
					.filter((e) => e.isShow)
					.map((e) => {
						let html = null;

						if (e.type == 0) {
							const isActive = app.isFold && index === 1 && hasActiveChild(e);
							html = h(
								<el-sub-menu />,
								{
									index: String(e.id),
									key: e.id,
									popperClass: "app-slider__menu",
									class: { "is-active-parent": isActive }
								},
								{
									title() {
										return (
											<div class={["wrap", app.isFold && index === 1 ? "wrap--vertical" : "", isActive ? "wrap--active" : ""]}>
												<cl-svg name={e.icon} />
												<span>
													{e.name}
												</span>
											</div>
										);
									},
									default() {
										return deep(e.children || [], index + 1);
									}
								}
							);
						} else {
							html = h(
								<el-menu-item />,
								{
									index: e.path,
									key: e.id
								},
								{
									default() {
										return (
											<div class="wrap">
												<cl-svg name={e.icon} />
												<span v-show={!app.isFold || index != 1}>
													{e.name}
												</span>
											</div>
										);
									}
								}
							);
						}

						return html;
					});
			}
			return deep(menu.list, 1);
		}

		return () => {
			// 缩进模式：只显示一级菜单
			if (app.isFold && !browser.isMini) {
				return (
					<div class="app-slider__menu app-slider__menu--fold">
						<div class="menu-level1">
							{renderLevel1()}
						</div>
					</div>
				);
			}

			// 非缩进模式：原有菜单
			return (
				<div class="app-slider__menu">
					<el-menu
						default-active={route.path}
						background-color="transparent"
						collapse-transition={true}
						collapse={browser.isMini ? false : app.isFold}
						onSelect={toView}
					>
						{renderMenu()}
					</el-menu>
				</div>
			);
		};
	}
});
