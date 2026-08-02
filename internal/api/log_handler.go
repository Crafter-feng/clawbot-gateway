package api

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

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
	since := c.Query("since") // ISO8601 时间戳，只返回该时间之后的条目
	q := c.Query("q")         // 关键词搜索（匹配 message 和 attrs 值）

	entries := buf.Entries(limit, level, component, backend)

	// 按时间过滤
	if since != "" {
		filtered := make([]log.Entry, 0, len(entries))
		for _, e := range entries {
			if e.Time > since {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// 关键词搜索
	if q != "" {
		q = strings.ToLower(q)
		filtered := make([]log.Entry, 0, len(entries))
	outer:
		for _, e := range entries {
			if strings.Contains(strings.ToLower(e.Message), q) {
				filtered = append(filtered, e)
				continue
			}
			if attrs, ok := e.Attrs.(map[string]interface{}); ok {
				for _, v := range attrs {
					if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), q) {
						filtered = append(filtered, e)
						continue outer
					}
				}
			}
		}
		entries = filtered
	}

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

// handleLogStream SSE 实时日志推送
func (s *APIServer) handleLogStream(c *gin.Context) {
	buf := log.GetBuffer()
	if buf == nil {
		c.JSON(500, gin.H{"error": "log buffer not available"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		c.JSON(500, gin.H{"error": "streaming not supported"})
		return
	}

	ch := buf.Subscribe()
	defer buf.Unsubscribe(ch)

	c.Stream(func(w io.Writer) bool {
		select {
		case entry, ok := <-ch:
			if !ok {
				return false
			}
			data, err := json.Marshal(entry)
			if err != nil {
				return true
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}