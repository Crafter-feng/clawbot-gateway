package api

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"clawbot-gateway/internal/adapter"
	"clawbot-gateway/internal/bot"
	"clawbot-gateway/internal/ilink"
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
	clientReg   *ilink.ClientRegistry
	commandProc *CommandProcessor

	msgCount int64
	msgMu    sync.Mutex
	wg       sync.WaitGroup
	log      *log.Logger
}

// MessageCount 返回处理的消息总数
func (p *MessagePipeline) MessageCount() int64 {
	p.msgMu.Lock()
	defer p.msgMu.Unlock()
	return p.msgCount
}

func (p *MessagePipeline) SetLogger(l *log.Logger) {
	p.log = l.WithComponent("pipeline")
}
func NewMessagePipeline(
	conn *bot.Connector,
	r *route.Router,
	af *adapter.AdapterFactory,
	cm *session.ContextManager,
	clientReg *ilink.ClientRegistry,
) *MessagePipeline {

	p := &MessagePipeline{
		connector: conn,
		router:    r,
		adapters:  af,
		ctxManager: cm,
		clientReg: clientReg,
	}
	p.commandProc = NewCommandProcessor(r, af, cm, conn)
	return p
}

// Start 启动消息处理循环
func (p *MessagePipeline) Start(ctx context.Context) {
	p.log.Info("message pipeline started")
	var processLoopFunc func()
	processLoopFunc = func() {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				p.log.Error(fmt.Sprintf("processLoop panic: %v", r), "panic", r, "stack", stack)
				time.Sleep(time.Second)
				go processLoopFunc()
			}
		}()
		p.processLoop(ctx)
	}
	go processLoopFunc()
	go p.cleanupLoop(ctx)
}

// Wait 等待所有 in-flight 消息处理完成，用于优雅关闭
func (p *MessagePipeline) Wait() {
	p.wg.Wait()
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
	p.wg.Add(1)
	defer p.wg.Done()

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

		// 一次性转发命令：/<backend_id> 或 /help <backend_id>
		if cmd.Action == "forward_to" {
			backendID := cmd.Args[0]
			forwardContent := content
			parts := strings.Fields(forwardContent)

			if strings.HasPrefix(forwardContent, "/help ") {
				if len(parts) > 2 {
					forwardContent = "/help " + strings.Join(parts[2:], " ")
				} else {
					forwardContent = "/help"
				}
			} else {
				if len(parts) > 1 {
					forwardContent = strings.Join(parts[1:], " ")
				} else {
					reply := p.commandProc.ShowBackendStatus(backendID)
					creds := p.connector.GetAccountCredentials(msg.AccountID)
					if creds != nil {
						contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
						_ = p.connector.SendTextWithCreds(context.WithoutCancel(ctx), creds, msg.FromUser, reply, contextToken)
					}
					return
				}
			}

			p.log.Info("forwarding to backend", "seq", seq, "backend", backendID, "content", forwardContent)
			p.forwardToBackend(ctx, msg, forwardContent, backendID, seq)
			return
		}

		// 执行其他命令（/use, /backends, /help 等）
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

	// 3. 路由决策（三层优先级：用户会话覆写 → 关键词规则 → 默认后端）
	decision := p.router.Route(content, msg.FromUser, msg.FromUser, msg.ToUser, "")
	if decision.BackendID == "" {
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
	backendID := decision.BackendID
	p.log.Info("routing to backend", "seq", seq, "from", msg.FromUser, "backend", backendID, "matched_by", decision.MatchedBy)

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

	// 4. ilink_proxy 是连接适配器：消息入虚拟 Bot 队列，不调用 Handle()，不回复
	if adapter.IsConnectionAdapter(bak.Type()) {
		if p.enqueueToVirtualBot(msg, backendID, seq) {
			return
		}
		reply := fmt.Sprintf("❌ 后端 [%s] 虚拟 Bot 未注册", backendID)
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.WithoutCancel(ctx), creds, msg.FromUser, reply, contextToken)
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
		History:   convertChatHistory(ctxSession.GetHistory()),
	})
	if err != nil {
		p.log.Error("backend error", "seq", seq, "backend", backendID, "error", err)
		reply := fmt.Sprintf("⚠️ [%s] 处理出错，请稍后重试", backendID)
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.WithoutCancel(ctx), creds, msg.FromUser, reply, contextToken)
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
			_ = p.connector.SendTextWithCreds(context.WithoutCancel(ctx), creds, msg.FromUser, reply, contextToken)
		}
		return
	}

	// ilink_proxy 是连接适配器：消息入虚拟 Bot 队列，不调用 Handle()，不回复
	// 外部服务通过 getupdates 消费队列后处理并回复
	if adapter.IsConnectionAdapter(bak.Type()) {
		if p.enqueueToVirtualBot(msg, backendID, seq) {
			return
		}
		// 找不到虚拟 Bot：回复错误
		reply := fmt.Sprintf("❌ 后端 [%s] 虚拟 Bot 未注册", backendID)
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.WithoutCancel(ctx), creds, msg.FromUser, reply, contextToken)
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
		History:   convertChatHistory(ctxSession.GetHistory()),
	})
	if err != nil {
		p.log.Error("backend error", "seq", seq, "backend", backendID, "error", err)
		reply := fmt.Sprintf("⚠️ [%s] 处理出错，请稍后重试", backendID)
		creds := p.connector.GetAccountCredentials(msg.AccountID)
		if creds != nil {
			contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
			_ = p.connector.SendTextWithCreds(context.WithoutCancel(ctx), creds, msg.FromUser, reply, contextToken)
		}
		return
	}
	// 添加回复前缀标识
	replyText := fmt.Sprintf("[%s] %s", backendID, resp.Text)
	ctxSession.AddTurn(content, replyText)

	// 发送回复
	creds := p.connector.GetAccountCredentials(msg.AccountID)
	if creds != nil {
		contextToken := p.connector.GetContextToken(msg.AccountID, msg.FromUser)
		if err := p.connector.SendTextWithCreds(ctx, creds, msg.FromUser, replyText, contextToken); err != nil {
			p.log.Warn("send reply error", "seq", seq, "error", err)
		}
	} else {
		p.log.Warn("no credentials for reply", "seq", seq, "account_id", msg.AccountID)
	}

	p.log.Info("message processed", "seq", seq, "backend", backendID, "reply_chars", len(replyText))
}

