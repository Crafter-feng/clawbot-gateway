package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/log"
	"clawbot-gateway/internal/route"
	"clawbot-gateway/internal/session"
)

// ── 消息处理管道 ──
// 将 ClawBot 收到的消息经过：命令解析 → 路由决策 → 后端处理 → 回复

type MessagePipeline struct {
	connector   *bot.Connector
	router      *route.Router
	adapters    *adapter.AdapterFactory
	ctxManager  *session.ContextManager
	commandProc *CommandProcessor

	msgCount int64
	msgMu    sync.Mutex
	log      *log.Logger
}

func (p *MessagePipeline) SetLogger(l *log.Logger) {
	p.log = l.WithComponent("pipeline")
}

func NewMessagePipeline(
	conn *bot.Connector,
	r *route.Router,
	af *adapter.AdapterFactory,
	cm *session.ContextManager,
) *MessagePipeline {

	p := &MessagePipeline{
		connector:  conn,
		router:     r,
		adapters:   af,
		ctxManager: cm,
	}
	p.commandProc = NewCommandProcessor(r, af, cm, conn)
	return p
}

// Start 启动消息处理循环
func (p *MessagePipeline) Start(ctx context.Context) {
	p.log.Info("message pipeline started")
	go p.processLoop(ctx)
	go p.cleanupLoop(ctx)
}

func (p *MessagePipeline) processLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			p.log.Info("message pipeline stopped")
			return
		case msg, ok := <-p.connector.Messages():
			if !ok {
				return
			}
			p.msgMu.Lock()
			p.msgCount++
			count := p.msgCount
			p.msgMu.Unlock()

			p.log.Debug("message received", "seq", count, "from", msg.FromUser, "content", truncate(msg.Content, 50))
			p.processMessage(ctx, msg, count)
		}
	}
}

func (p *MessagePipeline) processMessage(ctx context.Context, msg bot.NormalizedMessage, seq int64) {
	if !p.connector.IsRunning() {
		p.log.Warn("connector not running, skipping message", "seq", seq)
		return
	}

	// 1. 语音消息模板
	content := msg.Content
	if msg.Type == 3 || strings.Contains(content, "[语音]") {
		content = "[收到一条语音消息，已自动转录为文字]\n" + content
	}

	// 2. 命令解析（始终执行，优先级最高）
	if cmd := p.commandProc.Parse(content); cmd != nil {
		p.log.Info("command matched", "seq", seq, "action", cmd.Action, "args", cmd.Args)

		// 一次性转发命令：/<backend_id>
		if cmd.Action == "forward_to" {
			backendID := cmd.Args[0]
			p.log.Info("forwarding to backend", "seq", seq, "backend", backendID)
			p.forwardToBackend(ctx, msg, content, backendID, seq)
			return
		}

		reply := p.commandProc.Execute(cmd, msg)
		if reply != "" {
			creds := p.connector.GetAccountCredentials(msg.AccountID)
			if creds != nil {
				contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
				p.log.Info("sending command reply", "seq", seq, "to", msg.FromUser, "reply_len", len(reply))
				if err := p.connector.SendTextWithCreds(ctx, creds, msg.FromUser, reply, contextToken); err != nil {
					p.log.Warn("command reply send error", "seq", seq, "error", err)
				}
			} else {
				p.log.Warn("no credentials for command reply", "seq", seq, "account_id", msg.AccountID)
			}
		} else {
			p.log.Warn("command returned empty reply", "seq", seq, "action", cmd.Action)
		}
		return
	}

	// 3. 检查用户是否已选中后端
	backendID, hasOverride := p.router.GetUserBackend(msg.FromUser)
	if !hasOverride {
		// 未选中后端 → 提示用户先选后端
		p.log.Info("no backend selected", "seq", seq, "from", msg.FromUser)
		reply := "❌ 请先选择后端\n输入 /backends 查看可用后端\n输入 /use <后端ID> 切换"
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			if err := p.connector.SendTextWithCreds(ctx, creds, msg.FromUser, reply, contextToken); err != nil {
				p.log.Warn("reply send error", "seq", seq, "error", err)
			}
		}
		return
	}

	// 4. 已选中后端 → 直接路由到该后端（不使用正则）
	p.log.Info("routing to selected backend", "seq", seq, "from", msg.FromUser, "backend", backendID)

	bak, ok := p.adapters.Get(backendID)
	if !ok {
		p.log.Warn("selected backend not found", "seq", seq, "backend", backendID)
		reply := fmt.Sprintf("❌ 后端 [%s] 不可用，请输入 /backends 查看可用后端", backendID)
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(ctx, creds, msg.FromUser, reply, contextToken)
		}
		return
	}

	// 5. 处理消息
	ctxSession := p.ctxManager.GetContext(msg.FromUser, backendID)
	backendCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := bak.Handle(backendCtx, &adapter.ChatRequest{
		Message:   content,
		UserID:    msg.FromUser,
		BackendID: backendID,
		History:   ctxSession.GetHistory(),
	})
	if err != nil {
		p.log.Warn("backend error", "seq", seq, "backend", backendID, "error", err)
		reply := fmt.Sprintf("⚠️ [%s] 处理出错: %s", backendID, err.Error())
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.Background(), creds, msg.FromUser, reply, contextToken)
		}
		return
	}

	ctxSession.AddTurn(content, resp.Text)

	// 6. 发送回复
	creds := p.connector.GetAccountCredentials(msg.AccountID)
	if creds != nil {
		contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
		if err := p.connector.SendTextWithCreds(ctx, creds, msg.FromUser, resp.Text, contextToken); err != nil {
			p.log.Warn("send reply error", "seq", seq, "error", err)
		}
	} else {
		p.log.Warn("no credentials for reply", "seq", seq, "account_id", msg.AccountID)
	}

	p.log.Info("message processed", "seq", seq, "backend", backendID, "reply_chars", len(resp.Text))
}

