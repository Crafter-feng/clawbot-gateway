package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/database"
)

func (s *APIServer) handleListBackends(c *gin.Context) {
	backends, err := s.db.ListBackends()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
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
		"config":  b.Config,
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// ilink_proxy 类型还需要创建虚拟 Bot
	if req.Type == "ilink_proxy" {
		accountID := "gw_" + req.ID
		userID := accountID + "@im.wechat"
		baseURL := "http://localhost:8080"
		if s.config != nil && s.config.Server.Port != 8080 {
			baseURL = "http://localhost:" + itoa(s.config.Server.Port)
		}

		vb := database.VirtualBot{
			ID:        req.ID,
			AccountID: accountID,
			UserID:    userID,
			BaseURL:   baseURL,
		}
		s.db.SaveVirtualBot(vb)

		// 注册到 ClientRegistry
		s.clientReg.Register(accountID, userID)
	}

	// 重新加载适配器
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	s.reloadAdapters()
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleRemoveBackend(c *gin.Context) {
	id := c.Param("id")

	// 删除虚拟 Bot（如果是 ilink_proxy）
	s.db.DeleteVirtualBot(id)
	s.clientReg.Unregister("gw_" + id)

	s.db.DeleteBackend(id)
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
	switch b.Type {
	case "echo":
		return adapter.NewEchoAdapter(b.ID, b.Name)
	case "openai_compatible":
		apiKey := getJSONString(b.Config, "api_key")
		baseURL := getJSONString(b.Config, "base_url")
		model := getJSONString(b.Config, "model")
		if model == "" {
			model = "gpt-4o"
		}
		return adapter.NewOpenAICompatibleAdapter(b.ID, b.Name, apiKey, baseURL, model)
	case "webhook":
		url := getJSONString(b.Config, "url")
		return adapter.NewWebhookAdapter(b.ID, b.Name, url, nil)
	default:
		return nil
	}
}

func (s *APIServer) reloadAdapters() {
	// 清空并重新加载
	s.adapters = adapter.NewAdapterFactory()

	backends, _ := s.db.ListBackends()
	for _, b := range backends {
		if !b.Enabled {
			continue
		}
		adapterInstance := s.getAdapterFromDB(b)
		if adapterInstance != nil {
			s.adapters.Register(adapterInstance)
		}
	}

	// 重新注册连接适配器
	vbots, _ := s.db.ListVirtualBots()
	for _, vb := range vbots {
		s.adapters.RegisterConnection(adapter.NewILinkProxyAdapter(vb.ID, vb.ID, vb.BaseURL))
	}

	// 默认 echo
	s.adapters.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
}

func getJSONString(jsonStr, key string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
