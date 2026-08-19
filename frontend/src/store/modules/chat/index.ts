import { defineStore } from 'pinia'
import type { Conversation, FriendRequest, Message, NetworkStatus, Peer, Profile } from './types'

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
        Object.values(this.messages).forEach((list) => list.forEach((item) => { if (item.attachmentId === value.attachmentId) item.attachmentStatus = value.status }))
        return
      }
      if (name === 'chat:message' || name === 'chat:attachment') {
        if (!value?.conversationId) return
        const list = this.messages[value.conversationId] || []
        if (!list.some((item) => item.messageId === value.messageId)) this.messages[value.conversationId] = [...list, value]
      }
    },
    selectPeer(deviceId: string) { this.activePeerId = deviceId },
    setSection(section: string) { this.activeSection = section },
  },
})
