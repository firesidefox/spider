# Spec: Tool Call 结构化数据持久化

## 背景

智能运维对话中，Agent 执行 tool calls（execute_cli、get_device_info 等）的结构化数据（工具名、输入、结果、耗时）仅在 SSE 流式传输时展示，未持久化到数据库。重新加载会话历史时 tool call 展开效果丢失。

## 目标

- 完整存储每个 tool call 的 name、input、result、isError、riskLevel、duration_ms
- 历史会话加载时还原 tool call 展开效果
- 兼容旧数据（无 tool calls 的消息）

## 存储方案

`messages` 表新增 `tool_calls TEXT NOT NULL DEFAULT ''` 列，存 JSON 数组：

```json
[{
  "id": "tc-xxx",
  "name": "execute_cli",
  "input": {"host": "local110", "command": "show ip route"},
  "result": "S   10.0.0.0/8 [1/0] via 10.37.129.1",
  "is_error": false,
  "risk_level": "moderate",
  "duration_ms": 1230
}]
```

空字符串 = 无 tool calls。

## 改动清单

### 后端

| 文件 | 改动 |
|------|------|
| `internal/db/schema.go` | migration: `ALTER TABLE messages ADD COLUMN tool_calls TEXT NOT NULL DEFAULT ''` |
| `internal/models/conversation.go` | `Message` struct 加 `ToolCalls string` 字段 |
| `internal/agent/agent.go` | `MessageStorer.Save` 签名加 `toolCalls string`；Run 中序列化 tool call 数据 + duration |
| `internal/store/message.go` | `Save` 加 `toolCalls` 参数；`ListByConversation` SELECT/scan 加 `tool_calls` |
| `internal/agent/agent_test.go` | mockMsgStore.Save 签名同步 |

### 前端

| 文件 | 改动 |
|------|------|
| `web/src/api/chat.ts` | `ChatMessage` interface 加 `tool_calls?: string` |
| `web/src/views/ChatView.vue` | `selectConversation` 解析 `tool_calls` JSON → `DisplayMessage.toolCalls` |
| `web/src/components/ChatMessage.vue` | tool call 展示加 duration 显示 |

## 验证

1. 发送触发 tool call 的消息 → 流式展示正常
2. 刷新页面 → 历史消息 tool calls 完整还原（可展开查看 input/result/duration）
3. 旧消息（无 tool_calls）→ 正常显示，无报错
4. `go test ./internal/store/...` 通过
5. `go test ./internal/agent/...` 通过
