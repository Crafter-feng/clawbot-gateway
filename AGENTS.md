# AGENTS.md - ClawBot Gateway 核心规则

## 架构规则

### 1. iLink 客户端 vs 服务端必须清晰区分

| 组件 | 位置 | 职责 |
|------|------|------|
| iLink 客户端 | `internal/bot/` | 连接腾讯 iLink API，接收/发送微信消息 |
| iLink 服务端 | `internal/ilink/` | 对外提供 iLink 兼容 API，供虚拟 Bot 连接 |

**禁止**：
- 在 iLink 服务端代码中直接调用腾讯 iLink API
- 在 iLink 客户端代码中处理外部服务请求

### 2. 两种转发模式必须区分

| 模式 | 说明 | 是否需要解析消息 |
|------|------|-----------------|
| iLink → iLink 透传 | 虚拟 Bot 代理，外部服务通过 Gateway 访问真实 iLink API | 否（透明通道） |
| iLink → AI/Relay | AI 处理或文件中转，需要解析消息内容 | 是（需要 NormalizedMessage） |

**iLink 服务端只负责透传**，不参与消息解析和 AI 处理。

### 3. 虚拟 Bot 不需要 QR 扫码

- 虚拟 Bot 由 Gateway 直接生成连接配置（account_id、user_id、base_url）
- 只有主客户端（真实微信账号）需要 QR 扫码登录
- iLink 服务端不提供 `/get_bot_qrcode` 和 `/get_qrcode_status` 端点

### 4. Webhook 是独立模块

- Webhook 是独立的单向通道模块，不是适配器
- 用途：允许外部系统向微信用户发送消息
- 不参与消息代理，不影响虚拟 Bot 机制
- 端点：`/api/v1/notify/send`

### 5. Adapter 类型区分

| 类型 | 类别 | 是否创建虚拟 Bot |
|------|------|-----------------|
| `ilink_proxy` | 连接适配器 | 是 |
| `openai_compatible` | 后端适配器 | 否 |
| `echo` | 后端适配器 | 否 |

**只有 `ilink_proxy` 会创建虚拟 Bot**。

### 5.1 适配器注册表（扩展规则）

适配器采用**自注册模式**，新增适配器类型时：

1. 在 `internal/adapter/` 下创建新文件（如 `anthropic.go`）
2. 实现 `BackendAdapter` 或 `ConnectionAdapter` 接口
3. 在 `init()` 中调用 `RegisterAdapter("type", creator)`

**禁止**：
- 修改 `factory.go` 或 `registry.go` 来添加新适配器
- 在 `types.go` 中添加具体适配器实现
- 在 `CreateAdapterFromDB` 中添加 switch case

## 配置规则

### 6. 配置管理方式

| 类型 | 方式 | 说明 |
|------|------|------|
| 启动配置 | .env 文件 | 服务器启动时读取的环境变量 |
| 运行时配置 | 数据库 + Web UI | 所有业务配置通过管理页面设置 |

**禁止**：
- 使用 YAML/JSON 配置文件存储业务配置
- 通过配置文件管理后端适配器、路由规则等

### 7. 配置项分类

**启动配置（.env）**：
- `CLAWBOT_DB_PATH`：数据库路径
- `CLAWBOT_HOST`：监听地址
- `CLAWBOT_PORT`：监听端口
- `CLAWBOT_LOGIN_PASSWORD`：登录密码
- `CLAWBOT_LOG_LEVEL`：日志级别

**运行时配置（数据库）**：
- 后端适配器配置
- 路由规则配置
- 系统设置（JWT 有效期、会话策略等）
- 通知 Token 配置

## 代码规则

### 8. 包依赖方向

```
internal/api → internal/bot, internal/ilink, internal/adapter, internal/database
internal/bot → (无外部依赖)
internal/ilink → internal/bot
internal/adapter → (无外部依赖)
internal/database → (无外部依赖)
```

**禁止**：
- `internal/bot` 依赖 `internal/ilink`（会导致循环依赖）
- `internal/ilink` 依赖 `internal/api`

### 9. 错误处理

- 所有 API 端点必须返回统一的错误格式：`{"error": "message"}`
- 401 错误触发自动登出
- 500 错误记录日志但不暴露内部细节

## 前端规则

### 10. 组件库规范

- 所有基础组件放在 `web/src/components/ui/`
- 使用 CSS 自定义属性，不使用硬编码颜色
- 所有交互组件必须支持键盘操作
- 删除操作必须使用 ConfirmDialog，不使用原生 `confirm()`

### 11. 状态管理

- 使用 Zustand 进行状态管理
- 每个领域一个 store（auth、accounts、backends、routes、stats）
- Store 只负责数据获取和状态管理，不处理业务逻辑

## 安全规则

### 12. 认证方式

| 场景 | 认证方式 |
|------|----------|
| Web UI 登录 | JWT Token |
| 管理 API | JWT Token |
| iLink 服务端 | Bearer Token（虚拟 Bot） |
| Webhook | Bearer Token（通知 Token） |

### 13. Token 管理

- 真实微信凭证（bot_token）仅存储在数据库中
- 虚拟 Bot 使用 Gateway 生成的 token
- 通知 Token 支持创建和删除，创建后仅显示一次

## 测试规则

### 14. 测试覆盖

- 所有 API 端点必须有集成测试
- 核心业务逻辑必须有单元测试
- 数据库操作必须有测试

### 15. 测试数据

- 测试使用内存数据库，不使用真实数据
- 测试完成后清理测试数据

## 部署规则

### 16. Docker 部署

- 使用多阶段构建减小镜像体积
- 数据卷挂载 `data/` 目录
- 环境变量通过 `.env` 文件或 docker-compose 注入

### 17. 数据备份

- SQLite 数据库文件需要定期备份
- 备份路径：`data/clawbot.db`
