# AGENTS.md - ClawBot Gateway 核心规则

## 架构规则

### 1. iLink 客户端 vs 服务端必须清晰区分

| 组件 | 位置 | 职责 |
|------|------|------|
| iLink 客户端 | `internal/bot/` | **唯一**连接腾讯 iLink API，独占轮询、收发微信消息 |
| iLink 服务端 | `internal/ilink/` | 对外提供 iLink 兼容 API，`getupdates` 从队列消费，其它端点透明转发回复到腾讯 |

**绝对禁止**（任何场景，不许例外）：
- 在 iLink 服务端代码中直接调用腾讯 iLink API 的 `getupdates`（即使作为"代理"也不行）
- 在 iLink 服务端代码中通过 `http.NewRequest`、`http.Post`、`forwardToILink` 等任何方式向腾讯 iLink API 发起 `getupdates` 调用
- 在 iLink 客户端代码中处理外部服务请求

**说明**：iLink 服务端的 `sendmessage`/`sendtyping`/`getuploadurl` 通过 `ForwardFunc` 回调经 Connector 处理，不直接连接腾讯。`getconfig` 从虚拟 Bot 注册表返回自身配置，不依赖真实账号。`getupdates` 必须从 Pipeline 维护的虚拟 Bot 消息队列消费，**绝不许直接转发到腾讯**。

### 2. Pipeline 是消息统一入口

所有微信消息统一经过 `MessagePipeline`（命令→路由→后端），没有任何消息能绕过 Pipeline。

| 模式 | 说明 | 是否需要解析消息 |
|------|------|-----------------|
| 后端处理 | 路由到后端（echo/openai），调用 `Handle()` 处理后同步回复 | 是（NormalizedMessage → ChatRequest → Handle()） |
| 虚拟 Bot 代理 | 路由到 ilink_proxy，`Handle()` 入队虚拟 Bot 后返回空 resp.Text，Pipeline 跳过直接回复，外部服务通过 getupdates 消费 + sendmessage 异步回复 | 是（NormalizedMessage → ChatRequest → Handle() → 空 resp） |
| 文件中转 | 路由到 RelayMgr 转发文件 | 是（NormalizedMessage → FileMessage） |

**iLink 服务端不参与消息解析和路由**。iLink 服务端只和 Pipeline 交互：从队列拿消息（getupdates），回复通过 ForwardFunc 回调经 Connector 处理（sendmessage 等）。

### 3. 虚拟 Bot 不需要 QR 扫码

- 虚拟 Bot 由 Gateway 直接生成连接配置（account_id、user_id、base_url）
- 只有主客户端（真实微信账号）需要 QR 扫码登录
- iLink 服务端不提供 `/get_bot_qrcode` 和 `/get_qrcode_status` 端点

### 4. 通知是独立模块

- 通知是独立的单向通道模块，不是适配器
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

**统一 Handle()**：所有后端统一通过 `Handle()` 入口。`forwardToBackend` 不再按类型分支，统一调用 `bak.Handle()`。`ilink_proxy` 的 `Handle()` 通过 `EnqueueFunc` 回调入队虚拟 Bot 后返回空响应，Pipeline 识别后跳过直接回复，等待外部服务通过 `SendOutgoingMessage` 异步回复。

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
- 系统设置（JWT 有效期、会话超时等，独立模式内置，无切换策略配置）

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
- 任何 PR 不许在 `ilink/handler.go` 中直接调用腾讯 iLink API（违反架构规则 1）
- code review 必须检查 `internal/ilink/handler.go` 的实现：确认所有端点不直接连接腾讯 iLink API

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
| 通知通道 | Bearer Token（通知 Token） |

### 13. Token 管理

- 真实微信凭证（bot_token）仅存储在数据库中
- 虚拟 Bot 使用 Gateway 生成的 token
- 通知 Token 支持创建和删除，创建后仅显示一次

## 版本管理

### 18. 版本统一管理

版本号的**唯一权威来源**是项目根目录的 `VISION.txt`，其他所有位置的版本号通过构建工具自动注入，不得手动修改。

| 位置 | 版本来源 | 说明 |
|------|---------|------|
| `VISION.txt` | 手动维护 | **唯一权威来源**，修改版本号时只需改此处 |
| `internal/version/version.go` | `-ldflags` 注入 | 构建时由 `build-fpk.sh` 读取写入 |
| `web/package.json` | 手动同步 | 前端版本号，独立于构建版本 |
| `EzFavorites/FnDepot/clawbot-gateway/manifest` | 手动同步或 CI 设置 | FPK 版本号，发布于 FnDepot 仓库 |
| Git tag | `v${VERSION}` | 发布时打 tag 与 VISION.txt 一致 |

**发布流程**：
1. 更新 `VISION.txt` 中的 `VERSION`
2. 同步 `web/package.json` 的 `version` 字段
3. 提交并打 tag：`git tag vX.Y.Z && git push origin vX.Y.Z`
4. CI 自动构建 FPK 和 Docker 镜像

**禁止**：
- 直接修改 `internal/version/version.go` 中的版本号
- 直接修改 `EzFavorites/FnDepot/clawbot-gateway/manifest` 中的 `version` 字段（在 FnDepot 仓库中修改）
- 发布时忘记更新 `VISION.txt`

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
