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
| `webhook` | 单向通知推送 | 否 |
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
    ilink.POST("/sendmessage", s.handleSendMessage)
    ilink.POST("/sendtyping", s.handleSendTyping)
    ilink.GET("/getupdates", s.handleGetUpdates)
    ilink.GET("/get_bot_qrcode", s.handleGetQRCode)
    ilink.GET("/get_qrcode_status", s.handleGetQRCodeStatus)
    ilink.POST("/getuploadurl", s.handleGetUploadURL)
}
```

#### 当前限制

1. **单消息通道**：`handleGetUpdates` 从 `s.bot.Messages()` 读取，这是全局共享的 channel，多个外部服务会竞争消费
2. **无消息队列**：没有为每个外部客户端维护独立的消息队列
3. **sendmessage 只支持文本**：`SendMessageRequest` 的 `ItemList` 只定义了 `TextItem`
4. **无 client_id 跟踪**：无法区分不同的外部客户端

#### 需要改进

1. 添加 `ClientRegistry` 管理外部客户端
2. 为每个客户端维护独立的 `MessageQueue`
3. `handleGetUpdates` 从客户端自己的队列取消息
4. `handleSendMessage` 支持所有消息类型并转发到真实 iLink

### 消息队列系统

每个外部客户端拥有独立的消息队列：

```go
type MessageQueue struct {
    mu    sync.Mutex
    msgs  []Message      // 缓存的消息
    event chan struct{}   // 通知有新消息
    cap   int            // 队列容量（默认 200）
}

type ClientSession struct {
    AccountID  string         // 客户端标识
    Queue      *MessageQueue  // 独立消息队列
    UpdateBuf  string         // get_updates 游标
    LastActive time.Time
}

type ClientRegistry struct {
    mu      sync.RWMutex
    clients map[string]*ClientSession
}
```

#### 消息广播机制

Connector 收到微信消息后，需要同时通知：
1. **MessagePipeline**（AI 处理模式）— 通过 `msgChan` channel
2. **ClientRegistry**（纯代理模式）— 通过 `Broadcast()` 方法

```go
// Connector 收到消息后的分发逻辑
func (c *Connector) accountPollLoop(ctx context.Context, creds *Credentials) {
    // ... 轮询逻辑 ...
    for _, raw := range resp.Msgs {
        msg := normalize(raw)
        msg.AccountID = creds.AccountID

        // 1. 发送到 Pipeline（AI 处理模式）
        select {
        case c.msgChan <- msg:
        default:
            c.log.Warn("msg channel full")
        }

        // 2. 广播到所有外部客户端（纯代理模式）
        c.clientRegistry.Broadcast(msg)
    }
}
```

#### 消息流

**入站（微信 → 外部服务 / AI 处理）**：
```
微信消息 → Connector.PollLoop() → NormalizedMessage
    ├──→ msgChan → MessagePipeline → Adapter → 回复微信
    └──→ ClientRegistry.Broadcast()
         → 每个 ClientSession.Queue.Enqueue()
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

## 配置文件

### config.yaml 完整示例

