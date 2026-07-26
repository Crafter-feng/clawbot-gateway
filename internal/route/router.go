package route

import (
	"regexp"
	"strings"
	"sync"
)

// Router 路由引擎
type Router struct {
	mu              sync.RWMutex
	rules           []RouteRule      // 路由规则（按优先级排序）
	defaultBackend  string           // 默认后端
	userBackends    map[string]string // 用户会话级覆写（userID → backendID）
	compiledRegex   map[string]*regexp.Regexp
}

// RouteRule 路由规则（内部使用）
type RouteRule struct {
	ID          int
	Name        string
	BackendID   string
	Priority    int
	Enabled     bool
	Groups      []RouteRuleGroup
	GroupLogic  string
}

// RouteRuleGroup 规则组
type RouteRuleGroup struct {
	ID         string
	Logic      string           // and/or
	Conditions []RouteCondition
}

// RouteCondition 匹配条件
type RouteCondition struct {
	ID            string
	Field         string
	Operator      string
	Value         string
	CaseSensitive bool
	Negate        bool
}

// RouteDecision 路由决策结果
type RouteDecision struct {
	BackendID string
	MatchedBy string // "session", "keyword", "default"
	RuleID    int    // 匹配的规则 ID
}

// NewRouter 创建路由引擎
func NewRouter() *Router {
	return &Router{
		rules:          make([]RouteRule, 0),
		userBackends:   make(map[string]string),
		compiledRegex:  make(map[string]*regexp.Regexp),
	}
}

// SetDefaultBackend 设置默认后端
func (r *Router) SetDefaultBackend(backendID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultBackend = backendID
}

// GetDefaultBackend 获取默认后端
func (r *Router) GetDefaultBackend() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultBackend
}

// SetUserBackend 设置用户会话级覆写
func (r *Router) SetUserBackend(userID, backendID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if backendID == "" {
		delete(r.userBackends, userID)
	} else {
		r.userBackends[userID] = backendID
	}
}

// GetUserBackend 获取用户会话级覆写
// 返回 backendID 和是否存在覆写
func (r *Router) GetUserBackend(userID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backendID, ok := r.userBackends[userID]
	return backendID, ok
}

// ClearUserBackend 清除用户会话级覆写
func (r *Router) ClearUserBackend(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.userBackends, userID)
}

// LoadRules 加载路由规则
func (r *Router) LoadRules(rules []RouteRule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rules = make([]RouteRule, len(rules))
	copy(r.rules, rules)

	// 预编译正则表达式
	r.compiledRegex = make(map[string]*regexp.Regexp)
	for _, rule := range rules {
		for _, group := range rule.Groups {
			for _, cond := range group.Conditions {
				if cond.Operator == "regex" && cond.Value != "" {
					if compiled, err := regexp.Compile(cond.Value); err == nil {
						r.compiledRegex[cond.Value] = compiled
					}
				}
			}
		}
	}
}

// AddRule 添加路由规则
func (r *Router) AddRule(rule RouteRule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 预编译正则表达式
	for _, group := range rule.Groups {
		for _, cond := range group.Conditions {
			if cond.Operator == "regex" && cond.Value != "" {
				if compiled, err := regexp.Compile(cond.Value); err == nil {
					r.compiledRegex[cond.Value] = compiled
				}
			}
		}
	}

	// 添加规则并按优先级排序
	r.rules = append(r.rules, rule)
	r.sortRules()
}

// RemoveRule 移除路由规则
func (r *Router) RemoveRule(id int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, rule := range r.rules {
		if rule.ID == id {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			break
		}
	}
}

// UpdateRule 更新路由规则
func (r *Router) UpdateRule(rule RouteRule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 移除旧规则
	for i, existing := range r.rules {
		if existing.ID == rule.ID {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			break
		}
	}

	// 预编译正则表达式
	for _, group := range rule.Groups {
		for _, cond := range group.Conditions {
			if cond.Operator == "regex" && cond.Value != "" {
				if compiled, err := regexp.Compile(cond.Value); err == nil {
					r.compiledRegex[cond.Value] = compiled
				}
			}
		}
	}

	// 添加新规则并排序
	r.rules = append(r.rules, rule)
	r.sortRules()
}

// GetRules 获取所有路由规则
func (r *Router) GetRules() []RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RouteRule, len(r.rules))
	copy(result, r.rules)
	return result
}

// RouteMulti 多后端路由（兼容旧接口）
func (r *Router) RouteMulti(message, userID string) []RouteDecision {
	decision := r.Route(message, userID, "", "", "")
	return []RouteDecision{decision}
}

// AddKeywordRule 添加关键词规则（兼容旧接口）
func (r *Router) AddKeywordRule(keyword, backendID string, isRegexp bool) {
	rule := RouteRule{
		ID:        len(r.rules) + 1,
		Name:      keyword,
		BackendID: backendID,
		Priority:  len(r.rules) + 1,
		Enabled:   true,
		Groups: []RouteRuleGroup{
			{
				ID:    "group_1",
				Logic: "and",
				Conditions: []RouteCondition{
					{
						ID:       "cond_1",
						Field:    "message",
						Operator: "contains",
						Value:    keyword,
					},
				},
			},
		},
		GroupLogic: "and",
	}
	r.AddRule(rule)
}

