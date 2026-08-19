<template>
  <div class="chat-app" :class="{ 'theme-dark': isDark, 'is-mac': isMac }">
    <div v-if="isMac" class="window-drag-region" aria-hidden="true"></div>
    <div v-if="isMac" class="mac-window-controls" aria-label="macOS 窗口控制">
      <button type="button" class="mac-control close" title="关闭" @click.stop="closeWindow"></button>
      <button type="button" class="mac-control minimise" title="最小化" @click.stop="minimiseWindow"></button>
      <button type="button" class="mac-control maximise" title="最大化" @click.stop="toggleMaximise"></button>
    </div>
    <aside class="rail">
      <button class="profile-button" :class="{ active: section === 'settings' }" @click="openSettings('general')">
        <div class="avatar large" :style="avatarStyle(store.profile.nickname, store.profile.avatarData)">{{ store.profile.avatarData ? '' : initials(store.profile.nickname) }}</div>
      </button>
      <nav class="rail-nav">
        <button :class="{ active: section === 'friends' }" @click="section = 'friends'"><span>◉</span><small>好友</small></button>
        <button :class="{ active: section === 'discover' }" @click="section = 'discover'"><span>⌕</span><small>发现</small><b v-if="store.requests.length">{{ store.requests.length }}</b></button>
      </nav>
      <button class="rail-settings" :class="{ active: section === 'settings' }" @click="openSettings('general')"><span>⚙</span><small>设置</small></button>
    </aside>

    <section v-if="section === 'friends'" class="workspace">
      <aside class="list-pane">
        <div class="pane-title"><div><strong>好友</strong><span>{{ store.friends.length }}</span></div><button class="icon-button" @click="section = 'discover'">＋</button></div>
        <a-input v-model="friendSearch" class="search" placeholder="搜索好友" allow-clear />
        <div class="list-scroll">
          <button v-for="peer in filteredFriends" :key="peer.deviceId" class="peer-row" :class="{ selected: store.activePeerId === peer.deviceId }" @click="selectPeer(peer)">
            <div class="avatar" :style="avatarStyle(peer.nickname)">{{ initials(peer.nickname) }}<i :class="{ online: peer.online }" /></div>
            <div class="peer-copy"><strong>{{ peer.remark || peer.nickname }}</strong><span>{{ peer.online ? '在线' : '离线' }}</span></div>
          </button>
          <div v-if="!filteredFriends.length" class="empty-small"><div class="empty-icon">⌁</div><p>还没有好友</p><a-button type="primary" size="small" @click="section = 'discover'">去发现好友</a-button></div>
        </div>
      </aside>
      <main class="conversation" v-if="activePeer">
        <header class="conversation-head">
          <div class="head-peer"><div class="avatar" :style="avatarStyle(activePeer.nickname)">{{ initials(activePeer.nickname) }}</div><div><strong>{{ activePeer.remark || activePeer.nickname }}</strong><span :class="{ onlineText: activePeer.online }">{{ activePeer.online ? '在线' : '离线' }} · {{ activePeer.platform }}</span></div></div>
          <a-button type="text" @click="showPeerInfo = !showPeerInfo">ⓘ</a-button>
        </header>
        <div class="message-scroll" ref="messageScroll">
          <div v-if="!activeMessages.length" class="conversation-empty"><div class="empty-icon">✦</div><h3>开始聊天</h3><p>向 {{ activePeer.remark || activePeer.nickname }} 发送第一条消息</p></div>
          <div v-for="message in activeMessages" :key="message.messageId" class="message-line" :class="{ mine: message.senderDeviceId === deviceInfo?.deviceId }">
            <div class="message-bubble"><template v-if="message.kind === 'file'"><strong>📎 {{ message.attachmentName || message.content }}</strong><span class="attachment-meta">{{ formatBytes(message.attachmentSize || 0) }} · {{ message.attachmentStatus || message.status }}</span><div v-if="message.senderDeviceId !== deviceInfo?.deviceId && message.attachmentStatus === 'pending'" class="attachment-actions"><a-button size="mini" type="primary" @click="acceptAttachment(message)">接收</a-button><a-button size="mini" status="danger" @click="rejectAttachment(message)">拒绝</a-button></div></template><template v-else>{{ message.content }}</template><small>{{ formatTime(message.createdAt) }} · {{ message.status }}</small></div>
          </div>
        </div>
        <footer class="composer">
          <div class="composer-tools"><button title="表情">☺</button><button title="附件" @click="pickFile">⌕</button><span v-if="pickedFile" class="picked-file">{{ pickedFile }}</span></div>
          <textarea v-model="draft" placeholder="输入消息，Enter 发送，Shift + Enter 换行" @keydown.enter.exact.prevent="sendMessage" />
          <div class="composer-foot"><span>消息将通过局域网加密传输</span><a-button type="primary" :disabled="!draft.trim()" @click="sendMessage">发送</a-button></div>
        </footer>
      </main>
      <main v-else class="blank-state"><div class="brand-mark">✦</div><h2>POPChat</h2><p>选择一位好友开始聊天</p><a-button type="primary" @click="section = 'discover'">发现局域网好友</a-button></main>
      <aside v-if="showPeerInfo && activePeer" class="info-pane">
        <div class="info-head"><strong>好友资料</strong><button class="icon-button" @click="showPeerInfo = false">×</button></div>
        <div class="info-profile"><div class="avatar huge" :style="avatarStyle(activePeer.nickname)">{{ initials(activePeer.nickname) }}</div><h3>{{ activePeer.remark || activePeer.nickname }}</h3><span>{{ activePeer.online ? '在线' : '离线' }}</span></div>
        <div class="info-fields"><label>平台<strong>{{ activePeer.platform }} {{ activePeer.osVersion }}</strong></label><label>IP 地址<strong>{{ activePeer.ip || '未知' }}:{{ activePeer.port || '-' }}</strong></label><label>设备 ID<strong class="mono">{{ activePeer.deviceId }}</strong></label><label>证书指纹<strong class="mono">{{ activePeer.certificateFingerprint || '未知' }}</strong></label></div>
      </aside>
    </section>

    <section v-else-if="section === 'discover'" class="workspace">
      <aside class="list-pane discovery-pane">
        <div class="pane-title"><div><strong>发现</strong><span>{{ store.discovered.length }}</span></div><a-button size="small" @click="refreshPeers">重新扫描</a-button></div>
        <div class="discover-group"><button class="group-title" @click="groups.requests = !groups.requests"><span>{{ groups.requests ? '⌄' : '›' }} 新的朋友</span><b v-if="store.requests.length">{{ store.requests.length }}</b></button><button v-for="request in store.requests" v-show="groups.requests" :key="request.requestId" class="request-row" :class="{ selected: selectedRequest?.requestId === request.requestId }" @click="selectedRequest = request"><div class="avatar" :style="avatarStyle(request.nickname)">{{ initials(request.nickname) }}</div><div><strong>{{ request.nickname }}</strong><span>{{ request.message || '请求添加你为好友' }}</span></div></button></div>
        <div class="discover-group"><button class="group-title" @click="groups.discovered = !groups.discovered"><span>{{ groups.discovered ? '⌄' : '›' }} 已发现</span><b>{{ store.discovered.length }}</b></button><button v-for="peer in store.discovered" v-show="groups.discovered" :key="peer.deviceId" class="request-row" :class="{ selected: selectedDiscovery?.deviceId === peer.deviceId }" @click="selectedDiscovery = peer"><div class="avatar" :style="avatarStyle(peer.nickname)">{{ initials(peer.nickname) }}<i :class="{ online: peer.online }" /></div><div><strong>{{ peer.nickname }}</strong><span>{{ peer.platform }} · {{ peer.online ? '在线' : '离线' }}</span></div></button></div>
      </aside>
      <main class="detail-pane" v-if="selectedRequest">
        <div class="detail-card"><div class="avatar huge" :style="avatarStyle(selectedRequest.nickname)">{{ initials(selectedRequest.nickname) }}</div><h2>{{ selectedRequest.nickname }}</h2><p>{{ selectedRequest.message || '想和你成为好友' }}</p><span class="subtle">申请时间 {{ formatTime(selectedRequest.createdAt) }}</span><div class="detail-actions"><a-button type="primary" @click="acceptRequest">同意</a-button><a-button status="danger" @click="rejectRequest">拒绝</a-button></div></div>
      </main>
      <main class="detail-pane" v-else-if="selectedDiscovery">
        <div class="detail-card"><div class="avatar huge" :style="avatarStyle(selectedDiscovery.nickname)">{{ initials(selectedDiscovery.nickname) }}</div><h2>{{ selectedDiscovery.nickname }}</h2><div class="tags"><a-tag>{{ selectedDiscovery.platform }}</a-tag><a-tag color="green">{{ selectedDiscovery.online ? '在线' : '离线' }}</a-tag></div><div class="basic-info"><label>设备类型<strong>{{ selectedDiscovery.platform }}</strong></label><label>操作系统<strong>{{ selectedDiscovery.osVersion }}</strong></label><label>状态<strong>{{ selectedDiscovery.online ? '在线' : '最近可见' }}</strong></label></div><a-button type="primary" long @click="addPeer">发送好友申请</a-button><p class="subtle">成为好友后，才会显示 IP、端口和完整设备指纹。</p></div>
      </main>
      <main v-else class="blank-state"><div class="brand-mark">⌕</div><h2>发现局域网好友</h2><p>已开启“允许被发现”的设备会显示在这里</p><a-button type="primary" @click="refreshPeers">立即扫描</a-button></main>
    </section>

    <section v-else class="settings-shell">
      <header class="settings-head"><div><h2>设置</h2><p>管理个人资料、网络和应用行为</p></div><div class="settings-tabs"><button :class="{ active: settingsTab === 'general' }" @click="settingsTab = 'general'">通用</button><button :class="{ active: settingsTab === 'network' }" @click="settingsTab = 'network'">网络</button><button :class="{ active: settingsTab === 'device' }" @click="settingsTab = 'device'">设备信息</button><button :class="{ active: settingsTab === 'about' }" @click="settingsTab = 'about'">关于</button></div></header>
      <div class="settings-panel">
      <main class="settings-content" v-if="settingsTab === 'general'"><section class="setting-card profile-card"><div class="avatar huge" :style="avatarStyle(editProfile.nickname, editProfile.avatarData)">{{ editProfile.avatarData ? '' : initials(editProfile.nickname) }}</div><div class="profile-edit"><a-input v-model="editProfile.nickname" label="昵称" maxlength="32" @blur="syncNickname" @keyup.enter.prevent="saveProfile" /><p>没有自定义头像时，系统会根据设备 ID 生成稳定头像。</p><div class="profile-buttons"><a-button type="primary" @click="chooseAvatar">选择头像</a-button><a-button @mousedown.prevent="resetAvatar">恢复默认头像</a-button><a-button type="primary" @mousedown.prevent="saveProfile">保存资料</a-button></div></div></section><section class="setting-card"><h3>外观</h3><div class="setting-line"><div><strong>主题</strong><span>选择应用的颜色主题</span></div><a-select v-model="editProfile.theme" style="width: 170px"><a-option value="light">亮色</a-option><a-option value="dark">暗色</a-option><a-option value="system">跟随系统</a-option></a-select></div></section><section class="setting-card"><h3>隐私与启动</h3><div class="setting-line"><div><strong>允许被发现</strong><span>关闭后，局域网设备无法在发现列表看到你</span></div><a-switch v-model="editProfile.discoverable" @change="saveProfile(false)" /></div><div class="setting-line"><div><strong>开机启动</strong><span>登录系统后自动启动 POPChat</span></div><a-switch v-model="editProfile.launchAtStartup" @change="toggleStartup" /></div><div class="setting-line"><div><strong>自动保存附件</strong><span>关闭后，收到图片和文件需要手动点击接收</span></div><a-switch v-model="editProfile.autoSave" @change="saveProfile(false)" /></div></section><section class="setting-card"><h3>文件</h3><div class="setting-line"><div><strong>保存路径</strong><span class="path">{{ editProfile.fileSavePath || '未设置' }}</span></div><a-button @click="chooseDirectory">选择目录</a-button></div></section></main>
      <main class="settings-content" v-else-if="settingsTab === 'network'"><section class="setting-card network-card"><div class="network-summary"><div class="network-dot" :class="store.network.status" /><div><strong>{{ store.network.status === 'normal' ? '网络正常' : '网络需要检查' }}</strong><span>{{ store.network.localIps.join('、') || '尚未获取局域网地址' }}</span></div><a-button type="primary" @click="runDiagnostic">网络诊断</a-button></div><div class="diagnostic-list" v-if="diagnostic"><div v-for="item in diagnostic.items" :key="item.name" class="diagnostic-row"><span :class="['diagnostic-icon', item.status]">{{ item.status === 'ok' ? '✓' : '!' }}</span><div><strong>{{ item.name }}</strong><span>{{ item.detail }} · {{ item.status === 'ok' ? '正常' : item.advice }}</span></div></div></div></section><section class="setting-card"><h3>监听信息</h3><div class="setting-line"><div><strong>UDP 发现端口</strong><span>用于局域网设备发现</span></div><code>{{ store.network.discoveryPort }}</code></div><div class="setting-line"><div><strong>TCP 发现端口</strong><span>UDP 不可用时的设备发现</span></div><code>{{ store.network.discoveryPort }}</code></div><div class="setting-line"><div><strong>TCP/TLS 聊天端口</strong><span>用于好友连接和消息传输</span></div><code>{{ store.network.chatPort || '启动中' }}</code></div><div class="setting-line"><div><strong>设备状态</strong><span>{{ store.network.peerCount }} 台已发现，{{ store.network.onlineCount }} 台在线</span></div><a-button @click="refreshPeers">重新扫描</a-button></div></section></main>
      <main class="settings-content" v-else-if="settingsTab === 'device'"><section class="setting-card device-card"><div class="device-fields"><label>平台<strong>{{ deviceInfo?.platform }}</strong></label><label>操作系统<strong>{{ deviceInfo?.osVersion }}</strong></label><label>设备 ID<strong class="mono">{{ deviceInfo?.deviceId }}</strong></label><label>证书指纹<strong class="mono">{{ deviceInfo?.certificateFingerprint }}</strong></label></div></section></main>
      <main class="settings-content" v-else><section class="setting-card about-card"><div class="brand-mark">✦</div><h2>POPChat</h2><p>局域网点对点聊天工具</p><div class="about-rows"><span>应用版本<strong>0.1.0</strong></span><span>协议版本<strong>POPChat/1.0</strong></span><span>数据存储<strong>本地 SQLite</strong></span></div><a-button @click="termsVisible = true">使用条款与隐私说明</a-button></section></main>
      </div>
    </section>
    <a-modal v-model:visible="termsVisible" title="使用条款与隐私说明" hide-cancel><p>POPChat 仅在局域网内进行点对点通信。聊天记录、设备信息和附件保存在本机，不上传云端。请确认你有权在当前网络中发现和联系其他设备。</p></a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { System, Window } from '@wailsio/runtime'
