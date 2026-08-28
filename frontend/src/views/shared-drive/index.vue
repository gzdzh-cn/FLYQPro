<template>
  <div class="shared-drive" :class="{ dark: isDark, mac: isMac, embedded }">
    <div v-if="isMac && !embedded" class="mac-controls" aria-label="窗口控制"><button class="close" @click="closeWindow" /><button class="minimise" @click="Window.Minimise" /><button class="maximise" @click="Window.ToggleMaximise" /></div>
    <header class="shared-head" :class="{ draggable: isMac && !embedded }">
      <div class="head-title"><IconCloud /><div><strong>{{ mode === 'owner' ? '共享' : mode === 'friend' ? '好友共享' : '共享' }}</strong><span>{{ mode === 'owner' ? '管理本机共享文件夹' : mode === 'friend' ? '查看好友共享文件夹' : '共享窗口参数无效' }}</span></div></div>
    </header>
    <main class="shared-body">
      <section v-if="mode === 'owner'" class="owner-summary card">
        <div class="summary-main"><div class="summary-label">共享开关</div><strong>{{ settings.enabled ? '已开启' : '已关闭' }}</strong><span>{{ settings.enabled ? '好友可以浏览和下载共享目录' : '开启后好友才可以访问' }}</span></div>
        <a-switch :model-value="settings.enabled" @change="toggleEnabled" />
        <div class="summary-item"><span>共享文件</span><strong>{{ formatStatNumber(settings.fileCount) }}</strong></div><div class="summary-item"><span>共享文件夹</span><strong>{{ formatStatNumber(settings.folderCount) }}</strong></div><div class="summary-item"><span>磁盘剩余</span><strong>{{ formatAvailableBytes() }}</strong></div>
      </section>
      <div class="shared-fixed-header">
        <section v-if="mode === 'owner'" class="toolbar management-toolbar card"><div class="path-info"><span>共享目录</span><strong :title="settings.rootPath">{{ settings.rootPath || '未设置' }}</strong></div><button @click="chooseRoot">选择共享目录</button><button :disabled="!settings.rootPath" @click="refresh">刷新</button><button :disabled="!settings.rootPath" @click="createFolder">新建文件夹</button><button :disabled="!settings.rootPath" @click="importFiles">导入文件</button><button :disabled="!settings.rootPath" @click="importFolder">导入文件夹</button><button :disabled="!settings.rootPath" @click="openOwnerFolder">在文件管理器中打开</button></section>
        <section v-else-if="mode === 'invalid'" class="remote-banner card"><IconCloud /><div><strong>共享窗口参数无效</strong><span>无法确定要访问的好友设备，请从好友聊天窗口重新打开</span></div></section>

        <nav class="breadcrumbs card">
          <div class="breadcrumb-path"><button :disabled="!relativePath" @click="goParent">上一级</button><button :class="{ active: !relativePath }" @click="openPath('')">共享根目录</button><template v-for="(part, index) in pathParts" :key="part + index"><span>/</span><button @click="openPath(pathParts.slice(0, index + 1).join('/'))">{{ part }}</button></template></div>
          <div class="breadcrumb-actions" @click.stop>
            <button class="icon-action" :title="searchVisible ? '关闭搜索' : '搜索'" @click="searchVisible = !searchVisible"><IconSearch /></button>
            <button @click="selectAll">{{ selected.size === entries.length && entries.length ? '取消全选' : '全选' }}</button>
            <button v-if="mode === 'friend'" :disabled="!selectedFiles.length || sharedDisabled" @click="downloadSelected">下载</button>
            <button v-if="mode === 'friend'" :disabled="!selectedFiles.length || sharedDisabled" @click="saveSelected">另存为</button>
            <button :disabled="loading" @click="refresh">刷新</button>
            <button class="icon-action" :class="{ active: viewMode === 'list' }" title="列表视图" @click="viewMode = 'list'"><IconList /></button>
            <button class="icon-action" :class="{ active: viewMode === 'thumb' }" title="缩略图视图" @click="viewMode = 'thumb'"><IconApps /></button>
            <span class="view-note">{{ filteredEntries.length }} 项</span>
          </div>
          <div v-if="searchVisible" class="breadcrumb-search" @click.stop>
            <IconSearch />
            <input v-model="search" autofocus placeholder="搜索文件和文件夹" @keydown.esc="searchVisible = false" />
            <button v-if="search" class="icon-action" title="清空搜索" @click="search = ''"><IconClose /></button>
          </div>
        </nav>
      </div>
      <section class="file-panel card" @contextmenu.prevent>
        <div v-if="loading" class="empty-state">正在读取共享目录…</div>
        <div v-else-if="mode === 'invalid'" class="empty-state"><IconCloud /><strong>共享窗口参数无效</strong><span>无法确定要访问的好友设备</span></div>
        <div v-else-if="sharedDisabled" class="empty-state"><IconCloud /><strong>{{ mode === 'owner' ? (settings.rootPath ? '共享已关闭' : '请先选择共享目录') : '对方已关闭共享' }}</strong><span>{{ mode === 'owner' ? '本机仍可管理共享目录，开启开关后好友才可访问' : '共享开关开启后，刷新即可继续访问' }}</span></div>
        <div v-else-if="!filteredEntries.length" class="empty-state"><IconFolder /><span>此文件夹为空</span></div>
        <div v-else-if="viewMode === 'thumb'" class="file-list thumb-grid">
          <div v-for="entry in filteredEntries" :key="entry.entryId" class="thumb-card" :class="{ selected: selected.has(entry.relativePath) }" :data-thumbnail-path="entry.relativePath" :ref="(element) => registerThumbnailElement(element, entry)" @dblclick="entry.isDirectory ? openPath(entry.relativePath) : preview(entry)" @contextmenu.prevent.stop="openContext($event, entry)">
            <input v-if="mode === 'friend'" class="thumb-check" type="checkbox" :checked="selected.has(entry.relativePath)" @click.stop="toggleSelected(entry)" />
            <div class="thumb-preview">
              <img v-if="thumbnailUrls[entry.relativePath]" :src="thumbnailUrls[entry.relativePath]" :alt="entry.name" loading="lazy" decoding="async" />
              <span v-else-if="entry.isDirectory" class="thumb-placeholder folder"><IconFolder /></span>
              <span v-else-if="isImageEntry(entry) && thumbnailLoading.has(entry.relativePath)" class="thumb-placeholder loading">加载中…</span>
              <span v-else-if="isImageEntry(entry)" class="thumb-placeholder"><IconFile /></span>
              <span v-else class="thumb-placeholder"><IconFile /></span>
            </div>
            <strong class="thumb-name" :title="entry.name">{{ entry.name }}</strong>
            <span class="thumb-meta">{{ entry.isDirectory ? '文件夹' : formatBytes(entry.size) }}</span>
          </div>
        </div>
        <div v-else class="file-list">
          <div v-for="entry in filteredEntries" :key="entry.entryId" class="file-row" :class="{ selected: selected.has(entry.relativePath) }" :data-thumbnail-path="entry.relativePath" :ref="(element) => registerThumbnailElement(element, entry)" @dblclick="entry.isDirectory ? openPath(entry.relativePath) : preview(entry)" @contextmenu.prevent.stop="openContext($event, entry)">
            <input v-if="mode === 'friend'" type="checkbox" :checked="selected.has(entry.relativePath)" @click.stop="toggleSelected(entry)" />
            <IconFolder v-if="entry.isDirectory" class="file-icon folder" /><img v-else-if="isImageEntry(entry) && thumbnailUrls[entry.relativePath]" class="file-icon file-thumb" :src="thumbnailUrls[entry.relativePath]" :alt="entry.name" loading="lazy" decoding="async" /><IconFile v-else class="file-icon" />
            <div class="file-name"><strong>{{ entry.name }}</strong><span>{{ entry.isDirectory ? '文件夹' : formatBytes(entry.size) }}</span></div><span class="file-date">{{ formatDate(entry.modifiedAt) }}</span><button class="row-more" @click.stop="openContext($event, entry)">···</button>
          </div>
        </div>
        <div v-if="hasMore" class="load-more"><button :disabled="loadingMore" @click="loadMore">{{ loadingMore ? '正在加载…' : '继续加载更多文件' }}</button></div>
      </section>
      <section v-if="Object.keys(transfers).length" class="transfer-float" :class="{ collapsed: !transferExpanded }">
        <button class="transfer-header" @click="transferExpanded = !transferExpanded">
          <span class="transfer-title"><strong>下载任务</strong><small>{{ transferSummary }}</small></span>
          <IconUp v-if="transferExpanded" /><IconDown v-else />
        </button>
        <div v-if="transferExpanded" class="transfer-body">
          <div v-for="transfer in transfers" :key="transfer.transferId" class="transfer-row">
            <div class="transfer-main"><strong>{{ transfer.fileName || transfer.relativePath }}</strong><span>{{ transferStatusText(transfer) }}</span><div class="transfer-progress"><i :style="{ width: `${transferPercent(transfer)}%` }" /></div></div>
            <div class="transfer-actions"><strong>{{ transferPercent(transfer) }}%</strong><button v-if="['starting', 'transferring'].includes(transfer.status)" @click.stop="pauseTransfer(transfer.transferId)">暂停</button><button v-if="['paused', 'failed'].includes(transfer.status)" @click.stop="resumeTransfer(transfer.transferId)">继续</button><button v-if="!['completed', 'canceled'].includes(transfer.status)" @click.stop="cancelTransfer(transfer.transferId)">取消</button></div>
          </div>
        </div>
      </section>
    </main>
    <div v-if="context.visible && context.entry" class="context-menu" :style="{ left: context.x + 'px', top: context.y + 'px' }" @pointerdown.stop @click.stop>
      <template v-if="mode === 'owner'"><button v-if="context.entry.isDirectory" @click="newContextFolder">打开</button><button v-if="!context.entry.isDirectory && isPreviewable(context.entry)" @click="preview(context.entry)">预览</button><button @click="renameEntry">重命名</button><button @click="deleteEntry" class="danger">删除</button><button @click="copyRelative">复制相对路径</button><button @click="showDetails">属性</button></template>
      <template v-else><button v-if="context.entry.isDirectory" @click="openPath(context.entry.relativePath)">打开</button><button v-if="!context.entry.isDirectory && isPreviewable(context.entry)" @click="preview(context.entry)">预览</button><button v-if="!context.entry.isDirectory" @click="downloadEntry">下载</button><button v-if="!context.entry.isDirectory" @click="saveEntry">另存为</button><button v-if="!context.entry.isDirectory" @click="copyName">复制文件名</button><button @click="copyRelative">复制相对路径</button><button @click="showDetails">详情</button></template>
    </div>
    <a-modal v-model:visible="renameDialog.visible" title="重命名" :footer="false" :mask-closable="false" @cancel="cancelRename">
      <div class="rename-dialog">
        <label>新名称</label>
        <a-input v-model="renameDialog.name" allow-clear placeholder="请输入新的文件或文件夹名称" @press-enter="confirmRename" />
        <div class="rename-actions"><a-button @click="cancelRename">取消</a-button><a-button type="primary" :loading="renameDialog.loading" @click="confirmRename">确定</a-button></div>
      </div>
    </a-modal>
    <a-modal v-model:visible="detailsDialog.visible" title="文件属性" :footer="false" :mask-closable="true">
      <div v-if="detailsDialog.entry" class="details-dialog">
        <div class="detail-row"><span>名称</span><strong>{{ detailsDialog.entry.name }}</strong></div>
        <div class="detail-row"><span>类型</span><strong>{{ detailsDialog.entry.isDirectory ? '文件夹' : (detailsDialog.entry.mimeType || '文件') }}</strong></div>
        <div class="detail-row"><span>大小</span><strong>{{ detailsDialog.loading ? '统计中…' : formatBytes(detailsDialog.entry.size) }}</strong></div>
        <div class="detail-row"><span>修改时间</span><strong>{{ formatDate(detailsDialog.entry.modifiedAt) }}</strong></div>
        <div v-if="detailsDialog.entry.sha256" class="detail-row"><span>SHA256</span><strong class="detail-value">{{ detailsDialog.entry.sha256 }}</strong></div>
        <div v-if="detailsDialog.loading" class="detail-loading">正在统计文件夹及子目录大小，请稍候…</div>
        <div v-if="detailsDialog.error" class="detail-error">{{ detailsDialog.error }}</div>
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconApps, IconCloud, IconClose, IconDown, IconFile, IconFolder, IconList, IconSearch, IconUp } from '@arco-design/web-vue/es/icon'
import { Clipboard, Events, System, Window } from '@wailsio/runtime'
import { ChatService, ImageViewerService, SharedDriveWindowService } from '/#/flyqpro/internal/service'

