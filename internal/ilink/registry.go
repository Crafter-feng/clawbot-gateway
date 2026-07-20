package ilink

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"clawbot-gateway/internal/bot"
)

// ── 消息持久化 ──

// PersistedMessage 持久化消息格式
type PersistedMessage struct {
	ID        string                  `json:"id"`
	Message   bot.NormalizedMessage   `json:"message"`
	RetryCount int                    `json:"retry_count"`
	MaxRetries int                    `json:"max_retries"`
	Status     string                 `json:"status"` // "pending", "sent", "failed"
	CreatedAt  time.Time              `json:"created_at"`
	SentAt     *time.Time             `json:"sent_at,omitempty"`
	NextRetry  *time.Time             `json:"next_retry,omitempty"`
}

// ── 消息队列 ──

// MessageQueue 每个虚拟 Bot 的消息队列（支持持久化和重试）
type MessageQueue struct {
	mu          sync.Mutex
	msgs        []bot.NormalizedMessage
	persisted   []*PersistedMessage   // 待持久化的消息
	event       chan struct{}
	cap         int
	accountID   string                // 所属虚拟 Bot 的 account_id
	dataDir     string                // 持久化目录
	retryConfig RetryConfig
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries     int           // 最大重试次数
	InitialBackoff time.Duration // 初始退避时间
	MaxBackoff     time.Duration // 最大退避时间
	RetryFactor    float64       // 退避因子
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Minute,
		RetryFactor:    2.0,
	}
}

// NewMessageQueue 创建消息队列
func NewMessageQueue(capacity int, accountID, dataDir string) *MessageQueue {
	q := &MessageQueue{
		msgs:        make([]bot.NormalizedMessage, 0, capacity),
		persisted:   make([]*PersistedMessage, 0),
		event:       make(chan struct{}, 1),
		cap:         capacity,
		accountID:   accountID,
		dataDir:     dataDir,
		retryConfig: DefaultRetryConfig(),
	}

	// 加载持久化的消息
	q.load()

	return q
}

// Enqueue 入队消息
func (q *MessageQueue) Enqueue(msg bot.NormalizedMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.msgs) >= q.cap {
		// 队列满，丢弃最旧的消息
		q.msgs = q.msgs[1:]
	}

	q.msgs = append(q.msgs, msg)

	// 创建持久化消息
	persisted := &PersistedMessage{
		ID:         msg.MsgID,
		Message:    msg,
		RetryCount: 0,
		MaxRetries: q.retryConfig.MaxRetries,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	q.persisted = append(q.persisted, persisted)

	// 保存到磁盘
	q.save()

	// 通知等待者
	select {
	case q.event <- struct{}{}:
	default:
	}
}

// DequeueAll 出队所有消息（支持长轮询）
func (q *MessageQueue) DequeueAll(timeout time.Duration) []bot.NormalizedMessage {
	// 等待消息或超时
	if timeout > 0 {
		select {
		case <-q.event:
		case <-time.After(timeout):
		}
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.msgs) == 0 {
		return nil
	}

	result := q.msgs
	q.msgs = make([]bot.NormalizedMessage, 0, q.cap)

	// 标记消息为已发送
	now := time.Now()
	for _, p := range q.persisted {
		if p.Status == "pending" {
			p.Status = "sent"
			p.SentAt = &now
		}
	}
	q.persisted = q.persisted[:0]

	// 保存到磁盘
	q.save()

	return result
}

// MarkFailed 标记消息发送失败（用于重试）
func (q *MessageQueue) MarkFailed(msgID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, p := range q.persisted {
		if p.ID == msgID && p.Status == "sent" {
			p.Status = "failed"
			p.RetryCount++

			if p.RetryCount < p.MaxRetries {
				// 计算下次重试时间
				backoff := q.calculateBackoff(p.RetryCount)
				nextRetry := time.Now().Add(backoff)
				p.NextRetry = &nextRetry
				p.Status = "pending"

				// 重新入队
				q.msgs = append(q.msgs, p.Message)
			}
			break
		}
	}

	q.save()
}

// calculateBackoff 计算退避时间
func (q *MessageQueue) calculateBackoff(retryCount int) time.Duration {
	backoff := q.retryConfig.InitialBackoff
	for i := 0; i < retryCount; i++ {
		backoff = time.Duration(float64(backoff) * q.retryConfig.RetryFactor)
		if backoff > q.retryConfig.MaxBackoff {
			backoff = q.retryConfig.MaxBackoff
			break
		}
	}
	return backoff
}

// GetRetryable 获取可重试的消息
func (q *MessageQueue) GetRetryable() []*PersistedMessage {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	result := make([]*PersistedMessage, 0)

	for _, p := range q.persisted {
		if p.Status == "pending" && p.NextRetry != nil && p.NextRetry.Before(now) {
			result = append(result, p)
		}
	}

	return result
}

// Size 返回队列大小
func (q *MessageQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.msgs)
}

// PendingCount 返回待处理消息数
func (q *MessageQueue) PendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	count := 0
	for _, p := range q.persisted {
		if p.Status == "pending" {
			count++
		}
	}
	return count
}

