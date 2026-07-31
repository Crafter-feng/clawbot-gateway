# ClawBot Gateway 设计文档

## 项目概述

ClawBot Gateway 是一个微信多后端代理网关。通过 iLink 协议接入微信，作为**唯一的 iLink 轮询者**管理微信连接，消息统一经过命令→路由→后端管线处理，同时对外提供 iLink 兼容 API 让外部服务（Hermes Agent、OpenClaw、OpenCode 等）接入。

**核心能力**：
- 作为 iLink 客户端连接真实微信，**独占轮询**
- 作为 iLink 服务端对外提供 API，供外部服务获取**已路由的消息**
- 消息统一经过 Pipeline：命令解析 → 路由决策 → 后端处理 → 回复
- 文件中转到 Obsidian 等外部服务

## 核心概念

### iLink 客户端 vs 服务端

Gateway 同时扮演两个角色，必须清晰区分：

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ClawBot Gateway                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─── iLink 客户端（Connector）─────────────────────────────────┐  │
│  │  作用：连接真实的微信 iLink API                                │  │
│  │  方向：Gateway → ilinkai.weixin.qq.com（腾讯服务器）          │  │
│  │  功能：                                                       │  │
│  │    - 扫码登录获取 bot_token                                   │  │
│  │    - 长轮询接收微信消息                                        │  │
│  │    - 发送消息到微信                                            │  │
│  │    - 管理多账号独立轮询                                        │  │
│  │  代码：internal/bot/                                          │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── iLink 服务端（Server）────────────────────────────────────┐  │
│  │  作用：对外提供 iLink 兼容 API                                 │  │
│  │  方向：外部服务 → Gateway:8080/ilink/bot/*                     │  │
│  │  功能：                                                       │  │
│  │    - getupdates：从 Pipeline 维护的虚拟 Bot 队列消费已路由消息  │  │
│  │    - sendmessage：透明转发到真实 iLink API（回复用户）         │  │
│  │    - sendtyping/getconfig/getuploadurl：透明转发到真实 API    │  │
│  │    - 管理虚拟 Bot 的消息队列（按后端隔离）                     │  │
│  │  代码：internal/ilink/                                        │  │
│  └──────────────────────────────────────────────────────────────┘  │
```

**关键区别**：

| 维度 | iLink 客户端 | iLink 服务端 |
|------|-------------|-------------|
| 代码位置 | `internal/bot/` | `internal/ilink/` |
| 核心结构 | `Connector` | `Server` |
| 连接目标 | 腾讯 iLink API | 外部服务（Hermes、OpenClaw） |
| 主要端点 | `getupdates`, `sendmessage` | `/ilink/bot/*` |
| 认证方式 | `bot_token`（真实微信凭证） | `Bearer <虚拟token>` |
| 消息流向 | 微信 → Gateway | Gateway 队列 → 外部服务 → Gateway → 微信 |
| 消息来源 | 腾讯 iLink API（独占轮询） | Pipeline 维护的虚拟 Bot 消息队列 |
### 消息流转

**核心原则**：iLink 客户端（Connector）是**唯一轮询者**，所有消息统一经过 Pipeline，没有任何消息能绕过Pipeline。

iLink 客户端和 iLink 服务端之间通过**管道**交互，不是各自直连腾讯服务器。

```
                    ┌──────────────────────────────────────────┐
                    │          微信用户                         │
                    └───────────────┬──────────────────────────┘
                                    │
                    ┌───────────────▼──────────────────────────┐
                    │     iLink API (腾讯)                      │
                    │  ilinkai.weixin.qq.com                   │
                    └───────────────┬──────────────────────────┘
                                    │
                                    │  唯一轮询者（独占）
                    ┌───────────────▼──────────────────────────┐
                    │      Connector (轮询)                     │
                    │  accountPollLoop()                       │
                    └───────────────┬──────────────────────────┘
                                    │
                          NormalizedMessage
                                    │
                    ┌───────────────▼──────────────────────────┐
                    │      MessagePipeline                      │
                    │  ┌─────────────────────────────────────┐  │
                    │  │ Command Processor (命令解析)        │  │
                    │  │  /use → 切换后端                     │  │
                    │  │  /backends → 列出后端                │  │
                    │  │  /help → 显示帮助                    │  │
                    │  │  /<id> → 一次性转发到后端            │  │
                    │  │  /help <id> [args] → 代理后端命令    │  │
                    │  └─────────────────────────────────────┘  │
                    │                    │                       │
                    │  ┌─────────────────▼───────────────────┐  │
                    │  │ Router (路由决策)                    │  │
                    │  │  1. 用户会话级覆写 (/use)            │  │
                    │  │  2. 关键词规则匹配                   │  │
                    │  │  3. 默认后端兜底                     │  │
                    │  └─────────────────┬───────────────────┘  │
                    │                    │                       │
                    │         ┌──────────┼──────────┐            │
                    │         ▼                     ▼          │
                    │  ┌──────────────┐    ┌──────────────────┐  │
                    │  │ BackendAdapter│    │ ConnectionAdapter│  │
                    │  │ (echo/openai) │    │ (ilink_proxy)    │  │
                    │  │ → Handle()    │    │ → 消息入队        │  │
                    │  │ → AI API     │    │ → 外部服务消费    │  │
                    │  └──────┬───────┘    └────────┬─────────┘  │
                    └─────────┼─────────────────────┼────────────┘
                              │                     │
                              ▼                     ▼
                    ┌─────────────────┐  ┌─────────────────────┐
                    │  AI API 响应      │  │  iLink 服务端队列     │
                    └────────┬────────┘  └─────────┬───────────┘
                             │                     │
                             │      ┌──────────────┘
                             │      │ 外部服务 getupdates
                             │      ▼
                             │  ┌─────────────┐
                             │  │ 外部服务     │
                             │  │ (Hermes等)   │
                             │  └──────┬──────┘
                             │         │ sendmessage
                             │         ▼
                             │  ┌─────────────────┐
                             │  │ iLink 服务端     │
                             │  │ (透明转发回复)   │
                             │  └────────┬────────┘
                             │           │
                             └─────┬─────┘
                                   │
                    ┌──────────────▼──────────────────────────┐
                    │  Connector.Send()                        │
                    │  转发到真实 iLink API                    │
                    └──────────────┬──────────────────────────┘
                                   │
                    ┌──────────────▼──────────────────────────┐
                    │    微信用户                               │
                    └─────────────────────────────────────────┘
```

#### Pipeline 消息处理流程

**所有消息**统一经过 Pipeline，没有消息能绕过这个管线。

```
Connector 收到消息
    │
    ▼
NormalizedMessage (标准化)
    │
    ▼
┌─── Command Processor ──────────────────────────────────────┐
│  解析以 / 开头的命令                                        │
│                                                            │
│  内部命令（Gateway 处理，不转发）：                         │
│    /use            → 显示状态 / 切换后端
    /use main       → 回到主命令模式                    │
│    /backends       → 列出所有后端                            │
│    /help           → 显示 Gateway 帮助                      │
│                                                            │
│  代理命令（转发到后端处理）：                                │
│    /<id>           → 显示后端状态（无参数时）                │
│    /<id> <msg>     → 剥离前缀，<msg> 转发到 <id> 后端         │
│    /help <id>      → 转发 /help 到 <id> 后端                 │
│    /help <id> args → 转发 /help args 到 <id> 后端            │
└────────────────────┬───────────────────────────────────────┘
                     │ 非命令消息
                     ▼
┌─── Router ────────────────────────────────────────────────┐
│  三层优先级：                                               │
│    1. 用户会话级覆写（/use 设置）— 最高                      │
│    2. 关键词规则匹配 — 按优先级遍历                          │
│    3. 默认后端兜底                                         │
│                                                            │
│  路由结果：backendID                                       │
└────────────────────┬───────────────────────────────────────┘
                     │
         ┌───────────┴───────────┐
         ▼                       ▼
┌─── BackendAdapter ───┐  ┌─── ConnectionAdapter (ilink_proxy) ───┐
│  echo                │  │  消息写入虚拟 Bot 消息队列               │
│  openai_compatible   │  │  （不调用 Handle()，不回复）              │
│  → Handle()          │  │  外部服务通过 getupdates 消费队列         │
│  → AI API 响应       │  │  处理后通过 sendmessage 回复              │
│  → 管道回复用户      │  │                                          │
└──────────┬───────────┘  └────────────────────┬─────────────────────┘
           │                                    │
           │                       ┌────────────┘
           │                       │ 外部服务处理后
           │                       │ POST /ilink/bot/sendmessage
           │                       ▼
           │              ┌─────────────────┐
           │              │ iLink 服务端     │
           │              │ 透明转发回复     │
           │              └────────┬────────┘
           │                       │
           └───────────┬───────────┘
                       │
            ┌──────────▼──────────┐
            │  Connector.Send()   │
            │  转发到真实 iLink    │
            └──────────┬──────────┘
                       │
            ┌──────────▼──────────┐
            │    微信用户          │
            └─────────────────────┘
```

**关键理解**：
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
HermesClaw 通过 iLink 协议连接到 Gateway 的 iLink 服务端
```

### Adapter 类型

| 类型 | 类别 | 说明 | 是否创建虚拟 Bot |
|------|------|------|-----------------|
| `ilink_proxy` | 连接适配器 | 外部 iLink 服务（Hermes、OpenClaw） | 是 |
| `openai_compatible` | 后端适配器 | OpenAI 兼容 API（Claude、DeepSeek） | 否 |
| `echo` | 后端适配器 | 调试回显 | 否 |

**注意**：只有 `ilink_proxy` 会创建虚拟 Bot，用于外部 iLink 服务接入。

### 适配器注册表机制

适配器采用 **自注册（Self-Registration）模式**，新增适配器类型无需修改任何核心代码。

```
┌─────────────────────────────────────────────────────────────┐
│                    adapter 包启动时                         │
│                                                             │
│  echo.go  init() → RegisterAdapter("echo", creator)        │
  openai.go init() → RegisterAdapter("openai_compatible",…)  │
  ilink_proxy.go init() → RegisterAdapter("ilink_proxy",…)   │
│                        ↓                                     │
│  ┌─────────────────────────────────────────┐                │
│  │  registry = map[string]AdapterCreator   │                │
│  │    "echo"           → EchoCreator       │                │
│  │    "openai_compatible" → OAICreator     │                │
│  │    "ilink_proxy"    → ProxyCreator      │                │
│  └─────────────────────────────────────────┘                │
│                        ↓                                     │
│  CreateAdapterFromDB(b) → registry[b.Type].creator(b)      │
└─────────────────────────────────────────────────────────────┘
```

**新增适配器只需 3 步**：

1. 创建新文件（如 `anthropic.go`）
2. 实现 `BackendAdapter` 接口
3. 添加 `init()` 函数注册：

```go
func init() {
    RegisterAdapter("anthropic", func(b database.Backend) BackendAdapter {
        apiKey := GetJSONString(b.Config, "api_key")
        return NewAnthropicAdapter(b.ID, b.Name, apiKey)
    })
}
```

**零改** `factory.go`、`registry.go`、`types.go` 或任何现有文件。

### 通知模块


```
┌─────────────────────────────────────────────────────────────────────┐
│                        外部系统                                      │
│                    (企业微信、Slack、CI/CD 等)                        │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ 通知 POST 请求
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Gateway 通知端点                                │
│                  /api/v1/notify/send                                │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ 验证 Token
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        iLink 客户端                                  │
│                  Connector.SendTextWithCreds()                       │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        微信用户                                      │
│                  收到外部系统发送的消息                               │
└─────────────────────────────────────────────────────────────────────┘
```

**通知用途**：
- 允许外部系统（企业微信、Slack、CI/CD 等）向微信用户发送消息
- 单向通道：外部系统 → Gateway → 微信用户
- 通过 Token 认证，确保安全性
- 不参与消息代理，不影响虚拟 Bot 机制

**配置示例**：
```bash
# 外部系统调用通知端点发送消息
curl -X POST http://localhost:8080/api/v1/notify/send \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "to_user": "wxid_xxx",
    "content": "服务器部署完成"
  }'
```

### 虚拟 Bot 机制

#### 核心概念

虚拟 Bot 是 Gateway 的核心能力之一。它基于一个**真实微信账号**，通过代理机制**虚拟出多个独立的 Bot 实例**，供不同的外部服务使用。

```
┌─────────────────────────────────────────────────────────────────────┐
│                        真实微信账号                                   │
│                    (通过 QR 扫码登录)                                 │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ Gateway 代理
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        虚拟 Bot 实例                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │  Hermes Bot   │  │  OpenClaw Bot │  │  OpenCode Bot │  ...       │
│  │  gw_a1b2c3d4  │  │  gw_e5f6g7h8  │  │  gw_i9j0k1l2  │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│                                                                      │
│  每个虚拟 Bot：                                                       │
│  - 独立的 account_id (gw_xxxxx)                                      │
│  - 独立的消息队列                                                     │
│  - 独立的认证凭证                                                     │
│  - 可被不同的外部服务独立使用                                          │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        外部服务                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Hermes Agent  ←── 通过 iLink 协议连接 gw_a1b2c3d4                  │
│  OpenClaw      ←── 通过 iLink 协议连接 gw_e5f6g7h8                  │
│  OpenCode      ←── 通过 iLink 协议连接 gw_i9j0k1l2                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```
1. **一个真实账号**：用户通过 QR 扫码登录一个真实的微信账号（仅主客户端需要）
2. **独占轮询**：Gateway 作为 iLink 客户端唯一轮询真实微信，接收所有消息
3. **管道分发**：所有消息统一经过 Pipeline（命令→路由→后端），路由到 ilink_proxy 后端的消息进入虚拟 Bot 消息队列
4. **独立使用**：每个外部服务通过独立的虚拟 Bot token 认证，互不干扰
5. **统一发送**：外部服务发送的消息通过 Gateway 代理转发到真实微信

#### 优势

- **资源复用**：一个真实微信账号可供多个外部服务使用
- **隔离性**：每个外部服务有独立的认证 token，互不影响
- **扩展性**：可以创建无限个虚拟 Bot，不受微信登录限制
- **安全性**：外部服务不需要知道真实微信凭证

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ClawBot Gateway (端口 8080)                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─── 对外服务层 ────────────────────────────────────────────────┐  │
│  │  /ilink/bot/*      iLink 服务端端点（虚拟 Bot 接入）          │  │
│  │  /api/v1/*         管理 API（需鉴权）                         │  │
│  │  /ws               WebSocket                                  │  │
│  │  /                 Web UI                                     │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── 核心层 ───────────────────────────────────────────────────┐  │
│  │                                                               │  │
│  │  ┌─── iLink 客户端 ──────────────────────────────────────┐  │  │
│  │  │  Connector         连接腾讯 iLink API + 多账号轮询     │  │  │
│  │  │  accountPollLoop   长轮询接收微信消息                   │  │  │
│  │  │  SendTextWithCreds 发送消息到微信                       │  │  │
│  │  └────────────────────────────────────────────────────────┘  │  │
│  │                                                               │  │
│  │  │  ClientRegistry    虚拟 Bot 注册管理（透明代理）              │  │  │
│  │                                                               │  │
│  │  ┌─── 消息处理 ──────────────────────────────────────────┐  │  │
│  │  │  MessagePipeline   消息处理管道（命令→路由→后端→回复）   │  │  │
│  │  └────────────────────────────────────────────────────────┘  │  │
│  │                                                               │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── 适配器层（配置中心）──────────────────────────────────────┐  │
│  │  连接适配器                                                  │  │
│  │    ilink_proxy     外部 iLink 服务（生成虚拟 Bot 配置）      │  │
│  │                                                               │  │
│  │  后端适配器                                                   │  │
│  │    OpenAI Compatible  OpenAI 兼容 API（Claude/DeepSeek等）    │  │
│  │    Echo               调试回显                                 │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── Notify 通知模块（独立）───────────────────────────────────┐  │
│  │  允许外部系统向微信用户发送消息                                │  │
│  │  用途：企业微信、Slack、CI/CD 等外部系统推送通知               │  │
│  │  端点：/api/v1/notify/send                                   │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── 路由层 ───────────────────────────────────────────────────┐  │
│  │  会话级后端覆写（/use 设置）                                   │  │
│  │  默认后端兜底                                                 │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── 基础设施层 ───────────────────────────────────────────────┐  │
│  │  SessionContext    用户会话上下文管理                          │  │
│  │  Store             持久化存储（路由规则/用户设置/API Token）   │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 数据结构流转

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

#### ilink_proxy 配置流程（连接适配器）

```
1. 用户在管理页面添加 Adapter
   ┌─────────────────────────────────────┐
   │  添加后端                            │
   │  ├─ 类型: ilink_proxy               │
   │  ├─ 名称: Hermes Agent              │
   │  └─ [保存]                           │
   └─────────────────────────────────────┘

2. Gateway 直接生成虚拟 Bot 配置（无需 QR 扫码）
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

**注意**：虚拟 Bot 不需要 QR 扫码登录，Gateway 直接生成连接配置。

#### openai_compatible 配置流程（后端适配器）

```
1. 用户在管理页面添加 Adapter
   ┌─────────────────────────────────────┐
   │  添加后端                            │
   │  ├─ 类型: openai_compatible         │
   │  ├─ 名称: Claude 3.5 Sonnet         │
   │  ├─ API Key: sk-xxx                 │
   │  ├─ Base URL: https://api.anthropic │
   │  ├─ Model: claude-3-5-sonnet        │
   │  └─ [保存]                           │
   └─────────────────────────────────────┘

2. 用户通过微信发送消息
   微信消息 → Gateway → Pipeline → 路由 → Claude API → 回复微信
```

### 通知配置流程

```
1. 用户在通知页面创建 Token
   ┌─────────────────────────────────────┐
   │  创建 Token                         │
   │  ├─ 名称: 企业微信通知               │
   │  ├─ 绑定账号: 选择微信账号           │
   │  └─ [创建]                           │
   └─────────────────────────────────────┘

2. 复制 Token 到外部系统
   TOKEN=gw_xxxxx

3. 外部系统调用通知端点发送消息
   ┌─────────────────────────────────────┐
   │  外部系统                            │
   │  POST /api/v1/notify/send           │
   │  Authorization: Bearer <token>      │
   │  Body: { to_user, content }         │
   └─────────────────────────────────────┘

4. Gateway 转发到微信
   外部系统 → Gateway → Connector.Send() → 微信用户
```

### iLink 客户端端点（Connector）

Connector 使用的真实 iLink API 端点（腾讯服务器）：

| 端点 | 方法 | 说明 | 代码位置 | 用途 |
|------|------|------|----------|------|
| `/ilink/bot/getupdates` | POST | 长轮询获取消息 | `bot/account.go` | 接收微信消息 |
| `/ilink/bot/sendmessage` | POST | 发送消息 | `bot/send.go` | 发送消息到微信 |
| `/ilink/bot/sendtyping` | POST | 输入状态 | `bot/send.go` | 显示输入状态 |
| `/ilink/bot/getuploadurl` | POST | 获取上传 URL | `bot/media.go` | 上传文件 |
| `/ilink/bot/get_bot_qrcode` | GET | 获取登录二维码 | `bot/qrlogin.go` | 主客户端登录 |
| `/ilink/bot/get_qrcode_status` | GET | 检查二维码状态 | `bot/qrlogin.go` | 主客户端登录 |

**注意**：`get_bot_qrcode` 和 `get_qrcode_status` 仅用于主客户端（真实微信账号）的 QR 扫码登录。虚拟 Bot 不需要这些端点，因为 Gateway 直接生成连接配置。

### iLink 服务端端点（Server）

Gateway 对外提供的 iLink 兼容 API，供虚拟 Bot 连接。

| 端点 | 方法 | 说明 | 实现方式 |
|------|------|------|----------|
| `/ilink/bot/getupdates` | POST | 长轮询获取消息 | **队列消费**：从 Pipeline 维护的虚拟 Bot 队列获取已路由消息，不直接转发到腾讯 |
| `/ilink/bot/sendmessage` | POST | 发送消息 | 透明转发到真实 iLink API（回复用户） |
| `/ilink/bot/sendtyping` | POST | 输入状态 | 透明转发到真实 iLink API |
| `/ilink/bot/getconfig` | POST | 获取配置 | 透明转发到真实 iLink API |
| `/ilink/bot/getuploadurl` | POST | 获取上传 URL | 透明转发到真实 iLink API |

**getupdates 设计（队列消费）**：
```
外部服务 → Gateway iLink 服务端 → 验证虚拟 Bot token → 从虚拟 Bot 消息队列取出消息（长轮询）→ 返回消息列表
```

**sendmessage 设计（透明转发）**：
```
外部服务 → Gateway iLink 服务端 → 验证虚拟 Bot token → 透明转发到腾讯 iLink API → 微信用户收到回复
```

**注意**：
- 虚拟 Bot 不需要 QR 扫码登录，Gateway 直接生成连接配置
- `/get_bot_qrcode` 和 `/get_qrcode_status` 端点不在服务端提供
- `getupdates` 不直接转发到腾讯，而是从 Pipeline 管理的虚拟 Bot 消息队列消费
- `sendmessage`/`sendtyping`/`getconfig`/`getuploadurl` 透明转发到腾讯 iLink API

### iLink 服务端实现细节

#### 当前实现（internal/ilink/）

```go
// server.go - 路由注册
func (s *Server) RegisterRoutes(r *gin.Engine) {
    ilink := r.Group("/ilink/bot")
    ilink.Use(s.rateLimitMiddleware())
    ilink.Use(maxBodySizeMiddleware(MaxRequestBodySize))
    {
        ilink.POST("/getupdates", s.handleGetUpdates)     // 队列消费
        ilink.POST("/sendmessage", s.handleSendMessage)   // 透明转发
        ilink.POST("/sendtyping", s.handleSendTyping)     // 透明转发
        ilink.POST("/getconfig", s.handleGetConfig)       // 透明转发
        ilink.POST("/getuploadurl", s.handleGetUploadURL) // 透明转发
    }
}

// handler.go
// handleGetUpdates: 从虚拟 Bot 消息队列消费消息（不转发到腾讯）
// handleProxy (sendmessage等): 透明转发到腾讯 iLink API
```

#### 架构特点

- **getupdates 队列消费**：从 Pipeline 维护的虚拟 Bot 消息队列获取已路由消息，不直接转发到腾讯
- **其他端点透明转发**：sendmessage/sendtyping/getconfig/getuploadurl 透明转发到真实 iLink API
- **虚拟 Bot 消息队列**：由 Pipeline 生产消息，外部服务通过 getupdates 消费

**注意**：虚拟 Bot 不需要 QR 扫码登录，Gateway 直接生成连接配置，因此 `/get_bot_qrcode` 和 `/get_qrcode_status` 端点不在服务端提供。

### 虚拟 Bot 管理

#### 虚拟 Bot 结构

```go
// VirtualBot 虚拟 Bot 实例
type VirtualBot struct {
    AccountID  string         // 虚拟 Bot ID（如 gw_a1b2c3d4）
    UserID     string         // 用户 ID（如 gw_a1b2c3d4@im.wechat）
    BaseURL    string         // iLink API 地址
    Token      string         // 随机生成的认证 token（持久化到数据库）
    LastActive time.Time      // 最后活跃时间
    CreatedAt  time.Time      // 创建时间
}

// ClientRegistry 管理所有虚拟 Bot
type ClientRegistry struct {
    mu   sync.RWMutex
    bots map[string]*VirtualBot
}
```

#### 虚拟 Bot 消息队列

每个虚拟 Bot 拥有独立的消息队列，由 Pipeline 生产消息，外部服务通过 getupdates 消费。

```
Pipeline 路由到 ilink_proxy 后端
    │
    ▼
将 NormalizedMessage 转换为 iLink 原始格式（RawMessageItem）
    │
    ▼
放入虚拟 Bot 消息队列
    │
    ▼
外部服务通过 getupdates 长轮询消费队列中的消息
    │
    ▼
消费后消息从队列中移除
```

**队列模式特点**：
- 消息按后端隔离，每个虚拟 Bot 独立队列
- Pipeline 生产消息，外部服务消费消息
- 未选择后端时消息不进入虚拟 Bot 队列
- 消息消费后从队列中移除

#### 消息流

**虚拟 Bot 代理（ConnectionAdapter / ilink_proxy）**：
```
微信消息 → Connector → Pipeline → 路由到 ilink_proxy → 消息入队 → 外部服务 getupdates 消费 → 外部服务处理 → sendmessage 回复
```

**内置 AI 处理（BackendAdapter）**：
```
微信消息 → Connector → Pipeline → 路由到 openai_compatible → Handle() → AI API → 回复
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
| **BackendAdapter** | Handle/HandleStream | 处理消息，返回 AI 响应 | openai_compatible, echo |
| **ConnectionAdapter** | GetConnectionInfo | 提供外部服务连接配置 | ilink_proxy |

### Provider 类型一览

| 类型 | 类别 | 协议 | 流式支持 | 用途 | 配置项 |
|------|------|------|---------|------|--------|
| `ilink_proxy` | Connection | iLink | - | 外部 iLink 服务（Hermes、OpenClaw） | `name` |
| `openai_compatible` | Backend | HTTP (SSE) | 支持 | AI 对话（Claude、DeepSeek、GPT 等） | `api_key`, `base_url`, `model` |
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

### 一级命令（Gateway 内部命令）

| 命令 | 说明 |
|------|------|
| `/use` | 显示当前状态（后端、会话数） |
| `/use <后端ID>` | 持久切换到指定后端 |
| `/use main` | 回到主命令模式（清除后端选择） |
| `/backends` | 列出所有可用后端（含健康状态） |
| `/help` | 显示 Gateway 帮助信息 |

一级命令由 Gateway 的 CommandProcessor 直接处理，**不转发到任何后端**。

### 二级命令（代理命令）

根据配置的 providers 自动生成，格式为 `/<backend_id>`：

| 命令 | 行为 |
|------|------|
| `/<id>` (无参数) | 显示该后端状态 |
| `/<id> <msg>` | 剥离 `/<id>` 前缀，将 `<msg>` 一次性转发到 `<id>` 后端 |
| `/help <id>` (无参数) | 转发 `/help` 到 `<id>` 后端 |
| `/help <id> <args>` | 转发 `/help <args>` 到 `<id>` 后端 |

**注意**：二级命令是**一次性**的，不改变用户的持久后端选择。

## 路由引擎

### 三层优先级

1. **用户会话级覆写**（`/use` 设置）— 最高优先级
2. **关键词规则匹配** — 按优先级遍历规则
3. **默认后端兜底** — 未匹配时使用配置的 `default_backend`

### 路由规则数据模型

#### 规则条件

```go
// RouteCondition 单个匹配条件
type RouteCondition struct {
    ID            string `json:"id"`             // 条件 ID
    Field         string `json:"field"`          // 匹配字段
    Operator      string `json:"operator"`       // 匹配操作符
    Value         string `json:"value"`          // 匹配值
    CaseSensitive bool   `json:"case_sensitive"` // 是否区分大小写
    Negate        bool   `json:"negate"`         // 是否取反（非逻辑）
}

// 匹配字段
const (
    FieldMessage  = "message"  // 消息内容
    FieldFromUser = "from_user" // 发送者
    FieldToUser   = "to_user"  // 接收者
    FieldMsgType  = "msg_type" // 消息类型
)

// 匹配操作符
const (
    OpExact      = "exact"       // 精确匹配
    OpContains   = "contains"    // 包含
    OpStartsWith = "starts_with" // 前缀
    OpEndsWith   = "ends_with"   // 后缀
    OpRegex      = "regex"       // 正则表达式
)
```

#### 规则组

```go
// RouteRuleGroup 规则组（支持 AND/OR 逻辑）
type RouteRuleGroup struct {
    ID         string           `json:"id"`          // 组 ID
    Logic      string           `json:"logic"`       // 组内逻辑：and/or
    Conditions []RouteCondition `json:"conditions"`  // 条件列表
}

// RouteRule 路由规则
type RouteRule struct {
    ID          int              `json:"id"`
    Name        string           `json:"name"`        // 规则名称
    BackendID   string           `json:"backend_id"`  // 目标后端
    Priority    int              `json:"priority"`    // 优先级（越小越优先）
    Enabled     bool             `json:"enabled"`     // 是否启用
    Description string           `json:"description"` // 规则描述
    Groups      []RouteRuleGroup `json:"groups"`      // 规则组列表
    GroupLogic  string           `json:"group_logic"` // 组间逻辑：and/or
    CreatedAt   time.Time        `json:"created_at"`
    UpdatedAt   time.Time        `json:"updated_at"`
}
```

### 逻辑运算说明

| 逻辑 | 符号 | 说明 | 示例 |
|------|------|------|------|
| AND (且) | `&&` | 所有条件必须满足 | 消息包含"天气" AND 发送者是"admin" |
| OR (或) | `\|\|` | 任一条件满足即可 | 消息包含"天气" OR 消息包含"预报" |
| NOT (非) | `!` | 条件不满足时生效 | NOT 消息包含"测试" |

**组合示例**：
```
(消息包含"天气" OR 消息包含"预报") AND NOT 消息包含"测试"
= ((消息包含"天气") OR (消息包含"预报")) AND (NOT 消息包含"测试")
```

### 路由决策

```go
func (r *Router) Route(message string, userID string, msgType string) RouteDecision {
    // 1. 检查用户会话级覆写（最高优先级）
    if backendID, ok := r.userBackends[userID]; ok {
        return RouteDecision{BackendID: backendID, MatchedBy: "session"}
    }

    // 2. 按优先级遍历路由规则
    for _, rule := range r.keywordRules {
        if !rule.Enabled {
            continue
        }
        if r.matchRule(message, userID, msgType, rule) {
            return RouteDecision{
                BackendID: rule.BackendID,
                MatchedBy: "keyword",
                RuleID:    rule.ID,
            }
        }
    }

    // 3. 返回默认后端
    return RouteDecision{BackendID: r.defaultBackend, MatchedBy: "default"}
}

// matchRule 检查消息是否匹配规则（支持 AND/OR/NOT 逻辑）
func (r *Router) matchRule(message, userID, msgType string, rule RouteRule) bool {
    if len(rule.Groups) == 0 {
        return false
    }

    // 评估每个组
    groupResults := make([]bool, len(rule.Groups))
    for i, group := range rule.Groups {
        groupResults[i] = r.evaluateGroup(message, userID, msgType, group)
    }

    // 根据组间逻辑合并结果
    if rule.GroupLogic == "and" {
        for _, result := range groupResults {
            if !result {
                return false
            }
        }
        return true
    }
    // OR 逻辑
    for _, result := range groupResults {
        if result {
            return true
        }
    }
    return false
}

// evaluateGroup 评估单个规则组
func (r *Router) evaluateGroup(message, userID, msgType string, group RouteRuleGroup) bool {
    if len(group.Conditions) == 0 {
        return false
    }

    // 评估每个条件
    conditionResults := make([]bool, len(group.Conditions))
    for i, condition := range group.Conditions {
        result := r.evaluateCondition(message, userID, msgType, condition)
        conditionResults[i] = result
    }

    // 根据组内逻辑合并结果
    if group.Logic == "and" {
        for _, result := range conditionResults {
            if !result {
                return false
            }
        }
        return true
    }
    // OR 逻辑
    for _, result := range conditionResults {
        if result {
            return true
        }
    }
    return false
}

// evaluateCondition 评估单个条件（支持 NOT 逻辑）
func (r *Router) evaluateCondition(message, userID, msgType string, condition RouteCondition) bool {
    var result bool

    // 根据字段获取值
    var fieldValue string
    switch condition.Field {
    case FieldMessage:
        fieldValue = message
    case FieldFromUser:
        fieldValue = userID
    case FieldToUser:
        fieldValue = "" // 需要从上下文获取
    case FieldMsgType:
        fieldValue = msgType
    }

    // 根据操作符匹配
    switch condition.Operator {
    case OpExact:
        result = fieldValue == condition.Value
    case OpContains:
        if condition.CaseSensitive {
            result = strings.Contains(fieldValue, condition.Value)
        } else {
            result = strings.Contains(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
        }
    case OpStartsWith:
        if condition.CaseSensitive {
            result = strings.HasPrefix(fieldValue, condition.Value)
        } else {
            result = strings.HasPrefix(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
        }
    case OpEndsWith:
        if condition.CaseSensitive {
            result = strings.HasSuffix(fieldValue, condition.Value)
        } else {
            result = strings.HasSuffix(strings.ToLower(fieldValue), strings.ToLower(condition.Value))
        }
    case OpRegex:
        if compiled, ok := r.compiledRegex[condition.Value]; ok {
            result = compiled.MatchString(fieldValue)
        }
    }

    // NOT 逻辑：取反
    if condition.Negate {
        result = !result
    }

    return result
}
```

### 路由匹配流程

```
用户消息
    ↓
┌─────────────────────────────────────────────┐
│ 1. 检查用户会话级覆写（/use 设置）            │
│    如果有 → 使用覆写的后端                    │
└─────────────────────────────────────────────┘
    ↓ (无覆写)
┌─────────────────────────────────────────────┐
│ 2. 按优先级遍历路由规则                       │
│    - 检查规则是否启用                         │
│    - 根据匹配类型检查消息                      │
│    - 第一个匹配的规则生效                      │
└─────────────────────────────────────────────┘
    ↓ (无匹配)
┌─────────────────────────────────────────────┐
│ 3. 使用默认后端                              │
└─────────────────────────────────────────────┘
```

### 正则表达式安全

- 长度限制：200 字符
- 禁止嵌套量词（如 `(a+)+`）
- 执行超时：100ms
- 使用 `regexp.Compile` 预编译

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

- **get_context**：获取或创建用户会话（按 userID:backendID 隔离，独立模式）
- **cleanup_expired**：定期清理过期会话

## 多账号管理

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

### 服务器启动配置（.env）

服务器启动时读取的环境变量配置：

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
# WEIXIN_BASE_URL=https://ilinkai.weixin.qq.com
# WEIXIN_POLL_TIMEOUT=35
# WEIXIN_BOT_TYPE=3

# ── 日志 ──
CLAWBOT_LOG_LEVEL=info
```

### 运行时配置（数据库 + Web UI）

除了启动配置外，所有运行时配置通过 **数据库 + Web UI** 管理，不使用配置文件。

```
┌─────────────────────────────────────────────────────────────────────┐
│                        配置管理流程                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─── Web UI ─────────────────────────────────────────────────────┐ │
│  │  用户在管理页面配置各项设置                                      │ │
│  │  - 后端适配器：添加/编辑/删除                                    │ │
│  │  - 路由规则：添加/编辑/删除                                      │ │
│  │  - 系统设置：JWT 有效期、会话策略等                              │ │
│  │  - 通知 Token：创建/删除                                         │ │
│  └──────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│                              │ API 调用                              │
│                              ▼                                       │
│  ┌─── 管理 API ───────────────────────────────────────────────────┐ │
│  │  /api/v1/*                                                      │ │
│  └──────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│                              │ 读写                                 │
│                              ▼                                       │
│  ┌─── SQLite 数据库 ─────────────────────────────────────────────┐ │
│  │  data/clawbot.db                                                │ │
│  │  - settings 表：系统配置                                         │ │
│  │  - backends 表：后端适配器                                       │ │
│  │  - routes 表：路由规则                                           │ │
│  │  - accounts 表：微信账号                                         │ │
│  │  - virtual_bots 表：虚拟 Bot                                     │ │
│  │  - notify_tokens 表：通知 Token                                  │ │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 系统配置项

通过 Web UI 或 API 管理的配置：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `api.jwt_expiry_hours` | `24` | JWT 有效期（小时） |
| `api.allowed_origins` | `*` | 允许的来源 |
| `context.max_history` | `20` | 最大历史条数 |
| `context.ttl` | `3600` | 会话超时（秒） |
| `route.default_backend` | - | 默认后端 |

### 后端适配器配置

| 类型 | 配置项 | 说明 |
|------|--------|------|
| `echo` | 无 | 调试回显 |
| `openai_compatible` | `api_key`, `base_url`, `model` | OpenAI 兼容 API |
| `ilink_proxy` | 无（自动生成） | 外部 iLink 服务 |

### 配置管理 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/backends` | GET/POST | 后端适配器管理 |
| `/api/v1/backends/:id` | PUT/DELETE | 单个后端操作 |
| `/api/v1/routes` | GET/POST | 路由规则管理 |
| `/api/v1/routes/:id` | DELETE | 单个路由操作 |
| `/api/v1/accounts` | GET | 微信账号列表 |
| `/api/v1/config` | GET/PUT | 系统配置 |
| `/api/v1/notify/tokens` | GET/POST | 通知 Token 管理 |
| `/api/v1/notify/tokens/:id` | DELETE | 单个 Token 操作 |

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.22 |
| HTTP 框架 | Gin |
| WebSocket | gorilla/websocket |
| 数据库 | SQLite (go-sqlite3) |
| 启动配置 | .env 环境变量 |
| 运行时配置 | 数据库 + Web UI |
| 日志 | log/slog |
| 前端 | React 19 + TypeScript + Vite + Zustand |
| 设计系统 | CSS 自定义属性 + 组件库 |
| 部署 | Docker |

## 目录结构

```
clawbot-gateway/
├── main.go                      # 入口
├── .env                         # 服务器启动配置（环境变量）
├── DESIGN.md                    # 设计文档
├── Dockerfile                   # Docker 构建
├── go.mod / go.sum              # Go 依赖
│
├── internal/
│   ├── adapter/                 # 后端适配器（注册表模式，支持自注册扩展）
│   │   ├── types.go            # 接口定义 + 数据类型（ChatRequest/ChatResponse/ConnectionInfo）
│   │   ├── registry.go         # 适配器注册表（RegisterAdapter/CreateAdapter 自注册机制）
│   │   ├── factory.go          # AdapterFactory 实例管理（注册中心）
│   │   ├── echo.go             # Echo 调试适配器 + init() 自注册
│   │   ├── openai.go           # OpenAI 兼容适配器 + init() 自注册
│   │   └── ilink_proxy.go      # iLink 代理适配器（虚拟 Bot）+ init() 自注册
│   │
│   ├── api/                     # HTTP API 服务
│   │   ├── server.go            # API 服务器 + 路由注册 + 中间件
│   │   ├── pipeline.go          # 消息处理管道 + 命令处理器
│   │   ├── jwt.go               # JWT 鉴权
│   │   ├── backend_handler.go   # 后端管理 API
│   │   ├── route_handler.go     # 路由规则 API
│   │   ├── wechat_handler.go    # 微信账号 API
│   │   ├── account_handler.go   # 账号管理 API
│   │   ├── config_handler.go    # 配置管理 API
│   │   ├── token_handler.go     # Token 管理 API
│   │   ├── connection_handler.go # 连接信息 API
│   │   ├── message_handler.go   # 消息处理 API
│   │   └── user_handler.go      # 用户管理 API
│   │
│   ├── bot/                     # iLink 协议客户端
│   │   ├── client.go            # Connector 核心结构 + 消息广播接口
│   │   ├── message.go           # 消息模型 + 解析
│   │   ├── send.go              # 消息发送
│   │   ├── registry.go          # ClientRegistry + VirtualBot
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
│   │   ├── registry.go          # ClientRegistry + VirtualBot 管理
│   │   └── ratelimit.go         # 速率限制器
│   │
│   ├── log/                     # 日志
│   │   └── log.go               # slog 封装
│   │
│   ├── notify/                  # 通知模块
│   │   └── handler.go           # 通知处理
│   ├── relay/                   # 文件中转
│   │   ├── relay.go             # 文件转发接口
│   │   ├── forwarder.go         # LocalRelay
│   │   └── obsidian.go          # ObsidianRelay
│   ├── route/                   # 路由引擎
│   │   └── router.go            # Router + 关键词规则（route.go 已迁移）
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
| `ClientRegistry` 虚拟 Bot 管理 | ✅ 已实现 | `ilink/registry.go` |
| `VirtualBot` 队列消费 | 🔄 待实现 | getupdates 从虚拟 Bot 队列消费（当前为透明代理，待改造） |
| `accountPollLoop` 消息轮询 | ✅ 已实现 | `bot/account.go` |
### 实现状态（2026-07-26 更新）
#### 当前存在的问题

| 严重度 | 位置 | 描述 | 原因 |
|-------|------|------|------|
| 1 | `ilink/handler.go` | `forwardToILink()` 请求体为空 | 已修复 |
| 2 | `ilink/handler.go` | `handleGetUpdates` 直接代理而非使用队列 | 🔄 待改造 - 需改为从虚拟 Bot 队列消费 |
| 3 | `ilink/handler.go` | 自引用 URL 循环 | 虚拟 Bot 的 `BaseURL` 配置错误可能导致循环 |

#### 遗留设计参考

| 组件 | 状态 | 说明 |
|------|------|------|
| `ClientRegistry` 虚拟 Bot 管理 | ✅ 已实现 | `ilink/registry.go` |
| `VirtualBot` 队列消费 | 🔄 待实现 | getupdates 从虚拟 Bot 队列消费（当前为透明代理，待改造） |
| `accountPollLoop` 消息轮询 | ✅ 已实现 | `bot/account.go` |
| `MessageQueue` 消息队列 | 🔄 待实现 | 管道模式需要为每个虚拟 Bot 维护消息队列 |
| `MessageBroadcaster` 接口 | 🚫 已废弃 | 管道模式不再需要广播，改用队列 |

**注意**：虚拟 Bot 不需要 QR 扫码登录，因此 `/get_bot_qrcode` 和 `/get_qrcode_status` 端点不在服务端提供。

### 当前实现（已完成）

#### 1. Adapter 层

```go
// adapter.go - 两种适配器接口

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
    BaseURL   string  // iLink API 地址
}
```

#### 2. ilink_proxy 适配器

```go
// ilink_proxy.go

func NewILinkProxyAdapter(id, name, accountID, userID, baseURL string) *ILinkProxyAdapter {
    return &ILinkProxyAdapter{
        id:        id,
        name:      name,
        accountID: accountID,
        userID:    userID,
        baseURL:   baseURL,
        createdAt: time.Now(),
    }
}
```

#### 3. ClientRegistry（虚拟 Bot 管理）

```go
// ilink/registry.go - 虚拟 Bot 管理（队列消费模式）

type VirtualBot struct {
    AccountID  string
    UserID     string
    BaseURL    string
    LastActive time.Time
    CreatedAt  time.Time
}

type ClientRegistry struct {
    mu   sync.RWMutex
    bots map[string]*VirtualBot
}

func (r *ClientRegistry) Register(accountID, userID, baseURL string) *VirtualBot {
    r.mu.Lock()
    defer r.mu.Unlock()
    bot := &VirtualBot{
        AccountID:  accountID,
        UserID:     userID,
        BaseURL:    baseURL,
        LastActive: time.Now(),
        CreatedAt:  time.Now(),
    }
    r.bots[accountID] = bot
    return bot
}
```

#### 4. iLink Server（透明代理）

```go
// ilink/server.go

func (s *Server) RegisterRoutes(r *gin.Engine) {
    ilink := r.Group("/ilink/bot")
    {
        ilink.POST("/getupdates", s.handleGetUpdates)
        ilink.POST("/sendmessage", s.handleSendMessage)
        ilink.POST("/sendtyping", s.handleSendTyping)
        ilink.POST("/getconfig", s.handleGetConfig)
        ilink.POST("/getuploadurl", s.handleGetUploadURL)
    }
}

// handler.go - 透明代理
func (s *Server) handleProxy(c *gin.Context) {
    // 1. 验证虚拟 Bot token
    // 2. 获取真实 bot_token
    // 3. 读取请求体
    // 4. 转发到真实 iLink API
    // 5. 返回响应
}
```

#### 5. Connector

```go
// bot/client.go

type Connector struct {
    // ... 字段 ...
    broadcaster MessageBroadcaster
    syncBufStore SyncBufStore
}

// GetAccountTokenByVirtualID 根据虚拟 Bot ID 获取真实账号的 token
func (c *Connector) GetAccountTokenByVirtualID(virtualAccountID string) string {
    c.accountMu.RLock()
    defer c.accountMu.RUnlock()
    for _, a := range c.accounts {
        if a.Credentials != nil && a.Credentials.Token != "" {
            return a.Credentials.Token
        }
    }
    return ""
}
```

**虚拟 Bot 队列模式特点**：
- 消息按后端隔离，每个虚拟 Bot 独立队列
- Pipeline 生产消息，外部服务消费消息
- 未选择后端时消息不进入虚拟 Bot 队列
- 消息消费后从队列中移除
## 实现状态

### 已完成

| 组件 | 状态 | 文件 |
|------|------|------|
| **iLink 客户端** | | |
| `Connector` 核心结构 | ✅ 完成 | `internal/bot/client.go` |
| `accountPollLoop` 轮询 | ✅ 完成 | `internal/bot/account.go` |
| **iLink 服务端（透明代理）** | | |
| `Server` 基础结构 | ✅ 完成 | `internal/ilink/server.go` |
| 透明代理 handler | ✅ 完成 | `internal/ilink/handler.go` |
| 虚拟 Bot token 验证 | ✅ 完成 | `internal/ilink/server.go` |
| 真实 bot_token 获取 | ✅ 完成 | `internal/bot/client.go` |
| `ConnectionAdapter` 接口 | ✅ 完成 | `internal/adapter/types.go` |
| `ilink_proxy` 连接适配器 | ✅ 完成 | `internal/adapter/ilink_proxy.go` |
| `openai_compatible` 后端适配器 | ✅ 完成 | `internal/adapter/openai.go` |
| `echo` 后端适配器 | ✅ 完成 | `internal/adapter/echo.go` |
| `AdapterFactory` 两种适配器 | ✅ 完成 | `internal/adapter/factory.go` |
| `Registry` 自注册机制 | ✅ 完成 | `internal/adapter/registry.go`（新增适配器零改核心代码） |
| **通知模块** | | |
| Token 管理 | ✅ 完成 | `internal/api/notify_handler.go` |
| 消息发送端点 | ✅ 完成 | `/api/v1/notify/send` |
| **前端** | | |
| `ilink_proxy` 显示 | ✅ 完成 | `web/src/pages/ManagePage.tsx` |
| Token 管理页面 | ✅ 完成 | `web/src/pages/NotificationPage.tsx` |
| **数据库** | | |
| 所有存储层 | ✅ 完成 | `internal/database/` |

### 待修复

| 优先级 | 问题 | 文件 | 说明 |
|--------|------|------|------|
| **P1 - iLink 服务端** | | | |
| | 响应格式不完整 | `ilink/handler.go` | `handleGetUpdates` 缺少 `get_updates_buf` 字段 |
| | 消息转换不完整 | `ilink/handler.go` | `convertToILinkMessage` 需要支持更多消息类型 |
| **P2 - iLink 客户端** | | | |
| | `msgChan` 容量偏小 | `bot/client.go:69` | 容量 100，高负载可能丢消息 |

**已修复的 P0 问题**：
- ✅ 请求体丢失：使用 `bytes.NewReader(body)` 替代 `nil`
- ✅ 消息队列未使用：`handleGetUpdates` 现在从队列取消息
- ✅ 自引用 URL 循环：虚拟 Bot BaseURL 现在指向真实 iLink API
- ✅ 随机 account_id：`ILinkProxyAdapter` 现在使用确定性 ID
- ✅ 缺少 Authorization 头：转发请求现在包含真实的 bot_token

**注意**：虚拟 Bot 不需要 QR 扫码登录，Gateway 直接生成连接配置，因此 `/get_bot_qrcode` 和 `/get_qrcode_status` 端点不在服务端提供。

## 参考项目

- [HermesClaw](https://github.com/AaronWong1999/hermesclaw) — iLink 代理架构参考
- [WeClawBot-API](https://github.com/Cp0204/WeClawBot-API) — iLink API 实现参考
- [Hermes Agent](https://github.com/NousResearch/hermes-agent) — 外部服务接入参考
- [OpenClaw](https://github.com/openclaw/openclaw) — 外部服务接入参考