// SetUserRouteMode 设置用户路由模式（兼容旧接口）
func (r *Router) SetUserRouteMode(userID, mode string) {
	// 当前实现不支持多后端路由模式
	// 保留此方法以兼容旧接口
}

// GetUserRouteMode 获取用户路由模式（兼容旧接口）
func (r *Router) GetUserRouteMode(userID string) string {
	return "single"
}

// SetUserSecondaries 设置用户次要后端（兼容旧接口）
func (r *Router) SetUserSecondaries(userID string, secondaries []string) {
	// 当前实现不支持多后端路由
}

// GetUserSecondaries 获取用户次要后端（兼容旧接口）
func (r *Router) GetUserSecondaries(userID string) []string {
	return nil
}

// Route 路由决策
func (r *Router) Route(message, userID, fromUser, toUser, msgType string) RouteDecision {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. 检查用户会话级覆写（最高优先级）
	if backendID, ok := r.userBackends[userID]; ok {
		return RouteDecision{BackendID: backendID, MatchedBy: "session"}
	}

	// 2. 按优先级遍历路由规则
	for _, rule := range r.rules {
		if !rule.Enabled {
			continue
		}
		if r.matchRule(message, userID, fromUser, toUser, msgType, rule) {
			return RouteDecision{
				BackendID: rule.BackendID,
				MatchedBy: "keyword",
				RuleID:    rule.ID,
			}
		}
	}

	// 3. 返回默认后端
	return RouteDecision{BackendID: r.defaultBackend, MatchedBy: "default"}
}

// matchRule 检查消息是否匹配规则（支持 AND/OR/NOT 逻辑）
func (r *Router) matchRule(message, userID, fromUser, toUser, msgType string, rule RouteRule) bool {
	if len(rule.Groups) == 0 {
		return false
	}

	// 评估每个组
	groupResults := make([]bool, len(rule.Groups))
	for i, group := range rule.Groups {
		groupResults[i] = r.evaluateGroup(message, userID, fromUser, toUser, msgType, group)
	}

	// 根据组间逻辑合并结果
	if rule.GroupLogic == "and" {
		for _, result := range groupResults {
			if !result {
				return false
			}
		}
		return true
	}
	// OR 逻辑
	for _, result := range groupResults {
		if result {
			return true
		}
	}
	return false
}

// evaluateGroup 评估单个规则组
func (r *Router) evaluateGroup(message, userID, fromUser, toUser, msgType string, group RouteRuleGroup) bool {
	if len(group.Conditions) == 0 {
		return false
	}

	// 评估每个条件
	conditionResults := make([]bool, len(group.Conditions))
	for i, condition := range group.Conditions {
		conditionResults[i] = r.evaluateCondition(message, userID, fromUser, toUser, msgType, condition)
	}

	// 根据组内逻辑合并结果
	if group.Logic == "and" {
		for _, result := range conditionResults {
			if !result {
				return false
			}
		}
		return true
	}
	// OR 逻辑
	for _, result := range conditionResults {
		if result {
			return true
		}
	}
	return false
}

// evaluateCondition 评估单个条件（支持 NOT 逻辑）
func (r *Router) evaluateCondition(message, userID, fromUser, toUser, msgType string, condition RouteCondition) bool {
	var result bool

	// 根据字段获取值
	var fieldValue string
	switch condition.Field {
	case "message":
		fieldValue = message
	case "from_user":
		fieldValue = fromUser
	case "to_user":
		fieldValue = toUser
	case "msg_type":
		fieldValue = msgType
	default:
		fieldValue = message
	}

	// 根据操作符匹配
	switch condition.Operator {
	case "exact":
		if condition.CaseSensitive {
			result = fieldValue == condition.Value
		} else {
			result = strings.EqualFold(fieldValue, condition.Value)
		}
	case "contains":
		if condition.CaseSensitive {
			result = strings.Contains(fieldValue, condition.Value)
		} else {
			result = strings.Contains(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
		}
	case "starts_with":
		if condition.CaseSensitive {
			result = strings.HasPrefix(fieldValue, condition.Value)
		} else {
			result = strings.HasPrefix(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
		}
	case "ends_with":
		if condition.CaseSensitive {
			result = strings.HasSuffix(fieldValue, condition.Value)
		} else {
			result = strings.HasSuffix(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
		}
	case "regex":
		if compiled, ok := r.compiledRegex[condition.Value]; ok {
			result = compiled.MatchString(fieldValue)
		}
	}

	// NOT 逻辑：取反
	if condition.Negate {
		result = !result
	}

	return result
}

// sortRules 按优先级排序规则
func (r *Router) sortRules() {
	for i := 0; i < len(r.rules); i++ {
		for j := i + 1; j < len(r.rules); j++ {
			if r.rules[j].Priority < r.rules[i].Priority {
				r.rules[i], r.rules[j] = r.rules[j], r.rules[i]
			}
		}
	}
}

// ValidateRegexp 验证正则表达式安全性
func ValidateRegexp(pattern string) error {
	// 长度限制
	if len(pattern) > 200 {
		return &RegexpError{Pattern: pattern, Err: "pattern too long"}
	}

	// 编译测试
	_, err := regexp.Compile(pattern)
	return err
}

// RegexpError 正则表达式错误
type RegexpError struct {
	Pattern string
	Err     string
}

func (e *RegexpError) Error() string {
	return "regexp error: " + e.Err
}
