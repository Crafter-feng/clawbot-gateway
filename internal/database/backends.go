package database

import (
	"database/sql"
	"time"
)

type Backend struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Config    string `json:"config"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (db *DB) ListBackends() ([]Backend, error) {
	rows, err := db.conn.Query("SELECT id, name, type, config, enabled FROM backends ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Backend, 0)
	for rows.Next() {
		var b Backend
		var enabled int
		if err := rows.Scan(&b.ID, &b.Name, &b.Type, &b.Config, &enabled); err != nil {
			continue
		}
		b.Enabled = enabled == 1
		result = append(result, b)
	}
	return result, nil
}

func (db *DB) GetBackend(id string) (*Backend, error) {
	var b Backend
	var enabled int
	err := db.conn.QueryRow("SELECT id, name, type, config, enabled FROM backends WHERE id = ?", id).
		Scan(&b.ID, &b.Name, &b.Type, &b.Config, &enabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.Enabled = enabled == 1
	return &b, nil
}

func (db *DB) CreateBackend(b Backend) error {
	enabled := 0
	if b.Enabled {
		enabled = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.Exec(
		"INSERT INTO backends (id, name, type, config, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		b.ID, b.Name, b.Type, b.Config, enabled, now, now,
	)
	return err
}

func (db *DB) UpdateBackend(id string, b Backend) error {
	enabled := 0
	if b.Enabled {
		enabled = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.Exec(
		"UPDATE backends SET name=?, type=?, config=?, enabled=?, updated_at=? WHERE id=?",
		b.Name, b.Type, b.Config, enabled, now, id,
	)
	return err
}

func (db *DB) DeleteBackend(id string) error {
	_, err := db.conn.Exec("DELETE FROM backends WHERE id = ?", id)
	return err
}
