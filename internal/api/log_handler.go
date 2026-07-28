package api

import (
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/log"
)

func (s *APIServer) handleGetLogs(c *gin.Context) {
	buf := log.GetBuffer()
	if buf == nil {
		c.JSON(200, gin.H{"entries": []log.Entry{}, "total": 0})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	level := c.Query("level")
	component := c.Query("component")
	backend := c.Query("backend")

	entries := buf.Entries(limit, level, component, backend)
	c.JSON(200, gin.H{
		"entries": entries,
		"total":   len(entries),
	})
}

func (s *APIServer) handleGetLogCategories(c *gin.Context) {
	buf := log.GetBuffer()
	if buf == nil {
		c.JSON(200, gin.H{"components": []string{}, "backends": []string{}})
		return
	}

	components, backends := buf.GetComponents()
	sort.Strings(components)
	sort.Strings(backends)

	c.JSON(200, gin.H{
		"components": components,
		"backends":   backends,
	})
}