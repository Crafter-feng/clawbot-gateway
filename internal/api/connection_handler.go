package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ── 连接适配器 API ──

// handleListConnections 列出所有连接适配器（虚拟 Bot 配置）
func (s *APIServer) handleListConnections(c *gin.Context) {
	connections := s.adapters.ListConnections()
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

// handleGetConnection 获取单个连接适配器详情
func (s *APIServer) handleGetConnection(c *gin.Context) {
	id := c.Param("id")

	conn, ok := s.adapters.GetConnection(id)
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
