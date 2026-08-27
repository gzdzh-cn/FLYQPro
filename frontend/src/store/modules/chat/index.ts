import { defineStore } from 'pinia'
import type { AttachmentMigrationProgress, Conversation, FriendRequest, Message, NetworkStatus, Peer, Profile, TransferProgress } from './types'

export const useChatStore = defineStore('chat', {
  state: () => ({
    profile: { nickname: '新用户', avatarPath: '', discoverable: false, autoSave: false, fileSavePath: '', theme: 'system', launchAtStartup: false, sharedRootPath: '', sharedEnabled: false, sharedDriveMultiWindow: true } as Profile,
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
    attachmentMigration: { active: false, phase: '', sourceRoot: '', targetRoot: '', current: 0, total: 0, fileName: '', peerDeviceId: '', migrated: 0, skipped: 0, failed: 0, unclassified: 0, errorMessage: '' } as AttachmentMigrationProgress & { active: boolean },
  }),
  getters: {
    // A removal tombstone is retained for identity/synchronization, but it is
    // not a contact. It must stay discoverable as a stranger so the user can
    // send a fresh request, and must never reappear in 通讯录 after a page
    // reload merely because the peer was rediscovered.
    contacts: (state) => state.peers.filter((peer) => peer.relation === 'friend' && peer.friendshipState !== 'removed'),
    friends: (state) => state.peers.filter((peer) => !state.hiddenFriendIds[peer.deviceId] && peer.visibleInFriends !== false && (peer.relation === 'friend' || peer.friendshipState === 'removed')),
    discovered: (state) => state.peers.filter((peer) => peer.discoveryVisible && peer.online && peer.relation !== 'friend'),
    pendingRequests: (state) => state.requests.filter((request) => request.status === 'pending' && request.direction !== 'sent'),
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
        // The database retains the complete lifecycle, while the friends UI
        // shows one current request per device. A new request must replace the
        // old visual row even when its request id is different.
        this.requests = [value, ...this.requests.filter((item) => item.requestId !== value.requestId && item.deviceId !== value.deviceId)]
      }
      if (name === 'chat:friend-request-updated') {
        if (value?.status === 'accepted' && value?.deviceId) delete this.hiddenFriendIds[value.deviceId]
        const index = this.requests.findIndex((item) => item.requestId === value?.requestId)
        if (index < 0 && value?.requestId) {
          const current = this.requests.find((item) => item.deviceId === value.deviceId)
          const valueTime = Date.parse(value.updatedAt || value.createdAt || '')
          const currentTime = Date.parse(current?.updatedAt || current?.createdAt || '')
          if (!current || !Number.isFinite(currentTime) || (Number.isFinite(valueTime) && valueTime >= currentTime)) {
            this.requests = [value, ...this.requests.filter((item) => item.deviceId !== value.deviceId)]
          }
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
          if (['completed', 'canceled', 'rejected', 'failed'].includes(progress.phase)) delete this.transferProgress[progress.attachmentId]
          else this.transferProgress[progress.attachmentId] = {
            ...this.transferProgress[progress.attachmentId],
            ...progress,
          }
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