// ── 持久化 ──

func (q *MessageQueue) save() {
	if q.dataDir == "" {
		return
	}

	path := q.filePath()
	data, err := json.MarshalIndent(q.persisted, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(q.dataDir, 0700)
	os.WriteFile(path, data, 0600)
}

func (q *MessageQueue) load() {
	if q.dataDir == "" {
		return
	}

	path := q.filePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var persisted []*PersistedMessage
	if err := json.Unmarshal(data, &persisted); err != nil {
		return
	}

	// 恢复未完成的消息
	for _, p := range persisted {
		if p.Status == "pending" {
			q.persisted = append(q.persisted, p)
			q.msgs = append(q.msgs, p.Message)
		}
	}

	// 通知有消息
	if len(q.msgs) > 0 {
		select {
		case q.event <- struct{}{}:
		default:
		}
	}
}

func (q *MessageQueue) filePath() string {
	safeName := filepath.Base(q.accountID)
	if safeName == "." || safeName == "/" {
		safeName = "unknown"
	}
	return filepath.Join(q.dataDir, safeName+".queue.json")
}

// ── 虚拟 Bot ──

// VirtualBot 虚拟 Bot 实例
type VirtualBot struct {
	AccountID  string         // 虚拟 Bot ID（如 gw_a1b2c3d4）
	UserID     string         // 用户 ID（如 gw_a1b2c3d4@im.wechat）
	BaseURL    string         // iLink API 地址
	Queue      *MessageQueue  // 独立消息队列
	UpdateBuf  string         // get_updates 游标
	LastActive time.Time      // 最后活跃时间
	CreatedAt  time.Time      // 创建时间
}

// ── 客户端注册表 ──

// ClientRegistry 管理所有虚拟 Bot
type ClientRegistry struct {
	mu      sync.RWMutex
	bots    map[string]*VirtualBot // accountID → VirtualBot
	dataDir string                 // 持久化目录
}

// NewClientRegistry 创建客户端注册表
func NewClientRegistry(dataDir string) *ClientRegistry {
	if dataDir == "" {
		dataDir = "data/queues"
	}
	return &ClientRegistry{
		bots:    make(map[string]*VirtualBot),
		dataDir: dataDir,
	}
}

// Register 注册虚拟 Bot
func (r *ClientRegistry) Register(accountID, userID, baseURL string) *VirtualBot {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.bots[accountID]; ok {
		return existing
	}

	bot := &VirtualBot{
		AccountID:  accountID,
		UserID:     userID,
		BaseURL:    baseURL,
		Queue:      NewMessageQueue(200, accountID, r.dataDir),
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

// GetByToken 通过 token 查找虚拟 Bot（token 就是 accountID）
func (r *ClientRegistry) GetByToken(token string) *VirtualBot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bots[token]
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

// Broadcast 广播消息到所有虚拟 Bot
func (r *ClientRegistry) Broadcast(msg bot.NormalizedMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, bot := range r.bots {
		bot.Queue.Enqueue(msg)
		bot.LastActive = time.Now()
	}
}

// Count 返回虚拟 Bot 数量
func (r *ClientRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bots)
}

// GetStats 获取统计信息
func (r *ClientRegistry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]interface{}{
		"total_bots":     len(r.bots),
		"total_pending":  0,
		"total_messages": 0,
	}

	for _, bot := range r.bots {
		stats["total_pending"] = stats["total_pending"].(int) + bot.Queue.PendingCount()
		stats["total_messages"] = stats["total_messages"].(int) + bot.Queue.Size()
	}

	return stats
}

// ── 重试工作器 ──

// RetryWorker 重试工作器
type RetryWorker struct {
	registry *ClientRegistry
	interval time.Duration
	stopCh   chan struct{}
	log      func(string, ...interface{})
}

// NewRetryWorker 创建重试工作器
func NewRetryWorker(registry *ClientRegistry, interval time.Duration, logger func(string, ...interface{})) *RetryWorker {
	if interval == 0 {
		interval = 30 * time.Second
	}
	if logger == nil {
		logger = func(msg string, args ...interface{}) {}
	}
	return &RetryWorker{
		registry: registry,
		interval: interval,
		stopCh:   make(chan struct{}),
		log:      logger,
	}
}

// Start 启动重试工作器
func (w *RetryWorker) Start() {
	go w.loop()
}

// Stop 停止重试工作器
func (w *RetryWorker) Stop() {
	close(w.stopCh)
}

func (w *RetryWorker) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processRetries()
		}
	}
}

func (w *RetryWorker) processRetries() {
	w.registry.mu.RLock()
	bots := make([]*VirtualBot, 0, len(w.registry.bots))
	for _, bot := range w.registry.bots {
		bots = append(bots, bot)
	}
	w.registry.mu.RUnlock()

	for _, bot := range bots {
		retryable := bot.Queue.GetRetryable()
		for _, msg := range retryable {
			w.log("retrying message",
				"account_id", bot.AccountID,
				"msg_id", msg.ID,
				"retry_count", msg.RetryCount,
			)
			// 消息已经在队列中，等待下次 getupdates 时发送
		}
	}
}
