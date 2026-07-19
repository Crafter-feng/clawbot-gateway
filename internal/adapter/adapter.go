package adapter

import (
	"context"

	"clawbot-gateway/internal/session"
)

// ChatRequest 聊天请求
type ChatRequest struct {
	Message     string
	UserID      string
	SessionID   string
	BackendID   string
	History     []session.ChatMessage
	Attachments []Attachment
	Stream      bool // 是否流式输出
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Text    string `json:"text"`
	Backend string `json:"backend"`
	Stream  bool   `json:"stream,omitempty"`
}

// Attachment 附件
type Attachment struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data,omitempty"`
	URL      string `json:"url,omitempty"`
}

// BackendAdapter 后端适配器接口（处理消息，返回 AI 响应）
type BackendAdapter interface {
	ID() string
	Name() string
	Type() string
	Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error
	HealthCheck(ctx context.Context) bool
}

// ConnectionInfo 外部服务连接配置
type ConnectionInfo struct {
	AccountID string `json:"account_id"` // 虚拟 Bot ID（如 gw_a1b2c3d4）
	UserID    string `json:"user_id"`    // 用户 ID（如 gw_a1b2c3d4@im.wechat）
	BaseURL   string `json:"base_url"`   // Gateway 地址（如 http://localhost:8080）
}

// ConnectionAdapter 连接适配器接口（提供外部服务连接配置）
type ConnectionAdapter interface {
	ID() string
	Name() string
	Type() string
	GetConnectionInfo() *ConnectionInfo
	HealthCheck(ctx context.Context) bool
}
