export interface Profile {
  nickname: string
  avatarPath: string
  avatarData?: string
  avatarHash?: string
  avatarVersion?: number
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
  avatarData?: string
  avatarHash?: string
  avatarVersion?: number
  platform: string
  osVersion: string
  ip: string
  port: number
  publicKeyPem: string
  certificateFingerprint: string
  relation: string
  remark: string
  protocolName?: string
  protocolMajor?: number
  discoveryMagic?: string
  capabilities?: string[]
  online: boolean
  lastSeen: string
}

export interface FriendRequest {
  requestId: string
  deviceId: string
  nickname: string
  message: string
  status: string
  direction: string
  createdAt: string
  acceptedAt?: string
  updatedAt: string
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
  attachmentId?: string
  attachmentName?: string
  attachmentSize?: number
  attachmentMime?: string
  attachmentStatus?: string
  attachmentPath?: string
}

export interface TransferProgress {
  messageId?: string
  attachmentId: string
  peerDeviceId?: string
  transferred: number
  total: number
  percent: number
  sent?: number
  received?: number
  remoteReceived?: number
  direction: 'send' | 'receive' | 'remote-receive'
  phase: 'transferring' | 'receiving' | 'completed' | 'failed' | string
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

export interface AttachmentMigrationProgress {
  phase: string
  sourceRoot: string
  targetRoot: string
  current: number
  total: number
  fileName?: string
  peerDeviceId?: string
  migrated: number
  skipped: number
  failed: number
  unclassified: number
  errorMessage?: string
}
