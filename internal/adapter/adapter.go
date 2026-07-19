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

// BackendAdapter 后端适配器接口
type BackendAdapter interface {
	ID() string
	Name() string
	Type() string
	Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error
	HealthCheck(ctx context.Context) bool
}
