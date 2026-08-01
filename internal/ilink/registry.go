package ilink

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"

	"clawbot-gateway/internal/bot"
)

// ── 虚拟 Bot ──

// VirtualBot 虚拟 Bot 实例
// 队列消费模式：Pipeline 生产消息入队，外部服务通过 getupdates 消费
type VirtualBot struct {
	AccountID  string         // 虚拟 Bot ID（如 gw_hermes）
	UserID     string        // 用户 ID（如 gw_hermes@im.wechat）
	BaseURL    string         // iLink API 地址（用于回复类端点透明转发）
	Token      string         // 随机生成的认证 token（持久化到数据库）
	LastAccountID string        // 最后入队的消息来源真实账号 ID
	LastActive    time.Time      // 最后活跃时间
	CreatedAt     time.Time      // 创建时间

	// 消息队列：Pipeline 路由到此后入队，外部服务 getupdates 消费
	queueMu sync.Mutex
	queue   []bot.RawMessageItem
	notify  chan struct{} // buffered cap 1: Enqueue 发信号唤醒等待的 Dequeue
}

// ── 客户端注册表 ──

// ClientRegistry 管理所有虚拟 Bot
// 队列消费模式：管理虚拟 Bot 的连接配置和消息队列
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
// token 参数：如果非空则使用该 token（从数据库恢复），否则生成随机 token
func (r *ClientRegistry) Register(accountID, userID, baseURL, token string) *VirtualBot {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.bots[accountID]; ok {
		existing.LastActive = time.Now()
		return existing
	}

	// 生成新的随机 token
	if token == "" {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			// 熵源不可用是严重错误，panic 而非静默降级
			panic("ilink: failed to generate random token: " + err.Error())
		}
		token = hex.EncodeToString(tokenBytes)
	}

	bot := &VirtualBot{
		AccountID:  accountID,
		UserID:     userID,
		BaseURL:    baseURL,
		Token:      token,
		LastActive: time.Now(),
		CreatedAt:  time.Now(),
	}
	bot.notify = make(chan struct{}, 1)
	r.bots[accountID] = bot
	return bot
}

// Enqueue 将消息入队（由 Pipeline 调用）
// 唤醒等待中的 getupdates 长轮询
func (b *VirtualBot) Enqueue(msg bot.RawMessageItem) {
	b.queueMu.Lock()
	b.queue = append(b.queue, msg)
	b.queueMu.Unlock()
	// 非阻塞发信号：notify buffered cap 1，没有等待者时不阻塞
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// Dequeue 阻塞等待并取出队列中的所有消息（由 iLink 服务端 getupdates 调用）
// timeout 为最长等待时间；返回消息列表和是否超时
// 取出后清空队列
func (b *VirtualBot) Dequeue(timeout time.Duration) ([]bot.RawMessageItem, bool) {
	// 快速路径：先检查队列是否有消息
	b.queueMu.Lock()
	if len(b.queue) > 0 {
		msgs := b.queue
		b.queue = nil
		b.queueMu.Unlock()
		return msgs, false
	}
	b.queueMu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-b.notify:
		// 有消息入队，取出所有
		b.queueMu.Lock()
		defer b.queueMu.Unlock()
		if len(b.queue) > 0 {
			msgs := b.queue
			b.queue = nil
			return msgs, false
		}
		// 信号被消费但没有消息：其他 Dequeue 已经取走，返回空
		return nil, false
	case <-timer.C:
		// 超时最后一次检查，避免丢失在 select 之前 Enqueue 的消息
		b.queueMu.Lock()
		defer b.queueMu.Unlock()
		if len(b.queue) > 0 {
			msgs := b.queue
			b.queue = nil
			return msgs, false
		}
		return nil, true
	}
}

// QueueLength 返回队列中消息数（用于统计/调试）
func (b *VirtualBot) QueueLength() int {
	b.queueMu.Lock()
	defer b.queueMu.Unlock()
	return len(b.queue)
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
	b, ok := r.bots[accountID]
	if !ok {
		return nil
	}
	return b
}

// GetByToken 通过 token 查找虚拟 Bot
func (r *ClientRegistry) GetByToken(token string) *VirtualBot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, bot := range r.bots {
		if subtle.ConstantTimeCompare([]byte(bot.Token), []byte(token)) == 1 {
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

// IsConnected 判断虚拟 Bot 是否在最近 timeout 时间内活跃过
// 外部客户端通过长轮询 getupdates，如果在 timeout 内有请求则认为已连接
func (r *ClientRegistry) IsConnected(accountID string, timeout time.Duration) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if bot, ok := r.bots[accountID]; ok {
		return !bot.LastActive.IsZero() && time.Since(bot.LastActive) < timeout
	}
	return false
}

// GetStats 获取统计信息
func (r *ClientRegistry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()
	connected := 0
	for _, bot := range r.bots {
		if !bot.LastActive.IsZero() && now.Sub(bot.LastActive) < 2*time.Minute {
			connected++
		}
	}

	return map[string]interface{}{
		"total_bots": len(r.bots),
		"connected":  connected,
	}
}
