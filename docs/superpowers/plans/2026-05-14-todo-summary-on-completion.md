# Todo Summary on Completion — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When all Todo tasks in a turn complete, broadcast a summary SSE event, append it as an assistant message in the chat, and hide the TASKS panel.

**Architecture:** Add `turn_id` (string) to the `todos` table and `Todo` model. `TodoTool` receives a `turn_id` at construction time (generated per user message in `chatSendMessage`). When the last task in a turn is marked completed, the tool broadcasts a `todo_summary` SSE event. The frontend appends the summary as an assistant message and clears the task map. The `getConversation` API filters out turns where all tasks are done.

**Tech Stack:** Go (SQLite/database/sql, encoding/json), Vue 3 (Composition API, SSE)

---

## File Map

| File | Change |
|------|--------|
| `internal/db/schema.go` | Add `ALTER TABLE todo_tasks ADD COLUMN turn_id` migration |
| `internal/models/todo_task.go` | Add `TurnID string` field |
| `internal/store/todo_task_store.go` | Write `turn_id` on Create; add `ListByTurn`; filter `List` to exclude completed turns; update `allTasksDoneForTurn` |
| `internal/agent/tools_todo_task.go` | Accept `turnID` param; `execUpdate` broadcasts `todo_summary` when turn done |
| `internal/agent/factory.go` | Add `TurnID string` field; pass to `NewTodoTool` |
| `internal/api/chat.go` | Generate `turn_id` per request; set `factory.TurnID` |
| `web/src/views/ChatView.vue` | Handle `todo_summary` SSE event |

---

### Task 1: Schema migration — add `turn_id` to `todo_tasks`

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add migration line**

In `migrate()`, after the existing `db.Exec` calls, add:

```go
db.Exec("ALTER TABLE todo_tasks ADD COLUMN turn_id TEXT NOT NULL DEFAULT ''")
```

- [ ] **Step 2: Verify migration runs without error**

```bash
go build ./internal/db/... && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add turn_id column to todo_tasks"
```

---

### Task 2: Add `TurnID` to `Todo` model

**Files:**
- Modify: `internal/models/todo_task.go`

- [ ] **Step 1: Add field**

