package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── 消息分片配置 ──

const (
	MaxMessageLength = 2048  // 单条消息最大长度
	ChunkSeparator   = "\n---\n" // 分片分隔符
)

// SplitMessage 将长消息拆分为多条消息
func SplitMessage(text string, maxLen int) []string {
	if text == "" {
		return []string{}
	}
	if maxLen <= 0 {
		maxLen = MaxMessageLength
	}

	// 如果消息未超限，直接返回
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	// 按段落拆分
	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]string, 0)
	currentChunk := ""

	for _, para := range paragraphs {
		// 如果当前段落加上新段落超限
		if len([]rune(currentChunk))+len([]rune(para))+2 > maxLen {
			// 保存当前 chunk
			if currentChunk != "" {
				chunks = append(chunks, strings.TrimSpace(currentChunk))
			}
			// 如果单个段落就超限，需要进一步拆分
			if len([]rune(para)) > maxLen {
				subChunks := splitByRunes(para, maxLen)
				chunks = append(chunks, subChunks...)
				currentChunk = ""
			} else {
				currentChunk = para
			}
		} else {
			if currentChunk != "" {
				currentChunk += "\n\n"
			}
			currentChunk += para
		}
	}

	// 保存最后一个 chunk
	if currentChunk != "" {
		chunks = append(chunks, strings.TrimSpace(currentChunk))
	}

	return chunks
}

// splitByRunes 按 rune 数量拆分字符串
func splitByRunes(text string, maxLen int) []string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	chunks := make([]string, 0)
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

// SendTextWithCredsChunked 发送分片消息
func (c *Connector) SendTextWithCredsChunked(ctx context.Context, creds *Credentials, toUser, text, contextToken string) error {
	chunks := SplitMessage(text, MaxMessageLength)
	for i, chunk := range chunks {
		// 如果有多片，添加序号
		if len(chunks) > 1 {
			chunk = fmt.Sprintf("[%d/%d]\n%s", i+1, len(chunks), chunk)
		}
		if err := c.SendTextWithCreds(ctx, creds, toUser, chunk, contextToken); err != nil {
			return err
		}
		// 多片之间添加延迟，避免消息乱序
		if i < len(chunks)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}
	return nil
}

// ── 发送消息 ──

type sendMessagePayload struct {
	Msg struct {
		FromUserID   string `json:"from_user_id"`
		ToUserID     string `json:"to_user_id"`
		ClientID     string `json:"client_id"`
		MessageType  int    `json:"message_type"`  // 2=bot
		MessageState int    `json:"message_state"` // 2=finish
		ItemList     []struct {
			Type      int `json:"type"`
			TextItem  *struct {
				Text string `json:"text"`
			} `json:"text_item,omitempty"`
		} `json:"item_list"`
		ContextToken string `json:"context_token,omitempty"`
	} `json:"msg"`
	BaseInfo BaseInfo `json:"base_info"`
}

type sendTypingPayload struct {
	IlinkUserID  string   `json:"ilink_user_id"`
	TypingTicket string   `json:"typing_ticket"`
	Status       int      `json:"status"`
	BaseInfo     BaseInfo `json:"base_info"`
}

// SendRawPayload 发送完整 iLink 消息负载（map 格式）
func (c *Connector) SendRawPayload(ctx context.Context, creds *Credentials, payload map[string]interface{}) error {
	return c.sendMessage(ctx, creds, payload)
}

// ForwardAPIRequest 将请求转发到真实 iLink API，返回原始响应
func (c *Connector) ForwardAPIRequest(ctx context.Context, creds *Credentials, endpoint string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", creds.BaseURL+"/"+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("forward request: %w", err)
	}
	c.setFullHeaders(req, creds.Token, string(body))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forward do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("forward read: %w", err)
	}
	return respBody, nil
}

func (c *Connector) SendTextWithCreds(ctx context.Context, creds *Credentials, toUser, text, contextToken string) error {
	clientID := GenerateClientID()
	var payload sendMessagePayload
	payload.Msg.FromUserID = ""
	payload.Msg.ToUserID = toUser
	payload.Msg.ClientID = clientID
	payload.Msg.MessageType = 2
	payload.Msg.MessageState = 2
	payload.Msg.ItemList = []struct {
		Type      int `json:"type"`
		TextItem  *struct {
			Text string `json:"text"`
		} `json:"text_item,omitempty"`
	}{{
		Type: 1,
		TextItem: &struct {
			Text string `json:"text"`
		}{Text: text},
	}}
	if contextToken != "" {
		payload.Msg.ContextToken = contextToken
	}
	payload.BaseInfo = BuildBaseInfo()

	return c.sendMessage(ctx, creds, &payload)
}

