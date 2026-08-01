package ilink

import (
	"encoding/json"
	"io"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
)

// handleGetUpdates 从虚拟 Bot 消息队列消费消息（长轮询）
func (s *Server) handleGetUpdates(c *gin.Context) {
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}
	s.registry.UpdateLastActive(accountID)

	vbot := s.registry.Get(accountID)
	if vbot == nil {
		c.JSON(500, gin.H{"ret": -1, "errmsg": "bot not found"})
		return
	}

	_, _ = io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))

	msgs, timedOut := vbot.Dequeue(35 * time.Second)

	result := struct {
		Ret           int                   `json:"ret"`
		Errmsg        string                `json:"errmsg,omitempty"`
		Msgs          []bot.RawMessageItem `json:"msgs"`
		AccountID     string                `json:"account_id,omitempty"`
		GetUpdatesBuf string                `json:"get_updates_buf"`
	}{
		Ret:           0,
		Msgs:          msgs,
		AccountID:     accountID,
		GetUpdatesBuf: "",
	}
	resultBytes, _ := json.Marshal(result)
	_ = timedOut

	c.Data(200, "application/json", resultBytes)
}

// handleSendMessage 经 Pipeline 路由到 Connector 发送
func (s *Server) handleSendMessage(c *gin.Context) {
	s.handleForward(c)
}

// handleSendTyping 经 Pipeline 路由到 Connector
func (s *Server) handleSendTyping(c *gin.Context) {
	s.handleForward(c)
}

// handleGetConfig 返回虚拟 Bot 自身配置，不依赖真实账号
func (s *Server) handleGetConfig(c *gin.Context) {
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	s.registry.UpdateLastActive(accountID)

	vbot := s.registry.Get(accountID)
	if vbot == nil {
		c.JSON(500, gin.H{"error": "bot not found"})
		return
	}

	c.JSON(200, gin.H{
		"account_id": vbot.AccountID,
		"user_id":    vbot.UserID,
		"base_url":   vbot.BaseURL,
		"token":      vbot.Token,
	})
}

// handleGetUploadURL 经 Pipeline 路由到 Connector
func (s *Server) handleGetUploadURL(c *gin.Context) {
	s.handleForward(c)
}

// handleForward 通用转发：验证 token → 读取 body → 调用 ForwardFunc
func (s *Server) handleForward(c *gin.Context) {
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	s.registry.UpdateLastActive(accountID)

	vbot := s.registry.Get(accountID)
	if vbot == nil {
		c.JSON(500, gin.H{"error": "bot not found"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(500, gin.H{"error": "read body error"})
		return
	}

	if s.forwardFunc == nil {
		c.JSON(500, gin.H{"error": "forward handler not configured"})
		return
	}

	endpoint := c.FullPath()
	respBody, statusCode, err := s.forwardFunc(c.Request.Context(), accountID, endpoint, body)
	if err != nil {
		s.log.Error("forward error", "error", err, "endpoint", endpoint, "account_id", accountID)
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.Data(statusCode, "application/json", respBody)
}
