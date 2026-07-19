package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/session"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/config"
	"clawbot-gateway/internal/ilink"
	"clawbot-gateway/internal/log"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/store"
)

// ══════════════════════════════════════════════════════════════════════
//  API Server — 核心结构
// ══════════════════════════════════════════════════════════════════════

type APIServer struct {
	config       *config.Config
	store        *store.Store
	accountStore *store.AccountStore
	router       *route.Router
	adapters     *adapter.AdapterFactory
	ctxManager   *session.ContextManager
	connector    *bot.Connector
	wsClients    map[string]*websocket.Conn
	wsMu         sync.RWMutex
	restServer   *http.Server
	log          *log.Logger
}

func NewAPIServer(
	cfg *config.Config,
	st *store.Store,
	accountStore *store.AccountStore,
	r *route.Router,
	af *adapter.AdapterFactory,
	cm *session.ContextManager,
	conn *bot.Connector,
) *APIServer {
	return &APIServer{
		config:       cfg,
		store:        st,
		accountStore: accountStore,
		router:       r,
		adapters:     af,
		ctxManager:   cm,
		connector:    conn,
		wsClients:    make(map[string]*websocket.Conn),
	}
}

func (s *APIServer) SetLogger(l *log.Logger) {
	s.log = l.WithComponent("api")
}

// ══════════════════════════════════════════════════════════════════════
//  Start — 路由注册
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) Start() error {
	gin.SetMode(gin.ReleaseMode)

	rest := gin.New()
	rest.Use(gin.Recovery())
	rest.Use(s.corsMiddleware())
	if s.log != nil {
		rest.Use(log.GinMiddleware(s.log))
	}

	// ── 公开端点（无需鉴权） ──
	rest.GET("/health", s.handleHealth)
	rest.GET("/api/v1/health", s.handleHealth)
	rest.POST("/auth/login", s.handleLogin)

	// ── iLink API（外部服务接入，无需鉴权） ──
	ilinkServer := ilink.NewServer(s.connector)
	ilinkServer.RegisterRoutes(rest)

	// ── 需要鉴权的 API ──
	api := rest.Group("/api/v1", s.authMiddleware())

	// API Token 管理（鉴权后）
	auth := api.Group("/auth")
	auth.GET("/token", s.handleGetAPIToken)
	auth.POST("/token", s.handleRegenAPIToken)

	// 微信账号
	wc := api.Group("/wechat")
	wc.POST("/qrcode", s.handleGetQRCode)
	wc.POST("/qrcode/status", s.handleQRStatus)
	wc.GET("/accounts", s.handleListAccounts)
	wc.POST("/accounts/:id/disconnect", s.handleDisconnectAccount)

	// 后端管理
	be := api.Group("/backends")
	be.GET("", s.handleListBackends)
	be.POST("", s.handleRegisterBackend)
	be.DELETE("/:id", s.handleRemoveBackend)
	be.POST("/:id/test", s.handleTestBackend)
	be.PUT("/default", s.handleSetDefaultBackend)

	// 路由规则
	rt := api.Group("/routes")
	rt.GET("", s.handleListRoutes)
	rt.POST("", s.handleAddRoute)
	rt.DELETE("/:index", s.handleRemoveRoute)

	// 用户会话
	us := api.Group("/users")
	us.GET("", s.handleListUsers)
	us.GET("/:id/context", s.handleGetUserContext)
	us.POST("/:id/switch", s.handleSwitchUserBackend)
	us.DELETE("/:id/context", s.handleClearUserContext)
	us.GET("/:id/route-mode", s.handleGetUserRouteMode)
	us.POST("/:id/route-mode", s.handleSetUserRouteMode)

	// 消息发送
	msg := api.Group("/message")
	msg.POST("/send", s.handlePushSend)
	msg.POST("/broadcast", s.handlePushBroadcast)
	msg.POST("/send-by-account", s.handleSendByAccount)
	msg.GET("/accounts", s.handleMessageAccounts)

	// 通用聊天 API
	api.POST("/chat", s.handleExternalChat)
	api.GET("/stats", s.handleStats)

	// 静态页面 — 优先从 Vite build (web/dist) 加载 SPA，否则回退到旧版 web/
	if _, err := os.Stat("./web/dist/index.html"); os.IsNotExist(err) {
		rest.Static("/web", "./web")
		rest.GET("/", func(c *gin.Context) {
			c.File("./web/index.html")
		})
	} else {
		dist := "./web/dist"
		// Vite build 产物
		rest.Static("/assets", dist+"/assets")
		rest.StaticFile("/sw.js", dist+"/sw.js")
		rest.StaticFile("/manifest.webmanifest", dist+"/manifest.webmanifest")
		rest.StaticFile("/registerSW.js", dist+"/registerSW.js")
		rest.GET("/", func(c *gin.Context) {
			c.File(dist + "/index.html")
		})
		// SPA fallback: 所有未匹配的非 API 路径返回 index.html 以支持客户端路由
		rest.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/auth") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			c.File(dist + "/index.html")
		})
	}

	rest.GET("/ws", s.authMiddleware(), func(c *gin.Context) {
		s.handleWSConnection(c.Writer, c.Request)
	})

	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.log.Info("REST API listening", "addr", addr)
	s.restServer = &http.Server{Addr: addr, Handler: rest.Handler()}
	go func() {
		if err := s.restServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("REST server failed", "error", err)
			os.Exit(1)
		}
	}()
	return nil
}

