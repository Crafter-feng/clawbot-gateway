package api

import (
	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/route"
)

// ══════════════════════════════════════════════════════════════════════
//  用户会话管理
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) handleListUsers(c *gin.Context) {
	backends := s.router.GetAllUserBackends()
	users := make([]gin.H, 0)
	for uid, bid := range backends {
		users = append(users, gin.H{
			"user_id": uid,
			"backend": bid,
			"active":  true,
		})
	}
	c.JSON(200, gin.H{"users": users, "total": len(users)})
}

func (s *APIServer) handleGetUserContext(c *gin.Context) {
	userID := c.Param("id")
	backendID := c.Query("backend")
	if backendID == "" {
		c.JSON(400, gin.H{"error": "backend query parameter is required"})
		return
	}
	ctx := s.ctxManager.GetContext(userID, backendID)
	c.JSON(200, gin.H{
		"user_id":     userID,
		"backend":     backendID,
		"history":     ctx.History,
		"created_at":  ctx.CreatedAt,
		"last_active": ctx.LastActive,
	})
}

func (s *APIServer) handleSwitchUserBackend(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		BackendID string `json:"backend_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	oldBackend, hasOld := s.router.GetUserBackend(userID)
	s.router.SetUserBackend(userID, req.BackendID)
	if hasOld {
		s.ctxManager.SwitchBackend(userID, oldBackend, req.BackendID)
	}
	_ = s.store.SetUserBackend(userID, req.BackendID)
	c.JSON(200, gin.H{"success": true, "user_id": userID, "backend": req.BackendID})
}

func (s *APIServer) handleClearUserContext(c *gin.Context) {
	userID := c.Param("id")
	backendID := c.Query("backend")
	if backendID != "" {
		s.ctxManager.ClearContext(userID, backendID)
	} else {
		s.ctxManager.ClearAllUserContext(userID)
	}
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleGetUserRouteMode(c *gin.Context) {
	userID := c.Param("id")
	mode := s.router.GetUserRouteMode(userID)
	secondaries := s.router.GetUserSecondaries(userID)
	c.JSON(200, gin.H{
		"user_id":     userID,
		"mode":        mode,
		"secondaries": secondaries,
	})
}

func (s *APIServer) handleSetUserRouteMode(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Mode        string   `json:"mode"`
		Secondaries []string `json:"secondaries,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	switch req.Mode {
	case route.ModeSingle, route.ModeBoth, route.ModeThree:
		s.router.SetUserRouteMode(userID, req.Mode)
		if len(req.Secondaries) > 0 {
			s.router.SetUserSecondaries(userID, req.Secondaries)
			_ = s.store.SetUserSecondaries(userID, req.Secondaries)
		}
		_ = s.store.SetUserRouteMode(userID, req.Mode)
		c.JSON(200, gin.H{"success": true, "user_id": userID, "mode": req.Mode, "secondaries": req.Secondaries})
	default:
		c.JSON(400, gin.H{"error": "invalid mode, must be single/both/three"})
	}
}