// forwardToBackend 转发消息到指定后端（一次性，不切换）
func (p *MessagePipeline) forwardToBackend(ctx context.Context, msg bot.NormalizedMessage, content, backendID string, seq int64) {
	bak, ok := p.adapters.Get(backendID)
	if !ok {
		p.log.Warn("backend not found", "seq", seq, "backend", backendID)
		reply := fmt.Sprintf("❌ 后端 [%s] 不可用\n输入 /backends 查看可用后端", backendID)
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.Background(), creds, msg.FromUser, reply, contextToken)
		}
		return
	}

	// 处理消息
	ctxSession := p.ctxManager.GetContext(msg.FromUser, backendID)
	backendCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := bak.Handle(backendCtx, &adapter.ChatRequest{
		Message:   content,
		UserID:    msg.FromUser,
		BackendID: backendID,
		History:   ctxSession.GetHistory(),
	})
	if err != nil {
		p.log.Warn("backend error", "seq", seq, "backend", backendID, "error", err)
		reply := fmt.Sprintf("⚠️ [%s] 处理出错: %s", backendID, err.Error())
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.Background(), creds, msg.FromUser, reply, contextToken)
		}
		return
	}

	ctxSession.AddTurn(content, resp.Text)

	// 发送回复
	creds := p.connector.GetAccountCredentials(msg.AccountID)
	if creds != nil {
		contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
		if err := p.connector.SendTextWithCreds(ctx, creds, msg.FromUser, resp.Text, contextToken); err != nil {
			p.log.Warn("send reply error", "seq", seq, "error", err)
		}
	} else {
		p.log.Warn("no credentials for reply", "seq", seq, "account_id", msg.AccountID)
	}

	p.log.Info("message processed", "seq", seq, "backend", backendID, "reply_chars", len(resp.Text))
}

func (p *MessagePipeline) typingKeepalive(ctx context.Context, accountID, userID string, done chan struct{}) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if p.connector.IsRunning() {
				creds := p.connector.GetAccountCredentials(accountID)
				if creds != nil {
					_ = p.connector.SendTypingWithCreds(context.Background(), creds, userID)
				}
			}
		}
	}
}

func (p *MessagePipeline) adapterName(backendID string) string {
	adapter, ok := p.adapters.Get(backendID)
	if ok {
		return adapter.Name()
	}
	return backendID
}

func (p *MessagePipeline) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.ctxManager.CleanupExpired()
		}
	}
}