// ══════════════════════════════════════════════════════════════════════
//  中间件
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) corsMiddleware() gin.HandlerFunc {
	hasWildcard := false
	for _, o := range s.config.API.AllowedOrigins {
		if o == "*" {
			hasWildcard = true
			break
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		for _, allowed := range s.config.API.AllowedOrigins {
			if allowed == "*" || allowed == origin {
				c.Header("Access-Control-Allow-Origin", origin)
				break
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-ClawBot-Secret")
		// Only set credentials header when NOT using wildcard (CORS spec violation otherwise)
		if !hasWildcard {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func (s *APIServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)

		// 1. 尝试验证 JWT
		if s.validateJWT(token) {
			c.Next()
			return
		}

		// 2. Fallback: 验证 API Token
		if s.validateAPIToken(token) {
			c.Next()
			return
		}

		// 3. 无有效凭证
		c.JSON(401, gin.H{"error": "unauthorized", "message": "invalid or missing auth token"})
		c.Abort()
	}
}

// extractBearerToken 从 Authorization header 提取 token
func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return auth
}

// validateJWT 验证 JWT token 有效性
func (s *APIServer) validateJWT(token string) bool {
	if token == "" || s.config.API.JWTSecret == "" {
		return false
	}
	_, err := VerifyJWT(s.config.API.JWTSecret, token)
	return err == nil
}

// validateAPIToken 验证 API Token
func (s *APIServer) validateAPIToken(token string) bool {
	if token == "" {
		return false
	}
	expected := s.store.GetAPIToken()
	return expected != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// ══════════════════════════════════════════════════════════════════════
//  WebSocket
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) wsCheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // no origin = not a browser request
	}
	for _, allowed := range s.config.API.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func (s *APIServer) handleWSConnection(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: s.wsCheckOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("ws upgrade error", "error", err)
		return
	}
	defer conn.Close()

	clientID := fmt.Sprintf("ws_%d", atomic.AddInt64(&wsClientCounter, 1))
	s.wsMu.Lock()
	s.wsClients[clientID] = conn
	s.wsMu.Unlock()
	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, clientID)
		s.wsMu.Unlock()
	}()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var req struct {
			Type      string `json:"type"`
			BackendID string `json:"backend_id,omitempty"`
			Message   string `json:"message"`
			UserID    string `json:"user_id"`
			Stream    bool   `json:"stream,omitempty"`
		}
		if err := json.Unmarshal(msgData, &req); err != nil {
			if err := conn.WriteJSON(gin.H{"type": "error", "message": "invalid request"}); err != nil {
				break
			}
			continue
		}

		if req.Type == "chat" {
			bak, ok := s.adapters.Get(req.BackendID)
			if !ok {
				if err := conn.WriteJSON(gin.H{"type": "error", "message": "backend not found"}); err != nil {
					break
				}
				continue
			}
			ctx := s.ctxManager.GetContext(req.UserID, req.BackendID)
			chatReq := &adapter.ChatRequest{
				Message:   req.Message,
				UserID:    req.UserID,
				BackendID: req.BackendID,
				History:   ctx.History,
				Stream:    req.Stream,
			}
			if req.Stream {
				ch := make(chan string, 10)
				errCh := make(chan error, 1)
				go func() { errCh <- bak.HandleStream(r.Context(), chatReq, ch) }()
				fullReply := ""
				for chunk := range ch {
					fullReply += chunk
					if err := conn.WriteJSON(gin.H{"type": "chunk", "content": chunk}); err != nil {
						break
					}
				}
				if streamErr := <-errCh; streamErr != nil {
					conn.WriteJSON(gin.H{"type": "error", "message": streamErr.Error()})
				}
				ctx.AddTurn(req.Message, fullReply)
				if err := conn.WriteJSON(gin.H{"type": "done"}); err != nil {
					break
				}
			} else {
				resp, err := bak.Handle(r.Context(), chatReq)
				if err != nil {
					if err := conn.WriteJSON(gin.H{"type": "error", "message": err.Error()}); err != nil {
						break
					}
					continue
				}
				ctx.AddTurn(req.Message, resp.Text)
				if err := conn.WriteJSON(gin.H{"type": "reply", "content": resp.Text, "backend": resp.Backend}); err != nil {
					break
				}
			}
		}
	}
}

