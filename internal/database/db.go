package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
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

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

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
		`CREATE TABLE IF NOT EXISTS routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			keyword TEXT NOT NULL,
			backend_id TEXT NOT NULL,
			is_regexp INTEGER DEFAULT 0,
			priority INTEGER DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now'))
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
			base_url TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			user_id TEXT PRIMARY KEY,
			backend_id TEXT NOT NULL DEFAULT '',
			route_mode TEXT DEFAULT 'single',
			secondaries TEXT DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS api_tokens (
			token TEXT PRIMARY KEY,
			name TEXT DEFAULT '',
			created_at TEXT DEFAULT (datetime('now'))
		)`,
	}

	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
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
