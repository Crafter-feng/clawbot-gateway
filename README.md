# 🦞 ClawBot Proxy Gateway

**微信 ClawBot 多后端代理网关** — 借鉴 HermesClaw v4 架构设计，Go 实现，带 Web 管理面板。

把微信 ClawBot 的 iLink 消息流通过「三层路由」分发到不同的 AI 后端（OpenClaw / Claude / DeepSeek / 自建服务…），同时对外提供独立的 HTTP 消息推送接口。

---

## 架构图

```
微信 iLink API
      │
      ▼  (长轮询 35s)
┌─────────────┐
│  ClawBot     │  ← 扫码登录 → 收消息 → 发消息
│  Connector   │
└──────┬──────┘
       │ 收到消息
       ▼
┌──────────────┐     命令（/switch /help /routes）
│  Message      │ ── → 直接回复
│  Pipeline     │
│    +          │      关键词匹配
│  Router       │ ── → 命中 → 指定后端
│   3层路由      │      默认兜底
└──────┬──────┘
       │ 路由到后端
       ▼
┌─────────────────────────────────────┐
│  Adapter Factory                     │
│  ├── OpenAI 兼容（Claude/DeepSeek/…）│
│  ├── Echo 调试                       │
│  └── Webhook 转发（外部 HTTP 服务）    │
└─────────────────────────────────────┘
       │ 回复
       ▼
┌──────────┐
│  发送回   │  发送到微信 + 推送到 Webhook
│  微信用户  │
└──────────┘
```

## 功能清单

| 功能 | 状态 |
|:----|:----:|
| **扫码登录微信 ClawBot** | ✅ 生成二维码 → 后台轮询 → 自动保存凭证 → 重连 |
| **iLink 长轮询接收消息** | ✅ 35s 长轮询，增量 get_updates_buf |
| **发送文本 / 正在输入 / 媒体文件** | ✅ AES-128-CBC 加密上传 |
| **三层路由引擎** | ✅ 用户覆写 > 关键词规则 > 默认兜底 |
| **微信命令** | ✅ `/switch`, `/backends`, `/route add/del`, `/routes`, `/status`, `/clear`, `/help` |
| **OpenAI 兼容后端** | ✅ 通用接口（GPT / Claude / DeepSeek / 通义千问…） |
| **Webhook 后端** | ✅ 消息 POST 到外部服务 |
| **Echo 调试后端** | ✅ 原样返回 |
| **HTTP 消息推送 API** | ✅ 外部系统 `POST /api/v1/push/send` 发消息到微信 + 获取 AI 回复 |
| **广播消息** | ✅ `POST /api/v1/push/broadcast` |
| **Web 管理面板** | ✅ 仪表盘 / 账号管理 / 后端管理 / 路由规则 / 推送配置 / 用户会话 |
| **WebSocket API** | ✅ 实时对话接口 |
| **上下文管理** | ✅ 3 种策略：keep / clear / isolated |
| **JSON 文件持久化** | ✅ 凭证、路由规则、Webhook 配置 |
| **Docker 部署** | ✅ Dockerfile + docker-compose.yml |

---

## 快速开始

### 1. 按配置文件

```bash
cp config.yaml config.local.yaml
# 编辑 config.local.yaml，填入 API Key
```

### 2. 构建运行

```bash
# 方式一：直接 Go 运行
cd clawbot-gateway
go mod tidy
go run ./cmd/clawbot-gateway config.local.yaml

# 方式二：Docker Compose
docker compose up --build -d
```

### 3. 打开管理面板

浏览器访问 `http://localhost:8080` → 点击「📱 扫码绑定微信」→ 用微信扫码 → 绑定成功开始收发消息。

---

## API 文档

### 微信 ClawBot 原生 API

```
POST /api/v1/wechat/qrcode              # 获取二维码
POST /api/v1/wechat/qrcode/status       # 查询扫码状态
GET  /api/v1/wechat/accounts            # 已绑定账号列表
POST /api/v1/wechat/:id/disconnect      # 断开账号
```

