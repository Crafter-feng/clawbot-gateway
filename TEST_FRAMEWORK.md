# ClawBot Gateway 测试验证框架

## 概述

使用 WeClawBot-API 作为模拟 iLink 后端，验证 ClawBot Gateway 的转发功能。

## 测试架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        测试环境                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─── 测试客户端 ───────────────────────────────────────────────┐  │
│  │  发送测试消息到 Gateway                                       │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│                              ▼                                       │
│  ┌─── ClawBot Gateway ──────────────────────────────────────────┐  │
│  │  /api/v1/routes/test  测试路由匹配                             │  │
│  │  /ilink/bot/*         透明代理转发                             │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│                              ▼                                       │
│  ┌─── WeClawBot-API (模拟后端) ─────────────────────────────────┐  │
│  │  接收并记录转发的请求                                          │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## 测试场景

### 场景 1：路由规则匹配测试

验证不同匹配类型的路由规则是否正确工作。

| 测试用例 | 输入消息 | 期望匹配规则 | 期望后端 |
|----------|----------|--------------|----------|
| 精确匹配 | "天气" | 天气查询 | hermes |
| 包含匹配 | "今天天气怎么样" | 天气查询 | hermes |
| 前缀匹配 | "天气预报" | 天气查询 | hermes |
| 后缀匹配 | "查天气" | 天气查询 | hermes |
| 正则匹配 | "/help me" | 帮助命令 | echo |
| 不匹配 | "你好世界" | 无 | 默认后端 |

### 场景 2：逻辑组合测试

验证 AND/OR/NOT 逻辑组合是否正确工作。

| 测试用例 | 输入条件 | 期望结果 |
|----------|----------|----------|
| AND 逻辑 | 消息包含"天气" AND 发送者是"admin" | 匹配 |
| OR 逻辑 | 消息包含"天气" OR 消息包含"预报" | 匹配 |
| NOT 逻辑 | NOT 消息包含"测试" | 匹配 |
| 混合逻辑 | (消息包含"天气" OR 消息包含"预报") AND NOT 消息包含"测试" | 匹配 |

### 场景 3：优先级测试

验证路由规则优先级是否正确工作。

| 测试用例 | 输入消息 | 期望匹配规则 | 期望后端 |
|----------|----------|--------------|----------|
| 高优先级 | "天气" | 规则1（优先级1） | hermes |
| 低优先级 | "天气预报" | 规则2（优先级2） | deepseek |

### 场景 4：透明代理测试

验证 iLink 服务端的透明代理功能。

| 测试用例 | 操作 | 期望结果 |
|----------|------|----------|
| getupdates | 外部服务调用 getupdates | 返回真实 iLink API 响应 |
| sendmessage | 外部服务调用 sendmessage | 消息发送成功 |
| sendtyping | 外部服务调用 sendtyping | 输入状态设置成功 |

### 场景 5：虚拟 Bot 认证测试

验证虚拟 Bot 的认证机制。

| 测试用例 | 操作 | 期望结果 |
|----------|------|----------|
| 有效 token | 使用正确的虚拟 Bot token | 认证成功 |
| 无效 token | 使用错误的 token | 401 未授权 |
| 过期 token | 使用过期的 token | 401 未授权 |

## 测试文件结构

```
test/
├── integration/                    # 集成测试
│   ├── route_test.go              # 路由规则测试
│   ├── proxy_test.go              # 透明代理测试
│   └── auth_test.go               # 认证测试
├── e2e/                            # 端到端测试
│   ├── full_flow_test.go          # 完整流程测试
│   └── weclawbot_integration.go   # WeClawBot-API 集成
├── mock/                           # 模拟服务
│   └── weclawbot.go               # WeClawBot-API 模拟器
└── fixtures/                       # 测试数据
    └── route_rules.json           # 测试用路由规则
```

## 测试执行

```bash
# 运行所有测试
go test ./test/... -v

# 运行路由规则测试
go test ./test/integration/... -v -run TestRoute

# 运行透明代理测试
go test ./test/integration/... -v -run TestProxy

# 运行端到端测试
go test ./test/e2e/... -v
```

## 测试数据

### 路由规则测试数据

```json
{
  "rules": [
    {
      "name": "天气查询",
      "backend_id": "hermes",
      "priority": 1,
      "enabled": true,
      "groups": [
        {
          "id": "g1",
          "logic": "and",
          "conditions": [
            {
              "id": "c1",
              "field": "message",
              "operator": "contains",
              "value": "天气",
              "case_sensitive": false,
              "negate": false
            }
          ]
        }
      ],
      "group_logic": "and"
    },
    {
      "name": "帮助命令",
      "backend_id": "echo",
      "priority": 2,
      "enabled": true,
      "groups": [
        {
          "id": "g1",
          "logic": "and",
          "conditions": [
            {
              "id": "c1",
              "field": "message",
              "operator": "regex",
              "value": "^/help.*",
              "case_sensitive": false,
              "negate": false
            }
          ]
        }
      ],
      "group_logic": "and"
    },
    {
      "name": "非测试消息",
      "backend_id": "deepseek",
      "priority": 3,
      "enabled": true,
      "groups": [
        {
          "id": "g1",
          "logic": "and",
          "conditions": [
            {
              "id": "c1",
              "field": "message",
              "operator": "contains",
              "value": "测试",
              "case_sensitive": false,
              "negate": true
            }
          ]
        }
      ],
      "group_logic": "and"
    }
  ]
}
```

## 测试报告格式

```json
{
  "test_run": "2026-07-26T15:00:00Z",
  "total_tests": 20,
  "passed": 18,
  "failed": 2,
  "results": [
    {
      "test_id": "route_001",
      "name": "精确匹配测试",
      "status": "passed",
      "duration_ms": 45
    },
    {
      "test_id": "route_002",
      "name": "包含匹配测试",
      "status": "failed",
      "duration_ms": 32,
      "error": "期望后端 'hermes'，实际得到 'echo'"
    }
  ]
}
```
