package ilink

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
)

// handleProxy 透明代理 - 直接转发到真实 iLink API
func (s *Server) handleProxy(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	// 更新最后活跃时间（用于判断连接状态）
	s.registry.UpdateLastActive(accountID)

	// 2. 获取虚拟 Bot 的配置
	vbot := s.registry.Get(accountID)
	if vbot == nil {
		c.JSON(500, gin.H{"error": "bot not found"})
		return
	}

	// 3. 获取真实微信账号的凭证（用于转发到真实 iLink API）
	realBotToken := s.bot.GetAccountTokenByVirtualID(accountID)
	if realBotToken == "" {
		c.JSON(500, gin.H{"error": "no real bot token"})
		return
	}

	// 4. 读取原始请求体
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(500, gin.H{"error": "read body error"})
		return
	}

	// 5. 转发到真实 iLink API
	endpoint := c.FullPath()
	resp, err := s.forwardToILink(endpoint, body, vbot.BaseURL, realBotToken)
	if err != nil {
		s.log.Error("forward error", "error", err, "endpoint", endpoint, "account_id", accountID)
		c.JSON(502, gin.H{"error": "forward error"})
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
	// 确保 baseURL 无尾随斜杠 + endpoint 有前导斜杠 = 正确 URL
	url := strings.TrimRight(baseURL, "/") + endpoint

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

	// 使用 Server 的共享 HTTP 客户端
	return s.httpClient.Do(req)
}

// handleGetUpdates 从虚拟 Bot 消息队列消费消息（长轮询）
// 不直接转发到腾讯 iLink API；消息由 Connector 独占轮询后经 Pipeline 路由入队
func (s *Server) handleGetUpdates(c *gin.Context) {
	// 1. 验证虚拟 Bot token
	accountID := s.validateToken(c)
	if accountID == "" {
		c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
		return
	}
	s.registry.UpdateLastActive(accountID)

	// 2. 获取虚拟 Bot 配置
	vbot := s.registry.Get(accountID)
	if vbot == nil {
		c.JSON(500, gin.H{"ret": -1, "errmsg": "bot not found"})
		return
	}

	// 3. 阻塞等待队列中的消息（长轮询，最长等待 35 秒）
	// 外部客户端可能在请求体中传入 get_updates_buf，我们忽略（管道侧自管理游标）
	_, _ = io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))

	msgs, timedOut := vbot.Dequeue(35 * time.Second)

	// 4. 返回 iLink 兼容响应格式
	// get_updates_buf 由外部客户端自管理，这里返回占位
	result := struct {
		Ret           int                   `json:"ret"`
		Errmsg        string                `json:"errmsg,omitempty"`
		Msgs          []bot.RawMessageItem `json:"msgs"`
		GetUpdatesBuf string                `json:"get_updates_buf"`
	}{
		Ret:           0,
		Msgs:          msgs,
		GetUpdatesBuf: "",
	}
	resultBytes, _ := json.Marshal(result)
	_ = timedOut // 不区分超时/正常返回，均为 200

	c.Data(200, "application/json", resultBytes)
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
