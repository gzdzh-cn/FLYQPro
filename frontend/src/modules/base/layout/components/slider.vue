<template>
	<div class="app-slider">
		<div class="app-slider__logo" @click="toHome">
			<img class="avatar" :src="logo" alt="Logo" />
			<span v-if="!app.isFold || browser.isMini">{{ siteName }}</span>
		</div>

		<div class="app-slider__container">
			<b-menu />
		</div>
	</div>
</template>

<script lang="ts" setup>
import { useBase } from "/$/base";
import { useBrowser } from "/@/cool";
import BMenu from "./bmenu";
import { onMounted, ref, watch } from "vue";

const { browser } = useBrowser();
const { app } = useBase();

const logo = ref(app.info.logo);
const siteName = ref(app.info.name);

function toHome() { }

// 监听 app.info 变化
watch(
	() => app.info,
	(newInfo) => {
		logo.value = newInfo.logo;
		siteName.value = newInfo.name;
	},
	{ deep: true }
);

onMounted(() => {
	// 初始化时设置值
	// logo.value = setting.setting.logo || app.info.logo;
	// siteName.value = setting.setting.siteName || app.info.name;
});
</script>

<style lang="scss">
.app-slider {
	height: 100%;
	box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
	background-color: #2f3447;
	border: 0px #f0e8e8 solid;

	&__logo {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 50px;
		cursor: pointer;

		img {
			// height: 30px;
			width: 50px;
		}

		span {
			color: var(--color-primary);
			font-weight: bold;
			font-size: 18px;
			margin-left: 10px;
			font-family: inherit;
			white-space: nowrap;
		}
	}

	&__container {
		height: calc(100% - 80px);
		overflow-y: auto;

		&::-webkit-scrollbar {
			width: 0;
			height: 0;
		}
	}

	&__menu {
		&.el-popper {
			&.is-light {
				border: 0;
			}
		}

		.el-menu {
			border-right: 0;
			background-color: transparent;

			&--popup {

				.cl-svg,
				span {
					color: #000;
				}
			}

			.el-sub-menu__title,
			&-item {

				&.is-active,
				&:hover {
					background-color: var(--color-primary) !important;

					.cl-svg,
					span {
						color: #fff;
					}
				}
			}

			.el-sub-menu__title,
			&-item,
			&__title {
				color: #eee;
				letter-spacing: 0.5px;
				height: 50px;
				line-height: 50px;

				.wrap {
					width: 100%;

					&--vertical {
						display: flex;
						flex-direction: column;
						align-items: center;
						justify-content: center;
						height: 100%;
						padding: 4px 0;
						box-sizing: border-box;

						.cl-svg {
							margin-bottom: 4px;
						}

						span {
							margin-left: 0;
							font-size: 11px;
							line-height: 1.2;
							white-space: nowrap;
							overflow: hidden;
							text-overflow: ellipsis;
							max-width: 100%;
						}
					}
				}

				.cl-svg {
					font-size: 16px;
				}

				span {
					display: inline-block;
					font-size: 14px;
					letter-spacing: 1px;
					margin-left: 10px;
					user-select: none;
				}
			}

			&--collapse {
				.wrap {
					text-align: center;

					.cl-svg {
						font-size: 18px;
					}

					&--active {
						background-color: var(--color-primary) !important;
						border-radius: 4px;

						.cl-svg,
						span {
							color: #fff !important;
						}
					}
				}

				.el-sub-menu__title {
					display: flex !important;
					align-items: center !important;
					justify-content: center !important;
					line-height: normal !important;
					padding: 0 !important;

					.wrap--vertical {
						display: flex;
						flex-direction: column;
						align-items: center;
						justify-content: center;
						height: 100%;
						gap: 2px;
					}

					span {
						display: block !important;
						visibility: visible !important;
						height: auto !important;
						width: auto !important;
						overflow: visible !important;
						position: static !important;
						margin: 0 !important;
						padding: 0 !important;
					}
				}
			}
		}
	}

	// 缩进模式：只显示一级菜单
	&__menu--fold {
		display: flex;
		height: 100%;

		.menu-level1 {
			width: 65px;
			min-width: 65px;
			background-color: rgba(0, 0, 0, 0.2);
			overflow-y: auto;

			&::-webkit-scrollbar {
				width: 0;
				height: 0;
			}

			&-item {
				display: flex;
				flex-direction: column;
				align-items: center;
				justify-content: center;
				padding: 10px 4px;
				cursor: pointer;
				color: #eee;
				transition: background-color 0.2s;

				&:hover {
					background-color: rgba(255, 255, 255, 0.1);
				}

				&.is-active {
					background-color: var(--color-primary);

					.cl-svg,
					span {
						color: #fff;
					}
				}

				.cl-svg {
					font-size: 18px;
					margin-bottom: 4px;
				}

				span {
					font-size: 11px;
					line-height: 1.2;
					text-align: center;
					white-space: nowrap;
					overflow: hidden;
					text-overflow: ellipsis;
					max-width: 100%;
				}
			}
		}
	}
}
</style>
