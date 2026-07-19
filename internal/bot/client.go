package bot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"sync"
	"time"

	"clawbot-gateway/internal/log"
)

// ── ClawBot Connector ──

type Connector struct {
	mu            sync.RWMutex
	credentials   *Credentials
	token         string
	baseURL       string
	pollTimeout   int
	updateBuf     string
	running       bool
	cancel        context.CancelFunc
	msgChan       chan NormalizedMessage
	client        *http.Client
	accounts      []*AccountInfo
	accountMu     sync.RWMutex
	dataDir       string
	qrManager     *QRCodeManager
	botType       int
	contextTokens map[string]string // accountID:userID → context_token
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
	Token       string         // 可选：启动时自动注册的 iLink token
	BotType     int            // iLink bot_type 参数（默认 3）
	DataDir     string         // 持久化目录（同步缓冲区、上下文令牌等），空=不持久化
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
	dataDir := cfg.DataDir
	if dataDir != "" {
		os.MkdirAll(dataDir, 0700)
	}
	c := &Connector{
		baseURL:     cfg.BaseURL,
		pollTimeout: cfg.PollTimeout,
		botType:     cfg.BotType,
		dataDir:     dataDir,
		msgChan:     make(chan NormalizedMessage, 100),
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

func (c *Connector) GetCredentials() *Credentials {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.credentials
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

// ── 辅助方法 ──

func randomUIN() string {
	buf := make([]byte, 4)
	rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}
