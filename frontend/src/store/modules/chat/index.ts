import { defineStore } from 'pinia'
import type { AttachmentMigrationProgress, Conversation, FriendRequest, Message, NetworkStatus, Peer, Profile, TransferProgress } from './types'

const requestInProgress = new Set(['queued', 'sent', 'pending'])

function requestTime(request: FriendRequest) {
  const updated = Date.parse(request.updatedAt || '')
  const created = Date.parse(request.createdAt || '')
  return Number.isFinite(updated) ? updated : Number.isFinite(created) ? created : 0
}

function requestStatusRank(status: string) {
  switch (status) {
    case 'accepted': return 5
    case 'rejected': return 4
    case 'failed': return 3
    case 'superseded': return 2
    default: return 1
  }
}

const discoveryNameCollator = new Intl.Collator('zh-Hans-u-co-pinyin', { sensitivity: 'base', numeric: true })

function peerDisplayName(peer: Peer) {
  return (peer.remark || peer.nickname || '').trim()
}

/** Keep history in state, but expose one current request projection per device. */
function latestRequestsByDevice(requests: FriendRequest[]) {
  const grouped = new Map<string, FriendRequest[]>()
  requests.forEach((request) => {
    const key = request.deviceId || request.requestId
    const list = grouped.get(key) || []
    list.push(request)
    grouped.set(key, list)
  })
  return [...grouped.values()]
    .map((list) => {
      const active = list.filter((request) => requestInProgress.has(request.status))
      const incoming = active.filter((request) => request.direction !== 'sent')
      const outgoing = active.filter((request) => request.direction === 'sent')
      const mutual = incoming.length > 0 && outgoing.length > 0
      const candidates = mutual ? incoming : active.length ? active : list
      const selected = [...candidates].sort((left, right) => {
        const timeDelta = requestTime(right) - requestTime(left)
        return timeDelta || requestStatusRank(right.status) - requestStatusRank(left.status) || right.requestId.localeCompare(left.requestId)
      })[0]
      return selected && mutual ? { ...selected, direction: 'mutual' } : selected
    })
    .filter(Boolean) as FriendRequest[]
}

