package route

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// maxRegexpLen 限制用户自定义正则的最大长度，防止 ReDoS 攻击
const maxRegexpLen = 200

// ValidateRegexp 检查正则表达式是否安全（长度限制 + 编译验证 + 超时）
func ValidateRegexp(pattern string) error {
	if len(pattern) > maxRegexpLen {
		return fmt.Errorf("regex pattern too long (%d chars, max %d)", len(pattern), maxRegexpLen)
	}
	// 检查常见的灾难性回溯模式
	if containsNestedQuantifiers(pattern) {
		return fmt.Errorf("regex pattern contains nested quantifiers which may cause ReDoS")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	// 用超时验证执行时间
	done := make(chan struct{})
	go func() {
		re.MatchString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("regex pattern is too slow (possible ReDoS)")
	}
	return nil
}

// containsNestedQuantifiers 检查是否有嵌套量词（如 (a+)+ 这种典型 ReDoS 模式）
func containsNestedQuantifiers(pattern string) bool {
	// 移除转义字符后检查
	cleaned := strings.Builder{}
	escaped := false
	for i := 0; i < len(pattern); i++ {
		if escaped {
			cleaned.WriteByte(pattern[i])
			escaped = false
			continue
		}
		if pattern[i] == '\\' {
			escaped = true
			cleaned.WriteByte(pattern[i])
			continue
		}
		cleaned.WriteByte(pattern[i])
	}
	return repeatedQuantifiers(cleaned.String())
}

func repeatedQuantifiers(s string) bool {
	quantifiers := "+*"
	for _, q := range quantifiers {
		for _, q2 := range quantifiers {
			pattern := string(q) + string(q2)
			if strings.Contains(s, pattern) {
				return true
			}
		}
	}
	return false
}

// RouteDecision 路由决策结果
type RouteDecision struct {
	BackendID string
	MatchedBy string // "session", "keyword", "default"
	Keyword   string
}

// Router 三层路由引擎
// 1. 用户会话级覆写（/switch 设置）
// 2. 关键词规则匹配（/route add 设置）
// 3. 默认后端兜底
type Router struct {
	mu              sync.RWMutex
	userBackends    map[string]string   `json:"-"`
	userRouteModes  map[string]string   `json:"-"`
	userSecondaries map[string][]string `json:"-"`
	keywordRules    []KeywordRoute
	defaultBackend  string
}

type KeywordRoute struct {
	Keyword  string         `json:"keyword"`
	Backend  string         `json:"backend"`
	IsRegexp bool           `json:"is_regexp,omitempty"`
	compiled *regexp.Regexp `json:"-"` // 预编译的正则
}

func NewRouter(defaultBackend string) *Router {
	return &Router{
		userBackends:    make(map[string]string),
		userRouteModes:  make(map[string]string),
		userSecondaries: make(map[string][]string),
		keywordRules:    make([]KeywordRoute, 0),
		defaultBackend:  defaultBackend,
	}
}

// Route 决定一条消息应该路由到哪个后端
// 只检查用户会话级覆写，不使用正则规则和默认后端
func (r *Router) Route(message string, userID string) RouteDecision {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 只检查用户会话级覆写
	if backendID, ok := r.userBackends[userID]; ok {
		return RouteDecision{
			BackendID: backendID,
			MatchedBy: "session",
		}
	}

	// 无覆写 → 返回空（表示未选中后端）
	return RouteDecision{
		BackendID: "",
		MatchedBy: "none",
	}
}

// SetUserBackend 设置用户会话级后端
func (r *Router) SetUserBackend(userID, backendID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userBackends[userID] = backendID
}

// ClearUserBackend 清除用户会话级覆写
func (r *Router) ClearUserBackend(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.userBackends, userID)
}

// GetUserBackend 获取用户当前后端
func (r *Router) GetUserBackend(userID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, ok := r.userBackends[userID]
	return backend, ok
}

// AddKeywordRule 添加关键词路由规则（支持正则）
func (r *Router) AddKeywordRule(keyword, backendID string, isRegexp ...bool) error {
	re := len(isRegexp) > 0 && isRegexp[0]
	if keyword == "" {
		return fmt.Errorf("keyword must not be empty")
	}
	var compiled *regexp.Regexp
	if re {
		if err := ValidateRegexp(keyword); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		var err error
		compiled, err = regexp.Compile(keyword)
		if err != nil {
			return fmt.Errorf("compile regex: %w", err)
		}
	} else {
		var err error
		compiled, err = regexp.Compile("(?i)" + regexp.QuoteMeta(keyword))
		if err != nil {
			return fmt.Errorf("compile keyword regex: %w", err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keywordRules = append(r.keywordRules, KeywordRoute{
		Keyword:  keyword,
		Backend:  backendID,
		IsRegexp: re,
		compiled: compiled,
	})
	return nil
}

// RemoveKeywordRule 按序号删除规则
func (r *Router) RemoveKeywordRule(index int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < 0 || index >= len(r.keywordRules) {
		return false
	}
	r.keywordRules = append(r.keywordRules[:index], r.keywordRules[index+1:]...)
	return true
}

// GetKeywordRules 获取所有关键词规则
func (r *Router) GetKeywordRules() []KeywordRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rules := make([]KeywordRoute, len(r.keywordRules))
	copy(rules, r.keywordRules)
	return rules
}

// GetAllUserBackends 获取所有用户覆写
func (r *Router) GetAllUserBackends() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range r.userBackends {
		result[k] = v
	}
	return result
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

// ── 多后端路由模式 ──

const (
	ModeSingle = "single"
	ModeBoth   = "both"
	ModeThree  = "three"
)

// GetUserRouteMode 获取用户路由模式
func (r *Router) GetUserRouteMode(userID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if mode, ok := r.userRouteModes[userID]; ok {
		return mode
	}
	return ModeSingle
}

// SetUserRouteMode 设置用户路由模式 (single/both/three)
func (r *Router) SetUserRouteMode(userID, mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userRouteModes[userID] = mode
}

// ClearUserRouteMode 清除用户路由模式（恢复 single）
func (r *Router) ClearUserRouteMode(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.userRouteModes, userID)
}

// SetUserSecondaries 设置用户的第二/第三后端
func (r *Router) SetUserSecondaries(userID string, secondaries []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userSecondaries[userID] = secondaries
}

// GetUserSecondaries 获取用户的第二/第三后端
func (r *Router) GetUserSecondaries(userID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if sec, ok := r.userSecondaries[userID]; ok {
		return sec
	}
	return nil
}

// GetAllUserRouteModes 获取所有用户路由模式
func (r *Router) GetAllUserRouteModes() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range r.userRouteModes {
		result[k] = v
	}
	return result
}

// RouteMulti 多后端路由决策
// 根据用户的路由模式（single/both/three）返回多个后端
func (r *Router) RouteMulti(message string, userID string) []RouteDecision {
	primary := r.Route(message, userID)
	mode := r.GetUserRouteMode(userID)

	if mode == ModeSingle {
		return []RouteDecision{primary}
	}

	secondaries := r.GetUserSecondaries(userID)
	if len(secondaries) == 0 {
		return []RouteDecision{primary}
	}

	result := []RouteDecision{primary}
	for _, sid := range secondaries {
		if sid == primary.BackendID {
			continue
		}
		result = append(result, RouteDecision{
			BackendID: sid,
			MatchedBy: "session",
		})
		if mode == ModeBoth && len(result) >= 2 {
			break
		}
		if mode == ModeThree && len(result) >= 3 {
			break
		}
	}
	return result
}
