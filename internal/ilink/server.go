package ilink

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/log"
)

// Server iLink API 服务器
// 在同一端口提供 /ilink/bot/* 端点，让外部服务无缝接入
type Server struct {
	bot      *bot.Connector
	registry *ClientRegistry
	limiter  *RateLimiter
	log      *log.Logger
}

// NewServer 创建 iLink API 服务器
func NewServer(bot *bot.Connector, registry *ClientRegistry) *Server {
	return &Server{
		bot:      bot,
		registry: registry,
		limiter:  NewRateLimiter(10, 20), // 每秒 10 个请求，突发 20 个
		log:      log.Default().WithComponent("ilink"),
	}
}

// GetRegistry 获取客户端注册表
func (s *Server) GetRegistry() *ClientRegistry {
	return s.registry
}

// RegisterRoutes 注册 iLink API 路由
func (s *Server) RegisterRoutes(r *gin.Engine) {
	ilink := r.Group("/ilink/bot")
	{
		// 应用速率限制中间件
		ilink.Use(s.rateLimitMiddleware())
		// 应用请求体大小限制
		ilink.Use(maxBodySizeMiddleware(MaxRequestBodySize))

		ilink.POST("/getupdates", s.handleGetUpdates)
		ilink.POST("/sendmessage", s.handleSendMessage)
		ilink.POST("/sendtyping", s.handleSendTyping)
		ilink.POST("/getconfig", s.handleGetConfig)
		ilink.GET("/get_bot_qrcode", s.handleGetQRCode)
		ilink.GET("/get_qrcode_status", s.handleGetQRCodeStatus)
		ilink.POST("/getuploadurl", s.handleGetUploadURL)
	}
	s.log.Info("iLink API routes registered")
}

// rateLimitMiddleware 速率限制中间件
func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if accountID := s.validateToken(c); accountID != "" {
			key = accountID
		}

		if !s.limiter.Allow(key) {
			c.JSON(429, gin.H{
				"ret":    -1,
				"errmsg": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// maxBodySizeMiddleware 请求体大小限制中间件
func maxBodySizeMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}
