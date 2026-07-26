package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/adapter"
)

// 虚拟 Bot 连接超时时间：超过此时间无请求则认为离线
const connectionTimeout = 2 * time.Minute

// ── 连接适配器 API ──

func (s *APIServer) buildConnectionResponse(conn adapter.ConnectionAdapter, ctx context.Context) gin.H {
	info := conn.GetConnectionInfo()
	token := ""
	connected := false
	lastActive := ""
	if vbot := s.clientReg.Get(info.AccountID); vbot != nil {
		token = vbot.Token
		connected = s.clientReg.IsConnected(info.AccountID, connectionTimeout)
		if !vbot.LastActive.IsZero() {
			lastActive = vbot.LastActive.Format(time.RFC3339)
		}
	}
	return gin.H{
		"id":          conn.ID(),
		"name":        conn.Name(),
		"type":        conn.Type(),
		"account_id":  info.AccountID,
		"user_id":     info.UserID,
		"base_url":    info.BaseURL,
		"token":       token,
		"connected":   connected,
		"last_active": lastActive,
		"healthy":     conn.HealthCheck(ctx),
	}
}

// handleListConnections 列出所有连接适配器（虚拟 Bot 配置）
func (s *APIServer) handleListConnections(c *gin.Context) {
	s.mu.RLock()
	connections := s.adapters.ListConnections()
	s.mu.RUnlock()
	result := make([]gin.H, 0, len(connections))

	for _, conn := range connections {
		result = append(result, s.buildConnectionResponse(conn, c.Request.Context()))
	}

	c.JSON(http.StatusOK, gin.H{
		"connections": result,
	})
}

func (s *APIServer) handleGetConnection(c *gin.Context) {
	id := c.Param("id")

	s.mu.RLock()
	conn, ok := s.adapters.GetConnection(id)
	s.mu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}

	c.JSON(http.StatusOK, s.buildConnectionResponse(conn, c.Request.Context()))
}

// handleConnectionStats 获取连接统计信息
func (s *APIServer) handleConnectionStats(c *gin.Context) {
	stats := s.clientReg.GetStats()
	c.JSON(http.StatusOK, stats)
}