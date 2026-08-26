<template>
  <main class="image-viewer-app" :class="{ dark: isDark, 'is-macos': isMac, 'is-windows': isWindows }">
    <header class="image-viewer-head" @dblclick="isWindows && toggleWindowMaximise()">
      <div class="image-viewer-toolbar" role="toolbar" aria-label="图片工具栏" @dblclick.stop>
        <button type="button" aria-label="上一张" title="上一张" :disabled="currentIndex <= 0 || loading" @click="moveImage(-1)"><icon-left /></button>
        <button type="button" aria-label="下一张" title="下一张" :disabled="currentIndex >= imageMessages.length - 1 || loading" @click="moveImage(1)"><icon-right /></button>
        <button type="button" aria-label="缩小" title="缩小" :disabled="loading || scale <= .5" @click="zoomImage(-.25)"><icon-zoom-out /></button>
        <button type="button" aria-label="放大" title="放大" :disabled="loading || scale >= 6" @click="zoomImage(.25)"><icon-zoom-in /></button>
        <button type="button" aria-label="向左旋转90度" title="向左旋转90°" :disabled="loading || !source" @click="rotateImageLeft"><icon-rotate-left /></button>
      </div>
      <div class="image-viewer-window-actions" @dblclick.stop>
        <template v-if="isWindows">
          <button type="button" aria-label="最小化" title="最小化" @click.stop="Window.Minimise()"><icon-minus /></button>
          <button type="button" aria-label="最大化或还原" title="最大化或还原" @click.stop="toggleWindowMaximise"><icon-fullscreen /></button>
        </template>
        <button type="button" class="window-close" aria-label="关闭图片查看器" title="关闭" @click.stop="closeViewer"><icon-close /></button>
      </div>
    </header>

    <section class="image-viewer-canvas" @wheel.prevent.stop="handleWheel" @dblclick.prevent.stop="toggleZoom" @pointerdown.stop="handlePointerDown" @pointermove.stop="handlePointerMove" @pointerup.stop="handlePointerUp" @pointercancel.stop="handlePointerUp">
      <div v-if="imageMessages.length > 1" class="image-nav-side image-nav-side-prev">
        <button type="button" class="image-nav-arrow" aria-label="上一张" title="上一张" :disabled="currentIndex <= 0 || loading" @pointerdown.stop @click.stop="moveImage(-1)"><icon-left /></button>
      </div>
      <img v-if="source" :src="source" :alt="imageViewerName" :style="imageTransform" draggable="false" />
      <div v-if="imageMessages.length > 1" class="image-nav-side image-nav-side-next">
        <button type="button" class="image-nav-arrow" aria-label="下一张" title="下一张" :disabled="currentIndex >= imageMessages.length - 1 || loading" @pointerdown.stop @click.stop="moveImage(1)"><icon-right /></button>
      </div>
      <div v-if="loading" class="image-viewer-state"><icon-loading /><span>正在读取图片</span></div>
      <div v-else-if="error" class="image-viewer-state error"><icon-close-circle /><strong>{{ error }}</strong></div>
      <div v-else-if="!imageMessages.length" class="image-viewer-state"><icon-close-circle /><strong>没有可预览的图片</strong></div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Events, Window } from '@wailsio/runtime'
import { ChatService } from '/#/flyqpro/internal/service'
import { IconClose, IconCloseCircle, IconFullscreen, IconLeft, IconLoading, IconMinus, IconRight, IconRotateLeft, IconZoomIn, IconZoomOut } from '@arco-design/web-vue/es/icon'

const route = useRoute()
const conversationId = computed(() => String(route.query.conversationId || ''))
const initialMessageId = computed(() => String(route.query.messageId || ''))
const messages = ref<any[]>([])
const currentIndex = ref(0)
const currentMessageId = ref('')
const source = ref('')
const sourceType = ref<'thumbnail' | 'original'>('thumbnail')
const loading = ref(false)
const error = ref('')
const scale = ref(1)
const rotation = ref(0)
const offset = ref({ x: 0, y: 0 })
const isDark = ref(false)
const isMac = /Macintosh|Mac OS X|MacIntel/i.test(`${navigator.platform || ''} ${navigator.userAgent || ''}`)
const isWindows = /Win32|Windows/i.test(`${navigator.platform || ''} ${navigator.userAgent || ''}`)
const cache = new Map<string, { source: string; type: 'thumbnail' | 'original' }>()
const imageMessages = computed(() => messages.value.filter((message) => message.kind === 'file' && message.attachmentId && isImageMessage(message)))
const currentMessage = computed(() => imageMessages.value[currentIndex.value])
const imageViewerName = computed(() => currentMessage.value?.attachmentName || '图片预览')
const imageTransform = computed(() => ({
  transform: `translate(${offset.value.x}px, ${offset.value.y}px) rotate(${rotation.value}deg) scale(${scale.value})`,
  cursor: scale.value > 1 ? 'grab' : 'zoom-in',
}))

