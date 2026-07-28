package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// loginAttempt 登录尝试记录（用于速率限制）
type loginAttempt struct {
	mu        sync.Mutex
	count     int
	blocked   bool
	blockedAt time.Time
}
// maskToken 对令牌进行掩码处理，只显示前4位和后4位
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

func (s *APIServer) handleGetAPIToken(c *gin.Context) {
	token := s.db.GetSetting("api.token")
	if token == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Printf("ERROR: failed to generate random token: %v", err)
			c.JSON(500, gin.H{"error": "internal server error"})
			return
		}
		token = hex.EncodeToString(b)
		s.db.SetSetting("api.token", token)
	}
	c.JSON(200, gin.H{"token": maskToken(token)})
}

func (s *APIServer) handleRegenAPIToken(c *gin.Context) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("ERROR: failed to generate random token: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	token := hex.EncodeToString(b)
	s.db.SetSetting("api.token", token)
	c.JSON(200, gin.H{"token": token})
}



func (s *APIServer) handleLogin(c *gin.Context) {
	// 速率限制：每个 IP 5次失败后锁定30秒
	ip := c.ClientIP()
	val, _ := s.loginAttempts.LoadOrStore(ip, &loginAttempt{})
	attempt := val.(*loginAttempt)
	attempt.mu.Lock()
	if attempt.blocked && time.Since(attempt.blockedAt) < 30*time.Second {
		attempt.mu.Unlock()
		c.JSON(429, gin.H{"error": "too many requests, try again later"})
		return
	}
	if attempt.blocked && time.Since(attempt.blockedAt) >= 30*time.Second {
		attempt.count = 0
		attempt.blocked = false
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		attempt.mu.Unlock()
		log.Printf("bad request: %v", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	// 使用 config 中的密码（已处理环境变量和自动生成）
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.config.API.LoginPassword)) != 1 {
		attempt.count++
		if attempt.count >= 5 {
			attempt.blocked = true
			attempt.blockedAt = time.Now()
		}
		attempt.mu.Unlock()
		c.JSON(401, gin.H{"error": "invalid password"})
		return
	}

	// 登录成功，清零尝试次数
	attempt.count = 0
	attempt.blocked = false
	attempt.mu.Unlock()

	token, err := GenerateJWT(s.config.API.JWTSecret, s.config.API.JWTExpiryHours)
	if err != nil {
		log.Printf("ERROR: failed to generate JWT: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(200, gin.H{"token": token})
}

func (s *APIServer) handleChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	if len(req.NewPassword) < 8 {
		c.JSON(400, gin.H{"error": "密码长度至少8位"})
		return
	}

	// 验证旧密码
	if subtle.ConstantTimeCompare([]byte(req.OldPassword), []byte(s.config.API.LoginPassword)) != 1 {
		c.JSON(401, gin.H{"error": "old password is incorrect"})
		return
	}

	// 更新内存中的配置
	s.config.API.LoginPassword = req.NewPassword

	// 将当前 JWT token 加入黑名单，使旧 token 失效
	if token := extractBearerToken(c); token != "" {
		s.revokeJWT(token)
	}

	c.JSON(200, gin.H{"success": true})
}
