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
	c.JSON(200, gin.H{"token": token})
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
	attempt.mu.Unlock()

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 使用 config 中的密码（已处理环境变量和自动生成）
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.config.API.LoginPassword)) != 1 {
		attempt.mu.Lock()
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
	attempt.mu.Lock()
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

	if req.NewPassword == "" {
		c.JSON(400, gin.H{"error": "new password is required"})
		return
	}

	// 验证旧密码
	if subtle.ConstantTimeCompare([]byte(req.OldPassword), []byte(s.config.API.LoginPassword)) != 1 {
		c.JSON(401, gin.H{"error": "old password is incorrect"})
		return
	}

	// 更新数据库
	if err := s.db.SetSetting("api.login_password", req.NewPassword); err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	// 更新内存中的配置
	s.config.API.LoginPassword = req.NewPassword

	c.JSON(200, gin.H{"success": true})
}
