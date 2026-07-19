package database

type Route struct {
	ID         int    `json:"id"`
	Keyword    string `json:"keyword"`
	BackendID  string `json:"backend_id"`
	IsRegexp   bool   `json:"is_regexp"`
	Priority   int    `json:"priority"`
}

func (db *DB) ListRoutes() ([]Route, error) {
	rows, err := db.conn.Query("SELECT id, keyword, backend_id, is_regexp, priority FROM routes ORDER BY priority DESC, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Route, 0)
	for rows.Next() {
		var r Route
		var isRegexp int
		if err := rows.Scan(&r.ID, &r.Keyword, &r.BackendID, &isRegexp, &r.Priority); err != nil {
			continue
		}
		r.IsRegexp = isRegexp == 1
		result = append(result, r)
	}
	return result, nil
}

func (db *DB) CreateRoute(r Route) error {
	isRegexp := 0
	if r.IsRegexp {
		isRegexp = 1
	}
	_, err := db.conn.Exec(
		"INSERT INTO routes (keyword, backend_id, is_regexp, priority) VALUES (?, ?, ?, ?)",
		r.Keyword, r.BackendID, isRegexp, r.Priority,
	)
	return err
}

func (db *DB) UpdateRoute(id int, r Route) error {
	isRegexp := 0
	if r.IsRegexp {
		isRegexp = 1
	}
	_, err := db.conn.Exec(
		"UPDATE routes SET keyword=?, backend_id=?, is_regexp=?, priority=? WHERE id=?",
		r.Keyword, r.BackendID, isRegexp, r.Priority, id,
	)
	return err
}

func (db *DB) DeleteRoute(id int) error {
	_, err := db.conn.Exec("DELETE FROM routes WHERE id = ?", id)
	return err
}
