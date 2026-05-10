# Plan: TodoTask Tool Implementation

**Spec:** `docs/spec-20260510-1932-todotask-tool.md`  
**Date:** 2026-05-10

## Phase 1: DB + Model

**Files:**
- `internal/db/schema.go` — add `todo_tasks` table migration
- `internal/models/todo_task.go` — new file

**Steps:**

1. Add to `migrate()` in `schema.go`:
```go
db.Exec(`CREATE TABLE IF NOT EXISTS todo_tasks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT    NOT NULL,
    subject         TEXT    NOT NULL,
    description     TEXT    NOT NULL DEFAULT '',
    status          TEXT    NOT NULL DEFAULT 'pending',
    owner           TEXT    NOT NULL DEFAULT '',
    blocked_by      TEXT    NOT NULL DEFAULT '[]',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
)`)
```

2. Create `internal/models/todo_task.go`:
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

**Verify:** `go build ./...` passes.

---

## Phase 2: TodoTaskStore

**File:** `internal/store/todo_task_store.go` — new file

**Pattern:** follow `memory_store.go` (simple `*sql.DB` wrapper + mutex).

```go
type TodoTaskStore struct {
    db *sql.DB
    mu sync.Mutex
}

func NewTodoTaskStore(db *sql.DB) *TodoTaskStore

func (s *TodoTaskStore) Create(task *models.TodoTask) error
// INSERT, set task.ID from LastInsertId, set task.CreatedAt/UpdatedAt

func (s *TodoTaskStore) Update(id int64, subject, description, status, owner string, blockedBy []int64) error
// Build UPDATE SET dynamically for non-empty fields only; always set updated_at

func (s *TodoTaskStore) List(conversationID string) ([]*models.TodoTask, error)
// SELECT WHERE conversation_id=? AND status != 'deleted' ORDER BY id ASC
// Unmarshal blocked_by JSON into []int64
```

Mutex wraps Create and Update only (reads are safe without lock).

**Verify:** `go test ./internal/store/...` passes.

---

## Phase 3: TodoTaskTool

**File:** `internal/agent/tools_todo_task.go` — new file

```go
type SSEBroadcaster interface {
    BroadcastSSE(conversationID string, data []byte)
}

type TodoTaskTool struct {
    store          *store.TodoTaskStore
    broadcaster    SSEBroadcaster
    conversationID string
}

func NewTodoTaskTool(store *store.TodoTaskStore, broadcaster SSEBroadcaster, conversationID string) *TodoTaskTool
```

`Execute()` logic:
- `action=create`: validate `subject` present → `store.Create()` → broadcast → return `{"id": N}`
- `action=update`: validate `task_id` present + at least one other field → `store.Update()` → fetch updated task → broadcast → return full task JSON
- `action=list`: `store.List()` → return JSON array
- unknown action: return error

Broadcast payload:
```json
{"type": "todotask_update", "content": { /* TodoTask */ }}
```

**Verify:** `go build ./internal/agent/...` passes.

---

## Phase 4: Factory + App Integration

**Files:**
- `internal/agent/factory.go` — add fields + update `NewAgent` signature
- `internal/mcp/server.go` — add `TodoTaskStore` to `App`, wire in `NewAgentFactory`
- `internal/api/chat.go` — update `factory.NewAgent(systemPrompt)` call

**Steps:**

1. `factory.go` — add to `Factory`:
```go
TodoTaskStore  *store.TodoTaskStore
SSEBroadcaster agent.SSEBroadcaster  // avoid import cycle: define in agent pkg
```
Change `NewAgent` signature:
```go
func (f *Factory) NewAgent(systemPrompt string, conversationID string) *Agent
```
Register in `NewAgent`:
```go
if f.TodoTaskStore != nil {
    registry.Register(NewTodoTaskTool(f.TodoTaskStore, f.SSEBroadcaster, conversationID))
}
```

2. `mcp/server.go` — add to `App`:
```go
TodoTaskStore *store.TodoTaskStore
```
In `NewAgentFactory()`:
```go
f.TodoTaskStore = a.TodoTaskStore
f.SSEBroadcaster = a
```

3. `cmd/spider/main.go` (or wherever `App` is initialized) — add:
```go
app.TodoTaskStore = store.NewTodoTaskStore(db)
```

4. `internal/api/chat.go` — update call site (line 144):
```go
a := factory.NewAgent(systemPrompt, id)  // id = conversationID
```

**Verify:** `go build ./...` passes.

---

## Phase 5: Frontend

**Files:**
- `web/src/api/chat.ts` — add `todotask_update` to `ChatEvent` type
- `web/src/views/ChatView.vue` — add state + SSE handler + TodoTaskPanel component inline

**Steps:**

1. `chat.ts` — extend `ChatEvent.type`:
```ts
type: '...' | 'todotask_update'
```

2. `ChatView.vue`:

Add state:
```ts
const todoTasks = ref<Map<number, TodoTask>>(new Map())
```

Add `TodoTask` interface:
```ts
interface TodoTask {
  id: number; conversation_id: string; subject: string
  description?: string; status: string; owner?: string
  blocked_by?: number[]; created_at: string; updated_at: string
}
```

In `handleConvEvent`, add case:
```ts
case 'todotask_update': {
  const task = event.content as TodoTask
  todoTasks.value = new Map(todoTasks.value).set(task.id, task)
  break
}
```

Reset on conversation switch:
```ts
todoTasks.value = new Map()
```

3. Add `TodoTaskPanel` component in template, sticky above input:
```html
<div v-if="todoTasks.size > 0" class="todo-panel">
  <div class="todo-header">
    TASKS {{ completedCount }}/{{ todoTasks.size }}
  </div>
  <div v-for="task in sortedTasks" :key="task.id" class="todo-row" :class="taskClass(task)">
    <span class="todo-icon">{{ taskIcon(task) }}</span>
    <span class="todo-subject">{{ task.subject }}</span>
  </div>
</div>
```

CSS (scoped, using existing CSS vars):
```css
.todo-panel {
  border: 1px solid var(--border);
  border-left: 3px solid var(--primary);
  border-radius: 6px;
  background: var(--input-bg);
  margin-bottom: 8px;
  font-family: 'SF Mono', monospace;
  font-size: 12px;
}
.todo-header {
  padding: 5px 10px;
  font-size: 10px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.8px;
  color: var(--primary);
  border-bottom: 1px solid var(--border);
}
.todo-row { display: flex; align-items: center; gap: 8px; padding: 4px 10px; }
.todo-icon { width: 14px; text-align: center; flex-shrink: 0; }
.todo-subject { flex: 1; }
.todo-row.status-completed .todo-subject { color: var(--muted); }
.todo-row.status-completed .todo-icon { color: var(--green); }
.todo-row.status-in_progress .todo-icon { color: var(--primary); animation: blink 1s step-end infinite; }
.todo-row.status-in_progress .todo-subject { color: var(--text-sub); }
.todo-row.status-pending .todo-icon,
.todo-row.status-pending .todo-subject { color: var(--muted); }
.todo-row.blocked .todo-subject,
.todo-row.blocked .todo-icon { color: var(--muted); opacity: 0.5; }
```

**Verify:** `npm run build` in `web/` passes. Start dev server, send a message that triggers TodoTask tool calls, verify card appears and updates in real time.

---

## Completion Criteria

- [ ] `go test ./...` passes
- [ ] `npm run build` passes
- [ ] Agent creates tasks → card appears sticky above input
- [ ] Status updates reflect in real time without page refresh
- [ ] Empty update (task_id only) returns error to LLM
- [ ] Parallel tool calls don't cause SQLITE_BUSY
