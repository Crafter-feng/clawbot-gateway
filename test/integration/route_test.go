// Package integration contains unit-level integration tests for the clawbot-gateway router.
// These tests exercise the route matching logic (exact, contains, prefix, suffix, regex,
// boolean logic, priority, overrides) in isolation from the database and HTTP API layers.
// They validate correctness of the routing engine itself. For complete coverage, implement
// database-backed integration scenarios (rules loaded from DB) and full API integration
// scenarios (end-to-end request routing via the Gateway HTTP handlers).
package integration

import (
	"testing"

	"clawbot-gateway/internal/route"
)

func TestExactMatch(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	// 添加精确匹配规则
	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气查询",
		BackendID: "hermes",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "exact", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试精确匹配
	decision := r.Route("天气", "user1", "", "", "")
	if decision.BackendID != "hermes" {
		t.Errorf("精确匹配: 期望 'hermes'，实际 '%s'", decision.BackendID)
	}
	if decision.MatchedBy != "keyword" {
		t.Errorf("精确匹配: 期望 'keyword'，实际 '%s'", decision.MatchedBy)
	}

	// 测试不匹配
	decision = r.Route("今天天气", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("精确匹配不匹配: 期望 'default'，实际 '%s'", decision.BackendID)
	}
}

func TestContainsMatch(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气查询",
		BackendID: "hermes",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试包含匹配
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "hermes" {
		t.Errorf("包含匹配: 期望 'hermes'，实际 '%s'", decision.BackendID)
	}
}

func TestStartsWithMatch(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气查询",
		BackendID: "hermes",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "starts_with", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试前缀匹配
	decision := r.Route("天气预报", "user1", "", "", "")
	if decision.BackendID != "hermes" {
		t.Errorf("前缀匹配: 期望 'hermes'，实际 '%s'", decision.BackendID)
	}
}

func TestEndsWithMatch(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气查询",
		BackendID: "hermes",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "ends_with", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试后缀匹配
	decision := r.Route("查天气", "user1", "", "", "")
	if decision.BackendID != "hermes" {
		t.Errorf("后缀匹配: 期望 'hermes'，实际 '%s'", decision.BackendID)
	}
}

func TestRegexMatch(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "帮助命令",
		BackendID: "echo",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "regex", Value: "^/help.*"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试正则匹配
	decision := r.Route("/help me", "user1", "", "", "")
	if decision.BackendID != "echo" {
		t.Errorf("正则匹配: 期望 'echo'，实际 '%s'", decision.BackendID)
	}
}

func TestANDLogic(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气+管理员",
		BackendID: "admin",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
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
		t.Errorf("AND 逻辑: 期望 'admin'，实际 '%s'", decision.BackendID)
	}

	// 只有一个条件满足
	decision = r.Route("天气查询", "user1", "user2", "", "")
	if decision.BackendID != "default" {
		t.Errorf("AND 逻辑不匹配: 期望 'default'，实际 '%s'", decision.BackendID)
	}
}

func TestORLogic(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气或预报",
		BackendID: "weather",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "or",
				Conditions: []route.RouteCondition{
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
		t.Errorf("OR 逻辑: 期望 'weather'，实际 '%s'", decision.BackendID)
	}

	// 匹配第二个条件
	decision = r.Route("天气预报", "user1", "", "", "")
	if decision.BackendID != "weather" {
		t.Errorf("OR 逻辑: 期望 'weather'，实际 '%s'", decision.BackendID)
	}
}

func TestNOTLogic(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "非测试消息",
		BackendID: "ai",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "测试", Negate: true},
				},
			},
		},
		GroupLogic: "and",
	})

	// 不包含"测试"
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "ai" {
		t.Errorf("NOT 逻辑: 期望 'ai'，实际 '%s'", decision.BackendID)
	}

	// 包含"测试"
	decision = r.Route("测试天气", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("NOT 逻辑不匹配: 期望 'default'，实际 '%s'", decision.BackendID)
	}
}

func TestPriorityOrder(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	// 添加两个规则，优先级不同
	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "低优先级",
		BackendID: "low",
		Priority:  10,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	r.AddRule(route.RouteRule{
		ID:        2,
		Name:      "高优先级",
		BackendID: "high",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 测试优先级
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "high" {
		t.Errorf("优先级: 期望 'high'，实际 '%s'", decision.BackendID)
	}
}

func TestDisabledRule(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "禁用规则",
		BackendID: "disabled",
		Priority:  1,
		Enabled:   false, // 禁用
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 禁用的规则不匹配
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "default" {
		t.Errorf("禁用规则: 期望 'default'，实际 '%s'", decision.BackendID)
	}
}

func TestUserOverride(t *testing.T) {
	r := route.NewRouter()
	r.SetDefaultBackend("default")

	r.AddRule(route.RouteRule{
		ID:        1,
		Name:      "天气查询",
		BackendID: "hermes",
		Priority:  1,
		Enabled:   true,
		Groups: []route.RouteRuleGroup{
			{
				ID:    "g1",
				Logic: "and",
				Conditions: []route.RouteCondition{
					{ID: "c1", Field: "message", Operator: "contains", Value: "天气"},
				},
			},
		},
		GroupLogic: "and",
	})

	// 设置用户覆写
	r.SetUserBackend("user1", "claude")

	// 用户覆写优先级最高
	decision := r.Route("今天天气怎么样", "user1", "", "", "")
	if decision.BackendID != "claude" {
		t.Errorf("用户覆写: 期望 'claude'，实际 '%s'", decision.BackendID)
	}
	if decision.MatchedBy != "session" {
		t.Errorf("用户覆写: 期望 'session'，实际 '%s'", decision.MatchedBy)
	}
}
