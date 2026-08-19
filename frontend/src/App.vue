<template>
  <router-view />
</template>

<script setup lang="ts">
import { Events } from '@wailsio/runtime';
import { onBeforeUnmount } from 'vue';
import { useChatStore } from '@/store/modules/chat';

const chatStore = useChatStore();
const eventNames = ['chat:profile-updated', 'chat:network-status', 'chat:peer-updated', 'chat:friend-request', 'chat:friend-request-updated', 'chat:message', 'chat:attachment', 'chat:transfer-progress'];
const handlers = eventNames.map((name) => Events.On(name, (event: any) => chatStore.handleEvent(name, event?.data ?? event)));

onBeforeUnmount(() => handlers.forEach((cancel: any) => typeof cancel === 'function' && cancel()));
</script>

<style lang="less">
#app { width: 100%; height: 100%; color: var(--color-text-1); }
</style>
