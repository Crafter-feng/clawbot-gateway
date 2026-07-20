package database

import (
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewDB(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		t.Fatal("db is nil")
	}
}

func TestSettingsCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Set
	if err := db.SetSetting("test.key", "test.value"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Get
	value := db.GetSetting("test.key")
	if value != "test.value" {
		t.Errorf("GetSetting want 'test.value', got '%s'", value)
	}

	// Update
	if err := db.SetSetting("test.key", "updated.value"); err != nil {
		t.Fatalf("SetSetting update failed: %v", err)
	}
	value = db.GetSetting("test.key")
	if value != "updated.value" {
		t.Errorf("GetSetting after update want 'updated.value', got '%s'", value)
	}

	// Get default
	value = db.GetSetting("nonexistent.key")
	if value != "" {
		t.Errorf("GetSetting default want empty, got '%s'", value)
	}
}

func TestBackendsCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create
	b := Backend{
		ID:      "test-backend",
		Name:    "Test Backend",
		Type:    "echo",
		Config:  "{}",
		Enabled: true,
	}
	if err := db.CreateBackend(b); err != nil {
		t.Fatalf("CreateBackend failed: %v", err)
	}

	// Get
	got, err := db.GetBackend("test-backend")
	if err != nil {
		t.Fatalf("GetBackend failed: %v", err)
	}
	if got.Name != "Test Backend" {
		t.Errorf("GetBackend name want 'Test Backend', got '%s'", got.Name)
	}

	// List
	backends, err := db.ListBackends()
	if err != nil {
		t.Fatalf("ListBackends failed: %v", err)
	}
	if len(backends) != 1 {
		t.Errorf("ListBackends want 1, got %d", len(backends))
	}

	// Update
	b.Name = "Updated Backend"
	if err := db.UpdateBackend("test-backend", b); err != nil {
		t.Fatalf("UpdateBackend failed: %v", err)
	}
	got, _ = db.GetBackend("test-backend")
	if got.Name != "Updated Backend" {
		t.Errorf("UpdateBackend name want 'Updated Backend', got '%s'", got.Name)
	}

	// Delete
	if err := db.DeleteBackend("test-backend"); err != nil {
		t.Fatalf("DeleteBackend failed: %v", err)
	}
	got, _ = db.GetBackend("test-backend")
	if got != nil {
		t.Error("DeleteBackend: backend should be nil after deletion")
	}
}

func TestRoutesCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create
	r := Route{
		Keyword:   "天气",
		BackendID: "openclaw",
		IsRegexp:  false,
		Priority:  0,
	}
	if err := db.CreateRoute(r); err != nil {
		t.Fatalf("CreateRoute failed: %v", err)
	}

	// List
	routes, err := db.ListRoutes()
	if err != nil {
		t.Fatalf("ListRoutes failed: %v", err)
	}
	if len(routes) != 1 {
		t.Errorf("ListRoutes want 1, got %d", len(routes))
	}
	if routes[0].Keyword != "天气" {
		t.Errorf("Route keyword want '天气', got '%s'", routes[0].Keyword)
	}

	// Update
	routes[0].Keyword = "温度"
	if err := db.UpdateRoute(routes[0].ID, routes[0]); err != nil {
		t.Fatalf("UpdateRoute failed: %v", err)
	}

	// Delete
	if err := db.DeleteRoute(routes[0].ID); err != nil {
		t.Fatalf("DeleteRoute failed: %v", err)
	}
	routes, _ = db.ListRoutes()
	if len(routes) != 0 {
		t.Errorf("ListRoutes after delete want 0, got %d", len(routes))
	}
}

func TestAccountsCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Save
	a := Account{
		AccountID: "test_account",
		UserID:    "user123",
		Token:     "token123",
		BaseURL:   "https://example.com",
		LoginAt:   1234567890,
	}
	if err := db.SaveAccount(a); err != nil {
		t.Fatalf("SaveAccount failed: %v", err)
	}

	// Get
	got, err := db.GetAccount("test_account")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if got.Token != "token123" {
		t.Errorf("GetAccount token want 'token123', got '%s'", got.Token)
	}

	// List
	accounts, err := db.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("ListAccounts want 1, got %d", len(accounts))
	}

	// Delete
	if err := db.DeleteAccount("test_account"); err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}
	accounts, _ = db.ListAccounts()
	if len(accounts) != 0 {
		t.Errorf("ListAccounts after delete want 0, got %d", len(accounts))
	}
}

func TestVirtualBotsCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Save
	vb := VirtualBot{
		ID:        "test-bot",
		AccountID: "gw_test",
		UserID:    "gw_test@im.wechat",
		BaseURL:   "http://localhost:8080",
	}
	if err := db.SaveVirtualBot(vb); err != nil {
		t.Fatalf("SaveVirtualBot failed: %v", err)
	}

	// Get
	got, err := db.GetVirtualBot("test-bot")
	if err != nil {
		t.Fatalf("GetVirtualBot failed: %v", err)
	}
	if got.AccountID != "gw_test" {
		t.Errorf("GetVirtualBot account_id want 'gw_test', got '%s'", got.AccountID)
	}

	// List
	bots, err := db.ListVirtualBots()
	if err != nil {
		t.Fatalf("ListVirtualBots failed: %v", err)
	}
	if len(bots) != 1 {
		t.Errorf("ListVirtualBots want 1, got %d", len(bots))
	}

	// Delete
	if err := db.DeleteVirtualBot("test-bot"); err != nil {
		t.Fatalf("DeleteVirtualBot failed: %v", err)
	}
	bots, _ = db.ListVirtualBots()
	if len(bots) != 0 {
		t.Errorf("ListVirtualBots after delete want 0, got %d", len(bots))
	}
}

func TestSyncBuf(t *testing.T) {
	db := setupTestDB(t)

	// Set
	if err := db.SetSyncBuf("account1", "buf_value"); err != nil {
		t.Fatalf("SetSyncBuf failed: %v", err)
	}

	// Get
	buf := db.GetSyncBuf("account1")
	if buf != "buf_value" {
		t.Errorf("GetSyncBuf want 'buf_value', got '%s'", buf)
	}

	// Empty value should delete
	if err := db.SetSyncBuf("account1", ""); err != nil {
		t.Fatalf("SetSyncBuf empty failed: %v", err)
	}
	buf = db.GetSyncBuf("account1")
	if buf != "" {
		t.Errorf("GetSyncBuf after delete want empty, got '%s'", buf)
	}
}
