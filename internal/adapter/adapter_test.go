package adapter

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	m.Run()
}

func TestNewAdapterFactory(t *testing.T) {
	f := NewAdapterFactory()
	if f == nil {
		t.Fatal("NewAdapterFactory returned nil")
	}
	bots := f.List()
	if len(bots) != 0 {
		t.Errorf("List want 0, got %d", len(bots))
	}
}

func TestAdapterFactoryRegisterAndGet(t *testing.T) {
	f := NewAdapterFactory()
	adapter := NewEchoAdapter("echo-1", "Echo Adapter 1")
	f.Register(adapter)

	got, ok := f.Get("echo-1")
	if !ok {
		t.Fatal("Get returned false for registered adapter")
	}
	if got.ID() != "echo-1" {
		t.Errorf("ID want 'echo-1', got '%s'", got.ID())
	}
	if got.Name() != "Echo Adapter 1" {
		t.Errorf("Name want 'Echo Adapter 1', got '%s'", got.Name())
	}
	if got.Type() != "echo" {
		t.Errorf("Type want 'echo', got '%s'", got.Type())
	}
}

func TestAdapterFactoryGetNonExistent(t *testing.T) {
	f := NewAdapterFactory()
	_, ok := f.Get("nonexistent")
	if ok {
		t.Error("Get for nonexistent should return false")
	}
}

func TestAdapterFactoryRemove(t *testing.T) {
	f := NewAdapterFactory()
	f.Register(NewEchoAdapter("echo-1", "Echo 1"))
	f.Remove("echo-1")

	_, ok := f.Get("echo-1")
	if ok {
		t.Error("Get after Remove should return false")
	}
}

func TestAdapterFactoryList(t *testing.T) {
	f := NewAdapterFactory()
	f.Register(NewEchoAdapter("echo-1", "Echo 1"))
	f.Register(NewEchoAdapter("echo-2", "Echo 2"))

	bots := f.List()
	if len(bots) != 2 {
		t.Errorf("List want 2, got %d", len(bots))
	}
}

func TestAdapterFactoryHealthyList(t *testing.T) {
	f := NewAdapterFactory()
	f.Register(NewEchoAdapter("echo-1", "Echo 1"))

	healthy := f.HealthyList()
	if len(healthy) != 1 {
		t.Errorf("HealthyList want 1, got %d", len(healthy))
	}
}

