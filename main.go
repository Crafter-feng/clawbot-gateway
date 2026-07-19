package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/session"
	"clawbot-gateway/internal/api"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/config"
	"clawbot-gateway/internal/log"
	"clawbot-gateway/internal/relay"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/store"
)

// tokenAccountID 从 token 派生确定性的账号 ID，确保重启后 ID 一致
func tokenAccountID(token string) string {
	h := sha256.Sum256([]byte(token))
	return "cfg_" + hex.EncodeToString(h[:8])
}

func main() {
	logLevel := os.Getenv("CLAWBOT_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	log.SetDefault(log.New(logLevel))

	// 1. 加载配置
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Default().Error("failed to load config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	if cfg.LogLevel != "" {
		logLevel = cfg.LogLevel
	}
	log.SetDefault(log.New(logLevel))
	log := log.Default().WithComponent("main")

	log.Info("config loaded", "path", cfgPath)

	// 2. 初始化存储
	st, err := store.NewStore("data/store.json")
	if err != nil {
		log.Error("failed to init store", "error", err)
		os.Exit(1)
	}
	log.Info("store initialized")

	// 2b. 初始化账号存储（每个账号独立文件，位于 data/accounts/ 目录下）
	accountStore, err := store.NewAccountStore("data/accounts")
	if err != nil {
		log.Error("failed to init account store", "error", err)
		os.Exit(1)
	}
	log.Info("account store initialized", "count", accountStore.Count())

	// 3. 初始化路由引擎
	r := route.NewRouter(cfg.Backend.DefaultBackend)

	// 3a. 从配置加载 keyword_rules（静态配置，config.yaml 中定义）
	for _, rule := range cfg.Backend.KeywordRules {
		for _, kw := range rule.Keywords {
			// config.yaml 规则默认 isRegexp=false（非正则）
			_ = r.AddKeywordRule(kw, rule.Backend, false)
		}
	}

	// 3b. 从存储中恢复运行时添加的路由规则（覆盖或追加到配置规则之上）
	for _, rule := range st.GetKeywordRules() {
		r.AddKeywordRule(rule.Keyword, rule.Backend, rule.IsRegexp)
	}
	// 恢复用户后端设置
	for userID, backendID := range st.GetUserBackends() {
		r.SetUserBackend(userID, backendID)
	}
	for userID, mode := range st.GetUserRouteModes() {
		r.SetUserRouteMode(userID, mode)
	}
	for userID, secondaries := range st.GetUserSecondaries() {
		r.SetUserSecondaries(userID, secondaries)
	}
	log.Info("router initialized", "default_backend", cfg.Backend.DefaultBackend)

	// 4. 初始化适配器工厂
	af := adapter.NewAdapterFactory()

	// 从配置加载后端
	for _, provider := range cfg.Backend.Providers {
		if !provider.Enabled {
			continue
		}
		switch provider.Type {
		case "echo":
			af.Register(adapter.NewEchoAdapter(provider.ID, provider.Name))
		case "openai_compatible":
			apiKey, _ := provider.Config["api_key"].(string)
			baseURL, _ := provider.Config["base_url"].(string)
			model, _ := provider.Config["model"].(string)
			af.Register(adapter.NewOpenAICompatibleAdapter(provider.ID, provider.Name, apiKey, baseURL, model))
		case "webhook":
			url, _ := provider.Config["url"].(string)
			headersRaw, _ := provider.Config["headers"].(map[string]interface{})
			headers := make(map[string]string)
			for k, v := range headersRaw {
				if s, ok := v.(string); ok {
					headers[k] = s
				}
			}
			af.Register(adapter.NewWebhookAdapter(provider.ID, provider.Name, url, headers))
		default:
			log.Warn("unknown backend type", "type", provider.Type, "id", provider.ID)
		}
	}
	// 注册默认 echo 调试后端
	af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	log.Info("adapter factory initialized", "count", len(af.List()))

	// 4b. 初始化文件中转管理器
	relayMgr := relay.NewRelayManager()
	relayMgr.SetLogger(log)
	// 注册 Obsidian 中转（可通过配置启用）
	// relayMgr.Register(relay.NewObsidianRelay("/path/to/vault", "临时收集", true))
	log.Info("relay manager initialized")

	// 5. 初始化上下文管理器
	ttl := time.Duration(cfg.Context.TTL) * time.Second
	cm := session.NewContextManager(cfg.Context.MaxHistory, cfg.Context.SwitchStrategy, ttl)

	// 6. 初始化 ClawBot 连接器
	conn := bot.NewConnector(bot.ConnectorConfig{
		BaseURL:     cfg.ClawBot.BaseURL,
		PollTimeout: cfg.ClawBot.PollTimeout,
		Token:       cfg.ClawBot.Token,
		BotType:     cfg.ClawBot.BotType,
		DataDir:     "data/syncbuf",
		Log:         log,
	})
	log.Info("ClawBot connector initialized")

	// 7. 如果有保存的凭证，自动恢复多账号连接
	storedCreds := accountStore.List()
	existingIDs := make(map[string]bool)
	for _, cred := range storedCreds {
		existingIDs[cred.AccountID] = true
		c := &bot.Credentials{
			Token:     cred.Token,
			BaseURL:   cred.BaseURL,
			AccountID: cred.AccountID,
			UserID:    cred.UserID,
			LoginAt:   cred.LoginAt,
		}
		if err := conn.AddAccount(context.Background(), c); err != nil {
			log.Warn("auto-reconnect account failed", "account_id", cred.AccountID, "error", err)
		} else {
			log.Info("auto-connected ClawBot account", "account_id", cred.AccountID, "user_id", cred.UserID)
		}
	}
	if len(storedCreds) > 0 {
		log.Info("restored ClawBot accounts from store", "count", len(storedCreds))
	}

	// 7b. 从配置注册初始账号（当 store 中没有且 config 中配置了账号时）
	initCtx := context.Background()
	registered := 0

	// 优先使用 clawbot.accounts[]（多账号配置）
	for _, acct := range cfg.ClawBot.Accounts {
		if acct.Token == "" {
			continue
		}
		aid := acct.AccountID
		if aid == "" {
			aid = tokenAccountID(acct.Token)
		}
		if existingIDs[aid] {
			continue
		}
		uid := acct.UserID
		if uid == "" {
			uid = "user_" + aid
		}
		baseURL := acct.BaseURL
		if baseURL == "" {
			baseURL = cfg.ClawBot.BaseURL
		}
		creds := &bot.Credentials{
			Token:     acct.Token,
			BaseURL:   baseURL,
			AccountID: aid,
			UserID:    uid,
			LoginAt:   time.Now().Unix(),
		}
		if err := conn.AddAccount(initCtx, creds); err != nil {
			log.Warn("auto-register account failed", "account_id", aid, "error", err)
		} else {
			log.Info("auto-registered ClawBot account from config", "account_id", aid, "user_id", uid)
			_ = accountStore.Save(store.StoredCredential{
				AccountID:   aid,
				Token:       acct.Token,
				BaseURL:     baseURL,
				UserID:      uid,
				AccountName: acct.AccountName,
				LoginAt:     time.Now().Unix(),
			})
			registered++
		}
	}

	if registered > 0 {
		log.Info("registered ClawBot accounts from config", "count", registered)
	}

	// 8. 初始化消息处理管道
	pipelineCtx, pipelineCancel := context.WithCancel(context.Background())
	pipeline := api.NewMessagePipeline(conn, r, af, cm)
	pipeline.SetLogger(log)
	pipeline.Start(pipelineCtx)

	// 9. 启动 API 服务器
	apiServer := api.NewAPIServer(cfg, st, accountStore, r, af, cm, conn)
	apiServer.SetLogger(log)
	if err := apiServer.Start(); err != nil {
		log.Error("failed to start API server", "error", err)
		os.Exit(1)
	}

	// 10. 等待关闭信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")

	// 停止消息处理管道
	pipelineCancel()

	// 优雅关闭 HTTP 服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	apiServer.Shutdown(shutdownCtx)

	// 断开所有微信账号
	for _, a := range conn.GetAccounts() {
		if err := conn.RemoveAccount(a.Credentials.AccountID); err != nil {
			log.Warn("error disconnecting account", "account_id", a.Credentials.AccountID, "error", err)
		}
	}
	log.Info("ClawBot Proxy Gateway stopped")
}