import { ChatService } from '/#/helpfly/internal/service'
import { useChatStore } from '@/store/modules/chat'
import type { FriendRequest, Peer } from '@/store/modules/chat/types'

const store = useChatStore()
const section = ref('friends')
const settingsTab = ref('general')
const friendSearch = ref('')
const draft = ref('')
const pickedFile = ref('')
const showPeerInfo = ref(false)
const selectedRequest = ref<FriendRequest>()
const selectedDiscovery = ref<Peer>()
const termsVisible = ref(false)
const diagnostic = ref<any>()
const deviceInfo = ref<any>()
const isDark = ref(false)
const isMac = ref(false)
const editProfile = reactive({ ...store.profile })
const groups = reactive({ requests: true, discovered: true })

const filteredFriends = computed(() => store.friends.filter((peer) => `${peer.nickname}${peer.remark}`.toLowerCase().includes(friendSearch.value.toLowerCase())))
const activePeer = computed(() => store.activePeer)
const activeMessages = computed(() => activePeer.value ? store.messages[`conv-${activePeer.value.deviceId}`] || [] : [])

function initials(value: string) { return (value || '?').trim().slice(0, 1).toUpperCase() }
function avatarStyle(value: string, image?: string) { if (image) return { backgroundImage: `url(${image})`, backgroundSize: 'cover', backgroundPosition: 'center' }; let hash = 0; for (const char of value || '?') hash = (hash * 31 + char.charCodeAt(0)) >>> 0; const hue = hash % 360; return { background: `linear-gradient(135deg, hsl(${hue} 80% 65%), hsl(${(hue + 42) % 360} 75% 45%))` } }
function formatTime(value: string) { if (!value) return ''; return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }
function applyTheme(theme: string) { const dark = theme === 'dark' || (theme === 'system' && window.matchMedia?.('(prefers-color-scheme: dark)').matches); isDark.value = Boolean(dark); if (dark) { document.body.setAttribute('arco-theme', 'dark'); document.body.classList.add('popchat-dark') } else { document.body.removeAttribute('arco-theme'); document.body.classList.remove('popchat-dark') } }
async function load() { try { store.profile = await ChatService.GetProfile(); Object.assign(editProfile, store.profile); applyTheme(store.profile.theme); deviceInfo.value = await ChatService.GetDeviceInfo(); store.peers = await ChatService.ListPeers(); store.requests = await ChatService.ListFriendRequests(); store.conversations = await ChatService.ListConversations(); store.network = await ChatService.NetworkStatus() } catch (error: any) { Message.error(error?.message || '初始化聊天服务失败') } }
function selectPeer(peer: Peer) { store.selectPeer(peer.deviceId); showPeerInfo.value = false; ChatService.EnsureConversation(peer.deviceId).then((id) => ChatService.ListMessages(id).then((messages) => { store.messages[id] = messages })) }
function openSettings(tab: string) { section.value = 'settings'; settingsTab.value = tab }
async function saveProfile(showMessage = true) { try { const profile = await ChatService.UpdateProfile({ ...editProfile }); store.$patch({ profile: { ...store.profile, ...profile } }); Object.assign(editProfile, profile); applyTheme(profile.theme); if (showMessage) Message.success('设置已保存') } catch (error: any) { Message.error(error?.message || '保存失败') } }
function syncNickname() { editProfile.nickname = editProfile.nickname.trim() }
async function toggleStartup() { try { store.profile = await ChatService.SetLaunchAtStartup(editProfile.launchAtStartup); Object.assign(editProfile, store.profile) } catch (error: any) { editProfile.launchAtStartup = !editProfile.launchAtStartup; Message.error(error?.message || '设置失败') } }
async function chooseDirectory() { const path = await ChatService.PickDirectory(); if (path) { editProfile.fileSavePath = path; await saveProfile() } }
async function chooseAvatar() { const path = await ChatService.PickFile(); if (path) { try { store.profile = await ChatService.SetAvatar(path); Object.assign(editProfile, store.profile); Message.success('头像已更新') } catch (error: any) { Message.error(error?.message || '头像更新失败') } } }
async function resetAvatar() { try { const theme = editProfile.theme; const profile = await ChatService.ResetAvatar(); const nextProfile = { ...profile, theme: theme || profile.theme }; store.$patch({ profile: { ...store.profile, ...nextProfile } }); Object.assign(editProfile, nextProfile); applyTheme(theme || profile.theme) } catch (error: any) { Message.error(error?.message || '恢复头像失败') } }
async function refreshPeers() { await ChatService.ScanPeers(); await new Promise((resolve) => setTimeout(resolve, 700)); store.peers = await ChatService.ListPeers(); store.network = await ChatService.NetworkStatus(); Message.success('已刷新局域网设备') }
async function addPeer() { if (!selectedDiscovery.value) return; try { await ChatService.SendFriendRequest(selectedDiscovery.value.deviceId, '你好，我想和你成为好友'); Message.success('好友申请已发送') } catch (error: any) { Message.error(error?.message || '发送申请失败') } }
async function acceptRequest() { if (!selectedRequest.value) return; await ChatService.AcceptFriendRequest(selectedRequest.value.requestId); Message.success('已添加好友'); selectedRequest.value = undefined; store.requests = await ChatService.ListFriendRequests(); store.peers = await ChatService.ListPeers() }
async function rejectRequest() { if (!selectedRequest.value) return; await ChatService.RejectFriendRequest(selectedRequest.value.requestId); selectedRequest.value = undefined; store.requests = await ChatService.ListFriendRequests() }
async function sendMessage() { if (!activePeer.value || !draft.value.trim()) return; try { const message = await ChatService.SendMessage(activePeer.value.deviceId, draft.value.trim()); const id = message.conversationId; store.messages[id] = [...(store.messages[id] || []), message]; draft.value = '' } catch (error: any) { Message.error(error?.message || '发送失败') } }
async function pickFile() { pickedFile.value = await ChatService.PickFile(); if (pickedFile.value && activePeer.value) { try { const message = await ChatService.SendFile(activePeer.value.deviceId, pickedFile.value); store.messages[message.conversationId] = [...(store.messages[message.conversationId] || []), message]; Message.success('文件已发送') } catch (error: any) { Message.error(error?.message || '文件发送失败') } finally { pickedFile.value = '' } } }
async function acceptAttachment(message: any) { try { await ChatService.AcceptAttachment(message.attachmentId); message.attachmentStatus = 'saved'; Message.success('文件已保存') } catch (error: any) { Message.error(error?.message || '接收文件失败') } }
async function rejectAttachment(message: any) { try { await ChatService.RejectAttachment(message.attachmentId); message.attachmentStatus = 'rejected' } catch (error: any) { Message.error(error?.message || '拒绝文件失败') } }
function formatBytes(value: number) { if (!value) return '未知大小'; if (value < 1024) return `${value} B`; if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB` }
async function runDiagnostic() { diagnostic.value = await ChatService.RunNetworkDiagnostic() }
function minimiseWindow() { Window.Minimise() }
async function toggleMaximise() { if (await Window.IsMaximised()) Window.UnMaximise(); else Window.Maximise() }
function closeWindow() { Window.Close() }
watch(() => store.profile, (value) => Object.assign(editProfile, value), { deep: true })
watch(() => editProfile.theme, (value) => applyTheme(value))
onMounted(() => { isMac.value = System.IsMac(); load() })
</script>

<style scoped lang="less">
.chat-app { height: 100%; display: flex; overflow: hidden; background: #f5f7fb; color: #1d2129; }
.rail { width: 76px; flex: 0 0 76px; background: #17233c; display: flex; align-items: center; flex-direction: column; padding: 22px 10px 16px; box-sizing: border-box; color: #c9d4e8; }
.profile-button, .rail-nav button, .rail-settings { border: 0; background: transparent; color: inherit; cursor: pointer; border-radius: 14px; }
.profile-button { padding: 0; margin-bottom: 28px; }.rail-nav { display: flex; flex-direction: column; gap: 10px; align-items: center; flex: 1; }.rail-nav button, .rail-settings { width: 54px; height: 58px; position: relative; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; }.rail-nav button span, .rail-settings span { font-size: 22px; line-height: 22px; }.rail-nav small, .rail-settings small { font-size: 11px; }.rail-nav button.active, .rail-settings.active { color: #fff; background: #2e5bba; }.rail-nav b { position: absolute; top: 2px; right: 5px; min-width: 16px; height: 16px; border-radius: 9px; background: #f53f3f; color: #fff; font-size: 10px; line-height: 16px; }
.avatar { width: 44px; height: 44px; border-radius: 14px; color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 700; position: relative; flex: 0 0 auto; }.avatar.large { width: 46px; height: 46px; border-radius: 15px; }.avatar.huge { width: 92px; height: 92px; border-radius: 28px; font-size: 30px; }.avatar i { position: absolute; width: 10px; height: 10px; border: 2px solid #fff; border-radius: 50%; background: #86909c; bottom: -1px; right: -1px; }.avatar i.online { background: #00b42a; }
.workspace { flex: 1; display: flex; min-width: 0; }.list-pane { width: 290px; flex: 0 0 290px; background: #fff; border-right: 1px solid #e5e6eb; display: flex; flex-direction: column; }.pane-title { padding: 26px 20px 18px; display: flex; justify-content: space-between; align-items: center; }.pane-title div { display: flex; align-items: baseline; gap: 8px; }.pane-title strong { font-size: 22px; }.pane-title span { color: #86909c; font-size: 13px; }.icon-button { border: 0; background: transparent; cursor: pointer; color: #4e5969; font-size: 22px; }.search { margin: 0 16px 14px; width: calc(100% - 32px); }.list-scroll { flex: 1; overflow: auto; padding: 0 10px 20px; }.peer-row, .request-row { width: 100%; border: 0; background: transparent; text-align: left; display: flex; align-items: center; gap: 12px; padding: 11px 10px; border-radius: 12px; cursor: pointer; }.peer-row:hover, .request-row:hover, .peer-row.selected, .request-row.selected { background: #f2f5ff; }.peer-copy, .request-row > div:last-child { display: flex; flex-direction: column; gap: 4px; min-width: 0; }.peer-copy strong, .request-row strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.peer-copy span, .request-row span { font-size: 12px; color: #86909c; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.empty-small { text-align: center; color: #86909c; padding: 90px 20px; }.empty-icon, .brand-mark { font-size: 42px; color: #4e7cff; }.conversation, .detail-pane, .blank-state { flex: 1; min-width: 0; display: flex; flex-direction: column; }.conversation-head { height: 76px; flex: 0 0 76px; background: #fff; border-bottom: 1px solid #e5e6eb; padding: 0 28px; display: flex; align-items: center; justify-content: space-between; }.head-peer { display: flex; gap: 12px; align-items: center; }.head-peer > div:last-child { display: flex; flex-direction: column; gap: 4px; }.head-peer span { font-size: 12px; color: #86909c; }.onlineText { color: #00b42a !important; }.message-scroll { flex: 1; overflow: auto; padding: 28px 12%; }.message-line { display: flex; margin: 12px 0; }.message-line.mine { justify-content: flex-end; }.message-bubble { max-width: 65%; padding: 11px 15px; border-radius: 16px 16px 16px 4px; background: #fff; box-shadow: 0 4px 16px rgba(28, 49, 93, .05); white-space: pre-wrap; line-height: 1.55; }.message-line.mine .message-bubble { color: #fff; background: #3767e8; border-radius: 16px 16px 4px 16px; }.message-bubble small { display: block; opacity: .65; font-size: 10px; margin-top: 5px; }.conversation-empty, .blank-state { align-items: center; justify-content: center; color: #86909c; }.conversation-empty h3, .conversation-empty p { margin: 4px; }.blank-state h2, .blank-state p { margin: 7px; }.composer { padding: 12px 24px 18px; background: #fff; border-top: 1px solid #e5e6eb; }.composer-tools { height: 28px; display: flex; align-items: center; gap: 10px; }.composer-tools button { border: 0; background: transparent; color: #4e5969; font-size: 18px; cursor: pointer; }.picked-file { color: #4e7cff; font-size: 12px; }.composer textarea { display: block; width: 100%; min-height: 68px; border: 0; outline: none; resize: none; font-size: 14px; padding: 8px 0; box-sizing: border-box; }.composer-foot { display: flex; align-items: center; justify-content: space-between; color: #86909c; font-size: 12px; }.info-pane { width: 280px; flex: 0 0 280px; background: #fff; border-left: 1px solid #e5e6eb; padding: 24px 20px; }.info-head { display: flex; justify-content: space-between; }.info-profile { text-align: center; padding: 30px 0 24px; }.info-profile .avatar { margin: auto; }.info-profile h3 { margin: 12px 0 4px; }.info-profile span { color: #00b42a; font-size: 12px; }.info-fields, .basic-info, .device-fields { display: flex; flex-direction: column; gap: 18px; }.info-fields label, .basic-info label, .device-fields label { color: #86909c; font-size: 12px; display: flex; flex-direction: column; gap: 5px; }.info-fields strong, .basic-info strong, .device-fields strong { color: #1d2129; font-weight: 500; word-break: break-all; }.mono { font-family: monospace; font-size: 11px; }.discovery-pane { width: 320px; flex-basis: 320px; }.group-title { border: 0; background: transparent; display: flex; justify-content: space-between; width: 100%; padding: 14px 20px 7px; cursor: pointer; color: #4e5969; font-weight: 600; }.group-title b { background: #e8f3ff; color: #165dff; padding: 1px 7px; border-radius: 10px; }.request-row { padding: 12px 18px; }.detail-pane { overflow: auto; align-items: center; justify-content: center; padding: 40px; box-sizing: border-box; }.detail-card { width: min(440px, 100%); background: #fff; border-radius: 20px; padding: 42px; box-sizing: border-box; text-align: center; box-shadow: 0 16px 50px rgba(32, 56, 99, .08); }.detail-card .avatar { margin: auto; }.detail-card h2 { margin: 18px 0 8px; }.detail-card p { color: #4e5969; line-height: 1.6; }.detail-actions { display: flex; justify-content: center; gap: 12px; margin-top: 26px; }.subtle { color: #86909c; font-size: 12px; }.tags { display: flex; justify-content: center; gap: 8px; margin: 16px; }.basic-info { text-align: left; padding: 18px 0 25px; }.settings-shell { flex: 1; overflow: auto; }.settings-head { padding: 30px 52px 0; background: #fff; }.settings-head h2 { margin: 0 0 6px; font-size: 26px; }.settings-head p { color: #86909c; margin: 0 0 24px; }.settings-tabs { display: flex; gap: 25px; }.settings-tabs button { border: 0; background: transparent; padding: 12px 2px; color: #86909c; cursor: pointer; border-bottom: 2px solid transparent; }.settings-tabs button.active { color: #165dff; border-color: #165dff; }.settings-content { max-width: 900px; padding: 28px 52px 60px; }.setting-card { background: #fff; border-radius: 16px; padding: 24px 28px; margin-bottom: 16px; }.setting-card h3 { margin: 0 0 16px; }.profile-card, .device-card { display: flex; gap: 28px; align-items: center; }.profile-edit { flex: 1; }.profile-edit p { color: #86909c; font-size: 12px; }.setting-line { min-height: 58px; border-top: 1px solid #f2f3f5; display: flex; align-items: center; justify-content: space-between; gap: 20px; }.setting-line > div { display: flex; flex-direction: column; gap: 5px; }.setting-line span { color: #86909c; font-size: 12px; }.path { max-width: 550px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.network-summary { display: flex; align-items: center; gap: 14px; }.network-summary > div:nth-child(2) { flex: 1; display: flex; flex-direction: column; gap: 5px; }.network-summary span { color: #86909c; font-size: 12px; }.network-dot { width: 12px; height: 12px; border-radius: 50%; background: #00b42a; }.network-dot.warning { background: #ff7d00; }.network-dot.error { background: #f53f3f; }.diagnostic-list { margin-top: 24px; border-top: 1px solid #f2f3f5; }.diagnostic-row { display: flex; align-items: center; gap: 12px; padding: 13px 0; border-bottom: 1px solid #f2f3f5; }.diagnostic-icon { width: 20px; height: 20px; border-radius: 50%; text-align: center; line-height: 20px; color: #fff; background: #00b42a; }.diagnostic-icon.error { background: #f53f3f; }.diagnostic-row div { display: flex; flex-direction: column; gap: 3px; }.diagnostic-row span:last-child { font-size: 12px; color: #86909c; }.about-card { text-align: center; padding: 60px; }.about-card .brand-mark { font-size: 60px; }.about-rows { max-width: 380px; margin: 25px auto; text-align: left; }.about-rows span { display: flex; justify-content: space-between; padding: 12px 0; border-bottom: 1px solid #f2f3f5; color: #86909c; }.about-rows strong { color: #1d2129; font-weight: 500; }
.profile-buttons { display: flex; gap: 8px; flex-wrap: wrap; }
 .attachment-meta { display: block; font-size: 12px; opacity: .72; margin-top: 6px; }
 .attachment-actions { display: flex; gap: 6px; margin-top: 8px; }

.chat-app.theme-dark { background: #101827; color: #e5e7eb; }
.chat-app.theme-dark .conversation-head,
.chat-app.theme-dark .composer,
.chat-app.theme-dark .info-pane,
.chat-app.theme-dark .settings-head,
.chat-app.theme-dark .setting-card,
.chat-app.theme-dark .detail-card { background: #182235; color: #e5e7eb; }
.chat-app.theme-dark .message-scroll,
.chat-app.theme-dark .detail-pane { background: #101827; }
.chat-app.theme-dark .list-pane,
.chat-app.theme-dark .conversation-head,
.chat-app.theme-dark .composer,
.chat-app.theme-dark .info-pane { border-color: #2c394e; }
.chat-app.theme-dark .pane-title span,
.chat-app.theme-dark .peer-copy span,
.chat-app.theme-dark .request-row span,
.chat-app.theme-dark .head-peer span,
.chat-app.theme-dark .settings-head p,
.chat-app.theme-dark .profile-edit p,
.chat-app.theme-dark .setting-line span,
.chat-app.theme-dark .subtle,
.chat-app.theme-dark .detail-card p,
.chat-app.theme-dark .composer-foot,
.chat-app.theme-dark .diagnostic-row span:last-child { color: #a9b5c7; }
.chat-app.theme-dark .icon-button,
.chat-app.theme-dark .composer-tools button,
.chat-app.theme-dark .settings-tabs button,
.chat-app.theme-dark .group-title { color: #c5d0df; }
.chat-app.theme-dark .peer-row:hover,
.chat-app.theme-dark .request-row:hover,
.chat-app.theme-dark .peer-row.selected,
.chat-app.theme-dark .request-row.selected { background: #263653; }
.chat-app.theme-dark .message-bubble { background: #253249; color: #e5e7eb; box-shadow: 0 4px 16px rgba(0, 0, 0, .18); }
.chat-app.theme-dark .composer textarea { background: transparent; color: #e5e7eb; }
.chat-app.theme-dark .composer textarea::placeholder { color: #8492a6; }
.chat-app.theme-dark .info-fields strong,
.chat-app.theme-dark .basic-info strong,
.chat-app.theme-dark .device-fields strong,
.chat-app.theme-dark .about-rows strong { color: #e5e7eb; }
.chat-app.theme-dark .setting-line,
.chat-app.theme-dark .about-rows span,
.chat-app.theme-dark .diagnostic-list,
.chat-app.theme-dark .diagnostic-row { border-color: #2c394e; }
.chat-app.theme-dark .group-title b { background: #203c69; color: #8db5ff; }
.chat-app.theme-dark .detail-card { box-shadow: 0 16px 50px rgba(0, 0, 0, .24); }

/* Final surface system: the app is intentionally divided into distinct layers. */
.chat-app:not(.theme-dark) {
  --app-bg: #edf0f3;
  --surface-1: #f7f8fa;
  --surface-2: #e6eaef;
  --surface-3: #dde3e9;
  --surface-4: #d3dae2;
  --line: #cfd6de;
  --text: #20252b;
  --muted: #5d6874;
  --hover: #e1e7ef;
  --list-bg: #f1f3f6;
  --accent: #5c7398;
  --shadow: 0 12px 30px rgba(37, 48, 62, .08);
}
.chat-app.theme-dark {
  --app-bg: #0f1115;
  --surface-1: #15181d;
  --surface-2: #1b2027;
  --surface-3: #242a32;
  --surface-4: #2d343d;
  --line: #39424d;
  --text: #f0f2f5;
  --muted: #a4adb8;
  --hover: #2a3442;
  --list-bg: #202428;
  --accent: #7897d0;
  --shadow: 0 14px 36px rgba(0, 0, 0, .28);
}
.chat-app,
.chat-app .workspace,
.chat-app .settings-shell { background: var(--app-bg); color: var(--text); }
.chat-app .list-pane,
.chat-app .conversation-head,
.chat-app .composer,
.chat-app .info-pane,
.chat-app .settings-head,
.chat-app .settings-nav,
.chat-app .setting-card,
.chat-app .detail-card { background: var(--surface-1); color: var(--text); border-color: var(--line); }
.chat-app .message-scroll,
.chat-app .detail-pane,
.chat-app .settings-panel { background: var(--app-bg); }
.chat-app .list-pane { border-right-color: var(--line); }
.chat-app .conversation-head,
.chat-app .composer { border-color: var(--line); }
.chat-app .peer-row:hover,
.chat-app .request-row:hover,
.chat-app .peer-row.selected,
.chat-app .request-row.selected { background: var(--hover); }
.chat-app .request-row { color: var(--text); }
.chat-app .request-row strong { color: var(--text); font-weight: 600; }
.chat-app .request-row span { color: var(--muted); }
.chat-app .peer-row { color: var(--text); }
.chat-app .peer-row strong { color: var(--text); font-weight: 600; }
.chat-app .peer-row span { color: var(--muted); }
.chat-app .message-bubble { background: var(--surface-1); color: var(--text); box-shadow: var(--shadow); }
.chat-app .composer textarea { background: transparent; color: var(--text); }
.chat-app .composer textarea::placeholder { color: var(--muted); }
.chat-app .pane-title span,
.chat-app .peer-copy span,
.chat-app .request-row span,
.chat-app .head-peer span,
.chat-app .settings-head p,
.chat-app .profile-edit p,
.chat-app .setting-line span,
.chat-app .subtle,
.chat-app .detail-card p,
.chat-app .composer-foot { color: var(--muted); }
.chat-app .info-fields strong,
.chat-app .basic-info strong,
.chat-app .device-fields strong,
.chat-app .about-rows strong { color: var(--text); }
.chat-app .setting-line,
.chat-app .about-rows span,
.chat-app .diagnostic-list,
.chat-app .diagnostic-row { border-color: var(--line); }

.settings-shell { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; }
.settings-head { flex: 0 0 auto; padding: 28px 34px 22px; border-bottom: 1px solid var(--line); }
.settings-head h2 { margin: 0 0 6px; color: var(--text); font-size: 25px; }
.settings-head p { margin: 0; color: var(--muted); }
.settings-layout { flex: 1; min-height: 0; min-width: 0; display: grid; grid-template-columns: 226px minmax(0, 1fr); }
.settings-nav { min-width: 0; padding: 18px 12px; border-right: 1px solid var(--line); }
.settings-nav button { width: 100%; display: grid; grid-template-columns: 30px minmax(0, 1fr); grid-template-rows: 22px 18px; column-gap: 8px; align-items: center; border: 0; border-radius: 12px; padding: 12px 12px; margin-bottom: 7px; background: transparent; color: var(--muted); text-align: left; cursor: pointer; transition: background .18s ease, color .18s ease; }
.settings-nav button > span { grid-row: 1 / span 2; align-self: center; text-align: center; font-size: 20px; color: currentColor; }
.settings-nav button strong { color: inherit; font-size: 14px; font-weight: 600; }
.settings-nav button small { color: inherit; font-size: 11px; line-height: 16px; opacity: .8; }
.settings-nav button:hover { background: var(--hover); color: var(--text); }
.settings-nav button.active { background: #315fbd; color: #fff; box-shadow: 0 6px 16px rgba(49, 95, 189, .24); }
.settings-nav button.active small { color: #dce8ff; }
.settings-panel { min-width: 0; min-height: 0; overflow: auto; }
.settings-content { width: 100%; max-width: none; box-sizing: border-box; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; gap: 18px; padding: 28px 34px 56px; }
.settings-content > .setting-card { min-width: 0; margin: 0; }
.settings-content > .profile-card,
.settings-content > .network-card,
.settings-content > .device-card { grid-column: 1 / -1; }
.settings-content > .network-card + .setting-card { grid-column: 1 / -1; }
.setting-card { border: 1px solid var(--line); border-radius: 14px; box-shadow: none; }
.settings-content .setting-card h3 { color: var(--text); }
.profile-card { min-height: 154px; }
.device-card { display: block; min-height: 190px; }
.device-card .device-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 22px 42px; padding: 10px 8px; }
.device-card .device-fields label { min-width: 0; }
.about-card { width: min(760px, 100%); justify-self: center; }

.chat-app.theme-dark .settings-nav,
.chat-app.theme-dark .settings-panel,
.chat-app.theme-dark .settings-head { background: var(--surface-1); }
.chat-app.theme-dark .settings-panel { background: var(--app-bg); }
.chat-app.theme-dark .setting-card { background: var(--surface-2); }
.chat-app.theme-dark .setting-card:hover { border-color: #4a5664; }
.chat-app.theme-dark .settings-nav button.active { background: #426fc9; }
.chat-app.theme-dark .settings-nav button:hover:not(.active) { background: var(--hover); }
.chat-app.theme-dark :deep(.arco-input-wrapper),
.chat-app.theme-dark :deep(.arco-select-view),
.chat-app.theme-dark :deep(.arco-textarea-wrapper) { background: var(--surface-3); border-color: var(--line); color: var(--text); }
.chat-app.theme-dark :deep(.arco-input),
.chat-app.theme-dark :deep(.arco-textarea),
.chat-app.theme-dark :deep(.arco-select-view-value) { color: var(--text); }
.chat-app.theme-dark :deep(.arco-input::placeholder),
.chat-app.theme-dark :deep(.arco-textarea::placeholder) { color: var(--muted); }
:global(body.popchat-dark .arco-trigger-popup),
:global(body.popchat-dark .arco-select-popup),
:global(body.popchat-dark .arco-modal-container) { background: #1b2027; color: #f0f2f5; border-color: #39424d; }

@media (max-width: 1050px) {
  .settings-layout { grid-template-columns: 196px minmax(0, 1fr); }
  .settings-content { padding: 24px 24px 48px; gap: 14px; }
}
@media (max-width: 860px) {
  .settings-layout { grid-template-columns: 176px minmax(0, 1fr); }
  .settings-content { grid-template-columns: minmax(0, 1fr); }
  .settings-content > .setting-card { grid-column: 1; }
  .device-card .device-fields { grid-template-columns: minmax(0, 1fr); }
}

/* Compact, unified navigation and the original top-level settings tabs. */
.rail { width: 64px; flex-basis: 64px; padding: 18px 7px 14px; background: var(--surface-2); color: var(--muted); border-right: 1px solid var(--line); }
.profile-button, .rail-nav button, .rail-settings { color: var(--muted); }
.rail-nav button, .rail-settings { width: 50px; height: 54px; }
.rail-nav button.active, .rail-settings.active { color: #fff; background: var(--accent); box-shadow: 0 5px 14px rgba(73, 109, 182, .18); }
.rail-nav button:hover:not(.active), .rail-settings:hover:not(.active), .profile-button:hover { background: var(--hover); color: var(--text); }
.settings-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; }
.settings-tabs { display: flex; align-items: flex-end; gap: 26px; }
.settings-tabs button { color: var(--muted); }
.settings-tabs button:hover { color: var(--text); }
.settings-tabs button.active { color: var(--accent); border-color: var(--accent); }
.settings-panel { flex: 1; min-height: 0; min-width: 0; overflow: auto; }
.settings-content { grid-template-columns: repeat(2, minmax(0, 1fr)); width: min(100%, 1320px); margin: 0 auto; }
.chat-app.theme-dark .settings-head { background: var(--surface-2); }
.chat-app.theme-dark .settings-tabs button.active { color: #8eafff; border-color: #8eafff; }
.chat-app.theme-dark .rail { background: var(--surface-2); }
.chat-app.theme-dark .rail-nav button.active, .chat-app.theme-dark .rail-settings.active { background: var(--accent); }

@media (min-width: 1250px) {
  .settings-content { padding-left: 48px; padding-right: 48px; }
}

/* Return settings to the original single-column rhythm, but center it in the available window. */
.chat-app .list-pane,
.chat-app .discovery-pane { background: var(--list-bg); }
.settings-content { grid-template-columns: minmax(0, 1fr); width: min(100%, 980px); padding: 28px 40px 56px; gap: 16px; }
.settings-content > .setting-card,
.settings-content > .profile-card,
.settings-content > .network-card,
.settings-content > .device-card,
.settings-content > .network-card + .setting-card { grid-column: auto; }
.settings-content > .setting-card { width: 100%; }
.device-card .device-fields { grid-template-columns: minmax(0, 1fr); max-width: 720px; margin: 0 auto; }
.about-card { width: min(100%, 760px); margin-left: auto; margin-right: auto; }

@media (max-width: 760px) {
  .settings-head { align-items: flex-start; flex-direction: column; gap: 12px; }
  .settings-tabs { width: 100%; justify-content: space-between; gap: 12px; overflow-x: auto; }
  .settings-content { padding-left: 24px; padding-right: 24px; }
}

/* The about page is a centered card, not a full-window colored panel. */
.chat-app .settings-panel,
.chat-app .settings-content,
.chat-app .settings-content > .setting-card { background: var(--surface-1); }
.chat-app.theme-dark .setting-card,
.chat-app.theme-dark .settings-content > .about-card { background: var(--surface-1); }
.settings-content { width: min(1180px, calc(100% - clamp(32px, 7vw, 120px))); max-width: none; padding: 28px 0 56px; }
.settings-content > .about-card { width: min(760px, 100%); justify-self: center; }
.about-card { min-height: 0; padding: 44px 52px; }

/* Keep every settings page inside the visible window at narrow sizes. */
.settings-shell,
.settings-panel,
.settings-content,
.settings-content > .setting-card { min-width: 0; max-width: 100%; overflow-x: hidden; }
.settings-content > .setting-card { box-sizing: border-box; }
.profile-card { min-width: 0; }
.profile-edit { min-width: 0; }
.profile-edit :deep(.arco-input-wrapper) { max-width: 100%; }
.setting-line { min-width: 0; flex-wrap: wrap; padding-top: 8px; padding-bottom: 8px; }
.setting-line > div { min-width: 0; flex: 1 1 240px; }
.setting-line > code,
.setting-line > :deep(.arco-btn),
.setting-line > :deep(.arco-select-view) { flex: 0 0 auto; max-width: 100%; }
.network-summary { min-width: 0; flex-wrap: wrap; }
.network-summary > div:nth-child(2) { min-width: 0; flex: 1 1 240px; }
.network-summary > :deep(.arco-btn) { flex: 0 0 auto; }
.path,
.mono,
.info-fields strong,
.basic-info strong,
.device-fields strong { min-width: 0; max-width: 100%; overflow-wrap: anywhere; word-break: break-word; }
.device-card .device-fields { min-width: 0; width: 100%; }
.about-card { box-sizing: border-box; }

@media (max-width: 1050px) {
  .settings-content { width: calc(100% - 32px); }
  .setting-card { padding-left: 22px; padding-right: 22px; }
  .profile-card { gap: 18px; }
}

/* Selected navigation follows the current neutral surface instead of using a blue fill. */
.rail-nav button.active,
.rail-settings.active { box-sizing: border-box; color: var(--text); background: var(--surface-4); border-left: 3px solid var(--accent); box-shadow: none; }
.rail-nav button.active:hover,
.rail-settings.active:hover { background: var(--surface-4); color: var(--text); }
.chat-app.theme-dark .rail-nav button.active,
.chat-app.theme-dark .rail-settings.active { background: var(--surface-4); color: var(--text); }
.settings-tabs button.active { color: var(--accent); border-color: var(--accent); }

/* Friends and discovery right panes use the same content surface as settings. */
.chat-app .conversation,
.chat-app .message-scroll,
.chat-app .blank-state,
.chat-app .detail-pane { background: var(--surface-1); }
.chat-app .conversation-head,
.chat-app .composer,
.chat-app .info-pane { background: var(--surface-1); }
.chat-app.theme-dark .list-pane,
.chat-app.theme-dark .discovery-pane { background: var(--list-bg); }
.chat-app.theme-dark .message-scroll,
.chat-app.theme-dark .detail-pane { background: var(--surface-1); }
.chat-app.theme-dark .list-pane .peer-row:hover,
.chat-app.theme-dark .list-pane .peer-row.selected,
.chat-app.theme-dark .discovery-pane .request-row:hover,
.chat-app.theme-dark .discovery-pane .request-row.selected { background: #2b3035; }
.chat-app:not(.theme-dark) .list-pane .peer-row:hover,
.chat-app:not(.theme-dark) .list-pane .peer-row.selected,
.chat-app:not(.theme-dark) .discovery-pane .request-row:hover,
.chat-app:not(.theme-dark) .discovery-pane .request-row.selected { background: #e8e5e1; }

/* macOS-only frameless chrome. Windows keeps its native framed titlebar and the
   right-side layout does not receive any extra titlebar padding. */
:global(html),
:global(body),
:global(#app) { background: transparent; }
.chat-app { position: relative; border-radius: 16px; overflow: hidden; }
.window-drag-region {
  position: fixed;
  z-index: 20;
  top: 0;
  left: 0;
  right: 0;
  height: 38px;
  --wails-draggable: drag;
  --wails-non-client-region: caption;
  background: transparent;
}
.mac-window-controls {
  position: fixed;
  z-index: 30;
  top: 10px;
  left: 8px;
  width: 48px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.mac-control {
  width: 12px;
  height: 12px;
  flex: 0 0 12px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  cursor: pointer;
  box-shadow: inset 0 0 0 0.5px rgba(0, 0, 0, .18), 0 1px 2px rgba(0, 0, 0, .12);
  position: relative;
}
.mac-control.close { background: #ff5f57; }
.mac-control.minimise { background: #febc2e; }
.mac-control.maximise { background: #28c840; }
.mac-control::before {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(58, 38, 20, .78);
  font-family: Arial, sans-serif;
  font-size: 9px;
  font-weight: 700;
  line-height: 12px;
  opacity: 1;
}
.mac-control.close::before { content: '×'; color: rgba(75, 18, 15, .82); }
.mac-control.minimise::before { content: '−'; color: rgba(83, 54, 7, .86); }
.mac-control.maximise::before { content: '＋'; color: rgba(12, 72, 25, .82); font-size: 8px; }
.mac-control:hover { filter: brightness(.9); }
.mac-control:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.chat-app.is-mac .rail {
  position: relative;
  z-index: 1;
  padding-top: 48px;
}
.workspace > .list-pane,
.workspace > .conversation,
.workspace > .detail-pane,
.workspace > .blank-state,
.workspace > .info-pane { min-height: 0; }

</style>
