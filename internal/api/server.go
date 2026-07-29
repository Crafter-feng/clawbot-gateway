package api

import (
	"encoding/json"
	"context"
	"crypto/subtle"
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
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/config"
	"clawbot-gateway/internal/database"
	"clawbot-gateway/internal/ilink"
	"clawbot-gateway/internal/notify"
	"clawbot-gateway/internal/log"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/session"
	"clawbot-gateway/internal/version"
)

// revokedTokens 存储已撤销的 JWT token 的 jti（JWT ID）
var revokedTokens sync.Map

type APIServer struct {
	config       *config.Config
	db           *database.DB
	router       *route.Router
	adapters     *adapter.AdapterFactory
	ctxManager   *session.ContextManager
	connector    *bot.Connector
	clientReg    *ilink.ClientRegistry

	wsClients     map[string]*websocket.Conn
	wsMu          sync.RWMutex
	restServer    *http.Server
	log           *log.Logger
	mu            sync.RWMutex
	loginAttempts sync.Map
}

func NewAPIServer(
	cfg *config.Config,
	db *database.DB,
	r *route.Router,
	af *adapter.AdapterFactory,
	cm *session.ContextManager,
	conn *bot.Connector,
	clientReg *ilink.ClientRegistry,
) *APIServer {
	return &APIServer{
		config:        cfg,
		db:            db,
		router:        r,
		adapters:      af,
		ctxManager:    cm,
		connector:     conn,
		clientReg:     clientReg,

		wsClients:     make(map[string]*websocket.Conn),
	}
}

func (s *APIServer) SetLogger(l *log.Logger) {
	s.log = l.WithComponent("api")
}

func (s *APIServer) Start(addr string) error {
	gin.SetMode(gin.ReleaseMode)

	rest := gin.New()
	rest.Use(gin.Recovery())
	rest.Use(s.corsMiddleware())
	if s.log != nil {
		rest.Use(log.GinMiddleware(s.log))
	}
	rest.Use(maxBodySizeMiddleware(1 << 20)) // 1MB request body limit
	rest.Use(securityHeadersMiddleware())

	// 公开端点
	rest.GET("/health", s.handleHealth)
	rest.POST("/auth/login", s.handleLogin)

	// iLink API
	ilinkServer := ilink.NewServer(s.connector, s.clientReg)
	ilinkServer.RegisterRoutes(rest)

	// 通知 API
	notifyHandler := notify.NewHandler(s.db, s.sendNotifyMessage)
	notifyHandler.RegisterRoutes(rest)

	// 需要鉴权的 API
	api := rest.Group("/api/v1", s.authMiddleware())

	// 通知管理（需要认证）
	notify := api.Group("/notify")
	notifyHandler.RegisterManagementRoutes(notify)

	// API Token 管理
	auth := api.Group("/auth")
	auth.GET("/token", s.handleGetAPIToken)
	auth.POST("/token", s.handleRegenAPIToken)
	auth.PUT("/password", s.handleChangePassword)

	// 配置管理
	cfg := api.Group("/config")
	cfg.GET("", s.handleGetAllConfig)
	cfg.PUT("", s.handleUpdateConfig)
	cfg.GET("/:key", s.handleGetConfig)
	cfg.PUT("/:key", s.handleSetConfig)

	// 后端管理
	be := api.Group("/backends")
	be.GET("", s.handleListBackends)
	be.GET("/:id", s.handleGetBackend)
	be.POST("", s.handleRegisterBackend)
	be.PUT("/:id", s.handleUpdateBackend)
	be.DELETE("/:id", s.handleRemoveBackend)
	be.POST("/:id/test", s.handleTestBackend)
	be.PUT("/default", s.handleSetDefaultBackend)

	// 路由规则
	rt := api.Group("/routes")
	rt.GET("", s.handleListRouteRules)
	rt.GET("/:id", s.handleGetRouteRule)
	rt.POST("", s.handleCreateRouteRule)
	rt.PUT("/:id", s.handleUpdateRouteRule)
	rt.DELETE("/:id", s.handleDeleteRouteRule)
	rt.PUT("/:id/toggle", s.handleToggleRouteRule)
	rt.PUT("/reorder", s.handleReorderRouteRules)
	rt.POST("/test", s.handleTestRouteRule)

	// 微信账号
	wc := api.Group("/accounts")
	wc.GET("", s.handleListWechatAccounts)
	wc.POST("", s.handleSaveWechatAccount)
	wc.DELETE("/:id", s.handleDisconnectAccount)

	// 微信扫码
	wx := api.Group("/wechat")
	wx.POST("/qrcode", s.handleGetQRCode)
	wx.POST("/qrcode/status", s.handleQRStatus)

	// 连接适配器
	conn := api.Group("/connections")
	conn.GET("", s.handleListConnections)
	conn.GET("/stats", s.handleConnectionStats)  // 静态路径必须在参数化路径之前
	conn.GET("/:id", s.handleGetConnection)
	// 日志
	logs := api.Group("/logs")
	logs.GET("", s.handleGetLogs)
	logs.GET("/categories", s.handleGetLogCategories)

	// 消息 API
	msg := api.Group("/message")
	msg.POST("/send", s.handlePushSend)

	// 用户管理
	users := api.Group("/users")
	users.GET("", s.handleListUsers)
	users.GET("/:id/context", s.handleGetUserContext)
	users.PUT("/:id/backend", s.handleSwitchUserBackend)
	users.DELETE("/:id/context", s.handleClearUserContext)
	users.GET("/:id/routemode", s.handleGetUserRouteMode)
	users.PUT("/:id/routemode", s.handleSetUserRouteMode)

	// 统计
	api.GET("/stats", s.handleStats)

	// 静态文件
	if _, err := os.Stat("./web/dist/index.html"); os.IsNotExist(err) {
		rest.Static("/web", "./web")
		rest.GET("/", func(c *gin.Context) {
			c.File("./web/index.html")
		})
	} else {
		dist := "./web/dist"
		rest.Static("/assets", dist+"/assets")
		rest.StaticFile("/manifest.webmanifest", dist+"/manifest.webmanifest")
		rest.GET("/", func(c *gin.Context) {
			c.File(dist + "/index.html")
		})
		rest.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/auth") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			// 尝试查找静态文件（sw.js, workbox-*.js, registerSW.js 等）
			filePath := dist + p
			if _, err := os.Stat(filePath); err == nil {
				c.File(filePath)
				return
			}
			// SPA 路由 fallback
			c.File(dist + "/index.html")
		})
	}

	// WebSocket
	rest.GET("/ws", s.authMiddleware(), func(c *gin.Context) {
		s.handleWSConnection(c.Writer, c.Request)
	})

	s.restServer = &http.Server{Addr: addr, Handler: rest.Handler()}
	go func() {
		if err := s.restServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("REST server failed", "error", err)
		}
	}()
	return nil
}

