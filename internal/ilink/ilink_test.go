package ilink

import (
	"os"
	"sync"
	"testing"
	"time"

	"clawbot-gateway/internal/bot"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	m.Run()
}

func TestNewClientRegistry(t *testing.T) {
	reg := NewClientRegistry()
	if reg == nil {
		t.Fatal("NewClientRegistry returned nil")
	}
	if reg.Count() != 0 {
		t.Errorf("Count want 0, got %d", reg.Count())
	}
}

func TestClientRegistryRegister(t *testing.T) {
	reg := NewClientRegistry()

	vbot := reg.Register("gw_test", "gw_test@im.wechat", "https://ilinkai.weixin.qq.com", "")
	if vbot == nil {
		t.Fatal("Register returned nil")
	}
	if vbot.AccountID != "gw_test" {
		t.Errorf("AccountID want 'gw_test', got '%s'", vbot.AccountID)
	}
	if vbot.UserID != "gw_test@im.wechat" {
		t.Errorf("UserID want 'gw_test@im.wechat', got '%s'", vbot.UserID)
	}
	if vbot.BaseURL != "https://ilinkai.weixin.qq.com" {
		t.Errorf("BaseURL want 'https://ilinkai.weixin.qq.com', got '%s'", vbot.BaseURL)
	}
}

func TestClientRegistryGet(t *testing.T) {
	reg := NewClientRegistry()
	reg.Register("gw_test", "gw_test@im.wechat", "https://ilinkai.weixin.qq.com", "")

	vbot := reg.Get("gw_test")
	if vbot == nil {
		t.Fatal("Get returned nil for existing bot")
	}
	if vbot.AccountID != "gw_test" {
		t.Errorf("AccountID want 'gw_test', got '%s'", vbot.AccountID)
	}

	// Non-existent
	vbot = reg.Get("nonexistent")
	if vbot != nil {
		t.Error("Get for nonexistent should return nil")
	}
}

func TestClientRegistryGetByToken(t *testing.T) {
	reg := NewClientRegistry()
	vbot := reg.Register("gw_test", "gw_test@im.wechat", "https://ilinkai.weixin.qq.com", "")

	found := reg.GetByToken(vbot.Token)
	if found == nil {
		t.Fatal("GetByToken returned nil for existing token")
	}
	if found.AccountID != "gw_test" {
		t.Errorf("AccountID want 'gw_test', got '%s'", found.AccountID)
	}

	// Non-existent token
	found = reg.GetByToken("nonexistent")
	if found != nil {
		t.Error("GetByToken for nonexistent should return nil")
	}
}

func TestClientRegistryList(t *testing.T) {
	reg := NewClientRegistry()

	bots := reg.List()
	if len(bots) != 0 {
		t.Errorf("List want 0, got %d", len(bots))
	}

	reg.Register("gw_a", "gw_a@im.wechat", "https://a.com", "")
	reg.Register("gw_b", "gw_b@im.wechat", "https://b.com", "")

	bots = reg.List()
	if len(bots) != 2 {
		t.Errorf("List want 2, got %d", len(bots))
	}
}

func TestClientRegistryCount(t *testing.T) {
	reg := NewClientRegistry()
	if reg.Count() != 0 {
		t.Errorf("Count want 0, got %d", reg.Count())
	}

	reg.Register("gw_a", "gw_a@im.wechat", "https://a.com", "")
	if reg.Count() != 1 {
		t.Errorf("Count want 1, got %d", reg.Count())
	}
}

func TestClientRegistryUnregister(t *testing.T) {
	reg := NewClientRegistry()
	reg.Register("gw_test", "gw_test@im.wechat", "https://ilinkai.weixin.qq.com", "")
	if reg.Count() != 1 {
		t.Fatalf("Count want 1, got %d", reg.Count())
	}

	reg.Unregister("gw_test")
	if reg.Count() != 0 {
		t.Errorf("Count after Unregister want 0, got %d", reg.Count())
	}
	if reg.Get("gw_test") != nil {
		t.Error("Get after Unregister should return nil")
	}
}

