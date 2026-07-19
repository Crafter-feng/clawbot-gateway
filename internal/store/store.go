package store

import (
	"encoding/json"
	"os"
	"sync"
)

// ── 持久化存储 ──

type Store struct {
	mu       sync.RWMutex
	filePath string
	data     *StoreData
}

type StoreData struct {
	KeywordRules    []StoredRule         `json:"keyword_rules"`
	UserBackends    map[string]string    `json:"user_backends"`
	UserRouteModes  map[string]string    `json:"user_route_modes,omitempty"`
	UserSecondaries map[string][]string  `json:"user_secondaries,omitempty"`

	Messages []StoredMessage `json:"messages,omitempty"`
	APIToken string          `json:"api_token,omitempty"`
}

type StoredRule struct {
	Keyword  string `json:"keyword"`
	Backend  string `json:"backend"`
	IsRegexp bool   `json:"is_regexp,omitempty"`
}

// ── 消息记录 ──

type StoredMessage struct {
	ID        string `json:"id"`
	ToUser    string `json:"to_user"`
	Content   string `json:"content"`
	MsgType   int    `json:"msg_type"`
	AccountID string `json:"account_id"`
	Timestamp int64  `json:"timestamp"`
	Direction string `json:"direction"` // "outgoing" or "incoming"
}

func (s *Store) SaveMessage(msg StoredMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Messages == nil {
		s.data.Messages = make([]StoredMessage, 0)
	}
	s.data.Messages = append(s.data.Messages, msg)
	// Keep only last 1000 messages to avoid unbounded growth
	if len(s.data.Messages) > 1000 {
		s.data.Messages = s.data.Messages[len(s.data.Messages)-1000:]
	}
	return s.save()
}

func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		data: &StoreData{
			KeywordRules:    make([]StoredRule, 0),
			UserBackends:    make(map[string]string),
			UserRouteModes:  make(map[string]string),
			UserSecondaries: make(map[string][]string),
		},
	}
	if err := s.load(); err != nil {
		// 文件不存在则初始化
		if os.IsNotExist(err) {
			return s, s.save()
		}
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, s.data)
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// ── 路由规则 ──

func (s *Store) GetKeywordRules() []StoredRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]StoredRule, len(s.data.KeywordRules))
	copy(result, s.data.KeywordRules)
	return result
}

func (s *Store) SetKeywordRules(rules []StoredRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.KeywordRules = rules
	return s.save()
}

// ── 用户后端 ──

func (s *Store) GetUserBackends() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range s.data.UserBackends {
		result[k] = v
	}
	return result
}

func (s *Store) SetUserBackend(userID, backendID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.UserBackends[userID] = backendID
	return s.save()
}

func (s *Store) ClearUserBackend(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.UserBackends, userID)
	return s.save()
}

// ── 用户路由模式 ──

func (s *Store) GetUserRouteModes() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range s.data.UserRouteModes {
		result[k] = v
	}
	return result
}

func (s *Store) SetUserRouteMode(userID, mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.UserRouteModes[userID] = mode
	return s.save()
}

func (s *Store) ClearUserRouteMode(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.UserRouteModes, userID)
	return s.save()
}

func (s *Store) GetUserSecondaries() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string][]string)
	for k, v := range s.data.UserSecondaries {
		cp := make([]string, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}

func (s *Store) SetUserSecondaries(userID string, secondaries []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.UserSecondaries[userID] = secondaries
	return s.save()
}

func (s *Store) ClearUserSecondaries(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.UserSecondaries, userID)
	return s.save()
}

// ── API Token 管理 ──

func (s *Store) GetAPIToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.APIToken
}

func (s *Store) SetAPIToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.APIToken = token
	return s.save()
}
