<template>
  <div class="app-shell" :class="{ 'file-drop-target-active': draggingFiles }" data-file-drop-target="app" @dragenter.prevent="handleDragEnter" @dragover.prevent="handleDragOver" @dragleave.prevent="handleDragLeave" @drop.prevent="handleDrop">
    <div class="global-file-drop-indicator" aria-hidden="true"><div class="global-file-drop-card"><span class="global-file-drop-icon">↓</span><strong>松开以添加文件</strong><small>文件会加入输入框，不会立即发送</small></div></div>
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
function handleDragEnter(event: DragEvent) { if (!isFileDrag(event)) return; dragDepth++; draggingFiles.value = true; event.dataTransfer!.dropEffect = 'copy' }
function handleDragOver(event: DragEvent) { if (!isFileDrag(event)) return; draggingFiles.value = true; event.dataTransfer!.dropEffect = 'copy' }
function handleDragLeave(event: DragEvent) { if (!isFileDrag(event)) return; dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) draggingFiles.value = false }
function handleDrop(event: DragEvent) {
  if (!isFileDrag(event)) return
  dragDepth = 0
  draggingFiles.value = false
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

onBeforeUnmount(() => { handlers.forEach((cancel: any) => typeof cancel === 'function' && cancel()); dragDepth = 0; draggingFiles.value = false });
</script>

<style lang="less">
#app, .app-shell { width: 100%; height: 100%; min-height: 0; color: var(--color-text-1); }
.app-shell { position: relative; overflow: hidden; }
.global-file-drop-indicator { position: absolute; z-index: 999; inset: 0; display: grid; place-items: center; pointer-events: none; opacity: 0; visibility: hidden; transition: opacity .16s ease, visibility .16s ease; background: rgba(20, 92, 220, .08); }
.app-shell.file-drop-target-active .global-file-drop-indicator { opacity: 1; visibility: visible; }
.global-file-drop-indicator::before { content: ''; position: absolute; inset: 14px; border: 2px dashed rgba(54, 103, 232, .72); border-radius: 18px; animation: global-file-drop-pulse 1.1s ease-in-out infinite; }
.global-file-drop-card { position: relative; display: flex; min-width: 250px; padding: 24px 34px; flex-direction: column; align-items: center; gap: 7px; border: 1px solid rgba(54, 103, 232, .35); border-radius: 14px; background: color-mix(in srgb, var(--color-bg-2, #fff) 92%, #4e7cff); box-shadow: 0 14px 45px rgba(30, 71, 150, .18); color: var(--color-text-1, #1d2129); animation: global-file-drop-card-in .2s ease-out; }
.global-file-drop-icon { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 50%; background: #3767e8; color: #fff; font-size: 28px; line-height: 1; animation: global-file-drop-bounce 1s ease-in-out infinite; }
.global-file-drop-card strong { font-size: 16px; }
.global-file-drop-card small { color: var(--color-text-3, #86909c); font-size: 12px; }
@keyframes global-file-drop-pulse { 0%, 100% { opacity: .45; transform: scale(1); } 50% { opacity: 1; transform: scale(1.008); } }
@keyframes global-file-drop-card-in { from { opacity: 0; transform: translateY(8px) scale(.98); } to { opacity: 1; transform: translateY(0) scale(1); } }
@keyframes global-file-drop-bounce { 0%, 100% { transform: translateY(0); } 50% { transform: translateY(4px); } }
.boot-error { width: 100%; height: 100%; box-sizing: border-box; padding: 32px; background: #171a20; color: #ffb4ab; font: 14px/1.6 -apple-system, BlinkMacSystemFont, sans-serif; }
</style>
