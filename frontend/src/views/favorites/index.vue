<template>
  <section class="favorites-page" @pointerdown="closeContext" @contextmenu="closeContext">
    <div class="favorites-content">
      <header class="favorites-head">
        <div>
          <h2>收藏</h2>
          <p>查看收藏的聊天、图片和文件</p>
        </div>
        <button type="button" class="favorites-refresh" title="刷新收藏" aria-label="刷新收藏" :disabled="loading" @click="loadFavorites">
          <icon-refresh :class="{ spinning: loading }" />
        </button>
      </header>

      <div v-if="loading && !favoriteMessages.length" class="favorites-state">
        <icon-loading />
        <span>正在读取收藏</span>
      </div>
      <div v-else-if="error && !favoriteMessages.length" class="favorites-state error">
        <icon-close-circle />
        <span>{{ error }}</span>
        <a-button size="small" type="primary" @click="loadFavorites">重试</a-button>
      </div>
      <div v-else-if="!favoriteGroups.length" class="favorites-state">
        <icon-bookmark />
        <strong>还没有收藏</strong>
        <span>在聊天消息上点击右键即可收藏</span>
      </div>
      <div v-else class="favorite-groups">
        <section v-for="group in favoriteGroups" :key="group.deviceId" class="favorite-group">
          <header class="favorite-group-head">
            <div class="avatar favorite-peer-avatar" :style="avatarStyle(group.peer?.nickname || group.deviceId, group.peer?.avatarData)">
              {{ group.peer?.avatarData ? '' : initials(group.peer?.nickname || group.deviceId) }}
            </div>
            <strong class="nickname-ellipsis">{{ peerName(group.deviceId) }}</strong>
            <span>{{ group.messages.length }} 条</span>
          </header>
          <div class="favorite-list">
            <article
              v-for="message in group.messages"
              :key="message.messageId"
              class="favorite-row"
              @contextmenu.prevent.stop="openContext($event, message)"
              @pointerdown.stop
            >
              <button v-if="isImageMessage(message)" type="button" class="favorite-thumbnail" :aria-label="`预览 ${message.attachmentName || '图片'}`" :title="message.attachmentName || '预览图片'" @click.stop="openImagePreview(message)">
                <img v-if="thumbnailUrls[message.messageId]" :src="thumbnailUrls[message.messageId]" :alt="message.attachmentName || '图片缩略图'" />
                <icon-loading v-else-if="thumbnailLoading[message.messageId]" />
                <icon-file-image v-else />
              </button>
              <icon-file v-else-if="message.kind === 'file'" class="favorite-file-icon" />
              <div class="favorite-row-copy">
                <strong>{{ messageTitle(message) }}</strong>
                <span>{{ formatTime(message.createdAt) }}</span>
              </div>
            </article>
          </div>
        </section>
      </div>
    </div>

    <div v-if="context.visible && context.message" class="favorite-context-menu" :style="contextStyle" @click.stop @pointerdown.stop>
      <button type="button" @click="forwardContext"><icon-forward />转发</button>
      <button type="button" class="danger" @click="deleteContext"><icon-delete />删除</button>
    </div>

    <a-modal v-model:visible="previewVisible" :title="previewMessage?.attachmentName || '图片预览'" :footer="false" modal-class="favorite-image-modal">
      <div class="favorite-image-preview">
        <img v-if="previewUrl" :src="previewUrl" :alt="previewMessage?.attachmentName || '图片预览'" />
        <icon-loading v-else />
      </div>
    </a-modal>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconBookmark, IconCloseCircle, IconDelete, IconFile, IconFileImage, IconForward, IconLoading, IconRefresh } from '@arco-design/web-vue/es/icon'
import { ChatService } from '/#/flyqpro/internal/service'
import type { Message as ChatMessage, Peer } from '@/store/modules/chat/types'

