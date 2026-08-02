package adapter

import (
	"context"
	"encoding/json"

	"clawbot-gateway/internal/database"
)

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message      string
	UserID       string
	SessionID    string
	BackendID    string
	History      []ChatMessage
	Attachments  []Attachment
	Stream       bool // 是否流式输出
	AccountID    string // 真实账号 ID（ilink_proxy 用于 LastAccountID）
	ContextToken string // 原始消息的 context_token
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Text    string `json:"text"`
	Backend string `json:"backend"`
	Stream  bool   `json:"stream,omitempty"`
	Async   bool   // 异步后端（ilink_proxy），回复通过 SendOutgoingMessage 回传
}

// Attachment 附件
type Attachment struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data,omitempty"`
	URL      string `json:"url,omitempty"`
}

// ConnectionInfo 外部服务连接配置
type ConnectionInfo struct {
	AccountID string `json:"account_id"` // 虚拟 Bot ID（如 gw_a1b2c3d4）
	UserID    string `json:"user_id"`    // 用户 ID（如 gw_a1b2c3d4@im.wechat）
	BaseURL   string `json:"base_url"`   // iLink API 地址（如 https://ilinkai.weixin.qq.com）
}

// ── 接口定义 ──

// BackendAdapter 后端适配器接口（处理消息，返回 AI 响应）
type BackendAdapter interface {
	ID() string
	Name() string
	Type() string
	Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error
	HealthCheck(ctx context.Context) bool
}

// ConnectionAdapter 连接适配器接口（提供外部服务连接配置）
type ConnectionAdapter interface {
	ID() string
	Name() string
	Type() string
	GetConnectionInfo() *ConnectionInfo
	HealthCheck(ctx context.Context) bool
}

// ── 适配器工厂函数类型 ──

// AdapterCreator 从数据库记录创建后端适配器的工厂函数
type AdapterCreator func(b database.Backend) BackendAdapter

// ── 工具函数 ──

// GetJSONString 从 JSON 字符串中提取指定 key 的值
func GetJSONString(jsonStr, key string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
