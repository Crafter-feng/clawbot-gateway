package session

import (
	"strings"
	"sync"
	"time"
)

// SessionContext 用户会话上下文
type SessionContext struct {
	mu         sync.Mutex
	UserID     string
	BackendID  string
	History    []ChatMessage
	MaxHistory int
	CreatedAt  time.Time
	LastActive time.Time
}

type ChatMessage struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

func NewSessionContext(userID string, maxHistory int) *SessionContext {
	now := time.Now()
	return &SessionContext{
		UserID:     userID,
		MaxHistory: maxHistory,
		History:    make([]ChatMessage, 0, maxHistory),
		CreatedAt:  now,
		LastActive: now,
	}
}

func (s *SessionContext) AddTurn(message, reply string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, ChatMessage{Role: "user", Content: message})
	s.History = append(s.History, ChatMessage{Role: "assistant", Content: reply})
	if len(s.History) > s.MaxHistory*2 {
		s.History = s.History[len(s.History)-s.MaxHistory*2:]
	}
	s.LastActive = time.Now()
}

func (s *SessionContext) GetHistory() []ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]ChatMessage, len(s.History))
	copy(result, s.History)
	return result
}

func (s *SessionContext) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = s.History[:0]
}

func (s *SessionContext) IsExpired(ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.LastActive) > ttl
}

// ── Context Manager ──

type ContextManager struct {
	mu         sync.RWMutex
	sessions   map[string]*SessionContext
	maxHistory int
	ttl        time.Duration
}

func NewContextManager(maxHistory int, ttl time.Duration) *ContextManager {
	return &ContextManager{
		sessions:   make(map[string]*SessionContext),
		maxHistory: maxHistory,
		ttl:        ttl,
	}
}

func (cm *ContextManager) GetContext(userID string, backendID string) *SessionContext {
	key := cm.buildKey(userID, backendID)
	cm.mu.RLock()
	if s, ok := cm.sessions[key]; ok {
		cm.mu.RUnlock()
		return s
	}
	cm.mu.RUnlock()
	cm.mu.Lock()
	defer cm.mu.Unlock()
	// Double-check after acquiring write lock
	if s, ok := cm.sessions[key]; ok {
		return s
	}
	s := NewSessionContext(userID, cm.maxHistory)
	s.BackendID = backendID
	cm.sessions[key] = s
	return s
}

func (cm *ContextManager) ClearContext(userID string, backendID string) {
	key := cm.buildKey(userID, backendID)
	cm.mu.Lock()
	delete(cm.sessions, key)
	cm.mu.Unlock()
}

func (cm *ContextManager) ClearAllUserContext(userID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for k := range cm.sessions {
		if strings.HasPrefix(k, userID+":") {
			delete(cm.sessions, k)
		}
	}
}

func (cm *ContextManager) CleanupExpired() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	now := time.Now()
	for k, s := range cm.sessions {
		if now.Sub(s.LastActive) > cm.ttl {
			delete(cm.sessions, k)
		}
	}
}

func (cm *ContextManager) buildKey(userID, backendID string) string {
	return userID + ":" + backendID
}

func (cm *ContextManager) SessionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.sessions)
}

func (cm *ContextManager) SessionCountForUser(userID string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	count := 0
	prefix := userID + ":"
	for k := range cm.sessions {
		if strings.HasPrefix(k, prefix) {
			count++
		}
	}
	return count
}
