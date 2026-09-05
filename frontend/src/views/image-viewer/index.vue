<template>
  <main class="image-viewer-app" :class="{ dark: isDark, 'is-macos': isMac, 'is-windows': isWindows }">
    <header class="image-viewer-head" @dblclick="isWindows && toggleWindowMaximise()">
      <div v-if="isMac" class="image-viewer-mac-controls" aria-label="窗口控制" @dblclick.stop>
        <button type="button" class="mac-traffic-light mac-traffic-close" aria-label="关闭" title="关闭" @click.stop="closeViewer"></button>
        <button type="button" class="mac-traffic-light mac-traffic-minimise" aria-label="最小化" title="最小化" @click.stop="Window.Minimise()"></button>
        <button type="button" class="mac-traffic-light mac-traffic-maximise" aria-label="最大化或还原" title="最大化或还原" @click.stop="toggleWindowMaximise"></button>
      </div>
      <div v-if="!isPdfPreview && !isVideoPreview" class="image-viewer-toolbar" role="toolbar" aria-label="图片工具栏" @dblclick.stop>
        <button type="button" aria-label="上一张" title="上一张" :disabled="!canMovePrevious || loading" @click="moveImage(-1)"><icon-left /></button>
        <button type="button" aria-label="下一张" title="下一张" :disabled="!canMoveNext || loading" @click="moveImage(1)"><icon-right /></button>
        <button type="button" aria-label="缩小" title="缩小" :disabled="loading || scale <= .5" @click="zoomImage(-.25)"><icon-zoom-out /></button>
        <button type="button" aria-label="放大" title="放大" :disabled="loading || scale >= 6" @click="zoomImage(.25)"><icon-zoom-in /></button>
        <button type="button" aria-label="向左旋转90度" title="向左旋转90°" :disabled="loading || !source" @click="rotateImageLeft"><icon-rotate-left /></button>
        <button v-if="isSharedPreview" type="button" aria-label="下载" title="下载" :disabled="loading || !sharedActionPath" @click="downloadSharedImage"><icon-download /></button>
        <button v-if="isSharedPreview" type="button" aria-label="另存为" title="另存为" :disabled="loading || !sharedActionPath" @click="saveSharedImageAs"><icon-save /></button>
      </div>
      <div v-if="isWindows" class="image-viewer-window-actions" @dblclick.stop>
        <button type="button" aria-label="最小化" title="最小化" @click.stop="Window.Minimise()"><icon-minus /></button>
        <button type="button" aria-label="最大化或还原" title="最大化或还原" @click.stop="toggleWindowMaximise"><icon-fullscreen /></button>
        <button type="button" class="window-close" aria-label="关闭图片查看器" title="关闭" @click.stop="closeViewer"><icon-close /></button>
      </div>
    </header>

    <section class="image-viewer-canvas" @wheel.prevent.stop="handleWheel" @dblclick.prevent.stop="toggleZoom" @pointerdown.stop="handlePointerDown" @pointermove.stop="handlePointerMove" @pointerup.stop="handlePointerUp" @pointercancel.stop="handlePointerUp">
      <div v-if="galleryLength > 1" class="image-nav-side image-nav-side-prev">
        <button type="button" class="image-nav-arrow" aria-label="上一张" title="上一张" :disabled="!canMovePrevious || loading" @pointerdown.stop @click.stop="moveImage(-1)"><icon-left /></button>
      </div>
      <iframe v-if="source && isPdfPreview" :key="viewerContentKey" class="pdf-preview" :src="source" :title="imageViewerName" />
      <template v-else-if="isVideoPreview">
        <video v-if="source" ref="videoElement" :key="viewerContentKey" class="video-preview" :src="source" :poster="thumbnailSource || undefined" :title="imageViewerName" controls playsinline crossorigin="anonymous" preload="none" @pointerdown.stop @pointermove.stop @pointerup.stop @pointercancel.stop @wheel.stop @click.stop @contextmenu.stop @loadeddata="handleVideoReady" @canplay="handleVideoReady" @play="handleVideoPlay" @playing="handleVideoPlaying" @waiting="handleVideoWaiting" @pause="handleVideoPause" @ended="handleVideoPause" @error="handleVideoError" @dblclick.stop />
        <img v-else-if="thumbnailSource" class="video-poster" :src="thumbnailSource" :alt="imageViewerName" draggable="false" />
        <div v-else class="video-poster-empty" aria-hidden="true"></div>
        <div v-if="videoLoading || videoPosterLoading" class="video-loading-state"><icon-loading /><span>{{ videoLoading ? '正在启动视频' : '正在加载视频预览' }}</span></div>
        <button v-if="videoPaused && !videoLoading && !error" type="button" class="video-play-button" aria-label="播放视频" title="播放视频" @pointerdown.stop @click.stop="playVideo"><icon-play-circle-fill /></button>
      </template>
      <template v-else>
        <img v-if="thumbnailSource" class="preview-image preview-thumbnail" :class="{ 'preview-image-faded': originalReady }" :src="thumbnailSource" :alt="imageViewerName" :style="imageTransform" draggable="false" />
        <img v-if="originalSource" class="preview-image preview-original" :class="{ 'preview-image-visible': originalReady }" :src="originalSource" :alt="imageViewerName" :style="imageTransform" draggable="false" @load="handleOriginalLoad" @error="handleOriginalError" />
      </template>
      <div v-if="galleryLength > 1" class="image-nav-side image-nav-side-next">
        <button type="button" class="image-nav-arrow" aria-label="下一张" title="下一张" :disabled="!canMoveNext || loading" @pointerdown.stop @click.stop="moveImage(1)"><icon-right /></button>
      </div>
      <div v-if="loading" class="image-viewer-state"><icon-loading /><span>{{ isVideoPreview ? '正在读取视频' : '正在读取图片' }}</span></div>
      <div v-else-if="thumbnailPending" class="image-viewer-state"><icon-loading /><span>正在生成缩略图</span></div>
      <div v-else-if="error && !thumbnailSource" class="image-viewer-state error"><icon-close-circle /><strong>{{ error }}</strong></div>
      <div v-else-if="originalLoading" class="image-viewer-quality"><icon-loading />正在加载高清原图</div>
      <div v-if="error && thumbnailSource" class="image-viewer-quality error">高清原图暂时无法读取，当前显示缩略图</div>
      <div v-else-if="!isSharedPreview && !imageMessages.length" class="image-viewer-state"><icon-close-circle /><strong>没有可预览的图片</strong></div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Events, Window } from '@wailsio/runtime'
