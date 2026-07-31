package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/config"
	"clawbot-gateway/internal/database"
	"clawbot-gateway/internal/ilink"
	"clawbot-gateway/internal/log"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/session"
)

// ── CommandProcessor Tests ──

func setupTestCommandProcessor() *CommandProcessor {
	r := route.NewRouter()
	r.SetDefaultBackend("default")
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	af.Register(adapter.NewEchoAdapter("hermes", "Hermes AI"))
	cm := session.NewContextManager(10, 0)
	return NewCommandProcessor(r, af, cm, nil)
}

func TestParseUse(t *testing.T) {
	cp := setupTestCommandProcessor()
	tests := []struct {
		name       string
		text       string
		wantAction string
		wantNil    bool
	}{
		{"/use", "/use", "show_status", false},
		{"/use echo", "/use echo", "switch_backend", false},
		{"/use  echo", "/use  echo", "switch_backend", false},
		{"/use unknown", "/use unknown", "switch_backend", false},
		{"/use ", "/use ", "show_status", false},
		{"/use main", "/use main", "switch_backend", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cp.Parse(tt.text)
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("Parse(%q) = %+v, want nil", tt.text, cmd)
				}
				return
			}
			if cmd == nil {
				t.Fatalf("Parse(%q) = nil, want %s", tt.text, tt.wantAction)
			}
			if cmd.Action != tt.wantAction {
				t.Errorf("Parse(%q).Action = %s, want %s", tt.text, cmd.Action, tt.wantAction)
			}
		})
	}
}

func TestParseBackends(t *testing.T) {
	cp := setupTestCommandProcessor()
	cmd := cp.Parse("/backends")
	if cmd == nil {
		t.Fatal("Parse(/backends) = nil")
	}
	if cmd.Action != "list_backends" {
		t.Errorf("Action = %s, want list_backends", cmd.Action)
	}
}

func TestParseHelp(t *testing.T) {
	cp := setupTestCommandProcessor()
	tests := []struct {
		name       string
		text       string
		wantAction string
		wantNil    bool
	}{
		{"/help", "/help", "show_help", false},
		{"/help echo", "/help echo", "forward_to", false},
		{"/help hermes", "/help hermes", "forward_to", false},
		{"/help unknown", "/help unknown", "unknown_command", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cp.Parse(tt.text)
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("Parse(%q) = %+v, want nil", tt.text, cmd)
				}
				return
			}
			if cmd == nil {
				t.Fatalf("Parse(%q) = nil, want %s", tt.text, tt.wantAction)
			}
			if cmd.Action != tt.wantAction {
				t.Errorf("Parse(%q).Action = %s, want %s", tt.text, cmd.Action, tt.wantAction)
			}
		})
	}
}

func TestParseForwardTo(t *testing.T) {
	cp := setupTestCommandProcessor()
	tests := []struct {
		name       string
		text       string
		wantAction string
		wantNil    bool
	}{
		{"/echo", "/echo", "forward_to", false},
		{"/echo hello world", "/echo hello world", "forward_to", false},
		{"/hermes", "/hermes", "forward_to", false},
		{"/unknown", "/unknown", "unknown_command", false},
		{"normal text", "normal text", "", true},
		{"/use", "/use", "show_status", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cp.Parse(tt.text)
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("Parse(%q) = %+v, want nil", tt.text, cmd)
				}
				return
			}
			if cmd == nil {
				t.Fatalf("Parse(%q) = nil, want %s", tt.text, tt.wantAction)
			}
			if cmd.Action != tt.wantAction {
				t.Errorf("Parse(%q).Action = %s, want %s", tt.text, cmd.Action, tt.wantAction)
			}
		})
	}
}

func TestParseTypoCommands(t *testing.T) {
	cp := setupTestCommandProcessor()
	tests := []struct {
		name       string
		text       string
		wantAction string
	}{
		{"/ues (typo for /use)", "/ues", "unknown_command"},
		{"/bakends (typo for /backends)", "/bakends", "unknown_command"},
		{"/hel", "/hel", "unknown_command"},
		{"/usee", "/usee", "unknown_command"},
		{"/ ", "/ ", "unknown_command"},
		{"/zorp", "/zorp", "unknown_command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cp.Parse(tt.text)
			if cmd == nil {
				t.Fatalf("Parse(%q) = nil, want %s", tt.text, tt.wantAction)
			}
			if cmd.Action != tt.wantAction {
				t.Errorf("Parse(%q).Action = %s, want %s", tt.text, cmd.Action, tt.wantAction)
			}
		})
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	cp := NewCommandProcessor(route.NewRouter(), adapter.NewAdapterFactory(), nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}

	reply := cp.Execute(&CommandMatch{Action: "unknown_command", Args: []string{"/ues"}}, msg)
	if !strings.Contains(reply, "未知命令") {
		t.Errorf("unknown_command reply = %q, want containing '未知命令'", reply)
	}
	if strings.Contains(reply, "/ues") {
		t.Errorf("unknown_command reply should not contain user input, got %q", reply)
	}
}

