package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/api"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/config"
	"clawbot-gateway/internal/database"
	"clawbot-gateway/internal/ilink"
	"clawbot-gateway/internal/log"
	"clawbot-gateway/internal/relay"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/session"
)

func main() {
	// 加载 .env 文件
	godotenv.Load()

	logLevel := os.Getenv("CLAWBOT_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	log.SetDefault(log.New(logLevel))
	log := log.Default().WithComponent("main")

	// 1. 初始化数据库
	dbPath := os.Getenv("CLAWBOT_DB_PATH")
	if dbPath == "" {
		dbPath = "data/clawbot.db"
	}
	db, err := database.New(dbPath)
	if err != nil {
		log.Error("failed to init database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database initialized", "path", dbPath)

	// 2. 加载配置
	cfg := config.LoadFromDB(db)
	log.Info("config loaded")

	// 打印登录密码（仅首次生成时）
	if os.Getenv("CLAWBOT_LOGIN_PASSWORD") == "" {
		log.Info("login password (auto-generated)", "password", cfg.API.LoginPassword)
	}

	// 3. 初始化路由引擎
	r := route.NewRouter()
	r.SetDefaultBackend(db.GetSetting("route.default_backend"))

	// 3a. 从数据库加载路由规则
	routeRules, err := db.ListRouteRules()
	if err == nil {
		for _, rule := range routeRules {
			r.AddRule(route.RouteRule{
				ID:         rule.ID,
				Name:       rule.Name,
				BackendID:  rule.BackendID,
				Priority:   rule.Priority,
				Enabled:    rule.Enabled,
				Groups:     convertGroups(rule.Groups),
				GroupLogic: rule.GroupLogic,
			})
		}
	}

	// 3b. 恢复用户后端设置
	sessions, err := db.GetAllUserSessions()
	if err == nil {
		for _, s := range sessions {
			r.SetUserBackend(s.UserID, s.BackendID)
			r.SetUserRouteMode(s.UserID, s.RouteMode)
		}
	}
	log.Info("router initialized")

	// 4. 初始化适配器工厂
	af := adapter.NewAdapterFactory()

	// 4a. 从数据库加载后端
	backends, err := db.ListBackends()
	if err == nil {
		for _, b := range backends {
			if !b.Enabled {
				continue
			}
			adapterInstance := createAdapterFromDB(b)
			if adapterInstance != nil {
				af.Register(adapterInstance)
			}
		}
	}

	// 4b. 加载 ilink_proxy 连接适配器
	vbots, err := db.ListVirtualBots()
	if err == nil {
		for _, vb := range vbots {
			af.RegisterConnection(adapter.NewILinkProxyAdapter(vb.ID, vb.ID, vb.AccountID, vb.UserID, vb.BaseURL))
		}
	}

	// 注册默认 echo 调试后端
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	log.Info("adapter factory initialized", "backends", len(af.List()), "connections", len(af.ListConnections()))

	// 5. 初始化文件中转管理器
	relayMgr := relay.NewRelayManager()
	relayMgr.SetLogger(log)
	log.Info("relay manager initialized")

	// 6. 初始化上下文管理器
	ttl := time.Duration(cfg.Context.TTL) * time.Second
	cm := session.NewContextManager(cfg.Context.MaxHistory, cfg.Context.SwitchStrategy, ttl)

	// 7. 初始化 ClawBot 连接器
	conn := bot.NewConnector(bot.ConnectorConfig{
		BaseURL:     cfg.ClawBot.BaseURL,
		PollTimeout: cfg.ClawBot.PollTimeout,
		BotType:     cfg.ClawBot.BotType,
		Log:         log,
	})
	log.Info("ClawBot connector initialized")

	// 8. 初始化 ClientRegistry（虚拟 Bot 管理）
	clientRegistry := ilink.NewClientRegistry()

	// 8a. 注册虚拟 Bot
	for _, vb := range vbots {
		clientRegistry.Register(vb.AccountID, vb.UserID, vb.BaseURL)
		log.Info("registered virtual bot", "id", vb.ID, "account_id", vb.AccountID)
	}

	// 8b. 设置 Connector 的同步缓冲区存储
	conn.SetSyncBufStore(db)
	// 注意：透明代理模式下，不需要设置消息广播器
	// 虚拟 Bot 通过 iLink 服务端直接访问真实 iLink API

	// 9. 从数据库恢复微信账号
	accounts, err := db.ListAccounts()
	if err == nil {
		for _, acct := range accounts {
			creds := &bot.Credentials{
				Token:     acct.Token,
				BaseURL:   acct.BaseURL,
				AccountID: acct.AccountID,
				UserID:    acct.UserID,
				LoginAt:   acct.LoginAt,
			}
			if err := conn.AddAccount(context.Background(), creds); err != nil {
				log.Warn("auto-reconnect account failed", "account_id", acct.AccountID, "error", err)
			} else {
				log.Info("auto-connected account", "account_id", acct.AccountID)
			}
		}
	}

	// 10. 初始化消息处理管道
	pipelineCtx, pipelineCancel := context.WithCancel(context.Background())
	pipeline := api.NewMessagePipeline(conn, r, af, cm)
	pipeline.SetLogger(log)
	pipeline.Start(pipelineCtx)

	// 11. 启动 API 服务器
	apiServer := api.NewAPIServer(cfg, db, r, af, cm, conn, clientRegistry)
	apiServer.SetLogger(log)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := apiServer.Start(addr); err != nil {
		log.Error("failed to start API server", "error", err)
		os.Exit(1)
	}
	log.Info("API server started", "addr", addr)

	// 12. 等待关闭信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	pipelineCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	apiServer.Shutdown(shutdownCtx)

	for _, a := range conn.GetAccounts() {
		conn.RemoveAccount(a.Credentials.AccountID)
	}
	log.Info("stopped")
}

func createAdapterFromDB(b database.Backend) adapter.BackendAdapter {
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
	default:
		return nil
	}
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

// convertGroups 将数据库的路由组转换为路由引擎格式
func convertGroups(dbGroups []database.RouteRuleGroup) []route.RouteRuleGroup {
	groups := make([]route.RouteRuleGroup, len(dbGroups))
	for i, dbGroup := range dbGroups {
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
	return groups
}
