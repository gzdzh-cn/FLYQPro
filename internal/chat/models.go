package chat

import "time"

const (
	ProtocolName    = "POPChat"
	ProtocolMajor   = 1
	ProtocolMinor   = 0
	DiscoveryPort   = 39190
	DiscoveryMagic  = "POPCHAT_DISCOVERY_V1"
	PeerRelation    = "friend"
	DiscoveredState = "discovered"
)

type Profile struct {
	Nickname        string `json:"nickname"`
	AvatarPath      string `json:"avatarPath"`
	AvatarData      string `json:"avatarData,omitempty"`
	Discoverable    bool   `json:"discoverable"`
	AutoSave        bool   `json:"autoSave"`
	FileSavePath    string `json:"fileSavePath"`
	Theme           string `json:"theme"`
	LaunchAtStartup bool   `json:"launchAtStartup"`
}

type DeviceInfo struct {
	Platform               string `json:"platform"`
	OSVersion              string `json:"osVersion"`
	DeviceID               string `json:"deviceId"`
	PublicKeyPEM           string `json:"publicKeyPem"`
	CertificateFingerprint string `json:"certificateFingerprint"`
	IP                     string `json:"ip"`
	Port                   int    `json:"port"`
}

type Peer struct {
	DeviceID               string    `json:"deviceId"`
	Nickname               string    `json:"nickname"`
	AvatarPath             string    `json:"avatarPath"`
	Platform               string    `json:"platform"`
	OSVersion              string    `json:"osVersion"`
	IP                     string    `json:"ip"`
	Port                   int       `json:"port"`
	PublicKeyPEM           string    `json:"publicKeyPem"`
	CertificateFingerprint string    `json:"certificateFingerprint"`
	Relation               string    `json:"relation"`
	Remark                 string    `json:"remark"`
	Online                 bool      `json:"online"`
	LastSeen               string    `json:"lastSeen"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type FriendRequest struct {
	RequestID string `json:"requestId"`
	DeviceID  string `json:"deviceId"`
	Nickname  string `json:"nickname"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Conversation struct {
	ConversationID string `json:"conversationId"`
	PeerDeviceID   string `json:"peerDeviceId"`
	LastMessage    string `json:"lastMessage"`
	LastMessageAt  string `json:"lastMessageAt"`
	UnreadCount    int    `json:"unreadCount"`
	Pinned         bool   `json:"pinned"`
}

type Message struct {
	MessageID        string `json:"messageId"`
	ConversationID   string `json:"conversationId"`
	SenderDeviceID   string `json:"senderDeviceId"`
	Kind             string `json:"kind"`
	Content          string `json:"content"`
	Status           string `json:"status"`
	CreatedAt        string `json:"createdAt"`
	AttachmentID     string `json:"attachmentId,omitempty"`
	AttachmentName   string `json:"attachmentName,omitempty"`
	AttachmentSize   int64  `json:"attachmentSize,omitempty"`
	AttachmentMime   string `json:"attachmentMime,omitempty"`
	AttachmentStatus string `json:"attachmentStatus,omitempty"`
}

type Attachment struct {
	AttachmentID string `json:"attachmentId"`
	MessageID    string `json:"messageId"`
	FileName     string `json:"fileName"`
	MimeType     string `json:"mimeType"`
	FileSize     int64  `json:"fileSize"`
	SHA256       string `json:"sha256"`
	LocalPath    string `json:"localPath"`
	Status       string `json:"status"`
}

type NetworkStatus struct {
	Status        string   `json:"status"`
	Interfaces    []string `json:"interfaces"`
	LocalIPs      []string `json:"localIps"`
	DiscoveryPort int      `json:"discoveryPort"`
	ChatPort      int      `json:"chatPort"`
	PeerCount     int      `json:"peerCount"`
	OnlineCount   int      `json:"onlineCount"`
	LastScanAt    string   `json:"lastScanAt"`
	LastError     string   `json:"lastError"`
}

type DiagnosticItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Advice string `json:"advice"`
}

type DiagnosticResult struct {
	Status    string           `json:"status"`
	Items     []DiagnosticItem `json:"items"`
	CreatedAt string           `json:"createdAt"`
}

type wireMessage struct {
	Magic        string   `json:"magic,omitempty"`
	Type         string   `json:"type"`
	Protocol     string   `json:"protocol,omitempty"`
	Major        int      `json:"major,omitempty"`
	Minor        int      `json:"minor,omitempty"`
	MinMajor     int      `json:"minMajor,omitempty"`
	MinMinor     int      `json:"minMinor,omitempty"`
	RequestID    string   `json:"requestId,omitempty"`
	MessageID    string   `json:"messageId,omitempty"`
	DeviceID     string   `json:"deviceId,omitempty"`
	Nickname     string   `json:"nickname,omitempty"`
	Platform     string   `json:"platform,omitempty"`
	OSVersion    string   `json:"osVersion,omitempty"`
	IP           string   `json:"ip,omitempty"`
	Port         int      `json:"port,omitempty"`
	PublicKey    string   `json:"publicKey,omitempty"`
	CertFP       string   `json:"certificateFingerprint,omitempty"`
	Content      string   `json:"content,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	Status       string   `json:"status,omitempty"`
	FileName     string   `json:"fileName,omitempty"`
	MimeType     string   `json:"mimeType,omitempty"`
	FileSize     int64    `json:"fileSize,omitempty"`
	SHA256       string   `json:"sha256,omitempty"`
	AttachmentID string   `json:"attachmentId,omitempty"`
	MessageIDs   []string `json:"messageIds,omitempty"`
	ChunkIndex   int      `json:"chunkIndex,omitempty"`
	Payload      string   `json:"payload,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}
