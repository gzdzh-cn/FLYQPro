<template>
    <!-- 有子菜单：可展开/收起 -->
    <div v-if="item.children && item.children.length > 0">
        <div class="app-sub-menu__item"
            :class="{ 'is-expanded': expandedIds.includes(item.id), 'is-nested': level > 1 }"
            :style="{ paddingLeft: (16 + (level - 1) * 14) + 'px' }"
            @click="toggleExpand(item.id)">
            <i v-if="item.icon" :class="['menu-icon', 'cl-svg', item.icon]" />
            <span class="menu-label">{{ item.name }}</span>
            <el-icon class="arrow-icon" :class="{ 'is-rotated': expandedIds.includes(item.id) }">
                <ArrowRight />
            </el-icon>
        </div>
        <!-- 子菜单列表 -->
        <div v-show="expandedIds.includes(item.id)" class="app-sub-menu__sub-list">
            <menu-node
                v-for="sub in item.children.filter((e: any) => e.isShow)"
                :key="sub.id"
                :item="sub"
                :level="level + 1"
            />
        </div>
    </div>
    <!-- 无子菜单：直接跳转 -->
    <div v-else class="app-sub-menu__sub-item"
        :class="{ 'is-active': route.path === item.path }"
        :style="{ paddingLeft: (16 + level * 14) + 'px' }"
        @click="toView(item.path)">
        <span class="menu-label">{{ item.name }}</span>
    </div>
</template>

<script lang="ts" setup>
import { inject } from "vue";
import { ArrowRight } from "@element-plus/icons-vue";

defineProps<{
    item: any;
    level: number;
}>();

const { expandedIds, toggleExpand, toView, route } = inject<any>("subMenuCtx")!;
</script>
