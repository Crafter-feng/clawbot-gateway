package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/database"
)

func (s *APIServer) handleListRoutes(c *gin.Context) {
	routes, err := s.db.ListRoutes()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"routes": routes})
}

func (s *APIServer) handleAddRoute(c *gin.Context) {
	var req struct {
		Keyword   string `json:"keyword"`
		BackendID string `json:"backend_id"`
		IsRegexp  bool   `json:"is_regexp"`
		Priority  int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	route := database.Route{
		Keyword:   req.Keyword,
		BackendID: req.BackendID,
		IsRegexp:  req.IsRegexp,
		Priority:  req.Priority,
	}

	if err := s.db.CreateRoute(route); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// 同步到内存路由引擎
	s.router.AddKeywordRule(req.Keyword, req.BackendID, req.IsRegexp)

	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleUpdateRoute(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid route id"})
		return
	}

	var req struct {
		Keyword   string `json:"keyword"`
		BackendID string `json:"backend_id"`
		IsRegexp  bool   `json:"is_regexp"`
		Priority  int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	route := database.Route{
		ID:        id,
		Keyword:   req.Keyword,
		BackendID: req.BackendID,
		IsRegexp:  req.IsRegexp,
		Priority:  req.Priority,
	}

	if err := s.db.UpdateRoute(id, route); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleRemoveRoute(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid route id"})
		return
	}

	if err := s.db.DeleteRoute(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}
