package route

import (
	"testing"
)

func assertBackend(t *testing.T, got RouteDecision, want string) {
	t.Helper()
	if got.BackendID != want {
		t.Errorf("RouteDecision{%+v}.BackendID want %s, got %s", got, want, got.BackendID)
	}
}

// TestRouterNoBackendSelected 测试未选中后端时返回空
func TestRouterNoBackendSelected(t *testing.T) {
	r := NewRouter("openclaw")
	decision := r.Route("今天天气怎么样", "user1")
	if decision.BackendID != "" {
		t.Errorf("expected empty backend, got %s", decision.BackendID)
	}
	if decision.MatchedBy != "none" {
		t.Errorf("expected matched_by=none, got %s", decision.MatchedBy)
	}
}

// TestRouterUserOverride 测试用户覆写后端
func TestRouterUserOverride(t *testing.T) {
	r := NewRouter("openclaw")

	// 未覆写时返回空
	decision := r.Route("hello", "user1")
	assertBackend(t, decision, "")

	// 设置覆写
	r.SetUserBackend("user1", "claude")
	decision = r.Route("hello", "user1")
	assertBackend(t, decision, "claude")

	// 不同用户不受影响
	decision = r.Route("hello", "user2")
	assertBackend(t, decision, "")
}

// TestRouterClearUserBackend 测试清除用户覆写
func TestRouterClearUserBackend(t *testing.T) {
	r := NewRouter("openclaw")
	r.SetUserBackend("user1", "claude")

	decision := r.Route("hello", "user1")
	assertBackend(t, decision, "claude")

	r.ClearUserBackend("user1")
	decision = r.Route("hello", "user1")
	assertBackend(t, decision, "")
}

// TestRouterKeywordRulesIgnored 测试正则规则在 Route() 中不生效
func TestRouterKeywordRulesIgnored(t *testing.T) {
	r := NewRouter("echo")
	r.AddKeywordRule("天气", "openclaw")
	r.AddKeywordRule("代码", "claude")

	// 无用户覆写时，不匹配关键词规则，返回空
	decision := r.Route("今天天气真好", "user1")
	if decision.BackendID != "" {
		t.Errorf("expected empty (keyword rules ignored), got %s", decision.BackendID)
	}
}

// TestRouterGetSetDefault 测试默认后端的 get/set（不影响 Route 行为）
func TestRouterGetSetDefault(t *testing.T) {
	r := NewRouter("openclaw")
	if r.GetDefaultBackend() != "openclaw" {
		t.Errorf("default want openclaw, got %s", r.GetDefaultBackend())
	}
	r.SetDefaultBackend("claude")
	if r.GetDefaultBackend() != "claude" {
		t.Errorf("default want claude after set, got %s", r.GetDefaultBackend())
	}
}

// TestGetSetUserRouteMode 测试路由模式设置
func TestGetSetUserRouteMode(t *testing.T) {
	r := NewRouter("openclaw")
	if r.GetUserRouteMode("user1") != ModeSingle {
		t.Errorf("default mode want single, got %s", r.GetUserRouteMode("user1"))
	}
	r.SetUserRouteMode("user1", ModeBoth)
	if r.GetUserRouteMode("user1") != ModeBoth {
		t.Errorf("mode want both, got %s", r.GetUserRouteMode("user1"))
	}
}

// TestGetKeywordRules 测试规则列表
func TestGetKeywordRules(t *testing.T) {
	r := NewRouter("echo")
	r.AddKeywordRule("天气", "openclaw")
	rules := r.GetKeywordRules()
	if len(rules) != 1 {
		t.Errorf("want 1 rule, got %d", len(rules))
	}
}

// TestRemoveKeywordRule 测试规则删除
func TestRemoveKeywordRule(t *testing.T) {
	r := NewRouter("echo")
	r.AddKeywordRule("天气", "openclaw")
	if !r.RemoveKeywordRule(0) {
		t.Error("expected true for valid removal")
	}
	if r.RemoveKeywordRule(0) {
		t.Error("expected false for out-of-range removal")
	}
}
