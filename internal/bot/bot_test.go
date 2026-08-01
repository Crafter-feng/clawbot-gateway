package bot

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	m.Run()
}

func TestNewConnector(t *testing.T) {
	cfg := ConnectorConfig{
		BaseURL:     "https://ilinkai.weixin.qq.com",
		PollTimeout: 35,
	}
	conn := NewConnector(cfg)
	if conn == nil {
		t.Fatal("NewConnector returned nil")
	}
	if conn.IsRunning() {
		t.Error("NewConnector should not be running initially")
	}
	if conn.GetCredentials() != nil {
		t.Error("GetCredentials should return nil before AddAccount")
	}
}

func TestNewConnectorDefaultValues(t *testing.T) {
	cfg := ConnectorConfig{
		BaseURL: "https://example.com",
	}
	conn := NewConnector(cfg)
	if conn == nil {
		t.Fatal("NewConnector returned nil")
	}
}

func TestConnectorMessages(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	ch := conn.Messages()
	if ch == nil {
		t.Error("Messages() returned nil channel")
	}
}

func TestConnectorQRManager(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := conn.QRManager()
	if qm == nil {
		t.Error("QRManager() returned nil")
	}
}

func TestGenerateClientID(t *testing.T) {
	id1 := GenerateClientID()
	id2 := GenerateClientID()

	if id1 == "" {
		t.Error("GenerateClientID returned empty")
	}
	if id1 == id2 {
		t.Error("GenerateClientID should produce unique values")
	}
	if !strings.HasPrefix(id1, "openclaw-weixin:") {
		t.Errorf("GenerateClientID want prefix 'openclaw-weixin:', got '%s'", id1)
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   int
	}{
		{"short message", "hello", 100, 1},
		{"exact fit", "hello", 5, 1},
		{"needs split", "hello world", 5, 3}, // "hello" (5), " worl" (5), "d" (1)
		{"empty", "", 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitMessage(tt.text, tt.maxLen)
			if len(got) != tt.want {
				t.Errorf("SplitMessage length want %d, got %d: %v", tt.want, len(got), got)
			}
		})
	}
}

func TestSplitMessageJoin(t *testing.T) {
	text := "This is a long message that should be split into multiple parts"
	parts := SplitMessage(text, 20)
	joined := strings.Join(parts, "")
	if joined != text {
		t.Errorf("SplitMessage+Join want '%s', got '%s'", text, joined)
	}
}

func TestBuildBaseInfo(t *testing.T) {
	info := BuildBaseInfo()
	if info.ChannelVersion != "1.0.2" {
		t.Errorf("ChannelVersion want '1.0.2', got '%s'", info.ChannelVersion)
	}
}

func TestNormalizedMessage(t *testing.T) {
	msg := NormalizedMessage{
		MsgID:     "12345",
		FromUser:  "user1",
		ToUser:    "bot1",
		Content:   "hello",
		Type:      1,
		Timestamp: time.Now().Unix(),
	}
	if msg.MsgID != "12345" {
		t.Errorf("MsgID want '12345', got '%s'", msg.MsgID)
	}
	if msg.Content != "hello" {
		t.Errorf("Content want 'hello', got '%s'", msg.Content)
	}
	if msg.Type != 1 {
		t.Errorf("Type want 1, got %d", msg.Type)
	}
}

func TestCredentials(t *testing.T) {
	creds := &Credentials{
		Token:   "test-token",
		BaseURL: "https://example.com",
	}
	if creds.Token != "test-token" {
		t.Errorf("Token want 'test-token', got '%s'", creds.Token)
	}
	if creds.BaseURL != "https://example.com" {
		t.Errorf("BaseURL want 'https://example.com', got '%s'", creds.BaseURL)
	}
}

func TestConnectorContextToken(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)

	// Set and get context token
	conn.SetContextToken("account1", "user1", "ctx-token-123")
	token := conn.GetContextToken("account1", "user1")
	if token != "ctx-token-123" {
		t.Errorf("GetContextToken want 'ctx-token-123', got '%s'", token)
	}

	// Non-existent key should return empty
	token = conn.GetContextToken("account1", "nonexistent")
	if token != "" {
		t.Errorf("GetContextToken for nonexistent want empty, got '%s'", token)
	}
}

func TestConnectorSyncBufStore(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	// SetSyncBufStore should not panic
	conn.SetSyncBufStore(nil)
}

func TestNewQRCodeManager(t *testing.T) {
	cfg := ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := NewConnector(cfg)
	qm := NewQRCodeManager(conn)
	if qm == nil {
		t.Fatal("NewQRCodeManager returned nil")
	}
}