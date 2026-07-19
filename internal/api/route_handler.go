package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/store"
)

// ══════════════════════════════════════════════════════════════════════
//  路由规则管理
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) handleListRoutes(c *gin.Context) {
	rules := s.router.GetKeywordRules()
	c.JSON(200, gin.H{"routes": rules})
}

func (s *APIServer) handleAddRoute(c *gin.Context) {
	var req struct {
		Keyword  string `json:"keyword"`
		Backend  string `json:"backend"`
		IsRegexp bool   `json:"is_regexp,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Keyword == "" {
		c.JSON(400, gin.H{"error": "keyword must not be empty"})
		return
	}
	if req.Backend == "" {
		c.JSON(400, gin.H{"error": "backend must not be empty"})
		return
	}
	if err := s.router.AddKeywordRule(req.Keyword, req.Backend, req.IsRegexp); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	stored := s.store.GetKeywordRules()
	stored = append(stored, store.StoredRule{Keyword: req.Keyword, Backend: req.Backend, IsRegexp: req.IsRegexp})
	_ = s.store.SetKeywordRules(stored)
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleRemoveRoute(c *gin.Context) {
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid index"})
		return
	}
	if s.router.RemoveKeywordRule(index) {
		stored := s.store.GetKeywordRules()
		if index >= 0 && index < len(stored) {
			stored = append(stored[:index], stored[index+1:]...)
			_ = s.store.SetKeywordRules(stored)
		}
		c.JSON(200, gin.H{"success": true})
	} else {
		c.JSON(404, gin.H{"error": "rule not found"})
	}
}
