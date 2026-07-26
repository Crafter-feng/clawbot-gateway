package config

import (
	"os"
	"testing"

	"clawbot-gateway/internal/database"

	_ "modernc.org/sqlite"
)

func TestGenerateSecret(t *testing.T) {
	secret := GenerateSecret()
	if len(secret) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("GenerateSecret length want 64, got %d", len(secret))
	}

	// Test uniqueness
	secret2 := GenerateSecret()
	if secret == secret2 {
		t.Error("GenerateSecret should produce unique values")
	}
}

func TestLoadFromDB(t *testing.T) {
	// Create test database
	dbPath := t.TempDir() + "/test.db"
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Load config
	cfg := LoadFromDB(db)

	// Check defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host want '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port want 8080, got %d", cfg.Server.Port)
	}
	if cfg.ClawBot.BaseURL != "https://ilinkai.weixin.qq.com" {
		t.Errorf("ClawBot.BaseURL want 'https://ilinkai.weixin.qq.com', got '%s'", cfg.ClawBot.BaseURL)
	}
	if cfg.ClawBot.PollTimeout != 35 {
		t.Errorf("ClawBot.PollTimeout want 35, got %d", cfg.ClawBot.PollTimeout)
	}
	if cfg.Context.MaxHistory != 20 {
		t.Errorf("Context.MaxHistory want 20, got %d", cfg.Context.MaxHistory)
	}
	if cfg.Context.TTL != 3600 {
		t.Errorf("Context.TTL want 3600, got %d", cfg.Context.TTL)
	}
}

func TestLoadFromDBWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("CLAWBOT_HOST", "127.0.0.1")
	os.Setenv("CLAWBOT_PORT", "9090")
	defer os.Unsetenv("CLAWBOT_HOST")
	defer os.Unsetenv("CLAWBOT_PORT")

	dbPath := t.TempDir() + "/test.db"
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := LoadFromDB(db)

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host want '127.0.0.1', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port want 9090, got %d", cfg.Server.Port)
	}
}

func TestLoadFromDBPasswordPriority(t *testing.T) {
	// Clean up any existing env var to ensure clean state
	origPassword := os.Getenv("CLAWBOT_LOGIN_PASSWORD")
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	defer os.Setenv("CLAWBOT_LOGIN_PASSWORD", origPassword)


	// 1. Auto-generate
	dbPath := t.TempDir() + "/test.db"
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := LoadFromDB(db)
	if cfg.API.LoginPassword == "" {
		t.Error("LoginPassword should be auto-generated")
	}
	if len(cfg.API.LoginPassword) < 10 {
		t.Errorf("LoginPassword too short: %s", cfg.API.LoginPassword)
	}

	// 2. Database value
	db.SetSetting("api.login_password", "db_password")
	cfg = LoadFromDB(db)
	if cfg.API.LoginPassword != "db_password" {
		t.Errorf("LoginPassword want 'db_password', got '%s'", cfg.API.LoginPassword)
	}

	// 3. Environment variable (highest priority)
	os.Setenv("CLAWBOT_LOGIN_PASSWORD", "env_password")
	defer os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	cfg = LoadFromDB(db)
	if cfg.API.LoginPassword != "env_password" {
		t.Errorf("LoginPassword want 'env_password', got '%s'", cfg.API.LoginPassword)
	}
}
