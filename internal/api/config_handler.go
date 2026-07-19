package api

import (
	"github.com/gin-gonic/gin"
)

func (s *APIServer) handleGetAllConfig(c *gin.Context) {
	settings := s.db.GetAllSettings()
	c.JSON(200, gin.H{"settings": settings})
}

func (s *APIServer) handleUpdateConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	for k, v := range req {
		s.db.SetSetting(k, v)
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleGetConfig(c *gin.Context) {
	key := c.Param("key")
	value := s.db.GetSetting(key)
	c.JSON(200, gin.H{"key": key, "value": value})
}

func (s *APIServer) handleSetConfig(c *gin.Context) {
	key := c.Param("key")
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