const props = withDefaults(defineProps<{ peers: Peer[]; active?: boolean; preload?: boolean }>(), { active: false, preload: false })
const emit = defineEmits<{ (event: 'forward', message: ChatMessage): void }>()
const favoriteMessages = ref<ChatMessage[]>([])
const loading = ref(false)
const error = ref('')
const thumbnailUrls = reactive<Record<string, string>>({})
const thumbnailLoading = reactive<Record<string, boolean>>({})
const thumbnailLoadingGeneration = reactive<Record<string, number>>({})
const previewVisible = ref(false)
const previewMessage = ref<ChatMessage>()
const context = reactive<{ visible: boolean; x: number; y: number; message?: ChatMessage }>({ visible: false, x: 0, y: 0 })
let loadGeneration = 0
let loadPromise: Promise<void> | undefined
let lastLoadedAt = 0

const peerMap = computed(() => new Map(props.peers.map((peer) => [peer.deviceId, peer])))
const favoriteGroups = computed(() => {
  const groups = new Map<string, ChatMessage[]>()
  for (const message of favoriteMessages.value) {
    const deviceId = message.conversationId.startsWith('conv-') ? message.conversationId.slice(5) : message.senderDeviceId
    const items = groups.get(deviceId) || []
    items.push(message)
    groups.set(deviceId, items)
  }
  return [...groups.entries()]
    .map(([deviceId, messages]) => ({ deviceId, peer: peerMap.value.get(deviceId), messages: messages.sort((left, right) => timeValue(right.createdAt) - timeValue(left.createdAt)) }))
    .sort((left, right) => timeValue(right.messages[0]?.createdAt) - timeValue(left.messages[0]?.createdAt))
})
const contextStyle = computed(() => ({ left: `${context.x}px`, top: `${context.y}px` }))
const previewUrl = computed(() => previewMessage.value ? thumbnailUrls[previewMessage.value.messageId] || thumbnailDataURL(previewMessage.value) : '')