// HandleDirectMessage 处理外部系统通过 API 发送的消息（不经过 ClawBot）
// 支持通过 backendID 指定单一后端，或留空使用路由决策（含多后端模式）
func (p *MessagePipeline) HandleDirectMessage(ctx context.Context, content, userID, backendID string) (string, error) {
	backendIDs := []string{}
	if backendID != "" {
		backendIDs = []string{backendID}
	} else {
		decisions := p.router.RouteMulti(content, userID)
		for _, d := range decisions {
			backendIDs = append(backendIDs, d.BackendID)
		}
	}

	var replies []string
	for _, bid := range backendIDs {
		bak, ok := p.adapters.Get(bid)
		if !ok {
			replies = append(replies, fmt.Sprintf("[%s] 不可用", bid))
			continue
		}
		ctxSession := p.ctxManager.GetContext(userID, bid)
		resp, err := bak.Handle(ctx, &adapter.ChatRequest{
			Message:   content,
			UserID:    userID,
			BackendID: bid,
			History:   ctxSession.GetHistory(),
		})
		if err != nil {
			replies = append(replies, fmt.Sprintf("[%s] 错误: %s", bid, err.Error()))
			continue
		}
		ctxSession.AddTurn(content, resp.Text)
		if len(backendIDs) > 1 && bid != backendIDs[0] {
			replies = append(replies, fmt.Sprintf("[%s] %s", p.adapterName(bid), resp.Text))
		} else {
			replies = append(replies, resp.Text)
		}
	}
	return strings.Join(replies, "\n\n---\n\n"), nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// ── 命令处理器 ──

type CommandMatch struct {
	Action string
	Args   []string
}

type CommandProcessor struct {
	router      *route.Router
	adapters    *adapter.AdapterFactory
	ctxManager  *session.ContextManager
	connector   *bot.Connector
	log         *log.Logger
}

func NewCommandProcessor(r *route.Router, af *adapter.AdapterFactory, cm *session.ContextManager, conn *bot.Connector) *CommandProcessor {
	return &CommandProcessor{
		router:      r,
		adapters:    af,
		ctxManager:  cm,
		connector:   conn,
		log:         log.Default().WithComponent("command"),
	}
}

func (cp *CommandProcessor) Parse(text string) *CommandMatch {
	text = strings.TrimSpace(text)
	cp.log.Debug("command parse", "text", text)

	// /use — 无参数显示状态，有参数切换后端
	if text == "/use" {
		return &CommandMatch{Action: "show_status", Args: []string{}}
	}
	if match := matchPrefix(text, "/use "); match != "" {
		return &CommandMatch{Action: "switch_backend", Args: []string{match}}
	}

	// /backends — 列出所有后端
	if text == "/backends" {
		return &CommandMatch{Action: "list_backends"}
	}

	// /help — 显示帮助
	if text == "/help" {
		return &CommandMatch{Action: "show_help"}
	}

	// 动态生成一次性转发命令：/<backend_id>
	// 根据配置的 providers 自动生成，如 /hermes、/openclaw
	if strings.HasPrefix(text, "/") && text != "/use" {
		backendID := strings.TrimPrefix(text, "/")
		if backendID != "" {
			if _, ok := cp.adapters.Get(backendID); ok {
				return &CommandMatch{Action: "forward_to", Args: []string{backendID}}
			}
		}
	}

	return nil
}

func (cp *CommandProcessor) Execute(cmd *CommandMatch, msg bot.NormalizedMessage) string {
	switch cmd.Action {
	case "switch_backend":
		if len(cmd.Args) == 0 || cmd.Args[0] == "" {
			return "❌ 请指定后端 ID，例如: /use openclaw\n输入 /backends 查看可用后端"
		}
		backendID := cmd.Args[0]
		if _, ok := cp.adapters.Get(backendID); !ok {
			available := cp.adapters.List()
			names := make([]string, 0)
			for _, b := range available {
				names = append(names, fmt.Sprintf("%s(%s)", b.ID(), b.Name()))
			}
			return fmt.Sprintf("❌ 后端 [%s] 不存在\n可用后端：%s", backendID, strings.Join(names, ", "))
		}
		cp.router.SetUserBackend(msg.FromUser, backendID)
		adapter, _ := cp.adapters.Get(backendID)
		return fmt.Sprintf("✅ 已切换至 [%s]，后续消息将使用此后端", adapter.Name())

	case "list_backends":
		backends := cp.adapters.List()
		currentBackend, hasOverride := cp.router.GetUserBackend(msg.FromUser)
		lines := []string{"📊 **可用后端**"}
		for _, b := range backends {
			mark := "  "
			if hasOverride && b.ID() == currentBackend {
				mark = "▶"
			}
			status := "🟢"
			if !b.HealthCheck(context.Background()) {
				status = "🔴"
			}
			lines = append(lines, fmt.Sprintf("  %s %s %s — %s", mark, status, b.ID(), b.Name()))
		}
		if hasOverride {
			lines = append(lines, fmt.Sprintf("\n当前后端：%s", currentBackend))
		}
		return strings.Join(lines, "\n")

	case "show_status":
		backends := cp.adapters.List()
		healthyCount := 0
		for _, b := range backends {
			if b.HealthCheck(context.Background()) {
				healthyCount++
			}
		}
		currentBackend, hasOverride := cp.router.GetUserBackend(msg.FromUser)
		current := "未选择"
		if hasOverride {
			current = currentBackend
		}
		return fmt.Sprintf(
			"📊 **ClawBot 状态**\n\n"+
				"🟢 当前后端：**%s**\n"+
				"🔌 已注册后端：%d（%d 在线）",
			current, len(backends), healthyCount)

	case "show_help":
		backends := cp.adapters.List()
		lines := []string{"📖 **帮助**"}
		lines = append(lines, "/use              — 查看当前状态")
		lines = append(lines, "/use <后端ID>     — 切换后端（持久）")
		lines = append(lines, "/backends         — 列出所有后端")
		lines = append(lines, "/help             — 显示此帮助")
		if len(backends) > 0 {
			lines = append(lines, "")
			lines = append(lines, "⚡ 一次性转发命令（不切换后端）：")
			for _, b := range backends {
				lines = append(lines, fmt.Sprintf("/%-16s — 转发到 %s", b.ID(), b.Name()))
			}
		}
		return strings.Join(lines, "\n")

	default:
		return "❓ 未知命令，输入 /help 查看帮助"
	}
}

func matchPrefix(text string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			result := strings.TrimSpace(text[len(prefix):])
			if result != "" {
				return result
			}
		}
	}
	return ""
}
