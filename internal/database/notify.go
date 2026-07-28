package database

import "log"

type NotifyToken struct {
	ID        string `json:"id"`
	ToUser    string `json:"to_user"` // 绑定的推送目标（空=全部客户）
	Name      string `json:"name"`
	Token     string `json:"token"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func (db *DB) ListNotifyTokens() ([]NotifyToken, error) {
	rows, err := db.conn.Query("SELECT id, to_user, name, token, enabled, created_at FROM notify_tokens ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []NotifyToken
	for rows.Next() {
		var t NotifyToken
		var enabled int
		if err := rows.Scan(&t.ID, &t.ToUser, &t.Name, &t.Token, &enabled, &t.CreatedAt); err != nil {
			log.Printf("scan error: %v", err)
			continue
		}
		t.Enabled = enabled == 1
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}


func (db *DB) CreateNotifyToken(t NotifyToken) error {
	enabled := 0
	if t.Enabled {
		enabled = 1
	}
	_, err := db.conn.Exec(
		"INSERT INTO notify_tokens (id, to_user, name, token, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		t.ID, t.ToUser, t.Name, t.Token, enabled, t.CreatedAt,
	)
	return err
}

func (db *DB) DeleteNotifyToken(id string) error {
	_, err := db.conn.Exec("DELETE FROM notify_tokens WHERE id = ?", id)
	return err
}
