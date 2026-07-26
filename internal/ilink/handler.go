package ilink

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleProxy 透明代理 - 直接转发到真实 iLink API
func (s *Server) handleProxy(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}

	// 2. 获取虚拟 Bot 的配置
	vbot := s.registry.Get(accountID)
	if vbot == nil {
		c.JSON(400, gin.H{"ret": -1, "errmsg": "bot not registered"})
		return
	}

	// 3. 获取真实微信账号的凭证（用于转发到真实 iLink API）
	realBotToken := s.bot.GetAccountTokenByVirtualID(accountID)
	if realBotToken == "" {
		c.JSON(500, gin.H{"ret": -1, "errmsg": "no real bot credentials found"})
		return
	}

	// 4. 读取原始请求体
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(500, gin.H{"ret": -1, "errmsg": "failed to read request body"})
		return
	}

	// 5. 转发到真实 iLink API
	endpoint := c.FullPath()
	resp, err := s.forwardToILink(endpoint, body, vbot.BaseURL, realBotToken)
	if err != nil {
		s.log.Warn("proxy forward failed", "error", err)
		c.JSON(502, gin.H{"ret": -1, "errmsg": err.Error()})
		return
	}
	defer resp.Body.Close()

	// 6. 直接返回响应（透明管道）
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", respBody)
}

// forwardToILink 转发请求到真实 iLink API（透明代理）
func (s *Server) forwardToILink(endpoint string, body []byte, baseURL string, botToken string) (*http.Response, error) {
	// c.FullPath() 返回带前导斜杠的路径（如 /ilink/bot/getupdates）
	// 拼接时去掉 endpoint 的前导斜杠，避免双斜杠
	url := baseURL + strings.TrimLeft(endpoint, "/")

	// 使用 bytes.NewReader(body) 确保请求体被正确转发
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 设置请求头（与 Connector 保持一致）
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("iLink-App-Id", "bot")
	req.Header.Set("iLink-App-ClientVersion", "131584")

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
