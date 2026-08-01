package ilink

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/bot"
)

func TestServerSetForwardFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := bot.ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := bot.NewConnector(cfg)
	reg := NewClientRegistry()
	s := NewServer(conn, reg)

	if s.forwardFunc != nil {
		t.Error("forwardFunc should be nil initially")
	}

	fn := ForwardFunc(func(ctx context.Context, accountID, endpoint string, body []byte) ([]byte, int, error) {
		return []byte(`{"status":"ok"}`), 200, nil
	})
	s.SetForwardFunc(fn)
	if s.forwardFunc == nil {
		t.Error("forwardFunc should not be nil after SetForwardFunc")
	}
}

func TestServerHandleGetConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := bot.ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := bot.NewConnector(cfg)
	reg := NewClientRegistry()
	reg.Register("gw_test", "gw_test@im.wechat", "https://example.com", "test-token-123")

	s := NewServer(conn, reg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/ilink/bot/getconfig", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token-123")

	s.handleGetConfig(c)

	if w.Code != 200 {
		t.Fatalf("handleGetConfig status = %d, want 200", w.Code)
	}

	var resp struct {
		AccountID string `json:"account_id"`
		UserID    string `json:"user_id"`
		BaseURL   string `json:"base_url"`
		Token     string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if resp.AccountID != "gw_test" {
		t.Errorf("account_id want 'gw_test', got '%s'", resp.AccountID)
	}
	if resp.UserID != "gw_test@im.wechat" {
		t.Errorf("user_id want 'gw_test@im.wechat', got '%s'", resp.UserID)
	}
	if resp.BaseURL != "https://example.com" {
		t.Errorf("base_url want 'https://example.com', got '%s'", resp.BaseURL)
	}
	if resp.Token != "test-token-123" {
		t.Errorf("token want 'test-token-123', got '%s'", resp.Token)
	}
}

func TestServerHandleGetConfigUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := bot.ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := bot.NewConnector(cfg)
	reg := NewClientRegistry()
	s := NewServer(conn, reg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/ilink/bot/getconfig", nil)

	s.handleGetConfig(c)

	if w.Code != 401 {
		t.Errorf("handleGetConfig without auth status = %d, want 401", w.Code)
	}
}

func TestServerHandleForward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := bot.ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := bot.NewConnector(cfg)
	reg := NewClientRegistry()
	reg.Register("gw_test", "gw_test@im.wechat", "https://example.com", "test-token-456")

	s := NewServer(conn, reg)

	called := false
	s.SetForwardFunc(ForwardFunc(func(ctx context.Context, accountID, endpoint string, body []byte) ([]byte, int, error) {
		called = true
		if accountID != "gw_test" {
			t.Errorf("accountID want 'gw_test', got '%s'", accountID)
		}
		return []byte(`{"ret":0}`), 200, nil
	}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/ilink/bot/sendmessage",
		bytes.NewReader([]byte(`{"to_user_id":"user1","msg":{"text":"hello"}}`)))
	c.Request.Header.Set("Authorization", "Bearer test-token-456")

	s.handleSendMessage(c)

	if !called {
		t.Error("forwardFunc was not called")
	}
	if w.Code != 200 {
		t.Errorf("handleSendMessage status = %d, want 200", w.Code)
	}

	var resp struct {
		Ret int `json:"ret"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Ret != 0 {
		t.Errorf("ret want 0, got %d", resp.Ret)
	}
}

func TestServerHandleForwardNoForwardFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := bot.ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := bot.NewConnector(cfg)
	reg := NewClientRegistry()
	reg.Register("gw_test", "gw_test@im.wechat", "https://example.com", "test-token-789")

	s := NewServer(conn, reg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/ilink/bot/sendmessage", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token-789")

	s.handleSendMessage(c)

	if w.Code != 500 {
		t.Errorf("handleSendMessage without forward func status = %d, want 500", w.Code)
	}
}