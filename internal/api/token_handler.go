package api

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

func (s *APIServer) handleGetAPIToken(c *gin.Context) {
	token := s.db.GetSetting("api.token")
	if token == "" {
		b := make([]byte, 32)
		rand.Read(b)
		token = hex.EncodeToString(b)
		s.db.SetSetting("api.token", token)
	}
	c.JSON(200, gin.H{"token": token})
}

func (s *APIServer) handleRegenAPIToken(c *gin.Context) {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	s.db.SetSetting("api.token", token)
	c.JSON(200, gin.H{"token": token})
}

func (s *APIServer) handleLogin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 使用 config 中的密码（已处理环境变量和自动生成）
	if req.Password != s.config.API.LoginPassword {
		c.JSON(401, gin.H{"error": "invalid password"})
		return
	}

	token, err := GenerateJWT(s.config.API.JWTSecret, s.config.API.JWTExpiryHours)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"token": token})
}