import { ChatService, PreviewStreamService } from '/#/flyqpro/internal/service'
import { Message } from '@arco-design/web-vue'
import { IconClose, IconCloseCircle, IconDownload, IconFullscreen, IconLeft, IconLoading, IconMinus, IconPlayCircleFill, IconRight, IconRotateLeft, IconSave, IconZoomIn, IconZoomOut } from '@arco-design/web-vue/es/icon'
import { getOrLoadVideoThumbnail, type VideoThumbnailIdentity } from '@/utils/video-thumbnail-cache'

const route = useRoute()
const conversationId = computed(() => String(route.query.conversationId || ''))
const initialMessageId = computed(() => String(route.query.messageId || ''))
const previewSource = computed(() => String(route.query.source || ''))
const sharedPreviewType = computed(() => String(route.query.previewType || '').toLowerCase())
const sharedDeviceId = computed(() => String(route.query.deviceId || '').trim())
const sharedFolderId = computed(() => String(route.query.sharedFolderId || '').trim())
const sharedRelativePath = computed(() => String(route.query.relativePath || ''))
const sharedEntryId = computed(() => String(route.query.entryId || ''))
const sharedFileSize = computed(() => Number(route.query.fileSize || 0))
const sharedModifiedAt = computed(() => String(route.query.modifiedAt || ''))
const isSharedPreview = computed(() => previewSource.value === 'shared-owner' || previewSource.value === 'shared-friend')
const messages = ref<any[]>([])
type SharedPreviewEntry = { name: string; relativePath: string; mimeType?: string; isDirectory?: boolean; entryId?: string; size?: number; modifiedAt?: string }
const sharedEntries = ref<SharedPreviewEntry[]>([])
const sharedCurrentPath = ref(sharedRelativePath.value)
const currentIndex = ref(0)
const currentMessageId = ref('')
const source = ref('')
const thumbnailSource = ref('')
const originalSource = ref('')
const thumbnailReady = ref(false)
const originalLoading = ref(false)
const originalReady = ref(false)
const sourceType = ref<'thumbnail' | 'original'>('thumbnail')
const sharedName = ref('')
const sharedIsPdf = ref(false)
const sharedIsVideo = ref(false)
const loading = ref(false)
const thumbnailPending = ref(false)
const error = ref('')
const videoElement = ref<HTMLVideoElement>()
const videoPaused = ref(true)
const videoLoading = ref(false)
const videoPosterLoading = ref(false)
const scale = ref(1)
const rotation = ref(0)
const offset = ref({ x: 0, y: 0 })
const isDark = ref(false)
const isMac = /Macintosh|Mac OS X|MacIntel/i.test(`${navigator.platform || ''} ${navigator.userAgent || ''}`)
const isWindows = /Win32|Windows/i.test(`${navigator.platform || ''} ${navigator.userAgent || ''}`)
const cache = new Map<string, { source: string; type: 'thumbnail' | 'original' }>()
const imageMessages = computed(() => messages.value.filter((message) => message.kind === 'file' && message.attachmentId && isImageMessage(message)))
const currentMessage = computed(() => imageMessages.value[currentIndex.value])
const sharedCurrentEntry = computed(() => sharedEntries.value[currentIndex.value])
const galleryLength = computed(() => isSharedPreview.value ? sharedEntries.value.length : imageMessages.value.length)
const canMovePrevious = computed(() => currentIndex.value > 0)
const canMoveNext = computed(() => currentIndex.value < galleryLength.value - 1)
const imageViewerName = computed(() => isSharedPreview.value ? (sharedCurrentEntry.value?.name || sharedName.value || '共享文件预览') : (currentMessage.value?.attachmentName || '图片预览'))
const isPdfPreview = computed(() => sharedIsPdf.value)
const isVideoPreview = computed(() => sharedIsVideo.value)
const sharedActionPath = computed(() => sharedCurrentEntry.value?.relativePath || sharedCurrentPath.value || sharedRelativePath.value)
const viewerContentKey = computed(() => `${isSharedPreview.value ? sharedCurrentPath.value : currentMessage.value?.messageId || ''}:${isPdfPreview.value || isVideoPreview.value ? source.value : ''}`)
const imageTransform = computed(() => ({
  transform: `translate(${offset.value.x}px, ${offset.value.y}px) rotate(${rotation.value}deg) scale(${scale.value})`,
  cursor: scale.value > 1 ? 'grab' : 'zoom-in',
  transition: gestureActive.value ? 'none' : 'transform .08s linear',
}))

let loadToken = 0
let wheelX = 0
let wheelResetTimer = 0
let wheelSwitchAt = 0
let reloadTimer = 0
let zoomFrame = 0
let zoomTarget = 1
let pinchFrame = 0
let pinchDelta = 0
let dragFrame = 0
let pendingDrag = { x: 0, y: 0 }
let videoSourcePromise: Promise<string> | undefined
const gestureActive = ref(false)
const switchingImage = ref(false)
type PointerPosition = { x: number; y: number }
let pointers = new Map<number, PointerPosition>()
let pointer: { id: number; x: number; y: number; offsetX: number; offsetY: number } | undefined
let multiPointerGesture = false
let multiGesture: { type: 'pinch' | 'three'; initialDistance: number; initialScale: number; lastX: number; lastY: number } | undefined
let eventCancels: Array<() => void> = []

function isImageMessage(message: any) {
  if (message?.attachmentMime) return String(message.attachmentMime).toLowerCase().startsWith('image/')
  return /\.(avif|bmp|gif|heic|heif|jpe?g|png|webp)$/i.test(message?.attachmentName || message?.content || '')
}

function completed(message: any) { return ['sent', 'saved'].includes(message?.attachmentStatus || message?.status) }

function thumbnailDataURL(message: any) {
  const data = String(message?.attachmentThumbnail || '')
  if (!data) return ''
  if (data.startsWith('data:')) return data
  return `data:${message?.attachmentThumbnailMime || 'image/jpeg'};base64,${data}`
}

function applyTheme(theme: string) {
  const dark = theme === 'dark' || (theme === 'system' && window.matchMedia?.('(prefers-color-scheme: dark)').matches)
  isDark.value = Boolean(dark)
  // Keep the native window background in sync with the in-page title bar.
  // This is important for the area occupied by the platform title bar on
  // macOS, and also prevents a light/dark strip from appearing while the
  // viewer is open on Windows and Linux.
  const surface = dark ? { hex: '#15181d', rgb: [21, 24, 29] } : { hex: '#f7f8fa', rgb: [247, 248, 250] }
  document.documentElement.style.setProperty('--window-corner-bg', surface.hex)
  document.body.style.backgroundColor = surface.hex
  void Window.SetBackgroundColour(surface.rgb[0], surface.rgb[1], surface.rgb[2], 255).catch(() => undefined)
  if (dark) document.body.setAttribute('arco-theme', 'dark')
  else document.body.removeAttribute('arco-theme')
}