let loadToken = 0
let wheelX = 0
let wheelResetTimer = 0
let wheelSwitchAt = 0
let reloadTimer = 0
let zoomFrame = 0
let zoomTarget = 1
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
function toggleZoom() { if (scale.value > 1) setZoomTarget(1); else setZoomTarget(2) }
function rotateImageLeft() { rotation.value = (rotation.value - 90) % 360 }

function readImage(sourceValue: string) {
  return new Promise<boolean>((resolve) => {
    const image = new Image()
    image.onload = () => resolve(true)
    image.onerror = () => resolve(false)
    image.src = sourceValue
  })
}

async function sourceFor(message: any, token: number) {
  const original = completed(message)
  const type: 'thumbnail' | 'original' = original ? 'original' : 'thumbnail'
  const cacheKey = `${message.attachmentId}:${type}`
  const cached = cache.get(cacheKey)
  if (cached) return cached.source
  let sourceValue = ''
  if (original) {
    try { sourceValue = await ChatService.GetAttachmentImage(message.attachmentId) } catch { sourceValue = '' }
  } else {
    sourceValue = thumbnailDataURL(message)
    if (!sourceValue) {
      try { sourceValue = await ChatService.GetAttachmentThumbnail(message.attachmentId) } catch { sourceValue = '' }
    }
  }
  if (token !== loadToken) return ''
  if (sourceValue) cache.set(cacheKey, { source: sourceValue, type })
  return sourceValue
}

function pruneCache() {
  const keep = new Set(imageMessages.value.slice(Math.max(0, currentIndex.value - 1), currentIndex.value + 2).map((message) => message.attachmentId))
  for (const key of cache.keys()) if (!keep.has(key.split(':')[0])) cache.delete(key)
}

async function loadCurrent() {
  const message = currentMessage.value
  const token = ++loadToken
  loading.value = true
  error.value = ''
  source.value = ''
  sourceType.value = completed(message) ? 'original' : 'thumbnail'
  if (!message?.attachmentId) {
    loading.value = false
    error.value = '图片附件不存在'
    return
  }
  const sourceValue = await sourceFor(message, token)
  if (token !== loadToken) return
  if (!sourceValue) {
    loading.value = false
    error.value = completed(message) ? '本地原图不存在' : '图片预览暂时无法读取'
    return
  }
  const loaded = await readImage(sourceValue)
  if (token !== loadToken) return
  loading.value = false
  if (!loaded) error.value = completed(message) ? '本地原图无法打开' : '图片预览无法打开'
  else source.value = sourceValue
  void Window.SetTitle(`图片预览 - ${imageViewerName.value}`).catch(() => undefined)
  pruneCache()
}

async function loadMessages() {
  if (!conversationId.value) {
    messages.value = []
    currentIndex.value = 0
    currentMessageId.value = ''
    await loadCurrent()
    return
  }
  try {
    const loaded = await ChatService.ListMessages(conversationId.value)
    messages.value = loaded || []
    const wanted = currentMessageId.value || initialMessageId.value
    const index = imageMessages.value.findIndex((message) => message.messageId === wanted)
    currentIndex.value = index >= 0 ? index : Math.min(currentIndex.value, Math.max(0, imageMessages.value.length - 1))
    currentMessageId.value = imageMessages.value[currentIndex.value]?.messageId || ''
    resetTransform()
    await loadCurrent()
  } catch {
    messages.value = []
    loading.value = false
    error.value = '聊天记录读取失败'
  }
}

async function moveImage(direction: number) {
  if (loading.value) return
  const target = Math.max(0, Math.min(imageMessages.value.length - 1, currentIndex.value + direction))
  if (target === currentIndex.value || !imageMessages.value[target]) return
  currentIndex.value = target
  currentMessageId.value = imageMessages.value[target].messageId
  resetTransform()
  await loadCurrent()
}

function closeViewer() { void Window.Close() }
function toggleWindowMaximise() { void Window.ToggleMaximise() }

