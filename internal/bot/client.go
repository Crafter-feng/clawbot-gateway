package bot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"clawbot-gateway/internal/log"
)

// ── ClawBot Connector ──

type Connector struct {
	mu            sync.RWMutex
	baseURL       string
	pollTimeout   int
	updateBuf     string
	msgChan       chan NormalizedMessage
	client        *http.Client
	accounts      []*AccountInfo
	accountMu     sync.RWMutex
	qrManager     *QRCodeManager
	botType       int
	contextTokens map[string]string  // accountID:userID → context_token
	syncBufStore  SyncBufStore       // 轮询游标存储
	log           *log.Logger
}

type AccountInfo struct {
	Credentials *Credentials
	Connector   *Connector
	Status      string // "online", "offline"
	Cancel      context.CancelFunc
}

type ConnectorConfig struct {
	BaseURL     string
	PollTimeout int
	BotType     int            // iLink bot_type 参数（默认 3）
	Log         *log.Logger    // 日志记录器，nil 时使用默认
}

func NewConnector(cfg ConnectorConfig) *Connector {
	if cfg.BotType == 0 {
		cfg.BotType = 3
	}
	logger := cfg.Log
	if logger == nil {
		logger = log.Default().WithComponent("bot")
	}
	c := &Connector{
		baseURL:     cfg.BaseURL,
		pollTimeout: cfg.PollTimeout,
		botType:     cfg.BotType,
		msgChan:     make(chan NormalizedMessage, 1000),
		client: &http.Client{
			Timeout: time.Duration(cfg.PollTimeout+10) * time.Second,
		},
		accounts: make([]*AccountInfo, 0),
		log:      logger.WithComponent("bot"),
	}
	c.qrManager = NewQRCodeManager(c)
	return c
}

func (c *Connector) QRManager() *QRCodeManager {
	return c.qrManager
}

// Messages 返回消息通道
func (c *Connector) Messages() <-chan NormalizedMessage {
	return c.msgChan
}

func (c *Connector) IsRunning() bool {
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()
	return len(c.accounts) > 0
}

// GetCredentials 返回第一个可用账号的凭证
func (c *Connector) GetCredentials() *Credentials {
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()
	if len(c.accounts) > 0 && c.accounts[0].Credentials != nil {
		return c.accounts[0].Credentials
	}
	return nil
}

func (c *Connector) SetBaseURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = url
}

// GetAccountTokenByVirtualID 根据虚拟 Bot ID 获取真实账号的 token
func (c *Connector) GetAccountTokenByVirtualID(virtualAccountID string) string {
	c.accountMu.RLock()
	defer c.accountMu.RUnlock()

	// 去掉 "gw_" 前缀后匹配真实 accountID（格式约定）
	// 例如：虚拟 Bot ID 为 "gw_user1_device1"，映射到真实账号 "user1"
	realID := virtualAccountID
	if len(virtualAccountID) > 3 && virtualAccountID[:3] == "gw_" {
		parts := strings.SplitN(virtualAccountID, "_", 3)
		if len(parts) >= 2 {
			realID = parts[1]
		}
	}
	for _, a := range c.accounts {
		if a.Credentials != nil && a.Credentials.Token != "" {
			if a.Credentials.AccountID == realID || a.Credentials.UserID == realID {
				return a.Credentials.Token
			}
		}
	}
	return ""
}

// ── context_token 缓存 ──

func (c *Connector) SetContextToken(accountID, userID, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.contextTokens == nil {
		c.contextTokens = make(map[string]string)
	}
	c.contextTokens[accountID+":"+userID] = token
}

func (c *Connector) GetContextToken(accountID, userID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.contextTokens == nil {
		return ""
	}
	return c.contextTokens[accountID+":"+userID]
}

// ── 轮询游标存储 ──

func (c *Connector) SetSyncBufStore(store SyncBufStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncBufStore = store
}

// ── 辅助方法 ──

func randomUIN() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.StdEncoding.EncodeToString(buf)
}