async function loadTheme() {
  try { const profile = await ChatService.GetProfile(); applyTheme(profile.theme || 'system') } catch { applyTheme('system') }
}

function clampScale(value: number) { return Math.min(6, Math.max(.5, value)) }
function stopZoomAnimation() {
  if (!zoomFrame) return
  window.cancelAnimationFrame(zoomFrame)
  zoomFrame = 0
}
function animateZoom() {
  if (zoomFrame) return
  const tick = () => {
    const distance = zoomTarget - scale.value
    if (Math.abs(distance) < .002) {
      scale.value = zoomTarget
      zoomFrame = 0
      return
    }
    scale.value += distance * .22
    if (scale.value <= 1) offset.value = { x: 0, y: 0 }
    zoomFrame = window.requestAnimationFrame(tick)
  }
  zoomFrame = window.requestAnimationFrame(tick)
}
function setZoomTarget(value: number) {
  zoomTarget = clampScale(value)
  if (zoomTarget <= 1) offset.value = { x: 0, y: 0 }
  animateZoom()
}
function resetTransform() { stopZoomAnimation(); zoomTarget = 1; scale.value = 1; rotation.value = 0; offset.value = { x: 0, y: 0 } }
function zoomImage(delta: number) {
  setZoomTarget(zoomTarget + delta)
}
function zoomByPinch(delta: number) { setZoomTarget(zoomTarget * Math.exp(-delta * .002)) }
function queuePinchZoom(delta: number) {
  pinchDelta += delta
  if (pinchFrame) return
  pinchFrame = window.requestAnimationFrame(() => {
    const next = clampScale(zoomTarget * Math.exp(-pinchDelta * .002))
    pinchDelta = 0
    pinchFrame = 0
    zoomTarget = next
    scale.value = next
    if (next <= 1) offset.value = { x: 0, y: 0 }
  })
}
function queueThreeFingerDrag(deltaX: number, deltaY: number) {
  pendingDrag.x += deltaX
  pendingDrag.y += deltaY
  if (dragFrame) return
  dragFrame = window.requestAnimationFrame(() => {
    offset.value = { x: offset.value.x + pendingDrag.x, y: offset.value.y + pendingDrag.y }
    pendingDrag = { x: 0, y: 0 }
    dragFrame = 0
  })
}
function flushThreeFingerDrag() {
  if (dragFrame) window.cancelAnimationFrame(dragFrame)
  dragFrame = 0
  if (!pendingDrag.x && !pendingDrag.y) return
  offset.value = { x: offset.value.x + pendingDrag.x, y: offset.value.y + pendingDrag.y }
  pendingDrag = { x: 0, y: 0 }
}
function toggleZoom() { if (scale.value > 1) setZoomTarget(1); else setZoomTarget(2) }
function rotateImageLeft() { rotation.value = (rotation.value - 90) % 360 }

function handleOriginalLoad() {
  originalReady.value = true
  originalLoading.value = false
  sourceType.value = 'original'
  source.value = originalSource.value
}

function handleOriginalError() {
  originalReady.value = false
  originalLoading.value = false
  if (thumbnailSource.value) error.value = '高清原图暂时无法读取，当前显示缩略图'
  else error.value = '本地原图不存在'
}

async function thumbnailFor(message: any, token: number) {
  const cacheKey = `${message.attachmentId}:thumbnail`
  const cached = cache.get(cacheKey)
  if (cached) return cached.source
  let sourceValue = thumbnailDataURL(message)
  if (!sourceValue) {
    try { sourceValue = await ChatService.GetAttachmentThumbnail(message.attachmentId) } catch { sourceValue = '' }
  }
  if (token !== loadToken) return ''
  if (sourceValue) cache.set(cacheKey, { source: sourceValue, type: 'thumbnail' })
  return sourceValue
}

async function originalFor(message: any, token: number) {
  const cacheKey = `${message.attachmentId}:original`
  const cached = cache.get(cacheKey)
  if (cached) return cached.source
  let sourceValue = ''
  try { sourceValue = await PreviewStreamService.CreateAttachmentPreviewURL(message.attachmentId) } catch { sourceValue = '' }
  if (token !== loadToken) return ''
  if (sourceValue) cache.set(cacheKey, { source: sourceValue, type: 'original' })
  return sourceValue
}

async function sharedSourceFor(relativePath: string) {
  if (!sharedFolderId.value) throw new Error('共享文件夹参数无效')
  if (previewSource.value === 'shared-owner') return ChatService.GetSharedEntryPreview(sharedFolderId.value, relativePath)
  if (previewSource.value === 'shared-friend' && sharedDeviceId.value) return ChatService.GetFriendSharedEntryPreview(sharedDeviceId.value, sharedFolderId.value, relativePath)
  throw new Error('共享预览参数无效')
}

async function sharedThumbnailFor(relativePath: string, entry?: SharedPreviewEntry) {
  if (!sharedFolderId.value) throw new Error('共享文件夹参数无效')
  if (previewSource.value === 'shared-owner') return ChatService.GetSharedEntryThumbnail(sharedFolderId.value, relativePath)
  if (previewSource.value === 'shared-friend' && sharedDeviceId.value) {
    const entryId = entry?.entryId || sharedEntryId.value
    const fileSize = entry?.size || sharedFileSize.value
    const modifiedAt = entry?.modifiedAt || sharedModifiedAt.value
    if (entryId || fileSize || modifiedAt) return ChatService.GetFriendSharedEntryThumbnailCached(sharedDeviceId.value, sharedFolderId.value, relativePath, entryId, fileSize, modifiedAt)
    return ChatService.GetFriendSharedEntryThumbnail(sharedDeviceId.value, sharedFolderId.value, relativePath)
  }
  throw new Error('共享预览参数无效')
}

async function sharedOriginalFor(relativePath: string) {
  if (!sharedFolderId.value) throw new Error('共享文件夹参数无效')
  // Videos must remain a streaming URL so playback can start before a large
  // file has finished transferring. The endpoint implements HTTP Range.
  return PreviewStreamService.CreateSharedPreviewURL(previewSource.value, sharedDeviceId.value, sharedFolderId.value, relativePath)
}

