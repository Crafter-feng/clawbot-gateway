package ilink

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/log"
)

// ForwardFunc 处理虚拟 Bot 的请求转发（由 api 包设置，通过 Connector 处理）
type ForwardFunc func(ctx context.Context, accountID, endpoint string, body []byte) ([]byte, int, error)

// Server iLink API 服务器
// 不直接连接腾讯 iLink API，所有消息经 Pipeline 路由到 Connector
type Server struct {
	bot         *bot.Connector
	registry    *ClientRegistry
	limiter     *RateLimiter
	log         *log.Logger
	forwardFunc ForwardFunc
}

// NewServer 创建 iLink API 服务器
func NewServer(bot *bot.Connector, registry *ClientRegistry) *Server {
	return &Server{
		bot:      bot,
		registry: registry,
		limiter:  NewRateLimiter(10, 20),
		log:      log.Default().WithComponent("ilink"),
	}
}

// SetForwardFunc 设置请求转发函数（由 api 包调用，通过 Connector 转发）
func (s *Server) SetForwardFunc(fn ForwardFunc) {
	s.forwardFunc = fn
}

// GetRegistry 获取客户端注册表
func (s *Server) GetRegistry() *ClientRegistry {
	return s.registry
}

// Stop 停止 iLink 服务器，释放资源
func (s *Server) Stop() {
	s.limiter.Stop()
}

// validateToken 验证虚拟 Bot token
func (s *Server) validateToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		token := auth[7:]
		if bot := s.registry.GetByToken(token); bot != nil {
			return bot.AccountID
		}
	}
	return ""
}

// RegisterRoutes 注册 iLink API 路由
func (s *Server) RegisterRoutes(r *gin.Engine) {
	ilink := r.Group("/ilink/bot")
	{
		ilink.Use(s.rateLimitMiddleware())
		ilink.Use(maxBodySizeMiddleware(MaxRequestBodySize))

		ilink.POST("/getupdates", s.handleGetUpdates)
		ilink.POST("/sendmessage", s.handleSendMessage)
		ilink.POST("/sendtyping", s.handleSendTyping)
		ilink.POST("/getconfig", s.handleGetConfig)
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
			c.JSON(429, gin.H{"error": "rate limit exceeded"})
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