// ══════════════════════════════════════════════════════════════════════
//  Shutdown — 优雅关闭
// ══════════════════════════════════════════════════════════════════════

func (s *APIServer) Shutdown(ctx context.Context) {
	if s.restServer != nil {
	if err := s.restServer.Shutdown(ctx); err != nil {
		s.log.Warn("server shutdown error", "error", err)
	}
	}
}

// ══════════════════════════════════════════════════════════════════════
//  健康检查 / 统计 / 外部聊天
// ══════════════════════════════════════════════════════════════════════

var startTime = time.Now()
var wsClientCounter int64

func (s *APIServer) handleHealth(c *gin.Context) {
	backends := s.adapters.List()
	healthyCount := 0
	for _, b := range backends {
		if b.HealthCheck(c.Request.Context()) {
			healthyCount++
		}
	}
	c.JSON(200, gin.H{
		"status":   "ok",
		"version":  "2.0.0",
		"backends": len(backends),
		"healthy":  healthyCount,
		"accounts": s.accountStore.Count(),
		"sessions": s.ctxManager.SessionCount(),
		"uptime":   time.Since(startTime).String(),
	})
}

func (s *APIServer) handleStats(c *gin.Context) {
	backends := s.adapters.List()
	backendStats := make([]gin.H, 0)
	for _, b := range backends {
		backendStats = append(backendStats, gin.H{
			"id":      b.ID(),
			"name":    b.Name(),
			"type":    b.Type(),
			"healthy": b.HealthCheck(c.Request.Context()),
		})
	}
	c.JSON(200, gin.H{
		"backends":        backendStats,
		"sessions":        s.ctxManager.SessionCount(),
		"routes":          len(s.router.GetKeywordRules()),
		"accounts":        s.connector.GetAccountCount(),
		"default_backend": s.router.GetDefaultBackend(),
	})
}

// handleExternalChat 外部系统通过指定后端对话
// POST /api/v1/chat
func (s *APIServer) handleExternalChat(c *gin.Context) {
	var req struct {
		BackendID string `json:"backend_id"`
		Message   string `json:"message"`
		UserID    string `json:"user_id"`
		Stream    bool   `json:"stream"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == "" {
		req.UserID = fmt.Sprintf("ext_%d", time.Now().UnixNano())
	}

	backendID := req.BackendID
	if backendID == "" {
		decision := s.router.Route(req.Message, req.UserID)
		backendID = decision.BackendID
	}

	bak, ok := s.adapters.Get(backendID)
	if !ok {
		c.JSON(400, gin.H{"error": "backend not found: " + backendID})
		return
	}

	ctx := s.ctxManager.GetContext(req.UserID, backendID)

	// 添加请求超时
	reqCtx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	if req.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		ch := make(chan string, 10)
		go bak.HandleStream(reqCtx, &adapter.ChatRequest{
			Message:   req.Message,
			UserID:    req.UserID,
			BackendID: backendID,
			History:   ctx.History,
			Stream:    true,
		}, ch)
		fullReply := ""
		for chunk := range ch {
			fullReply += chunk
			fmt.Fprintf(c.Writer, "data: %s\n\n", chunk)
			c.Writer.Flush()
		}
		ctx.AddTurn(req.Message, fullReply)
		return
	}

	resp, err := bak.Handle(reqCtx, &adapter.ChatRequest{
		Message:   req.Message,
		UserID:    req.UserID,
		BackendID: backendID,
		History:   ctx.History,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	ctx.AddTurn(req.Message, resp.Text)
	c.JSON(200, gin.H{"reply": resp.Text, "backend": resp.Backend, "user_id": req.UserID})
}
