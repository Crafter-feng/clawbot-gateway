package database

import (
	"encoding/json"
	"time"
)

// RouteCondition 单个匹配条件
type RouteCondition struct {
	ID            string `json:"id"`              // 条件 ID
	Field         string `json:"field"`           // 匹配字段
	Operator      string `json:"operator"`        // 匹配操作符
	Value         string `json:"value"`           // 匹配值
	CaseSensitive bool   `json:"case_sensitive"`  // 是否区分大小写
	Negate        bool   `json:"negate"`          // 是否取反（非逻辑）
}

// 匹配字段常量
const (
	ConditionFieldMessage  = "message"   // 消息内容
	ConditionFieldFromUser = "from_user" // 发送者
	ConditionFieldToUser   = "to_user"   // 接收者
	ConditionFieldMsgType  = "msg_type"  // 消息类型
)

// 匹配操作符常量
const (
	ConditionOpExact      = "exact"       // 精确匹配
	ConditionOpContains   = "contains"    // 包含
	ConditionOpStartsWith = "starts_with" // 前缀
	ConditionOpEndsWith   = "ends_with"   // 后缀
	ConditionOpRegex      = "regex"       // 正则表达式
)

// RouteRuleGroup 规则组（支持 AND/OR 逻辑）
type RouteRuleGroup struct {
	ID         string           `json:"id"`          // 组 ID
	Logic      string           `json:"logic"`       // 组内逻辑：and/or
	Conditions []RouteCondition `json:"conditions"`  // 条件列表
}

// RouteRule 路由规则
type RouteRule struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`         // 规则名称
	BackendID   string           `json:"backend_id"`   // 目标后端
	Priority    int              `json:"priority"`     // 优先级（越小越优先）
	Enabled     bool             `json:"enabled"`      // 是否启用
	Description string           `json:"description"`  // 规则描述
	Groups      []RouteRuleGroup `json:"groups"`       // 规则组列表
	GroupLogic  string           `json:"group_logic"`  // 组间逻辑：and/or
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// RouteRuleResponse API 响应格式
type RouteRuleResponse struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	BackendID   string           `json:"backend_id"`
	Priority    int              `json:"priority"`
	Enabled     bool             `json:"enabled"`
	Description string           `json:"description"`
	Groups      []RouteRuleGroup `json:"groups"`
	GroupLogic  string           `json:"group_logic"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
}

// ListRouteRules 列出所有路由规则
func (db *DB) ListRouteRules() ([]RouteRule, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, backend_id, priority, enabled, description, groups, group_logic, created_at, updated_at
		FROM route_rules
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []RouteRule
	for rows.Next() {
		var rule RouteRule
		var groupsJSON string
		var createdAt, updatedAt string

		err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.BackendID,
			&rule.Priority,
			&rule.Enabled,
			&rule.Description,
			&groupsJSON,
			&rule.GroupLogic,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		// 解析 JSON
		if err := json.Unmarshal([]byte(groupsJSON), &rule.Groups); err != nil {
			rule.Groups = []RouteRuleGroup{}
		}

		rule.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		rule.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		rules = append(rules, rule)
	}

	return rules, nil
}

// GetRouteRule 获取单个路由规则
func (db *DB) GetRouteRule(id int) (*RouteRule, error) {
	var rule RouteRule
	var groupsJSON string
	var createdAt, updatedAt string

	err := db.conn.QueryRow(`
		SELECT id, name, backend_id, priority, enabled, description, groups, group_logic, created_at, updated_at
		FROM route_rules
		WHERE id = ?
	`, id).Scan(
		&rule.ID,
		&rule.Name,
		&rule.BackendID,
		&rule.Priority,
		&rule.Enabled,
		&rule.Description,
		&groupsJSON,
		&rule.GroupLogic,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(groupsJSON), &rule.Groups); err != nil {
		rule.Groups = []RouteRuleGroup{}
	}

	rule.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	rule.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &rule, nil
}

// CreateRouteRule 创建路由规则
func (db *DB) CreateRouteRule(rule RouteRule) (int64, error) {
	groupsJSON, err := json.Marshal(rule.Groups)
	if err != nil {
		return 0, err
	}

	result, err := db.conn.Exec(`
		INSERT INTO route_rules (name, backend_id, priority, enabled, description, groups, group_logic)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		rule.Name,
		rule.BackendID,
		rule.Priority,
		rule.Enabled,
		rule.Description,
		string(groupsJSON),
		rule.GroupLogic,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateRouteRule 更新路由规则
func (db *DB) UpdateRouteRule(id int, rule RouteRule) error {
	groupsJSON, err := json.Marshal(rule.Groups)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		UPDATE route_rules
		SET name = ?, backend_id = ?, priority = ?, enabled = ?, description = ?, groups = ?, group_logic = ?, updated_at = datetime('now')
		WHERE id = ?
	`,
		rule.Name,
		rule.BackendID,
		rule.Priority,
		rule.Enabled,
		rule.Description,
		string(groupsJSON),
		rule.GroupLogic,
		id,
	)
	return err
}

// DeleteRouteRule 删除路由规则
func (db *DB) DeleteRouteRule(id int) error {
	_, err := db.conn.Exec("DELETE FROM route_rules WHERE id = ?", id)
	return err
}

// ToggleRouteRule 切换路由规则启用状态
func (db *DB) ToggleRouteRule(id int) error {
	_, err := db.conn.Exec(`
		UPDATE route_rules
		SET enabled = CASE WHEN enabled = 1 THEN 0 ELSE 1 END, updated_at = datetime('now')
		WHERE id = ?
	`, id)
	return err
}

// ReorderRouteRules 重新排序路由规则
func (db *DB) ReorderRouteRules(ids []int) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		_, err := tx.Exec("UPDATE route_rules SET priority = ? WHERE id = ?", i+1, id)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
