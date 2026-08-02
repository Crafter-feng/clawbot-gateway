package adapter

import (
	"context"
	"fmt"
	"time"

	"clawbot-gateway/internal/database"
)

// ILinkProxyAdapter iLink 代理适配器（虚拟 Bot）
// 为外部 iLink 服务（Hermes、OpenClaw）生成连接配置
type ILinkProxyAdapter struct {
	id        string
	name      string
	accountID string    // 虚拟 Bot ID（如 gw_a1b2c3d4）
	userID    string    // 用户 ID（如 gw_a1b2c3d4@im.wechat）
	baseURL   string    // iLink API 地址
	createdAt time.Time
}

// NewILinkProxyAdapter 创建 iLink 代理适配器
// accountID 和 userID 必须由调用者提供（从数据库加载或生成确定性 ID）
func NewILinkProxyAdapter(id, name, accountID, userID, baseURL string) *ILinkProxyAdapter {
	return &ILinkProxyAdapter{
		id:        id,
		name:      name,
		accountID: accountID,
		userID:    userID,
		baseURL:   baseURL,
		createdAt: time.Now(),
	}
}

func (a *ILinkProxyAdapter) ID() string   { return a.id }
func (a *ILinkProxyAdapter) Name() string { return a.name }
func (a *ILinkProxyAdapter) Type() string { return "ilink_proxy" }

// GetConnectionInfo 返回虚拟 Bot 连接配置
func (a *ILinkProxyAdapter) GetConnectionInfo() *ConnectionInfo {
	return &ConnectionInfo{
		AccountID: a.accountID,
		UserID:    a.userID,
		BaseURL:   a.baseURL,
	}
}

// HealthCheck 虚拟 Bot 始终健康
func (a *ILinkProxyAdapter) HealthCheck(ctx context.Context) bool {
	return true
}

// GetAccountID 获取虚拟 Bot 的 account_id
func (a *ILinkProxyAdapter) GetAccountID() string {
	return a.accountID
}

// GetUserID 获取虚拟 Bot 的 user_id
func (a *ILinkProxyAdapter) GetUserID() string {
	return a.userID
}

// EnqueueFunc 由 Pipeline 设置，用于将消息入队到虚拟 Bot
type EnqueueFunc func(req *ChatRequest) bool

// ILinkProxyBackendAdapter iLink 代理后端适配器
// Handle() 将消息入队虚拟 Bot 后返回空响应（异步回复，通过 SendOutgoingMessage 回传）
type ILinkProxyBackendAdapter struct {
	id      string
	name    string
	enqueue EnqueueFunc
}

func (a *ILinkProxyBackendAdapter) ID() string                           { return a.id }
func (a *ILinkProxyBackendAdapter) Name() string                         { return a.name }
func (a *ILinkProxyBackendAdapter) Type() string                         { return "ilink_proxy" }
func (a *ILinkProxyBackendAdapter) HealthCheck(ctx context.Context) bool { return true }

// SetEnqueueFunc 设置入队回调（由 Pipeline 在初始化时调用）
func (a *ILinkProxyBackendAdapter) SetEnqueueFunc(f EnqueueFunc) {
	a.enqueue = f
}

// Handle 入队消息到虚拟 Bot，返回空响应表示异步回复
// 外部服务通过 getupdates 消费后，通过 sendmessage 返回回复
func (a *ILinkProxyBackendAdapter) Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if a.enqueue == nil {
		return nil, fmt.Errorf("ilink_proxy: enqueue func not set for backend %s", a.id)
	}
	if !a.enqueue(req) {
		return nil, fmt.Errorf("ilink_proxy: enqueue failed for backend %s", a.id)
	}
	// 空 Text 表示异步后端，Pipeline 会跳过直接回复
	return &ChatResponse{Text: "", Backend: a.id}, nil
}

func (a *ILinkProxyBackendAdapter) HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error {
	return fmt.Errorf("ilink_proxy backend does not support streaming")
}

// 自注册
func init() {
	RegisterAdapter("ilink_proxy", func(b database.Backend) BackendAdapter {
		return &ILinkProxyBackendAdapter{id: b.ID, name: b.Name}
	})
}