function handleWheel(event: WheelEvent) {
  // Chromium exposes macOS trackpad pinch as a ctrl+wheel stream. Applying
  // the delta continuously keeps the gesture proportional instead of turning
  // it into a series of visible .15x jumps.
  if (event.ctrlKey) {
    const delta = Math.abs(event.deltaY) >= Math.abs(event.deltaX) ? event.deltaY : event.deltaX
    if (Math.abs(delta) >= .01) zoomByPinch(delta)
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
    if (Math.abs(wheelX) >= 360 && performance.now() - wheelSwitchAt > 700) {
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
    if (scale.value > 1) offset.value = { x: offset.value.x + center.x - multiGesture.lastX, y: offset.value.y + center.y - multiGesture.lastY }
    multiGesture.lastX = center.x
    multiGesture.lastY = center.y
    return
  }
  if (pointers.size === 2 && multiGesture?.type === 'pinch') {
    const distance = pointerDistance()
    if (distance > 0 && multiGesture.initialDistance > 0) setZoomTarget(multiGesture.initialScale * distance / multiGesture.initialDistance)
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
  multiPointerGesture = false
  multiGesture = undefined
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
  if (event.key === 'Escape' || event.key === ' ' || event.code === 'Space') { event.preventDefault(); closeViewer(); return }
  if (event.key === 'ArrowLeft') { event.preventDefault(); void moveImage(-1) }
  if (event.key === 'ArrowRight') { event.preventDefault(); void moveImage(1) }
  if (event.key === '+' || event.key === '=') { event.preventDefault(); zoomImage(.25) }
  if (event.key === '-' || event.key === '_') { event.preventDefault(); zoomImage(-.25) }
}

watch([conversationId, initialMessageId], () => { currentMessageId.value = initialMessageId.value; void loadMessages() }, { immediate: true })

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
.image-viewer-head { position: relative; display: flex; min-height: 50px; padding: 8px 10px 8px 16px; box-sizing: border-box; align-items: center; justify-content: flex-end; gap: 12px; flex: 0 0 50px; background: var(--viewer-surface); border-bottom: 1px solid var(--viewer-line); cursor: grab; -webkit-app-region: drag; --wails-draggable: drag; }
.image-viewer-app.is-macos .image-viewer-head { padding-left: 84px; }
.image-viewer-toolbar { position: absolute; left: 50%; display: flex; align-items: center; gap: 2px; transform: translateX(-50%); }
.image-viewer-window-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 2px; }
.image-viewer-toolbar,
.image-viewer-toolbar button,
.image-viewer-window-actions,
.image-viewer-window-actions button { -webkit-app-region: no-drag; --wails-draggable: no-drag; --wails-non-client-region: none; }
.image-viewer-toolbar button { display: inline-flex; width: 32px; height: 32px; padding: 0; align-items: center; justify-content: center; border: 0; border-radius: 7px; background: transparent; color: var(--viewer-muted); cursor: pointer; }
.image-viewer-window-actions button { display: inline-flex; width: 32px; height: 32px; padding: 0; align-items: center; justify-content: center; border: 0; border-radius: 7px; background: transparent; color: var(--viewer-muted); cursor: pointer; }
.image-viewer-toolbar button:hover:not(:disabled) { background: color-mix(in srgb, var(--viewer-text) 10%, transparent); color: var(--viewer-text); }
.image-viewer-window-actions button:hover:not(:disabled) { background: color-mix(in srgb, var(--viewer-text) 10%, transparent); color: var(--viewer-text); }
.image-viewer-toolbar button:disabled { opacity: .32; cursor: default; }
.image-viewer-window-actions button:disabled { opacity: .32; cursor: default; }
.image-viewer-window-actions .window-close:hover:not(:disabled) { background: #e5484d; color: #fff; }
.image-viewer-canvas { position: relative; display: flex; min-width: 0; min-height: 0; flex: 1; overflow: hidden; align-items: center; justify-content: center; --wails-draggable: no-drag; -webkit-app-region: no-drag; --wails-non-client-region: none; }
.image-viewer-canvas img { display: block; max-width: calc(100% - 24px); max-height: calc(100% - 24px); object-fit: contain; user-select: none; will-change: transform; transition: transform .08s linear; }
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
@media (max-width: 720px), (max-height: 540px) { .image-viewer-head { min-height: 44px; flex-basis: 44px; padding-top: 5px; padding-bottom: 5px; } .image-viewer-app.is-macos .image-viewer-head { padding-left: 76px; } .image-viewer-toolbar button, .image-viewer-window-actions button { width: 28px; height: 28px; } }
</style>
