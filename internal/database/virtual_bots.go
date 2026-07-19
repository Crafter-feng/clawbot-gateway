package database

type VirtualBot struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	UserID    string `json:"user_id"`
	BaseURL   string `json:"base_url"`
}

func (db *DB) ListVirtualBots() ([]VirtualBot, error) {
	rows, err := db.conn.Query("SELECT id, account_id, user_id, base_url FROM virtual_bots")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VirtualBot
	for rows.Next() {
		var vb VirtualBot
		if err := rows.Scan(&vb.ID, &vb.AccountID, &vb.UserID, &vb.BaseURL); err != nil {
			continue
		}
		result = append(result, vb)
	}
	return result, nil
}

func (db *DB) GetVirtualBot(id string) (*VirtualBot, error) {
	var vb VirtualBot
	err := db.conn.QueryRow("SELECT id, account_id, user_id, base_url FROM virtual_bots WHERE id = ?", id).
		Scan(&vb.ID, &vb.AccountID, &vb.UserID, &vb.BaseURL)
	if err != nil {
		return nil, err
	}
	return &vb, nil
}

func (db *DB) GetVirtualBotByAccountID(accountID string) (*VirtualBot, error) {
	var vb VirtualBot
	err := db.conn.QueryRow("SELECT id, account_id, user_id, base_url FROM virtual_bots WHERE account_id = ?", accountID).
		Scan(&vb.ID, &vb.AccountID, &vb.UserID, &vb.BaseURL)
	if err != nil {
		return nil, err
	}
	return &vb, nil
}

func (db *DB) SaveVirtualBot(vb VirtualBot) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO virtual_bots (id, account_id, user_id, base_url) VALUES (?, ?, ?, ?)",
		vb.ID, vb.AccountID, vb.UserID, vb.BaseURL,
	)
	return err
}

func (db *DB) DeleteVirtualBot(id string) error {
	_, err := db.conn.Exec("DELETE FROM virtual_bots WHERE id = ?", id)
	return err
}

// ── User Sessions ──

type UserSession struct {
	UserID      string `json:"user_id"`
	BackendID   string `json:"backend_id"`
	RouteMode   string `json:"route_mode"`
	Secondaries string `json:"secondaries"`
}

func (db *DB) GetUserSession(userID string) (*UserSession, error) {
	var s UserSession
	err := db.conn.QueryRow("SELECT user_id, backend_id, route_mode, secondaries FROM user_sessions WHERE user_id = ?", userID).
		Scan(&s.UserID, &s.BackendID, &s.RouteMode, &s.Secondaries)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) GetAllUserSessions() ([]UserSession, error) {
	rows, err := db.conn.Query("SELECT user_id, backend_id, route_mode, secondaries FROM user_sessions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UserSession
	for rows.Next() {
		var s UserSession
		if err := rows.Scan(&s.UserID, &s.BackendID, &s.RouteMode, &s.Secondaries); err != nil {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}

func (db *DB) SaveUserSession(s UserSession) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO user_sessions (user_id, backend_id, route_mode, secondaries) VALUES (?, ?, ?, ?)",
		s.UserID, s.BackendID, s.RouteMode, s.Secondaries,
	)
	return err
}

func (db *DB) DeleteUserSession(userID string) error {
	_, err := db.conn.Exec("DELETE FROM user_sessions WHERE user_id = ?", userID)
	return err
}