func (c *Connector) sendMessage(ctx context.Context, creds *Credentials, payload interface{}) error {
	// 与 Python json.dumps(ensure_ascii=False) 对齐，不转义 HTML 字符
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("marshal sendMessage payload: %w", err)
	}
	bodyJSON := buf.Bytes()
	// Encode 会添加换行符，去掉
	bodyJSON = bytes.TrimRight(bodyJSON, "\n")

	// 打印发送的 payload 便于调试（不打印完整内容，避免敏感信息泄露）
	c.log.Debug("sendMessage", "base_url", creds.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", creds.BaseURL+"/ilink/bot/sendmessage", bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setFullHeaders(req, creds.Token, string(bodyJSON))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read sendMessage response: %w", err)
	}

	// 打印发送结果便于调试
	c.log.Debug("sendMessage response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode != 200 {
		return fmt.Errorf("sendMessage failed: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应体中的 errcode/errmsg，即使 HTTP 200 也可能有业务错误
	var sendResp struct {
		Ret     int    `json:"ret"`
		Errcode int    `json:"errcode,omitempty"`
		Errmsg  string `json:"errmsg,omitempty"`
	}
	if err := json.Unmarshal(respBody, &sendResp); err == nil {
		if sendResp.Ret != 0 || sendResp.Errcode != 0 {
			return fmt.Errorf("sendMessage api error: ret=%d, errcode=%d, errmsg=%s", sendResp.Ret, sendResp.Errcode, sendResp.Errmsg)
		}
	}
	return nil
}

func (c *Connector) SendText(ctx context.Context, toUser, text, replyID string) error {
	creds := c.findCredentials(replyID)
	if creds == nil {
		accounts := c.GetAccounts()
		if len(accounts) == 0 {
			return fmt.Errorf("no accounts available")
		}
		creds = accounts[0].Credentials
	}
	return c.SendTextWithCreds(ctx, creds, toUser, text, "")
}

func (c *Connector) SendTypingWithCreds(ctx context.Context, creds *Credentials, toUser string) error {
	payload := sendTypingPayload{
		IlinkUserID:  toUser,
		TypingTicket: "",
		Status:       1,
		BaseInfo: BuildBaseInfo(),
	}

	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sendTyping payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", creds.BaseURL+"/ilink/bot/sendtyping", bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setFullHeaders(req, creds.Token, string(bodyJSON))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Connector) SendTyping(ctx context.Context, toUser string) error {
	accounts := c.GetAccounts()
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}
	return c.SendTypingWithCreds(ctx, accounts[0].Credentials, toUser)
}

func (c *Connector) setFullHeaders(req *http.Request, token, body string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", randomUIN())
	req.Header.Set("iLink-App-Id", "bot")
	req.Header.Set("iLink-App-ClientVersion", "131584")
}

func (c *Connector) findCredentials(accountOrUserID string) *Credentials {
	if accountOrUserID == "" {
		return nil
	}
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()
	for _, a := range c.accounts {
		if a.Credentials.AccountID == accountOrUserID || a.Credentials.UserID == accountOrUserID {
			copied := *a.Credentials
			return &copied
		}
	}
	return nil
}

func (c *Connector) GetAccountCredentials(accountID string) *Credentials {
	return c.findCredentials(accountID)
}

// ── 统一发送（按 msg_type 分发） ──

func (c *Connector) SendMessage(ctx context.Context, toUser, content string, msgType int, replyID string) error {
	creds := c.findCredentials(replyID)
	if creds == nil {
		accounts := c.GetAccounts()
		if len(accounts) == 0 {
			return fmt.Errorf("no accounts available")
		}
		creds = accounts[0].Credentials
	}
	switch msgType {
	case 1:
		return c.SendTextWithCreds(ctx, creds, toUser, content, "")
	case 3:
		return c.SendMediaMessage(ctx, toUser, content, 3)
	case 34:
		return c.SendMediaMessage(ctx, toUser, content, 34)
	case 49:
		return c.SendMediaMessage(ctx, toUser, content, 49)
	default:
		return c.SendTextWithCreds(ctx, creds, toUser, content, "")
	}
}