function videoThumbnailIdentity(relativePath: string, entry?: SharedPreviewEntry): VideoThumbnailIdentity {
  return {
    source: previewSource.value,
    deviceId: sharedDeviceId.value,
    sharedFolderId: sharedFolderId.value,
    entryId: entry?.entryId || sharedEntryId.value,
    relativePath,
    fileSize: entry?.size || sharedFileSize.value,
    modifiedAt: entry?.modifiedAt || sharedModifiedAt.value,
  }
}

async function loadVideoPoster(relativePath: string, entry: SharedPreviewEntry | undefined, token: number) {
  const thumbnail = await getOrLoadVideoThumbnail(videoThumbnailIdentity(relativePath, entry), async () => {
    for (let attempt = 0; attempt < 6; attempt++) {
      try {
        const value = await sharedThumbnailFor(relativePath, entry)
        if (value) return value
      } catch { /* retry while the peer generates the thumbnail */ }
      if (attempt < 5) await new Promise((resolve) => window.setTimeout(resolve, 300 + attempt * 200))
    }
    return ''
  })
  if (token !== loadToken || !thumbnail) return
  thumbnailSource.value = thumbnail
  thumbnailReady.value = true
}

function isImageSharedEntry(entry: SharedPreviewEntry) {
  return !entry.isDirectory && (String(entry.mimeType || '').toLowerCase().startsWith('image/') || /\.(avif|bmp|gif|heic|heif|jpe?g|png|webp)$/i.test(entry.name))
}
function isVideoSharedEntry(entry: SharedPreviewEntry) {
  return !entry.isDirectory && (String(entry.mimeType || '').toLowerCase().startsWith('video/') || /\.(3gp|avi|flv|m4v|mkv|mov|mp4|mpeg|mpg|ogv|ts|webm|wmv)$/i.test(entry.name))
}
function isSharedMediaEntry(entry: SharedPreviewEntry) { return isImageSharedEntry(entry) || isVideoSharedEntry(entry) }
function handleVideoError() {
  if (isVideoPreview.value) {
    loading.value = false
    videoPaused.value = true
    videoLoading.value = false
    error.value = '视频暂时无法读取'
  }
}
function handleVideoReady() {
  loading.value = false
  videoPaused.value = videoElement.value?.paused ?? true
}
function handleVideoPlay() {
  videoPaused.value = false
  error.value = ''
}
function handleVideoPlaying() {
  videoLoading.value = false
  videoPaused.value = false
  error.value = ''
}
function handleVideoWaiting() {
  if (!videoElement.value?.paused) videoLoading.value = true
}
function handleVideoPause() {
  videoPaused.value = true
}
async function playVideo() {
  const token = loadToken
  if (videoLoading.value) return
  videoLoading.value = true
  try {
    if (!source.value) {
      if (!videoSourcePromise) videoSourcePromise = sharedOriginalFor(sharedCurrentPath.value || sharedRelativePath.value).catch(() => '')
      const sourceValue = await videoSourcePromise
      if (token !== loadToken || !sourceValue) throw new Error('视频地址暂时无法读取')
      source.value = sourceValue
      await nextTick()
    }
    const video = videoElement.value
    if (!video || token !== loadToken) return
    if (video.ended) video.currentTime = 0
    await video.play()
  } catch (playError: any) {
    videoPaused.value = true
    videoLoading.value = false
    error.value = playError?.message ? `视频无法播放：${playError.message}` : '视频暂时无法播放'
  }
}
function parentSharedPath(path: string) {
  const parts = String(path || '').split('/').filter(Boolean)
  parts.pop()
  return parts.join('/')
}

function pruneCache() {
  const keep = new Set(imageMessages.value.slice(Math.max(0, currentIndex.value - 1), currentIndex.value + 2).map((message) => message.attachmentId))
  for (const key of cache.keys()) if (!keep.has(key.split(':')[0])) cache.delete(key)
}

async function loadCurrent() {
  if (isSharedPreview.value) {
    const token = ++loadToken
    const entry = sharedCurrentEntry.value
    const activePath = entry?.relativePath || sharedCurrentPath.value || sharedRelativePath.value
    loading.value = true
    thumbnailPending.value = false
    error.value = ''
    source.value = ''
    thumbnailSource.value = ''
    originalSource.value = ''
    thumbnailReady.value = false
    originalLoading.value = false
    originalReady.value = false
    videoPaused.value = true
    videoLoading.value = false
    videoPosterLoading.value = false
    videoSourcePromise = undefined
    sharedCurrentPath.value = activePath
    sharedName.value = entry?.name || activePath.split('/').filter(Boolean).pop() || '共享文件预览'
    sharedIsPdf.value = sharedPreviewType.value === 'pdf' || /\.pdf$/i.test(sharedName.value) || String(entry?.mimeType || '').toLowerCase() === 'application/pdf'
    sharedIsVideo.value = !sharedIsPdf.value && (sharedPreviewType.value === 'video' || (entry ? isVideoSharedEntry(entry) : false))
    resetTransform()
    if (sharedIsVideo.value) {
      loading.value = false
      thumbnailPending.value = false
      videoPosterLoading.value = true
      void loadVideoPoster(activePath, entry, token).finally(() => { if (token === loadToken) videoPosterLoading.value = false })
      void Window.SetTitle(`共享预览 - ${imageViewerName.value}`).catch(() => undefined)
      return
    }
    if (sharedIsPdf.value) {
      try {
        const sourceValue = await sharedSourceFor(activePath)
        if (token !== loadToken) return
        if (sourceValue) source.value = sourceValue
        else error.value = sharedIsVideo.value ? '视频暂时无法读取' : '共享文件预览失败'
        loading.value = false
        void Window.SetTitle(`共享预览 - ${imageViewerName.value}`).catch(() => undefined)
      } catch (loadError: any) {
        if (token !== loadToken) return
        loading.value = false
        error.value = loadError?.message || '共享文件预览失败'
      }
      return
    }
    // Open immediately. Thumbnail generation and the streaming original are
    // independent so a cache miss never delays the first usable preview.
    loading.value = false
    thumbnailPending.value = true
    void Window.SetTitle(`共享预览 - ${imageViewerName.value}`).catch(() => undefined)
    void (async () => {
      let thumbnail = ''
      try { thumbnail = await sharedThumbnailFor(activePath, entry) } catch { /* placeholder remains */ }
      if (token !== loadToken) return
      if (thumbnail) {
        thumbnailSource.value = thumbnail
        thumbnailReady.value = true
        source.value = thumbnail
      }
      thumbnailPending.value = false
    })()
    void (async () => {
      originalLoading.value = true
      try {
        const original = await sharedOriginalFor(activePath)
        if (token !== loadToken) return
        if (original) originalSource.value = original
        else if (!thumbnailSource.value) error.value = '图片原图暂时无法读取'
      } catch {
        if (token === loadToken && !thumbnailSource.value) error.value = '共享图片暂时无法读取'
      } finally {
        if (token === loadToken) originalLoading.value = false
      }
    })()
    return
  }
  const message = currentMessage.value
  const token = ++loadToken
  loading.value = true
  thumbnailPending.value = false
  error.value = ''
  source.value = ''
  thumbnailSource.value = ''
  originalSource.value = ''
  thumbnailReady.value = false
  originalLoading.value = false
  originalReady.value = false
  videoPaused.value = true
  sourceType.value = 'thumbnail'
  if (!message?.attachmentId) {
    loading.value = false
    error.value = '图片附件不存在'
    return
  }
  // A message already contains its thumbnail in the common case. Put it on
  // screen synchronously and fetch missing preview data without blocking the
  // first paint.
  const immediateThumbnail = thumbnailDataURL(message)
  if (immediateThumbnail) {
    thumbnailSource.value = immediateThumbnail
    thumbnailReady.value = true
    source.value = immediateThumbnail
    loading.value = false
  } else {
    loading.value = false
    thumbnailPending.value = true
  }
  void Window.SetTitle(`图片预览 - ${imageViewerName.value}`).catch(() => undefined)
  void (async () => {
    let thumbnail = immediateThumbnail
    if (!thumbnail) {
      try { thumbnail = await thumbnailFor(message, token) } catch { /* keep placeholder */ }
      if (token !== loadToken) return
      if (thumbnail) {
        thumbnailSource.value = thumbnail
        thumbnailReady.value = true
        source.value = thumbnail
      }
      thumbnailPending.value = false
    }
    if (!completed(message)) {
      if (token === loadToken && !thumbnail) error.value = '图片缩略图暂时无法读取'
      return
    }
    if (token !== loadToken) return
    originalLoading.value = true
    try {
      const original = await originalFor(message, token)
      if (token !== loadToken) return
      if (original) originalSource.value = original
      else if (!thumbnail) error.value = '本地原图不存在'
    } catch {
      if (token === loadToken && !thumbnail) error.value = '本地原图不存在'
    } finally {
      if (token === loadToken) originalLoading.value = false
    }
  })()
  pruneCache()
}

