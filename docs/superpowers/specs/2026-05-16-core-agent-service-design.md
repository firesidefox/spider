# Core Agent Service — Design Spec

**Date:** 2026-05-16  
**Status:** Draft

---

## 1. 背景与目标

将 spider.ai 的 agent 编排能力独立为一个 HTTP 微服务，供 spider.ai 及其他业务系统（5+ 个，不同语言/团队）调用。

**核心价值：**
- 统一 LLM 配置（API key、model 管理集中，调用方不感知）
- 统一审计日志与 token 追踪
- 统一 EPA 行为约束（Explore→Plan→Act）
- 各业务系统无需自行实现 LLM 编排循环

---

## 2. 设计决策

| 决策 | 选择 | 原因 |
|------|------|------|
| 部署模型 | 独立进程/微服务 | 多系统、多语言接入 |
| 会话状态 | 无状态 | 调用方传完整 messages，service 纯计算 |
| 工具执行 | HTTP 回调（调用方实现） | 工具执行权在调用方，service 不耦合业务 |
| LLM 配置 | 服务端管理，调用方用别名 override | 统一 key 管理，调用方零感知升级 |
| 断线重连 | SSE Last-Event-ID + 内存 ring buffer | 无 DB 依赖，符合无状态设计 |

---

## 3. 架构总览

```
调用方 (spider.ai / 其他系统)
    │
    │  POST /v1/run {messages, tools, model, ...}
    │  Authorization: Bearer <api_key>
    ▼
┌─────────────────────────────────────┐
│         Core Agent Service          │
│                                     │
│  ┌──────────┐   ┌────────────────┐  │
│  │  Router  │   │  Audit Logger  │  │
│  │ (model   │   │  Token Tracker │  │
│  │  alias)  │   └────────────────┘  │
│  └──────────┘                       │
│  ┌──────────────────────────────┐   │
│  │        Agent Engine          │   │
│  │  EPA loop + retry + hooks    │   │
│  │  ┌──────────────────────┐   │   │
│  │  │  Tool Executor       │   │   │
│  │  │  (concurrent-aware)  │   │   │
│  │  └──────────────────────┘   │   │
│  │  ┌──────────────────────┐   │   │
│  │  │  Subagent Manager    │   │   │
│  │  └──────────────────────┘   │   │
│  │  ┌──────────────────────┐   │   │
│  │  │  Compressor          │   │   │
│  │  │  (4 strategies)      │   │   │
│  │  └──────────────────────┘   │   │
│  └──────────────────────────────┘   │
└─────────────────────────────────────┘
    │                        │
    │  SSE events stream      │  HTTP callback (tool execution)
    ▼                        ▼
调用方 SSE consumer      调用方 tool endpoint
```

**关键边界：**
- Core service 不存对话历史，不存工具实现
- 工具执行 100% 在调用方，core service 只做 HTTP 回调
- LLM API key 在 core service，调用方用 Bearer token 鉴权

---

## 4. API 接口

### 端点列表

```
POST /v1/run          运行 agent，SSE 流式返回
POST /v1/compress     主动压缩 messages
GET  /v1/models       查询可用 model 别名
```

### POST /v1/run 请求体

```json
{
  "messages": [
    {"role": "user", "content": "检查所有主机的磁盘使用率"}
  ],
  "tools": [
    {
      "name": "RunCommand",
      "description": "在主机上执行 CLI 命令",
      "schema": {
        "type": "object",
        "properties": {
          "host_id": {"type": "string"},
          "command": {"type": "string"}
        }
      },
      "callback_url": "http://spider.ai/agent-tools/execute",
      "concurrent": true
    }
  ],
  "system": "你是网络运维助手...",
  "model": "smart",
  "max_turns": 20,
  "compress": {
    "strategy": "llm_summary",
    "threshold": 80000
  },
  "hooks": {
    "confirm_required": "http://spider.ai/agent-hooks/confirm",
    "pre_tool_use":     "http://spider.ai/agent-hooks/pre-tool"
  }
}
```

### SSE 事件类型

每个事件带 `seq` 字段（自增）和 `run_id`，用于断线重连。

| 事件 | 说明 |
|------|------|
| `text_delta` | LLM 输出文本片段 |
| `tool_start` | 工具调用开始（含 input） |
| `tool_result` | 工具执行结果 |
| `confirm_required` | 等待人工审批 |
| `subagent_start` | 子 agent 启动 |
| `subagent_done` | 子 agent 完成 |
| `compress_start` | 压缩开始 |
| `compress_done` | 压缩完成 |
| `turn_usage` | 每轮 token 用量 |
| `retrying` | LLM 重试中 |
| `error` | 错误 |
| `done` | 运行结束 |

### 工具回调协议

调用方需实现一个 HTTP endpoint，core service 遇到工具调用时 POST 到 `callback_url`：

```
POST http://spider.ai/agent-tools/execute
{
  "run_id": "run_abc",
  "tool": "RunCommand",
  "input": {"host_id": "host1", "command": "df -h"}
}

← 响应
{
  "result": "Filesystem  Size  Used  Avail  Use%\n...",
  "is_error": false
}
```

---

## 5. 核心机制

### 5.1 多模型路由

