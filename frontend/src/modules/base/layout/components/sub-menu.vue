<template>
    <div class="app-sub-menu-wrap" v-if="app.isFold && !browser.isMini">
        <!-- 浮动吸附按钮：始终显示，位置随 collapsed 变化 -->
        <div class="app-sub-menu-toggle" :class="{ 'is-collapsed': collapsed }" @click="collapsed = !collapsed">
            <el-icon :size="14"><ArrowRight v-if="collapsed" /><ArrowLeft v-else /></el-icon>
        </div>

        <!-- 二级菜单展开时 -->
        <div class="app-sub-menu" v-show="!collapsed">
            <div class="app-sub-menu__header">
                <div class="app-sub-menu__title">
                    {{ activeItem?.name }}
                </div>
            </div>
            <div class="app-sub-menu__list">
                <menu-node v-for="item in children" :key="item.id" :item="item" :level="1" />
            </div>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch, provide } from "vue";
import { useStore } from "../../store";
import { useCool } from "/@/cool";
import { ArrowRight, ArrowLeft } from "@element-plus/icons-vue";
import MenuNode from "./menu-node.vue";

const { router, route, browser } = useCool();
const { menu, app } = useStore();

// 二级菜单折叠状态
const collapsed = ref(false);

// 展开的菜单ID列表
const expandedIds = ref<number[]>([]);

// 页面跳转
function toView(url: string) {
    if (url != route.path) {
        router.push(url);
    }
}

// 检查当前路由是否在菜单的子菜单中（递归）
function hasActiveChild(item: any): boolean {
    if (!item.children) return false;
    return item.children.some((child: any) => {
        if (child.path === route.path) return true;
        if (child.children) return hasActiveChild(child);
        return false;
    });
}

// 展开/收起菜单
function toggleExpand(id: number) {
    const idx = expandedIds.value.indexOf(id);
    if (idx >= 0) {
        expandedIds.value.splice(idx, 1);
    } else {
        expandedIds.value.push(id);
    }
}

// 当前选中的一级菜单
const activeItem = computed(() => {
    return menu.list[menu.foldActiveIndex];
});

// 当前一级菜单的子菜单
const children = computed(() => {
    return (activeItem.value?.children || []).filter((e) => e.isShow);
});

// 自动展开包含当前路由的菜单（递归）
function autoExpand() {
    const list = children.value;
    if (!list) return;
    expandActiveItems(list);
}

function expandActiveItems(list: any[]) {
    list.forEach((item: any) => {
        if (item.children && item.children.length > 0) {
            if (hasActiveChild(item)) {
                if (!expandedIds.value.includes(item.id)) {
                    expandedIds.value.push(item.id);
                }
                expandActiveItems(item.children.filter((e: any) => e.isShow));
            }
        }
    });
}

// 通过 provide 向递归子组件注入共享状态
provide("subMenuCtx", {
    expandedIds,
    toggleExpand,
    toView,
    route
});

// 监听路由变化，自动展开
watch(() => route.path, autoExpand, { immediate: true });

// 监听一级菜单切换，重置展开状态并自动展开
watch(() => menu.foldActiveIndex, () => {
    collapsed.value = false;
    expandedIds.value = [];
    autoExpand();
});
</script>

<style lang="scss">
.app-sub-menu-wrap {
    height: 100%;
    flex-shrink: 0;
    transition: width 0.2s ease-in-out;
    position: relative;
    align-self: stretch;
}

.app-sub-menu-toggle {
    width: 20px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: #fff;
    border: 1px solid #e4e7ed;
    border-left: none;
    border-radius: 0 4px 4px 0;
    cursor: pointer;
    color: #909399;
    position: fixed;
    top: 50vh;
    left: 204px;
    transform: translateY(-50%);
    box-shadow: 2px 0 4px rgba(0, 0, 0, 0.06);
    transition: left 0.2s ease-in-out;
    z-index: 100;

    &.is-collapsed {
        left: 64px;
    }

    &:hover {
        color: #409eff;
        background-color: #ecf5ff;
    }
}

.app-sub-menu {
    width: 140px;
    height: 100%;
    background-color: #fff;
    border-right: 1px solid #e4e7ed;
    overflow-y: auto;

    &::-webkit-scrollbar {
        width: 0;
        height: 0;
    }

    &__header {
        display: flex;
        align-items: center;
        border-bottom: 1px solid #e4e7ed;
    }

    &__title {
        padding: 12px;
        font-size: 18px;
        color: #303133;
        font-weight: bold;
        flex: 1;
    }

    &__list {
        padding: 8px 0;
    }

    &__item {
        display: flex;
        align-items: center;
        padding: 12px 16px;
        cursor: pointer;
        color: #606266;
        transition: all 0.2s;
        margin: 4px 8px;
        border-radius: 4px;

        &:hover {
            background-color: #f5f7fa;
            color: #409eff;
        }

        &.is-active {
            background-color: #ecf5ff;
            color: #409eff;

            .cl-svg {
                color: #409eff;
            }
        }

        &.is-nested {
            font-size: 13px;
            padding-top: 10px;
            padding-bottom: 10px;
        }

        .cl-svg {
            font-size: 16px;
            margin-right: 10px;
            flex-shrink: 0;
            color: #909399;
        }

        .arrow-icon {
            font-size: 12px;
            margin-left: auto;
            color: #c0c4cc;
            transition: transform 0.2s;

            &.is-rotated {
                transform: rotate(90deg);
            }
        }

        span {
            font-size: 14px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
    }

    &__sub-list {
        padding: 4px 0;
    }

    &__sub-item {
        padding: 10px 16px 10px 42px;
        cursor: pointer;
        color: #606266;
        transition: all 0.2s;
        margin: 2px 8px;
        border-radius: 4px;
        font-size: 13px;

        &:hover {
            background-color: #f5f7fa;
            color: #409eff;
        }

        &.is-active {
            background-color: #ecf5ff;
            color: #409eff;
        }
    }
}
</style>
