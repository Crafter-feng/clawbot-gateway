package api

import (
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/database"
)

func (s *APIServer) handleListBackends(c *gin.Context) {
	backends, err := s.db.ListBackends()
	if err != nil {
		log.Printf("ERROR: failed to list backends: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	result := make([]gin.H, 0)
	ctx := c.Request.Context()
	for _, b := range backends {
		adapterInstance := s.getAdapterFromDB(b)
		healthy := false
		if adapterInstance != nil {
			healthy = adapterInstance.HealthCheck(ctx)
		}
		result = append(result, gin.H{
			"id":      b.ID,
			"name":    b.Name,
			"type":    b.Type,
			"healthy": healthy,
			"enabled": b.Enabled,
		})
	}

	defaultBackend := s.db.GetSetting("route.default_backend")
	c.JSON(200, gin.H{"backends": result, "default": defaultBackend})
}

func (s *APIServer) handleGetBackend(c *gin.Context) {
	id := c.Param("id")
	b, err := s.db.GetBackend(id)
	if err != nil || b == nil {
		c.JSON(404, gin.H{"error": "backend not found"})
		return
	}

	c.JSON(200, gin.H{
		"id":      b.ID,
		"name":    b.Name,
		"type":    b.Type,
		"config":  sanitizeBackendConfig(b.Config),
		"enabled": b.Enabled,
	})
}

func (s *APIServer) handleRegisterBackend(c *gin.Context) {
	var req struct {
		ID      string                 `json:"id"`
		Name    string                 `json:"name"`
		Type    string                 `json:"type"`
		Config  map[string]interface{} `json:"config"`
		Enabled *bool                  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ID == "" || req.Name == "" || req.Type == "" {
		c.JSON(400, gin.H{"error": "id, name, type are required"})
		return
	}

	configJSON, _ := json.Marshal(req.Config)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	b := database.Backend{
		ID:      req.ID,
		Name:    req.Name,
		Type:    req.Type,
		Config:  string(configJSON),
		Enabled: enabled,
	}
	if err := s.db.CreateBackend(b); err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	// ilink_proxy 类型还需要创建虚拟 Bot
	if req.Type == "ilink_proxy" {
		accountID := "gw_" + req.ID
		userID := accountID + "@im.wechat"
		// 虚拟 Bot 的 BaseURL 应指向真实 iLink API，而非 Gateway 自身
		// 这样 sendmessage 等请求才能正确转发到腾讯服务器
		baseURL := "https://ilinkai.weixin.qq.com"

		// 先注册到 ClientRegistry（生成随机 token），再保存到数据库
		// 这样重启后 token 保持不变
		vbot := s.clientReg.Register(accountID, userID, baseURL, "")
		vb := database.VirtualBot{
			ID:        req.ID,
			AccountID: accountID,
			UserID:    userID,
			BaseURL:   baseURL,
			Token:     vbot.Token,
		}
		s.db.SaveVirtualBot(vb)
	}

	s.reloadAdapters()
	c.JSON(200, gin.H{"success": true, "backend_id": req.ID})
}

func (s *APIServer) handleUpdateBackend(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name    string                 `json:"name"`
		Type    string                 `json:"type"`
		Config  map[string]interface{} `json:"config"`
		Enabled *bool                  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	existing, err := s.db.GetBackend(id)
	if err != nil || existing == nil {
		c.JSON(404, gin.H{"error": "backend not found"})
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Type != "" {
		existing.Type = req.Type
	}
	if req.Config != nil {
		configJSON, _ := json.Marshal(req.Config)
		existing.Config = string(configJSON)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := s.db.UpdateBackend(id, *existing); err != nil {
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	s.reloadAdapters()
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleRemoveBackend(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.DeleteVirtualBot(id); err != nil {
		log.Printf("ERROR: failed to delete virtual bot %s: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	s.clientReg.Unregister("gw_" + id)
	if err := s.db.DeleteBackend(id); err != nil {
		log.Printf("ERROR: failed to delete backend %s: %v", id, err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	s.reloadAdapters()
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleTestBackend(c *gin.Context) {
	id := c.Param("id")
	b, err := s.db.GetBackend(id)
	if err != nil || b == nil {
		c.JSON(404, gin.H{"error": "backend not found"})
		return
	}

	adapterInstance := s.getAdapterFromDB(*b)
	if adapterInstance == nil {
		c.JSON(400, gin.H{"error": "cannot create adapter"})
		return
	}

	healthy := adapterInstance.HealthCheck(c.Request.Context())
	result := gin.H{"backend_id": id, "healthy": healthy}

	// ilink_proxy 是连接适配器，不支持消息处理，只检查健康状态
	if b.Type == "ilink_proxy" {
		c.JSON(200, result)
		return
	}

	if healthy {
		resp, err := adapterInstance.Handle(c.Request.Context(), &adapter.ChatRequest{
			Message: "ping",
			UserID:  "test",
		})
		if err != nil {
			result["error"] = err.Error()
		} else {
			result["reply"] = resp.Text
		}
	}
	c.JSON(200, result)
}

func (s *APIServer) handleSetDefaultBackend(c *gin.Context) {
	var req struct {
		BackendID string `json:"backend_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	s.db.SetSetting("route.default_backend", req.BackendID)
	s.router.SetDefaultBackend(req.BackendID)
	c.JSON(200, gin.H{"success": true, "default": req.BackendID})
}

func (s *APIServer) getAdapterFromDB(b database.Backend) adapter.BackendAdapter {
	return adapter.CreateAdapterFromDB(b)
}

func (s *APIServer) reloadAdapters() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.adapters = adapter.NewAdapterFactory()

	backends, _ := s.db.ListBackends()
	for _, b := range backends {
		if !b.Enabled {
			continue
		}
		adapterInstance := adapter.CreateAdapterFromDB(b)
		if adapterInstance != nil {
			s.adapters.Register(adapterInstance)
		}
	}

	vbots, _ := s.db.ListVirtualBots()
	for _, vb := range vbots {
		s.adapters.RegisterConnection(adapter.NewILinkProxyAdapter(vb.ID, vb.ID, vb.AccountID, vb.UserID, vb.BaseURL))
	}

	s.adapters.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
}
func sanitizeBackendConfig(config string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(config), &m); err != nil {
		return config
	}
	// 移除敏感字段
	delete(m, "api_key")
	sanitized, _ := json.Marshal(m)
	return string(sanitized)
}
