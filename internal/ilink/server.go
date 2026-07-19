package ilink

import (
	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/log"
)

// Server iLink API 服务器
// 在同一端口提供 /ilink/bot/* 端点，让外部服务无缝接入
type Server struct {
	bot *bot.Connector
	log *log.Logger
}

func NewServer(bot *bot.Connector) *Server {
	return &Server{
		bot: bot,
		log: log.Default().WithComponent("ilink"),
	}
}

// RegisterRoutes 注册 iLink API 路由
func (s *Server) RegisterRoutes(r *gin.RouterGroup) {
	ilink := r.Group("/ilink/bot")
	{
		ilink.POST("/sendmessage", s.handleSendMessage)
		ilink.POST("/sendtyping", s.handleSendTyping)
		ilink.GET("/getupdates", s.handleGetUpdates)
		ilink.GET("/get_bot_qrcode", s.handleGetQRCode)
		ilink.GET("/get_qrcode_status", s.handleGetQRCodeStatus)
		ilink.POST("/getuploadurl", s.handleGetUploadURL)
	}
	s.log.Info("iLink API routes registered")
}
