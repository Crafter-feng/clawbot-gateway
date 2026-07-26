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

// TestRouterNoBackendSelected 测试未选中后端时返回默认后端
func TestRouterNoBackendSelected(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("openclaw")
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "openclaw" {
		t.Errorf("expected default backend 'openclaw', got %s", decision.BackendID)
	}
	if decision.MatchedBy != "default" {
		t.Errorf("expected matched_by=default, got %s", decision.MatchedBy)
	}
}

// TestRouterUserOverride 测试用户覆写后端
func TestRouterUserOverride(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("openclaw")

	// 未覆写时返回默认后端
	decision := r.Route("hello", "user1", "", "", "")
	assertBackend(t, decision, "openclaw")

	// 设置覆写
	r.SetUserBackend("user1", "claude")
	decision = r.Route("hello", "user1", "", "", "")
	assertBackend(t, decision, "claude")

	// 不同用户不受影响
	decision = r.Route("hello", "user2", "", "", "")
	assertBackend(t, decision, "openclaw")
}

// TestRouterClearUserBackend 测试清除用户覆写
func TestRouterClearUserBackend(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("openclaw")
	r.SetUserBackend("user1", "claude")

	decision := r.Route("hello", "user1", "", "", "")
	assertBackend(t, decision, "claude")

	r.ClearUserBackend("user1")
	decision = r.Route("hello", "user1", "", "", "")
	assertBackend(t, decision, "openclaw")
}

// TestRouterKeywordRules 测试关键词规则匹配
func TestRouterKeywordRules(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	// 添加规则
	r.AddRule(RouteRule{
		ID:        1,
		Name:      "天气查询",
		BackendID: "weather",
		Priority:  1,
		Enabled:   true,
		Groups: []RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []RouteCondition{
					{
						ID:       "c1",
						Field:    "message",
						Operator: "contains",
						Value:    "天气",
					},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试匹配
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "weather" {
		t.Errorf("expected 'weather', got %s", decision.BackendID)
	}
	if decision.MatchedBy != "keyword" {
		t.Errorf("expected matched_by=keyword, got %s", decision.MatchedBy)
	}

	// 测试不匹配
	decision = r.Route("你好世界", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("expected 'default', got %s", decision.BackendID)
	}
}

// TestRouterANDLogic 测试 AND 逻辑
func TestRouterANDLogic(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{
		ID:        1,
		Name:      "天气+管理员",
		BackendID: "admin",
		Priority:  1,
		Enabled:   true,
		Groups: []RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
					{ID: "c2", Field: "from_user", Operator: "exact", Value: "admin"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 两个条件都满足
	decision := r.Route("天气查询", "user1", "admin", "", "")
	if decision.BackendID != "admin" {
		t.Errorf("expected 'admin', got %s", decision.BackendID)
	}

	// 只有一个条件满足
	decision = r.Route("天气查询", "user1", "user2", "", "")
	if decision.BackendID != "default" {
		t.Errorf("expected 'default', got %s", decision.BackendID)
	}
}

// TestRouterORLogic 测试 OR 逻辑
func TestRouterORLogic(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{
		ID:        1,
		Name:      "天气或预报",
		BackendID: "weather",
		Priority:  1,
		Enabled:   true,
		Groups: []RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "or",
				Conditions: []RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
					{ID: "c2", Field: "message", Operator: "contains", Value: "预报"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 匹配第一个条件
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "weather" {
		t.Errorf("expected 'weather', got %s", decision.BackendID)
	}

	// 匹配第二个条件
	decision = r.Route("天气预报", "user1", "", "", "")
	if decision.BackendID != "weather" {
		t.Errorf("expected 'weather', got %s", decision.BackendID)
	}

	// 不匹配
	decision = r.Route("你好世界", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("expected 'default', got %s", decision.BackendID)
	}
}

// TestRouterNOTLogic 测试 NOT 逻辑
func TestRouterNOTLogic(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{
		ID:        1,
		Name:      "非测试消息",
		BackendID: "ai",
		Priority:  1,
		Enabled:   true,
		Groups: []RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "测试", Negate: true},
				},
			},
		},
		GroupLogic: "and",
	})

	// 不包含"测试"
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "ai" {
		t.Errorf("expected 'ai', got %s", decision.BackendID)
	}

	// 包含"测试"
	decision = r.Route("测试天气", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("expected 'default', got %s", decision.BackendID)
	}
}

// TestRouterRegex 测试正则匹配
func TestRouterRegex(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{
		ID:        1,
		Name:      "帮助命令",
		BackendID: "help",
		Priority:  1,
		Enabled:   true,
		Groups: []RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []RouteCondition{
					{ID: "c1", Field: "message", Operator: "regex", Value: "^/help.*"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 匹配正则
	decision := r.Route("/help me", "user1", "", "", "")
	if decision.BackendID != "help" {
		t.Errorf("expected 'help', got %s", decision.BackendID)
	}

	// 不匹配
	decision = r.Route("hello", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("expected 'default', got %s", decision.BackendID)
	}
}

// TestRouterDisabledRule 测试禁用的规则
func TestRouterDisabledRule(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{
		ID:        1,
		Name:      "禁用规则",
		BackendID: "disabled",
		Priority:  1,
		Enabled:   false, // 禁用
		Groups: []RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "测试"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 禁用的规则不匹配
	decision := r.Route("测试消息", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("expected 'default', got %s", decision.BackendID)
	}
}

// TestRouterGetSetDefault 测试默认后端的 get/set
func TestRouterGetSetDefault(t *testing.T) {
	r := NewRouter()
	if r.GetDefaultBackend() != "" {
		t.Errorf("default want empty, got %s", r.GetDefaultBackend())
	}
	r.SetDefaultBackend("openclaw")
	if r.GetDefaultBackend() != "openclaw" {
		t.Errorf("default want openclaw, got %s", r.GetDefaultBackend())
	}
}

// TestGetRules 测试规则列表
func TestGetRules(t *testing.T) {
	r := NewRouter()
	r.AddRule(RouteRule{
		ID:       1,
		Name:     "测试规则",
		BackendID: "test",
		Priority: 1,
		Enabled:  true,
	})
	rules := r.GetRules()
	if len(rules) != 1 {
		t.Errorf("want 1 rule, got %d", len(rules))
	}
}
