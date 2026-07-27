package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── 多微信账号管理 ──

// AddAccount 添加微信账号并启动该账号的独立轮询
func (c *Connector) AddAccount(ctx context.Context, creds *Credentials) error {
	c.accountMu.Lock()
	defer c.accountMu.Unlock()

	// 检查是否已添加同账号
	for _, a := range c.accounts {
		if a.Credentials.AccountID == creds.AccountID {
			return fmt.Errorf("account %s already exists", creds.AccountID)
		}
	}

	info := &AccountInfo{
		Credentials: creds,
		Status:      "online",
	}

	// 启动该账号的独立轮询 goroutine
	ctx, cancel := context.WithCancel(ctx)
	go c.accountPollLoop(ctx, creds)

	info.Connector = c
	info.Cancel = cancel
	c.accounts = append(c.accounts, info)
	c.log.Info("account added", "account_id", creds.AccountID, "user_id", creds.UserID)
	return nil
}

// RemoveAccount 移除并断开指定微信账号
func (c *Connector) RemoveAccount(accountID string) error {
	c.accountMu.Lock()
	defer c.accountMu.Unlock()

	for i, a := range c.accounts {
		if a.Credentials.AccountID == accountID {
			if a.Cancel != nil {
				a.Cancel()
			}
			c.accounts = append(c.accounts[:i], c.accounts[i+1:]...)
			c.log.Info("account removed", "account_id", accountID)
			return nil
		}
	}
	return fmt.Errorf("account %s not found", accountID)
}

// GetAccounts 返回所有已绑定的微信账号
func (c *Connector) GetAccounts() []*AccountInfo {
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()
	result := make([]*AccountInfo, len(c.accounts))
	copy(result, c.accounts)
	return result
}

// GetAccountCount 返回账号数量
func (c *Connector) GetAccountCount() int {
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()
	return len(c.accounts)
}

// AccountExists 检查账号是否已绑定
func (c *Connector) AccountExists(accountID string) bool {
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()
	for _, a := range c.accounts {
		if a.Credentials.AccountID == accountID {
			return true
		}
	}
	return false
}

// ── 多账号独立轮询 ──

func (c *Connector) accountPollLoop(ctx context.Context, creds *Credentials) {
	buf := c.loadSyncBuf(creds.AccountID)
	consecutiveFailures := 0
	const maxFailures = 5
	const backoffDelay = 30 * time.Second
	const retryDelay = 3 * time.Second
	const sessionExpiryPause = 10 * time.Minute

	for {
		select {
		case <-ctx.Done():
			c.log.Info("poll loop stopped", "account_id", creds.AccountID)
			c.saveSyncBuf(creds.AccountID, buf)
			return
		default:
		}

		resp, err := c.getUpdatesWithCreds(ctx, creds, buf)
		if err != nil {
			if ctx.Err() != nil {
				c.saveSyncBuf(creds.AccountID, buf)
				return
			}
			consecutiveFailures++
			c.log.Warn("getUpdates error", "account_id", creds.AccountID, "error", err, "failures", consecutiveFailures)
			delay := retryDelay
			if consecutiveFailures >= maxFailures {
				delay = backoffDelay
				consecutiveFailures = 0
			}
			select {
			case <-ctx.Done():
				c.saveSyncBuf(creds.AccountID, buf)
				return
			case <-time.After(delay):
			}
			continue
		}

		// 检查会话过期 (ret=-14 或 errcode=-14) 或 stale session (ret=-2/errcode=-2 且 errmsg 包含 "unknown error")
		isSessionExpired := resp.Ret == -14 || resp.Errcode == -14
		isStaleSession := (resp.Ret == -2 || resp.Errcode == -2) &&
			strings.Contains(strings.ToLower(resp.Errmsg), "unknown error")

		if isSessionExpired || isStaleSession {
			if isStaleSession {
				c.log.Warn("stale session detected (ret/errcode=-2, unknown error), treating as expired", "account_id", creds.AccountID)
			} else {
				c.log.Error("session expired for account, pausing 10min", "account_id", creds.AccountID)
			}
			c.saveSyncBuf(creds.AccountID, buf)
			select {
			case <-ctx.Done():
				c.saveSyncBuf(creds.AccountID, buf)
				return
			case <-time.After(sessionExpiryPause):
			}
			consecutiveFailures = 0
			continue
		}

		if resp.Ret != 0 || resp.Errcode != 0 {
			delay := retryDelay
			if consecutiveFailures >= maxFailures {
				delay = backoffDelay
				consecutiveFailures = 0
			}
			select {
			case <-ctx.Done():
				c.saveSyncBuf(creds.AccountID, buf)
				return
			case <-time.After(delay):
			}
			continue
		}

		consecutiveFailures = 0

		for _, raw := range resp.Msgs {
			// 打印原始消息便于调试
			if rawJSON, err := json.Marshal(raw); err == nil {
				c.log.Info("raw message", "account_id", creds.AccountID, "json", string(rawJSON))
			}
			// 缓存 context_token
			if raw.ContextToken != "" {
				c.SetContextToken(creds.AccountID, raw.FromUserid, raw.ContextToken)
			}
			msg := normalize(raw)
			msg.AccountID = creds.AccountID

			// 发送到 Pipeline（内置 AI 处理）
			select {
			case c.msgChan <- msg:
			default:
				c.log.Warn("msg channel full, dropping msg", "account_id", creds.AccountID)
			}
			// 注意：透明代理模式下，不需要广播到虚拟 Bot
			// 虚拟 Bot 通过 iLink 服务端直接访问真实 iLink API
		}

		buf = resp.GetUpdatesBuf
		c.saveSyncBuf(creds.AccountID, buf)
	}
}


func (c *Connector) loadSyncBuf(accountID string) string {
	c.mu.RLock()
	store := c.syncBufStore
	c.mu.RUnlock()
	if store != nil {
		return store.GetSyncBuf(accountID)
	}
	return ""
}

func (c *Connector) saveSyncBuf(accountID, buf string) {
	c.mu.RLock()
	store := c.syncBufStore
	c.mu.RUnlock()
	if store != nil {
		if err := store.SetSyncBuf(accountID, buf); err != nil {
			c.log.Warn("failed to save sync buf", "account_id", accountID, "error", err)
		}
	}
}

// getUpdatesWithCreds 使用指定凭证获取消息（不依赖全局 token）
func (c *Connector) getUpdatesWithCreds(ctx context.Context, creds *Credentials, buf string) (*GetUpdatesResp, error) {
	body := GetUpdatesReq{
		GetUpdatesBuf: buf,
		BaseInfo: BaseInfo{
			ChannelVersion: "1.0.2",
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", creds.BaseURL+"/ilink/bot/getupdates", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	c.setFullHeaders(req, creds.Token, string(bodyBytes))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var result GetUpdatesResp
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("parse getupdates response: %w", err)
	}

	return &result, nil
}
