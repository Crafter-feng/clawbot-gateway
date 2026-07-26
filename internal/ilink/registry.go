package ilink

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// ── 虚拟 Bot ──

// VirtualBot 虚拟 Bot 实例
// 透明代理模式：只存储连接配置，不存储消息队列
type VirtualBot struct {
	AccountID  string         // 虚拟 Bot ID（如 gw_a1b2c3d4）
	UserID     string         // 用户 ID（如 gw_a1b2c3d4@im.wechat）
	BaseURL    string         // iLink API 地址
	Token      string         // 随机生成的认证 token
	LastActive time.Time      // 最后活跃时间
	CreatedAt  time.Time      // 创建时间
}

// ── 客户端注册表 ──

// ClientRegistry 管理所有虚拟 Bot
// 透明代理模式：只管理虚拟 Bot 的连接配置
type ClientRegistry struct {
	mu   sync.RWMutex
	bots map[string]*VirtualBot // accountID → VirtualBot
}

// NewClientRegistry 创建客户端注册表
func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{
		bots: make(map[string]*VirtualBot),
	}
}

// Register 注册虚拟 Bot
func (r *ClientRegistry) Register(accountID, userID, baseURL string) *VirtualBot {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.bots[accountID]; ok {
		return existing
	}

	// 生成随机 token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		tokenBytes = []byte(accountID) // fallback: 使用 accountID 作为 token
	}
	token := hex.EncodeToString(tokenBytes)

	bot := &VirtualBot{
		AccountID:  accountID,
		UserID:     userID,
		BaseURL:    baseURL,
		Token:      token,
		LastActive: time.Now(),
		CreatedAt:  time.Now(),
	}
	r.bots[accountID] = bot
	return bot
}

// Unregister 注销虚拟 Bot
func (r *ClientRegistry) Unregister(accountID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bots, accountID)
}

// Get 获取虚拟 Bot
func (r *ClientRegistry) Get(accountID string) *VirtualBot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bots[accountID]
}

// GetByToken 通过 token 查找虚拟 Bot
func (r *ClientRegistry) GetByToken(token string) *VirtualBot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, bot := range r.bots {
		if bot.Token == token {
			return bot
		}
	}
	return nil
}

// List 列出所有虚拟 Bot
func (r *ClientRegistry) List() []*VirtualBot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*VirtualBot, 0, len(r.bots))
	for _, bot := range r.bots {
		result = append(result, bot)
	}
	return result
}

// Count 返回虚拟 Bot 数量
func (r *ClientRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bots)
}

// UpdateLastActive 更新虚拟 Bot 的最后活跃时间
func (r *ClientRegistry) UpdateLastActive(accountID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if bot, ok := r.bots[accountID]; ok {
		bot.LastActive = time.Now()
	}
}

// GetStats 获取统计信息
func (r *ClientRegistry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"total_bots": len(r.bots),
	}
}
