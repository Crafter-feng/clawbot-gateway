# ClawBot Gateway 设计文档

## 项目概述

ClawBot Gateway 是一个微信多后端代理网关。通过 iLink 协议接入微信，作为唯一的 iLink 轮询者管理微信连接，同时对外提供 iLink 兼容 API，让外部服务（Hermes Agent、OpenClaw、OpenCode 等）无缝接入。

**核心能力**：
- 作为 iLink 客户端连接真实微信
- 作为 iLink 服务端对外提供 API
- 消息路由到多个 AI 后端
- 文件中转到 Obsidian 等外部服务

## 核心概念

### iLink 是协议，Adapter 是配置

**关键理解**：
- **iLink** 是微信的通信协议，定义了消息格式和 API 端点
- **Adapter** 是配置单元，定义了外部服务如何连接到 Gateway
- Gateway 为每个 Adapter **内部生成** 连接配置（account_id、user_id、base_url），用户复制配置到外部服务

```
用户在管理页面添加 Adapter（如 "Hermes Agent"）
    ↓
Gateway 内部生成：
  - account_id: gw_a1b2c3d4
  - user_id: gw_a1b2c3d4@im.wechat
  - base_url: http://localhost:8080
    ↓
用户复制配置到 HermesClaw 的 .env
    ↓
HermesClaw 通过 iLink 协议连接到 Gateway
```

### Adapter 类型

| 类型 | 说明 | 是否需要 iLink 连接 |
|------|------|-------------------|
| `ilink_proxy` | 外部 iLink 服务（Hermes、OpenClaw） | 是（虚拟 Bot） |
| `openai_compatible` | OpenAI 兼容 API（Claude、DeepSeek） | 否（内置处理） |
| `echo` | 调试回显 | 否 |

### 虚拟 Bot

