# Spec: TodoTask Tool

**Date:** 2026-05-10  
**Status:** Draft

## 1. 目标

为 spider.ai agent 添加 `TodoTaskTool`，让 LLM 在执行复杂任务时能创建和管理子任务，用户在对话框中实时看到进度。

## 2. 数据模型

### 2.1 DB 表：`todo_tasks`

```sql
CREATE TABLE IF NOT EXISTS todo_tasks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT    NOT NULL,
    subject         TEXT    NOT NULL,
    description     TEXT,
    status          TEXT    NOT NULL DEFAULT 'pending',
    owner           TEXT,
    blocked_by      TEXT,   -- JSON array of int ids, e.g. "[1,2]"
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

`status` 枚举：`pending` | `in_progress` | `completed` | `deleted`

### 2.2 Go 模型：`models.TodoTask`

```go
type TodoTask struct {
    ID             int64     `json:"id"`
    ConversationID string    `json:"conversation_id"`
    Subject        string    `json:"subject"`
    Description    string    `json:"description,omitempty"`
    Status         string    `json:"status"`
    Owner          string    `json:"owner,omitempty"`
    BlockedBy      []int64   `json:"blocked_by,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

## 3. Store：`TodoTaskStore`

位置：`internal/store/todo_task_store.go`

接口：
- `Create(task *models.TodoTask) error` — 插入，回填 ID
- `Update(id int64, subject, description, status, owner string, blockedBy []int64) error` — 空字符串字段跳过更新
- `List(conversationID string) ([]*models.TodoTask, error)` — 按 conversation 查询，排除 deleted，按 `id ASC` 排序

**并发安全：** LLM 可能并行发起多个工具调用。`TodoTaskStore` 内置 `sync.Mutex`，所有写操作（Create/Update）加锁，防止 `SQLITE_BUSY`。

## 4. Agent Tool：`TodoTaskTool`

位置：`internal/agent/tools_todo_task.go`

### 4.1 工具定义

- **Name:** `TodoTask`
- **RiskLevel:** `RiskL1`（读写任务元数据，无副作用）
- **Description:**
  ```
  Manage the todo task list for the current conversation.
  
  Actions:
  - create: Create a new task. Required: subject. Optional: description, owner, blocked_by (array of task IDs that must complete first).
  - update: Update an existing task. Required: task_id. Must include at least one of: subject, description, status, owner, blocked_by. Calling update with only task_id is invalid.
  - list: List all non-deleted tasks for the current conversation, ordered by creation time.
  
  Status values: pending, in_progress, completed, deleted.
  ```

### 4.2 InputSchema

```json
{
  "type": "object",
  "required": ["action"],
  "properties": {
    "action": {
      "type": "string",
      "enum": ["create", "update", "list"]
    },
    "subject":     { "type": "string" },
    "description": { "type": "string" },
    "status":      { "type": "string", "enum": ["pending","in_progress","completed","deleted"] },
    "owner":       { "type": "string" },
    "blocked_by":  { "type": "array", "items": { "type": "integer" } },
    "task_id":     { "type": "integer" }
  }
}
```

### 4.3 操作语义

| action   | 必填字段          | 行为                                      |
|----------|-------------------|-------------------------------------------|
| `create` | `subject`         | 创建任务，返回 `{"id": N}`                |
| `update` | `task_id` + 至少一个其他字段 | 更新任意字段，返回更新后的完整任务 |
| `list`   | —                 | 返回当前对话所有非 deleted 任务列表       |

`update` 若仅传 `task_id` 无其他字段，`Execute()` 返回 error（兜底，description 已说明）。

### 4.4 构造与 conversationID 注入

`TodoTaskTool` 构造时注入 `conversationID`（来自 `agent.Run()` 的参数）。每次 `agent.Run()` 调用前，`Factory.NewAgent()` 接收 `conversationID` 并传给 `NewTodoTaskTool`。

```go
func NewTodoTaskTool(store *store.TodoTaskStore, broadcaster SSEBroadcaster, conversationID string) *TodoTaskTool
```

## 5. SSE 事件：`todotask_update`

每次 `create` 或 `update` 操作成功后，通过现有 `app.BroadcastSSE(conversationID, data)` 推送：

```json
{
  "type": "todotask_update",
  "content": { /* 完整 TodoTask 对象 */ }
}
```

### 5.1 推送时机

`TodoTaskTool.Execute()` 调用 store 后，通过注入的 `broadcaster` 接口推送。接口定义在 `internal/agent` 包，`*mcppkg.App` 已有 `BroadcastSSE(convID string, data []byte)` 方法，直接满足接口。

```go
// 定义在 internal/agent/tools_todo_task.go
type SSEBroadcaster interface {
    BroadcastSSE(conversationID string, data []byte)
}
```

## 6. Factory 集成

`NewAgent()` 签名改为接收 `conversationID`：

```go
func (f *Factory) NewAgent(systemPrompt string, conversationID string) *Agent
```

注册时传入：

```go
registry.Register(NewTodoTaskTool(f.TodoTaskStore, f.SSEBroadcaster, conversationID))
```

`Factory` 新增字段：
- `TodoTaskStore *store.TodoTaskStore`
- `SSEBroadcaster SSEBroadcaster`（由 `*mcppkg.App` 实现，在 `NewAgentFactory()` 中赋值）

## 7. 前端

### 7.1 SSE 处理

`chat_stream.go` 已有 SSE 推送机制。前端监听 `todotask_update` 事件，维护 `todoTasks: Map<id, TodoTask>`。

### 7.2 渲染

任务卡片 **sticky** 在输入框上方，不插入消息流。`todoTasks` 为空时不显示。SSE `todotask_update` 触发原地更新，不产生新消息。

**视觉结构：**

```
┌─────────────────────────────────────┐
│ ▣ TASKS  3/5                        │  ← header
├─────────────────────────────────────┤
│ ✓  检查 local110 固件版本    2.1s   │  ← completed
│ →  检查 local201 固件版本    ···    │  ← in_progress（blink）
│ ○  检查 local7 固件版本             │  ← pending
│ 🔒 升级 local7 固件                 │  ← blocked
└─────────────────────────────────────┘
```

**CSS 规范（复用现有 CSS 变量，自动适配 dark/light 主题）：**

| 元素 | 样式 |
|------|------|
| 容器 | `border: 1px solid var(--border); border-left: 3px solid var(--primary); border-radius: 6px; background: var(--input-bg)` |
| header | `font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.8px; color: var(--primary)` |
| completed 行 | 图标 `color: var(--green)`，文字 `color: var(--muted)` |
| in_progress 行 | 图标 `color: var(--primary)` + blink 动画，文字 `color: var(--text-sub)` |
| pending 行 | 图标 + 文字 `color: var(--muted)` |
| blocked 行 | 图标 🔒，整行 `color: var(--muted); opacity: 0.5` |
| 耗时 | `color: var(--muted); font-size: 11px; margin-left: auto` |

## 8. 实现顺序

1. DB schema 迁移 + `models.TodoTask`
2. `TodoTaskStore`
3. `TodoTaskTool`（含 SSE 推送）
4. Factory 注册
5. 前端 SSE 处理 + 任务卡片渲染

## 9. 不在范围内

- 用户手动创建任务
- 跨对话的全局任务面板
- 任务优先级、截止日期
- 任务评论/附件
