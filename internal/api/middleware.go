package api

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/log"
)

// debugPaths 高频低价值路径，使用 Debug 级别
var debugPaths = map[string]bool{
	"/health":       true,
	"/api/v1/stats": true,
}

// GinMiddleware 返回 gin 请求日志中间件
func GinMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Duration("latency", latency),
			slog.String("ip", c.ClientIP()),
		}
		msg := fmt.Sprintf("%s %s", c.Request.Method, path)
		if query != "" {
			msg += "?" + c.Request.URL.Query().Encode()
		}

		if debugPaths[path] {
			logger.LogAttrs(nil, slog.LevelDebug, msg, attrs...)
			return
		}

		switch {
		case status >= 500:
			logger.LogAttrs(nil, slog.LevelError, msg, attrs...)
		case status >= 400:
			logger.LogAttrs(nil, slog.LevelWarn, msg, attrs...)
		default:
			logger.LogAttrs(nil, slog.LevelInfo, msg, attrs...)
		}
	}
}