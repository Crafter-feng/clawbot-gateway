# ClawBot Gateway 设计文档（修订版）

## 项目概述

ClawBot Gateway 是一个微信多后端代理网关。通过 iLink 协议接入微信，作为**唯一的 iLink 轮询者**管理微信连接，消息统一经过命令→路由→后端管线处理，同时对外提供 iLink 兼容 API 让外部服务接入。

**核心能力**：
- 作为 iLink 客户端连接真实微信，**独占轮询**
- 作为 iLink 服务端对外提供 API，供外部服务获取**已路由的消息**
- 消息统一经过 Pipeline：命令解析 → 路由决策 → 后端处理 → 回复
- 输出端回写到微信（通过 Connector 发送）

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
│  │    - 长轮询接收微信消息（独占轮询）                            │  │
│  │    - 发送消息到微信                                            │  │
│  │    - 管理多账号独立轮询                                        │  │
│  │  代码：internal/bot/                                          │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── iLink 服务端（Server）────────────────────────────────────┐  │
│  │  作用：对外提供 iLink 兼容 API                                 │  │
│  │  方向：外部服务 → Gateway:8080/ilink/bot/*                     │  │
│  │  功能：                                                       │  │
│  │    - getupdates：从 Pipeline 消费已路由的消息队列              │  │
│  │    - sendmessage：透明转发到真实 iLink API（回复用户）         │  │
│  │    - sendtyping/getconfig/getuploadurl：透明转发到真实 API    │  │
│  │    - 管理虚拟 Bot 的消息队列（按后端隔离）                     │  │
│  │  代码：internal/ilink/                                        │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
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

### 关键设计原则

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

### 消息处理 Pipeline

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
│    /use            → 显示状态 / 切换后端                    │
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

### iLink 服务端端点设计

| 端点 | 方法 | 实现方式 | 说明 |
|------|------|----------|------|
| `/ilink/bot/getupdates` | POST | **队列消费** | 从 Pipeline 维护的虚拟 Bot 队列获取已路由消息，不直接转发到腾讯 |
| `/ilink/bot/sendmessage` | POST | 透明转发 | 外部服务回复 → 转发到腾讯 iLink API → 微信用户 |
| `/ilink/bot/sendtyping` | POST | 透明转发 | 输入状态转发到腾讯 |
| `/ilink/bot/getconfig` | POST | 透明转发 | 获取配置转发到腾讯 |
| `/ilink/bot/getuploadurl` | POST | 透明转发 | 获取上传 URL 转发到腾讯 |

**getupdates 设计**：

```
外部服务 GET /ilink/bot/getupdates
    │
    ▼
Gateway iLink 服务端
    │ 验证虚拟 Bot token
    │ 从虚拟 Bot 消息队列取出消息（长轮询）
    │
    ▼
返回消息列表（已由 Pipeline 路由决策后入队的消息）
```

**sendmessage 设计**：

```
外部服务 POST /ilink/bot/sendmessage
    │
    ▼
Gateway iLink 服务端
    │ 验证虚拟 Bot token
    │ 透明转发到腾讯 iLink API
    │
    ▼
微信用户收到回复
```

### 虚拟 Bot 消息队列

每个虚拟 Bot 拥有独立的消息队列，由 Pipeline 生产消息，外部服务通过 getupdates 消费。

```go
// VirtualBot 虚拟 Bot 实例
type VirtualBot struct {
    AccountID  string
    UserID     string
    BaseURL    string
    Token      string
    LastActive time.Time
    CreatedAt  time.Time
}
```

Pipeline 在路由到 `ilink_proxy` 后端时，将 `NormalizedMessage` 转换为 iLink 原始格式（`RawMessageItem`），放入虚拟 Bot 消息队列。

外部服务通过 `getupdates` 长轮询消费队列中的消息，消费后消息从队列中移除。

### 与旧设计的区别

| 维度 | 旧设计 | 新设计 |
|------|--------|--------|
| `getupdates` 实现 | 透明代理到腾讯 iLink API | 从虚拟 Bot 队列消费 Pipeline 路由的消息 |
| 消息来源 | 两条独立轮询链路各拿各的 | 只有 Connector 轮询，Pipeline 统一分发 |
| 命令解析 | 透传拿原始消息，命令被两端各处理 | 命令由 Pipeline 统一处理，非命令消息路由后入队 |
| 路由决策 | 未选择后端时 Hermes 仍收到消息 | 未选择后端时消息不进入虚拟 Bot 队列 |
| `sendmessage` | 透明转发（不变） | 透明转发（不变） |
| `sendtyping` 等 | 透明转发（不变） | 透明转发（不变） |

## 命令系统

### 命令优先级

命令解析始终优先于路由，用户输入以 `/` 开头时优先匹配命令。

### 一级命令（Gateway 内部命令）

| 命令 | 说明 |
|------|------|
| `/use` | 显示当前状态（后端、会话数） |
| `/use <后端ID>` | 持久切换到指定后端 |
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

### 二级命令处理流程

```
/<id> <msg> 到达 Pipeline
    │
    ▼
CommandProcessor 识别为 forward_to
    │
    ▼
剥离命令前缀
  /hermes xxx        → forwardContent = "xxx"
  /help hermes xxx   → forwardContent = "/help xxx"
  /hermes            → 显示后端状态（不转发）
  /help hermes       → forwardContent = "/help"
    │
    ▼
forwardToBackend(forwardContent, backendID)
    │
    ├── BackendAdapter (echo/openai)
    │   → Handle(content) → 回复用户（带 [id] 前缀）
    │
    └── ConnectionAdapter (ilink_proxy)
        → 消息入队（虚拟 Bot 队列）
        → 不回复（外部服务自行处理并回复）
```

## 路由引擎

### 三层优先级

1. **用户会话级覆写**（`/use` 设置）— 最高优先级
2. **关键词规则匹配** — 按优先级遍历规则
3. **默认后端兜底** — 未匹配时使用配置的 `default_backend`

### 路由到 ilink_proxy 后端

当路由决策结果是 `ilink_proxy` 类型后端时：
1. 将 `NormalizedMessage` 转换为 iLink 原始格式
2. 放入对应虚拟 Bot 的消息队列
3. 不调用 `Handle()`，不回复用户
4. 外部服务通过 `getupdates` 消费消息后处理并回复

## 通知模块

通知模块是独立模块，不参与消息代理。

```
外部系统 → POST /api/v1/notify/send → 验证 Token → Connector.SendTextWithCreds() → 微信用户
```

## Adapter 配置流程

### ilink_proxy 配置流程（连接适配器）

```
1. 管理页面添加 Adapter → 类型 ilink_proxy → 名称 Hermes Agent
2. Gateway 生成虚拟 Bot（account_id, user_id, token）
3. 用户复制连接配置到 Hermes 的 .env
   WEIXIN_BASE_URL=http://gateway:8080
   WEIXIN_TOKEN=<虚拟 Bot token>
   WEIXIN_ACCOUNT_ID=gw_xxxx
4. Hermes 启动，通过 iLink 协议连接 Gateway 的 iLink 服务端
5. Hermes 轮询 getupdates 消费队列中的消息
6. Hermes 处理消息后通过 sendmessage 回复
```

## 安全

- 登录密码通过环境变量设置
- JWT + API Token 双重鉴权
- 虚拟 Bot token 持久化存储，重启不变
- Token 比较使用 constant-time compare
- 虚拟 Bot 使用 Gateway 生成的 token，不暴露真实微信凭证