package api

import (
	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/database"
)
func (s *APIServer) handleListUsers(c *gin.Context) {
	sessions, err := s.db.GetAllUserSessions()
	if err != nil {
		s.log.Error("failed to list users", "error", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, gin.H{"users": sessions})
}

func (s *APIServer) handleGetUserContext(c *gin.Context) {
	userID := c.Param("id")
	session, err := s.db.GetUserSession(userID)
	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, session)
}

func (s *APIServer) handleSwitchUserBackend(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		BackendID string `json:"backend_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		s.log.Warn("bad request: switch backend", "user", userID, "error", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	session := database.UserSession{
		UserID:    userID,
		BackendID: req.BackendID,
		RouteMode: "single",
	}
	s.db.SaveUserSession(session)
	if err := s.router.SetUserBackend(userID, req.BackendID, s.adapters.ListIDs()); err != nil {
		s.log.Warn("switch user backend failed", "user", userID, "backend", req.BackendID, "error", err)
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleClearUserContext(c *gin.Context) {
	userID := c.Param("id")
	s.db.DeleteUserSession(userID)
	s.router.ClearUserBackend(userID)
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleGetUserRouteMode(c *gin.Context) {
	userID := c.Param("id")
	session, err := s.db.GetUserSession(userID)
	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, gin.H{"route_mode": session.RouteMode})
}

func (s *APIServer) handleSetUserRouteMode(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		RouteMode string `json:"route_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		s.log.Warn("bad request: set route mode", "user", userID, "error", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	session, _ := s.db.GetUserSession(userID)
	if session == nil {
		session = &database.UserSession{UserID: userID}
	}
	session.RouteMode = req.RouteMode
	s.db.SaveUserSession(*session)
	s.router.SetUserRouteMode(userID, req.RouteMode)

	c.JSON(200, gin.H{"success": true})
}
