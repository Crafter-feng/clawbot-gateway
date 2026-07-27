package database

type NotifyToken struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"` // 绑定的微信账号（空=全部）
	Name      string `json:"name"`
	Token     string `json:"token"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func (db *DB) ListNotifyTokens() ([]NotifyToken, error) {
	rows, err := db.conn.Query("SELECT id, account_id, name, token, enabled, created_at FROM notify_tokens ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []NotifyToken
	for rows.Next() {
		var t NotifyToken
		var enabled int
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Name, &t.Token, &enabled, &t.CreatedAt); err != nil {
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

func (db *DB) GetNotifyToken(token string) (*NotifyToken, error) {
	var t NotifyToken
	var enabled int
	err := db.conn.QueryRow("SELECT id, account_id, name, token, enabled, created_at FROM notify_tokens WHERE token = ?", token).
		Scan(&t.ID, &t.AccountID, &t.Name, &t.Token, &enabled, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.Enabled = enabled == 1
	return &t, nil
}

func (db *DB) CreateNotifyToken(t NotifyToken) error {
	enabled := 0
	if t.Enabled {
		enabled = 1
	}
	_, err := db.conn.Exec(
		"INSERT INTO notify_tokens (id, account_id, name, token, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		t.ID, t.AccountID, t.Name, t.Token, enabled, t.CreatedAt,
	)
	return err
}

func (db *DB) DeleteNotifyToken(id string) error {
	_, err := db.conn.Exec("DELETE FROM notify_tokens WHERE id = ?", id)
	return err
}