```yaml
log_level: "info"

server:
  host: "0.0.0.0"
  port: 8080

clawbot:
  base_url: "https://ilinkai.weixin.qq.com"
  poll_timeout: 35
  max_retry_login: 3
  accounts:
    - token: "${BOT1_TOKEN}"
      user_id: "bot1"
      account_id: "wx_bot1"
      account_name: "Bot 1"

api:
  login_password: "${CLAWBOT_LOGIN_PASSWORD}"
  allowed_origins:
    - "*"

backend:
  default_backend: "openclaw"
  providers:
    - id: openclaw
      name: "OpenClaw Agent"
      type: openai_compatible
      enabled: true
      config:
        api_key: "${OPENCAW_KEY}"
        base_url: "http://localhost:18789/v1"
        model: "openclaw/default"

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

    - id: webhook_notify
      name: "Webhook 通知"
      type: webhook
      enabled: true
      config:
        url: "https://example.com/webhook"
        headers:
          Authorization: "Bearer ${WEBHOOK_TOKEN}"

    - id: echo
      name: "Echo Debug"
      type: echo
      enabled: true
      config: {}

context:
  max_history: 20
  switch_strategy: keep
  ttl: 3600
```

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
├── config.yaml                  # 配置文件
├── DESIGN.md                    # 设计文档
├── Dockerfile                   # Docker 构建
├── go.mod / go.sum              # Go 依赖
│
├── internal/
│   ├── adapter/                 # 后端适配器
│   │   ├── adapter.go           # BackendAdapter 接口
│   │   ├── factory.go           # 适配器工厂 + Echo/OpenAI 实现
│   │   └── webhook.go           # Webhook 适配器
│   │
│   ├── api/                     # HTTP API 服务
│   │   ├── server.go            # API 服务器 + 路由注册 + 中间件
│   │   ├── pipeline.go          # 消息处理管道 + 命令处理器
│   │   ├── jwt.go               # JWT 鉴权
│   │   ├── push_handler.go      # 消息推送 API
│   │   └── *_handler.go         # 其他 API handler
│   │
│   ├── bot/                     # iLink 协议客户端
│   │   ├── client.go            # Connector 核心结构
│   │   ├── message.go           # 消息模型 + 解析
│   │   ├── send.go              # 消息发送
│   │   ├── media.go             # 媒体文件上传
│   │   ├── qrlogin.go           # 扫码登录
│   │   └── account.go           # 多账号管理 + 轮询
│   │
│   ├── config/                  # 配置加载
│   │   └── config.go            # YAML 解析 + 环境变量展开
│   │
│   ├── ilink/                   # iLink API 对外服务
│   │   ├── server.go            # 路由注册
│   │   └── handler.go           # 端点实现
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
│   └── store/                   # 持久化存储
│       ├── store.go             # Store（路由/API Token）
│       └── accounts.go          # AccountStore（账号凭证）
│
├── web/                         # 前端
│   ├── src/
│   │   ├── pages/               # 页面组件
│   │   ├── components/          # UI 组件
│   │   ├── stores/              # Zustand 状态管理
│   │   └── api/                 # API 客户端
│   └── dist/                    # 构建产物
│
└── data/                        # 运行时数据
    ├── clawbot.db              # SQLite 数据库
    ├── accounts/                # 账号凭证
    └── syncbuf/                 # 同步缓冲区
```

## 代码审查结果

### 审查范围

对 `internal/adapter/`、`internal/ilink/`、`internal/bot/`、`internal/config/` 进行全面审查。

### 发现的问题

#### P0 - 架构缺失

| # | 文件 | 问题 | 说明 |
|---|------|------|------|
| 1 | `adapter/adapter.go` | 缺少 `ConnectionAdapter` 接口 | 只有 `BackendAdapter`，没有连接适配器接口 |
| 2 | `adapter/factory.go` | 只存储 `BackendAdapter` | `AdapterFactory` 不支持 `ConnectionAdapter` |
| 3 | `adapter/` | 缺少 `ilink_proxy` 适配器 | 设计文档中提到但代码未实现 |
| 4 | `ilink/server.go` | 缺少 `ClientRegistry` | 无法管理虚拟 Bot 和独立消息队列 |
| 5 | `bot/client.go` | 缺少消息广播机制 | `accountPollLoop` 只发到 `msgChan`，不广播到虚拟 Bot |

#### P1 - 功能缺陷

| # | 文件 | 问题 | 说明 |
|---|------|------|------|
| 6 | `ilink/handler.go:184` | `getupdates` 使用 GET 而非 POST | iLink 协议规定 `getupdates` 应为 POST |
| 7 | `ilink/handler.go:191` | `handleGetUpdates` 从全局 channel 读取 | 多个外部服务会竞争消费同一条消息 |
| 8 | `ilink/handler.go:116-121` | `handleSendMessage` 只支持文本 | 硬编码 `item.Type == 1`，不支持图片/文件/视频 |
| 9 | `ilink/handler.go:129` | `handleSendMessage` 用 `ToUserID` 查找凭证 | 应该用 `FromUserID`（虚拟 Bot 的 account_id） |
| 10 | `ilink/handler.go` | 缺少 `getconfig` 端点 | iLink 协议需要此端点获取 typing_ticket |
| 11 | `ilink/handler.go:56` | `Message` 缺少 `Timestamp` 字段 | 外部服务需要消息时间戳 |

#### P2 - 代码质量

| # | 文件 | 问题 | 说明 |
|---|------|------|------|
| 12 | `bot/client.go:70` | `msgChan` 容量只有 100 | 高负载下会丢消息 |
| 13 | `bot/client.go:17-34` | `Connector` 缺少 `clientRegistry` 字段 | 无法注入虚拟 Bot 管理 |
| 14 | `config/config.go` | 不支持 `ilink_proxy` 类型 | `ProviderConfig` 无法区分连接适配器和后端适配器 |
| 15 | `ilink/handler.go:284-290` | `handleGetQRCode` 返回空数据 | 应该调用真实 iLink API 获取二维码 |

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
