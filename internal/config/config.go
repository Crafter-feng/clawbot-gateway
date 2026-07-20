package config

import (
	"os"
	"strconv"

	"clawbot-gateway/internal/crypto"
	"clawbot-gateway/internal/database"
)

func GenerateSecret() string {
	return crypto.GenerateSecret(32)
}

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
	BaseURL     string
	PollTimeout int
	BotType     int
}

type APIConfig struct {
	JWTSecret      string
	JWTExpiryHours int
	AllowedOrigins []string
	LoginPassword  string
}

type ContextConfig struct {
	MaxHistory     int
	SwitchStrategy string
	TTL            int
}

// LoadFromDB 从数据库加载配置，环境变量始终优先
func LoadFromDB(db *database.DB) *Config {
	cfg := &Config{}

	// 服务器配置
	cfg.Server.Host = envOrDefault("CLAWBOT_HOST", db.GetSetting("server.host"))
	cfg.Server.Port = intEnvOrDefault("CLAWBOT_PORT", db.GetSetting("server.port"), 8080)

	// iLink 配置
	cfg.ClawBot.BaseURL = envOrDefault("CLAWBOT_ILINK_BASE_URL", db.GetSetting("clawbot.base_url"))
	cfg.ClawBot.PollTimeout = intEnvOrDefault("CLAWBOT_POLL_TIMEOUT", db.GetSetting("clawbot.poll_timeout"), 35)
	cfg.ClawBot.BotType = intEnvOrDefault("CLAWBOT_BOT_TYPE", db.GetSetting("clawbot.bot_type"), 3)

	// 密码：环境变量 > 数据库 > 自动生成
	cfg.API.LoginPassword = envOrDefault("CLAWBOT_LOGIN_PASSWORD", "")
	if cfg.API.LoginPassword == "" {
		cfg.API.LoginPassword = db.GetSetting("api.login_password")
	}
	if cfg.API.LoginPassword == "" {
		cfg.API.LoginPassword = "admin_" + crypto.GenerateSecret(6)
		db.SetSetting("api.login_password", cfg.API.LoginPassword)
	}

	// JWT 配置
	cfg.API.JWTSecret = envOrDefault("CLAWBOT_JWT_SECRET", "")
	if cfg.API.JWTSecret == "" {
		cfg.API.JWTSecret = db.GetSetting("api.jwt_secret")
	}
	if cfg.API.JWTSecret == "" {
		cfg.API.JWTSecret = GenerateSecret()
		db.SetSetting("api.jwt_secret", cfg.API.JWTSecret)
	}
	cfg.API.JWTExpiryHours = intEnvOrDefault("CLAWBOT_JWT_EXPIRY_HOURS", db.GetSetting("api.jwt_expiry_hours"), 24)
	cfg.API.AllowedOrigins = []string{envOrDefault("CLAWBOT_ALLOWED_ORIGINS", db.GetSetting("api.allowed_origins"))}

	// 上下文配置
	cfg.Context.MaxHistory = intEnvOrDefault("CLAWBOT_MAX_HISTORY", db.GetSetting("context.max_history"), 20)
	cfg.Context.SwitchStrategy = envOrDefault("CLAWBOT_SWITCH_STRATEGY", db.GetSetting("context.switch_strategy"))
	cfg.Context.TTL = intEnvOrDefault("CLAWBOT_CONTEXT_TTL", db.GetSetting("context.ttl"), 3600)

	// 日志
	cfg.LogLevel = envOrDefault("CLAWBOT_LOG_LEVEL", "info")

	return cfg
}

func envOrDefault(envKey, defaultValue string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultValue
}

func intEnvOrDefault(envKey, strValue string, defaultValue int) int {
	if v := os.Getenv(envKey); v != "" {
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
