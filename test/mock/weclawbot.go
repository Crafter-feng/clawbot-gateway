package mock

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Message 记录的消息
type Message struct {
	BotID   string
	Token   string
	Text    string
	Headers http.Header
}

// WeClawBotMock WeClawBot-API 模拟器
type WeClawBotMock struct {
	mu       sync.RWMutex
	messages []Message
	server   *http.Server
	port     string
}

// NewWeClawBotMock 创建模拟器
func NewWeClawBotMock(port string) *WeClawBotMock {
	return &WeClawBotMock{
		messages: make([]Message, 0),
		port:     port,
	}
}

// Start 启动模拟服务器
func (m *WeClawBotMock) Start() error {
	mux := http.NewServeMux()

	// 发送消息接口
	mux.HandleFunc("/bots/", m.handleBotRequest)

	m.server = &http.Server{
		Addr:    ":" + m.port,
		Handler: mux,
	}

	go m.server.ListenAndServe()
	return nil
}

// Stop 停止模拟服务器
func (m *WeClawBotMock) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

// GetMessages 获取所有记录的消息
func (m *WeClawBotMock) GetMessages() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// ClearMessages 清空消息记录
func (m *WeClawBotMock) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]Message, 0)
}

// GetMessageCount 获取消息数量
func (m *WeClawBotMock) GetMessageCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// handleBotRequest 处理 Bot 请求
func (m *WeClawBotMock) handleBotRequest(w http.ResponseWriter, r *http.Request) {
	// 解析路径：/bots/{bot_id}/messages 或 /bots/{bot_id}/typing
	path := r.URL.Path

	// 验证 token
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	// 读取请求体
	var text string
	if r.Body != nil {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			text = body["text"]
		}
	}

	// 记录消息
	m.mu.Lock()
	m.messages = append(m.messages, Message{
		BotID:   path,
		Token:   token,
		Text:    text,
		Headers: r.Header,
	})
	m.mu.Unlock()

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    200,
		"message": "OK",
	})
}

// GetPort 获取端口号
func (m *WeClawBotMock) GetPort() string {
	return m.port
}

// GetURL 获取服务地址
func (m *WeClawBotMock) GetURL() string {
	return "http://localhost:" + m.port
}