每个 `ilink_proxy` 类型的 Adapter 都有一个**虚拟 Bot**：
- 不需要 QR 登录
- 不需要真实 bot_token
- 使用 Gateway 生成的虚拟凭证
- 通过 iLink 协议与 Gateway 通信

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    ClawBot Gateway (端口 8080)                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─── 对外服务层 ──────────────────────────────────────────┐ │
│  │  /ilink/bot/*      iLink 协议端点（虚拟 Bot 接入）       │ │
│  │  /api/v1/*         管理 API（需鉴权）                    │ │
│  │  /ws               WebSocket                             │ │
│  │  /                 Web UI                                │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌─── 核心层 ─────────────────────────────────────────────┐  │
│  │  Connector         iLink 协议对接 + 多账号轮询           │  │
│  │  ClientRegistry    虚拟 Bot 注册管理                     │  │
│  │  MessagePipeline   消息处理管道（命令→路由→后端→回复）    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─── 适配器层（配置中心）─────────────────────────────────┐  │
│  │  ilink_proxy       外部 iLink 服务（生成虚拟 Bot 配置）  │  │
│  │  OpenAI Compatible  OpenAI 兼容 API（Claude/DeepSeek等）│  │
│  │  Webhook            单向通知推送                         │  │
│  │  Echo               调试回显                             │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─── 路由层 ─────────────────────────────────────────────┐  │
│  │  会话级后端覆写（/use 设置）                             │  │
│  │  默认后端兜底                                           │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─── 基础设施层 ─────────────────────────────────────────┐  │
│  │  SessionContext    用户会话上下文管理                    │  │
│  │  Store             持久化存储（路由规则/用户设置/API Token）│ │
│  │  RelayManager      文件中转（Local/Obsidian）            │  │
│  │  Logger            结构化日志（log/slog）                │  │
│  └────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 数据流转图

#### 完整数据流

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          数据流转全景图                                   │
└─────────────────────────────────────────────────────────────────────────┘

                          ┌──────────────────┐
                          │    微信用户       │
                          └────────┬─────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │     iLink API (腾讯)         │
                    │  ilinkai.weixin.qq.com       │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │      Connector (轮询)        │
                    │  accountPollLoop()           │
                    └──────────────┬──────────────┘
                                   │
                          NormalizedMessage
                                   │
                    ┌──────────────▼──────────────┐
                    │      消息分发点              │
                    └──────────────┬──────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                    │
              ▼                    ▼                    ▼
    ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
    │  Path A:        │  │  Path B:        │  │  Path C:        │
    │  内置 AI 处理    │  │  虚拟 Bot 代理   │  │  文件中转       │
    └────────┬────────┘  └────────┬────────┘  └────────┬────────┘
             │                    │                    │
             ▼                    ▼                    ▼
    ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
    │ MessagePipeline │  │ ClientRegistry  │  │ RelayManager    │
    │ → CommandProc   │  │ → Broadcast()   │  │ → Forward()     │
    │ → Router        │  │ → Queue.Enqueue │  │                 │
    └────────┬────────┘  └────────┬────────┘  └────────┬────────┘
             │                    │                    │
             ▼                    ▼                    ▼
    ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
    │ Adapter         │  │ VirtualBot.Queue│  │ Obsidian/Local  │
    │ (OpenAI/Webhook)│  │ (每 Bot 独立)   │  │ (文件保存)      │
    └────────┬────────┘  └────────┬────────┘  └─────────────────┘
             │                    │
             ▼                    ▼
    ┌─────────────────┐  ┌─────────────────┐
    │ AI API 响应      │  │ 外部服务        │
    │ (Claude/DeepSeek)│  │ (Hermes/OpenClaw)│
    └────────┬────────┘  └────────┬────────┘
             │                    │
             └──────────┬─────────┘
                        │
             ┌──────────▼──────────┐
             │  Connector.Send()   │
             │  转发到真实 iLink    │
             └──────────┬──────────┘
                        │
             ┌──────────▼──────────┐
             │  iLink API (腾讯)   │
             └──────────┬──────────┘
                        │
             ┌──────────▼──────────┐
             │    微信用户          │
             └─────────────────────┘
```

#### 两条主要路径

**路径 A：内置 AI 处理（BackendAdapter）**
```
微信消息 → Connector → Pipeline → Router → Adapter.Handle() → AI API → 回复
```

**路径 B：虚拟 Bot 代理（ConnectionAdapter）**
```
微信消息 → Connector → ClientRegistry.Broadcast() → VirtualBot.Queue
    → 外部服务 getupdates → 外部服务处理 → sendmessage → 回复
```

#### 关键数据结构流转

```
┌─────────────────────────────────────────────────────────────────┐
│  RawMessageItem (iLink 原始格式)                                │
│  ├── message_id                                                 │
│  ├── from_user_id                                               │
│  ├── to_user_id                                                 │
│  ├── msg_type                                                   │
│  ├── item_list[]                                                │
│  │   ├── type: 1 (text) / 3 (voice) / 4 (file) / 5 (video)    │
│  │   ├── text_item.text                                         │
│  │   ├── voice_item.text (转录)                                 │
│  │   └── image_item.media / file_item.media / video_item.media  │
│  └── context_token                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                    normalize()
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  NormalizedMessage (内部标准格式)                                │
│  ├── msg_id                                                     │
│  ├── from_user                                                  │
│  ├── to_user                                                    │
│  ├── account_id                                                 │
│  ├── type                                                       │
│  ├── content (提取的文本)                                        │
│  ├── items[] (原始消息项)                                        │
│  ├── timestamp                                                  │
│  └── context_token                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│ ChatRequest     │ │ iLink Message   │ │ FileMessage     │
│ (Adapter 输入)  │ │ (虚拟 Bot 输出) │ │ (中转输入)      │
├─────────────────┤ ├─────────────────┤ ├─────────────────┤
│ message: string │ │ from_user_id    │ │ from_user       │
│ user_id: string │ │ to_user_id      │ │ file_name       │
│ session_id      │ │ item_list[]     │ │ file_type       │
│ backend_id      │ │ context_token   │ │ file_data       │
│ history[]       │ │                 │ │ file_url        │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

### Adapter 配置流程

```
1. 用户在管理页面添加 Adapter
   ┌─────────────────────────────────────┐
   │  添加后端                            │
   │  ├─ 类型: ilink_proxy               │
   │  ├─ 名称: Hermes Agent              │
   │  └─ [保存]                           │
   └─────────────────────────────────────┘

2. Gateway 内部生成虚拟 Bot 配置
   ┌─────────────────────────────────────┐
   │  Hermes Agent (iLink Proxy)          │
   ├─────────────────────────────────────┤
   │  account_id: gw_a1b2c3d4           │
   │  user_id: gw_a1b2c3d4@im.wechat    │
   │  base_url: http://localhost:8080    │
   ├─────────────────────────────────────┤
   │  [复制配置] [复制 ID]                │
   └─────────────────────────────────────┘

3. 用户复制配置到外部服务
   # HermesClaw .env
   ILINK_BASE_URL=http://localhost:8080
   ILINK_TOKEN=gw_a1b2c3d4

4. 外部服务通过 iLink 协议连接到 Gateway
   外部服务 → POST /ilink/bot/getupdates → Gateway → 微信
```

### iLink 协议端点

Gateway 对外提供 iLink 兼容 API，供虚拟 Bot 连接：

| 端点 | 方法 | 说明 | 转发到真实 iLink |
|------|------|------|-----------------|
| `/ilink/bot/getupdates` | POST | 长轮询获取消息 | 否（从队列取） |
| `/ilink/bot/sendmessage` | POST | 发送消息 | 是 |
| `/ilink/bot/sendtyping` | POST | 输入状态 | 是 |
| `/ilink/bot/getconfig` | POST | 获取配置 | 是 |
| `/ilink/bot/get_bot_qrcode` | GET | 获取登录二维码 | 是 |
| `/ilink/bot/get_qrcode_status` | GET | 检查二维码状态 | 是 |
| `/ilink/bot/getuploadurl` | POST | 获取上传 URL | 是 |

### 消息队列系统

每个虚拟 Bot（Adapter）拥有独立的消息队列：

```go
type VirtualBot struct {
    AccountID  string         // 虚拟 Bot ID（如 gw_a1b2c3d4）
    Queue      *MessageQueue  // 独立消息队列
    UpdateBuf  string         // get_updates 游标
    LastActive time.Time
}

type ClientRegistry struct {
    mu      sync.RWMutex
    bots    map[string]*VirtualBot  // accountID → VirtualBot
}
```

### 消息流

**入站（微信 → 虚拟 Bot → 外部服务）**：
```
微信消息 → Connector.PollLoop() → NormalizedMessage
    → ClientRegistry.Broadcast(msg)
    → 每个 VirtualBot.Queue.Enqueue(msg)
    → 外部服务 getupdates 从自己队列取消息
    → 外部服务自己处理 AI 逻辑
```

**出站（外部服务 → 微信）**：
```
外部服务调用 sendmessage
    → Gateway 验证虚拟 Bot 凭证
    → Connector.SendTextWithCreds()
    → 转发到真实 iLink API
    → 消息送达微信用户
```

### 认证方式

虚拟 Bot 通过 `Authorization` header 认证：

```
Authorization: Bearer <virtual_bot_token>
AuthorizationType: ilink_bot_token
```

Gateway 验证 token 是否匹配某个虚拟 Bot 的 account_id。

### iLink 服务器实现细节

#### 当前实现（internal/ilink/）

```go
// server.go - 路由注册
func (s *Server) RegisterRoutes(r *gin.Engine) {
    ilink := r.Group("/ilink/bot")
    ilink.Use(s.rateLimitMiddleware())
    ilink.Use(maxBodySizeMiddleware(MaxRequestBodySize))
    {
        ilink.POST("/getupdates", s.handleGetUpdates)
        ilink.POST("/sendmessage", s.handleSendMessage)
        ilink.POST("/sendtyping", s.handleSendTyping)
        ilink.POST("/getconfig", s.handleGetConfig)
        ilink.POST("/getuploadurl", s.handleGetUploadURL)
    }
}
```

#### 当前限制

1. **透明代理模式**：所有 handler 直接代理到真实 iLink API，未使用 `ClientRegistry` 的消息队列
2. **自引用 URL 循环**：虚拟 Bot 的 `BaseURL` 是 `http://localhost:8080`，代理到自身会死循环
3. **请求体丢失**：`forwardToILink()` 创建请求时未附加请求体

#### 需要修复

1. `forwardToILink()` 需要正确附加请求体
2. `handleGetUpdates` 需要从 `ClientRegistry` 的消息队列取消息
3. 虚拟 Bot 的 `BaseURL` 需要指向真实 iLink API 而非 Gateway 自身

### 消息队列系统

每个虚拟 Bot 拥有独立的消息队列，支持持久化和重试：

```go
type MessageQueue struct {
    mu           sync.Mutex
    messages     []*PersistedMessage
    event        chan struct{}
    cap          int
    persistDir   string
    retryConfig  RetryConfig
}

type PersistedMessage struct {
    ID        string
    Message   NormalizedMessage
    Status    string  // "pending" | "sent" | "failed"
    CreatedAt time.Time
    RetryCount int
}

type VirtualBot struct {
    AccountID  string
    UserID     string
    BaseURL    string
    Queue      *MessageQueue
    UpdateBuf  string
    LastActive time.Time
}

type ClientRegistry struct {
    mu      sync.RWMutex
    bots    map[string]*VirtualBot
}
```

#### 消息广播机制

Connector 收到微信消息后，同时通知：
1. **MessagePipeline**（AI 处理模式）— 通过 `msgChan` channel
2. **ClientRegistry**（纯代理模式）— 通过 `Broadcast()` 方法

```go
// bot/account.go - accountPollLoop
func (c *Connector) accountPollLoop(ctx context.Context, creds *Credentials) {
    // ... 轮询逻辑 ...
    for _, raw := range resp.Msgs {
        msg := normalize(raw)
        msg.AccountID = creds.AccountID

        // 1. 发送到 Pipeline（内置 AI 处理）
        select {
        case c.msgChan <- msg:
        default:
            c.log.Warn("msg channel full, dropping msg")
        }

        // 2. 广播到所有虚拟 Bot（代理模式）
        if b := c.GetBroadcaster(); b != nil {
            b.Broadcast(msg)
        }
    }
}
```

#### 消息流

**入站（微信 → 外部服务 / AI 处理）**：
```
微信消息 → Connector.PollLoop() → NormalizedMessage
    ├──→ msgChan → MessagePipeline → Adapter → 回复微信
    └──→ ClientRegistry.Broadcast()
         → 每个 VirtualBot.Queue.Enqueue()
         → 外部服务 getupdates 从自己队列取消息
```

**出站（外部服务 → 微信）**：
```
外部服务调用 sendmessage
    → iLink API Server 验证请求
    → Connector.SendTextWithCreds()
    → 转发到真实 iLink API
    → 消息送达微信用户
```

#### 认证方式

```
Authorization: Bearer <bot_token>
AuthorizationType: ilink_bot_token
```

- **真实 token**：微信 iLink 返回的 bot_token
- **虚拟 token**：管理页面生成的虚拟 Bot 使用 gateway 生成的 token

## 后端适配器（Provider）

### 适配器接口

```go
// BackendAdapter 后端适配器接口（处理消息）
type BackendAdapter interface {
    ID() string
    Name() string
    Type() string
    Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error
    HealthCheck(ctx context.Context) bool
}

// ConnectionAdapter 连接适配器接口（提供外部服务连接配置）
type ConnectionAdapter interface {
    ID() string
    Name() string
    Type() string
    GetConnectionInfo() *ConnectionInfo  // 返回虚拟 Bot 配置
    HealthCheck(ctx context.Context) bool
}

type ConnectionInfo struct {
    AccountID string  // 虚拟 Bot ID（如 gw_a1b2c3d4）
    UserID    string  // 用户 ID（如 gw_a1b2c3d4@im.wechat）
    BaseURL   string  // 连接地址（如 http://localhost:8080）
}
```

### 两种适配器类型

| 类别 | 接口 | 用途 | 示例 |
|------|------|------|------|
| **BackendAdapter** | Handle/HandleStream | 处理消息，返回 AI 响应 | openai_compatible, webhook, echo |
| **ConnectionAdapter** | GetConnectionInfo | 提供外部服务连接配置 | ilink_proxy |

### Provider 类型一览

| 类型 | 类别 | 协议 | 流式支持 | 用途 | 配置项 |
|------|------|------|---------|------|--------|
| `ilink_proxy` | Connection | iLink | - | 外部 iLink 服务（Hermes、OpenClaw） | `name` |
| `openai_compatible` | Backend | HTTP (SSE) | 支持 | AI 对话（Claude、DeepSeek、GPT 等） | `api_key`, `base_url`, `model` |
| `webhook` | Backend | HTTP POST | 不支持 | 单向通知推送 | `url`, `headers` |
| `echo` | Backend | 本地 | 支持 | 调试回显 | 无 |

### ilink_proxy 适配器

提供外部 iLink 服务的连接配置。Gateway 内部生成虚拟 Bot 凭证，用户复制到外部服务。

**配置示例**：
```yaml
- id: hermes
  name: "Hermes Agent"
  type: ilink_proxy
  enabled: true
  config: {}

- id: openclaw
  name: "OpenClaw Gateway"
  type: ilink_proxy
  enabled: true
  config: {}
```

**生成的连接配置**（管理页面显示）：
```
┌─────────────────────────────────────┐
│  Hermes Agent (iLink Proxy)          │
├─────────────────────────────────────┤
│  account_id: gw_a1b2c3d4           │
│  user_id: gw_a1b2c3d4@im.wechat    │
│  base_url: http://localhost:8080    │
├─────────────────────────────────────┤
│  [复制配置] [复制 ID]                │
└─────────────────────────────────────┘
```

**用户复制到外部服务**：
```bash
# HermesClaw .env
ILINK_BASE_URL=http://localhost:8080
ILINK_TOKEN=gw_a1b2c3d4
```

**工作原理**：
1. 用户在管理页面添加 `ilink_proxy` 类型的 Adapter
2. Gateway 生成虚拟 Bot 凭证（account_id、user_id）
3. 虚拟 Bot 注册到 `ClientRegistry`，创建独立消息队列
4. 外部服务通过 iLink 协议连接到 Gateway
5. 微信消息通过虚拟 Bot 的队列转发到外部服务

### ilink_proxy 适配器实现

```go
// ilink_proxy.go
package adapter

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"
)

// ILinkProxyAdapter iLink 代理适配器（虚拟 Bot）
type ILinkProxyAdapter struct {
    id           string
    name         string
    accountID    string  // 虚拟 Bot ID（如 gw_a1b2c3d4）
    userID       string  // 用户 ID（如 gw_a1b2c3d4@im.wechat）
    baseURL      string  // Gateway 地址
    createdAt    time.Time
}

func NewILinkProxyAdapter(id, name, baseURL string) *ILinkProxyAdapter {
    // 生成确定性的虚拟 Bot ID
    accountID := "gw_" + generateID()
    userID := accountID + "@im.wechat"

    return &ILinkProxyAdapter{
        id:        id,
        name:      name,
        accountID: accountID,
        userID:    userID,
        baseURL:   baseURL,
        createdAt: time.Now(),
    }
}

func (a *ILinkProxyAdapter) ID() string   { return a.id }
func (a *ILinkProxyAdapter) Name() string { return a.name }
func (a *ILinkProxyAdapter) Type() string { return "ilink_proxy" }

// GetConnectionInfo 返回虚拟 Bot 连接配置
func (a *ILinkProxyAdapter) GetConnectionInfo() *ConnectionInfo {
    return &ConnectionInfo{
        AccountID: a.accountID,
        UserID:    a.userID,
        BaseURL:   a.baseURL,
    }
}

func (a *ILinkProxyAdapter) HealthCheck(ctx context.Context) bool {
    return true  // 虚拟 Bot 始终健康
}

func generateID() string {
    b := make([]byte, 4)
    rand.Read(b)
    return hex.EncodeToString(b)
}
```

### openai_compatible 适配器

调用 OpenAI 兼容 API（`/chat/completions`），支持非流式和 SSE 流式。

**配置示例**：
```yaml
- id: claude
  name: "Claude 3.5 Sonnet"
  type: openai_compatible
  enabled: true
  config:
    api_key: "${ANTHROPIC_KEY}"
    base_url: "https://api.anthropic.com/v1"
    model: "claude-3-5-sonnet-latest"

- id: deepseek
  name: "DeepSeek V3"
  type: openai_compatible
  enabled: true
  config:
    api_key: "${DEEPSEEK_KEY}"
    base_url: "https://api.deepseek.com/v1"
    model: "deepseek-chat"

- id: openclaw
  name: "OpenClaw Agent"
  type: openai_compatible
  enabled: true
  config:
    api_key: "${OPENCAW_KEY}"
    base_url: "http://localhost:18789/v1"
    model: "openclaw/default"
```

**流式处理**：通过 SSE（Server-Sent Events）逐块接收响应，实时转发到微信。

### webhook 适配器

单向推送消息到指定 URL，不等待回复。

**配置示例**：
```yaml
- id: notifier
  name: "企业微信通知"
  type: webhook
  enabled: true
  config:
    url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
    headers:
      Content-Type: "application/json"
```

**消息格式**：
```json
{
    "from_user": "wxid_xxx",
    "content": "消息内容",
    "type": "text",
    "time": "2025-01-01T00:00:00Z",
    "metadata": {}
}
```

### echo 适配器

调试用，原样返回输入内容。始终健康。

**配置示例**：
```yaml
- id: echo
  name: "Echo Debug"
  type: echo
  enabled: true
  config: {}
```

## 命令系统

### 命令优先级

命令解析始终优先于路由，用户输入以 `/` 开头时优先匹配命令。

### 一级命令

| 命令 | 说明 |
|------|------|
| `/use` | 显示当前状态（后端、会话数） |
| `/use <后端ID>` | 持久切换到指定后端 |
| `/backends` | 列出所有可用后端（含健康状态） |
| `/help` | 显示帮助信息 |

### 二级命令（动态生成）

根据配置的 providers 自动生成，格式为 `/<backend_id>`：

```
/hermes      → 一次性转发到 hermes 后端
/openclaw    → 一次性转发到 openclaw 后端
/deepseek    → 一次性转发到 deepseek 后端
```

**注意**：二级命令是**一次性**的，不改变用户的持久后端选择。

## 路由引擎

### 两层优先级

1. **用户会话级覆写**（`/use` 设置）— 最高优先级
2. **默认后端兜底** — 未设置时使用配置的 `default_backend`

### 路由决策

```go
func (r *Router) Route(message string, userID string) RouteDecision {
    // 1. 检查用户覆写
    if backendID, ok := r.userBackends[userID]; ok {
        return RouteDecision{BackendID: backendID, MatchedBy: "session"}
    }
    // 2. 返回空（由 pipeline 决定是否使用默认后端）
    return RouteDecision{BackendID: "", MatchedBy: "none"}
}
```

### 多后端路由模式

支持 `single`、`both`、`three` 三种模式，允许同时向多个后端发送消息。

## 文件中转系统

### 中转接口

```go
type FileRelay interface {
    Forward(ctx context.Context, file *FileMessage) error
    Name() string
}
```

### 中转适配器

| 适配器 | 说明 |
|--------|------|
| `LocalRelay` | 本地文件系统保存，按日期分目录 |
| `ObsidianRelay` | 保存到 Obsidian Vault，自动生成 Markdown 笔记 |

### 文件消息格式

```go
type FileMessage struct {
    FromUser  string
    FileName  string
    FileType  string    // "image", "file", "audio", "video"
    FileData  []byte
    FileURL   string
    MimeType  string
    Size      int64
    Timestamp time.Time
    Metadata  map[string]string
}
```

## 会话管理

### SessionContext

维护每个用户与每个后端的对话历史：

```go
type SessionContext struct {
    UserID     string
    BackendID  string
    History    []ChatMessage   // 对话历史
    MaxHistory int             // 最大历史条数
    CreatedAt  time.Time
    LastActive time.Time
}
```

### ContextManager

管理所有会话上下文，支持过期清理：

- **get_context**：获取或创建用户会话
- **switch_backend**：切换后端时的上下文策略（keep/clear/isolated）
- **cleanup_expired**：定期清理过期会话

## 多账号管理

### 账号存储

每个账号独立存储在 `data/accounts/` 目录下：

```
data/accounts/
├── account_abc123.json
├── account_def456.json
└── ...
```

### 账号凭证

```go
type Credentials struct {
    Token       string   // iLink bot_token
    BaseURL     string   // iLink API 地址
    AccountID   string   // 账号 ID（iLink bot_id）
    UserID      string   // 用户 ID（iLink user_id）
    AccountName string   // 显示名称
    LoginAt     int64    // 登录时间戳
}
```

### 多账号轮询

每个账号启动独立的轮询 goroutine，互不干扰：

```go
func (c *Connector) accountPollLoop(ctx context.Context, creds *Credentials) {
    // 每个账号独立的 long-poll 循环
    // 独立的 sync_buf 持久化
    // 独立的错误处理和重试
}
```

## 安全设计

- **登录密码**：通过环境变量 `CLAWBOT_LOGIN_PASSWORD` 设置，支持自动生成
- **JWT 签名**：HMAC-SHA256，密钥自动生成或通过 `CLAWBOT_JWT_SECRET` 设置
- **Token 比较**：使用 `crypto/subtle.ConstantTimeCompare` 防止时序攻击
- **API Token**：独立管理，支持轮换
- **CORS**：可配置允许的来源
- **WebSocket**：需要鉴权
- **iLink API**：需要 bot_token 认证

## 配置

### 环境变量配置（.env）

```bash
# ── 数据库 ──
CLAWBOT_DB_PATH=data/clawbot.db

# ── 服务器 ──
CLAWBOT_HOST=0.0.0.0
CLAWBOT_PORT=8080

# ── 认证 ──
CLAWBOT_LOGIN_PASSWORD=1234
# CLAWBOT_JWT_SECRET=your_jwt_secret_here

# ── 微信 / iLink ──
# WEIXIN_TOKEN=your_weixin_token_here
# WEIXIN_ACCOUNT_ID=your_weixin_account_id_here
# WEIXIN_BASE_URL=https://ilinkai.weixin.qq.com
# WEIXIN_POLL_TIMEOUT=35
# WEIXIN_BOT_TYPE=3

# ── 日志 ──
CLAWBOT_LOG_LEVEL=info
```

### 数据库配置

后端适配器、路由规则、微信账号等配置存储在 SQLite 数据库中，通过管理 API 或 Web UI 进行管理。

**后端适配器类型**：

| 类型 | 配置项 | 说明 |
|------|--------|------|
| `echo` | 无 | 调试回显 |
| `openai_compatible` | `api_key`, `base_url`, `model` | OpenAI 兼容 API |
| `ilink_proxy` | 无（自动生成） | 外部 iLink 服务 |

**配置管理 API**：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/backends` | GET/POST | 后端适配器管理 |
| `/api/v1/routes` | GET/POST/DELETE | 路由规则管理 |
| `/api/v1/accounts` | GET | 微信账号列表 |
| `/api/v1/config` | GET/PUT | 系统配置 |
| `/api/v1/notify/tokens` | GET/POST/DELETE | 通知 Token 管理 |

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.22 |
| HTTP 框架 | Gin |
| WebSocket | gorilla/websocket |
| 配置 | YAML + 环境变量展开 |
| 日志 | log/slog |
| 前端 | React 19 + TypeScript + Vite + Zustand |
| 部署 | Docker |

## 目录结构

```
clawbot-gateway/
├── main.go                      # 入口
├── .env                         # 环境变量配置
├── DESIGN.md                    # 设计文档
├── Dockerfile                   # Docker 构建
├── go.mod / go.sum              # Go 依赖
│
├── internal/
│   ├── adapter/                 # 后端适配器
│   │   ├── adapter.go           # BackendAdapter + ConnectionAdapter 接口
│   │   ├── factory.go           # 适配器工厂（支持两种适配器类型）
│   │   └── ilink_proxy.go       # iLink 代理适配器（虚拟 Bot）
│   │
│   ├── api/                     # HTTP API 服务
│   │   ├── server.go            # API 服务器 + 路由注册 + 中间件
│   │   ├── pipeline.go          # 消息处理管道 + 命令处理器
│   │   ├── jwt.go               # JWT 鉴权
│   │   ├── backend_handler.go   # 后端管理 API
│   │   ├── route_handler.go     # 路由规则 API
│   │   ├── wechat_handler.go    # 微信账号 API
│   │   ├── settings_handler.go  # 配置管理 API
│   │   ├── notify_handler.go    # 通知 Token API
│   │   └── push_handler.go      # 消息推送 API
│   │
│   ├── bot/                     # iLink 协议客户端
│   │   ├── client.go            # Connector 核心结构 + MessageBroadcaster 接口
│   │   ├── message.go           # 消息模型 + 解析
│   │   ├── send.go              # 消息发送
│   │   ├── media.go             # 媒体文件上传
│   │   ├── qrlogin.go           # 扫码登录
│   │   └── account.go           # 多账号管理 + 轮询 + 广播
│   │
│   ├── config/                  # 配置加载
│   │   └── config.go            # 配置结构 + 环境变量加载
│   │
│   ├── database/                # 数据库层
│   │   ├── db.go                # SQLite 数据库初始化
│   │   ├── settings.go          # 配置存储
│   │   ├── backends.go          # 后端存储
│   │   ├── routes.go            # 路由规则存储
│   │   ├── accounts.go          # 微信账号存储
│   │   ├── virtual_bots.go      # 虚拟 Bot 存储
│   │   └── notify.go            # 通知 Token 存储
│   │
│   ├── ilink/                   # iLink API 对外服务
│   │   ├── server.go            # 路由注册 + 限流中间件
│   │   ├── handler.go           # 端点实现（透明代理）
│   │   ├── registry.go          # ClientRegistry + VirtualBot + MessageQueue
│   │   └── ratelimit.go         # 速率限制器
│   │
│   ├── log/                     # 日志
│   │   └── log.go               # slog 封装
│   │
│   ├── relay/                   # 文件中转
│   │   ├── relay.go             # RelayManager + FileRelay 接口
│   │   ├── forwarder.go         # LocalRelay
│   │   └── obsidian.go          # ObsidianRelay
│   │
│   ├── route/                   # 路由引擎
│   │   └── route.go             # Router + 关键词规则
│   │
│   ├── session/                 # 会话管理
│   │   └── context.go           # SessionContext + ContextManager
│   │
│   └── crypto/                  # 加密工具
│       └── crypto.go            # Secret 生成
│
├── web/                         # 前端
│   ├── src/
│   │   ├── pages/               # 页面组件
│   │   │   ├── LoginPage.tsx
│   │   │   ├── DashboardPage.tsx
│   │   │   ├── ChannelsPage.tsx
│   │   │   ├── ManagePage.tsx
│   │   │   ├── SettingsPage.tsx
│   │   │   └── NotificationPage.tsx
│   │   ├── components/          # UI 组件
│   │   │   ├── ui/              # 基础组件库
│   │   │   │   ├── Button.tsx
│   │   │   │   ├── Input.tsx
│   │   │   │   ├── Select.tsx
│   │   │   │   ├── Textarea.tsx
│   │   │   │   ├── Tag.tsx
│   │   │   │   ├── Modal.tsx
│   │   │   │   ├── ConfirmDialog.tsx
│   │   │   │   ├── EmptyState.tsx
│   │   │   │   └── Skeleton.tsx
│   │   │   ├── AppShell.tsx     # 侧边栏布局
│   │   │   ├── LoginForm.tsx    # 登录表单
│   │   │   ├── MetricCard.tsx   # 指标卡片
│   │   │   ├── QrModal.tsx      # 扫码绑定模态框
│   │   │   └── Toast.tsx        # 通知系统
│   │   ├── stores/              # Zustand 状态管理
│   │   │   ├── auth.ts          # 认证状态
│   │   │   ├── accounts.ts      # 微信账号状态
│   │   │   ├── backends.ts      # 后端服务状态
│   │   │   ├── routes.ts        # 路由规则状态
│   │   │   └── stats.ts         # 统计数据状态
│   │   └── api/                 # API 客户端
│   │       └── client.ts        # HTTP 客户端
│   └── app.css                  # 设计系统 + 样式
│
└── data/                        # 运行时数据
    ├── clawbot.db               # SQLite 数据库
    ├── accounts/                # 账号凭证
    └── queues/                  # 消息队列持久化
```

## 代码审查结果

### 审查范围

对 `internal/adapter/`、`internal/ilink/`、`internal/bot/`、`internal/config/` 进行全面审查。

### 实现状态（2026-07-26 更新）

#### 已完成的 P0 架构

| 组件 | 状态 | 说明 |
|------|------|------|
| `ConnectionAdapter` 接口 | ✅ 已实现 | `adapter/adapter.go` |
| `AdapterFactory` 支持 `ConnectionAdapter` | ✅ 已实现 | `adapter/factory.go` |
| `ilink_proxy` 适配器 | ✅ 已实现 | `adapter/ilink_proxy.go` |
| `ClientRegistry` 虚拟 Bot 管理 | ✅ 已实现 | `ilink/registry.go` |
| `MessageQueue` 消息队列 | ✅ 已实现 | 支持持久化和重试 |
| `MessageBroadcaster` 接口 | ✅ 已实现 | `bot/client.go` |
| `accountPollLoop` 广播机制 | ✅ 已实现 | `bot/account.go` |

#### 当前存在的问题

##### CRITICAL - 必须修复

| # | 文件 | 问题 | 说明 |
|---|------|------|------|
| 1 | `ilink/handler.go:64` | `forwardToILink()` 请求体为空 | `body []byte` 参数未附加到请求，所有代理请求体丢失 |
| 2 | `ilink/handler.go` | `handleGetUpdates` 直接代理而非使用队列 | `ClientRegistry` 和 `MessageQueue` 未被任何 handler 使用 |
| 3 | `ilink/handler.go` | 自引用 URL 循环 | 虚拟 Bot 的 `BaseURL` 是 `http://localhost:8080`，代理到自身会死循环 |

##### MODERATE - 需要修复

| # | 文件 | 问题 | 说明 |
|---|------|------|------|
| 4 | `ilink/server.go` | 缺少 QR 端点路由 | `GET /get_bot_qrcode` 和 `GET /get_qrcode_status` 未注册 |
| 5 | `config/config.go` | 无 `ProviderConfig` 结构 | 后端配置从数据库加载，非 YAML |
| 6 | `adapter/` | 缺少 `webhook` 适配器 | 设计文档提到但未实现 |

##### MINOR - 可接受的偏差

| # | 文件 | 说明 |
|---|------|------|
| 7 | `bot/client.go` | 使用 `broadcaster MessageBroadcaster` 接口而非直接引用 `*ilink.ClientRegistry`（架构更优，避免循环依赖） |
| 8 | `ilink/registry.go` | 超出设计：支持消息持久化、重试、统计 |
| 9 | `bot/client.go:69` | `msgChan` 容量仍为 100（设计文档 P2 问题 #12） |

### 重新设计

#### 1. Adapter 层重构

```go
// adapter.go - 新增 ConnectionAdapter 接口

// BackendAdapter 处理消息的后端适配器
type BackendAdapter interface {
    ID() string
    Name() string
    Type() string
    Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error
    HealthCheck(ctx context.Context) bool
}

// ConnectionAdapter 提供外部服务连接配置的适配器
type ConnectionAdapter interface {
    ID() string
    Name() string
    Type() string
    GetConnectionInfo() *ConnectionInfo
    HealthCheck(ctx context.Context) bool
}

type ConnectionInfo struct {
    AccountID string  // 虚拟 Bot ID
    UserID    string  // 用户 ID
    BaseURL   string  // Gateway 地址
}
```

```go
// factory.go - 支持两种适配器

type AdapterFactory struct {
    mu       sync.RWMutex
    backends map[string]BackendAdapter    // 后端适配器
    conns    map[string]ConnectionAdapter // 连接适配器
}

func (f *AdapterFactory) RegisterConnection(adapter ConnectionAdapter) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.conns[adapter.ID()] = adapter
}

func (f *AdapterFactory) GetConnection(id string) (ConnectionAdapter, bool) {
    f.mu.RLock()
    defer f.mu.RUnlock()
    a, ok := f.conns[id]
    return a, ok
}

func (f *AdapterFactory) ListConnections() []ConnectionAdapter {
    f.mu.RLock()
    defer f.mu.RUnlock()
    result := make([]ConnectionAdapter, 0, len(f.conns))
    for _, a := range f.conns {
        result = append(result, a)
    }
    return result
}
```

#### 2. ilink_proxy 适配器

```go
// ilink_proxy.go

type ILinkProxyAdapter struct {
    id        string
    name      string
    accountID string  // 虚拟 Bot ID（如 gw_a1b2c3d4）
    userID    string  // 用户 ID（如 gw_a1b2c3d4@im.wechat）
    baseURL   string  // Gateway 地址
}

func (a *ILinkProxyAdapter) GetConnectionInfo() *ConnectionInfo {
    return &ConnectionInfo{
        AccountID: a.accountID,
        UserID:    a.userID,
        BaseURL:   a.baseURL,
    }
}
```

#### 3. ClientRegistry（虚拟 Bot 管理）

```go
// ilink/registry.go - 新增文件

type VirtualBot struct {
    AccountID  string
    Queue      *MessageQueue
    UpdateBuf  string
    LastActive time.Time
}

type ClientRegistry struct {
    mu   sync.RWMutex
    bots map[string]*VirtualBot
}

func (r *ClientRegistry) Register(accountID string) *VirtualBot {
    r.mu.Lock()
    defer r.mu.Unlock()
    bot := &VirtualBot{
        AccountID: accountID,
        Queue:     NewMessageQueue(200),
    }
    r.bots[accountID] = bot
    return bot
}

func (r *ClientRegistry) Broadcast(msg bot.NormalizedMessage) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, bot := range r.bots {
        bot.Queue.Enqueue(msg)
    }
}
```

#### 4. iLink Server 重构

```go
// ilink/server.go - 重构

type Server struct {
    bot      *bot.Connector
    registry *ClientRegistry
    log      *log.Logger
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
    ilink := r.Group("/ilink/bot")
    {
        ilink.POST("/getupdates", s.handleGetUpdates)      // 改为 POST
        ilink.POST("/sendmessage", s.handleSendMessage)
        ilink.POST("/sendtyping", s.handleSendTyping)
        ilink.POST("/getconfig", s.handleGetConfig)        // 新增
        ilink.GET("/get_bot_qrcode", s.handleGetQRCode)
        ilink.GET("/get_qrcode_status", s.handleGetQRCodeStatus)
        ilink.POST("/getuploadurl", s.handleGetUploadURL)
    }
}
```

#### 5. handleGetUpdates 重构

```go
func (s *Server) handleGetUpdates(c *gin.Context) {
    // 1. 验证虚拟 Bot token
    accountID := s.validateToken(c)
    if accountID == "" {
        c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
        return
    }

    // 2. 获取虚拟 Bot 的消息队列
    bot := s.registry.Get(accountID)
    if bot == nil {
        c.JSON(400, gin.H{"ret": -1, "errmsg": "bot not registered"})
        return
    }

    // 3. 从队列取消息（长轮询）
    timeout := 35 * time.Second
    msgs := bot.Queue.dequeueAll(timeout)

    // 4. 转换为 iLink 格式
    ilinkMsgs := make([]Message, 0, len(msgs))
    for _, msg := range msgs {
        ilinkMsgs = append(ilinkMsgs, s.convertToILinkMessage(msg))
    }

    c.JSON(200, GetUpdatesResponse{
        Ret:           0,
        Msgs:          ilinkMsgs,
        GetUpdatesBuf: bot.UpdateBuf,
    })
}
```

#### 6. handleSendMessage 重构

```go
func (s *Server) handleSendMessage(c *gin.Context) {
    // 1. 验证虚拟 Bot token
    accountID := s.validateToken(c)
    if accountID == "" {
        c.JSON(401, gin.H{"ret": -1, "errmsg": "unauthorized"})
        return
    }

    // 2. 解析请求（支持所有消息类型）
    var req SendMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"ret": -1, "errmsg": "invalid request"})
        return
    }

    // 3. 转发到真实 iLink API
    creds := s.bot.GetAccountCredentials(accountID)
    if creds == nil {
        c.JSON(500, gin.H{"ret": -1, "errmsg": "no credentials"})
        return
    }

    // 4. 发送（支持所有消息类型）
    err := s.bot.SendMessageWithCreds(ctx, creds, req)
    if err != nil {
        c.JSON(500, gin.H{"ret": -1, "errmsg": err.Error()})
        return
    }

    c.JSON(200, gin.H{"ret": 0, "message_id": 0})
}
```

#### 7. Connector 广播机制

```go
// bot/client.go - 新增

type Connector struct {
    // ... 现有字段 ...
    clientRegistry *ilink.ClientRegistry  // 新增
}

// SetClientRegistry 注入虚拟 Bot 管理器
func (c *Connector) SetClientRegistry(registry *ilink.ClientRegistry) {
    c.clientRegistry = registry
}
```

```go
// bot/account.go - 修改 accountPollLoop

func (c *Connector) accountPollLoop(ctx context.Context, creds *Credentials) {
    // ... 现有轮询逻辑 ...
    for _, raw := range resp.Msgs {
        msg := normalize(raw)
        msg.AccountID = creds.AccountID

        // 1. 发送到 Pipeline（内置 AI 处理）
        select {
        case c.msgChan <- msg:
        default:
            c.log.Warn("msg channel full")
        }

        // 2. 广播到所有虚拟 Bot（代理模式）
        if c.clientRegistry != nil {
            c.clientRegistry.Broadcast(msg)
        }
    }
}
```

## 实现计划

### 需要修改/新增的文件

| 文件 | 变更 | 优先级 |
|------|------|--------|
| `internal/adapter/adapter.go` | 添加 `ConnectionAdapter` 接口和 `ConnectionInfo` 结构 | P0 |
| `internal/adapter/ilink_proxy.go` | **新增** iLink 代理适配器实现 | P0 |
| `internal/adapter/factory.go` | 添加 `RegisterConnection`/`GetConnection`/`ListConnections` | P0 |
| `internal/ilink/registry.go` | **新增** `ClientRegistry` + `VirtualBot` + `MessageQueue` | P0 |
| `internal/ilink/server.go` | 注入 `ClientRegistry`，添加 `getconfig` 端点 | P0 |
| `internal/ilink/handler.go` | 重写所有 handler（队列、认证、全类型支持） | P0 |
| `internal/bot/client.go` | 添加 `clientRegistry` 字段和 `SetClientRegistry` 方法 | P0 |
| `internal/bot/account.go` | `accountPollLoop` 添加 `clientRegistry.Broadcast()` 调用 | P0 |
| `internal/config/config.go` | `ProviderConfig` 支持 `ilink_proxy` 类型 | P1 |
| `internal/api/server.go` | 新增 `GET /api/v1/adapters/connections` 端点 | P1 |
| `web/src/pages/ManagePage.tsx` | 显示虚拟 Bot 连接配置卡片 | P2 |

### 实现优先级

1. **P0 - ConnectionAdapter 接口**：定义连接适配器接口
2. **P0 - ilink_proxy 适配器**：实现虚拟 Bot 配置生成
3. **P0 - ClientRegistry**：虚拟 Bot 注册 + 消息队列 + 广播
4. **P0 - iLink Server 重构**：认证、队列、全类型支持
5. **P0 - Connector 广播**：accountPollLoop 添加广播调用
6. **P1 - 配置支持**：config.yaml 支持 ilink_proxy 类型
7. **P1 - 管理 API**：新增连接信息查询端点
8. **P2 - 管理页面**：显示虚拟 Bot 连接配置卡片

## 参考项目

- [HermesClaw](https://github.com/AaronWong1999/hermesclaw) — iLink 代理架构参考
- [WeClawBot-API](https://github.com/Cp0204/WeClawBot-API) — iLink API 实现参考
- [Hermes Agent](https://github.com/NousResearch/hermes-agent) — 外部服务接入参考
- [OpenClaw](https://github.com/openclaw/openclaw) — 外部服务接入参考
