package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/gin-gonic/gin"
)

func generateToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("cb_fallback_%x", len(b))
	}
	return "cb_" + hex.EncodeToString(b)
}

// ══════════════════════════════════════════════════════════════════════
//  API Token 管理（需要鉴权）
// ══════════════════════════════════════════════════════════════════════

// handleGetAPIToken 获取当前 API Token
func (s *APIServer) handleGetAPIToken(c *gin.Context) {
	token := s.store.GetAPIToken()
	if token == "" {
		token = generateToken()
		if err := s.store.SetAPIToken(token); err != nil {
			s.log.Warn("failed to save initial api token", "error", err)
		}
	}
	c.JSON(200, gin.H{
		"token":  token,
		"header": "Authorization: Bearer " + token,
	})
}

// handleRegenAPIToken 重新生成 API Token（旧 token 立即失效）
func (s *APIServer) handleRegenAPIToken(c *gin.Context) {
	newToken := generateToken()
	if err := s.store.SetAPIToken(newToken); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"token":   newToken,
		"header":  "Authorization: Bearer " + newToken,
		"message": "token regenerated, old token is invalid",
	})
}