func TestClientRegistryUpdateLastActive(t *testing.T) {
	reg := NewClientRegistry()
	vbot := reg.Register("gw_test", "gw_test@im.wechat", "https://ilinkai.weixin.qq.com", "")

	original := vbot.LastActive
	time.Sleep(time.Millisecond)
	reg.UpdateLastActive("gw_test")

	if vbot.LastActive == original {
		t.Error("UpdateLastActive should update LastActive timestamp")
	}
}

func TestClientRegistryGetStats(t *testing.T) {
	reg := NewClientRegistry()
	reg.Register("gw_a", "gw_a@im.wechat", "https://a.com", "")
	reg.Register("gw_b", "gw_b@im.wechat", "https://b.com", "")

	stats := reg.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats["total_bots"] != 2 {
		t.Errorf("stats['total_bots'] want 2, got %d", stats["total_bots"])
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	defer rl.Stop()
}

func TestNewRateLimiterStop(t *testing.T) {
	rl := NewRateLimiter(100, 200)
	// Stop should not panic
	rl.Stop()
	// Double stop should not panic
	rl.Stop()
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(100, 100)
	defer rl.Stop()

	// First requests should be allowed
	if !rl.Allow("test-key") {
		t.Error("Allow should return true for first request")
	}
}

func TestRateLimiterAllowBurst(t *testing.T) {
	// Create a limiter with burst=3
	rl := NewRateLimiter(100, 3)
	defer rl.Stop()

	// First 3 should be allowed (burst capacity)
	if !rl.Allow("burst-test") {
		t.Error("Allow should return true for first request")
	}
	if !rl.Allow("burst-test") {
		t.Error("Allow should return true for second request")
	}
	if !rl.Allow("burst-test") {
		t.Error("Allow should return true for third request")
	}
}

func TestRateLimiterSeparateKeys(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	// Different keys should not interfere
	if !rl.Allow("key-a") {
		t.Error("Allow for key-a should return true")
	}
	if !rl.Allow("key-b") {
		t.Error("Allow for key-b should return true")
	}
}

func TestNewServer(t *testing.T) {
	cfg := bot.ConnectorConfig{BaseURL: "https://ilinkai.weixin.qq.com"}
	conn := bot.NewConnector(cfg)
	reg := NewClientRegistry()

	s := NewServer(conn, reg)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.GetRegistry() != reg {
		t.Error("GetRegistry should return the same registry")
	}
}

func TestMaxRequestBodySize(t *testing.T) {
	if MaxRequestBodySize != 1*1024*1024 {
		t.Errorf("MaxRequestBodySize want %d, got %d", 1*1024*1024, MaxRequestBodySize)
	}
}

func TestVirtualBotStruct(t *testing.T) {
	vbot := &VirtualBot{
		AccountID: "gw_test",
		UserID:    "gw_test@im.wechat",
		BaseURL:   "https://example.com",
	}
	if vbot.AccountID != "gw_test" {
		t.Errorf("AccountID want 'gw_test', got '%s'", vbot.AccountID)
	}
	if vbot.UserID != "gw_test@im.wechat" {
		t.Errorf("UserID want 'gw_test@im.wechat', got '%s'", vbot.UserID)
	}
	if vbot.BaseURL != "https://example.com" {
		t.Errorf("BaseURL want 'https://example.com', got '%s'", vbot.BaseURL)
	}
}

// TestConcurrentRegistryAccess tests thread safety of ClientRegistry
func TestConcurrentRegistryAccess(t *testing.T) {
	reg := NewClientRegistry()
	var wg sync.WaitGroup

	// Concurrent registration
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			accountID := "gw_test_" + string(rune('a'+id))
			reg.Register(accountID, accountID+"@im.wechat", "https://example.com", "")
		}(i)
	}
	wg.Wait()

	if reg.Count() != 10 {
		t.Errorf("Count after 10 concurrent registers want 10, got %d", reg.Count())
	}
}