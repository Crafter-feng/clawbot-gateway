package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/crypto"
	"clawbot-gateway/internal/database"
)

// Handler Webhook 处理器
type Handler struct {
	db       *database.DB
	sendFunc func(ctx context.Context, toUser, content, accountID string) error
}

// NewHandler 创建 Webhook 处理器
func NewHandler(db *database.DB, sendFunc func(ctx context.Context, toUser, content, accountID string) error) *Handler {
	return &Handler{
		db:       db,
		sendFunc: sendFunc,
	}
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

	// 查找 Token
	notifyToken, err := h.db.GetNotifyToken(tokenStr)
	if err != nil || notifyToken == nil || !notifyToken.Enabled {
		c.JSON(401, gin.H{"error": "invalid or disabled token"})
		return
	}

	// 解析请求
	var req struct {
		ToUser  string `json:"to_user"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.ToUser == "" || req.Content == "" {
		c.JSON(400, gin.H{"error": "to_user and content are required"})
		return
	}

	// 发送消息
	if err := h.sendFunc(c.Request.Context(), req.ToUser, req.Content, notifyToken.AccountID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// HandleListTokens 列出所有 Token
func (h *Handler) HandleListTokens(c *gin.Context) {
	tokens, err := h.db.ListNotifyTokens()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"tokens": tokens})
}

// HandleCreateToken 创建 Token
func (h *Handler) HandleCreateToken(c *gin.Context) {
	var req struct {
		AccountID string `json:"account_id"` // 空=全部账号
		Name      string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}

	id := "wh_" + time.Now().Format("20060102150405") + "_" + crypto.GenerateSecret(6)
	token := crypto.GenerateSecret(32)

	t := database.NotifyToken{
		ID:        id,
		AccountID: req.AccountID,
		Name:      req.Name,
		Token:     token,
		Enabled:   true,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := h.db.CreateNotifyToken(t); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"id":    id,
		"token": token,
		"url":   fmt.Sprintf("%s/api/v1/notify/send", c.Request.Host),
	})
}

// HandleDeleteToken 删除 Token
func (h *Handler) HandleDeleteToken(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.DeleteNotifyToken(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/api/v1/notify/send", h.HandleSend)
	r.GET("/api/v1/notify/tokens", h.HandleListTokens)
	r.POST("/api/v1/notify/tokens", h.HandleCreateToken)
	r.DELETE("/api/v1/notify/tokens/:id", h.HandleDeleteToken)
}
