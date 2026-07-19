package api

import (
	"github.com/gin-gonic/gin"

	"clawbot-gateway/internal/adapter"
)

// ══════════════════════════════════════════════════════════════════════
//  后端管理
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) handleListBackends(c *gin.Context) {
	backends := s.adapters.List()
	result := make([]gin.H, 0)
	ctx := c.Request.Context()
	for _, b := range backends {
		result = append(result, gin.H{
			"id":      b.ID(),
			"name":    b.Name(),
			"type":    b.Type(),
			"healthy": b.HealthCheck(ctx),
		})
	}
	c.JSON(200, gin.H{"backends": result, "default": s.router.GetDefaultBackend()})
}

func (s *APIServer) handleRegisterBackend(c *gin.Context) {
	var req struct {
		ID     string                 `json:"id"`
		Name   string                 `json:"name"`
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ID == "" {
		c.JSON(400, gin.H{"error": "backend id must not be empty"})
		return
	}
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "backend name must not be empty"})
		return
	}
	if req.Type == "" {
		c.JSON(400, gin.H{"error": "backend type must not be empty"})
		return
	}

	var bak adapter.BackendAdapter
	switch req.Type {
	case "echo":
		bak = adapter.NewEchoAdapter(req.ID, req.Name)
	case "openai_compatible":
		apiKey, _ := req.Config["api_key"].(string)
		baseURL, _ := req.Config["base_url"].(string)
		model, _ := req.Config["model"].(string)
		if model == "" {
			model = "gpt-4o"
		}
		bak = adapter.NewOpenAICompatibleAdapter(req.ID, req.Name, apiKey, baseURL, model)
	case "webhook":
		url, _ := req.Config["url"].(string)
		headersRaw, _ := req.Config["headers"].(map[string]interface{})
		headers := make(map[string]string)
		for k, v := range headersRaw {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
		bak = adapter.NewWebhookAdapter(req.ID, req.Name, url, headers)

	default:
		c.JSON(400, gin.H{"error": "unsupported backend type: " + req.Type})
		return
	}

	s.adapters.Register(bak)
	c.JSON(200, gin.H{"success": true, "backend_id": req.ID})
}

func (s *APIServer) handleRemoveBackend(c *gin.Context) {
	id := c.Param("id")
	s.adapters.Remove(id)
	c.JSON(200, gin.H{"success": true})
}

func (s *APIServer) handleTestBackend(c *gin.Context) {
	id := c.Param("id")
	bak, ok := s.adapters.Get(id)
	if !ok {
		c.JSON(404, gin.H{"error": "backend not found"})
		return
	}

	healthy := bak.HealthCheck(c.Request.Context())
	result := gin.H{"backend_id": id, "healthy": healthy}
	if healthy {
		resp, err := bak.Handle(c.Request.Context(), &adapter.ChatRequest{
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
	if req.BackendID == "" {
		c.JSON(400, gin.H{"error": "backend_id must not be empty"})
		return
	}
	s.router.SetDefaultBackend(req.BackendID)
	c.JSON(200, gin.H{"success": true, "default": req.BackendID})
}
