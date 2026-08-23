<template>
  <div class="chat-app" :class="{ 'theme-dark': isDark, 'is-mac': isMac }">
    <div v-if="isMac" class="window-drag-region" aria-hidden="true"></div>
    <div v-if="isMac" class="mac-window-controls" aria-label="macOS 窗口控制">
      <button type="button" class="mac-control close" title="关闭" @click.stop="closeWindow"></button>
      <button type="button" class="mac-control minimise" title="最小化" @click.stop="minimiseWindow"></button>
      <button type="button" class="mac-control maximise" title="最大化" @click.stop="toggleMaximise"></button>
    </div>
    <aside class="rail" :class="{ 'is-locked': store.attachmentMigration.active }">
      <button class="profile-button" :class="{ active: section === 'settings' }" @click="openSettings('general')" :disabled="store.attachmentMigration.active">
        <div class="avatar large" :style="avatarStyle(store.profile.nickname, store.profile.avatarData)">{{ store.profile.avatarData ? '' : initials(store.profile.nickname) }}</div>
      </button>
      <nav class="rail-nav">
        <button :class="{ active: section === 'friends' }" @click="section = 'friends'" :disabled="store.attachmentMigration.active" aria-label="好友"><icon-user-group /><small>好友</small><b v-if="totalUnreadCount" class="rail-unread-badge">{{ unreadLabel(totalUnreadCount) }}</b></button>
        <button :class="{ active: section === 'discover' }" @click="section = 'discover'" :disabled="store.attachmentMigration.active" aria-label="发现"><icon-search /><small>发现</small><b v-if="store.requests.length">{{ store.requests.length }}</b></button>
      </nav>
      <button class="rail-settings" :class="{ active: section === 'settings' }" @click="openSettings('general')" :disabled="store.attachmentMigration.active" aria-label="设置"><icon-settings /><small>设置</small></button>
    </aside>

    <section v-if="section === 'friends'" class="workspace">
      <aside class="list-pane" :style="{ width: `${friendsWidth}px`, flexBasis: `${friendsWidth}px` }">
        <div class="pane-title"><div><strong>好友</strong><span>{{ store.friends.length }}</span></div><button class="icon-button" @click="section = 'discover'" title="发现好友"><icon-plus /></button></div>
        <a-input v-model="friendSearch" class="search" placeholder="搜索好友" allow-clear />
        <div class="list-scroll">
          <button v-for="peer in filteredFriends" :key="peer.deviceId" class="peer-row" :class="{ selected: store.activePeerId === peer.deviceId }" @click="selectPeer(peer)">
            <div class="avatar" :style="avatarStyle(peer.nickname, peer.avatarData)">{{ peer.avatarData ? '' : initials(peer.nickname) }}<i :class="{ online: peer.online }" /></div>
            <div class="peer-copy"><strong>{{ peer.remark || peer.nickname }}</strong><span>{{ peer.online ? '在线' : '离线' }}</span></div><b v-if="unreadCount(peer.deviceId)" class="unread-badge">{{ unreadLabel(unreadCount(peer.deviceId)) }}</b>
          </button>
          <div v-if="!filteredFriends.length" class="empty-small"><div class="empty-icon">⌁</div><p>还没有好友</p><a-button type="primary" size="small" @click="section = 'discover'">去发现好友</a-button></div>
        </div>
      </aside>
      <div class="vertical-resizer" @pointerdown="startResize('friends', $event)" title="调整列表宽度" />
        <main class="conversation" v-if="activePeer" :style="{ '--composer-height': `${composerHeight}px` }">
          <header class="conversation-head">
          <div class="head-peer"><strong>{{ activePeer.remark || activePeer.nickname }}</strong><span class="head-status" :class="{ onlineText: activePeer.online }"><i :class="{ online: activePeer.online }" />{{ activePeer.online ? '在线' : '离线' }} · {{ activePeer.platform }}</span></div>
          <a-button type="text" aria-label="好友资料" @click="showPeerInfo = !showPeerInfo" title="好友资料"><icon-more /></a-button>
        </header>
        <div class="message-scroll" ref="messageScroll" @scroll="onMessageScroll" @click="markActiveRead">
          <div v-if="!activeMessages.length" class="conversation-empty"><div class="empty-icon">✦</div><h3>开始聊天</h3><p>向 {{ activePeer.remark || activePeer.nickname }} 发送第一条消息</p></div>
          <div v-for="message in activeMessages" :key="message.messageId" class="message-line" :class="{ mine: message.senderDeviceId === deviceInfo?.deviceId }">
            <button v-if="message.senderDeviceId !== deviceInfo?.deviceId" type="button" class="avatar message-avatar avatar-button" :style="avatarStyle(activePeer.nickname, activePeer.avatarData)" aria-label="查看好友资料" title="查看好友资料" @click.stop="showPeerInfo = true">{{ activePeer.avatarData ? '' : initials(activePeer.nickname) }}</button>
            <div class="message-bubble"><template v-if="message.kind === 'file'"><button v-if="message.attachmentMime?.startsWith('image/')" class="image-message" @click="openImage(message)"><img v-if="messagePreviews[message.messageId]" :src="messagePreviews[message.messageId]" /><span v-else>图片 {{ message.attachmentName || message.content }}</span></button><template v-else><strong><icon-file /> {{ message.attachmentName || message.content }}</strong><span class="attachment-meta">{{ formatBytes(message.attachmentSize || 0) }} · {{ message.attachmentStatus || message.status }}</span><div v-if="message.senderDeviceId !== deviceInfo?.deviceId && message.attachmentStatus === 'pending'" class="attachment-actions"><a-button size="mini" type="primary" @click="acceptAttachment(message)">接收</a-button><a-button size="mini" status="danger" @click="rejectAttachment(message)">拒绝</a-button></div></template></template><template v-else>{{ message.content }}</template><small>{{ formatTime(message.createdAt) }}<template v-if="message.senderDeviceId === deviceInfo?.deviceId"> <span class="message-status">{{ messageStatusText(message.status) }}</span></template></small></div>
            <div v-if="message.senderDeviceId === deviceInfo?.deviceId" class="avatar message-avatar" :style="avatarStyle(store.profile.nickname, store.profile.avatarData)">{{ store.profile.avatarData ? '' : initials(store.profile.nickname) }}</div>
          </div>
        </div>
        <div class="horizontal-resizer" @pointerdown="startResize('composer', $event)" title="调整输入框高度" />
        <footer class="composer" :style="{ height: `${composerHeight}px` }">
          <div class="composer-tools"><button title="表情" @click="emojiOpen = !emojiOpen"><icon-face-smile-fill /></button><button title="附件" @click="pickFile"><icon-folder /></button></div>
          <div v-if="emojiOpen" class="emoji-panel"><button v-for="emoji in emojis" :key="emoji" @click="draft += emoji">{{ emoji }}</button></div>
          <div v-if="pendingImages.length" class="pending-images"><div v-for="(image, index) in pendingImages" :key="image" class="pending-image"><img :src="image" /><button @click="pendingImages.splice(index, 1)"><icon-close /></button></div></div>
          <textarea v-model="draft" placeholder="输入消息，Enter 发送，Shift + Enter 换行" @focus="markActiveRead" @paste="handlePaste" @keydown.enter.exact.prevent="sendMessage" />
          <div class="composer-foot"><span>消息将通过局域网加密传输</span><a-button type="primary" :disabled="!draft.trim() && !pendingImages.length" @click="sendMessage">发送</a-button></div>
          <button v-if="newMessageCount" class="new-message-button" @click="scrollToBottom">{{ newMessageCount }} 条新消息</button>
        </footer>
        <aside v-if="showPeerInfo" class="info-pane info-overlay" @click.stop>
          <div class="info-head"><strong>好友资料</strong><button class="icon-button" @click="showPeerInfo = false"><icon-close /></button></div>
          <div class="info-profile"><div class="avatar huge" :style="avatarStyle(activePeer.nickname, activePeer.avatarData)">{{ activePeer.avatarData ? '' : initials(activePeer.nickname) }}</div><h3>{{ activePeer.remark || activePeer.nickname }}</h3><span>{{ activePeer.online ? '在线' : '离线' }}</span></div>
          <div class="info-fields"><label>设备类型<strong>{{ activePeer.platform }} · {{ activePeer.osVersion }}</strong></label><label>备注<input v-model="peerRemark" @keyup.enter="savePeerRemark" @blur="savePeerRemark" /></label><label>IP 地址<strong>{{ activePeer.ip || '未知' }}:{{ activePeer.port || '-' }}</strong></label><label>设备 ID<strong class="mono">{{ activePeer.deviceId }}</strong></label><label>证书指纹<strong class="mono">{{ activePeer.certificateFingerprint || '未知' }}</strong></label><label>最近在线<strong>{{ formatLastSeen(activePeer.lastSeen) }}</strong></label></div>
        </aside>
        <a-modal v-model:visible="imageViewerOpen" :footer="false" :title="imageViewerName" width="min(900px, 90vw)"><div class="image-viewer"><button aria-label="上一张" title="上一张" @click="moveImage(-1)"><icon-left /></button><img v-if="imageViewerSource" :src="imageViewerSource" /><button aria-label="下一张" title="下一张" @click="moveImage(1)"><icon-right /></button></div><div v-if="imageMessages.length > 1" class="image-thumbnails"><button v-for="(image, index) in imageMessages" :key="image.messageId" :class="{ active: index === imageViewerIndex }" :title="image.attachmentName || '图片'" @click="imageViewerIndex = index"><img :src="messagePreviews[image.messageId]" /></button></div></a-modal>
      </main>
      <main v-else class="blank-state"><div class="brand-mark">✦</div><h2>POPChat</h2><p>选择一位好友开始聊天</p><a-button type="primary" @click="section = 'discover'">发现局域网好友</a-button></main>
    </section>

    <section v-else-if="section === 'discover'" class="workspace">
      <aside class="list-pane discovery-pane" :style="{ width: `${discoveryWidth}px`, flexBasis: `${discoveryWidth}px` }">
        <div class="pane-title"><div><strong>发现</strong><span>{{ store.discovered.length }}</span></div><a-button size="small" @click="refreshPeers">重新扫描</a-button></div>
        <div class="discover-group"><button class="group-title" @click="groups.requests = !groups.requests"><span>{{ groups.requests ? '⌄' : '›' }} 新的朋友</span><b v-if="store.requests.length">{{ store.requests.length }}</b></button><button v-for="request in store.requests" v-show="groups.requests" :key="request.requestId" class="request-row" :class="{ selected: selectedRequest?.requestId === request.requestId }" @click="selectedRequest = request"><div class="avatar" :style="avatarStyle(request.nickname)">{{ initials(request.nickname) }}</div><div><strong>{{ request.nickname }}</strong><span>{{ request.message || '请求添加你为好友' }}</span></div></button></div>
        <div class="discover-group"><button class="group-title" @click="groups.discovered = !groups.discovered"><span>{{ groups.discovered ? '⌄' : '›' }} 已发现</span><b>{{ store.discovered.length }}</b></button><button v-for="peer in store.discovered" v-show="groups.discovered" :key="peer.deviceId" class="request-row" :class="{ selected: selectedDiscovery?.deviceId === peer.deviceId }" @click="selectedDiscovery = peer"><div class="avatar" :style="avatarStyle(peer.nickname, peer.avatarData)">{{ peer.avatarData ? '' : initials(peer.nickname) }}<i :class="{ online: peer.online }" /></div><div><strong>{{ peer.nickname }}</strong><span>{{ peer.platform }} · {{ peer.online ? '在线' : '离线' }}</span></div></button></div>
      </aside>
      <div class="vertical-resizer" @pointerdown="startResize('discover', $event)" title="调整列表宽度" />
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
      <main class="settings-content" v-if="settingsTab === 'general'"><section class="setting-card profile-card"><button class="avatar-upload" type="button" title="更换头像" @click="chooseAvatar"><div class="avatar huge" :style="avatarStyle(editProfile.nickname, editProfile.avatarData)">{{ editProfile.avatarData ? '' : initials(editProfile.nickname) }}</div><span class="avatar-camera"><icon-camera /></span></button><div class="profile-edit"><a-input v-model="editProfile.nickname" label="昵称" maxlength="32" @blur="syncNickname" @keyup.enter.prevent="saveProfile" /><p>没有自定义头像时，系统会根据设备 ID 生成稳定头像。</p><div class="profile-buttons"><a-button type="primary" @mousedown.prevent="saveProfile">保存</a-button></div></div></section><section class="setting-card"><h3>外观</h3><div class="setting-line"><div><strong>主题</strong><span>选择应用的颜色主题</span></div><a-select v-model="editProfile.theme" style="width: 170px"><a-option value="light">亮色</a-option><a-option value="dark">暗色</a-option><a-option value="system">跟随系统</a-option></a-select></div></section><section class="setting-card"><h3>隐私与启动</h3><div class="setting-line"><div><strong>允许被发现</strong><span>关闭后，局域网设备无法在发现列表看到你</span></div><a-switch v-model="editProfile.discoverable" @change="saveProfile(false)" /></div><div class="setting-line"><div><strong>开机启动</strong><span>登录系统后自动启动 POPChat</span></div><a-switch v-model="editProfile.launchAtStartup" @change="toggleStartup" /></div><div class="setting-line"><div><strong>自动保存附件</strong><span>关闭后，收到图片和文件需要手动点击接收</span></div><a-switch v-model="editProfile.autoSave" @change="saveProfile(false)" /></div></section><section class="setting-card"><h3>文件</h3><div class="setting-line"><div><strong>保存路径</strong><span class="path">{{ editProfile.fileSavePath || '未设置' }}</span></div><div class="path-actions"><a-button @click="chooseDirectory" :disabled="store.attachmentMigration.active">选择目录</a-button><a-button v-if="!isDefaultPath" @click="resetAttachmentPath" :disabled="store.attachmentMigration.active">重置</a-button></div></div></section></main>
      <main class="settings-content" v-else-if="settingsTab === 'network'"><section class="setting-card network-card"><div class="network-summary"><div class="network-dot" :class="store.network.status" /><div><strong>{{ store.network.status === 'normal' ? '网络正常' : '网络需要检查' }}</strong><span>{{ store.network.localIps.join('、') || '尚未获取局域网地址' }}</span></div><a-button type="primary" @click="runDiagnostic">网络诊断</a-button></div><div class="diagnostic-list" v-if="diagnostic"><div v-for="item in diagnostic.items" :key="item.name" class="diagnostic-row"><span :class="['diagnostic-icon', item.status]">{{ item.status === 'ok' ? '✓' : '!' }}</span><div><strong>{{ item.name }}</strong><span>{{ item.detail }} · {{ item.status === 'ok' ? '正常' : item.advice }}</span></div></div></div></section><section class="setting-card"><h3>监听信息</h3><div class="setting-line"><div><strong>UDP 发现端口</strong><span>用于局域网设备发现</span></div><code>{{ store.network.discoveryPort }}</code></div><div class="setting-line"><div><strong>TCP 发现端口</strong><span>UDP 不可用时的设备发现</span></div><code>{{ store.network.discoveryPort }}</code></div><div class="setting-line"><div><strong>TCP/TLS 聊天端口</strong><span>用于好友连接和消息传输</span></div><code>{{ store.network.chatPort || '启动中' }}</code></div><div class="setting-line"><div><strong>设备状态</strong><span>{{ store.network.peerCount }} 台已发现，{{ store.network.onlineCount }} 台在线</span></div><a-button @click="refreshPeers">重新扫描</a-button></div></section></main>
      <main class="settings-content" v-else-if="settingsTab === 'device'"><section class="setting-card device-card"><div class="device-card-head"><div><span class="device-eyebrow">本机身份</span><h3>设备信息</h3><p>用于局域网发现与加密连接的本机凭据</p></div><span class="device-badge"><i />本机</span></div><div class="device-fields"><label><span class="device-field-label"><i class="device-field-icon">⌘</i>平台</span><strong>{{ deviceInfo?.platform || '未知' }}</strong></label><label><span class="device-field-label"><i class="device-field-icon">▣</i>操作系统</span><strong>{{ deviceInfo?.osVersion || '未知' }}</strong></label><label><span class="device-field-label"><i class="device-field-icon">ID</i>设备 ID</span><strong class="mono">{{ deviceInfo?.deviceId || '尚未生成' }}</strong></label><label><span class="device-field-label"><i class="device-field-icon">✓</i>证书指纹</span><strong class="mono">{{ deviceInfo?.certificateFingerprint || '尚未生成' }}</strong></label></div><div class="device-card-foot">设备身份信息仅保存在本机，用于验证局域网连接安全性。</div></section></main>
      <main class="settings-content" v-else><section class="setting-card about-card"><div class="brand-mark">✦</div><h2>POPChat</h2><p>局域网点对点聊天工具</p><div class="about-rows"><span>应用版本<strong>0.1.0</strong></span><span>协议版本<strong>POPChat/1.0</strong></span><span>数据存储<strong>本地 SQLite</strong></span></div><a-button @click="termsVisible = true">使用条款与隐私说明</a-button></section></main>
      </div>
    </section>
    <a-modal v-model:visible="termsVisible" title="使用条款与隐私说明" hide-cancel><p>POPChat 仅在局域网内进行点对点通信。聊天记录、设备信息和附件保存在本机，不上传云端。请确认你有权在当前网络中发现和联系其他设备。</p></a-modal>
    <div v-if="store.attachmentMigration.active || migrationResultVisible" class="migration-lock" @click.stop>
      <section class="migration-card" role="dialog" aria-modal="true" aria-label="附件迁移进度">
        <icon-loading v-if="store.attachmentMigration.active" class="migration-spinner" />
        <icon-check-circle v-else-if="!store.attachmentMigration.errorMessage" class="migration-success" />
        <icon-close-circle v-else class="migration-error" />
        <h3>{{ store.attachmentMigration.active ? '正在迁移附件' : store.attachmentMigration.errorMessage ? '附件迁移失败' : '附件迁移完成' }}</h3>
        <p class="migration-path">{{ store.attachmentMigration.targetRoot }}</p>
        <a-progress :percent="migrationPercent" :status="store.attachmentMigration.errorMessage ? 'danger' : 'normal'" />
        <p>{{ store.attachmentMigration.current }} / {{ store.attachmentMigration.total }} · 已迁移 {{ store.attachmentMigration.migrated }} · 跳过 {{ store.attachmentMigration.skipped }} · 失败 {{ store.attachmentMigration.failed }}</p>
        <p v-if="store.attachmentMigration.fileName" class="migration-file">{{ store.attachmentMigration.fileName }}</p>
        <p v-if="store.attachmentMigration.errorMessage" class="migration-error-text">{{ store.attachmentMigration.errorMessage }}</p>
        <a-button v-if="migrationResultVisible" type="primary" @click="migrationResultVisible = false">知道了</a-button>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconCamera, IconCheckCircle, IconClose, IconCloseCircle, IconFaceSmileFill, IconFile, IconFolder, IconLeft, IconLoading, IconMore, IconPlus, IconRight, IconSearch, IconSettings, IconUserGroup } from '@arco-design/web-vue/es/icon'
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
function storedSize(key: string, fallback: number, min: number, max: number) {
  const value = Number(localStorage.getItem(key))
  return Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : fallback
}
const friendsWidth = ref(storedSize('popchat.friendsWidth', 290, 220, 440))
const discoveryWidth = ref(storedSize('popchat.discoveryWidth', 320, 240, 460))
const composerHeight = ref(storedSize('popchat.composerHeight', 158, 120, 320))
const emojiOpen = ref(false)
const emojis = ['😀', '😂', '😍', '👍', '🎉', '🤔', '😭', '😎', '❤️', '👏', '🙏', '🔥']
const pendingImages = ref<string[]>([])
const messagePreviews = reactive<Record<string, string>>({})
const imageViewerOpen = ref(false)
const imageViewerIndex = ref(0)
const peerRemark = ref('')
const messageScroll = ref<HTMLElement>()
const newMessageCount = ref(0)
const userNearBottom = ref(true)
const migrationResultVisible = ref(false)
let resizeState: { kind: 'friends' | 'discover' | 'composer'; startX: number; startY: number; startValue: number } | undefined
let notificationAudio: AudioContext | undefined
let audioUnlocked = false

