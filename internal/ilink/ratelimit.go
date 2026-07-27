package ilink

import (
	"sync"
	"time"

)

// ── 速率限制器 ──

// RateLimiter 令牌桶速率限制器
type RateLimiter struct {
	mu       sync.Mutex
	tokens   map[string]*TokenBucket // key → bucket
	rate     int                     // 每秒产生的令牌数
	burst    int                     // 桶容量
	cleanup  time.Duration           // 清理间隔
	stopMu   sync.Mutex
	stopped  bool
	stopCh   chan struct{}
}

// TokenBucket 令牌桶
type TokenBucket struct {
	tokens    float64
	maxTokens float64
	refillRate float64 // 每秒补充的令牌数
	lastRefill time.Time
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		tokens:  make(map[string]*TokenBucket),
		rate:    rate,
		burst:   burst,
		cleanup: 5 * time.Minute,
		stopCh:  make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.tokens[key]
	if !exists {
		bucket = &TokenBucket{
			tokens:     float64(rl.burst),
			maxTokens:  float64(rl.burst),
			refillRate: float64(rl.rate),
			lastRefill: time.Now(),
		}
		rl.tokens[key] = bucket
	}

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastRefill = now

	// 尝试消费令牌
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}
func (rl *RateLimiter) Stop() {
	rl.stopMu.Lock()
	defer rl.stopMu.Unlock()
	if !rl.stopped {
		close(rl.stopCh)
		rl.stopped = true
	}
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanupBuckets()
		}
	}
}

func (rl *RateLimiter) cleanupBuckets() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, bucket := range rl.tokens {
		// 如果超过 5 分钟没使用，删除
		if now.Sub(bucket.lastRefill) > 5*time.Minute {
			delete(rl.tokens, key)
		}
	}
}

// ── 请求体大小限制 ──

// MaxRequestBodySize 最大请求体大小（1MB）
const MaxRequestBodySize = 1 * 1024 * 1024


