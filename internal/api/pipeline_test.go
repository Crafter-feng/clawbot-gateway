package api

import (
	"strings"
	"testing"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/session"
)

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
		{"/help unknown", "/help unknown", "", true},
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
		{"/unknown", "/unknown", "", true},
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
	if !strings.Contains(reply, "/echo") {
		t.Errorf("show_help reply = %q, want containing '/echo'", reply)
	}
}

func TestExecuteUnknownAction(t *testing.T) {
	cp := NewCommandProcessor(route.NewRouter(), adapter.NewAdapterFactory(), nil, nil)
	msg := bot.NormalizedMessage{FromUser: "user1", Content: "test"}
	reply := cp.Execute(&CommandMatch{Action: "unknown"}, msg)
	if !strings.Contains(reply, "未知命令") {
		t.Errorf("unknown action reply = %q, want containing '未知命令'", reply)
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