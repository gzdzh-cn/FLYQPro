import { defineStore } from 'pinia'
import type { AttachmentMigrationProgress, Conversation, FriendRequest, Message, NetworkStatus, Peer, Profile } from './types'

export const useChatStore = defineStore('chat', {
  state: () => ({
    profile: { nickname: '新用户', avatarPath: '', discoverable: false, autoSave: false, fileSavePath: '', theme: 'system', launchAtStartup: false } as Profile,
    peers: [] as Peer[],
    conversations: [] as Conversation[],
    requests: [] as FriendRequest[],
    messages: {} as Record<string, Message[]>,
    network: { status: 'unknown', interfaces: [], localIps: [], discoveryPort: 39190, chatPort: 0, peerCount: 0, onlineCount: 0, lastScanAt: '' } as NetworkStatus,
    activeSection: 'friends',
    activePeerId: '',
    lastMessageEvent: null as Message | null,
    attachmentMigration: { active: false, phase: '', sourceRoot: '', targetRoot: '', current: 0, total: 0, fileName: '', peerDeviceId: '', migrated: 0, skipped: 0, failed: 0, unclassified: 0, errorMessage: '' } as AttachmentMigrationProgress & { active: boolean },
  }),
  getters: {
    friends: (state) => state.peers.filter((peer) => peer.relation === 'friend'),
    discovered: (state) => state.peers.filter((peer) => peer.relation !== 'friend'),
    activePeer: (state) => state.peers.find((peer) => peer.deviceId === state.activePeerId),
  },
  actions: {
    handleEvent(name: string, value: any) {
      if (name === 'chat:profile-updated') this.profile = { ...this.profile, ...value }
      if (name === 'chat:network-status') this.network = value
      if (name === 'chat:peer-updated') this.peers = value || []
      if (name === 'chat:friend-request') this.requests = [value, ...this.requests.filter((item) => item.requestId !== value.requestId)]
      if (name === 'chat:friend-request-updated') this.requests = this.requests.filter((item) => item.requestId !== value.requestId)
      if (name === 'chat:message-status') {
        Object.values(this.messages).forEach((list) => list.forEach((item) => { if (item.messageId === value?.messageId) item.status = value.status }))
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
          const conversation = this.conversations.find((item) => item.conversationId === value.conversationId || item.peerDeviceId === peerDeviceId)
          if (conversation) {
            conversation.lastMessage = value.content || value.attachmentName || ''
            conversation.lastMessageAt = value.createdAt || conversation.lastMessageAt
            conversation.unreadCount = this.activePeerId === peerDeviceId ? 0 : (conversation.unreadCount || 0) + 1
          } else if (peerDeviceId) {
            this.conversations = [...this.conversations, {
              conversationId: value.conversationId,
              peerDeviceId,
              lastMessage: value.content || value.attachmentName || '',
              lastMessageAt: value.createdAt || '',
              unreadCount: this.activePeerId === peerDeviceId ? 0 : 1,
              pinned: false,
            }]
          }
          if (name === 'chat:message') this.lastMessageEvent = value
        } else {
          const next = list.slice()
          next[index] = { ...next[index], ...value }
          this.messages[value.conversationId] = next
        }
      }
    },
    selectPeer(deviceId: string) { this.activePeerId = deviceId },
    clearConversationUnread(deviceId: string) {
      const conversation = this.conversations.find((item) => item.peerDeviceId === deviceId)
      if (conversation) conversation.unreadCount = 0
    },
    setSection(section: string) { this.activeSection = section },
  },
})