func TestExecuteAllActions(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	r := route.NewRouter()
	cp := NewCommandProcessor(r, af, nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}

	tests := []struct {
		name       string
		action     string
		args       []string
		wantSubstr string
	}{
		{"switch_backend valid", "switch_backend", []string{"echo"}, "已切换至"},
		{"switch_backend main", "switch_backend", []string{"main"}, "已切换至主命令模式"},
		{"switch_backend no args", "switch_backend", []string{}, "请指定后端"},
		{"switch_backend nonexistent", "switch_backend", []string{"nonexistent"}, "不存在"},
		{"list_backends", "list_backends", nil, "可用后端"},
		{"show_status", "show_status", nil, "ClawBot 状态"},
		{"show_help", "show_help", nil, "帮助"},
		{"unknown_command", "unknown_command", []string{"/ues"}, "未知命令"},
		{"unknown_action", "unknown", nil, "未知命令"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := cp.Execute(&CommandMatch{Action: tt.action, Args: tt.args}, msg)
			if !strings.Contains(reply, tt.wantSubstr) {
				t.Errorf("Execute(%s) = %q, want containing %q", tt.action, reply, tt.wantSubstr)
			}
		})
	}
}

func TestExecuteSwitchBackend(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	cp := NewCommandProcessor(r, af, nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}

	reply := cp.Execute(&CommandMatch{Action: "switch_backend", Args: []string{"echo"}}, msg)
	if !strings.Contains(reply, "已切换至") {
		t.Errorf("switch_backend reply = %q, want containing '已切换至'", reply)
	}
	backendID, ok := r.GetUserBackend("user1")
	if !ok || backendID != "echo" {
		t.Errorf("GetUserBackend = %s, %v, want echo, true", backendID, ok)
	}
	reply = cp.Execute(&CommandMatch{Action: "switch_backend", Args: []string{}}, msg)
	if !strings.Contains(reply, "请指定后端") {
		t.Errorf("switch_backend no args reply = %q, want containing '请指定后端'", reply)
	}
	reply = cp.Execute(&CommandMatch{Action: "switch_backend", Args: []string{"nonexistent"}}, msg)
	if !strings.Contains(reply, "不存在") {
		t.Errorf("switch_backend unknown reply = %q, want containing '不存在'", reply)
	}
}

func TestExecuteSwitchBackendMain(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	cp := NewCommandProcessor(r, af, nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}

	// 先切换到 echo
	cp.Execute(&CommandMatch{Action: "switch_backend", Args: []string{"echo"}}, msg)
	backendID, ok := r.GetUserBackend("user1")
	if !ok || backendID != "echo" {
		t.Fatalf("after switch, GetUserBackend = %s, %v, want echo, true", backendID, ok)
	}

	// /use main 清除后端选择
	reply := cp.Execute(&CommandMatch{Action: "switch_backend", Args: []string{"main"}}, msg)
	if !strings.Contains(reply, "已切换至主命令模式") {
		t.Errorf("switch_backend main reply = %q, want containing '已切换至主命令模式'", reply)
	}
	_, ok = r.GetUserBackend("user1")
	if ok {
		t.Errorf("after /use main, GetUserBackend should return ok=false")
	}
}

func TestExecuteListBackends(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	cp := NewCommandProcessor(route.NewRouter(), af, nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}
	reply := cp.Execute(&CommandMatch{Action: "list_backends"}, msg)
	if !strings.Contains(reply, "可用后端") {
		t.Errorf("list_backends reply = %q, want containing '可用后端'", reply)
	}
	if !strings.Contains(reply, "echo") {
		t.Errorf("list_backends reply = %q, want containing 'echo'", reply)
	}
}

func TestExecuteShowStatus(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	cp := NewCommandProcessor(route.NewRouter(), af, nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}
	reply := cp.Execute(&CommandMatch{Action: "show_status"}, msg)
	if !strings.Contains(reply, "ClawBot 状态") {
		t.Errorf("show_status reply = %q, want containing 'ClawBot 状态'", reply)
	}
}

