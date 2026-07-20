package store

import "time"

// Device 设备表行。
type Device struct {
	ID           string
	UserID       string
	PairingCode  string // 空表示已签发长期 token 后清空
	TokenHash    string // SHA-256 hex，未签发时空
	Status       string // unbound | bound
	HardwareRev  string
	LastSeenAt   int64 // 毫秒
	CreatedAt    int64 // 毫秒
}

// User 用户表行。
type User struct {
	ID          string
	DisplayName string
	UserMD      string // 可选 per-user 覆盖 USER.md（Markdown 文本）
	SoulMD      string // 可选 per-user 覆盖 SOUL.md（Markdown 文本）
	CreatedAt   int64
	UpdatedAt   int64
}

// Conversation 会话表行。
type Conversation struct {
	ID         string
	DeviceID   string
	StartedAt  int64
	EndedAt    int64 // 0 表示进行中
	TurnCount  int
}

// Message 消息表行。
type Message struct {
	ID             string
	ConversationID string
	Role           string // user | assistant | tool
	Content        string
	ToolName       string
	ToolArgs       string // JSON
	LatencyMs      int
	TokensIn       int
	TokensOut      int
	CreatedAt      int64
}

// NowMs 返回当前毫秒时间戳。
func NowMs() int64 { return time.Now().UnixMilli() }
