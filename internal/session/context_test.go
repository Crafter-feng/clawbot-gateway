package session

import (
	"testing"
	"time"
)

func TestNewSessionContext(t *testing.T) {
	sc := NewSessionContext("user1", 20)
	if sc.UserID != "user1" {
		t.Errorf("UserID want 'user1', got '%s'", sc.UserID)
	}
	if sc.MaxHistory != 20 {
		t.Errorf("MaxHistory want 20, got %d", sc.MaxHistory)
	}
	if len(sc.History) != 0 {
		t.Errorf("History want empty, got %d", len(sc.History))
	}
}

func TestSessionContextAddTurn(t *testing.T) {
	sc := NewSessionContext("user1", 20)
	sc.AddTurn("hello", "hi there")

	if len(sc.History) != 2 {
		t.Errorf("History length want 2, got %d", len(sc.History))
	}
	if sc.History[0].Role != "user" {
		t.Errorf("First message role want 'user', got '%s'", sc.History[0].Role)
	}
	if sc.History[0].Content != "hello" {
		t.Errorf("First message content want 'hello', got '%s'", sc.History[0].Content)
	}
	if sc.History[1].Role != "assistant" {
		t.Errorf("Second message role want 'assistant', got '%s'", sc.History[1].Role)
	}
	if sc.History[1].Content != "hi there" {
		t.Errorf("Second message content want 'hi there', got '%s'", sc.History[1].Content)
	}
}

func TestSessionContextMaxHistory(t *testing.T) {
	sc := NewSessionContext("user1", 2) // Max 2 turns

	// Add 3 turns (6 messages)
	sc.AddTurn("msg1", "reply1")
	sc.AddTurn("msg2", "reply2")
	sc.AddTurn("msg3", "reply3")

	// Should keep only last 2 turns (4 messages)
	if len(sc.History) != 4 {
		t.Errorf("History length want 4, got %d", len(sc.History))
	}
	if sc.History[0].Content != "msg2" {
		t.Errorf("First message want 'msg2', got '%s'", sc.History[0].Content)
	}
}

func TestSessionContextClear(t *testing.T) {
	sc := NewSessionContext("user1", 20)
	sc.AddTurn("hello", "hi")
	sc.Clear()

	if len(sc.History) != 0 {
		t.Errorf("History want empty after clear, got %d", len(sc.History))
	}
}

func TestSessionContextIsExpired(t *testing.T) {
	sc := NewSessionContext("user1", 20)
	sc.LastActive = time.Now().Add(-2 * time.Hour)

	if !sc.IsExpired(1 * time.Hour) {
		t.Error("IsExpired should return true for old session")
	}

	sc.LastActive = time.Now()
	if sc.IsExpired(1 * time.Hour) {
		t.Error("IsExpired should return false for recent session")
	}
}

func TestContextManagerGetContext(t *testing.T) {
	cm := NewContextManager(20, 1*time.Hour)

	// Get new context
	ctx := cm.GetContext("user1", "backend1")
	if ctx.UserID != "user1" {
		t.Errorf("UserID want 'user1', got '%s'", ctx.UserID)
	}
	if ctx.BackendID != "backend1" {
		t.Errorf("BackendID want 'backend1', got '%s'", ctx.BackendID)
	}

	// Get same context
	ctx2 := cm.GetContext("user1", "backend1")
	if ctx != ctx2 {
		t.Error("GetContext should return same instance")
	}

	// Different backend gets different context
	ctx3 := cm.GetContext("user1", "backend2")
	if ctx == ctx3 {
		t.Error("Different backend should get different context")
	}
}

func TestContextManagerSessionCount(t *testing.T) {
	cm := NewContextManager(20, 1*time.Hour)

	cm.GetContext("user1", "backend1")
	cm.GetContext("user2", "backend1")
	cm.GetContext("user1", "backend2")

	if cm.SessionCount() != 3 {
		t.Errorf("SessionCount want 3, got %d", cm.SessionCount())
	}
}

func TestContextManagerClearContext(t *testing.T) {
	cm := NewContextManager(20, 1*time.Hour)

	cm.GetContext("user1", "backend1")
	cm.ClearContext("user1", "backend1")

	// Should create new context after clear
	ctx := cm.GetContext("user1", "backend1")
	if ctx == nil {
		t.Error("GetContext after clear should return new context")
	}
}

func TestContextManagerCleanupExpired(t *testing.T) {
	cm := NewContextManager(20, 1*time.Second) // 1 second TTL

	cm.GetContext("user1", "backend1")

	// Wait for expiry
	time.Sleep(2 * time.Second)

	cm.CleanupExpired()

	if cm.SessionCount() != 0 {
		t.Errorf("SessionCount after cleanup want 0, got %d", cm.SessionCount())
	}
}
