package database

import "os"

// 默认配置值
var defaults = map[string]string{
	"server.host":           "0.0.0.0",
	"server.port":           "8080",
	"clawbot.base_url":      "https://ilinkai.weixin.qq.com",
	"clawbot.poll_timeout":  "35",
	"clawbot.bot_type":      "3",
	"api.jwt_expiry_hours":  "24",
	"api.allowed_origins":   "*",
	"context.max_history":   "20",
	"context.switch_strategy": "keep",
	"context.ttl":           "3600",
	"route.default_backend": "echo",
}

// GetSetting 获取配置，优先环境变量
func (db *DB) GetSetting(key string) string {
	// 环境变量优先
	envKey := "CLAWBOT_" + toEnvKey(key)
	if v := os.Getenv(envKey); v != "" {
		return v
	}

	var value string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == nil {
		return value
	}

	return defaults[key]
}

// SetSetting 设置配置
func (db *DB) SetSetting(key, value string) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
		key, value,
	)
	return err
}

// GetAllSettings 获取所有配置
func (db *DB) GetAllSettings() map[string]string {
	result := make(map[string]string)
	for k, v := range defaults {
		result[k] = v
	}

	rows, err := db.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			result[k] = v
		}
	}

	// 环境变量覆盖
	for k := range result {
		envKey := "CLAWBOT_" + toEnvKey(k)
		if v := os.Getenv(envKey); v != "" {
			result[k] = v
		}
	}

	return result
}

func toEnvKey(key string) string {
	result := ""
	for _, c := range key {
		if c == '.' {
			result += "_"
		} else {
			result += string(c)
		}
	}
	return result
}
