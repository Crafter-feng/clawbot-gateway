package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// GenerateSecret 生成随机密钥（32 字节 → 64 hex）
func GenerateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// envVarRe matches ${VAR_NAME} patterns (NOT bare $var) for safe YAML expansion
var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	ClawBot  ClawBotConfig  `yaml:"clawbot"`
	API      APIConfig      `yaml:"api"`
	Backend  BackendConfig  `yaml:"backend"`
	Context  ContextConfig  `yaml:"context"`
	LogLevel string         `yaml:"log_level"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type ClawBotAccount struct {
	Token     string `yaml:"token"`
	UserID    string `yaml:"user_id"`
	AccountID string `yaml:"account_id"`
	BaseURL   string `yaml:"base_url"`
	AccountName string `yaml:"account_name"`
}

type ClawBotConfig struct {
	Token         string            `yaml:"token"`
	BaseURL       string            `yaml:"base_url"`
	PollTimeout   int               `yaml:"poll_timeout"`
	MaxRetryLogin int               `yaml:"max_retry_login"`
	Accounts      []ClawBotAccount  `yaml:"accounts"`
	BotType       int               `yaml:"bot_type"`
}

type APIConfig struct {
	// Web 登录密码（管理后台 UI 登录使用）
	LoginPassword string `yaml:"login_password"`
	// JWT 签名密钥（不填自动生成）
	JWTSecret string `yaml:"jwt_secret"`
	// JWT 有效期（小时，默认 24）
	JWTExpiryHours int `yaml:"jwt_expiry_hours"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type BackendConfig struct {
	DefaultBackend string           `yaml:"default_backend"`
	Providers      []ProviderConfig `yaml:"providers"`
	KeywordRules   []KeywordRule    `yaml:"keyword_rules"`
}

type ProviderConfig struct {
	ID      string                 `yaml:"id"`
	Name    string                 `yaml:"name"`
	Type    string                 `yaml:"type"`
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:"config"`
}

type KeywordRule struct {
	Keywords []string `yaml:"keywords"`
	Backend  string   `yaml:"backend"`
}

type ContextConfig struct {
	MaxHistory     int    `yaml:"max_history"`
	SwitchStrategy string `yaml:"switch_strategy"`
	TTL            int    `yaml:"ttl"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// 展开 ${VAR_NAME} 环境变量引用（在 yaml 反序列化之前，确保 map[string]interface{} 值也被展开）
	// 使用 ${VAR} 语法而非 os.ExpandEnv（$var），避免意外展开 YAML 内容中的 $ 符号
	expanded := envVarRe.ReplaceAllStringFunc(string(data), func(match string) string {
		key := match[2 : len(match)-1]
		if v := os.Getenv(key); v != "" {
			return v
		}
		return match
	})
	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, err
	}
	setDefaults(cfg)
	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.ClawBot.BaseURL == "" {
		cfg.ClawBot.BaseURL = "https://ilinkai.weixin.qq.com"
	}
	if cfg.ClawBot.PollTimeout == 0 {
		cfg.ClawBot.PollTimeout = 35
	}
	if cfg.ClawBot.MaxRetryLogin == 0 {
		cfg.ClawBot.MaxRetryLogin = 3
	}
	if cfg.ClawBot.BotType == 0 {
		cfg.ClawBot.BotType = 3
	}

	if cfg.Backend.DefaultBackend == "" {
		cfg.Backend.DefaultBackend = "openclaw"
	}
	if cfg.Context.MaxHistory == 0 {
		cfg.Context.MaxHistory = 20
	}
	if cfg.Context.SwitchStrategy == "" {
		cfg.Context.SwitchStrategy = "keep"
	}
	if cfg.Context.TTL == 0 {
		cfg.Context.TTL = 3600
	}
	if cfg.API.AllowedOrigins == nil {
		cfg.API.AllowedOrigins = []string{"*"}
	}

	// ── 登录密码 ──
	// 环境变量 CLAWBOT_LOGIN_PASSWORD 优先
	if v := os.Getenv("CLAWBOT_LOGIN_PASSWORD"); v != "" {
		cfg.API.LoginPassword = v
	}
	// 如果为空，生成随机登录密码
	if cfg.API.LoginPassword == "" {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err == nil {
			cfg.API.LoginPassword = "admin_" + hex.EncodeToString(b)
		}
	}

	// ── JWT 配置 ──
	if v := os.Getenv("CLAWBOT_JWT_SECRET"); v != "" {
		cfg.API.JWTSecret = v
	}
	if cfg.API.JWTSecret == "" {
		cfg.API.JWTSecret = GenerateSecret()
	}
	if cfg.API.JWTExpiryHours == 0 {
		cfg.API.JWTExpiryHours = 24
	}
}
