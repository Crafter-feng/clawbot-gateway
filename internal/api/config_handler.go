package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// sensitiveKeys 敏感配置键（不通过 API 暴露）
var sensitiveKeys = map[string]bool{
	"api.jwt_secret":      true,
	"api.login_password":  true,
	"api.token":           true,
}

func (s *APIServer) handleGetAllConfig(c *gin.Context) {
	settings := s.db.GetAllSettings()

	// 过滤敏感配置
	filtered := make(map[string]string)
	for k, v := range settings {
		if sensitiveKeys[k] {
			// 敏感配置只返回是否已设置，不返回实际值
			if v != "" {
				filtered[k] = "***"
			}
		} else {
			filtered[k] = v
		}
	}

	c.JSON(200, gin.H{"settings": filtered})
}

func (s *APIServer) handleUpdateConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 禁止通过 API 修改敏感配置
	for k := range req {
		if sensitiveKeys[k] {
			c.JSON(403, gin.H{"error": "cannot modify sensitive setting via API"})
			return
		}
	}

	for k, v := range req {
		s.db.SetSetting(k, v)
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleGetConfig(c *gin.Context) {
	key := c.Param("key")

	// 禁止获取敏感配置
	if sensitiveKeys[key] {
		c.JSON(403, gin.H{"error": "cannot access sensitive setting"})
		return
	}

	value := s.db.GetSetting(key)
	c.JSON(200, gin.H{"key": key, "value": value})
}

func (s *APIServer) handleSetConfig(c *gin.Context) {
	key := c.Param("key")

	// 禁止修改敏感配置
	if sensitiveKeys[key] {
		c.JSON(403, gin.H{"error": "cannot modify sensitive setting"})
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	s.db.SetSetting(key, req.Value)
	c.JSON(200, gin.H{"success": true})
}

// isSensitiveKey 检查是否为敏感配置
func isSensitiveKey(key string) bool {
	return sensitiveKeys[key] || strings.HasPrefix(key, "syncbuf:")
}