export const useChatStore = defineStore('chat', {
  state: () => ({
    profile: { nickname: '新用户', avatarPath: '', discoverable: false, autoSave: false, fileSavePath: '', theme: 'system', launchAtStartup: false, sharedRootPath: '', sharedEnabled: false, sharedDriveMultiWindow: false, showHiddenFiles: false, directoryOpenMode: 'double' } as Profile,
    deviceId: '',
    peers: [] as Peer[],
    // A hide action is local UI state as well as a persisted peer flag.  Keep
    // it separately so an already queued peer-updated event cannot resurrect
    // the row with an older visibleInFriends=true snapshot.
    hiddenFriendIds: {} as Record<string, boolean>,
    conversations: [] as Conversation[],
    requests: [] as FriendRequest[],
    messages: {} as Record<string, Message[]>,
    network: { status: 'unknown', interfaces: [], localIps: [], discoveryPort: 39190, chatPort: 0, peerCount: 0, onlineCount: 0, lastScanAt: '' } as NetworkStatus,
    activeSection: 'friends',
    activePeerId: '',
    lastMessageEvent: null as Message | null,
    transferProgress: {} as Record<string, TransferProgress>,
    transferHistory: {} as Record<string, TransferProgress>,
    attachmentMigration: { active: false, phase: '', sourceRoot: '', targetRoot: '', current: 0, total: 0, fileName: '', peerDeviceId: '', migrated: 0, skipped: 0, failed: 0, unclassified: 0, errorMessage: '' } as AttachmentMigrationProgress & { active: boolean },
  }),
  getters: {
    // A removal tombstone is retained for identity/synchronization, but it is
    // not a contact. It must stay discoverable as a stranger so the user can
    // send a fresh request, and must never reappear in 通讯录 after a page
    // reload merely because the peer was rediscovered.
    contacts: (state) => state.peers.filter((peer) => peer.relation === 'friend' && peer.friendshipState !== 'removed'),
    friends: (state) => state.peers.filter((peer) => !state.hiddenFriendIds[peer.deviceId] && peer.visibleInFriends !== false && (peer.relation === 'friend' || peer.friendshipState === 'removed')),
    discovered: (state) => state.peers
      .filter((peer) => peer.discoveryVisible && peer.online)
      .sort((left, right) => discoveryNameCollator.compare(peerDisplayName(left), peerDisplayName(right)) || left.deviceId.localeCompare(right.deviceId)),
    visibleRequests: (state) => latestRequestsByDevice(state.requests),
    pendingRequests: (state) => latestRequestsByDevice(state.requests).filter((request) => request.status === 'pending' && request.direction !== 'sent' && !state.peers.some((peer) => peer.deviceId === request.deviceId && peer.relation === 'friend' && peer.friendshipState !== 'removed')),
    activePeer: (state) => state.peers.find((peer) => peer.deviceId === state.activePeerId),
  },
  actions: {
    handleEvent(name: string, value: any) {
      if (name === 'chat:profile-updated') this.profile = { ...this.profile, ...value }
      if (name === 'chat:network-status') this.network = value
      if (name === 'chat:peer-updated') {
        this.peers = (value || []).map((peer: Peer) => this.hiddenFriendIds[peer.deviceId] && peer.relation === 'friend'
          ? { ...peer, visibleInFriends: false }
          : peer)
      }
      if (name === 'chat:friend-request' && value?.requestId) {
        // Keep the raw lifecycle by request id. The getter projects it to one
        // row per device, preserving both directions for mutual requests.
        this.requests = [value, ...this.requests.filter((item) => item.requestId !== value.requestId)]
      }
      if (name === 'chat:friend-request-updated') {
        if (value?.status === 'accepted' && value?.deviceId) delete this.hiddenFriendIds[value.deviceId]
        const index = this.requests.findIndex((item) => item.requestId === value?.requestId)
        if (index < 0 && value?.requestId) {
          // Supersede events may arrive before the initial list refresh. Keep
          // them as history; projection hides them when an active row exists.
          this.requests = [value, ...this.requests]
        }
        else if (index >= 0) this.requests[index] = { ...this.requests[index], ...value }
      }
      if (name === 'chat:message-status') {
        Object.values(this.messages).forEach((list) => list.forEach((item) => { if (item.messageId === value?.messageId) item.status = value.status }))
        return
      }
      if (name === 'chat:transfer-progress') {
        const progress = value as TransferProgress
        if (progress?.attachmentId) {
          const snapshot = { ...this.transferHistory[progress.attachmentId], ...this.transferProgress[progress.attachmentId], ...progress }
          if (['completed', 'canceled', 'rejected', 'failed'].includes(progress.phase)) {
            this.transferHistory[progress.attachmentId] = snapshot
            delete this.transferProgress[progress.attachmentId]
            const historyIds = Object.keys(this.transferHistory)
            while (historyIds.length > 100) delete this.transferHistory[historyIds.shift() as string]
          } else this.transferProgress[progress.attachmentId] = snapshot
        }
        return
      }
      if (name === 'chat:attachment' && !value?.conversationId) {
        Object.values(this.messages).forEach((list) => list.forEach((item) => {
          if (item.attachmentId === value.attachmentId) {
            item.attachmentStatus = value.status
            if (value.localPath) item.attachmentPath = value.localPath
          }
        }))
        return
      }
      if (name === 'chat:attachment-migration') {
        const progress = value as AttachmentMigrationProgress
        this.attachmentMigration = { ...this.attachmentMigration, ...progress, active: !['completed', 'failed'].includes(progress.phase) }
        if (progress.phase === 'completed') this.attachmentMigration.active = false
        if (progress.phase === 'failed') this.attachmentMigration.active = false
        return
      }
      if (name === 'chat:message' || name === 'chat:attachment') {
        if (!value?.conversationId) return
        const list = this.messages[value.conversationId] || []
        const index = list.findIndex((item) => item.messageId === value.messageId)
        if (index < 0) {
          this.messages[value.conversationId] = [...list, value]
          const peerDeviceId = value.conversationId.startsWith('conv-') ? value.conversationId.slice(5) : ''
          const incoming = value.senderDeviceId !== this.deviceId
          // Only a newly received text message is allowed to restore a
          // hidden friend.  Attachments and status events must not do so.
          if (name === 'chat:message' && incoming && value.kind === 'text') {
            delete this.hiddenFriendIds[peerDeviceId]
            this.peers = this.peers.map((peer) => peer.deviceId === peerDeviceId ? { ...peer, visibleInFriends: true } : peer)
          }
          const conversation = this.conversations.find((item) => item.conversationId === value.conversationId || item.peerDeviceId === peerDeviceId)
          if (conversation) {
            conversation.lastMessage = value.content || value.attachmentName || ''
            conversation.lastMessageAt = value.createdAt || conversation.lastMessageAt
            if (incoming) conversation.unreadCount = (conversation.unreadCount || 0) + 1
          } else if (peerDeviceId) {
            this.conversations = [...this.conversations, {
              conversationId: value.conversationId,
              peerDeviceId,
              lastMessage: value.content || value.attachmentName || '',
              lastMessageAt: value.createdAt || '',
              unreadCount: incoming ? 1 : 0,
              pinned: false,
            }]
          }
          if (name === 'chat:message') this.lastMessageEvent = value
        } else {
          const next = list.slice()
          // Completion/status events can arrive after the asynchronous image
          // thumbnail event. Do not let an older payload with an empty preview
          // erase the thumbnail already shown in the conversation.
          next[index] = {
            ...next[index],
            ...value,
            attachmentThumbnail: value.attachmentThumbnail || next[index].attachmentThumbnail,
            attachmentThumbnailMime: value.attachmentThumbnailMime || next[index].attachmentThumbnailMime,
          }
          this.messages[value.conversationId] = next
        }
      }
    },
    selectPeer(deviceId: string) { this.activePeerId = deviceId },
    setDeviceId(deviceId: string) { this.deviceId = deviceId },
    clearConversationUnread(deviceId: string) {
      const conversation = this.conversations.find((item) => item.peerDeviceId === deviceId)
      if (conversation) conversation.unreadCount = 0
    },
    clearConversationLocal(deviceId: string): Message[] {
      const conversationId = `conv-${deviceId}`
      const removed = this.messages[conversationId] || []
      delete this.messages[conversationId]
      this.conversations = this.conversations.filter((item) => item.peerDeviceId !== deviceId)
      return removed
    },
    hideFriendLocally(deviceId: string) { this.hiddenFriendIds[deviceId] = true },
    clearHiddenFriend(deviceId: string) {
      delete this.hiddenFriendIds[deviceId]
      this.peers = this.peers.map((peer) => peer.deviceId === deviceId ? { ...peer, visibleInFriends: true } : peer)
    },
    setSection(section: string) { this.activeSection = section },
  },
})
