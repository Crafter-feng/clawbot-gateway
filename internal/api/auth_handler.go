package api

import (
	"crypto/subtle"
	"time"

	"github.com/gin-gonic/gin"
)

// handleLogin 管理后台登录（密码验证 → 签发 JWT）
func (s *APIServer) handleLogin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid request"})
		return
	}
	if req.Password == "" {
		c.JSON(400, gin.H{"success": false, "error": "password must not be empty"})
		return
	}

	expected := s.config.API.LoginPassword
	if expected == "" {
		c.JSON(500, gin.H{"success": false, "error": "no login password configured"})
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(expected)) != 1 {
		c.JSON(401, gin.H{"success": false, "error": "invalid password"})
		return
	}

	// 签发 JWT
	expiry := time.Duration(s.config.API.JWTExpiryHours) * time.Hour
	jwt, err := SignJWT(s.config.API.JWTSecret, expiry)
	if err != nil {
		s.log.Error("failed to sign JWT", "error", err)
		c.JSON(500, gin.H{"success": false, "error": "failed to generate token"})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"token":   jwt,
	})
}