interface Entry { entryId: string; name: string; relativePath: string; isDirectory: boolean; size: number; mimeType: string; modifiedAt: string; sha256?: string }
type SharedMode = 'owner' | 'friend' | 'invalid'
type SharedViewMode = 'list' | 'thumb'
function readSharedQuery() {
  const hashQuery = location.hash.includes('?') ? location.hash.slice(location.hash.indexOf('?') + 1) : ''
  const candidates = [location.search, hashQuery].filter(Boolean).map((value) => new URLSearchParams(value))
  const query = candidates.find((candidate) => candidate.has('mode') || candidate.has('deviceId')) || new URLSearchParams()
  const requestedMode = query.get('mode')
  const requestedDeviceId = query.get('deviceId')?.trim() || ''
  const parsedMode: SharedMode = requestedMode === 'owner' ? 'owner' : requestedMode === 'friend' && requestedDeviceId ? 'friend' : 'invalid'
  return { mode: parsedMode, deviceId: requestedDeviceId, embedded: query.get('embedded') === '1' }
}
const sharedQuery = readSharedQuery()
const props = withDefaults(defineProps<{ embeddedMode?: boolean; ownerMode?: boolean; friendDeviceId?: string }>(), { embeddedMode: false, ownerMode: true, friendDeviceId: '' })
const embedded = props.embeddedMode || sharedQuery.embedded
const mode = ref<SharedMode>(props.embeddedMode ? (props.ownerMode ? 'owner' : props.friendDeviceId ? 'friend' : 'invalid') : sharedQuery.mode)
const deviceId = props.embeddedMode ? props.friendDeviceId.trim() : sharedQuery.deviceId
const isMac = ref(false); const isDark = ref(false); const loading = ref(false); const loadingMore = ref(false); const hasMore = ref(false); const nextOffset = ref(0); const sharedDisabled = ref(false); const search = ref(''); const searchVisible = ref(false); const relativePath = ref(''); const entries = ref<Entry[]>([]); const selected = reactive(new Set<string>()); const peerName = ref(''); const viewMode = ref<SharedViewMode>('list')
const thumbnailUrls = reactive<Record<string, string>>({})
const thumbnailLoading = reactive(new Set<string>())
const thumbnailFailed = reactive(new Set<string>())
const thumbnailQueued = new Set<string>()
const thumbnailQueue: Array<{ entry: Entry; generation: number; attempt: number }> = []
let thumbnailActive = 0
let thumbnailGeneration = 0
let thumbnailBatchTimer: number | undefined
let thumbnailObserver: IntersectionObserver | undefined
const settings = reactive({ enabled: false, rootPath: '', fileCount: 0, folderCount: 0, availableBytes: 0, statsLoading: false, statsReady: false, statsUpdatedAt: '' })
const context = reactive({ visible: false, x: 0, y: 0, entry: undefined as Entry | undefined })
const renameDialog = reactive({ visible: false, loading: false, name: '', entry: undefined as Entry | undefined })
const detailsDialog = reactive({ visible: false, loading: false, error: '', entry: undefined as Entry | undefined })
const transfers = reactive<Record<string, any>>({})
const dismissedTransfers = reactive(new Set<string>())
const notifiedTransferFailures = reactive(new Set<string>())
const notifiedTransferCompletions = reactive(new Set<string>())
const transferExpanded = ref(true)
let cancelSharedEvents: (() => void) | undefined
let cancelThemeEvent: (() => void) | undefined
const pathParts = computed(() => relativePath.value ? relativePath.value.split('/').filter(Boolean) : [])
const filteredEntries = computed(() => { const key = search.value.trim().toLowerCase(); return entries.value.filter((entry) => !key || entry.name.toLowerCase().includes(key)) })
const selectedFiles = computed(() => entries.value.filter((entry) => selected.has(entry.relativePath) && !entry.isDirectory))
const transferSummary = computed(() => {
  const all = Object.values(transfers)
  const active = all.filter((transfer) => ['starting', 'transferring'].includes(transfer.status)).length
  const completed = all.filter((transfer) => transfer.status === 'completed').length
  return active ? `${active} 个下载中${completed ? `，已完成 ${completed} 个` : ''}` : `已完成 ${completed} 个`
})
function formatBytes(value: number) { if (!value) return '—'; if (value < 1024) return `${value} B`; if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`; if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`; return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB` }
function formatStatNumber(value: number) { if (settings.statsLoading) return value > 0 ? `${value}（统计中…）` : '统计中…'; return settings.statsReady ? String(value) : '—' }
function formatAvailableBytes() { return settings.statsLoading ? (settings.availableBytes > 0 ? `${formatBytes(settings.availableBytes)}（统计中…）` : '统计中…') : settings.statsReady ? formatBytes(settings.availableBytes) : '—' }
function formatDate(value: string) { return value ? new Date(value).toLocaleString() : '—' }
function isImageEntry(entry: Entry) {
  const mime = String(entry.mimeType || '').toLowerCase()
  return mime.startsWith('image/') || /\.(avif|bmp|gif|heic|heif|jpe?g|png|webp)$/i.test(entry.name)
}
function registerThumbnailElement(element: unknown, entry: Entry) {
  if (!(element instanceof HTMLElement) || !thumbnailObserver || !isImageEntry(entry)) return
  element.dataset.thumbnailPath = entry.relativePath
  thumbnailObserver.observe(element)
}
function scheduleInitialThumbnails() {
  void nextTick(() => {
    // Generate only the first screen eagerly. The intersection observer adds
    // nearby rows as the user scrolls, in either list or thumbnail mode.
    filteredEntries.value.slice(0, viewMode.value === 'thumb' ? 16 : 24).forEach((entry) => enqueueThumbnail(entry))
  })
}
function enqueueThumbnail(entry: Entry) {
  const path = entry.relativePath
  if (!isImageEntry(entry) || thumbnailUrls[path] || thumbnailLoading.has(path) || thumbnailFailed.has(path) || thumbnailQueued.has(path)) return
  thumbnailQueued.add(path)
  thumbnailLoading.add(path)
  thumbnailQueue.push({ entry, generation: thumbnailGeneration, attempt: 0 })
  void drainThumbnailQueue()
}
async function drainThumbnailQueue() {
  if (mode.value === 'friend') {
    if (thumbnailBatchTimer !== undefined) return
    thumbnailBatchTimer = window.setTimeout(() => {
      thumbnailBatchTimer = undefined
      void drainFriendThumbnailQueue()
    }, 35)
    return
  }
  while (thumbnailActive < 3 && thumbnailQueue.length) {
    const item = thumbnailQueue.shift()
    if (!item) return
    thumbnailQueued.delete(item.entry.relativePath)
    thumbnailActive++
    void loadThumbnail(item.entry, item.generation).finally(() => {
      thumbnailActive--
      void drainThumbnailQueue()
    })
  }
}
async function drainFriendThumbnailQueue() {
  while (thumbnailActive < 2 && thumbnailQueue.length) {
    const first = thumbnailQueue.shift()
    if (!first) return
    const batch = [first]
    while (batch.length < 16 && thumbnailQueue.length) {
      const next = thumbnailQueue.shift()
      if (next) batch.push(next)
    }
    batch.forEach((item) => thumbnailQueued.delete(item.entry.relativePath))
    thumbnailActive++
    void loadThumbnailBatch(batch, first.generation).finally(() => {
      thumbnailActive--
      void drainFriendThumbnailQueue()
    })
  }
}
function retryThumbnail(item: { entry: Entry; generation: number; attempt: number }) {
  if (item.generation !== thumbnailGeneration || item.attempt >= 4) {
    if (item.generation === thumbnailGeneration) {
      thumbnailLoading.delete(item.entry.relativePath)
      thumbnailFailed.add(item.entry.relativePath)
    }
    return
  }
  const delays = [300, 700, 1500, 3000]
  window.setTimeout(() => {
    if (item.generation !== thumbnailGeneration || thumbnailUrls[item.entry.relativePath]) return
    thumbnailQueued.add(item.entry.relativePath)
    thumbnailQueue.push({ ...item, attempt: item.attempt + 1 })
    void drainThumbnailQueue()
  }, delays[item.attempt] || 3000)
}
async function loadThumbnailBatch(items: Array<{ entry: Entry; generation: number; attempt: number }>, generation: number) {
  if (generation !== thumbnailGeneration) return
  try {
    const results = await ChatService.GetFriendSharedEntryThumbnails(deviceId, items.map(({ entry }) => ({
      relativePath: entry.relativePath,
      entryId: entry.entryId,
      fileSize: entry.size,
      modifiedAt: entry.modifiedAt,
    })))
    if (generation !== thumbnailGeneration) return
    const byPath = new Map((results || []).map((result: any) => [result.relativePath, result]))
    items.forEach((item) => {
      const path = item.entry.relativePath
      const result: any = byPath.get(path)
      if (result?.status === 'ready' && result.payload) {
        const mime = result.thumbnailMime || result.mimeType || 'image/jpeg'
        thumbnailUrls[path] = `data:${mime};base64,${result.payload}`
        thumbnailLoading.delete(path)
      } else if (result?.status === 'pending') {
        retryThumbnail(item)
      } else {
        thumbnailLoading.delete(path)
        thumbnailFailed.add(path)
      }
    })
  } catch {
    items.forEach((item) => retryThumbnail(item))
  }
}
async function loadThumbnail(entry: Entry, generation: number) {
  const path = entry.relativePath
  // A cache miss starts generation on the backend and returns immediately.
  // Poll briefly so the visible row upgrades from placeholder to thumbnail
  // without making the initial directory request wait for image decoding.
  // A transient remote connection error is retried instead of permanently
  // disabling the thumbnail for the rest of this directory view.
  try {
    for (let attempt = 0; attempt < 24; attempt++) {
      let url = ''
      try {
        url = mode.value === 'owner'
          ? await ChatService.GetSharedEntryThumbnail(path)
          : mode.value === 'friend'
            ? await ChatService.GetFriendSharedEntryThumbnail(deviceId, path)
            : ''
      } catch {
        url = ''
      }
      if (generation !== thumbnailGeneration) return
      if (url) {
        thumbnailUrls[path] = url
        return
      }
      await new Promise((resolve) => window.setTimeout(resolve, Math.min(750, 220 + attempt * 25)))
    }
    if (generation === thumbnailGeneration) thumbnailFailed.add(path)
  } catch {
    if (generation === thumbnailGeneration) thumbnailFailed.add(path)
  } finally {
    if (generation === thumbnailGeneration) thumbnailLoading.delete(path)
  }
}
function transferPercent(transfer: any) { return transfer.fileSize > 0 ? Math.min(100, Math.floor((transfer.transferred || 0) * 100 / transfer.fileSize)) : 0 }
function transferStatusText(transfer: any) {
  if (transfer.status === 'starting') return '正在连接…'
  if (transfer.status === 'transferring') return `${formatBytes(transfer.transferred || 0)} / ${formatBytes(transfer.fileSize || 0)}`
  if (transfer.status === 'paused') return `已暂停 · ${formatBytes(transfer.transferred || 0)} / ${formatBytes(transfer.fileSize || 0)}`
  if (transfer.status === 'completed') return '下载完成'
  if (transfer.status === 'canceled') return '已取消'
  if (transfer.status === 'failed') return `下载失败：${transfer.errorMessage || '连接或文件校验失败'}`
  return '等待下载…'
}
function registerTransfer(transfer: any) {
  if (!transfer?.transferId || dismissedTransfers.has(transfer.transferId)) return
  const current = transfers[transfer.transferId]
  // A failed task can be explicitly resumed. Completed/canceled tasks must
  // ignore a late starting event from the original worker.
  if (current && ['completed', 'canceled'].includes(current.status) && transfer.status === 'starting') return
  transfers[transfer.transferId] = { ...current, ...transfer }
}
function handleTransferEvent(event: any) {
  const transfer = event?.data ?? event
  if (!transfer?.transferId || mode.value !== 'friend' || transfer.deviceId !== deviceId || dismissedTransfers.has(transfer.transferId)) return
  const previous = transfers[transfer.transferId]
  registerTransfer(transfer)
  if (transfer.status === 'failed' && !notifiedTransferFailures.has(transfer.transferId)) {
    notifiedTransferFailures.add(transfer.transferId)
    Message.error(`${transfer.fileName || transfer.relativePath || '共享文件'}下载失败：${transfer.errorMessage || '连接或文件校验失败'}`)
  }
  if (transfer.status === 'completed' && previous?.status !== 'completed' && !notifiedTransferCompletions.has(transfer.transferId)) {
    notifiedTransferCompletions.add(transfer.transferId)
    Message.success(`${transfer.fileName || transfer.relativePath || '共享文件'}下载完成`)
  }
}
function closeWindow() { void Window.Close() }
function applyTheme(theme: string) {
  isDark.value = theme === 'dark' || (theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.body.classList.toggle('flyqpro-dark', isDark.value)
}
async function loadTheme() { try { const profile = await ChatService.GetProfile(); applyTheme(profile.theme || 'system') } catch { applyTheme('system') } }
async function loadSettings() { if (mode.value !== 'owner') return; const result = await ChatService.GetSharedFolderSettings(); Object.assign(settings, result); sharedDisabled.value = false }
function resetThumbnailState() { thumbnailGeneration++; if (thumbnailBatchTimer !== undefined) { window.clearTimeout(thumbnailBatchTimer); thumbnailBatchTimer = undefined }; thumbnailQueue.length = 0; thumbnailQueued.clear(); Object.keys(thumbnailUrls).forEach((key) => delete thumbnailUrls[key]); thumbnailLoading.clear(); thumbnailFailed.clear() }
function resetViewState() { relativePath.value = ''; search.value = ''; searchVisible.value = false; entries.value = []; selected.clear(); hasMore.value = false; nextOffset.value = 0; loadingMore.value = false; resetThumbnailState(); context.visible = false; context.entry = undefined; renameDialog.visible = false; renameDialog.loading = false; renameDialog.name = ''; renameDialog.entry = undefined; detailsDialog.visible = false; detailsDialog.loading = false; detailsDialog.error = ''; detailsDialog.entry = undefined; Object.keys(transfers).forEach((key) => delete transfers[key]); dismissedTransfers.clear(); notifiedTransferFailures.clear(); notifiedTransferCompletions.clear(); transferExpanded.value = true }
async function loadEntriesPage(append = false) {
  if (mode.value === 'invalid' || (mode.value === 'friend' && !deviceId)) return
  const offset = append ? nextOffset.value : 0
  if (append) loadingMore.value = true
  else { loading.value = true; sharedDisabled.value = false; selected.clear(); resetThumbnailState() }
  try {
    const page = mode.value === 'owner'
      ? await ChatService.ListSharedEntriesPage(relativePath.value, offset, 100)
      : await ChatService.ListFriendSharedEntriesPage(deviceId, relativePath.value, offset, 100)
    const incoming = page.entries || []
    entries.value = append
      ? [...entries.value, ...incoming.filter((entry: Entry) => !entries.value.some((current) => current.relativePath === entry.relativePath))]
      : incoming
    nextOffset.value = page.nextOffset || 0
    hasMore.value = Boolean(page.hasMore)
  } catch (error: any) {
    const message = String(error?.message || error)
    sharedDisabled.value = mode.value === 'friend' && (message.includes('SHARED_DISABLED') || message.includes('SHARED_UNAVAILABLE'))
    if (!sharedDisabled.value) Message.error(error?.message || '读取共享目录失败')
    if (!append) entries.value = []
  } finally {
    loading.value = false
    loadingMore.value = false
    if (!append) scheduleInitialThumbnails()
  }
}
async function refresh() {
  if (mode.value === 'invalid' || (mode.value === 'friend' && !deviceId)) {
    sharedDisabled.value = true
    entries.value = []
    return
  }
  try {
    if (mode.value === 'owner') await loadSettings()
    if (mode.value === 'owner' && !settings.rootPath) { sharedDisabled.value = true; entries.value = []; return }
    await loadEntriesPage(false)
  } catch (error: any) { Message.error(error?.message || '读取共享目录失败') }
}
async function loadEntries() {
  if (mode.value === 'owner' && !settings.rootPath) {
    sharedDisabled.value = true
    entries.value = []
    return
  }
  await loadEntriesPage(false)
}
function loadMore() { if (hasMore.value && !loadingMore.value) void loadEntriesPage(true) }
async function chooseRoot() { try { const path = await ChatService.PickSharedDirectory(); if (!path) return; relativePath.value = ''; Object.assign(settings, await ChatService.SetSharedFolder(path, settings.enabled)); await loadEntries(); Message.success('共享目录已更新') } catch (error: any) { Message.error(error?.message || '设置共享目录失败') } }
async function toggleEnabled(value: boolean) { try { Object.assign(settings, await ChatService.SetSharedEnabled(value)); Message.success(value ? '共享已开启' : '共享已关闭') } catch (error: any) { Message.error(error?.message || '共享开关更新失败') } }
async function createFolder() { const name = window.prompt('请输入文件夹名称'); if (!name) return; try { await ChatService.CreateSharedFolder(relativePath.value, name); await refresh() } catch (error: any) { Message.error(error?.message || '新建文件夹失败') } }
async function importFiles() { try { await ChatService.ImportSharedFiles(relativePath.value); await refresh() } catch (error: any) { Message.error(error?.message || '导入文件失败') } }
async function importFolder() { try { await ChatService.ImportSharedFolder(relativePath.value); await refresh() } catch (error: any) { Message.error(error?.message || '导入文件夹失败') } }
async function openOwnerFolder() { try { await ChatService.RevealSharedEntry(relativePath.value) } catch (error: any) { Message.error(error?.message || '打开目录失败') } }
async function openPath(path: string) {
  relativePath.value = path
  resetThumbnailState()
  await refresh()
}
function goParent() { openPath(pathParts.value.slice(0, -1).join('/')) }
function toggleSelected(entry: Entry) { selected.has(entry.relativePath) ? selected.delete(entry.relativePath) : selected.add(entry.relativePath) }
function selectAll() { if (selected.size === entries.value.length && entries.value.length) selected.clear(); else entries.value.forEach((entry) => selected.add(entry.relativePath)) }
function openContext(event: MouseEvent, entry: Entry) { context.visible = true; context.x = Math.min(event.clientX, innerWidth - 190); context.y = Math.min(event.clientY, innerHeight - 220); context.entry = entry }
function closeContext() { context.visible = false; context.entry = undefined }
function newContextFolder() { if (context.entry) openPath(context.entry.relativePath); closeContext() }
function renameEntry() { if (!context.entry || mode.value !== 'owner') return; renameDialog.entry = context.entry; renameDialog.name = context.entry.name; renameDialog.visible = true; closeContext() }
function cancelRename() { if (renameDialog.loading) return; renameDialog.visible = false; renameDialog.name = ''; renameDialog.entry = undefined }
async function confirmRename() { const entry = renameDialog.entry; const name = renameDialog.name.trim(); if (!entry || renameDialog.loading) return; if (!name) { Message.warning('请输入新名称'); return } if (name === entry.name) { cancelRename(); return } renameDialog.loading = true; try { await ChatService.RenameSharedEntry(entry.relativePath, name); renameDialog.visible = false; renameDialog.name = ''; renameDialog.entry = undefined; await refresh(); Message.success('重命名成功') } catch (error: any) { Message.error(error?.message || '重命名失败') } finally { renameDialog.loading = false } }
async function deleteEntry() { if (!context.entry) return; const entry = context.entry; Modal.confirm({ title: '确认删除', content: entry.isDirectory ? '将递归删除该文件夹及其内容，确定继续吗？' : `确定删除“${entry.name}”吗？`, okButtonProps: { status: 'danger' }, onOk: async () => { try { await ChatService.DeleteSharedEntry(entry.relativePath); await refresh(); Message.success('已删除') } catch (error: any) { Message.error(error?.message || '删除失败') } } }); closeContext() }
async function copyRelative() { if (!context.entry) return; try { await Clipboard.SetText(context.entry.relativePath); Message.success('相对路径已复制') } catch { Message.error('复制失败') } closeContext() }
async function copyName() { if (!context.entry) return; try { await Clipboard.SetText(context.entry.name); Message.success('文件名已复制') } catch { Message.error('复制失败') } closeContext() }
let detailsRequestId = 0
async function showDetails() {
  const entry = context.entry
  if (!entry) return
  closeContext()
  const requestId = ++detailsRequestId
  detailsDialog.entry = { ...entry }
  detailsDialog.error = ''
  detailsDialog.loading = true
  detailsDialog.visible = true
  try {
    const detail = mode.value === 'owner' ? await ChatService.GetSharedEntryDetails(entry.relativePath) : await ChatService.GetFriendSharedEntryDetails(deviceId, entry.relativePath)
    if (requestId === detailsRequestId) {
      detailsDialog.entry = detail
      detailsDialog.loading = false
    }
  } catch (error: any) {
    if (requestId === detailsRequestId) {
      detailsDialog.loading = false
      detailsDialog.error = error?.message || '读取属性失败'
    }
  }
}
async function downloadEntry() { if (!context.entry || context.entry.isDirectory) return; try { const result = await ChatService.DownloadFriendSharedEntry(deviceId, context.entry.relativePath); registerTransfer(result); transferExpanded.value = true; Message.success('已开始下载') } catch (error: any) { Message.error(error?.message || '下载失败') } finally { closeContext() } }
async function saveEntry() { if (!context.entry || context.entry.isDirectory) return; try { const result = await ChatService.SaveFriendSharedEntryAs(deviceId, context.entry.relativePath); registerTransfer(result); transferExpanded.value = true; Message.success('已开始下载') } catch (error: any) { Message.error(error?.message || '保存失败') } finally { closeContext() } }
async function downloadSelected() { const files = [...selectedFiles.value]; if (!files.length) return; let started = 0; for (const entry of files) { try { const result = await ChatService.DownloadFriendSharedEntry(deviceId, entry.relativePath); registerTransfer(result); started++ } catch (error: any) { Message.error(`${entry.name} 下载失败：${error?.message || error}`) } } selected.clear(); if (started) { transferExpanded.value = true; Message.success(`已加入 ${started} 个下载任务`) } }
async function saveSelected() { const files = [...selectedFiles.value]; if (!files.length) return; let started = 0; for (const entry of files) { try { const result = await ChatService.SaveFriendSharedEntryAs(deviceId, entry.relativePath); registerTransfer(result); started++ } catch (error: any) { Message.error(`${entry.name} 保存失败：${error?.message || error}`) } } selected.clear(); if (started) { transferExpanded.value = true; Message.success(`已加入 ${started} 个下载任务`) } }
function cancelTransfer(transferId: string) { const transfer = transfers[transferId]; if (!transfer) return; Modal.confirm({ title: '取消下载', content: `确定取消“${transfer.fileName || transfer.relativePath}”的下载吗？`, okButtonProps: { status: 'danger' }, onOk: async () => { dismissedTransfers.add(transferId); try { await ChatService.CancelSharedTransfer(transferId); delete transfers[transferId]; Message.success('下载已取消') } catch (error: any) { dismissedTransfers.delete(transferId); Message.error(error?.message || '取消下载失败') } } }) }
async function pauseTransfer(transferId: string) { try { await ChatService.PauseSharedTransfer(transferId); if (transfers[transferId]) transfers[transferId].status = 'paused'; Message.success('下载已暂停，可继续下载') } catch (error: any) { Message.error(error?.message || '暂停下载失败') } }
async function resumeTransfer(transferId: string) { try { const result = await ChatService.ResumeSharedTransfer(transferId); registerTransfer(result); transferExpanded.value = true; Message.success('已继续下载') } catch (error: any) { Message.error(error?.message || '继续下载失败') } }
function isPreviewable(entry: Entry) {
  const mime = String(entry.mimeType || '').toLowerCase()
  return mime.startsWith('image/') || mime === 'application/pdf' || /\.(avif|bmp|gif|heic|heif|jpe?g|png|webp|pdf)$/i.test(entry.name)
}
async function preview(entry: Entry) {
  if (entry.isDirectory) return
  closeContext()
  if (!isPreviewable(entry)) { Message.info('该文件类型请先下载后打开'); return }
  try {
    if (mode.value === 'owner') await ImageViewerService.OpenSharedPreview(entry.relativePath)
    else if (mode.value === 'friend') await ImageViewerService.OpenFriendSharedPreview(deviceId, entry.relativePath)
    else Message.warning('共享窗口参数无效')
  } catch (error: any) { Message.error(error?.message || '打开预览失败') }
}
function handleKeydown(event: KeyboardEvent) { if (!embedded && isMac.value && event.metaKey && event.key.toLowerCase() === 'w') { event.preventDefault(); closeWindow() } }
function handlePointerDown(event: PointerEvent) {
  closeContext()
  const target = event.target
  if (searchVisible.value && (!(target instanceof Element) || !target.closest('.breadcrumb-search, .breadcrumb-actions'))) searchVisible.value = false
}
watch([viewMode, search], () => scheduleInitialThumbnails())
onMounted(async () => { try { isMac.value = System.IsMac() } catch {} thumbnailObserver = typeof IntersectionObserver !== 'undefined' ? new IntersectionObserver((items) => { for (const item of items) { if (!item.isIntersecting) continue; const path = (item.target as HTMLElement).dataset.thumbnailPath || ''; const entry = filteredEntries.value.find((candidate) => candidate.relativePath === path); if (entry) enqueueThumbnail(entry); thumbnailObserver?.unobserve(item.target) } }, { root: document.querySelector('.file-list'), rootMargin: '160px' }) : undefined; resetViewState(); window.addEventListener('pointerdown', handlePointerDown); window.addEventListener('keydown', handleKeydown); cancelSharedEvents = Events.On('chat:shared-progress', handleTransferEvent); const cancelStatsEvent = Events.On('chat:shared-stats-updated', (event: any) => { const status = event?.data ?? event; if (mode.value === 'owner' && status?.rootPath === settings.rootPath) Object.assign(settings, status) }); const previousCancel = cancelSharedEvents; cancelSharedEvents = () => { previousCancel?.(); cancelStatsEvent?.() }; cancelThemeEvent = Events.On('chat:profile-updated', (event: any) => { const profile = event?.data ?? event; if (profile?.theme) applyTheme(profile.theme); else void loadTheme() }); await loadTheme(); await refresh() })
onBeforeUnmount(() => { window.removeEventListener('pointerdown', handlePointerDown); window.removeEventListener('keydown', handleKeydown); thumbnailObserver?.disconnect(); thumbnailObserver = undefined; resetThumbnailState(); cancelSharedEvents?.(); cancelThemeEvent?.() })
</script>

<style scoped lang="less">
:global(html), :global(body) { margin: 0; background: transparent !important; overflow: hidden; }
.shared-drive { width:100%; height:100%; overflow:hidden; display:flex; flex-direction:column; color:#1d2129; background:#f5f5f5; --page-bg:#f5f5f5; --line:#e5e6eb; --surface:#fff; --muted:#86909c; --accent:#3767e8; }
.shared-drive.mac { border-radius:18px; }
.shared-drive.embedded { border-radius:0; }
.shared-drive.dark { color:#e5e7eb; background:#171b23; --page-bg:#171b23; --line:#364154; --surface:#222936; --muted:#9aa7ba; }
.mac-controls { position:absolute; z-index:10; top:10px; left:14px; display:flex; gap:8px; }.mac-controls button { width:12px; height:12px; border:0; border-radius:50%; cursor:pointer; }.mac-controls .close{background:#ff5f57}.mac-controls .minimise{background:#febc2e}.mac-controls .maximise{background:#28c840}
.shared-head { flex:0 0 58px; box-sizing:border-box; display:flex; align-items:center; justify-content:space-between; padding:0 20px 0 24px; border-bottom:1px solid var(--line); background:var(--surface); }.shared-head.draggable { padding-left:80px; --wails-draggable:drag; }.head-title { display:flex; gap:10px; align-items:center; }.head-title svg{width:21px;height:21px;color:var(--accent)}.head-title div{display:flex;flex-direction:column;gap:2px}.head-title strong{font-size:16px}.head-title span{font-size:12px;color:var(--muted)}
.shared-body { flex:1; min-height:0; display:flex; flex-direction:column; overflow:hidden; padding:0 20px 24px; }
.shared-fixed-header { flex:0 0 auto; background:var(--page-bg); padding-bottom:1px; box-sizing:border-box; }
.shared-fixed-header .management-toolbar { box-shadow:0 4px 12px color-mix(in srgb,#000 10%,transparent); }
.shared-fixed-header .breadcrumbs { margin-bottom:12px; }
.card{background:var(--surface);border:1px solid var(--line);border-radius:10px}.owner-summary{display:flex;align-items:center;gap:20px;padding:15px 18px;margin:8px 0 12px}.summary-main{flex:1;display:flex;flex-direction:column;gap:4px}.summary-label{font-size:12px;color:var(--muted)}.summary-main strong{font-size:17px}.summary-main span,.summary-item span{font-size:12px;color:var(--muted)}.summary-item{min-width:105px;display:flex;flex-direction:column;gap:4px;padding-left:18px;border-left:1px solid var(--line)}.summary-item strong{font-size:16px}.toolbar{display:flex;align-items:center;gap:8px;padding:10px 12px;margin-bottom:12px}.toolbar button,.file-toolbar button,.breadcrumbs button{border:1px solid var(--line);border-radius:6px;background:var(--surface);color:inherit;padding:6px 10px;cursor:pointer}.toolbar button:disabled,.file-toolbar button:disabled{opacity:.45;cursor:not-allowed}.path-info{min-width:0;flex:1;display:flex;flex-direction:column;gap:3px}.path-info span{font-size:11px;color:var(--muted)}.path-info strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.remote-banner{display:flex;align-items:center;gap:12px;padding:15px 18px;margin-bottom:12px}.remote-banner svg{color:var(--accent);width:22px;height:22px}.remote-banner div{flex:1;display:flex;flex-direction:column;gap:4px}.remote-banner span{font-size:12px;color:var(--muted)}.remote-banner button{border:1px solid var(--line);border-radius:6px;background:var(--surface);color:inherit;padding:6px 12px;cursor:pointer}.breadcrumbs{display:flex;align-items:center;gap:5px;padding:8px 12px;margin-bottom:12px;overflow:auto;white-space:nowrap}.breadcrumbs button{border:0;padding:4px 6px}.breadcrumbs button.active{color:var(--accent);font-weight:600}.file-panel{flex:1;min-height:0;display:flex;flex-direction:column;overflow:hidden}.file-toolbar{flex:0 0 auto;display:flex;align-items:center;gap:8px;padding:10px 12px;border-bottom:1px solid var(--line)}.file-toolbar input{flex:1;min-width:120px;border:1px solid var(--line);background:var(--surface);color:inherit;border-radius:6px;padding:7px 9px;outline:none}.view-note{margin-left:auto;color:var(--muted);font-size:12px}.file-list{flex:1;min-height:0;overflow:auto}.file-panel > .empty-state{flex:1;min-height:0}.file-row{display:flex;align-items:center;gap:10px;padding:10px 14px;border-bottom:1px solid var(--line);cursor:default}.file-row:hover,.file-row.selected{background:color-mix(in srgb,var(--accent) 8%,var(--surface))}.file-icon{width:20px;height:20px;color:var(--muted);flex:0 0 20px}.file-icon.folder{color:#e6a23c}.file-name{min-width:0;flex:1;display:flex;flex-direction:column;gap:3px}.file-name strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.file-name span,.file-date{font-size:12px;color:var(--muted)}.file-date{width:155px;text-align:right}.row-more{border:0;background:transparent;color:var(--muted);font-size:17px;cursor:pointer}.empty-state{min-height:300px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px;color:var(--muted)}.empty-state svg{width:34px;height:34px;color:var(--accent)}.empty-state strong{color:inherit}.context-menu{position:fixed;z-index:20;min-width:150px;padding:5px;border:1px solid var(--line);border-radius:8px;background:var(--surface);box-shadow:0 12px 30px rgba(0,0,0,.24)}.context-menu button{display:block;width:100%;border:0;border-radius:5px;background:transparent;color:inherit;text-align:left;padding:8px 10px;cursor:pointer}.context-menu button:hover{background:color-mix(in srgb,var(--accent) 12%,var(--surface))}.context-menu .danger{color:#f53f3f}
.load-more{display:flex;justify-content:center;padding:12px}.load-more button{border:1px solid var(--line);border-radius:6px;background:var(--surface);color:var(--accent);padding:6px 14px;cursor:pointer}.load-more button:disabled{opacity:.55;cursor:default}
.rename-dialog{display:flex;flex-direction:column;gap:10px}.rename-dialog label{font-size:13px;color:var(--muted)}.rename-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:8px}
.details-dialog{display:flex;flex-direction:column;gap:11px;min-width:360px}.detail-row{display:flex;align-items:flex-start;gap:18px}.detail-row>span{flex:0 0 64px;color:var(--muted);font-size:13px}.detail-row>strong{min-width:0;flex:1;font-size:13px;word-break:break-all}.detail-value{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px!important}.detail-loading{margin-top:4px;padding:9px 10px;border-radius:6px;background:color-mix(in srgb,var(--accent) 8%,var(--surface));color:var(--accent);font-size:12px}.detail-error{margin-top:4px;color:#f53f3f;font-size:12px}
.transfer-float{position:fixed;z-index:15;right:20px;bottom:20px;width:min(440px,calc(100vw - 40px));max-height:min(430px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--line);border-radius:12px;background:var(--surface);box-shadow:0 14px 36px rgba(0,0,0,.22)}.transfer-float.collapsed{width:230px}.transfer-header{display:flex;align-items:center;justify-content:space-between;width:100%;border:0;border-bottom:1px solid var(--line);background:var(--surface);color:inherit;padding:10px 13px;cursor:pointer}.transfer-title{display:flex;align-items:baseline;gap:8px}.transfer-title small{font-size:12px;color:var(--muted)}.transfer-header svg{width:16px;height:16px;color:var(--muted)}.transfer-body{max-height:365px;overflow:auto;padding:4px 12px}.transfer-row{display:flex;align-items:center;gap:12px;padding:10px 0;border-bottom:1px solid var(--line)}.transfer-row:last-child{border-bottom:0}.transfer-main{min-width:0;flex:1;display:flex;flex-direction:column;gap:4px}.transfer-main strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.transfer-row span{font-size:12px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.transfer-progress{height:4px;overflow:hidden;border-radius:4px;background:color-mix(in srgb,var(--accent) 14%,var(--surface))}.transfer-progress i{display:block;height:100%;border-radius:inherit;background:var(--accent);transition:width .2s ease}.transfer-actions{display:flex;align-items:center;gap:10px;flex:0 0 auto}.transfer-actions strong{font-size:12px;color:var(--accent)}.transfer-actions button{border:1px solid var(--line);border-radius:5px;background:var(--surface);color:inherit;padding:4px 8px;cursor:pointer}
.shared-head { flex-basis:44px; height:44px; padding-left:18px; padding-right:16px; }
.shared-head.draggable { padding-left:76px; }
.head-title { gap:8px; }
.head-title svg { width:18px; height:18px; }
.head-title div { gap:1px; }
.head-title strong { font-size:15px; }
.head-title span { font-size:11px; }
.owner-summary { gap:12px; padding:8px 12px; margin:4px 0 6px; }
.shared-head + .shared-body { padding-top:4px; }
.summary-main { gap:2px; }
.summary-label, .summary-main span, .summary-item span { font-size:11px; }
.summary-main strong { font-size:15px; }
.summary-item { min-width:90px; gap:2px; padding-left:12px; }
.summary-item strong { font-size:14px; }
.toolbar { gap:6px; padding:5px 8px; margin-bottom:6px; }
.toolbar button, .file-toolbar button, .breadcrumbs button { padding:4px 8px; }
.path-info { gap:1px; }
.path-info span { font-size:10px; }
.path-info strong { font-size:11px; }
.remote-banner { gap:8px; padding:8px 10px; margin-bottom:6px; }
.remote-banner svg { width:18px; height:18px; }
.remote-banner div { gap:2px; }
.remote-banner span { font-size:11px; }
.remote-banner button { padding:4px 8px; }
.shared-fixed-header .breadcrumbs { gap:3px; padding:4px 8px; margin-bottom:6px; }
.breadcrumbs button { padding:2px 5px; }
.file-toolbar { gap:6px; padding:6px 8px; }
.file-toolbar input { padding:5px 8px; }
.breadcrumbs { position:relative; overflow:visible; white-space:normal; }
.breadcrumb-path { min-width:0; flex:1 1 auto; display:flex; align-items:center; gap:3px; overflow:auto; white-space:nowrap; }
.breadcrumb-actions { flex:0 0 auto; display:flex; align-items:center; gap:4px; margin-left:8px; }
.breadcrumb-actions button { padding:3px 7px; font-size:12px; white-space:nowrap; }
.breadcrumb-actions .icon-action { display:inline-flex; align-items:center; justify-content:center; width:27px; height:27px; padding:0; }
.breadcrumb-actions .icon-action svg { width:16px; height:16px; }
.breadcrumb-search { position:absolute; z-index:12; top:calc(100% + 5px); right:8px; display:flex; align-items:center; gap:6px; min-width:260px; padding:6px 8px; border:1px solid var(--line); border-radius:8px; background:var(--surface); box-shadow:0 10px 24px rgba(0,0,0,.2); }
.breadcrumb-search > svg { width:16px; height:16px; color:var(--muted); flex:0 0 16px; }
.breadcrumb-search input { min-width:0; flex:1; border:0; outline:none; background:transparent; color:inherit; font-size:13px; }
.breadcrumb-search input::placeholder { color:var(--muted); }
.breadcrumb-search .icon-action { border:0; color:var(--muted); background:transparent; }
.breadcrumb-actions .icon-action.active { color:var(--accent); background:color-mix(in srgb,var(--accent) 12%,var(--surface)); }
.thumb-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(150px,1fr)); align-content:start; gap:12px; padding:12px; box-sizing:border-box; }
.thumb-card { position:relative; min-width:0; padding:8px; border:1px solid var(--line); border-radius:8px; background:var(--surface); cursor:default; }
.thumb-card:hover, .thumb-card.selected { border-color:color-mix(in srgb,var(--accent) 48%,var(--line)); background:color-mix(in srgb,var(--accent) 7%,var(--surface)); }
.thumb-check { position:absolute; z-index:1; top:10px; left:10px; width:16px; height:16px; }
.thumb-preview { height:130px; display:flex; align-items:center; justify-content:center; overflow:hidden; border-radius:6px; background:color-mix(in srgb,var(--accent) 6%,var(--page-bg)); }
.thumb-preview img { display:block; width:100%; height:100%; object-fit:contain; }
.thumb-placeholder { display:flex; align-items:center; justify-content:center; width:100%; height:100%; color:var(--muted); font-size:12px; }
.thumb-placeholder svg { width:40px; height:40px; }
.thumb-placeholder.folder { color:#e6a23c; }
.thumb-name { display:block; overflow:hidden; margin-top:8px; text-overflow:ellipsis; white-space:nowrap; font-size:13px; }
.thumb-meta { display:block; overflow:hidden; margin-top:3px; color:var(--muted); text-overflow:ellipsis; white-space:nowrap; font-size:11px; }
.thumb-placeholder.loading { color:var(--accent); }
.file-icon.file-thumb { object-fit:cover; border-radius:4px; background:var(--page-bg); }
</style>
