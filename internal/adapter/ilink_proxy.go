package adapter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// ILinkProxyAdapter iLink 代理适配器（虚拟 Bot）
// 为外部 iLink 服务（Hermes、OpenClaw）生成连接配置
type ILinkProxyAdapter struct {
	id        string
	name      string
	accountID string    // 虚拟 Bot ID（如 gw_a1b2c3d4）
	userID    string    // 用户 ID（如 gw_a1b2c3d4@im.wechat）
	baseURL   string    // Gateway 地址
	createdAt time.Time
}

// NewILinkProxyAdapter 创建 iLink 代理适配器
func NewILinkProxyAdapter(id, name, baseURL string) *ILinkProxyAdapter {
	accountID := "gw_" + generateID()
	userID := accountID + "@im.wechat"

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

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
