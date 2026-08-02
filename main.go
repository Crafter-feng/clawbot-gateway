package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
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
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/session"
	"clawbot-gateway/internal/version"
)

func main() {
	// 加载 .env 文件
	godotenv.Load()

	logLevel := os.Getenv("CLAWBOT_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := os.Getenv("CLAWBOT_LOG_FORMAT")
	if logFormat == "" {
		logFormat = "auto"
	}
	log.SetDefault(log.NewWriter(os.Stderr, logLevel, logFormat))
	log := log.Default().WithComponent("main")

	log.Info("starting ClawBot Gateway",
		"level", logLevel,
		"format", logFormat,
		"version", version.Version,
		"commit", version.Commit,
	)

	// 数据库路径
	dbPath := os.Getenv("CLAWBOT_DB_PATH")
	if dbPath == "" {
		dbPath = "data/clawbot.db"
	}

	// ── CLI 子命令 ──
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "reset-password":
			cmdResetPassword(dbPath)
			return
		case "version", "--version", "-v":
			fmt.Printf("ClawBot Gateway %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildTime)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		default:
			if os.Args[1][0] != '-' {
				fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", os.Args[1])
				printUsage()
				os.Exit(1)
			}
		}
	}
	// 1. 初始化数据库
	db, err := database.New(dbPath)
	if err != nil {
		log.Error("failed to init database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database initialized", "path", dbPath)

	// 2. 加载配置
	cfg := config.LoadFromDB(db)
	log.Info("config loaded", "host", cfg.Server.Host, "port", cfg.Server.Port, "jwt_expiry_hours", cfg.API.JWTExpiryHours)

	// 打印登录密码（仅首次生成时）
	if os.Getenv("CLAWBOT_LOGIN_PASSWORD") == "" && cfg.API.LoginPassword != "" {
		fmt.Fprintf(os.Stderr, "\n⚠  Generated login password: %s (change it immediately via Settings page)\n\n", cfg.API.LoginPassword)
	}

	// 3. 初始化路由引擎
	r := route.NewRouter()
	r.SetDefaultBackend(db.GetSetting("route.default_backend"))

	// 3a. 从数据库加载路由规则
	routeRules, err := db.ListRouteRules()
	if err != nil {
		log.Warn("failed to load route rules, continuing with empty", "error", err)
	} else {
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
	if err != nil {
		log.Warn("failed to load user sessions", "error", err)
	} else {
		for _, s := range sessions {
			r.SetUserBackend(s.UserID, s.BackendID, nil)
			r.SetUserRouteMode(s.UserID, s.RouteMode)
		}
	}
	log.Info("router initialized", "routes", len(routeRules), "sessions", len(sessions), "default_backend", r.GetDefaultBackend())

	// 4. 初始化适配器工厂
	af := adapter.NewAdapterFactory()

	// 4a. 从数据库加载后端
	backends, err := db.ListBackends()
	if err != nil {
		log.Warn("failed to load backends, continuing with empty", "error", err)
	} else {
		for _, b := range backends {
			if !b.Enabled {
				continue
			}
			adapterInstance := adapter.CreateAdapterFromDB(b)
			if adapterInstance != nil {
				af.Register(adapterInstance)
			}
		}
	}

	// 首次启动：数据库无后端时注册默认 echo 调试后端
	if len(backends) == 0 {
		log.Info("first startup detected, registering default echo debug backend")
		echoBackend := database.Backend{
			ID:      "echo",
			Name:    "Echo Debug",
			Type:    "echo",
			Config:  "{}",
			Enabled: true,
		}
		if err := db.CreateBackend(echoBackend); err != nil {
			log.Warn("failed to save default echo backend", "error", err)
		}
		af.Register(adapter.NewEchoAdapter("echo", "Echo Debug"))
	}

	// 4b. 加载 ilink_proxy 连接适配器
	vbots, err := db.ListVirtualBots()
	if err != nil {
		log.Warn("failed to load virtual bots", "error", err)
	} else {
		for _, vb := range vbots {
			af.RegisterConnection(adapter.NewILinkProxyAdapter(vb.ID, vb.ID, vb.AccountID, vb.UserID, vb.BaseURL))
		}
	}

	log.Info("adapter factory initialized", "backends", len(af.List()), "connections", len(af.ListConnections()))

	// 5. 初始化上下文管理器
	ttl := time.Duration(cfg.Context.TTL) * time.Second
	cm := session.NewContextManager(cfg.Context.MaxHistory, ttl)

	// 7. 初始化 ClawBot 连接器
	conn := bot.NewConnector(bot.ConnectorConfig{
		BaseURL:     cfg.ClawBot.BaseURL,
		PollTimeout: cfg.ClawBot.PollTimeout,
		BotType:     cfg.ClawBot.BotType,
		Log:         log,
	})
	log.Info("ClawBot connector initialized", "base_url", cfg.ClawBot.BaseURL, "poll_timeout", cfg.ClawBot.PollTimeout, "bot_type", cfg.ClawBot.BotType)

	// 8. 初始化 ClientRegistry（虚拟 Bot 管理）
	clientRegistry := ilink.NewClientRegistry()

	// 8a. 注册虚拟 Bot（使用数据库中的 token，确保重启后 token 不变）
	for _, vb := range vbots {
		vbot := clientRegistry.Register(vb.AccountID, vb.UserID, vb.BaseURL, vb.Token)
		// 如果 token 为空（旧数据库迁移），将新生成的 token 保存回数据库
		if vb.Token == "" && vbot.Token != "" {
			vb.Token = vbot.Token
			db.SaveVirtualBot(vb)
		}
		log.Info("registered virtual bot", "id", vb.ID, "account_id", vb.AccountID)
	}

	// 8b. 设置 Connector 的同步缓冲区存储
	conn.SetSyncBufStore(db)
	// 注意：透明代理模式下，不需要设置消息广播器
	// 虚拟 Bot 通过 iLink 服务端直接访问真实 iLink API

	// 9. 从数据库恢复微信账号
	accounts, err := db.ListAccounts()
	if err != nil {
		log.Warn("failed to load accounts", "error", err)
	} else {
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
	pipeline := api.NewMessagePipeline(conn, r, af, cm, clientRegistry)
	pipeline.SetLogger(log)
	pipeline.SetupProxyAdapters()
	pipeline.Start(pipelineCtx)

	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	apiServer := api.NewAPIServer(cfg, db, r, af, cm, conn, clientRegistry)
	apiServer.SetLogger(log)
	apiServer.Pipeline = pipeline
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

	// 先通知 Pipeline 停止接收新消息，然后等待 in-flight 消息处理完成
	pipelineCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		pipeline.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Info("pipeline drained")
	case <-shutdownCtx.Done():
		log.Warn("pipeline drain timeout, continuing shutdown")
	}

	apiServer.Shutdown(shutdownCtx)

	for _, a := range conn.GetAccounts() {
		conn.RemoveAccount(a.Credentials.AccountID)
	}
	log.Info("stopped")
}

func printUsage() {
	fmt.Println(`ClawBot Gateway - 微信 iLink 消息网关

用法:
  clawbot-gateway                  启动 HTTP 服务
  clawbot-gateway <命令> [参数]     执行管理命令

命令:
  reset-password [密码]   重置登录密码（不指定则生成随机密码）
  version, -v           显示版本信息
  help, -h, --help       显示此帮助信息

环境变量:
  CLAWBOT_DB_PATH        数据库路径（默认: data/clawbot.db）
  CLAWBOT_LOG_LEVEL      日志级别
  CLAWBOT_HOST           监听地址
  CLAWBOT_PORT           监听端口`)
}

func cmdResetPassword(dbPath string) {
	db, err := database.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var newPassword string
	if len(os.Args) > 2 {
		newPassword = os.Args[2]
	} else {
		newPassword = "admin_" + config.GenerateSecret()
	}

	if err := db.SetSetting("api.login_password", newPassword); err != nil {
		fmt.Fprintf(os.Stderr, "设置密码失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("密码已重置成功\n")
	fmt.Printf("新密码: %s\n", newPassword)
	fmt.Printf("\n请使用此密码登录 Web 管理界面\n")
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