async function loadMessages() {
  if (isSharedPreview.value) {
    const requestedPath = sharedRelativePath.value
    sharedCurrentPath.value = requestedPath
    sharedName.value = requestedPath.split('/').filter(Boolean).pop() || '共享文件预览'
    currentMessageId.value = ''
    const initialIsPDF = sharedPreviewType.value === 'pdf' || /\.pdf$/i.test(sharedName.value)
    if (initialIsPDF) {
      sharedEntries.value = []
      currentIndex.value = 0
      await loadCurrent()
      return
    }
    // Do not wait for the containing directory to be listed before opening
    // the selected image.  The selected path is enough for the thumbnail and
    // preview services; the directory request only fills the adjacent-image
    // gallery in the background.
    const initialMimeType = sharedPreviewType.value === 'video' ? 'video/*' : 'image/*'
    const initialEntry = { name: sharedName.value, relativePath: requestedPath, mimeType: initialMimeType, entryId: sharedEntryId.value, size: sharedFileSize.value, modifiedAt: sharedModifiedAt.value }
    sharedEntries.value = [initialEntry]
    currentIndex.value = 0
    void loadCurrent()
    try {
      const parentPath = parentSharedPath(requestedPath)
      const page = previewSource.value === 'shared-owner'
        ? await ChatService.ListSharedEntriesPage(sharedFolderId.value, parentPath, 0, 100)
        : await ChatService.ListFriendSharedEntriesPage(sharedDeviceId.value, sharedFolderId.value, parentPath, 0, 100)
      const media = (page.entries || []).filter((entry: SharedPreviewEntry) => isSharedMediaEntry(entry))
      if (!media.some((entry: SharedPreviewEntry) => entry.relativePath === requestedPath)) {
        media.push(initialEntry)
      }
      sharedEntries.value = media
      currentIndex.value = Math.max(0, media.findIndex((entry: SharedPreviewEntry) => entry.relativePath === requestedPath))
    } catch {
      sharedEntries.value = [initialEntry]
      currentIndex.value = 0
    }
    return
  }
  if (!conversationId.value) {
    messages.value = []
    currentIndex.value = 0
    currentMessageId.value = ''
    await loadCurrent()
    return
  }
  try {
    // Bootstrap the selected record first. A large conversation history can
    // otherwise delay the first thumbnail even though the user already chose
    // the exact image to open.
    let bootstrapped = false
    if (initialMessageId.value) {
      try {
        const selected = await ChatService.GetMessage(initialMessageId.value)
        if (selected?.conversationId === conversationId.value && selected?.attachmentId) {
          messages.value = [selected]
          currentMessageId.value = selected.messageId
          currentIndex.value = 0
          bootstrapped = true
          void loadCurrent()
        }
      } catch { /* the full history load below remains the fallback */ }
    }
    const loaded = await ChatService.ListMessages(conversationId.value)
    const bootstrappedMessageId = currentMessageId.value
    messages.value = loaded || []
    const wanted = bootstrappedMessageId || initialMessageId.value
    const index = imageMessages.value.findIndex((message) => message.messageId === wanted)
    currentIndex.value = index >= 0 ? index : Math.min(currentIndex.value, Math.max(0, imageMessages.value.length - 1))
    currentMessageId.value = imageMessages.value[currentIndex.value]?.messageId || ''
    if (!bootstrapped || currentMessageId.value !== bootstrappedMessageId) {
      resetTransform()
      await loadCurrent()
    }
  } catch {
    messages.value = []
    loading.value = false
    error.value = '聊天记录读取失败'
  }
}

