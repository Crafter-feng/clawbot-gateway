package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"

	"clawbot-gateway/internal/database"
)

// GenerateSecret 生成随机密钥
func GenerateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Config 运行时配置（从数据库加载）
type Config struct {
	Server   ServerConfig
	ClawBot  ClawBotConfig
	API      APIConfig
	Context  ContextConfig
	LogLevel string
}

type ServerConfig struct {
	Host string
	Port int
}

type ClawBotConfig struct {
	BaseURL      string
	PollTimeout  int
	BotType      int
}

type APIConfig struct {
	JWTSecret       string
	JWTExpiryHours  int
	AllowedOrigins  []string
	LoginPassword   string
}

type ContextConfig struct {
	MaxHistory      int
	SwitchStrategy  string
	TTL             int
}

// LoadFromDB 从数据库加载配置
func LoadFromDB(db *database.DB) *Config {
	cfg := &Config{}

	// 服务器配置（环境变量优先）
	cfg.Server.Host = getEnvOrDefault("CLAWBOT_HOST", db.GetSetting("server.host"))
	cfg.Server.Port = getIntEnvOrDefault("CLAWBOT_PORT", db.GetSetting("server.port"), 8080)

	// iLink 配置
	cfg.ClawBot.BaseURL = getEnvOrDefault("CLAWBOT_ILINK_BASE_URL", db.GetSetting("clawbot.base_url"))
	cfg.ClawBot.PollTimeout = getIntEnvOrDefault("CLAWBOT_POLL_TIMEOUT", db.GetSetting("clawbot.poll_timeout"), 35)
	cfg.ClawBot.BotType = getIntEnvOrDefault("CLAWBOT_BOT_TYPE", db.GetSetting("clawbot.bot_type"), 3)

	// API 配置
	cfg.API.JWTSecret = getEnvOrDefault("CLAWBOT_JWT_SECRET", "")
	if cfg.API.JWTSecret == "" {
		cfg.API.JWTSecret = db.GetSetting("api.jwt_secret")
	}
	if cfg.API.JWTSecret == "" {
		cfg.API.JWTSecret = GenerateSecret()
		db.SetSetting("api.jwt_secret", cfg.API.JWTSecret)
	}
	cfg.API.JWTExpiryHours = getIntEnvOrDefault("CLAWBOT_JWT_EXPIRY_HOURS", db.GetSetting("api.jwt_expiry_hours"), 24)
	cfg.API.LoginPassword = getEnvOrDefault("CLAWBOT_LOGIN_PASSWORD", "")
	if cfg.API.LoginPassword == "" {
		cfg.API.LoginPassword = db.GetSetting("api.login_password")
	}
	if cfg.API.LoginPassword == "" {
		b := make([]byte, 12)
		rand.Read(b)
		cfg.API.LoginPassword = "admin_" + hex.EncodeToString(b)
		db.SetSetting("api.login_password", cfg.API.LoginPassword)
	}
	cfg.API.AllowedOrigins = []string{getEnvOrDefault("CLAWBOT_ALLOWED_ORIGINS", db.GetSetting("api.allowed_origins"))}

	// 上下文配置
	cfg.Context.MaxHistory = getIntEnvOrDefault("CLAWBOT_MAX_HISTORY", db.GetSetting("context.max_history"), 20)
	cfg.Context.SwitchStrategy = getEnvOrDefault("CLAWBOT_SWITCH_STRATEGY", db.GetSetting("context.switch_strategy"))
	cfg.Context.TTL = getIntEnvOrDefault("CLAWBOT_CONTEXT_TTL", db.GetSetting("context.ttl"), 3600)

	// 日志级别
	cfg.LogLevel = getEnvOrDefault("CLAWBOT_LOG_LEVEL", "info")

	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getIntEnvOrDefault(key, strValue string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	if strValue != "" {
		if n, err := strconv.Atoi(strValue); err == nil {
			return n
		}
	}
	return defaultValue
}
