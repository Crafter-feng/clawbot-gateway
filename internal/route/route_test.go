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
	r.SetUserBackend("user1", "claude", nil)
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
	r.SetUserBackend("user1", "claude", nil)

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

func TestRouteMatchTypes(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{ID: 1, Name: "exact", BackendID: "exact_bk", Priority: 1, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "exact", Value: "hello"}}}}})
	r.AddRule(RouteRule{ID: 2, Name: "contains", BackendID: "contains_bk", Priority: 2, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "weather"}}}}})
	r.AddRule(RouteRule{ID: 3, Name: "prefix", BackendID: "prefix_bk", Priority: 3, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "starts_with", Value: "/cmd"}}}}})
	r.AddRule(RouteRule{ID: 4, Name: "suffix", BackendID: "suffix_bk", Priority: 4, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "ends_with", Value: "?"}}}}})
	r.AddRule(RouteRule{ID: 5, Name: "regex", BackendID: "regex_bk", Priority: 5, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "regex", Value: "^\\d+$"}}}}})

	tests := []struct {
		name   string
		msg    string
		want   string
		wantBy string
	}{
		{"exact match", "hello", "exact_bk", "keyword"},
		{"contains match", "weather today", "contains_bk", "keyword"},
		{"prefix match", "/cmd help", "prefix_bk", "keyword"},
		{"suffix match", "are you ok?", "suffix_bk", "keyword"},
		{"regex match", "12345", "regex_bk", "keyword"},
		{"no match falls to default", "你好世界", "default", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := r.Route(tt.msg, "user1", "", "", "")
			if d.BackendID != tt.want || d.MatchedBy != tt.wantBy {
				t.Errorf("Route(%q) = {%s, %s}, want {%s, %s}", tt.msg, d.BackendID, d.MatchedBy, tt.want, tt.wantBy)
			}
		})
	}
}

func TestRouteUserOverridePriority(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")
	r.AddRule(RouteRule{ID: 1, Name: "keyword", BackendID: "keyword_bk", Priority: 1, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "hello"}}}}})

	// User override takes priority over keyword rules
	r.SetUserBackend("user1", "override_bk", nil)
	d := r.Route("hello world", "user1", "", "", "")
	assertBackend(t, d, "override_bk")
	if d.MatchedBy != "session" {
		t.Errorf("expected matched_by=session, got %s", d.MatchedBy)
	}
}

func TestRouteEmptyRules(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")
	d := r.Route("hello", "user1", "", "", "")
	assertBackend(t, d, "default")
}

func TestRouteNoDefaultBackend(t *testing.T) {
	r := NewRouter()
	d := r.Route("hello", "user1", "", "", "")
	if d.BackendID != "" {
		t.Errorf("expected empty backend, got %s", d.BackendID)
	}
}

func TestRouteUserOverrideInvalid(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")
	err := r.SetUserBackend("user1", "nonexistent", []string{"echo", "hermes"})
	if err == nil {
		t.Error("SetUserBackend with invalid backend should return error")
	}
}

func TestAddRemoveRule(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{ID: 1, Name: "rule1", BackendID: "bk1", Priority: 1, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "hello"}}}}})

	d := r.Route("hello", "user1", "", "", "")
	assertBackend(t, d, "bk1")

	r.RemoveRule(1)
	d = r.Route("hello", "user1", "", "", "")
	assertBackend(t, d, "default")
}

func TestUpdateRule(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{ID: 1, Name: "rule1", BackendID: "bk1", Priority: 1, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "hello"}}}}})

	r.UpdateRule(RouteRule{ID: 1, Name: "updated", BackendID: "bk2", Priority: 1, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "hello"}}}}})

	d := r.Route("hello", "user1", "", "", "")
	assertBackend(t, d, "bk2")
}

func TestRulePriorityOrder(t *testing.T) {
	r := NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(RouteRule{ID: 1, Name: "low", BackendID: "low_bk", Priority: 10, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "test"}}}}})
	r.AddRule(RouteRule{ID: 2, Name: "high", BackendID: "high_bk", Priority: 1, Enabled: true,
		Groups: []RouteRuleGroup{{ID: "g1", Logic: "and",
			Conditions: []RouteCondition{{Field: "message", Operator: "contains", Value: "test"}}}}})

	// Higher priority (lower number) should match first
	d := r.Route("test message", "user1", "", "", "")
	assertBackend(t, d, "high_bk")
}

func TestValidateRegexp(t *testing.T) {
	if err := ValidateRegexp("^hello$"); err != nil {
		t.Errorf("valid regex should not error: %v", err)
	}
	if err := ValidateRegexp("[invalid"); err == nil {
		t.Error("invalid regex should error")
	}
}
