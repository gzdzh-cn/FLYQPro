<template>
  <div class="shared-drive" :class="{ dark: isDark, mac: isMac }">
    <div v-if="isMac" class="mac-controls" aria-label="窗口控制"><button class="close" @click="closeWindow" /><button class="minimise" @click="Window.Minimise" /><button class="maximise" @click="Window.ToggleMaximise" /></div>
    <header class="shared-head" :class="{ draggable: isMac }">
      <div class="head-title"><IconCloud /><div><strong>{{ mode === 'owner' ? '共享' : mode === 'friend' ? '好友共享' : '共享' }}</strong><span>{{ mode === 'owner' ? '管理本机共享文件夹' : mode === 'friend' ? '查看好友共享文件夹' : '共享窗口参数无效' }}</span></div></div>
    </header>

    <main class="shared-body">
      <section v-if="mode === 'owner'" class="owner-summary card">
        <div class="summary-main"><div class="summary-label">共享开关</div><strong>{{ settings.enabled ? '已开启' : '已关闭' }}</strong><span>{{ settings.enabled ? '好友可以浏览和下载共享目录' : '开启后好友才可以访问' }}</span></div>
        <a-switch :model-value="settings.enabled" @change="toggleEnabled" />
        <div class="summary-item"><span>共享文件</span><strong>{{ settings.fileCount }}</strong></div><div class="summary-item"><span>共享文件夹</span><strong>{{ settings.folderCount }}</strong></div><div class="summary-item"><span>磁盘剩余</span><strong>{{ formatBytes(settings.availableBytes) }}</strong></div>
      </section>
      <section v-if="mode === 'owner'" class="toolbar card"><div class="path-info"><span>共享目录</span><strong :title="settings.rootPath">{{ settings.rootPath || '未设置' }}</strong></div><button @click="chooseRoot">选择共享目录</button><button :disabled="!settings.rootPath" @click="refresh">刷新</button><button :disabled="!settings.rootPath" @click="createFolder">新建文件夹</button><button :disabled="!settings.rootPath" @click="importFiles">导入文件</button><button :disabled="!settings.rootPath" @click="importFolder">导入文件夹</button><button :disabled="!settings.rootPath" @click="openOwnerFolder">在文件管理器中打开</button></section>
      <section v-else-if="mode === 'friend'" class="remote-banner card"><IconCloud /><div><strong>{{ sharedDisabled ? '对方暂未开启共享' : '好友共享盘' }}</strong><span>{{ sharedDisabled ? '请等待对方开启共享后刷新' : '共享内容仅可读取和下载，不会修改对方文件' }}</span></div><button @click="refresh">刷新</button></section>
      <section v-else class="remote-banner card"><IconCloud /><div><strong>共享窗口参数无效</strong><span>无法确定要访问的好友设备，请从好友聊天窗口重新打开</span></div></section>

      <nav class="breadcrumbs card"><button :disabled="!relativePath" @click="goParent">上一级</button><button :class="{ active: !relativePath }" @click="openPath('')">共享根目录</button><template v-for="(part, index) in pathParts" :key="part + index"><span>/</span><button @click="openPath(pathParts.slice(0, index + 1).join('/'))">{{ part }}</button></template></nav>
      <section class="file-panel card" @contextmenu.prevent>
        <div class="file-toolbar"><input v-model="search" placeholder="搜索文件和文件夹" /><button @click="selectAll">{{ selected.size === entries.length && entries.length ? '取消全选' : '全选' }}</button><button v-if="mode === 'friend'" :disabled="!selectedFiles.length || sharedDisabled" @click="downloadSelected">下载</button><button v-if="mode === 'friend'" :disabled="!selectedFiles.length || sharedDisabled" @click="saveSelected">另存为</button><button :disabled="loading" @click="refresh">刷新</button><span class="view-note">{{ filteredEntries.length }} 项</span></div>
        <div v-if="loading" class="empty-state">正在读取共享目录…</div>
        <div v-else-if="mode === 'invalid'" class="empty-state"><IconCloud /><strong>共享窗口参数无效</strong><span>无法确定要访问的好友设备</span></div>
        <div v-else-if="sharedDisabled" class="empty-state"><IconCloud /><strong>{{ mode === 'owner' ? (settings.rootPath ? '共享已关闭' : '请先选择共享目录') : '对方已关闭共享' }}</strong><span>{{ mode === 'owner' ? '本机仍可管理共享目录，开启开关后好友才可访问' : '共享开关开启后，刷新即可继续访问' }}</span></div>
        <div v-else-if="!filteredEntries.length" class="empty-state"><IconFolder /><span>此文件夹为空</span></div>
        <div v-else class="file-list">
          <div v-for="entry in filteredEntries" :key="entry.entryId" class="file-row" :class="{ selected: selected.has(entry.relativePath) }" @dblclick="entry.isDirectory ? openPath(entry.relativePath) : preview(entry)" @contextmenu.prevent.stop="openContext($event, entry)">
            <input v-if="mode === 'friend'" type="checkbox" :checked="selected.has(entry.relativePath)" @click.stop="toggleSelected(entry)" />
            <IconFolder v-if="entry.isDirectory" class="file-icon folder" /><IconFile v-else class="file-icon" />
            <div class="file-name"><strong>{{ entry.name }}</strong><span>{{ entry.isDirectory ? '文件夹' : formatBytes(entry.size) }}</span></div><span class="file-date">{{ formatDate(entry.modifiedAt) }}</span><button class="row-more" @click.stop="openContext($event, entry)">···</button>
          </div>
        </div>
      </section>
      <section v-if="Object.keys(transfers).length" class="transfer-list card"><div v-for="transfer in transfers" :key="transfer.transferId" class="transfer-row"><div><strong>{{ transfer.fileName || transfer.relativePath }}</strong><span>{{ formatBytes(transfer.transferred || 0) }} / {{ formatBytes(transfer.fileSize || 0) }} · {{ transfer.status === 'completed' ? '已完成' : transfer.status === 'canceled' ? '已取消' : '下载中' }}</span></div><div class="transfer-actions"><strong>{{ transfer.fileSize ? Math.floor((transfer.transferred || 0) * 100 / transfer.fileSize) : 0 }}%</strong><button v-if="!['completed', 'canceled'].includes(transfer.status)" @click="cancelTransfer(transfer.transferId)">取消</button></div></div></section>
    </main>
    <div v-if="context.visible && context.entry" class="context-menu" :style="{ left: context.x + 'px', top: context.y + 'px' }" @pointerdown.stop @click.stop>
      <template v-if="mode === 'owner'"><button v-if="context.entry.isDirectory" @click="newContextFolder">打开</button><button @click="renameEntry">重命名</button><button @click="deleteEntry" class="danger">删除</button><button @click="copyRelative">复制相对路径</button><button @click="showDetails">属性</button></template>
      <template v-else><button v-if="context.entry.isDirectory" @click="openPath(context.entry.relativePath)">打开</button><button v-if="!context.entry.isDirectory" @click="downloadEntry">下载</button><button v-if="!context.entry.isDirectory" @click="saveEntry">另存为</button><button v-if="!context.entry.isDirectory" @click="copyName">复制文件名</button><button @click="copyRelative">复制相对路径</button><button @click="showDetails">详情</button></template>
    </div>
    <a-modal v-model:visible="renameDialog.visible" title="重命名" :footer="false" :mask-closable="false" @cancel="cancelRename">
      <div class="rename-dialog">
        <label>新名称</label>
        <a-input v-model="renameDialog.name" allow-clear placeholder="请输入新的文件或文件夹名称" @press-enter="confirmRename" />
        <div class="rename-actions"><a-button @click="cancelRename">取消</a-button><a-button type="primary" :loading="renameDialog.loading" @click="confirmRename">确定</a-button></div>
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconCloud, IconFile, IconFolder } from '@arco-design/web-vue/es/icon'
import { Clipboard, Events, System, Window } from '@wailsio/runtime'
import { ChatService, SharedDriveWindowService } from '/#/flyqpro/internal/service'

