# Agent Todo Context Design

**Date:** 2026-05-14  
**Status:** Draft

## Problem

When a user replies to an agent mid-task (e.g., confirming a destructive action), a new `Run()` is triggered with a new `turnID`. The LLM has no visibility into existing tasks and recreates them, causing duplicate tasks in the UI.

Root cause: task state lives in the DB but is never injected back into LLM context on subsequent turns.

## Design Decisions

### 1. Remove `turnID`

`turnID` was intended to group tasks by user request, but the grouping logic in `TodoStore.List()` causes duplicate display when the same conversation spans multiple `Run()` calls.

Claude Code has no turn concept. Tasks are conversation-scoped only.

**Changes:**
- Drop `turn_id` column from `todo_tasks` table (migration required)
- Remove `TurnID` from `Factory`, `TodoTool`, `TodoStore`
- `TodoStore.List()` becomes: `WHERE conversation_id = ? AND status NOT IN ('completed', 'deleted')`

### 2. Task Lifecycle

- **States:** `pending`, `in_progress`, `completed`, `deleted`
- **Completed tasks:** filtered out of `List()` — do not appear in UI
- **All completed:** when `List()` returns empty after an update, frontend clears the task panel (mirrors Claude Code behavior)
- **`deleted` state:** retained for explicit removal; `completed` is the normal terminal state

### 3. Context Injection (Attachment Pattern)

On every `Run()`, after history is built and before the first LLM call, query `TodoStore.List(conversationID)`. If active tasks exist, prepend a synthetic user message to history:

```
<system-reminder>
Current tasks for this conversation:
[1] in_progress: 检查 ecs-tencent 磁盘用量
[2] pending: 清理 30 天前的日志
</system-reminder>
```

This message is **not written to DB** — it exists only in the in-memory history slice for the current `Run()`. The LLM sees current task state on every turn and updates existing tasks rather than recreating them.

Injection point in `agent.go`:

```go
// after history is built, before turn loop
if a.todoStore != nil {
    tasks, _ := a.todoStore.List(conversationID)
    if len(tasks) > 0 {
        history = append(history, buildTaskReminderMessage(tasks))
    }
}
```

### 4. Task IDs

DB auto-increment `int64`. With `turnID` removed and no duplicate creation, IDs within a conversation are naturally sequential. LLM references tasks by `task_id` (the DB ID).

## Data Flow

```
User message → Run()
  → msgStore.Save("user", ...)
  → rebuild history from DB
  → TodoStore.List(conversationID)
      → if tasks exist: append <system-reminder> to history (not saved)
  → LLM call (sees current task state)
  → LLM calls Todo.update (not Todo.create) for existing tasks
  → TodoStore.Update(...)
  → SSE broadcast to frontend
```

## Schema Migration

```sql
ALTER TABLE todo_tasks DROP COLUMN turn_id;
```

Since spider.ai uses SQLite and SQLite does not support `DROP COLUMN` before version 3.35, the migration recreates the table:

```sql
CREATE TABLE todo_tasks_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    subject     TEXT NOT NULL,
    active_form TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending',
    owner       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);
INSERT INTO todo_tasks_new SELECT id, conversation_id, subject, active_form, description, status, owner, created_at, updated_at FROM todo_tasks;
DROP TABLE todo_tasks;
ALTER TABLE todo_tasks_new RENAME TO todo_tasks;
```

## Affected Files

| File | Change |
|------|--------|
| `internal/db/schema.go` | Remove `turn_id` from schema |
| `internal/db/migrations.go` | Add migration to drop `turn_id` |
| `internal/models/todo_task.go` | Remove `TurnID` field |
| `internal/store/todo_task_store.go` | Remove `turn_id` and `blocked_by` from all queries; simplify `List()` |
| `internal/agent/tools_todo_task.go` | Remove `turnID` param and `blocked_by` field from `NewTodoTool` and schema |
| `internal/agent/agent.go` | Add `todoStore` field; inject task reminder into history |
| `internal/agent/factory.go` | Remove `TurnID` field; wire `TodoStore` into `AgentConfig` |
| `internal/api/chat.go` | Remove `factory.TurnID = uuid.New().String()` |

## Success Criteria

1. User sends message → agent creates tasks → agent stops to ask confirmation → user replies → agent continues → **no duplicate tasks created**
2. All tasks completed → task panel clears
3. Existing conversations with `turn_id` data migrate cleanly
