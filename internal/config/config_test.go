package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// 写一个最小配置
	yaml := `---
server:
  host: "0.0.0.0"
  port: 8080
`
	tmp := t.TempDir() + "/test_config.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("port want 8080, got %d", cfg.Server.Port)
	}
	if cfg.ClawBot.BaseURL != "https://ilinkai.weixin.qq.com" {
		t.Errorf("default base_url want ...ilinkai..., got %s", cfg.ClawBot.BaseURL)
	}
	if cfg.ClawBot.PollTimeout != 35 {
		t.Errorf("default poll_timeout want 35, got %d", cfg.ClawBot.PollTimeout)
	}
	if cfg.Backend.DefaultBackend != "openclaw" {
		t.Errorf("default_backend want openclaw, got %s", cfg.Backend.DefaultBackend)
	}
}

func TestEnvAuthTokenOverride(t *testing.T) {
	t.Setenv("CLAWBOT_LOGIN_PASSWORD", "env_password_value")
	yaml := `---
server:
  host: "0.0.0.0"
  port: 8080
`
	tmp := t.TempDir() + "/env_config.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.API.LoginPassword != "env_password_value" {
		t.Errorf("expected env_password_value, got %s", cfg.API.LoginPassword)
	}
}

func TestLoadFullConfig(t *testing.T) {
	yaml := `---
server:
  host: "127.0.0.1"
  port: 9090

clawbot:
  token: "test_token"
  base_url: "https://test.ilink.com"
  poll_timeout: 30
  max_retry_login: 5

backend:
  default_backend: "claude"
  providers:
    - id: "test"
      name: "Test"
      type: echo
      enabled: true
      config: {}
  keyword_rules:
    - keywords: ["天气", "温度"]
      backend: "openclaw"

context:
  max_history: 50
  switch_strategy: clear
  ttl: 7200
`
	tmp := t.TempDir() + "/full_config.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("port want 9090, got %d", cfg.Server.Port)
	}
	if cfg.ClawBot.Token != "test_token" {
		t.Errorf("token want test_token, got %s", cfg.ClawBot.Token)
	}
	if cfg.ClawBot.PollTimeout != 30 {
		t.Errorf("poll_timeout want 30, got %d", cfg.ClawBot.PollTimeout)
	}
	if len(cfg.Backend.Providers) != 1 || cfg.Backend.Providers[0].ID != "test" {
		t.Errorf("provider id want test, got %+v", cfg.Backend.Providers)
	}
	if len(cfg.Backend.KeywordRules) != 1 || cfg.Backend.KeywordRules[0].Keywords[0] != "天气" {
		t.Errorf("keyword rule want 天气, got %+v", cfg.Backend.KeywordRules)
	}
	if cfg.Context.MaxHistory != 50 {
		t.Errorf("max_history want 50, got %d", cfg.Context.MaxHistory)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_API_KEY", "sk-test123")
	t.Setenv("TEST_BASE_URL", "https://test.example.com/v1")
	yaml := `---
server:
  host: "0.0.0.0"
  port: 8080
backend:
  providers:
    - id: test
      name: "Test"
      type: openai_compatible
      enabled: true
      config:
        api_key: "${TEST_API_KEY}"
        base_url: "${TEST_BASE_URL}"
        model: "gpt-4o"
`
	tmp := t.TempDir() + "/env_config.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Backend.Providers) != 1 {
		t.Fatalf("want 1 provider, got %d", len(cfg.Backend.Providers))
	}
	apiKey, _ := cfg.Backend.Providers[0].Config["api_key"].(string)
	if apiKey != "sk-test123" {
		t.Errorf("api_key want 'sk-test123', got %q", apiKey)
	}
	baseURL, _ := cfg.Backend.Providers[0].Config["base_url"].(string)
	if baseURL != "https://test.example.com/v1" {
		t.Errorf("base_url want 'https://test.example.com/v1', got %q", baseURL)
	}
}

func TestEnvVarExpansionUnset(t *testing.T) {
	// 未设置的环境变量应保留原样 ${VAR}
	yaml := `---
server:
  host: "0.0.0.0"
  port: 8080
backend:
  providers:
    - id: echo
      name: "Echo"
      type: echo
      enabled: true
      config:
        api_key: "${UNSET_VAR_12345}"
`
	tmp := t.TempDir() + "/unset_env_config.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	apiKey, _ := cfg.Backend.Providers[0].Config["api_key"].(string)
	if apiKey != "${UNSET_VAR_12345}" {
		t.Errorf("unset var should remain literal, got %q", apiKey)
	}
}

func TestMultiAccountConfig(t *testing.T) {
	yaml := `---
server:
  host: "0.0.0.0"
  port: 8080
clawbot:
  accounts:
    - token: "token_alpha"
      user_id: "alice"
      account_id: "wx_alice"
      account_name: "Alice's WeChat"
    - token: "token_beta"
      user_id: "bob"
      base_url: "https://custom.ilink.com"
`
	tmp := t.TempDir() + "/multi_account.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.ClawBot.Accounts) != 2 {
		t.Fatalf("want 2 accounts, got %d", len(cfg.ClawBot.Accounts))
	}
	if cfg.ClawBot.Accounts[0].Token != "token_alpha" {
		t.Errorf("account[0].token want token_alpha, got %s", cfg.ClawBot.Accounts[0].Token)
	}
	if cfg.ClawBot.Accounts[0].UserID != "alice" {
		t.Errorf("account[0].user_id want alice, got %s", cfg.ClawBot.Accounts[0].UserID)
	}
	if cfg.ClawBot.Accounts[0].AccountID != "wx_alice" {
		t.Errorf("account[0].account_id want wx_alice, got %s", cfg.ClawBot.Accounts[0].AccountID)
	}
	if cfg.ClawBot.Accounts[0].AccountName != "Alice's WeChat" {
		t.Errorf("account[0].account_name want 'Alice\\'s WeChat', got %s", cfg.ClawBot.Accounts[0].AccountName)
	}
	if cfg.ClawBot.Accounts[1].Token != "token_beta" {
		t.Errorf("account[1].token want token_beta, got %s", cfg.ClawBot.Accounts[1].Token)
	}
	if cfg.ClawBot.Accounts[1].UserID != "bob" {
		t.Errorf("account[1].user_id want bob, got %s", cfg.ClawBot.Accounts[1].UserID)
	}
	if cfg.ClawBot.Accounts[1].BaseURL != "https://custom.ilink.com" {
		t.Errorf("account[1].base_url want https://custom.ilink.com, got %s", cfg.ClawBot.Accounts[1].BaseURL)
	}
}

func TestEnvVarExpansionInAccounts(t *testing.T) {
	t.Setenv("BOT1_TOKEN", "real_token_1")
	t.Setenv("BOT1_USER", "charlie")
	yaml := `---
server:
  host: "0.0.0.0"
  port: 8080
clawbot:
  accounts:
    - token: "${BOT1_TOKEN}"
      user_id: "${BOT1_USER}"
`
	tmp := t.TempDir() + "/env_account_config.yaml"
	if err := os.WriteFile(tmp, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.ClawBot.Accounts) != 1 {
		t.Fatalf("want 1 account, got %d", len(cfg.ClawBot.Accounts))
	}
	if cfg.ClawBot.Accounts[0].Token != "real_token_1" {
		t.Errorf("token want 'real_token_1', got %q", cfg.ClawBot.Accounts[0].Token)
	}
	if cfg.ClawBot.Accounts[0].UserID != "charlie" {
		t.Errorf("user_id want 'charlie', got %q", cfg.ClawBot.Accounts[0].UserID)
	}
}