### 后端管理

```
GET    /api/v1/backends                 # 列出所有后端
POST   /api/v1/backends                 # 注册新后端
DELETE /api/v1/backends/:id             # 删除后端
POST   /api/v1/backends/:id/test        # 测试后端连通性
PUT    /api/v1/backends/default         # 设置默认后端
```

### 路由规则

```
GET    /api/v1/routes                   # 列出规则
POST   /api/v1/routes                   # 添加规则 {keyword, backend}
DELETE /api/v1/routes/:index            # 删除规则
```

### ★ HTTP 消息推送接口（核心对外 API）

```bash
# 发送消息到 ClawBot + 等待 AI 回复
curl -X POST http://localhost:8080/api/v1/push/send \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -d '{
    "to_user": "微信用户ID（可选）",
    "content": "今天天气怎么样",
    "msg_type": 1,
    "backend_id": "openclaw（可选）",
    "wait_reply": true
  }'

# 响应
{
  "success": true,
  "message_id": "msg_174...",
  "reply": "今天晴，22-28°C",
  "backend": "openclaw"
}
```

```bash
# 广播消息到所有绑定微信号
curl -X POST http://localhost:8080/api/v1/push/broadcast \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -d '{"content": "系统维护通知：今晚 2:00-4:00 停服", "msg_type": 1}'
```

### Webhook 推送管理

```
GET    /api/v1/push/webhooks            # 列出 Webhook 端点
POST   /api/v1/push/webhooks            # 添加 Webhook
PUT    /api/v1/push/webhooks/:id        # 更新 Webhook
DELETE /api/v1/push/webhooks/:id        # 删除 Webhook
GET    /api/v1/push/routes              # 列出推送路由
POST   /api/v1/push/routes              # 添加推送路由
DELETE /api/v1/push/routes/:id          # 删除推送路由
```

### WebSocket

```
ws://localhost:8080/ws

→ {"type":"chat", "backend_id":"claude", "message":"你好", "user_id":"u1"}
← {"type":"reply", "content":"你好！", "backend":"claude"}
```

---

## 配置说明

关键配置项：

```yaml
server:
  port: 8080              # 统一端口（Web UI + REST API + WebSocket）

backend:
  default_backend: openclaw
  providers:
    - id: claude
      type: openai_compatible  # 兼容 OpenAI 格式的任意后端
      config:
        api_key: "${ANTHROPIC_API_KEY}"  # 环境变量引用
        base_url: "https://api.anthropic.com/v1"
        model: "claude-3-5-sonnet-latest"

context:
  switch_strategy: keep   # keep | clear | isolated
  ttl: 3600               # 会话超时（秒）
```

支持的类型：
- `echo` — 调试用，原样返回
- `openai_compatible` — OpenAI、Anthropic、DeepSeek、通义千问等
- `webhook_output` — 转发到外部 HTTP 服务处理

---

## 与 HermesClaw v4 对比

| 特性 | HermesClaw v4 (Python) | ClawBot Gateway (Go) |
|:----|:---------------------:|:-------------------:|
| 扫码登录 | ❌ 手动填 Token | ✅ Web UI 一键扫码 |
| 多账号 | ❌ 单 Token | ✅ 支持多微信账号 |
| Web 管理面板 | ❌ | ✅ 全功能管理界面 |
| HTTP 消息推送 API | ❌ | ✅ `POST /api/v1/push/send` |
| Webhook 推送 | ❌ | ✅ 消息→外部系统 |
| 关键词路由 | ❌ | ✅ 可视化配置 |
| 上下文管理 | ❌ | ✅ 3 种策略 |
| 性能 | Python | Go 单二进制 + goroutine |
| 镜像大小 | — | ~15MB (Alpine) |

---

## License

MIT