func TestExecuteShowHelp(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	cp := NewCommandProcessor(route.NewRouter(), af, nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}
	reply := cp.Execute(&CommandMatch{Action: "show_help"}, msg)
	if !strings.Contains(reply, "帮助") {
		t.Errorf("show_help reply = %q, want containing '帮助'", reply)
	}
	if !strings.Contains(reply, "/use main") {
		t.Errorf("show_help reply = %q, want containing '/use main'", reply)
	}
	if !strings.Contains(reply, "/echo") {
		t.Errorf("show_help reply = %q, want containing '/echo'", reply)
	}
}

func TestShowBackendStatus(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	cp := NewCommandProcessor(route.NewRouter(), af, nil, nil)

	reply := cp.ShowBackendStatus("echo")
	if !strings.Contains(reply, "echo") {
		t.Errorf("ShowBackendStatus reply = %q, want containing 'echo'", reply)
	}

	reply = cp.ShowBackendStatus("nonexistent")
	if !strings.Contains(reply, "不存在") {
		t.Errorf("ShowBackendStatus nonexistent reply = %q, want containing '不存在'", reply)
	}
}

func TestMatchPrefix(t *testing.T) {
	tests := []struct {
		text     string
		prefixes []string
		want     string
	}{
		{"/use echo", []string{"/use "}, "echo"},
		{"/use", []string{"/use "}, ""},
		{"/help hermes", []string{"/help "}, "hermes"},
		{"/help", []string{"/help "}, ""},
		{"hello", []string{"/use ", "/help "}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := matchPrefix(tt.text, tt.prefixes...)
			if got != tt.want {
				t.Errorf("matchPrefix(%q, %v) = %q, want %q", tt.text, tt.prefixes, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"你好世界", 2, "你好..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := truncate(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

func TestConvertChatHistory(t *testing.T) {
	history := []session.ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	converted := convertChatHistory(history)
	if len(converted) != 2 {
		t.Fatalf("convertChatHistory length = %d, want 2", len(converted))
	}
	if converted[0].Role != "user" || converted[0].Content != "hello" {
		t.Errorf("converted[0] = %+v, want {Role:user Content:hello}", converted[0])
	}
	if converted[1].Role != "assistant" || converted[1].Content != "hi" {
		t.Errorf("converted[1] = %+v, want {Role:assistant Content:hi}", converted[1])
	}
}

func TestConvertChatHistoryEmpty(t *testing.T) {
	converted := convertChatHistory(nil)
	if len(converted) != 0 {
		t.Errorf("convertChatHistory(nil) length = %d, want 0", len(converted))
	}
}

// ── MessagePipeline Tests ──

func TestNewMessagePipeline(t *testing.T) {
	p := NewMessagePipeline(nil, route.NewRouter(), adapter.NewAdapterFactory(), session.NewContextManager(10, 0), ilink.NewClientRegistry())
	if p == nil {
		t.Fatal("NewMessagePipeline returned nil")
	}
	if p.commandProc == nil {
		t.Error("commandProc should be initialized")
	}
}

func TestMessageCount(t *testing.T) {
	p := NewMessagePipeline(nil, route.NewRouter(), adapter.NewAdapterFactory(), session.NewContextManager(10, 0), ilink.NewClientRegistry())
	if p.MessageCount() != 0 {
		t.Errorf("MessageCount = %d, want 0", p.MessageCount())
	}
}

func TestAdapterName(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	p := NewMessagePipeline(nil, route.NewRouter(), af, session.NewContextManager(10, 0), ilink.NewClientRegistry())

	if name := p.adapterName("echo"); name != "Echo Debug" {
		t.Errorf("adapterName(echo) = %q, want 'Echo Debug'", name)
	}
	if name := p.adapterName("nonexistent"); name != "nonexistent" {
		t.Errorf("adapterName(nonexistent) = %q, want 'nonexistent'", name)
	}
}

func TestHandleDirectMessageWithBackendID(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	p := NewMessagePipeline(nil, route.NewRouter(), af, session.NewContextManager(10, 0), nil)

	reply, err := p.HandleDirectMessage(context.Background(), "hello", "user1", "echo")
	if err != nil {
		t.Fatalf("HandleDirectMessage error: %v", err)
	}
	if !strings.Contains(reply, "hello") {
		t.Errorf("reply = %q, want containing 'hello'", reply)
	}
}

func TestHandleDirectMessageNoBackendID(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("echo")
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	p := NewMessagePipeline(nil, r, af, session.NewContextManager(10, 0), nil)

	reply, err := p.HandleDirectMessage(context.Background(), "hello", "user1", "")
	if err != nil {
		t.Fatalf("HandleDirectMessage error: %v", err)
	}
	if !strings.Contains(reply, "hello") {
		t.Errorf("reply = %q, want containing 'hello'", reply)
	}
}

func TestHandleDirectMessageNoBackendAvailable(t *testing.T) {
	p := NewMessagePipeline(nil, route.NewRouter(), adapter.NewAdapterFactory(), session.NewContextManager(10, 0), nil)

	_, err := p.HandleDirectMessage(context.Background(), "hello", "user1", "")
	if err == nil {
		t.Error("HandleDirectMessage with no backend should return error")
	}
}

func TestHandleDirectMessageNonExistentBackend(t *testing.T) {
	af := adapter.NewAdapterFactory()
	p := NewMessagePipeline(nil, route.NewRouter(), af, session.NewContextManager(10, 0), nil)

	reply, err := p.HandleDirectMessage(context.Background(), "hello", "user1", "nonexistent")
	if err != nil {
		t.Fatalf("HandleDirectMessage error: %v", err)
	}
	if !strings.Contains(reply, "不可用") {
		t.Errorf("reply = %q, want containing '不可用'", reply)
	}
}

func TestEnqueueToVirtualBotNoClientReg(t *testing.T) {
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	p := NewMessagePipeline(nil, route.NewRouter(), af, session.NewContextManager(10, 0), nil)
	p.SetLogger(log.Default().WithComponent("test"))
	msg := createTestMessage("hello", "user1")
	ok := p.enqueueToVirtualBot(msg, "echo", 1)
	if ok {
		t.Error("enqueueToVirtualBot with nil clientReg should return false")
	}
}

func TestEnqueueToVirtualBotNoVirtualBot(t *testing.T) {
	clientReg := ilink.NewClientRegistry()
	af := adapter.NewAdapterFactory()
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	p := NewMessagePipeline(nil, route.NewRouter(), af, session.NewContextManager(10, 0), clientReg)
	p.SetLogger(log.Default().WithComponent("test"))
	msg := createTestMessage("hello", "user1")
	ok := p.enqueueToVirtualBot(msg, "echo", 1)
	if ok {
		t.Error("enqueueToVirtualBot with unregistered virtual bot should return false")
	}
}

func TestSetLogger(t *testing.T) {
	p := NewMessagePipeline(nil, route.NewRouter(), adapter.NewAdapterFactory(), session.NewContextManager(10, 0), ilink.NewClientRegistry())
	logger := log.Default().WithComponent("test")
	p.SetLogger(logger)
	if p.log == nil {
		t.Error("log should be set")
	}
}

func TestWait(t *testing.T) {
	p := NewMessagePipeline(nil, route.NewRouter(), adapter.NewAdapterFactory(), session.NewContextManager(10, 0), ilink.NewClientRegistry())
	p.Wait()
}

func createTestMessage(content, fromUser string) bot.NormalizedMessage {
	return bot.NormalizedMessage{
		Content:   content,
		FromUser:  fromUser,
		AccountID: "test_account",
		MsgID:     "msg_1",
	}
}

// ── HTTP Handler Tests ──

func TestHandleHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := createTestAPIServer()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/health", nil)
	s.handleHealth(c)

	if w.Code != 200 {
		t.Errorf("health status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("health body = %s, want containing 'ok'", w.Body.String())
	}
}

func TestHandleStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := createTestAPIServer()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/stats", nil)
	s.handleStats(c)

	if w.Code != 200 {
		t.Errorf("stats status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "version") {
		t.Errorf("stats body = %s, want containing 'version'", w.Body.String())
	}
}

func TestHandleGetLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := createTestAPIServer()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/logs", nil)
	s.handleGetLogs(c)

	if w.Code != 200 {
		t.Errorf("logs status = %d, want 200", w.Code)
	}
}

func TestHandleGetLogsWithLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := createTestAPIServer()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/logs?limit=50&level=INFO", nil)
	s.handleGetLogs(c)

	if w.Code != 200 {
		t.Errorf("logs status = %d, want 200", w.Code)
	}
}

func TestHandleGetLogCategories(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := createTestAPIServer()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/logs/categories", nil)
	s.handleGetLogCategories(c)

	if w.Code != 200 {
		t.Errorf("categories status = %d, want 200", w.Code)
	}
}

func createTestAPIServer() *APIServer {
	db, err := database.New(":memory:")
	if err != nil {
		panic(err)
	}
	cfg := &config.Config{}
	r := route.NewRouter()
	af := adapter.NewAdapterFactory()
	cm := session.NewContextManager(10, 0)
	log.SetDefault(log.New("info"))
	return &APIServer{
		config:     cfg,
		db:         db,
		router:     r,
		adapters:   af,
		ctxManager: cm,
	}
}