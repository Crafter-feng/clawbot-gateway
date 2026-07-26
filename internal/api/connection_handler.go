package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ── 连接适配器 API ──

// handleListConnections 列出所有连接适配器（虚拟 Bot 配置）
func (s *APIServer) handleListConnections(c *gin.Context) {
	s.mu.RLock()
	connections := s.adapters.ListConnections()
	s.mu.RUnlock()
	result := make([]gin.H, 0, len(connections))

	for _, conn := range connections {
		info := conn.GetConnectionInfo()
		result = append(result, gin.H{
			"id":         conn.ID(),
			"name":       conn.Name(),
			"type":       conn.Type(),
			"account_id": info.AccountID,
			"user_id":    info.UserID,
			"base_url":   info.BaseURL,
			"healthy":    conn.HealthCheck(c.Request.Context()),
		})
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

	info := conn.GetConnectionInfo()
	c.JSON(http.StatusOK, gin.H{
		"id":         conn.ID(),
		"name":       conn.Name(),
		"type":       conn.Type(),
		"account_id": info.AccountID,
		"user_id":    info.UserID,
		"base_url":   info.BaseURL,
		"healthy":    conn.HealthCheck(c.Request.Context()),
	})
}

// handleConnectionStats 获取连接统计信息
func (s *APIServer) handleConnectionStats(c *gin.Context) {
	stats := s.clientReg.GetStats()
	c.JSON(http.StatusOK, stats)
}