```go
type Todo struct {
    ID             int64     `json:"id"`
    ConversationID string    `json:"conversation_id"`
    TurnID         string    `json:"turn_id"`
    Subject        string    `json:"subject"`
    Description    string    `json:"description,omitempty"`
    Status         string    `json:"status"`
    Owner          string    `json:"owner,omitempty"`
    BlockedBy      []int64   `json:"blocked_by,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/models/... && echo OK
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add internal/models/todo_task.go
git commit -m "feat(models): add TurnID field to Todo"
```

---

### Task 3: Update `TodoStore` — write/read `turn_id`, filter completed turns

**Files:**
- Modify: `internal/store/todo_task_store.go`
- Test: `internal/agent/tools_todo_task_test.go`

- [ ] **Step 1: Write failing test for turn-scoped allTasksDone**

Add to `internal/agent/tools_todo_task_test.go`:

```go
func TestTodoTool_SummaryBroadcastOnTurnComplete(t *testing.T) {
    tool, bc := newTestTodoTool(t)

    // create two tasks in same turn
    tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "task A"})
    tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "task B"})

    // complete first — no summary yet
    tool.Execute(context.Background(), map[string]any{"action": "update", "task_id": float64(1), "status": "completed"})
    for _, p := range bc.payloads {
        var m map[string]any
        json.Unmarshal(p, &m)
        if m["type"] == "todo_summary" {
            t.Fatal("should not broadcast summary after first task")
        }
    }

    // complete second — summary must fire
    bc.payloads = nil
    tool.Execute(context.Background(), map[string]any{"action": "update", "task_id": float64(2), "status": "completed"})
    found := false
    for _, p := range bc.payloads {
        var m map[string]any
        json.Unmarshal(p, &m)
        if m["type"] == "todo_summary" {
            found = true
            content, _ := m["content"].(string)
            if !strings.Contains(content, "task A") || !strings.Contains(content, "task B") {
                t.Errorf("summary missing tasks: %s", content)
            }
        }
    }
    if !found {
        t.Fatal("expected todo_summary broadcast after all tasks complete")
    }
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/... -run TestTodoTool_SummaryBroadcastOnTurnComplete -v
```

Expected: FAIL (summary not yet broadcast)

- [ ] **Step 3: Update `Create` to write `turn_id`**

In `todo_task_store.go`, update `Create`:

```go
func (s *TodoStore) Create(task *models.Todo) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    blockedBy, _ := json.Marshal(task.BlockedBy)
    if blockedBy == nil {
        blockedBy = []byte("[]")
    }
    now := time.Now().UTC()
    res, err := s.db.Exec(
        `INSERT INTO todo_tasks (conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        task.ConversationID, task.TurnID, task.Subject, task.Description,
        task.Status, task.Owner, string(blockedBy), now, now,
    )
    if err != nil {
        return err
    }
    id, _ := res.LastInsertId()
    task.ID = id
    task.CreatedAt = now
    task.UpdatedAt = now
    logger.Global().Debug().Str("table", "todo_tasks").Str("op", "insert").Int64("task_id", task.ID).Str("conv_id", task.ConversationID).Msg("store")
    return nil
}
```

- [ ] **Step 4: Add `ListByTurn` method**

```go
func (s *TodoStore) ListByTurn(turnID string) ([]*models.Todo, error) {
    rows, err := s.db.Query(
        `SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
         FROM todo_tasks WHERE turn_id = ? AND status != 'deleted' ORDER BY id ASC`,
        turnID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tasks []*models.Todo
    for rows.Next() {
        var t models.Todo
        var blockedByJSON string
        if err := rows.Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
            &t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
            return nil, err
        }
        json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy) //nolint:errcheck
        tasks = append(tasks, &t)
    }
    return tasks, rows.Err()
}
```

- [ ] **Step 5: Update `List` to exclude completed turns**

Replace the `List` method:

```go
func (s *TodoStore) List(conversationID string) ([]*models.Todo, error) {
    // Only return tasks from turns that have at least one non-completed task.
    rows, err := s.db.Query(
        `SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
         FROM todo_tasks
         WHERE conversation_id = ?
           AND status != 'deleted'
           AND turn_id IN (
               SELECT DISTINCT turn_id FROM todo_tasks
               WHERE conversation_id = ?
                 AND status NOT IN ('completed', 'deleted')
           )
         ORDER BY id ASC`,
        conversationID, conversationID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tasks []*models.Todo
    for rows.Next() {
        var t models.Todo
        var blockedByJSON string
        if err := rows.Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
            &t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
            return nil, err
        }
        json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy) //nolint:errcheck
        tasks = append(tasks, &t)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    logger.Global().Debug().Str("table", "todo_tasks").Str("op", "select").Str("conv_id", conversationID).Int("count", len(tasks)).Msg("store")
    return tasks, nil
}
```

- [ ] **Step 6: Update `Update` and `Get` scan to include `turn_id`**

In `Update`, update the SELECT scan:

```go
err = s.db.QueryRow(
    `SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
     FROM todo_tasks WHERE id = ?`, id,
).Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
    &t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
```

In `Get`, same scan update:

```go
err := s.db.QueryRow(
    `SELECT id, conversation_id, turn_id, subject, description, status, owner, blocked_by, created_at, updated_at
     FROM todo_tasks WHERE id = ?`, id,
).Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.Description,
    &t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
```

- [ ] **Step 7: Build**

```bash
go build ./internal/store/... && echo OK
```

Expected: `OK`

- [ ] **Step 8: Commit**

```bash
git add internal/store/todo_task_store.go
git commit -m "feat(store): turn_id support — write on create, filter completed turns from List"
```

---

### Task 4: Update `TodoTool` — inject `turn_id`, broadcast `todo_summary`

**Files:**
- Modify: `internal/agent/tools_todo_task.go`

- [ ] **Step 1: Add `turnID` to `TodoTool` struct and constructor**

```go
type TodoTool struct {
    store          *store.TodoStore
    broadcaster    SSEBroadcaster
    conversationID string
    turnID         string
}

func NewTodoTool(s *store.TodoStore, broadcaster SSEBroadcaster, conversationID, turnID string) *TodoTool {
    return &TodoTool{store: s, broadcaster: broadcaster, conversationID: conversationID, turnID: turnID}
}
```

- [ ] **Step 2: Update `execCreate` to set `TurnID`**

```go
task := &models.Todo{
    ConversationID: t.conversationID,
    TurnID:         t.turnID,
    Subject:        subject,
    Description:    strVal(input, "description"),
    Status:         "pending",
    Owner:          strVal(input, "owner"),
    BlockedBy:      int64Slice(input, "blocked_by"),
}
```

- [ ] **Step 3: Update `allTasksDone` to scope by `turn_id`**

```go
func (t *TodoTool) allTasksDone() bool {
    tasks, err := t.store.ListByTurn(t.turnID)
    if err != nil || len(tasks) == 0 {
        return false
    }
    for _, task := range tasks {
        if task.Status != "completed" && task.Status != "deleted" {
            return false
        }
    }
    return true
}
```

- [ ] **Step 4: Add `broadcastSummary` method**

```go
func (t *TodoTool) broadcastSummary() {
    if t.broadcaster == nil {
        return
    }
    tasks, err := t.store.ListByTurn(t.turnID)
    if err != nil {
        return
    }
    var sb strings.Builder
    sb.WriteString("**Tasks completed:**\n")
    n := 0
    for _, task := range tasks {
        if task.Status == "deleted" {
            continue
        }
        n++
        dur := task.UpdatedAt.Sub(task.CreatedAt).Round(time.Second)
        sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", n, task.Subject, dur))
    }
    payload, _ := json.Marshal(map[string]any{"type": "todo_summary", "content": sb.String()})
    t.broadcaster.BroadcastSSE(t.conversationID, payload)
}
```

- [ ] **Step 5: Call `broadcastSummary` in `execUpdate` before nudge**

Replace the end of `execUpdate`:

```go
t.broadcast(task)
out, _ := json.Marshal(task)

allDone := t.allTasksDone()
if allDone {
    t.broadcastSummary()
}
return &ToolResult{Content: string(out) + todoNudge(allDone), RiskLevel: RiskL1}, nil
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/agent/... -v 2>&1 | tail -20
```

Expected: all pass including `TestTodoTool_SummaryBroadcastOnTurnComplete`

- [ ] **Step 7: Commit**

```bash
git add internal/agent/tools_todo_task.go internal/agent/tools_todo_task_test.go
git commit -m "feat(agent): TodoTool broadcasts todo_summary when turn completes"
```

---

### Task 5: Update `Factory` and `chatSendMessage` — pass `turn_id`

**Files:**
- Modify: `internal/agent/factory.go`
- Modify: `internal/api/chat.go`

- [ ] **Step 1: Add `TurnID` to `Factory` struct**

In `factory.go`, add to the `Factory` struct:

```go
TurnID string
```

- [ ] **Step 2: Pass `TurnID` when constructing `TodoTool`**

In `buildRegistry`:

```go
if f.TodoStore != nil {
    registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID, f.TurnID))
}
```

- [ ] **Step 3: Generate `turn_id` in `chatSendMessage`**

In `chat.go`, after `factory, err := app.NewAgentFactory()`, add:

```go
factory.TurnID = uuid.New().String()
```

Ensure `github.com/google/uuid` is already imported (it is, used elsewhere in the file). If not, add to imports.

- [ ] **Step 4: Build**

```bash
go build ./... && echo OK
```

Expected: `OK`

- [ ] **Step 5: Commit**

```bash
git add internal/agent/factory.go internal/api/chat.go
git commit -m "feat(api): generate turn_id per request, wire into TodoTool"
```

---

### Task 6: Frontend — handle `todo_summary` SSE event

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Find the SSE event handler**

In `ChatView.vue`, locate the section handling `todo_update` (around line 408-412):

```js
if (!todoTasksMap.value[convId]) todoTasksMap.value[convId] = new Map()
todoTasksMap.value[convId].set(task.id, task)
todoTasksMap.value[convId] = todoTasksMap.value[convId]
```

- [ ] **Step 2: Add `todo_summary` handler after `todo_update` block**

Find the SSE message handler and add handling for `todo_summary`. The event arrives as `{ type: "todo_summary", content: "..." }` via the SSE stream. Add after the `todo_update` block:

```js
} else if (parsed.type === 'todo_summary') {
  // convId is in scope from the outer SSE handler (same as todo_update)
  // Append summary as assistant message
  const summaryMsg = {
    id: 'todo-summary-' + Date.now(),
    role: 'assistant',
    content: parsed.content,
    created_at: new Date().toISOString(),
  }
  if (!messages.value[convId]) messages.value[convId] = []
  messages.value[convId] = [...messages.value[convId], summaryMsg]
  // Clear task panel
  todoTasksMap.value[convId] = new Map()
}
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai && npm --prefix web run build 2>&1 | tail -5
```

Expected: build succeeds, no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(frontend): handle todo_summary SSE — append to chat, clear task panel"
```

---

### Task 7: End-to-end verification

- [ ] **Step 1: Build and start server**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data &
sleep 2
```

- [ ] **Step 2: Open browser and send a multi-step message**

Navigate to `http://localhost:8002`, open a conversation, send a message that causes the agent to create and complete multiple Todo tasks.

- [ ] **Step 3: Verify**

1. TASKS panel appears during execution with task progress
2. When last task completes, panel disappears
3. A summary message appears in the chat: `**Tasks completed:** 1. ... (Xs) 2. ...`
4. Refresh page — panel does not reappear

- [ ] **Step 4: Kill test server**

```bash
kill %1
```
