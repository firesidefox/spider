# Spec: Todo Summary on Completion

## 目标

Agent 完成所有 TodoTask 后，将任务列表以摘要形式输出到对话流，同时隐藏 TASKS 面板。

## 背景

TodoTask 的设计目标是为 agent 提供多步骤执行机制，不是持久化的任务管理系统。任务属于一次执行轮次（turn），该轮次所有任务完成后，面板消失，摘要进入对话作为执行记录。

## 数据模型变更

`todos` 表新增 `turn_id` 列（string，非空）。

- `turn_id` = 该轮次 assistant message 的 ID
- 每次 agent 开始执行时，`TodoTool` 从 factory 注入 `turn_id`
- 所有在同一轮次创建的 tasks 共享同一个 `turn_id`

## 行为规格

### 触发条件

当前 `turn_id` 下所有 tasks 状态均为 `completed` 或 `deleted`。

触发时机：agent 将最后一个 task 标为 `completed` 时，**立即**发送摘要（在 LLM 继续生成之前）。

### 后端

1. `execUpdate` 检测到当前 turn 所有 tasks 完成时，通过 `broadcaster` 发送：

```json
{ "type": "todo_summary", "content": "<markdown 任务列表>" }
```

2. `content` 格式：

```
**Tasks completed:**
1. task subject 1 (12s)
2. task subject 2 (34s)
3. task subject 3 (5s)
```

- 耗时 = `UpdatedAt - CreatedAt`（粗略近似）
- `deleted` 状态的 task 不出现在摘要中
- 按 task ID 升序排列

3. `todo_summary` 事件在 `todoAllDoneNudge` **之前**发送。

4. `getConversation` API 只返回**有未完成 tasks 的 turn** 的 tasks（即过滤掉所有 tasks 均为 completed/deleted 的 turn）。

### 前端

1. SSE handler 新增 `todo_summary` 事件处理：
   - 将 `content` 作为一条 assistant 消息追加到对话流（独立消息块，不与流式消息合并）
   - 清空该对话的 `todoTasksMap[convId]`

2. 面板因 `sortedTasks.length === 0` 自动隐藏。

3. `selectConversation` 加载时无需特殊处理（后端已过滤）。

### 刷新后的行为

- 执行中刷新：当前 turn 有未完成 tasks → API 返回，面板正常显示
- 全部完成后刷新：该 turn 所有 tasks 完成 → API 不返回，面板不显示

## 不改变的行为

- 进行中仍显示已完成任务的打勾状态
- DB 中 todo 记录不删除
- `todo_summary` 只在当前 turn 全部完成时发送一次

## 改动范围

| 文件 | 改动 |
|------|------|
| `internal/db/schema.go` | `todos` 表加 `turn_id` 列，migration |
| `internal/models/todo.go` | `Todo` struct 加 `TurnID` 字段 |
| `internal/store/todo_task_store.go` | Create 写入 `turn_id`；`allTasksDone` 改为按 turn_id 查询；List 过滤逻辑 |
| `internal/agent/tools_todo_task.go` | 注入 `turn_id`；`execUpdate` 检测 turn 完成，broadcast `todo_summary` |
| `internal/agent/factory.go` | 构建 `TodoTool` 时传入 `turn_id` |
| `internal/api/chat.go` | `getConversation` 过滤只返回有未完成 tasks 的 turn |
| `web/src/views/ChatView.vue` | SSE handler 处理 `todo_summary` 事件 |

## 验证

1. Agent 完成所有 task → TASKS 面板立即消失，对话流出现摘要
2. 摘要含编号、名称、耗时；`deleted` task 不出现
3. 部分完成时面板仍显示，无摘要
4. 执行中刷新页面 → 面板恢复显示未完成 tasks
5. 全部完成后刷新 → 面板不显示
6. 同一对话多轮次：每轮独立，上一轮完成不影响新一轮显示