async function moveImage(direction: number) {
  if (loading.value || switchingImage.value) return
  const total = galleryLength.value
  const target = Math.max(0, Math.min(total - 1, currentIndex.value + direction))
  if (target === currentIndex.value) return
  const targetSharedEntry = isSharedPreview.value ? sharedEntries.value[target] : undefined
  const targetMessage = isSharedPreview.value ? undefined : imageMessages.value[target]
  if (!targetSharedEntry && !targetMessage) return
  currentIndex.value = target
  if (targetSharedEntry) sharedCurrentPath.value = targetSharedEntry.relativePath
  if (targetMessage) currentMessageId.value = targetMessage.messageId
  switchingImage.value = true
  resetTransform()
  // Remove the old media node before creating the next one. This prevents
  // composited frames from overlapping during quick trackpad navigation.
  source.value = ''
  await nextTick()
  try { await loadCurrent() } finally { switchingImage.value = false }
}

function closeViewer() { void Window.Close() }
function toggleWindowMaximise() { void Window.ToggleMaximise() }

async function downloadSharedImage() {
  const path = sharedActionPath.value
  if (!isSharedPreview.value || !path) return
  try {
    if (previewSource.value === 'shared-owner') {
      await ChatService.DownloadSharedEntry(sharedFolderId.value, path)
      Message.success('图片已下载到应用目录')
    } else if (previewSource.value === 'shared-friend' && sharedDeviceId.value) {
      const transfer = await ChatService.DownloadFriendSharedEntry(sharedDeviceId.value, sharedFolderId.value, path)
      if (transfer?.transferId) Message.success('已开始下载图片')
    }
  } catch (operationError: any) {
    Message.error(operationError?.message || '下载图片失败')
  }
}

async function saveSharedImageAs() {
  const path = sharedActionPath.value
  if (!isSharedPreview.value || !path) return
  try {
    if (previewSource.value === 'shared-owner') {
      const target = await ChatService.SaveSharedEntryAs(sharedFolderId.value, path)
      if (target) Message.success('图片已保存')
    } else if (previewSource.value === 'shared-friend' && sharedDeviceId.value) {
      const transfer = await ChatService.SaveFriendSharedEntryAs(sharedDeviceId.value, sharedFolderId.value, path)
      if (transfer?.transferId) Message.success('已开始保存图片')
    }
  } catch (operationError: any) {
    Message.error(operationError?.message || '保存图片失败')
  }
}

function handleWheel(event: WheelEvent) {
  // Chromium exposes macOS trackpad pinch as a ctrl+wheel stream. Applying
  // the delta continuously keeps the gesture proportional instead of turning
  // it into a series of visible .15x jumps.
  if (event.ctrlKey) {
    gestureActive.value = true
    const delta = Math.abs(event.deltaY) >= Math.abs(event.deltaX) ? event.deltaY : event.deltaX
    if (Math.abs(delta) >= .01) queuePinchZoom(delta)
    window.clearTimeout(wheelResetTimer)
    wheelResetTimer = window.setTimeout(() => { gestureActive.value = false; wheelResetTimer = 0 }, 120)
    return
  }
  const horizontal = Math.abs(event.deltaX) > Math.abs(event.deltaY) * 1.15
  if (horizontal && !event.ctrlKey) {
    if (scale.value > 1) {
      offset.value = { x: offset.value.x - event.deltaX, y: offset.value.y - event.deltaY }
      return
    }
    if ((wheelX > 0 && event.deltaX < 0) || (wheelX < 0 && event.deltaX > 0)) wheelX = 0
    wheelX += event.deltaX
    if (wheelResetTimer) window.clearTimeout(wheelResetTimer)
    // Trackpads can emit a fairly large delta even for a light touch. Keep a
    // longer accumulation window and require a deliberate horizontal swipe.
    wheelResetTimer = window.setTimeout(() => { wheelX = 0; wheelResetTimer = 0 }, 320)
    // A two-finger horizontal trackpad swipe is one navigation gesture. The
    // cooldown prevents a single long swipe from skipping several pictures.
    if (Math.abs(wheelX) >= 420 && performance.now() - wheelSwitchAt > 700) {
      const direction = wheelX > 0 ? 1 : -1
      wheelX = 0
      wheelSwitchAt = performance.now()
      void moveImage(direction)
    }
    return
  }
  // Windows' regular mouse wheel is reported as a large, discrete vertical
  // delta. Smooth, small deltas are normally a two-finger trackpad scroll and
  // remain inert; pinch is handled by the ctrl+wheel branch above.
  if (Math.abs(event.deltaY) >= 80 || event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
    const steps = Math.max(1, Math.min(3, Math.round(Math.abs(event.deltaY) / 120)))
    setZoomTarget(zoomTarget + (event.deltaY < 0 ? 0.25 : -0.25) * steps)
  }
}

function pointerCentroid() {
  let x = 0
  let y = 0
  pointers.forEach((point) => { x += point.x; y += point.y })
  return pointers.size ? { x: x / pointers.size, y: y / pointers.size } : { x: 0, y: 0 }
}
function pointerDistance() {
  const points = [...pointers.values()]
  if (points.length < 2) return 0
  return Math.hypot(points[0].x - points[1].x, points[0].y - points[1].y)
}
function handlePointerDown(event: PointerEvent) {
  if (event.button !== undefined && event.button !== 0) return
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  if (pointers.size >= 2) {
    gestureActive.value = true
    multiPointerGesture = true
    pointer = undefined
    const center = pointerCentroid()
    multiGesture = pointers.size >= 3
      ? { type: 'three', initialDistance: 0, initialScale: scale.value, lastX: center.x, lastY: center.y }
      : { type: 'pinch', initialDistance: pointerDistance(), initialScale: scale.value, lastX: center.x, lastY: center.y }
    return
  }
  pointer = { id: event.pointerId, x: event.clientX, y: event.clientY, offsetX: offset.value.x, offsetY: offset.value.y }
  ;(event.currentTarget as HTMLElement)?.setPointerCapture?.(event.pointerId)
}
function handlePointerMove(event: PointerEvent) {
  if (!pointers.has(event.pointerId)) return
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  if (pointers.size >= 3 && multiGesture?.type === 'three') {
    const center = pointerCentroid()
    if (scale.value > 1) queueThreeFingerDrag(center.x - multiGesture.lastX, center.y - multiGesture.lastY)
    multiGesture.lastX = center.x
    multiGesture.lastY = center.y
    return
  }
  if (pointers.size === 2 && multiGesture?.type === 'pinch') {
    const distance = pointerDistance()
    if (distance > 0 && multiGesture.initialDistance > 0) {
      const next = clampScale(multiGesture.initialScale * distance / multiGesture.initialDistance)
      const center = pointerCentroid()
      const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
      const anchorX = center.x - rect.left - rect.width / 2
      const anchorY = center.y - rect.top - rect.height / 2
      const ratio = scale.value > 0 ? next / scale.value : 1
      zoomTarget = next
      scale.value = next
      if (next > 1) offset.value = { x: anchorX + (offset.value.x - anchorX) * ratio, y: anchorY + (offset.value.y - anchorY) * ratio }
      if (next <= 1) offset.value = { x: 0, y: 0 }
    }
    return
  }
  if (!pointer || pointer.id !== event.pointerId || scale.value <= 1) return
  offset.value = { x: pointer.offsetX + event.clientX - pointer.x, y: pointer.offsetY + event.clientY - pointer.y }
}
function handlePointerUp(event: PointerEvent) {
  const start = pointer?.id === event.pointerId ? pointer : undefined
  const deltaX = start ? event.clientX - start.x : 0
  pointers.delete(event.pointerId)
  if (pointers.size) {
    const center = pointerCentroid()
    if (pointers.size >= 3 && multiGesture) {
      multiGesture.type = 'three'
      multiGesture.lastX = center.x
      multiGesture.lastY = center.y
    } else if (pointers.size === 2) {
      multiGesture = { type: 'three', initialDistance: 0, initialScale: scale.value, lastX: center.x, lastY: center.y }
    }
    pointer = undefined
    return
  }
  if (start && !multiPointerGesture && scale.value <= 1 && Math.abs(deltaX) >= 48) void moveImage(deltaX < 0 ? 1 : -1)
  pointer = undefined
  flushThreeFingerDrag()
  multiPointerGesture = false
  multiGesture = undefined
  gestureActive.value = false
}

