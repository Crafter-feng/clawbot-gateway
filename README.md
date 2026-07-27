# ClawBot Gateway

**微信多后端代理网关** — 解决微信机器人只能绑定一个后端的问题。

通过 iLink 协议接入微信，支持同时连接多个 AI 后端（Hermes、OpenClaw、DeepSeek 等），用户可通过命令切换后端。

---

## 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ClawBot Gateway (端口 8080)                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─── iLink 客户端（Connector）─────────────────────────────────┐  │
│  │  连接腾讯 iLink API，接收/发送微信消息                         │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── iLink 服务端（透明代理）──────────────────────────────────┐  │
│  │  对外提供 iLink 兼容 API，供虚拟 Bot 连接                      │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── 适配器层 ─────────────────────────────────────────────────┐  │
│  │  ilink_proxy      外部 iLink 服务（生成虚拟 Bot 配置）        │  │
│  │  openai_compatible OpenAI 兼容 API（Claude/DeepSeek等）       │  │
│  │  echo             调试回显                                     │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─── Webhook 通知模块 ────────────────────────────────────────┐  │
│  │  允许外部系统向微信用户发送消息                                │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## 核心功能

| 功能 | 说明 |
|------|------|
| **iLink 客户端** | 连接腾讯 iLink API，多账号管理，QR 扫码登录 |
| **iLink 服务端** | 透明代理，供虚拟 Bot 连接 |
| **虚拟 Bot** | 基于一个真实微信账号，虚拟出多个独立 Bot 实例 |
| **命令切换** | `/use hermes` 切换后端，`/hermes` 一次性转发 |
| **OpenAI 兼容** | 支持 Claude、DeepSeek、GPT 等所有 OpenAI 兼容 API |
| **Webhook 通知** | 允许外部系统向微信用户发送消息 |
| **Web 管理** | React SPA，支持后端管理、路由配置、账号管理 |

## 两种转发模式

| 模式 | 说明 | 是否需要解析消息 |
|------|------|-----------------|
| **iLink → iLink 透传** | 虚拟 Bot 代理，外部服务通过 Gateway 访问真实 iLink API | 否（透明通道） |
| **iLink → AI/Relay** | AI 处理或文件中转，需要解析消息内容 | 是（需要 NormalizedMessage） |

## 快速开始

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env，设置 CLAWBOT_LOGIN_PASSWORD

# 2. 运行
go run main.go

# 3. 打开管理面板
# 浏览器访问 http://localhost:8080
```

## CLI 命令

Gateway 支持运维子命令，直接操作数据库，不启动 HTTP 服务：

```bash
# 重置登录密码（生成随机密码）
./clawbot-gateway reset-password

# 重置为指定密码
./clawbot-gateway reset-password myNewPass

# Docker 环境
docker exec clawbot-gateway ./clawbot-gateway reset-password

# 查看帮助
./clawbot-gateway help
```

## 配置管理

### 启动配置（.env）

```bash
# 数据库路径
CLAWBOT_DB_PATH=data/clawbot.db

# 服务器配置
CLAWBOT_HOST=0.0.0.0
CLAWBOT_PORT=8080

# 认证
CLAWBOT_LOGIN_PASSWORD=1234

# 日志级别
CLAWBOT_LOG_LEVEL=info
```

### 运行时配置（数据库 + Web UI）

所有业务配置通过管理页面设置：
- 后端适配器配置
- 路由规则配置
- 系统设置（JWT 有效期、会话策略等）
- 通知 Token 配置
- **登录密码修改**（设置页 → 密码设置）

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

## Adapter 类型

| 类型 | 类别 | 说明 | 是否创建虚拟 Bot |
|------|------|------|-----------------|
| `ilink_proxy` | 连接适配器 | 外部 iLink 服务（Hermes、OpenClaw） | 是 |
| `openai_compatible` | 后端适配器 | OpenAI 兼容 API（Claude、DeepSeek） | 否 |
| `echo` | 后端适配器 | 调试回显 | 否 |

## API 端点

### 公开端点
```
GET  /health                    健康检查
POST /auth/login                登录（获取 JWT）
```

### iLink API（透明代理）
```
POST /ilink/bot/getupdates      长轮询获取消息（透明代理）
POST /ilink/bot/sendmessage     发送消息（透明代理）
POST /ilink/bot/sendtyping      输入状态（透明代理）
POST /ilink/bot/getconfig       获取配置（透明代理）
POST /ilink/bot/getuploadurl    获取上传 URL（透明代理）
### 管理 API（需鉴权）
```
GET    /api/v1/backends             列出后端
POST   /api/v1/backends             注册后端
PUT    /api/v1/backends/:id         更新后端
DELETE /api/v1/backends/:id         删除后端

GET    /api/v1/routes               列出路由规则
POST   /api/v1/routes               添加路由规则
DELETE /api/v1/routes/:id           删除路由规则

GET    /api/v1/accounts             列出微信账号

GET    /api/v1/config               获取系统配置
PUT    /api/v1/config               更新系统配置

GET    /api/v1/connections          列出连接适配器（含虚拟 Bot token 和连接状态）
GET    /api/v1/connections/:id      获取连接详情
GET    /api/v1/connections/stats    连接统计

PUT    /api/v1/auth/password        修改登录密码（需旧密码验证）
GET    /api/v1/auth/token           获取 API Token
POST   /api/v1/auth/token           重新生成 API Token

GET    /api/v1/notify/tokens        列出通知 Token
POST   /api/v1/notify/tokens        创建通知 Token
DELETE /api/v1/notify/tokens/:id    删除通知 Token
```

## 技术栈

- **后端**: Go 1.22 + Gin
- **数据库**: SQLite
- **前端**: React 19 + TypeScript + Vite + Zustand
- **配置**: .env（启动配置） + 数据库（运行时配置）
- **部署**: Docker

## 安全

- 登录密码通过环境变量设置
- JWT + API Token 双重鉴权
- Token 比较使用 constant-time compare
- 虚拟 Bot 使用 Gateway 生成的 token，不暴露真实微信凭证

## 项目结构

```
clawbot-gateway/
├── main.go                      # 入口
├── .env                         # 服务器启动配置
├── DESIGN.md                    # 设计文档
├── AGENTS.md                    # 核心规则
├── internal/
│   ├── adapter/                 # 后端适配器
│   ├── api/                     # HTTP API 服务
│   ├── bot/                     # iLink 客户端
│   ├── config/                  # 配置加载
│   ├── database/                # 数据库层
│   ├── ilink/                   # iLink 服务端（透明代理）
│   ├── log/                     # 日志
│   ├── notify/                  # Webhook 通知模块
│   ├── relay/                   # 文件中转
│   ├── route/                   # 路由引擎
│   ├── session/                 # 会话管理
│   └── crypto/                  # 加密工具
├── web/                         # 前端
└── data/                        # 运行时数据
```

## License

MIT