func (s *APIServer) Shutdown(ctx context.Context) {
	if s.restServer != nil {
		s.restServer.Shutdown(ctx)
	}
}

// ── 中间件 ──

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
		allowed := false
		for _, o := range s.config.API.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			if hasWildcard {
				// 通配符模式：只在没有凭证时设置 *
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		if s.validateJWT(token) || s.validateAPIToken(token) {
			c.Next()
			return
		}
		c.JSON(401, gin.H{"error": "unauthorized"})
		c.Abort()
	}
}

func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (s *APIServer) validateJWT(token string) bool {
	if token == "" || s.config.API.JWTSecret == "" {
		return false
	}
	claims, err := VerifyJWT(s.config.API.JWTSecret, token)
	if err != nil {
		return false
	}
	// 检查 token 是否已被撤销
	if _, revoked := revokedTokens.Load(claims.Jti); revoked {
		return false
	}
	return true
}

// revokeJWT 将指定 JWT token 加入黑名单
func (s *APIServer) revokeJWT(token string) {
	if token == "" || s.config.API.JWTSecret == "" {
		return
	}
	claims, err := VerifyJWT(s.config.API.JWTSecret, token)
	if err != nil {
		return
	}
	revokedTokens.Store(claims.Jti, true)
}

func (s *APIServer) validateAPIToken(token string) bool {
	if token == "" {
		return false
	}
	expected := s.db.GetSetting("api.token")
	return expected != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// maxBodySizeMiddleware limits request body size to prevent DoS
func maxBodySizeMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// securityHeadersMiddleware sets security-related HTTP headers
func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

// ── WebSocket ──

var wsClientCounter int64

func (s *APIServer) handleWSConnection(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // 非浏览器请求
			}
			for _, allowed := range s.config.API.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
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
			Type    string `json:"type"`
			Message string `json:"message"`
			UserID  string `json:"user_id"`
		}
		if err := json.Unmarshal(msgData, &req); err != nil {
			continue
		}
		if req.Type == "chat" {
			// 简化的聊天处理
			if err := conn.WriteJSON(gin.H{"type": "reply", "content": "echo: " + req.Message}); err != nil {
				break
			}
		}
	}
}

// ── 健康检查 ──

var startTime = time.Now()

func (s *APIServer) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"version": "3.0.0",
		"uptime":  time.Since(startTime).String(),
	})
}

func (s *APIServer) handleStats(c *gin.Context) {
	backends, _ := s.db.ListBackends()
	c.JSON(200, gin.H{
		"backends": len(backends),
		"version":  version.Version,
	})
}

func (s *APIServer) sendNotifyMessage(ctx context.Context, toUser, content string) error {
	accounts, err := s.db.ListAccounts()
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	var errs []error
	for _, acct := range accounts {
		if acct.Token == "" || acct.BaseURL == "" {
			continue
		}
		if err := s.connector.SendTextWithCreds(ctx, &bot.Credentials{
			Token:   acct.Token,
			BaseURL: acct.BaseURL,
		}, toUser, content, ""); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("send errors: %v", errs)
	}
	return nil
}
