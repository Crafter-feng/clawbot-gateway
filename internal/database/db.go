package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB 数据库连接管理
type DB struct {
	conn     *sql.DB
	filePath string
}

// New 创建数据库连接并运行迁移
func New(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	db := &DB{
		conn:     conn,
		filePath: dbPath,
	}

	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS backends (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			config TEXT DEFAULT '{}',
			enabled INTEGER DEFAULT 1,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			account_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token TEXT NOT NULL,
			base_url TEXT NOT NULL DEFAULT 'https://ilinkai.weixin.qq.com',
			account_name TEXT DEFAULT '',
			login_at INTEGER DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS virtual_bots (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL UNIQUE,
			user_id TEXT NOT NULL,
			base_url TEXT NOT NULL,
			token TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			user_id TEXT PRIMARY KEY,
			backend_id TEXT NOT NULL DEFAULT '',
			route_mode TEXT DEFAULT 'single',
			secondaries TEXT DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS notify_tokens (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			enabled INTEGER DEFAULT 1,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		// 新的路由规则表（支持且/或/非逻辑）
		`CREATE TABLE IF NOT EXISTS route_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			backend_id TEXT NOT NULL,
			priority INTEGER DEFAULT 0,
			enabled INTEGER DEFAULT 1,
			description TEXT DEFAULT '',
			groups TEXT NOT NULL DEFAULT '[]',
			group_logic TEXT DEFAULT 'and',
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now'))
		)`,
	}
	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
		}
	}

	// 迁移：添加 token 列到 virtual_bots（兼容旧数据库）
	if _, err := db.conn.Exec("ALTER TABLE virtual_bots ADD COLUMN token TEXT NOT NULL DEFAULT ''"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate virtual_bots: %w", err)
		}
	}

	return nil
}

// ── SyncBuf (轮询游标) ──

func (db *DB) GetSyncBuf(accountID string) string {
	var buf string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", "syncbuf:"+accountID).Scan(&buf)
	if err != nil {
		return ""
	}
	return buf
}

func (db *DB) SetSyncBuf(accountID, buf string) error {
	if buf == "" {
		_, err := db.conn.Exec("DELETE FROM settings WHERE key = ?", "syncbuf:"+accountID)
		return err
	}
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
		"syncbuf:"+accountID, buf,
	)
	return err
}