// enqueueToVirtualBot 将消息入队到 ilink_proxy 后端对应的虚拟 Bot 队列
// 返回 true 表示成功，false 表示虚拟 Bot 未注册
func (p *MessagePipeline) enqueueToVirtualBot(msg bot.NormalizedMessage, backendID string, seq int64) bool {
	if p.clientReg == nil {
		p.log.Warn("clientReg not set, cannot enqueue", "seq", seq, "backend", backendID)
		return false
	}
	accountID := "gw_" + backendID
	vbot := p.clientReg.Get(accountID)
	if vbot == nil {
		p.log.Warn("virtual bot not registered", "seq", seq, "backend", backendID, "account_id", accountID)
		return false
	}
	// 优先使用原始 RawMessageItem（保留格式），否则从 NormalizedMessage 重建
	rawMsg := msg.GetRawItem()
	if rawMsg == nil {
		rawMsg = &bot.RawMessageItem{
			FromUserid: msg.FromUser,
			MsgID:      json.Number(msg.MsgID),
			MsgType:    msg.Type,
			ItemList: []bot.RawMessageItem_Item{{
				Type:     1,
				TextItem: &bot.RawMessageItem_TextItem{Text: msg.Content},
			}},
			Timestamp:    msg.Timestamp,
			ContextToken: msg.ContextToken,
		}
	}
	vbot.Enqueue(*rawMsg)
	p.log.Info("enqueued to virtual bot queue", "seq", seq, "backend", backendID, "queue_len", vbot.QueueLength())
	return true
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
		decision := p.router.Route(content, userID, userID, "", "")
		if decision.BackendID == "" {
			return "", fmt.Errorf("no backend available")
		}
		backendIDs = []string{decision.BackendID}
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
			History:   convertChatHistory(ctxSession.GetHistory()),
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

	// /help <backend_id> — 转发到指定后端（等同于 /<backend_id>）
	if match := matchPrefix(text, "/help "); match != "" {
		parts := strings.Fields(match)
		if len(parts) > 0 {
			backendID := parts[0]
			if _, ok := cp.adapters.Get(backendID); ok {
				return &CommandMatch{Action: "forward_to", Args: []string{backendID}}
			}
		}
	}

	// 动态生成一次性转发命令：/<backend_id>
	// 根据配置的 providers 自动生成，如 /hermes、/openclaw
	if strings.HasPrefix(text, "/") && !strings.HasPrefix(text, "/use ") && text != "/use" {
		// 提取 / 后面的第一个单词作为 backend ID
		afterSlash := strings.TrimPrefix(text, "/")
		parts := strings.Fields(afterSlash)
		if len(parts) > 0 {
			backendID := parts[0]
			if _, ok := cp.adapters.Get(backendID); ok {
				return &CommandMatch{Action: "forward_to", Args: []string{backendID}}
			}
		}
		// 未知命令 → 返回错误提示，不转发到后端
		return &CommandMatch{Action: "unknown_command", Args: []string{text}}
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

		// /use main — 清除后端选择，回到无后端状态
		if backendID == "main" {
			cp.router.ClearUserBackend(msg.FromUser)
			return "✅ 已切换至主命令模式，后续消息将按路由规则处理"
		}

		bak, ok := cp.adapters.Get(backendID)
		if !ok {
			available := cp.adapters.List()
			names := make([]string, 0)
			for _, b := range available {
				names = append(names, fmt.Sprintf("%s(%s)", b.ID(), b.Name()))
			}
			return fmt.Sprintf("❌ 后端 [%s] 不存在\n可用后端：%s", backendID, strings.Join(names, ", "))
		}
		if err := cp.router.SetUserBackend(msg.FromUser, backendID, cp.adapters.ListIDs()); err != nil {
			return fmt.Sprintf("❌ 切换失败: %s", err.Error())
		}
		return fmt.Sprintf("✅ 已切换至 [%s]，后续消息将使用此后端", bak.Name())

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
		current := "主命令模式（未选择）"
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
		lines = append(lines, "/use main         — 回到主命令模式（清除后端选择）")
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

// ShowBackendStatus 返回指定后端的状态信息
func (cp *CommandProcessor) ShowBackendStatus(backendID string) string {
	bak, ok := cp.adapters.Get(backendID)
	if !ok {
		return fmt.Sprintf("❌ 后端 [%s] 不存在", backendID)
	}
	status := "🟢 健康"
	if !bak.HealthCheck(context.Background()) {
		status = "🔴 异常"
	}
	return fmt.Sprintf(
		"📊 **%s**\n\n"+
			"🆔 ID：%s\n"+
			"🔗 类型：%s\n"+
			"🟢 状态：%s",
		bak.Name(), backendID, bak.Type(), status)
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

// convertChatHistory 将 session.ChatMessage 转换为 adapter.ChatMessage
func convertChatHistory(history []session.ChatMessage) []adapter.ChatMessage {
	result := make([]adapter.ChatMessage, len(history))
	for i, h := range history {
		result[i] = adapter.ChatMessage{Role: h.Role, Content: h.Content}
	}
	return result
}