服务端配置文件定义别名，调用方用别名不用真实 model ID：

```yaml
models:
  default: claude-sonnet-4-6
  fast:    claude-haiku-4-5
  smart:   claude-opus-4-7
```

`/v1/run` 不传 `model` 字段用 `default`，传别名按别名路由。LLM 升级只改配置，调用方零感知。

### 5.2 并发工具执行

工具声明 `"concurrent": true` 时，同一 turn 内的多个并发工具并行执行：

```
LLM 返回: [RunCommand(host1), RunCommand(host2), RunCommand(host3)]
              ↓ 全部 concurrent:true
同时发出三个 HTTP callback → 等所有结果 → 继续下一轮
```

非并发工具（有副作用/有序依赖）串行执行，调用方在工具定义里声明 `"concurrent": false`。

### 5.3 子 agent 派发

core service 内置 `SpawnAgent` 工具。主 agent 调用时，服务创建子 agent 实例：
- 继承父 agent 的 model 配置和工具集（同一批 callback_url）
- 独立 context，独立 turn 计数
- 子 agent 事件通过 `subagent_start` / `subagent_done` 包裹后推给调用方

调用方无需额外实现任何东西，子 agent 对调用方透明。

### 5.4 Context 压缩

支持四种策略，通过 `/v1/run` 的 `compress.strategy` 字段指定，或通过 `POST /v1/compress` 主动调用：

| 策略 | 说明 |
|------|------|
| `truncate` | 超出 threshold 时丢弃最早的消息 |
| `llm_summary` | 用 LLM 把历史压缩成摘要（现有 Compactor 复用） |
| `sliding_window` | 保留最近 N 轮，丢弃更早的 |
| `custom` | 调用方传入自定义压缩逻辑 callback_url |

**触发方式：**
- `/v1/run` 内部检测 token 数超阈值时自动触发（保底）
- `POST /v1/compress` 供调用方主动压缩

### 5.5 断线重连

每个 SSE event 带 `id` 字段（格式：`run_id:seq`）。断线后客户端自动重连并携带 `Last-Event-ID` header，服务端从对应 seq 开始重放内存 ring buffer。

| 场景 | 处理 |
|------|------|
| 断线时 run 还在跑 | buffer 有所有事件，重连后全部重放 |
| 断线时 run 已结束 | buffer 保留 5 分钟，重放完整历史 |
| 断线超过 5 分钟 | buffer 已释放，返回 `410 Gone` |
| 服务重启 | buffer 丢失，返回 `410 Gone` |

调用方收到 `410 Gone` 时重新发起 run。

### 5.6 Human-in-the-loop

高风险工具调用时，core service 发出 `confirm_required` 事件，同时 POST 到 `hooks.confirm_required` URL：

```
POST http://spider.ai/agent-hooks/confirm
{
  "run_id": "run_abc",
  "request_id": "req_xyz",
  "tool": "RunCommand",
  "input": {"command": "rm -rf /tmp/old"},
  "risk_level": "high"
}
```

调用方审批后回调 core service：

```
POST /v1/runs/{run_id}/confirm
{"request_id": "req_xyz", "approved": true}
```

超时（默认 5 分钟）未审批视为拒绝。

### 5.7 生命周期 Hook

调用方可在 `hooks` 字段注册 webhook，core service 在对应生命周期节点 POST 通知：

| Hook | 触发时机 | 用途 |
|------|---------|------|
| `pre_tool_use` | 工具执行前 | 审计、拦截、修改 input |
| `confirm_required` | 高风险工具等待审批 | 人工审批流 |

`pre_tool_use` 回调响应可返回 `{"action": "allow"}` 或 `{"action": "deny", "reason": "..."}` 来拦截工具调用。不响应或超时视为 allow。

---

## 6. 鉴权

调用方用 Bearer token 访问 core service：

```
Authorization: Bearer <api_key>
```

core service 维护 API key 表（key → caller_id），用于审计日志和 token 追踪。key 管理通过 admin API 或配置文件，不做复杂 RBAC。

---

## 7. 审计日志与 Token 追踪

每次 run 记录：
- `caller_id`（来自 API key）
- `run_id`、开始/结束时间、状态
- 每轮 input/output tokens
- 工具调用列表（tool name、duration、is_error）

按 `caller_id` 聚合 token 用量，支持限额配置（超限返回 `429`）。

---

## 8. 能力边界总览

| 能力 | 状态 |
|------|------|
| 多轮工具调用循环 + SSE 流 | ✅ |
| EPA 行为约束（内置） | ✅ |
| 工具注入（HTTP 回调） | ✅ |
| 并发工具执行 | ✅ |
| 子 agent 派发 | ✅ |
| Human-in-the-loop（webhook） | ✅ |
| Context 压缩（4 种策略） | ✅ |
| 断线重连（SSE Last-Event-ID） | ✅ |
| 多模型路由（命名别名） | ✅ |
| 审计日志 + Token 追踪 | ✅ |
| 会话历史存储 | ❌ 调用方自管 |
| 工具实现 | ❌ 调用方实现 |

---

## 9. 不在本期范围

- 多租户隔离（后期）
- spider.ai 接入适配层（独立 spec）
- Web 管理界面
- 计费系统
