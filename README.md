# 🦞 ClawBot Gateway

**微信多后端代理网关** — 解决微信机器人只能绑定一个后端的问题。

通过 iLink 协议接入微信，支持同时连接多个 AI 后端（Hermes、OpenClaw、DeepSeek 等），用户可通过命令切换后端。

---

## 架构图

```
微信用户
    ↓
┌─────────────────────────────────────────────────┐
│  ClawBot Gateway (端口 8080)                      │
│  ├── /ilink/bot/*        iLink API 服务          │
│  ├── /api/v1/*           管理 API (需鉴权)        │
│  └── /                   Web UI                  │
├─────────────────────────────────────────────────┤
│  命令系统                                        │
│  ├── /use hermes     切换到 Hermes（持久）        │
│  ├── /hermes         一次性转发到 Hermes          │
│  ├── /backends       列出所有后端                 │
│  └── /help           显示帮助                     │
├─────────────────────────────────────────────────┤
│  适配器层                                        │
│  ├── OpenAI Compatible  Claude / DeepSeek 等     │
│  ├── Webhook            单向通知推送              │
│  ├── iLink              转发到外部 iLink 服务     │
│  └── Echo               调试回显                  │
└─────────────────────────────────────────────────┘
```

## 核心功能

| 功能 | 说明 |
|------|------|
| **微信代理** | iLink 协议对接，多账号管理，QR 扫码登录 |
| **命令切换** | `/use hermes` 切换后端，`/hermes` 一次性转发 |
| **OpenAI 兼容** | 支持 Claude、DeepSeek、GPT 等所有 OpenAI 兼容 API |
| **Webhook 通知** | 单向推送通知到外部系统 |
| **文件中转** | 支持转发到 Obsidian 等外部服务 |
| **Web 管理** | React SPA，支持后端管理、路由配置、账号管理 |

## 快速开始

```bash
# 1. 配置
cp config.yaml config.local.yaml
# 编辑 config.local.yaml，设置 login_password 和 providers

# 2. 运行
go run main.go

# 3. 打开管理面板
# 浏览器访问 http://localhost:8080
```

## 命令系统

| 命令 | 行为 |
|------|------|
| `/use` | 显示当前状态 |
| `/use <backend>` | 切换到指定后端（持久） |
| `/backends` | 列出所有可用后端 |
| `/help` | 显示帮助 |
| `/<backend>` | **一次性转发**到该后端（不切换） |

**示例：**
```
/use hermes          → 切换到 Hermes（后续消息都走 Hermes）
/hermes              → 本次消息转发到 Hermes（不切换）
/openclaw            → 一次性转发到 OpenClaw
/backends            → 查看所有后端
```

## Provider 配置

```yaml
backend:
  providers:
    # OpenAI 兼容后端
    - id: claude
      name: "Claude"
      type: openai_compatible
      enabled: true
      config:
        api_key: "${ANTHROPIC_KEY}"
        base_url: "https://api.anthropic.com/v1"
        model: "claude-3-5-sonnet-latest"

    # Webhook 通知
    - id: notify
      name: "通知"
      type: webhook
      enabled: true
      config:
        url: "https://example.com/api/notify"

    # 调试回显
    - id: echo
      name: "Echo"
      type: echo
      enabled: true
      config: {}
```

## Provider 类型

| 类型 | 说明 |
|------|------|
| `openai_compatible` | OpenAI 兼容 API（Claude、DeepSeek、GPT 等） |
| `webhook` | 单向 HTTP POST 通知 |
| `echo` | 调试回显 |
| `ilink` | iLink 协议转发（Hermes、OpenClaw） |

## API 端点

### 公开端点
```
GET  /health              健康检查
POST /auth/login          登录（获取 JWT）
```

### iLink API（外部服务接入）
```
POST /ilink/bot/sendmessage    发送消息
POST /ilink/bot/sendtyping     输入状态
GET  /ilink/bot/getupdates     长轮询获取消息
```

### 管理 API（需鉴权）
```
GET    /api/v1/backends         列出后端
POST   /api/v1/backends         注册后端
POST   /api/v1/message/send     发送消息
POST   /api/v1/message/broadcast 广播消息
```

## 技术栈

- **后端**: Go 1.22 + Gin + gorilla/websocket
- **前端**: React 19 + TypeScript + Vite + Zustand
- **配置**: YAML + 环境变量展开
- **部署**: Docker

## 安全

- 登录密码通过环境变量设置
- JWT + API Token 双重鉴权
- Token 比较使用 constant-time compare

## License

MIT