const activePeer = computed(() => store.activePeer)
const filteredFriends = computed(() => {
  const keyword = friendSearch.value.trim().toLowerCase()
  if (!keyword) return store.friends
  return store.friends.filter((peer) => `${peer.remark || ''} ${peer.nickname}`.toLowerCase().includes(keyword))
})
const totalUnreadCount = computed(() => store.conversations.reduce((total, conversation) => total + Math.max(0, conversation.unreadCount || 0), 0))
const activeMessages = computed(() => activePeer.value ? store.messages[`conv-${activePeer.value.deviceId}`] || [] : [])
const imageMessages = computed(() => activeMessages.value.filter((message) => message.kind === 'file' && message.attachmentMime?.startsWith('image/') && messagePreviews[message.messageId]))
const imageViewerSource = computed(() => messagePreviews[imageMessages.value[imageViewerIndex.value]?.messageId] || '')
const imageViewerName = computed(() => imageMessages.value[imageViewerIndex.value]?.attachmentName || '图片预览')
const migrationPercent = computed(() => store.attachmentMigration.total ? Math.min(100, Math.round(store.attachmentMigration.current / store.attachmentMigration.total * 100)) : 0)
const isDefaultPath = computed(() => !editProfile.fileSavePath || editProfile.fileSavePath === defaultAttachmentPath.value)
const defaultAttachmentPath = ref('')

