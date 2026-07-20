package ilink

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// iLink 协议端点列表
var ilinkEndpoints = map[string]bool{
	"ilink/bot/getupdates":        true,
	"ilink/bot/sendmessage":       true,
	"ilink/bot/sendtyping":        true,
	"ilink/bot/getconfig":         true,
	"ilink/bot/getuploadurl":      true,
	"ilink/bot/get_bot_qrcode":    true,
	"ilink/bot/get_qrcode_status": true,
}

// handleProxy 透明代理 - 直接转发到真实 iLink API
func (s *Server) handleProxy(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}

	// 2. 获取虚拟 Bot 的 base URL
	bot := s.registry.Get(accountID)
	if bot == nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "bot not registered"})
		return
	}

	// 3. 读取原始请求体
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(500, gin.H{"ret": -1, "errmsg": "failed to read request body"})
		return
	}

	// 4. 转发到真实 iLink API
	endpoint := c.FullPath()
	resp, err := s.forwardToILink(endpoint, body, bot.BaseURL)
	if err != nil {
		s.log.Warn("proxy forward failed", "error", err)
		c.JSON(502, gin.H{"ret": -1, "errmsg": err.Error()})
		return
	}
	defer resp.Body.Close()

	// 5. 直接返回响应（透明管道）
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", respBody)
}

// forwardToILink 转发请求到真实 iLink API
func (s *Server) forwardToILink(endpoint string, body []byte, baseURL string) (*http.Response, error) {
	url := baseURL + "/" + endpoint

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	// 复制原始请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("iLink-App-Id", "")
	req.Header.Set("iLink-App-ClientVersion", "65547")

	// 发送请求
	client := &http.Client{Timeout: 60 * time.Second}
	return client.Do(req)
}

// handleGetUpdates 透明代理 - 长轮询
func (s *Server) handleGetUpdates(c *gin.Context) {
	s.handleProxy(c)
}

// handleSendMessage 透明代理 - 发送消息
func (s *Server) handleSendMessage(c *gin.Context) {
	s.handleProxy(c)
}

// handleSendTyping 透明代理 - 输入状态
func (s *Server) handleSendTyping(c *gin.Context) {
	s.handleProxy(c)
}

// handleGetConfig 透明代理 - 获取配置
func (s *Server) handleGetConfig(c *gin.Context) {
	s.handleProxy(c)
}

// handleGetUploadURL 透明代理 - 获取上传 URL
func (s *Server) handleGetUploadURL(c *gin.Context) {
	s.handleProxy(c)
}