func TestAdapterFactoryRegisterConnection(t *testing.T) {
	f := NewAdapterFactory()
	conn := NewILinkProxyAdapter("conn-1", "Conn 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	f.RegisterConnection(conn)

	got, ok := f.GetConnection("conn-1")
	if !ok {
		t.Fatal("GetConnection returned false for registered connection")
	}
	if got.ID() != "conn-1" {
		t.Errorf("ID want 'conn-1', got '%s'", got.ID())
	}
	if got.Name() != "Conn 1" {
		t.Errorf("Name want 'Conn 1', got '%s'", got.Name())
	}
	if got.Type() != "ilink_proxy" {
		t.Errorf("Type want 'ilink_proxy', got '%s'", got.Type())
	}
}

func TestAdapterFactoryGetConnectionNonExistent(t *testing.T) {
	f := NewAdapterFactory()
	_, ok := f.GetConnection("nonexistent")
	if ok {
		t.Error("GetConnection for nonexistent should return false")
	}
}

func TestAdapterFactoryRemoveConnection(t *testing.T) {
	f := NewAdapterFactory()
	conn := NewILinkProxyAdapter("conn-1", "Conn 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	f.RegisterConnection(conn)
	f.RemoveConnection("conn-1")

	_, ok := f.GetConnection("conn-1")
	if ok {
		t.Error("GetConnection after RemoveConnection should return false")
	}
}

func TestAdapterFactoryListConnections(t *testing.T) {
	f := NewAdapterFactory()
	f.RegisterConnection(NewILinkProxyAdapter("conn-1", "Conn 1", "gw_a", "gw_a@im.wechat", "https://a.com"))
	f.RegisterConnection(NewILinkProxyAdapter("conn-2", "Conn 2", "gw_b", "gw_b@im.wechat", "https://b.com"))

	conns := f.ListConnections()
	if len(conns) != 2 {
		t.Errorf("ListConnections want 2, got %d", len(conns))
	}
}

func TestNewEchoAdapter(t *testing.T) {
	a := NewEchoAdapter("echo-1", "Echo Adapter")
	if a == nil {
		t.Fatal("NewEchoAdapter returned nil")
	}
	if a.ID() != "echo-1" {
		t.Errorf("ID want 'echo-1', got '%s'", a.ID())
	}
	if a.Name() != "Echo Adapter" {
		t.Errorf("Name want 'Echo Adapter', got '%s'", a.Name())
	}
	if a.Type() != "echo" {
		t.Errorf("Type want 'echo', got '%s'", a.Type())
	}
}

func TestEchoAdapterHealthCheck(t *testing.T) {
	a := NewEchoAdapter("echo-1", "Echo")
	if !a.HealthCheck(context.Background()) {
		t.Error("EchoAdapter HealthCheck should return true")
	}
}

func TestEchoAdapterHandle(t *testing.T) {
	a := NewEchoAdapter("echo-1", "Echo")
	req := &ChatRequest{
		Message: "Hello, world!",
		UserID:  "user1",
	}
	resp, err := a.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !strings.Contains(resp.Text, "Hello, world!") {
		t.Errorf("Handle response should contain input message, got '%s'", resp.Text)
	}
	if resp.Backend != "echo-1" {
		t.Errorf("Backend want 'echo-1', got '%s'", resp.Backend)
	}
}

func TestEchoAdapterHandleStream(t *testing.T) {
	a := NewEchoAdapter("echo-1", "Echo")
	req := &ChatRequest{
		Message: "Hello",
		UserID:  "user1",
	}
	ch := make(chan string, 10)
	err := a.HandleStream(context.Background(), req, ch)
	if err != nil {
		t.Fatalf("HandleStream failed: %v", err)
	}
	var result string
	for msg := range ch {
		result += msg
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("HandleStream response should contain input message, got '%s'", result)
	}
}

func TestNewILinkProxyAdapter(t *testing.T) {
	a := NewILinkProxyAdapter("proxy-1", "Proxy 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	if a == nil {
		t.Fatal("NewILinkProxyAdapter returned nil")
	}
	if a.ID() != "proxy-1" {
		t.Errorf("ID want 'proxy-1', got '%s'", a.ID())
	}
	if a.Name() != "Proxy 1" {
		t.Errorf("Name want 'Proxy 1', got '%s'", a.Name())
	}
	if a.Type() != "ilink_proxy" {
		t.Errorf("Type want 'ilink_proxy', got '%s'", a.Type())
	}
}

func TestILinkProxyAdapterGetConnectionInfo(t *testing.T) {
	a := NewILinkProxyAdapter("proxy-1", "Proxy 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	info := a.GetConnectionInfo()
	if info == nil {
		t.Fatal("GetConnectionInfo returned nil")
	}
	if info.AccountID != "gw_test" {
		t.Errorf("AccountID want 'gw_test', got '%s'", info.AccountID)
	}
	if info.UserID != "gw_test@im.wechat" {
		t.Errorf("UserID want 'gw_test@im.wechat', got '%s'", info.UserID)
	}
	if info.BaseURL != "https://example.com" {
		t.Errorf("BaseURL want 'https://example.com', got '%s'", info.BaseURL)
	}
}

func TestILinkProxyAdapterHealthCheck(t *testing.T) {
	a := NewILinkProxyAdapter("proxy-1", "Proxy 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	if !a.HealthCheck(context.Background()) {
		t.Error("ILinkProxyAdapter HealthCheck should return true")
	}
}

func TestILinkProxyAdapterGetAccountID(t *testing.T) {
	a := NewILinkProxyAdapter("proxy-1", "Proxy 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	if a.GetAccountID() != "gw_test" {
		t.Errorf("GetAccountID want 'gw_test', got '%s'", a.GetAccountID())
	}
}

func TestILinkProxyAdapterGetUserID(t *testing.T) {
	a := NewILinkProxyAdapter("proxy-1", "Proxy 1", "gw_test", "gw_test@im.wechat", "https://example.com")
	if a.GetUserID() != "gw_test@im.wechat" {
		t.Errorf("GetUserID want 'gw_test@im.wechat', got '%s'", a.GetUserID())
	}
}

func TestChatRequest(t *testing.T) {
	req := &ChatRequest{
		Message:   "test",
		UserID:    "user1",
		SessionID: "session1",
		BackendID: "backend1",
	}
	if req.Message != "test" {
		t.Errorf("Message want 'test', got '%s'", req.Message)
	}
	if req.UserID != "user1" {
		t.Errorf("UserID want 'user1', got '%s'", req.UserID)
	}
	if req.SessionID != "session1" {
		t.Errorf("SessionID want 'session1', got '%s'", req.SessionID)
	}
}

func TestChatResponse(t *testing.T) {
	resp := &ChatResponse{
		Text:    "response",
		Backend: "echo-1",
	}
	if resp.Text != "response" {
		t.Errorf("Text want 'response', got '%s'", resp.Text)
	}
	if resp.Backend != "echo-1" {
		t.Errorf("Backend want 'echo-1', got '%s'", resp.Backend)
	}
}

func TestConnectionInfo(t *testing.T) {
	info := &ConnectionInfo{
		AccountID: "gw_test",
		UserID:    "gw_test@im.wechat",
		BaseURL:   "https://example.com",
	}
	if info.AccountID != "gw_test" {
		t.Errorf("AccountID want 'gw_test', got '%s'", info.AccountID)
	}
	if info.UserID != "gw_test@im.wechat" {
		t.Errorf("UserID want 'gw_test@im.wechat', got '%s'", info.UserID)
	}
	if info.BaseURL != "https://example.com" {
		t.Errorf("BaseURL want 'https://example.com', got '%s'", info.BaseURL)
	}
}

func TestAttachment(t *testing.T) {
	att := &Attachment{
		FileName: "test.txt",
		MimeType: "text/plain",
		Data:     []byte("hello"),
	}
	if att.FileName != "test.txt" {
		t.Errorf("FileName want 'test.txt', got '%s'", att.FileName)
	}
	if att.MimeType != "text/plain" {
		t.Errorf("MimeType want 'text/plain', got '%s'", att.MimeType)
	}
	if string(att.Data) != "hello" {
		t.Errorf("Data want 'hello', got '%s'", string(att.Data))
	}
}

func TestIsConnectionAdapter(t *testing.T) {
	tests := []struct {
		adapterType string
		want        bool
	}{
		{"echo", false},
		{"openai_compatible", false},
		{"ilink_proxy", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.adapterType, func(t *testing.T) {
			got := IsConnectionAdapter(tt.adapterType)
			if got != tt.want {
				t.Errorf("IsConnectionAdapter(%q) = %v, want %v", tt.adapterType, got, tt.want)
			}
		})
	}
}

func TestNewAdapterFactoryDefault(t *testing.T) {
	af := NewAdapterFactory()
	if af == nil {
		t.Fatal("NewAdapterFactory returned nil")
	}
	if af.List() == nil {
		t.Error("List() should return non-nil slice")
	}
	if af.ListIDs() == nil {
		t.Error("ListIDs() should return non-nil slice")
	}
}

func TestAdapterFactoryHealthyListEmpty(t *testing.T) {
	af := NewAdapterFactory()
	healthy := af.List()
	if len(healthy) != 0 {
		t.Errorf("Healthy list of empty factory = %d, want 0", len(healthy))
	}
}

func TestAdapterFactoryListIDs(t *testing.T) {
	af := NewAdapterFactory()
	af.Register(NewEchoAdapter("echo", "Echo Debug"))
	af.Register(NewEchoAdapter("hermes", "Hermes AI"))

	ids := af.ListIDs()
	if len(ids) != 2 {
		t.Fatalf("ListIDs count = %d, want 2", len(ids))
	}
	hasEcho, hasHermes := false, false
	for _, id := range ids {
		if id == "echo" { hasEcho = true }
		if id == "hermes" { hasHermes = true }
	}
	if !hasEcho || !hasHermes {
		t.Errorf("ListIDs = %v, missing echo or hermes", ids)
	}
}