function handleEvent(event: any) {
  const data = event?.data ?? event
  const current = currentMessage.value
  if (data?.conversationId && data.conversationId !== conversationId.value) return
  if (data?.messageId && data.messageId !== current?.messageId && data?.attachmentId !== current?.attachmentId) return
  if (reloadTimer) window.clearTimeout(reloadTimer)
  reloadTimer = window.setTimeout(() => { reloadTimer = 0; void loadMessages() }, 220)
}

function handleKey(event: KeyboardEvent) {
  if (event.key === 'Escape' || event.key === ' ' || event.code === 'Space' || (event.metaKey && event.key.toLowerCase() === 'w')) { event.preventDefault(); closeViewer(); return }
  if (event.key === 'ArrowLeft') { event.preventDefault(); void moveImage(-1) }
  if (event.key === 'ArrowRight') { event.preventDefault(); void moveImage(1) }
  if (event.key === '+' || event.key === '=') { event.preventDefault(); zoomImage(.25) }
  if (event.key === '-' || event.key === '_') { event.preventDefault(); zoomImage(-.25) }
}

watch([conversationId, initialMessageId, previewSource, sharedDeviceId, sharedRelativePath, sharedEntryId, sharedPreviewType], () => { currentMessageId.value = initialMessageId.value; void loadMessages() }, { immediate: true })

onMounted(async () => {
  await loadTheme()
  window.addEventListener('keydown', handleKey)
  for (const name of ['chat:message', 'chat:message-status', 'chat:attachment', 'chat:transfer-progress']) eventCancels.push(Events.On(name, handleEvent))
  eventCancels.push(Events.On('chat:profile-updated', (event: any) => {
    const theme = event?.data?.theme ?? event?.theme
    if (theme) applyTheme(theme)
    else void loadTheme()
  }))
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKey)
  eventCancels.forEach((cancel) => cancel())
  if (reloadTimer) window.clearTimeout(reloadTimer)
  if (wheelResetTimer) window.clearTimeout(wheelResetTimer)
  if (pinchFrame) window.cancelAnimationFrame(pinchFrame)
  if (dragFrame) window.cancelAnimationFrame(dragFrame)
  stopZoomAnimation()
  pointers.clear()
  multiGesture = undefined
  cache.clear()
})
</script>