function timeValue(value: string) {
  const result = new Date(value || '').getTime()
  return Number.isFinite(result) ? result : 0
}
function initials(value: string) { return (value || '?').trim().slice(0, 1).toUpperCase() }
function avatarStyle(value: string, image?: string) {
  if (image) return { backgroundImage: `url(${image})`, backgroundSize: 'cover', backgroundPosition: 'center' }
  let hash = 0
  for (const char of value || '?') hash = (hash * 31 + char.charCodeAt(0)) >>> 0
  const hue = hash % 360
  return { background: `linear-gradient(135deg, hsl(${hue} 80% 65%), hsl(${(hue + 42) % 360} 75% 45%))` }
}
function peerName(deviceId: string) {
  const peer = peerMap.value.get(deviceId)
  return peer?.remark || peer?.nickname || deviceId.slice(0, 8) || '未知好友'
}
function formatTime(value: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const sameDay = date.toDateString() === now.toDateString()
  const time = `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
  return sameDay ? time : `${date.getMonth() + 1}月${date.getDate()}日 ${time}`
}
function formatBytes(value: number) {
  if (!value) return '未知大小'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}
function isImageMessage(message: ChatMessage) {
  if (message.attachmentMime) return message.attachmentMime.toLowerCase().startsWith('image/')
  return /\.(avif|bmp|gif|heic|heif|jpe?g|png|webp)$/i.test(message.attachmentName || message.content || '')
}
function thumbnailDataURL(message: ChatMessage) {
  const data = String(message.attachmentThumbnail || '')
  if (!data) return ''
  if (data.startsWith('data:')) return data
  return `data:${message.attachmentThumbnailMime || 'image/jpeg'};base64,${data}`
}
function messageTitle(message: ChatMessage) {
  if (message.kind === 'file') {
    const name = message.attachmentName || message.content || '附件'
    return `${name}${message.attachmentSize ? ` · ${formatBytes(message.attachmentSize)}` : ''}`
  }
  return message.content || '收藏消息'
}
async function loadThumbnail(message: ChatMessage, generation: number) {
  if (!isImageMessage(message) || thumbnailUrls[message.messageId]) return
  if (thumbnailLoading[message.messageId] && thumbnailLoadingGeneration[message.messageId] === generation) return
  const immediate = thumbnailDataURL(message)
  if (immediate) {
    thumbnailUrls[message.messageId] = immediate
    return
  }
  thumbnailLoading[message.messageId] = true
  thumbnailLoadingGeneration[message.messageId] = generation
  try {
    const source = await ChatService.GetAttachmentThumbnail(message.attachmentId || '')
    if (generation === loadGeneration && source) thumbnailUrls[message.messageId] = source
  } catch {
    // The row remains usable when the attachment no longer has a thumbnail.
  } finally {
    if (thumbnailLoadingGeneration[message.messageId] === generation) {
      delete thumbnailLoading[message.messageId]
      delete thumbnailLoadingGeneration[message.messageId]
    }
  }
}
async function loadFavorites(force = true) {
  if (loadPromise) return loadPromise
  if (!force && lastLoadedAt > 0 && Date.now() - lastLoadedAt < 15000) return
  const generation = ++loadGeneration
  loading.value = true
  error.value = ''
  const request = (async () => {
    try {
      const conversations = await ChatService.ListConversations()
      const messageLists = await Promise.all((conversations || []).map(async (conversation) => {
        try { return await ChatService.ListMessages(conversation.conversationId) } catch { return [] }
      }))
      if (generation !== loadGeneration) return
      favoriteMessages.value = messageLists.flat().filter((message) => message.isFavorite && !message.deletedAt)
      lastLoadedAt = Date.now()
      for (const message of favoriteMessages.value) void loadThumbnail(message, generation)
    } catch (loadError: any) {
      if (generation === loadGeneration) error.value = loadError?.message || '读取收藏失败'
    } finally {
      if (generation === loadGeneration) loading.value = false
    }
  })()
  loadPromise = request
  try {
    await request
  } finally {
    if (loadPromise === request) loadPromise = undefined
  }
  return request
}
function refreshIfNeeded() {
  if (!props.active && !props.preload) return
  const hasFreshData = lastLoadedAt > 0 && Date.now() - lastLoadedAt < 15000
  if (!hasFreshData) void loadFavorites(false)
}
function closeContext() { context.visible = false; context.message = undefined }
function openContext(event: MouseEvent, message: ChatMessage) {
  context.message = message
  context.x = Math.max(8, Math.min(event.clientX, window.innerWidth - 170))
  context.y = Math.max(8, Math.min(event.clientY, window.innerHeight - 100))
  context.visible = true
}
function forwardContext() {
  const message = context.message
  closeContext()
  if (message) emit('forward', message)
}
function deleteContext() {
  const message = context.message
  closeContext()
  if (!message) return
  Modal.confirm({
    title: '删除收藏消息',
    content: '删除后只会移除本机的这条消息，不会影响对方的聊天记录。是否继续？',
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await ChatService.DeleteMessage(message.messageId)
        favoriteMessages.value = favoriteMessages.value.filter((item) => item.messageId !== message.messageId)
        Message.success('已删除收藏消息')
      } catch (deleteError: any) {
        Message.error(deleteError?.message || '删除收藏消息失败')
        throw deleteError
      }
    },
  })
}
async function openImagePreview(message: ChatMessage) {
  previewMessage.value = message
  previewVisible.value = true
  await loadThumbnail(message, loadGeneration)
  if (!thumbnailUrls[message.messageId]) {
    if (previewMessage.value === message) previewVisible.value = false
    Message.error('图片缩略图不可用')
    return
  }
}
function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closeContext()
}
watch(() => [props.active, props.preload], () => refreshIfNeeded())
onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  refreshIfNeeded()
})
onBeforeUnmount(() => {
  loadGeneration++
  loadPromise = undefined
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped lang="less">
.favorites-page { flex: 1; min-width: 0; min-height: 0; overflow: auto; background: var(--app-bg); color: var(--text); }
.favorites-content { width: min(960px, calc(100% - 88px)); margin: 0 auto; padding: 34px 0 54px; }
.favorites-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 28px; }
.favorites-head h2 { margin: 0 0 6px; font-size: 26px; }
.favorites-head p { margin: 0; color: var(--muted); font-size: 13px; }
.favorites-refresh { display: inline-flex; width: 34px; height: 34px; padding: 0; align-items: center; justify-content: center; border: 0; border-radius: 8px; background: transparent; color: var(--muted); cursor: pointer; }
.favorites-refresh:hover:not(:disabled) { background: var(--hover); color: var(--text); }
.favorites-refresh:disabled { cursor: default; opacity: .5; }
.favorites-refresh svg { font-size: 18px; }
.favorites-refresh .spinning { animation: favorite-spin .8s linear infinite; }
.favorites-state { min-height: 320px; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 10px; color: var(--muted); }
.favorites-state > svg { font-size: 34px; color: var(--accent); }
.favorites-state strong { color: var(--text); font-size: 16px; }
.favorites-state.error { color: var(--danger, #f53f3f); }
.favorite-group { margin-bottom: 24px; overflow: hidden; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-1); }
.favorite-group-head { height: 58px; padding: 0 18px; display: flex; align-items: center; gap: 10px; border-bottom: 1px solid var(--line); }
.favorite-group-head > span { margin-left: auto; color: var(--muted); font-size: 12px; }
.favorite-peer-avatar { width: 32px; height: 32px; border-radius: 10px; font-size: 13px; }
.favorite-list { background: var(--surface-1); }
.favorite-row { min-height: 72px; padding: 10px 18px; box-sizing: border-box; display: flex; align-items: center; gap: 12px; border-bottom: 1px solid var(--line); cursor: default; }
.favorite-row:last-child { border-bottom: 0; }
.favorite-row:hover { background: var(--hover); }
.favorite-thumbnail { width: 48px; height: 48px; padding: 0; display: inline-flex; flex: 0 0 48px; align-items: center; justify-content: center; overflow: hidden; border: 0; border-radius: 7px; background: var(--surface-3); color: var(--muted); cursor: zoom-in; }
.favorite-thumbnail img { width: 100%; height: 100%; object-fit: cover; }
.favorite-thumbnail svg { font-size: 20px; }
.favorite-file-icon { width: 24px; flex: 0 0 24px; color: var(--muted); font-size: 21px; }
.favorite-row-copy { min-width: 0; display: flex; flex: 1; flex-direction: column; gap: 6px; }
.favorite-row-copy strong { overflow: hidden; color: var(--text); font-size: 14px; font-weight: 500; line-height: 1.4; text-overflow: ellipsis; white-space: nowrap; }
.favorite-row-copy span { color: var(--muted); font-size: 12px; }
.favorite-context-menu { position: fixed; z-index: 9999; min-width: 132px; padding: 6px; display: flex; flex-direction: column; background: var(--surface-1); color: var(--text); border: 1px solid var(--line); border-radius: 8px; box-shadow: 0 12px 30px rgba(20, 30, 60, .28); }
.favorite-context-menu button { padding: 8px 9px; display: flex; align-items: center; gap: 8px; border: 0; border-radius: 5px; background: transparent; color: inherit; text-align: left; cursor: pointer; font-size: 13px; }
.favorite-context-menu button:hover { background: var(--hover); }
.favorite-context-menu button svg { width: 16px; font-size: 16px; }
.favorite-context-menu button.danger { color: #f53f3f; }
.favorite-image-preview { min-height: 120px; display: flex; align-items: center; justify-content: center; background: var(--surface-2); border-radius: 7px; }
.favorite-image-preview img { display: block; max-width: 100%; max-height: 68vh; object-fit: contain; }
.favorite-image-preview svg { color: var(--accent); font-size: 28px; }
@keyframes favorite-spin { to { transform: rotate(360deg); } }
@media (max-width: 720px) { .favorites-content { width: calc(100% - 32px); padding-top: 24px; } .favorite-group { border-radius: 8px; } }
</style>