interface Entry { entryId: string; name: string; relativePath: string; isDirectory: boolean; size: number; mimeType: string; modifiedAt: string; sha256?: string }
type SharedMode = 'owner' | 'friend' | 'invalid'
function readSharedQuery() {
  const hashQuery = location.hash.includes('?') ? location.hash.slice(location.hash.indexOf('?') + 1) : ''
  const candidates = [location.search, hashQuery].filter(Boolean).map((value) => new URLSearchParams(value))
  const query = candidates.find((candidate) => candidate.has('mode') || candidate.has('deviceId')) || new URLSearchParams()
  const requestedMode = query.get('mode')
  const requestedDeviceId = query.get('deviceId')?.trim() || ''
  const parsedMode: SharedMode = requestedMode === 'owner' ? 'owner' : requestedMode === 'friend' && requestedDeviceId ? 'friend' : 'invalid'
  return { mode: parsedMode, deviceId: requestedDeviceId }
}
const sharedQuery = readSharedQuery()
const mode = ref<SharedMode>(sharedQuery.mode)
const deviceId = sharedQuery.deviceId
const isMac = ref(false); const isDark = ref(false); const loading = ref(false); const sharedDisabled = ref(false); const search = ref(''); const relativePath = ref(''); const entries = ref<Entry[]>([]); const selected = reactive(new Set<string>()); const peerName = ref('')
const settings = reactive({ enabled: false, rootPath: '', fileCount: 0, folderCount: 0, availableBytes: 0 })
const context = reactive({ visible: false, x: 0, y: 0, entry: undefined as Entry | undefined })
const renameDialog = reactive({ visible: false, loading: false, name: '', entry: undefined as Entry | undefined })
const transfers = reactive<Record<string, any>>({})
let cancelSharedEvents: (() => void) | undefined
const pathParts = computed(() => relativePath.value ? relativePath.value.split('/').filter(Boolean) : [])
const filteredEntries = computed(() => { const key = search.value.trim().toLowerCase(); return entries.value.filter((entry) => !key || entry.name.toLowerCase().includes(key)) })
const selectedFiles = computed(() => entries.value.filter((entry) => selected.has(entry.relativePath) && !entry.isDirectory))
function formatBytes(value: number) { if (!value) return '—'; if (value < 1024) return `${value} B`; if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`; if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`; return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB` }
function formatDate(value: string) { return value ? new Date(value).toLocaleString() : '—' }
function closeWindow() { void Window.Close() }
async function loadTheme() { try { const profile = await ChatService.GetProfile(); isDark.value = profile.theme === 'dark' || (profile.theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches); document.body.classList.toggle('flyqpro-dark', isDark.value) } catch {} }
async function loadSettings() { if (mode.value !== 'owner') return; const result = await ChatService.GetSharedFolderSettings(); Object.assign(settings, result); sharedDisabled.value = false }
function resetViewState() { relativePath.value = ''; search.value = ''; entries.value = []; selected.clear(); context.visible = false; context.entry = undefined; renameDialog.visible = false; renameDialog.loading = false; renameDialog.name = ''; renameDialog.entry = undefined; Object.keys(transfers).forEach((key) => delete transfers[key]) }
async function refresh() {
  if (mode.value === 'invalid' || (mode.value === 'friend' && !deviceId)) {
    sharedDisabled.value = true
    entries.value = []
    return
  }
  loading.value = true; sharedDisabled.value = false
  try {
    if (mode.value === 'owner') await loadSettings()
    if (mode.value === 'owner' && !settings.rootPath) { sharedDisabled.value = true; entries.value = []; return }
    entries.value = mode.value === 'owner'
      ? await ChatService.ListSharedEntries(relativePath.value)
      : await ChatService.ListFriendSharedEntries(deviceId, relativePath.value)
    selected.clear()
  } catch (error: any) {
    const message = String(error?.message || error)
    sharedDisabled.value = mode.value === 'friend' && (message.includes('SHARED_DISABLED') || message.includes('SHARED_UNAVAILABLE'))
    if (!sharedDisabled.value) Message.error(error?.message || '读取共享目录失败')
    entries.value = []
  } finally { loading.value = false }
}
async function chooseRoot() { try { const path = await ChatService.PickSharedDirectory(); if (!path) return; relativePath.value = ''; Object.assign(settings, await ChatService.SetSharedFolder(path, settings.enabled)); await refresh(); Message.success('共享目录已更新') } catch (error: any) { Message.error(error?.message || '设置共享目录失败') } }
async function toggleEnabled(value: boolean) { try { Object.assign(settings, await ChatService.SetSharedEnabled(value)); await refresh(); Message.success(value ? '共享已开启' : '共享已关闭') } catch (error: any) { Message.error(error?.message || '共享开关更新失败') } }
async function createFolder() { const name = window.prompt('请输入文件夹名称'); if (!name) return; try { await ChatService.CreateSharedFolder(relativePath.value, name); await refresh() } catch (error: any) { Message.error(error?.message || '新建文件夹失败') } }
async function importFiles() { try { await ChatService.ImportSharedFiles(relativePath.value); await refresh() } catch (error: any) { Message.error(error?.message || '导入文件失败') } }
async function importFolder() { try { await ChatService.ImportSharedFolder(relativePath.value); await refresh() } catch (error: any) { Message.error(error?.message || '导入文件夹失败') } }
async function openOwnerFolder() { try { await ChatService.RevealSharedEntry(relativePath.value) } catch (error: any) { Message.error(error?.message || '打开目录失败') } }
function openPath(path: string) { relativePath.value = path; void refresh() }
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
async function showDetails() { if (!context.entry) return; try { const detail = mode.value === 'owner' ? await ChatService.GetSharedEntryDetails(context.entry.relativePath) : await ChatService.GetFriendSharedEntryDetails(deviceId, context.entry.relativePath); Modal.info({ title: '文件详情', content: `名称：${detail.name}\n大小：${formatBytes(detail.size)}\n类型：${detail.mimeType || '文件夹'}\n修改时间：${formatDate(detail.modifiedAt)}${detail.sha256 ? `\nSHA256：${detail.sha256}` : ''}` }) } catch (error: any) { Message.error(error?.message || '读取详情失败') } closeContext() }
async function downloadEntry() { if (!context.entry || context.entry.isDirectory) return; try { const result = await ChatService.DownloadFriendSharedEntry(deviceId, context.entry.relativePath); Message.success(`已下载到 ${result.targetPath}`) } catch (error: any) { Message.error(error?.message || '下载失败') } finally { closeContext() } }
async function saveEntry() { if (!context.entry || context.entry.isDirectory) return; try { const result = await ChatService.SaveFriendSharedEntryAs(deviceId, context.entry.relativePath); if (result.targetPath) Message.success('文件已保存') } catch (error: any) { Message.error(error?.message || '保存失败') } finally { closeContext() } }
async function downloadSelected() { for (const entry of selectedFiles.value) { try { await ChatService.DownloadFriendSharedEntry(deviceId, entry.relativePath) } catch (error: any) { Message.error(`${entry.name} 下载失败：${error?.message || error}`) } } await refresh() }
async function saveSelected() { for (const entry of selectedFiles.value) { try { await ChatService.SaveFriendSharedEntryAs(deviceId, entry.relativePath) } catch (error: any) { Message.error(`${entry.name} 保存失败：${error?.message || error}`) } } await refresh() }
async function cancelTransfer(transferId: string) { try { await ChatService.CancelSharedTransfer(transferId) } catch (error: any) { Message.error(error?.message || '取消下载失败') } }
function preview(entry: Entry) { if (entry.isDirectory) return; Message.info('共享文件请先下载后再打开') }
function handleKeydown(event: KeyboardEvent) { if (isMac.value && event.metaKey && event.key.toLowerCase() === 'w') { event.preventDefault(); closeWindow() } }
onMounted(async () => { try { isMac.value = System.IsMac() } catch {} resetViewState(); window.addEventListener('pointerdown', closeContext); window.addEventListener('keydown', handleKeydown); cancelSharedEvents = Events.On('chat:shared-progress', (event: any) => { const transfer = event?.data ?? event; if (transfer?.transferId && mode.value === 'friend' && transfer.deviceId === deviceId) transfers[transfer.transferId] = { ...transfers[transfer.transferId], ...transfer } }); await loadTheme(); await refresh() })
onBeforeUnmount(() => { window.removeEventListener('pointerdown', closeContext); window.removeEventListener('keydown', handleKeydown); cancelSharedEvents?.() })
</script>

<style scoped lang="less">
:global(html), :global(body) { margin: 0; background: transparent !important; overflow: hidden; }
.shared-drive { width:100%; height:100%; overflow:hidden; display:flex; flex-direction:column; color:#1d2129; background:#f5f5f5; --line:#e5e6eb; --surface:#fff; --muted:#86909c; --accent:#3767e8; }
.shared-drive.mac { border-radius:18px; }
.shared-drive.dark { color:#e5e7eb; background:#171b23; --line:#364154; --surface:#222936; --muted:#9aa7ba; }
.mac-controls { position:absolute; z-index:10; top:10px; left:14px; display:flex; gap:8px; }.mac-controls button { width:12px; height:12px; border:0; border-radius:50%; cursor:pointer; }.mac-controls .close{background:#ff5f57}.mac-controls .minimise{background:#febc2e}.mac-controls .maximise{background:#28c840}
.shared-head { flex:0 0 58px; box-sizing:border-box; display:flex; align-items:center; justify-content:space-between; padding:0 20px 0 24px; border-bottom:1px solid var(--line); background:var(--surface); }.shared-head.draggable { padding-left:80px; --wails-draggable:drag; }.head-title { display:flex; gap:10px; align-items:center; }.head-title svg{width:21px;height:21px;color:var(--accent)}.head-title div{display:flex;flex-direction:column;gap:2px}.head-title strong{font-size:16px}.head-title span{font-size:12px;color:var(--muted)}
.shared-body { flex:1; min-height:0; overflow:auto; padding:16px 20px 24px; }.card{background:var(--surface);border:1px solid var(--line);border-radius:10px}.owner-summary{display:flex;align-items:center;gap:20px;padding:15px 18px;margin-bottom:12px}.summary-main{flex:1;display:flex;flex-direction:column;gap:4px}.summary-label{font-size:12px;color:var(--muted)}.summary-main strong{font-size:17px}.summary-main span,.summary-item span{font-size:12px;color:var(--muted)}.summary-item{min-width:105px;display:flex;flex-direction:column;gap:4px;padding-left:18px;border-left:1px solid var(--line)}.summary-item strong{font-size:16px}.toolbar{display:flex;align-items:center;gap:8px;padding:10px 12px;margin-bottom:12px}.toolbar button,.file-toolbar button,.breadcrumbs button{border:1px solid var(--line);border-radius:6px;background:var(--surface);color:inherit;padding:6px 10px;cursor:pointer}.toolbar button:disabled,.file-toolbar button:disabled{opacity:.45;cursor:not-allowed}.path-info{min-width:0;flex:1;display:flex;flex-direction:column;gap:3px}.path-info span{font-size:11px;color:var(--muted)}.path-info strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.remote-banner{display:flex;align-items:center;gap:12px;padding:15px 18px;margin-bottom:12px}.remote-banner svg{color:var(--accent);width:22px;height:22px}.remote-banner div{flex:1;display:flex;flex-direction:column;gap:4px}.remote-banner span{font-size:12px;color:var(--muted)}.remote-banner button{border:1px solid var(--line);border-radius:6px;background:var(--surface);color:inherit;padding:6px 12px;cursor:pointer}.breadcrumbs{display:flex;align-items:center;gap:5px;padding:8px 12px;margin-bottom:12px;overflow:auto;white-space:nowrap}.breadcrumbs button{border:0;padding:4px 6px}.breadcrumbs button.active{color:var(--accent);font-weight:600}.file-panel{overflow:hidden}.file-toolbar{display:flex;align-items:center;gap:8px;padding:10px 12px;border-bottom:1px solid var(--line)}.file-toolbar input{flex:1;min-width:120px;border:1px solid var(--line);background:var(--surface);color:inherit;border-radius:6px;padding:7px 9px;outline:none}.view-note{margin-left:auto;color:var(--muted);font-size:12px}.file-list{min-height:300px}.file-row{display:flex;align-items:center;gap:10px;padding:10px 14px;border-bottom:1px solid var(--line);cursor:default}.file-row:hover,.file-row.selected{background:color-mix(in srgb,var(--accent) 8%,var(--surface))}.file-icon{width:20px;height:20px;color:var(--muted);flex:0 0 20px}.file-icon.folder{color:#e6a23c}.file-name{min-width:0;flex:1;display:flex;flex-direction:column;gap:3px}.file-name strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.file-name span,.file-date{font-size:12px;color:var(--muted)}.file-date{width:155px;text-align:right}.row-more{border:0;background:transparent;color:var(--muted);font-size:17px;cursor:pointer}.empty-state{min-height:300px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px;color:var(--muted)}.empty-state svg{width:34px;height:34px;color:var(--accent)}.empty-state strong{color:inherit}.context-menu{position:fixed;z-index:20;min-width:150px;padding:5px;border:1px solid var(--line);border-radius:8px;background:var(--surface);box-shadow:0 12px 30px rgba(0,0,0,.24)}.context-menu button{display:block;width:100%;border:0;border-radius:5px;background:transparent;color:inherit;text-align:left;padding:8px 10px;cursor:pointer}.context-menu button:hover{background:color-mix(in srgb,var(--accent) 12%,var(--surface))}.context-menu .danger{color:#f53f3f}
.rename-dialog{display:flex;flex-direction:column;gap:10px}.rename-dialog label{font-size:13px;color:var(--muted)}.rename-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:8px}
.transfer-list{margin-top:12px;padding:8px 12px}.transfer-row{display:flex;align-items:center;gap:12px;padding:8px 0;border-bottom:1px solid var(--line)}.transfer-row:last-child{border-bottom:0}.transfer-row>div:first-child{min-width:0;flex:1;display:flex;flex-direction:column;gap:3px}.transfer-row span{font-size:12px;color:var(--muted)}.transfer-actions{display:flex;align-items:center;gap:10px}.transfer-actions button{border:1px solid var(--line);border-radius:5px;background:var(--surface);color:inherit;padding:4px 8px;cursor:pointer}
</style>
