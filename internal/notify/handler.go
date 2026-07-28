package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/crypto"
	"clawbot-gateway/internal/database"
)

// Handler 通知处理器
type Handler struct {
	db          *database.DB
	sendFunc    func(ctx context.Context, toUser, content string) error
	log         *slog.Logger
	publicURL   string
	rateLimitMu sync.Mutex
	rateCounts  map[string]int
	rateReset   map[string]time.Time
}

// NewHandler 创建通知处理器
func NewHandler(db *database.DB, sendFunc func(ctx context.Context, toUser, content string) error) *Handler {
	return &Handler{
		db:         db,
		sendFunc:   sendFunc,
		log:        slog.Default(),
		publicURL:  "",
		rateCounts: make(map[string]int),
		rateReset:  make(map[string]time.Time),
	}
}

// SetLogger 设置日志记录器
func (h *Handler) SetLogger(log *slog.Logger) {
	h.log = log
}

// SetPublicURL 设置外部可访问的 URL
func (h *Handler) SetPublicURL(url string) {
	h.publicURL = url
}

func (h *Handler) checkRateLimit(token string) bool {
	h.rateLimitMu.Lock()
	defer h.rateLimitMu.Unlock()
	now := time.Now()
	resetAt, ok := h.rateReset[token]
	if !ok || now.After(resetAt) {
		h.rateCounts[token] = 0
		h.rateReset[token] = now.Add(time.Second)
	}
	h.rateCounts[token]++
	return h.rateCounts[token] <= 5
}

// HandleSend 发送消息
func (h *Handler) HandleSend(c *gin.Context) {
	// 验证 Token
	auth := c.GetHeader("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		c.JSON(401, gin.H{"error": "missing or invalid authorization header"})
		return
	}
	tokenStr := auth[7:]

	// 限流检查
	if !h.checkRateLimit(tokenStr) {
		h.log.Warn("rate limit exceeded", "component", "notify")
		c.JSON(429, gin.H{"error": "too many requests"})
		return
	}

	// 查找 Token（constant-time 比较）
	tokens, err := h.db.ListNotifyTokens()
	if err != nil {
		h.log.Error("list notify tokens", "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	var notifyToken *database.NotifyToken
	for _, t := range tokens {
		tt := t
		if crypto.SecureEqual(tt.Token, tokenStr) {
			notifyToken = &tt
			break
		}
	}
	if notifyToken == nil || !notifyToken.Enabled {
		c.JSON(401, gin.H{"error": "invalid or disabled token"})
		return
	}

	// 解析请求
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Content == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}

	if notifyToken.ToUser == "" {
		c.JSON(400, gin.H{"error": "token has no binding target"})
		return
	}
	if err := h.sendFunc(c.Request.Context(), notifyToken.ToUser, req.Content); err != nil {
		h.log.Error("failed to send notify message", "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *Handler) HandleListTokens(c *gin.Context) {
	tokens, err := h.db.ListNotifyTokens()
	if err != nil {
		h.log.Error("failed to list notify tokens", "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, gin.H{"tokens": tokens})
}

func (h *Handler) HandleCreateToken(c *gin.Context) {
	var req struct {
		ToUser string `json:"to_user"` // 空=推送全部客户
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}

	id := "nt_" + time.Now().Format("20060102150405") + "_" + crypto.MustGenerateSecret(6)
	token, err := crypto.GenerateSecret(32)
	if err != nil {
		h.log.Error("failed to generate token secret", "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	t := database.NotifyToken{
		ID:        id,
		ToUser:    req.ToUser,
		Name:      req.Name,
		Token:     token,
		Enabled:   true,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := h.db.CreateNotifyToken(t); err != nil {
		h.log.Error("failed to create notify token", "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	baseURL := h.publicURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://%s", c.Request.Host)
	}
	c.JSON(200, gin.H{
		"id":    id,
		"token": token,
		"url":   fmt.Sprintf("%s/api/v1/notify/send", baseURL),
	})
}

// HandleDeleteToken 删除 Token
func (h *Handler) HandleDeleteToken(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.DeleteNotifyToken(id); err != nil {
		h.log.Error("failed to delete notify token", "id", id, "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// RegisterRoutes 注册路由
// 注意：token 管理端点需要在 api/v1 认证组中注册，不能直接注册在 engine 上
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// 公开端点：发送消息（使用 notify token 认证）
	r.POST("/api/v1/notify/send", h.HandleSend)
}

// RegisterManagementRoutes 注册需要认证的管理路由
func (h *Handler) RegisterManagementRoutes(rg *gin.RouterGroup) {
	rg.GET("/tokens", h.HandleListTokens)
	rg.POST("/tokens", h.HandleCreateToken)
	rg.DELETE("/tokens/:id", h.HandleDeleteToken)
}