<style scoped lang="less">
:global(html), :global(body), :global(#app) { width: 100%; height: 100%; margin: 0; padding: 0; overflow: hidden; background: var(--window-corner-bg, #15181d); }
.image-viewer-app { --viewer-bg: #111722; --viewer-surface: #f7f8fa; --viewer-text: #20252b; --viewer-muted: #687482; --viewer-line: #cfd6de; width: 100%; height: 100%; min-width: 0; min-height: 0; display: flex; overflow: hidden; flex-direction: column; background: var(--viewer-bg); color: var(--viewer-text); --wails-draggable: no-drag; }
.image-viewer-app:not(.dark) { --viewer-bg: #edf0f3; }
.image-viewer-app.dark { --viewer-surface: #15181d; --viewer-text: #f0f2f5; --viewer-muted: #a4adb8; --viewer-line: #39424d; }
.image-viewer-head { position: relative; display: flex; min-height: 36px; padding: 4px 10px 4px 16px; box-sizing: border-box; align-items: center; justify-content: flex-end; gap: 8px; flex: 0 0 36px; background: var(--viewer-surface); border-bottom: 1px solid var(--viewer-line); cursor: grab; -webkit-app-region: drag; --wails-draggable: drag; }
.image-viewer-app.is-macos .image-viewer-head { padding-left: 16px; }
.image-viewer-mac-controls { position: absolute; top: 0; left: 14px; display: flex; height: 100%; align-items: center; gap: 8px; --wails-draggable: no-drag; -webkit-app-region: no-drag; --wails-non-client-region: none; }
.mac-traffic-light { position: relative; display: inline-flex; width: 13px; height: 13px; padding: 0; border: 0; border-radius: 50%; cursor: pointer; opacity: .96; }
.mac-traffic-light::after { position: absolute; inset: 0; display: grid; place-items: center; color: rgba(35, 35, 35, .72); content: ''; font-size: 10px; line-height: 1; opacity: 0; transition: opacity .12s ease; }
.image-viewer-mac-controls:hover .mac-traffic-light::after { opacity: 1; }
.mac-traffic-close { background: #ff605c; }
.mac-traffic-close::after { content: '×'; }
.mac-traffic-minimise { background: #ffbd44; }
.mac-traffic-minimise::after { content: '−'; }
.mac-traffic-maximise { background: #00ca4e; }
.mac-traffic-maximise::after { content: '+'; }
.image-viewer-toolbar { position: absolute; left: 50%; display: flex; align-items: center; gap: 2px; transform: translateX(-50%); }
.image-viewer-window-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 2px; }
.image-viewer-toolbar,
.image-viewer-toolbar button,
.image-viewer-window-actions,
.image-viewer-window-actions button { -webkit-app-region: no-drag; --wails-draggable: no-drag; --wails-non-client-region: none; }
.image-viewer-toolbar button { display: inline-flex; width: 28px; height: 28px; padding: 0; align-items: center; justify-content: center; border: 0; border-radius: 6px; background: transparent; color: var(--viewer-muted); cursor: pointer; }
.image-viewer-window-actions button { display: inline-flex; width: 28px; height: 28px; padding: 0; align-items: center; justify-content: center; border: 0; border-radius: 6px; background: transparent; color: var(--viewer-muted); cursor: pointer; }
.image-viewer-toolbar button:hover:not(:disabled) { background: color-mix(in srgb, var(--viewer-text) 10%, transparent); color: var(--viewer-text); }
.image-viewer-window-actions button:hover:not(:disabled) { background: color-mix(in srgb, var(--viewer-text) 10%, transparent); color: var(--viewer-text); }
.image-viewer-toolbar button:disabled { opacity: .32; cursor: default; }
.image-viewer-window-actions button:disabled { opacity: .32; cursor: default; }
.image-viewer-window-actions .window-close:hover:not(:disabled) { background: #e5484d; color: #fff; }
.image-viewer-canvas { position: relative; display: flex; min-width: 0; min-height: 0; flex: 1; overflow: hidden; align-items: center; justify-content: center; --wails-draggable: no-drag; -webkit-app-region: no-drag; --wails-non-client-region: none; }
.image-viewer-canvas img { display: block; max-width: calc(100% - 24px); max-height: calc(100% - 24px); object-fit: contain; user-select: none; will-change: transform, opacity; transition: transform .08s linear; }
.image-viewer-canvas .preview-image { position: absolute; inset: 12px; width: calc(100% - 24px); height: calc(100% - 24px); margin: auto; opacity: 1; transition: opacity .16s ease, transform .08s linear; }
.image-viewer-canvas .preview-thumbnail { z-index: 1; }
.image-viewer-canvas .preview-original { z-index: 2; opacity: 0; }
.image-viewer-canvas .preview-original.preview-image-visible { opacity: 1; }
.image-viewer-canvas .preview-thumbnail.preview-image-faded { opacity: 0; }
.image-viewer-canvas .pdf-preview { width: calc(100% - 24px); height: calc(100% - 24px); border: 0; border-radius: 6px; background: #fff; }
.image-viewer-canvas .video-preview { width: calc(100% - 24px); height: calc(100% - 24px); max-width: calc(100% - 24px); max-height: calc(100% - 24px); border-radius: 6px; background: #050608; object-fit: contain; }
.image-viewer-canvas .video-poster { display: block; width: calc(100% - 24px); height: calc(100% - 24px); max-width: calc(100% - 24px); max-height: calc(100% - 24px); border-radius: 6px; background: #050608; object-fit: contain; user-select: none; }
.image-viewer-canvas .video-poster-empty { width: calc(100% - 24px); height: calc(100% - 24px); border-radius: 6px; background: #050608; }
.video-loading-state { position: absolute; z-index: 4; top: 50%; left: 50%; display: inline-flex; align-items: center; gap: 7px; padding: 8px 12px; transform: translate(-50%, -50%); border: 1px solid rgba(255, 255, 255, .18); border-radius: 6px; background: rgba(20, 24, 30, .78); color: #fff; font-size: 12px; pointer-events: none; }
.video-loading-state > svg { font-size: 15px; animation: video-loading-spin .9s linear infinite; }
@keyframes video-loading-spin { to { transform: rotate(360deg); } }
.video-play-button { position: absolute; z-index: 3; left: 50%; top: 50%; display: inline-flex; width: 64px; height: 64px; padding: 0; align-items: center; justify-content: center; transform: translate(-50%, -50%); border: 1px solid rgba(255, 255, 255, .78); border-radius: 50%; background: rgba(20, 24, 30, .78); color: #fff; cursor: pointer; box-shadow: 0 8px 24px rgba(0, 0, 0, .3); -webkit-app-region: no-drag; --wails-draggable: no-drag; --wails-non-client-region: none; }
.video-play-button:hover { background: rgba(55, 103, 232, .92); transform: translate(-50%, -50%) scale(1.05); }
.video-play-button:active { transform: translate(-50%, -50%) scale(.97); }
.video-play-button > svg { font-size: 34px; }
.image-nav-side { position: absolute; z-index: 2; top: 0; bottom: 0; display: flex; width: 88px; align-items: center; opacity: 0; pointer-events: auto; transition: opacity .18s ease; --wails-draggable: no-drag; -webkit-app-region: no-drag; --wails-non-client-region: none; }
.image-nav-side-prev { left: 0; justify-content: flex-start; padding-left: 14px; }
.image-nav-side-next { right: 0; justify-content: flex-end; padding-right: 14px; }
.image-nav-side:hover { opacity: 1; }
.image-nav-arrow { display: inline-flex; width: 42px; height: 72px; padding: 0; align-items: center; justify-content: center; border: 1px solid color-mix(in srgb, var(--viewer-line) 80%, transparent); border-radius: 10px; background: color-mix(in srgb, var(--viewer-surface) 88%, transparent); color: var(--viewer-text); cursor: pointer; }
.image-nav-arrow:hover:not(:disabled) { background: color-mix(in srgb, var(--viewer-text) 12%, var(--viewer-surface)); }
.image-nav-arrow:disabled { opacity: .25; cursor: default; }
.image-viewer-state { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 10px; color: #d8e2f2; font-size: 13px; pointer-events: none; }
.image-viewer-state > svg { font-size: 30px; color: #84a8ff; }
.image-viewer-state.error { color: #ffb4b4; }
.image-viewer-state.error > svg { color: #ff8b8b; }
.image-viewer-quality { position: absolute; right: 16px; bottom: 14px; z-index: 4; display: inline-flex; align-items: center; gap: 5px; padding: 5px 9px; border-radius: 999px; background: color-mix(in srgb, var(--viewer-surface) 86%, transparent); color: var(--viewer-muted); font-size: 11px; pointer-events: none; }
.image-viewer-quality > svg { font-size: 13px; }
.image-viewer-quality.error { color: #c34d55; }
@media (max-width: 720px), (max-height: 540px) { .image-viewer-head { min-height: 34px; flex-basis: 34px; padding-top: 3px; padding-bottom: 3px; } .image-viewer-app.is-macos .image-viewer-head { padding-left: 16px; } .image-viewer-toolbar button, .image-viewer-window-actions button { width: 26px; height: 26px; } .mac-traffic-light { width: 11px; height: 11px; } }
</style>
