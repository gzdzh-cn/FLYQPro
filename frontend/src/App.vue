<template>
  <div class="app-shell" data-file-drop-target="app" @dragenter.prevent="handleDragEnter" @dragover.prevent="handleDragOver" @dragleave.prevent="handleDragLeave" @drop.prevent="handleDrop">
    <div v-if="bootError" class="boot-error">飞秋Pro 加载失败：{{ bootError }}</div>
    <router-view v-else />
  </div>
</template>

<script setup lang="ts">
import { Events } from '@wailsio/runtime';
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useChatStore } from '@/store/modules/chat';

const chatStore = useChatStore();
const bootError = ref('')
const draggingFiles = ref(false)
let dragDepth = 0
function isFileDrag(event: DragEvent) { return Boolean(event.dataTransfer?.types.includes('Files')) }
function publishDragState(active: boolean) {
  window.dispatchEvent(new CustomEvent('flyqpro:file-drag-state', { detail: { active } }))
}
function handleDragEnter(event: DragEvent) {
  if (!isFileDrag(event)) return
  dragDepth++
  if (!draggingFiles.value) publishDragState(true)
  draggingFiles.value = true
  event.dataTransfer!.dropEffect = 'copy'
}
function handleDragOver(event: DragEvent) {
  if (!isFileDrag(event)) return
  if (!draggingFiles.value) publishDragState(true)
  draggingFiles.value = true
  event.dataTransfer!.dropEffect = 'copy'
}
function handleDragLeave(event: DragEvent) {
  if (!isFileDrag(event)) return
  dragDepth = Math.max(0, dragDepth - 1)
  if (!dragDepth) {
    draggingFiles.value = false
    publishDragState(false)
  }
}
function handleDrop(event: DragEvent) {
  if (!isFileDrag(event)) return
  dragDepth = 0
  draggingFiles.value = false
  publishDragState(false)
  const paths = Array.from(event.dataTransfer?.files || []).map((file: any) => String(file.path || '')).filter(Boolean)
  if (paths.length) window.dispatchEvent(new CustomEvent('flyqpro:file-dropped', { detail: { filenames: paths } }))
}
window.addEventListener('error', (event) => {
  console.error('[FlyQPro] 前端运行时错误', event.error || event.message)
  if (!document.querySelector('.chat-app')) bootError.value = event.message || '页面脚本异常'
});
window.addEventListener('unhandledrejection', (event) => {
  console.error('[FlyQPro] 未处理的异步错误', event.reason)
  if (!document.querySelector('.chat-app')) bootError.value = String(event.reason?.message || event.reason || '页面初始化异常')
});
const eventNames = ['chat:profile-updated', 'chat:network-status', 'chat:peer-updated', 'chat:friend-request', 'chat:friend-request-updated', 'chat:message', 'chat:message-status', 'chat:attachment', 'chat:transfer-progress', 'chat:attachment-migration'];
let handlers: Array<() => void> = [];
onMounted(() => {
  document.getElementById('app-loading')?.remove();
  try {
    handlers = eventNames.map((name) => Events.On(name, (event: any) => chatStore.handleEvent(name, event?.data ?? event)));
  } catch (error) {
    console.error('[FlyQPro] Wails 事件初始化失败', error);
  }
});

onBeforeUnmount(() => { handlers.forEach((cancel: any) => typeof cancel === 'function' && cancel()); dragDepth = 0; draggingFiles.value = false; publishDragState(false) });
</script>

<style lang="less">
#app, .app-shell { width: 100%; height: 100%; min-height: 0; color: var(--color-text-1); }
.app-shell { position: relative; overflow: hidden; }
.boot-error { width: 100%; height: 100%; box-sizing: border-box; padding: 32px; background: #171a20; color: #ffb4ab; font: 14px/1.6 -apple-system, BlinkMacSystemFont, sans-serif; }
</style>