function initials(value: string) { return (value || '?').trim().slice(0, 1).toUpperCase() }
function avatarStyle(value: string, image?: string) { if (image) return { backgroundImage: `url(${image})`, backgroundSize: 'cover', backgroundPosition: 'center' }; let hash = 0; for (const char of value || '?') hash = (hash * 31 + char.charCodeAt(0)) >>> 0; const hue = hash % 360; return { background: `linear-gradient(135deg, hsl(${hue} 80% 65%), hsl(${(hue + 42) % 360} 75% 45%))` } }
function formatTime(value: string) { if (!value) return ''; const date = new Date(value); const now = new Date(); const day = new Date(date.getFullYear(), date.getMonth(), date.getDate()); const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()); const diff = Math.round((today.getTime() - day.getTime()) / 86400000); const time = date.toLocaleTimeString('zh-CN', { hour: 'numeric', minute: '2-digit', hour12: true }); if (diff === 0) return `今天 ${time}`; if (diff === 1) return `昨天 ${time}`; if (diff === 2) return `前天 ${time}`; return `${date.getFullYear()}/${String(date.getMonth() + 1).padStart(2, '0')}/${String(date.getDate()).padStart(2, '0')} ${time}` }
function formatLastSeen(value: string) { return value ? formatTime(value) : '未知' }
function messageStatusText(status: string) { return ({ sending: '发送中', sent: '发送成功', delivered: '发送成功', read: '已读', queued: '发送失败', failed: '发送失败' } as Record<string, string>)[status] || status }
function unreadCount(deviceId: string) { return store.conversations.find((conversation) => conversation.peerDeviceId === deviceId)?.unreadCount || 0 }
function unreadLabel(count: number) { return count > 99 ? '99+' : String(count) }
function applyTheme(theme: string) { const dark = theme === 'dark' || (theme === 'system' && window.matchMedia?.('(prefers-color-scheme: dark)').matches); isDark.value = Boolean(dark); if (dark) { document.body.setAttribute('arco-theme', 'dark'); document.body.classList.add('popchat-dark') } else { document.body.removeAttribute('arco-theme'); document.body.classList.remove('popchat-dark') } }
async function load() { try { store.profile = await ChatService.GetProfile(); Object.assign(editProfile, store.profile); applyTheme(store.profile.theme); deviceInfo.value = await ChatService.GetDeviceInfo(); store.peers = await ChatService.ListPeers(); store.requests = await ChatService.ListFriendRequests(); store.conversations = await ChatService.ListConversations(); store.network = await ChatService.NetworkStatus() } catch (error: any) { Message.error(error?.message || '初始化聊天服务失败') } }
function selectPeer(peer: Peer) { store.selectPeer(peer.deviceId); store.clearConversationUnread(peer.deviceId); showPeerInfo.value = false; peerRemark.value = peer.remark || ''; newMessageCount.value = 0; ChatService.EnsureConversation(peer.deviceId).then((id) => ChatService.ListMessages(id).then((messages) => { store.messages[id] = messages; requestAnimationFrame(scrollToBottom); return ChatService.MarkConversationRead(peer.deviceId) }).catch(() => undefined)) }
function openSettings(tab: string) { if (store.attachmentMigration.active) return; section.value = 'settings'; settingsTab.value = tab }
async function saveProfile(showMessage = true) { try { const profile = await ChatService.UpdateProfile({ ...editProfile }); store.$patch({ profile: { ...store.profile, ...profile } }); Object.assign(editProfile, profile); applyTheme(profile.theme); if (showMessage) Message.success('设置已保存') } catch (error: any) { Message.error(error?.message || '保存失败') } }
function syncNickname() { editProfile.nickname = editProfile.nickname.trim() }
async function toggleStartup() { try { store.profile = await ChatService.SetLaunchAtStartup(editProfile.launchAtStartup); Object.assign(editProfile, store.profile) } catch (error: any) { editProfile.launchAtStartup = !editProfile.launchAtStartup; Message.error(error?.message || '设置失败') } }
function confirmMigration(path: string) { return new Promise<boolean>((resolve) => { Modal.confirm({ title: '迁移附件保存路径', content: '现有附件将自动迁移到新路径。迁移期间应用无法操作其他页面，请勿退出应用。是否继续？', okText: '开始迁移', cancelText: '取消', onOk: () => resolve(true), onCancel: () => resolve(false) }) }) }
async function migrateAttachmentPath(path: string) {
  if (!path || path === editProfile.fileSavePath || store.attachmentMigration.active) return
  if (!(await confirmMigration(path))) return
  store.attachmentMigration = { ...store.attachmentMigration, active: true, phase: 'preparing', sourceRoot: editProfile.fileSavePath, targetRoot: path, current: 0, total: 0, migrated: 0, skipped: 0, failed: 0, unclassified: 0, errorMessage: '' }
  try {
    const result = await ChatService.MigrateAttachmentStorage(path)
    if (result.completed) {
      const profile = { ...store.profile, fileSavePath: path }
      store.profile = profile
      Object.assign(editProfile, profile)
      migrationResultVisible.value = true
    }
  } catch (error: any) {
    store.attachmentMigration = { ...store.attachmentMigration, active: false, phase: 'failed', errorMessage: error?.message || '附件迁移失败' }
    migrationResultVisible.value = true
    Message.error(error?.message || '附件迁移失败')
  }
}
async function chooseDirectory() { const path = await ChatService.PickDirectory(); if (path) await migrateAttachmentPath(path) }
async function resetAttachmentPath() { if (defaultAttachmentPath.value) await migrateAttachmentPath(defaultAttachmentPath.value) }
async function chooseAvatar() { const path = await ChatService.PickFile(); if (path) { try { store.profile = await ChatService.SetAvatar(path); Object.assign(editProfile, store.profile); Message.success('头像已更新') } catch (error: any) { Message.error(error?.message || '头像更新失败') } } }
async function resetAvatar() { try { const theme = editProfile.theme; const profile = await ChatService.ResetAvatar(); const nextProfile = { ...profile, theme: theme || profile.theme }; store.$patch({ profile: { ...store.profile, ...nextProfile } }); Object.assign(editProfile, nextProfile); applyTheme(theme || profile.theme) } catch (error: any) { Message.error(error?.message || '恢复头像失败') } }
async function refreshPeers() { await ChatService.ScanPeers(); await new Promise((resolve) => setTimeout(resolve, 700)); store.peers = await ChatService.ListPeers(); store.network = await ChatService.NetworkStatus(); Message.success('已刷新局域网设备') }
async function addPeer() { if (!selectedDiscovery.value) return; try { await ChatService.SendFriendRequest(selectedDiscovery.value.deviceId, '你好，我想和你成为好友'); Message.success('好友申请已发送') } catch (error: any) { Message.error(error?.message || '发送申请失败') } }
async function acceptRequest() { if (!selectedRequest.value) return; await ChatService.AcceptFriendRequest(selectedRequest.value.requestId); Message.success('已添加好友'); selectedRequest.value = undefined; selectedDiscovery.value = undefined; store.requests = await ChatService.ListFriendRequests(); store.peers = await ChatService.ListPeers() }
async function rejectRequest() { if (!selectedRequest.value) return; await ChatService.RejectFriendRequest(selectedRequest.value.requestId); selectedRequest.value = undefined; store.requests = await ChatService.ListFriendRequests() }
async function sendMessage() { if (!activePeer.value || (!draft.value.trim() && !pendingImages.value.length)) return; try { if (draft.value.trim()) { const message = await ChatService.SendMessage(activePeer.value.deviceId, draft.value.trim()); store.handleEvent('chat:message', message) } for (const image of pendingImages.value) { const message = await ChatService.SendImage(activePeer.value.deviceId, image); store.handleEvent('chat:message', message) } draft.value = ''; pendingImages.value = []; requestAnimationFrame(scrollToBottom) } catch (error: any) { Message.error(error?.message || '发送失败') } }
function handlePaste(event: ClipboardEvent) { const files = Array.from(event.clipboardData?.files || []).filter((file) => file.type.startsWith('image/')); if (!files.length) return; event.preventDefault(); files.forEach((file) => { const reader = new FileReader(); reader.onload = () => { if (typeof reader.result === 'string') pendingImages.value.push(reader.result) }; reader.readAsDataURL(file) }) }
async function loadMessagePreview(message: any) { if (!message?.attachmentId || !message.attachmentMime?.startsWith('image/') || messagePreviews[message.messageId]) return; try { messagePreviews[message.messageId] = await ChatService.GetAttachmentPreview(message.attachmentId) } catch { /* pending remote image; clicking the image retries */ } }
async function openImage(message: any) { await loadMessagePreview(message); const index = imageMessages.value.findIndex((item) => item.messageId === message.messageId); if (index >= 0) { imageViewerIndex.value = index; imageViewerOpen.value = true } else { Message.warning('图片仍在接收或暂时无法读取') } }
function moveImage(direction: number) { const count = imageMessages.value.length; if (count) imageViewerIndex.value = (imageViewerIndex.value + direction + count) % count }
function handleImageViewerKey(event: KeyboardEvent) { if (!imageViewerOpen.value) return; if (event.key === 'ArrowLeft') { event.preventDefault(); moveImage(-1) } if (event.key === 'ArrowRight') { event.preventDefault(); moveImage(1) } if (event.key === 'Escape') imageViewerOpen.value = false }
function unlockNotificationAudio() {
  if (audioUnlocked) return
  try {
    const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext
    if (!AudioContextClass) return
    notificationAudio = notificationAudio || new AudioContextClass()
    void notificationAudio.resume()
    audioUnlocked = true
  } catch { /* browser audio may be unavailable */ }
}
function playNotificationTone() {
  if (!audioUnlocked || !notificationAudio) return
  try {
    const context = notificationAudio
    const oscillator = context.createOscillator()
    const gain = context.createGain()
    oscillator.type = 'sine'
    oscillator.frequency.value = 760
    gain.gain.setValueAtTime(.0001, context.currentTime)
    gain.gain.exponentialRampToValueAtTime(.08, context.currentTime + .015)
    gain.gain.exponentialRampToValueAtTime(.0001, context.currentTime + .16)
    oscillator.connect(gain).connect(context.destination)
    oscillator.start()
    oscillator.stop(context.currentTime + .17)
  } catch { /* browser audio may be unavailable */ }
}
async function pickFile() { pickedFile.value = await ChatService.PickFile(); if (pickedFile.value && activePeer.value) { try { const message = await ChatService.SendFile(activePeer.value.deviceId, pickedFile.value); store.handleEvent('chat:message', message); Message.success('文件已发送') } catch (error: any) { Message.error(error?.message || '文件发送失败') } finally { pickedFile.value = '' } } }
async function acceptAttachment(message: any) { try { await ChatService.AcceptAttachment(message.attachmentId); message.attachmentStatus = 'saved'; await loadMessagePreview(message); Message.success('文件已保存') } catch (error: any) { Message.error(error?.message || '接收文件失败') } }
async function rejectAttachment(message: any) { try { await ChatService.RejectAttachment(message.attachmentId); message.attachmentStatus = 'rejected' } catch (error: any) { Message.error(error?.message || '拒绝文件失败') } }
function formatBytes(value: number) { if (!value) return '未知大小'; if (value < 1024) return `${value} B`; if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB` }
function markActiveRead() { if (activePeer.value) { store.clearConversationUnread(activePeer.value.deviceId); void ChatService.MarkConversationRead(activePeer.value.deviceId) } }
function onMessageScroll() { const el = messageScroll.value; if (!el) return; userNearBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 80; if (userNearBottom.value) newMessageCount.value = 0 }
function scrollToBottom() { const el = messageScroll.value; if (el) { el.scrollTop = el.scrollHeight; userNearBottom.value = true; newMessageCount.value = 0 } }
function startResize(kind: 'friends' | 'discover' | 'composer', event: PointerEvent) { resizeState = { kind, startX: event.clientX, startY: event.clientY, startValue: kind === 'friends' ? friendsWidth.value : kind === 'discover' ? discoveryWidth.value : composerHeight.value }; window.addEventListener('pointermove', onResize); window.addEventListener('pointerup', stopResize) }
function onResize(event: PointerEvent) { if (!resizeState) return; if (resizeState.kind === 'friends') friendsWidth.value = Math.min(440, Math.max(220, resizeState.startValue + event.clientX - resizeState.startX)); else if (resizeState.kind === 'discover') discoveryWidth.value = Math.min(460, Math.max(240, resizeState.startValue + event.clientX - resizeState.startX)); else composerHeight.value = Math.min(320, Math.max(120, resizeState.startValue - event.clientY + resizeState.startY)) }
function stopResize() { if (!resizeState) return; localStorage.setItem('popchat.friendsWidth', String(friendsWidth.value)); localStorage.setItem('popchat.discoveryWidth', String(discoveryWidth.value)); localStorage.setItem('popchat.composerHeight', String(composerHeight.value)); resizeState = undefined; window.removeEventListener('pointermove', onResize); window.removeEventListener('pointerup', stopResize) }
async function savePeerRemark() { if (!activePeer.value || peerRemark.value === activePeer.value.remark) return; try { await ChatService.SetPeerRemark(activePeer.value.deviceId, peerRemark.value.trim()); const peer = store.peers.find((item) => item.deviceId === activePeer.value?.deviceId); if (peer) peer.remark = peerRemark.value.trim() } catch (error: any) { Message.error(error?.message || '备注保存失败') } }
async function runDiagnostic() { diagnostic.value = await ChatService.RunNetworkDiagnostic() }
function minimiseWindow() { Window.Minimise() }
async function toggleMaximise() { if (await Window.IsMaximised()) Window.UnMaximise(); else Window.Maximise() }
function closeWindow() { Window.Close() }
watch(() => store.profile, (value) => Object.assign(editProfile, value), { deep: true })
watch(() => activePeer.value, (peer) => { peerRemark.value = peer?.remark || '' })
watch(() => activeMessages.value, (messages) => messages.forEach(loadMessagePreview), { deep: true, immediate: true })
watch(() => store.lastMessageEvent, (message) => { if (!message || message.senderDeviceId === deviceInfo.value?.deviceId) return; if (message.conversationId === `conv-${activePeer.value?.deviceId}` && userNearBottom.value) requestAnimationFrame(() => { scrollToBottom(); markActiveRead() }); else newMessageCount.value += 1; playNotificationTone() })
watch(() => editProfile.theme, (value) => applyTheme(value))
watch(() => store.peers, () => { if (selectedDiscovery.value && !store.discovered.some((peer) => peer.deviceId === selectedDiscovery.value?.deviceId)) selectedDiscovery.value = undefined }, { deep: true })
onMounted(async () => {
  window.addEventListener('keydown', handleImageViewerKey)
  window.addEventListener('pointerdown', unlockNotificationAudio, { once: true })
  window.addEventListener('keydown', unlockNotificationAudio, { once: true })
  try { isMac.value = System.IsMac() } catch { isMac.value = false }
  try { defaultAttachmentPath.value = await ChatService.DefaultAttachmentPath() } catch { defaultAttachmentPath.value = '' }
  await load()
})
onBeforeUnmount(() => { window.removeEventListener('keydown', handleImageViewerKey); window.removeEventListener('pointerdown', unlockNotificationAudio); window.removeEventListener('keydown', unlockNotificationAudio); void notificationAudio?.close() })
</script>

<style scoped lang="less">
.chat-app { height: 100%; display: flex; overflow: hidden; background: #f5f7fb; color: #1d2129; }
.rail { width: 76px; flex: 0 0 76px; background: #17233c; display: flex; align-items: center; flex-direction: column; padding: 22px 10px 16px; box-sizing: border-box; color: #c9d4e8; }
.profile-button, .rail-nav button, .rail-settings { border: 0; background: transparent; color: inherit; cursor: pointer; border-radius: 14px; }
.profile-button { padding: 0; margin-bottom: 28px; }.rail-nav { display: flex; flex-direction: column; gap: 10px; align-items: center; flex: 1; }.rail-nav button, .rail-settings { width: 54px; height: 58px; position: relative; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; }.rail-nav button span, .rail-settings span, .rail-nav button svg, .rail-settings svg { font-size: 22px; width: 22px; height: 22px; line-height: 22px; }.rail-nav small, .rail-settings small { font-size: 11px; }.rail-nav button.active, .rail-settings.active { color: #fff; background: #2e5bba; }.rail-nav b { position: absolute; top: 2px; right: 5px; min-width: 16px; height: 16px; border-radius: 9px; background: #f53f3f; color: #fff; font-size: 10px; line-height: 16px; }
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

.migration-lock { position: fixed; inset: 0; z-index: 1000; display: flex; align-items: center; justify-content: center; background: rgba(12, 18, 28, .62); backdrop-filter: blur(3px); cursor: wait; }
.migration-card { width: min(460px, calc(100vw - 48px)); padding: 30px 34px; border: 1px solid var(--line); border-radius: 14px; background: var(--surface-1); color: var(--text); box-shadow: 0 24px 80px rgba(0, 0, 0, .28); text-align: center; cursor: default; }
.migration-card h3 { margin: 12px 0 8px; font-size: 20px; }
.migration-card p { margin: 10px 0; color: var(--muted); font-size: 13px; }
.migration-path, .migration-file { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.migration-spinner { color: var(--accent); font-size: 34px; animation: migration-spin 1s linear infinite; }
.migration-success { color: #00b42a; font-size: 34px; }
.migration-error { color: #f53f3f; font-size: 34px; }
.migration-error-text { color: #f53f3f !important; }
.is-locked { pointer-events: none; }
.avatar-upload { position: relative; border: 0; padding: 0; background: transparent; cursor: pointer; border-radius: 28px; }
.avatar-camera { position: absolute; right: 2px; bottom: 2px; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; border-radius: 50%; background: var(--accent); color: #fff; opacity: 0; transition: opacity .16s ease; }
.avatar-upload:hover .avatar-camera, .avatar-upload:focus-visible .avatar-camera { opacity: 1; }
.path-actions { display: flex !important; flex-direction: row !important; gap: 8px; }
@keyframes migration-spin { to { transform: rotate(360deg); } }

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
  .device-card { padding: 24px 22px 20px; }
  .device-card .device-fields { grid-template-columns: minmax(0, 1fr); }
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

/* Device identity is a content card now that the profile avatar is no longer part of this page. */
.device-card { display: flex; flex-direction: column; gap: 24px; min-height: 0; padding: 28px 32px 22px; }
.device-card-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
.device-eyebrow { display: block; margin-bottom: 7px; color: var(--accent); font-size: 11px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.device-card-head h3 { margin: 0 0 6px; font-size: 19px; }
.device-card-head p { margin: 0; color: var(--muted); font-size: 12px; }
.device-badge { display: inline-flex; align-items: center; gap: 7px; flex: 0 0 auto; padding: 7px 11px; border: 1px solid color-mix(in srgb, var(--accent) 28%, var(--line)); border-radius: 999px; color: var(--accent); font-size: 12px; font-weight: 600; }
.device-badge i { width: 7px; height: 7px; border-radius: 50%; background: #00b42a; box-shadow: 0 0 0 3px color-mix(in srgb, #00b42a 16%, transparent); }
.device-card .device-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; max-width: none; margin: 0; padding: 0; }
.device-card .device-fields label { min-height: 76px; box-sizing: border-box; justify-content: center; gap: 8px; padding: 14px 16px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface-2); }
.device-field-label { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 12px; }
.device-field-icon { display: inline-flex; align-items: center; justify-content: center; width: 22px; height: 22px; border-radius: 7px; background: var(--surface-4); color: var(--accent); font-size: 11px; font-style: normal; font-weight: 700; }
.device-card .device-fields strong { padding-left: 30px; color: var(--text); font-size: 13px; font-weight: 600; }
.device-card .device-fields strong.mono { font-size: 11px; line-height: 1.45; }
.device-card-foot { padding-top: 2px; color: var(--muted); font-size: 11px; }

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
:global(#app) { background: var(--app-bg, #0f1115); }
.chat-app { position: relative; border-radius: 0; overflow: hidden; }
.chat-app.is-mac { border-radius: 16px; }
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

/* Chat interaction additions. */
.conversation { position: relative; }
.conversation-head { height: 58px; flex-basis: 58px; padding: 0 18px; }
.head-peer strong { font-size: 14px; }
.head-peer span { font-size: 11px; }
.message-line { align-items: flex-end; gap: 8px; }
.message-avatar { width: 32px; height: 32px; border-radius: 10px; font-size: 12px; }
.message-status { display: inline-flex; width: 52px; height: 17px; margin-left: 6px; align-items: center; justify-content: center; border-radius: 4px; background: rgba(255, 255, 255, .2); font-size: 10px; vertical-align: middle; }
.vertical-resizer { width: 5px; flex: 0 0 5px; margin-left: -3px; margin-right: -2px; cursor: col-resize; position: relative; z-index: 6; }
.vertical-resizer:hover::after, .vertical-resizer:active::after { content: ''; position: absolute; inset: 0 1px; background: var(--accent); }
.horizontal-resizer { height: 5px; flex: 0 0 5px; margin-top: -3px; cursor: row-resize; position: relative; z-index: 5; }
.horizontal-resizer:hover::after, .horizontal-resizer:active::after { content: ''; position: absolute; inset: 1px 0; background: var(--accent); }
.composer { box-sizing: border-box; flex: 0 0 auto; min-height: 120px; position: relative; overflow: auto; }
.emoji-panel { display: flex; flex-wrap: wrap; gap: 4px; padding: 7px 0; }
.emoji-panel button { width: 28px; height: 28px; border: 0; background: transparent; cursor: pointer; font-size: 18px; }
.pending-images { display: flex; flex-wrap: wrap; gap: 8px; padding: 6px 0; }
.pending-image { width: 58px; height: 58px; position: relative; border-radius: 6px; overflow: hidden; border: 1px solid var(--line); }
.pending-image img { width: 100%; height: 100%; object-fit: cover; }
.pending-image button { position: absolute; top: 2px; right: 2px; display: flex; border: 0; border-radius: 50%; padding: 2px; background: rgba(0, 0, 0, .64); color: #fff; cursor: pointer; }
.image-message { display: block; max-width: 270px; padding: 0; border: 0; background: transparent; cursor: zoom-in; overflow: hidden; border-radius: 8px; color: inherit; }
.image-message img { display: block; width: 100%; max-height: 220px; object-fit: cover; }
.new-message-button { position: absolute; right: 22px; top: -41px; z-index: 8; border: 0; border-radius: 5px; padding: 7px 10px; background: var(--accent); color: #fff; font-size: 12px; cursor: pointer; box-shadow: var(--shadow); }
.info-overlay { position: absolute; z-index: 12; top: 58px; right: 12px; bottom: 12px; width: min(320px, calc(100% - 24px)); overflow: auto; box-sizing: border-box; border: 1px solid var(--line); border-radius: 8px; box-shadow: var(--shadow); }
.info-fields input { width: 100%; box-sizing: border-box; padding: 7px 8px; border: 1px solid var(--line); border-radius: 4px; background: var(--surface-2); color: var(--text); }
.image-viewer { display: flex; min-height: 50vh; align-items: center; justify-content: center; gap: 12px; }
.image-viewer img { max-width: calc(100% - 96px); max-height: 70vh; object-fit: contain; }
.image-viewer button { width: 36px; height: 36px; border: 0; border-radius: 50%; background: var(--surface-3); color: var(--text); cursor: pointer; }
.image-thumbnails { display: flex; max-width: 100%; gap: 8px; overflow-x: auto; padding: 14px 4px 0; }
.image-thumbnails button { width: 56px; height: 56px; flex: 0 0 56px; padding: 0; overflow: hidden; border: 2px solid transparent; border-radius: 5px; background: transparent; cursor: pointer; }
.image-thumbnails button.active { border-color: var(--accent); }
.image-thumbnails img { width: 100%; height: 100%; object-fit: cover; }
.pane-title { padding: 18px 16px 12px; border-bottom: 1px solid var(--line); background: var(--surface-2); }
.search { margin-top: 12px; margin-bottom: 12px; }
.group-title { align-items: center; }
.group-title > span { display: inline-flex; align-items: center; min-height: 22px; }

/* Chat density and interaction polish. */
.conversation-head { height: 58px; flex-basis: 58px; padding: 0 18px; }
.head-peer { min-width: 0; gap: 9px; }
.head-peer strong { max-width: min(45vw, 360px); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.head-peer > .head-status { display: inline-flex; align-items: center; gap: 5px; min-width: 0; color: var(--muted); white-space: nowrap; }
.head-status i { width: 7px; height: 7px; flex: 0 0 7px; border-radius: 50%; background: var(--muted); }
.head-status i.online { background: #00b42a; }
.message-scroll { padding: 18px clamp(18px, 6vw, 72px); }
.message-line { margin: 8px 0; gap: 8px; }
.message-bubble { max-width: min(72%, 680px); padding: 9px 12px; border-radius: 14px 14px 14px 5px; line-height: 1.45; }
.message-line.mine .message-bubble { border-radius: 14px 14px 5px 14px; }
.message-avatar { width: 32px; height: 32px; border-radius: 10px; }
.avatar-button { padding: 0; border: 0; font: inherit; cursor: pointer; transition: transform .16s ease, filter .16s ease; }
.avatar-button:hover { transform: translateY(-1px); filter: brightness(1.06); }
.peer-row { position: relative; padding-right: 42px; }
.peer-copy { flex: 1; }
.unread-badge { position: absolute; right: 10px; top: 50%; min-width: 18px; height: 18px; padding: 0 5px; border-radius: 10px; transform: translateY(-50%); background: #f53f3f; color: #fff !important; font-size: 10px !important; font-weight: 700 !important; line-height: 18px; text-align: center; }
.rail-unread-badge { right: 2px !important; top: 0 !important; }
.info-overlay { top: 58px; bottom: calc(var(--composer-height, 158px) + 12px); width: min(320px, calc(100% - 24px)); padding: 16px; border-radius: 14px; }
.info-profile { padding: 20px 0 18px; }
.composer { display: flex; flex-direction: column; gap: 2px; padding: 8px 18px 10px; overflow: visible; }
.composer-tools { height: 24px; flex: 0 0 24px; }
.composer textarea { flex: 1 1 auto; min-height: 0; overflow: auto; padding: 5px 0; }
.composer-foot { min-height: 28px; flex: 0 0 28px; gap: 12px; }
.composer-foot > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.composer-foot :deep(.arco-btn) { flex: 0 0 auto; min-width: 72px; }
.emoji-panel { position: absolute; left: 18px; bottom: calc(100% - 2px); z-index: 15; width: 174px; padding: 8px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-1); box-shadow: var(--shadow); }
.pending-images { flex: 0 0 auto; max-height: 50px; overflow-x: auto; flex-wrap: nowrap; padding: 3px 0; }
.pending-image { width: 44px; height: 44px; flex: 0 0 44px; }

@media (max-width: 760px) {
  .vertical-resizer { display: none; }
  .list-pane { width: 220px !important; flex-basis: 220px !important; }
  .message-scroll { padding-left: 18px; padding-right: 18px; }
}

</style>
