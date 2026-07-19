package database

type Account struct {
	AccountID   string `json:"account_id"`
	UserID      string `json:"user_id"`
	Token       string `json:"token"`
	BaseURL     string `json:"base_url"`
	AccountName string `json:"account_name"`
	LoginAt     int64  `json:"login_at"`
}

func (db *DB) ListAccounts() ([]Account, error) {
	rows, err := db.conn.Query("SELECT account_id, user_id, token, base_url, account_name, login_at FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Account, 0)
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.AccountID, &a.UserID, &a.Token, &a.BaseURL, &a.AccountName, &a.LoginAt); err != nil {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

func (db *DB) GetAccount(accountID string) (*Account, error) {
	var a Account
	err := db.conn.QueryRow("SELECT account_id, user_id, token, base_url, account_name, login_at FROM accounts WHERE account_id = ?", accountID).
		Scan(&a.AccountID, &a.UserID, &a.Token, &a.BaseURL, &a.AccountName, &a.LoginAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (db *DB) SaveAccount(a Account) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO accounts (account_id, user_id, token, base_url, account_name, login_at) VALUES (?, ?, ?, ?, ?, ?)",
		a.AccountID, a.UserID, a.Token, a.BaseURL, a.AccountName, a.LoginAt,
	)
	return err
}

func (db *DB) DeleteAccount(accountID string) error {
	_, err := db.conn.Exec("DELETE FROM accounts WHERE account_id = ?", accountID)
	return err
}
