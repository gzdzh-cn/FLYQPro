export interface Profile {
  nickname: string
  avatarPath: string
  discoverable: boolean
  autoSave: boolean
  fileSavePath: string
  theme: string
  launchAtStartup: boolean
}

export interface Peer {
  deviceId: string
  nickname: string
  avatarPath: string
  platform: string
  osVersion: string
  ip: string
  port: number
  publicKeyPem: string
  certificateFingerprint: string
  relation: string
  remark: string
  online: boolean
  lastSeen: string
}

export interface FriendRequest {
  requestId: string
  deviceId: string
  nickname: string
  message: string
  status: string
  createdAt: string
  attachmentId?: string
  attachmentName?: string
  attachmentSize?: number
  attachmentMime?: string
  attachmentStatus?: string
}

export interface Conversation {
  conversationId: string
  peerDeviceId: string
  lastMessage: string
  lastMessageAt: string
  unreadCount: number
  pinned: boolean
}

export interface Message {
  messageId: string
  conversationId: string
  senderDeviceId: string
  kind: string
  content: string
  status: string
  createdAt: string
}

export interface NetworkStatus {
  status: string
  interfaces: string[]
  localIps: string[]
  discoveryPort: number
  chatPort: number
  peerCount: number
  onlineCount: number
  lastScanAt: string
  lastError?: string
}
