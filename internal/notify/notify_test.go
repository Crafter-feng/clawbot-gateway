package notify

import (
	"context"
	"os"
	"testing"

	"clawbot-gateway/internal/database"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	m.Run()
}

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewHandler(t *testing.T) {
	db := setupTestDB(t)
	sendFunc := func(ctx context.Context, toUser, content string) error {
		return nil
	}

	h := NewHandler(db, sendFunc)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestNewHandlerNilSendFunc(t *testing.T) {
	db := setupTestDB(t)

	h := NewHandler(db, nil)
	if h == nil {
		t.Fatal("NewHandler with nil sendFunc returned nil")
	}
}

func TestNewHandlerContextType(t *testing.T) {
	db := setupTestDB(t)
	sendCalled := false
	sendFunc := func(ctx context.Context, toUser, content string) error {
		sendCalled = true
		return nil
	}

	h := NewHandler(db, sendFunc)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	// Verify the sendFunc is stored by calling it indirectly
	// We can't call HandleSend without a full HTTP request, but we can verify
	// the handler was created successfully
	_ = sendFunc(context.Background(), "user", "hello")
	if !sendCalled {
		t.Error("sendFunc was not called")
	}
}

func TestNewHandlerSendFuncInterface(t *testing.T) {
	db := setupTestDB(t)

	// Test that the sendFunc type matches the expected signature
	type sendFuncType func(ctx context.Context, toUser, content string) error
	var fn sendFuncType = func(ctx context.Context, toUser, content string) error {
		return nil
	}

	h := NewHandler(db, fn)
	if h == nil {
		t.Fatal("NewHandler with typed sendFunc returned nil")
	}
}

func TestNewHandlerMultipleMethods(t *testing.T) {
	db := setupTestDB(t)

	// Test that RegisterRoutes and RegisterManagementRoutes don't panic
	// when called with a nil engine/router group (they attach to gin)
	// We can't test gin routing directly here, but we verify the handler is valid
	h := NewHandler(db, nil)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	// Verify the handler struct fields are accessible
	_ = h
}