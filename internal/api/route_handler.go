package api

import (
	"log"
	"strconv"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/database"
	"clawbot-gateway/internal/route"
)

func (s *APIServer) handleListRouteRules(c *gin.Context) {
	rules, err := s.db.ListRouteRules()
	if err != nil {
		log.Printf("ERROR: failed to list route rules: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(200, gin.H{"rules": rules})
}

// handleGetRouteRule 获取单个路由规则
func (s *APIServer) handleGetRouteRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid rule id"})
		return
	}

	rule, err := s.db.GetRouteRule(id)
	if err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	if rule == nil {
		c.JSON(404, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(200, rule)
}

// handleCreateRouteRule 创建路由规则
func (s *APIServer) handleCreateRouteRule(c *gin.Context) {
	var rule database.RouteRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		log.Printf("bad request: %v", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证必填字段
	if rule.Name == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	if rule.BackendID == "" {
		c.JSON(400, gin.H{"error": "backend_id is required"})
		return
	}

	// 验证正则表达式
	for _, group := range rule.Groups {
		for _, cond := range group.Conditions {
			if cond.Operator == "regex" {
				if err := route.ValidateRegexp(cond.Value); err != nil {
					c.JSON(400, gin.H{"error": "invalid regex: " + cond.Value})
					return
				}
			}
		}
	}

	id, err := s.db.CreateRouteRule(rule)
	if err != nil {
		log.Printf("ERROR: failed to create route rule: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	// 同步到内存路由引擎
	rule.ID = int(id)
	s.router.AddRule(convertToRouteRule(rule))

	c.JSON(200, gin.H{"success": true, "id": id})
}

// handleUpdateRouteRule 更新路由规则
func (s *APIServer) handleUpdateRouteRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid rule id"})
		return
	}

	var rule database.RouteRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		log.Printf("bad request: %v", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证正则表达式
	for _, group := range rule.Groups {
		for _, cond := range group.Conditions {
			if cond.Operator == "regex" {
				if err := route.ValidateRegexp(cond.Value); err != nil {
					c.JSON(400, gin.H{"error": "invalid regex: " + cond.Value})
					return
				}
			}
		}
	}
	if err := s.db.UpdateRouteRule(id, rule); err != nil {
		log.Printf("ERROR: failed to update route rule %d: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	// 同步到内存路由引擎
	rule.ID = id
	s.router.UpdateRule(convertToRouteRule(rule))

	c.JSON(200, gin.H{"success": true})
}

// handleDeleteRouteRule 删除路由规则
func (s *APIServer) handleDeleteRouteRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid rule id"})
		return
	}
	if err := s.db.DeleteRouteRule(id); err != nil {
		log.Printf("ERROR: failed to delete route rule %d: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	// 同步到内存路由引擎
	s.router.RemoveRule(id)

	c.JSON(200, gin.H{"success": true})
}

// handleToggleRouteRule 切换路由规则启用状态
func (s *APIServer) handleToggleRouteRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid rule id"})
		return
	}

	if err := s.db.ToggleRouteRule(id); err != nil {
		log.Printf("ERROR: failed to toggle route rule %d: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	// 重新加载规则到内存
	s.reloadRouteRules()

	c.JSON(200, gin.H{"success": true})
}

// handleReorderRouteRules 重新排序路由规则
func (s *APIServer) handleReorderRouteRules(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("bad request: %v", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	if err := s.db.ReorderRouteRules(req.IDs); err != nil {
		log.Printf("ERROR: failed to reorder route rules: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	// 重新加载规则到内存
	s.reloadRouteRules()

	c.JSON(200, gin.H{"success": true})
}

// handleTestRouteRule 测试路由匹配
func (s *APIServer) handleTestRouteRule(c *gin.Context) {
	var req struct {
		Message  string `json:"message"`
		UserID   string `json:"user_id"`
		FromUser string `json:"from_user"`
		ToUser   string `json:"to_user"`
		MsgType  string `json:"msg_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("bad request: %v", err)
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}

	decision := s.router.Route(req.Message, req.UserID, req.FromUser, req.ToUser, req.MsgType)

	c.JSON(200, gin.H{
		"matched":    decision.MatchedBy != "none",
		"backend_id": decision.BackendID,
		"matched_by": decision.MatchedBy,
		"rule_id":    decision.RuleID,
	})
}

// reloadRouteRules 重新加载路由规则到内存
func (s *APIServer) reloadRouteRules() {
	rules, err := s.db.ListRouteRules()
	if err != nil {
		return
	}

	// 清空并重新加载
	routeRules := make([]route.RouteRule, 0, len(rules))
	for _, rule := range rules {
		routeRules = append(routeRules, convertToRouteRule(rule))
	}
	s.router.LoadRules(routeRules)
}

// convertToRouteRule 将数据库路由规则转换为路由引擎格式
func convertToRouteRule(dbRule database.RouteRule) route.RouteRule {
	groups := make([]route.RouteRuleGroup, len(dbRule.Groups))
	for i, dbGroup := range dbRule.Groups {
		conditions := make([]route.RouteCondition, len(dbGroup.Conditions))
		for j, dbCond := range dbGroup.Conditions {
			conditions[j] = route.RouteCondition{
				ID:            dbCond.ID,
				Field:         dbCond.Field,
				Operator:      dbCond.Operator,
				Value:         dbCond.Value,
				CaseSensitive: dbCond.CaseSensitive,
				Negate:        dbCond.Negate,
			}
		}
		groups[i] = route.RouteRuleGroup{
			ID:         dbGroup.ID,
			Logic:      dbGroup.Logic,
			Conditions: conditions,
		}
	}

	return route.RouteRule{
		ID:         dbRule.ID,
		Name:       dbRule.Name,
		BackendID:  dbRule.BackendID,
		Priority:   dbRule.Priority,
		Enabled:    dbRule.Enabled,
		Groups:     groups,
		GroupLogic: dbRule.GroupLogic,
	}
}
