# Todo Panel UX Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the todo panel with group sorting, folding, in-progress highlighting, per-task timers, token display, and `active_form` field.

**Architecture:** Backend adds `active_form` column + `turn_usage` SSE event; frontend rewrites todo panel rendering with computed groups and timer management.

**Tech Stack:** Go (backend), Vue 3 + TypeScript (frontend), SQLite

---

## Task 1: Backend — `active_form` field in model + DB

**Files:**
- Modify: `internal/models/todo_task.go`
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add `ActiveForm` to model**

```go
// internal/models/todo_task.go
type Todo struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversation_id"`
	TurnID         string    `json:"turn_id"`
	Subject        string    `json:"subject"`
	ActiveForm     string    `json:"active_form,omitempty"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner,omitempty"`
	BlockedBy      []int64   `json:"blocked_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Add DB migration for `active_form` column**

In `internal/db/schema.go`, inside `migrate()`, after the existing `ALTER TABLE todo_tasks ADD COLUMN turn_id` line, add:

```go
db.Exec(`ALTER TABLE todo_tasks ADD COLUMN active_form TEXT NOT NULL DEFAULT ''`)
```

- [ ] **Step 3: Build to verify**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/models/todo_task.go internal/db/schema.go
git commit -m "feat(models): add active_form field to Todo + DB migration"
```

---

## Task 2: Backend — Wire `active_form` through store

**Files:**
- Modify: `internal/store/todo_task_store.go`

- [ ] **Step 1: Update `Create` to insert `active_form`**

Replace the INSERT in `Create()`:

```go
res, err := s.db.Exec(
    `INSERT INTO todo_tasks (conversation_id, turn_id, subject, active_form, description, status, owner, blocked_by, created_at, updated_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    task.ConversationID, task.TurnID, task.Subject, task.ActiveForm, task.Description,
    task.Status, task.Owner, string(blockedBy), now, now,
)
```

- [ ] **Step 2: Update `Update` to accept and set `active_form`**

Change the function signature:

```go
func (s *TodoStore) Update(conversationID string, id int64, subject, activeForm, description, status, owner string, blockedBy []int64) (*models.Todo, error) {
```

Add after `if subject != ""`block:

```go
if activeForm != "" {
    setClauses = append(setClauses, "active_form = ?")
    args = append(args, activeForm)
}
```

- [ ] **Step 3: Update all Scan calls to include `active_form`**

In `Update` (QueryRow after UPDATE), `List`, `ListByTurn`, and `Get`, update SELECT and Scan:

```go
// SELECT query — add active_form after subject:
`SELECT id, conversation_id, turn_id, subject, active_form, description, status, owner, blocked_by, created_at, updated_at
 FROM todo_tasks ...`

// Scan — add &t.ActiveForm after &t.Subject:
rows.Scan(&t.ID, &t.ConversationID, &t.TurnID, &t.Subject, &t.ActiveForm,
    &t.Description, &t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
```

Apply this pattern to all four query sites: `Update` (QueryRow), `List`, `ListByTurn`, `Get`.

- [ ] **Step 4: Build and run store tests**

```bash
go build ./...
go test ./internal/store/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/store/todo_task_store.go
git commit -m "feat(store): wire active_form through Create/Update/List/Get"
```

---

## Task 3: Backend — Wire `active_form` through TodoTool

**Files:**
- Modify: `internal/agent/tools_todo_task.go`

- [ ] **Step 1: Add `active_form` to InputSchema**

In `InputSchema()`, add to `properties`:

```go
"active_form": map[string]any{"type": "string"},
```

- [ ] **Step 2: Wire `active_form` in `execCreate`**

```go
task := &models.Todo{
    ConversationID: t.conversationID,
    TurnID:         t.turnID,
    Subject:        subject,
    ActiveForm:     strVal(input, "active_form"),
    Description:    strVal(input, "description"),
    Status:         "pending",
    Owner:          strVal(input, "owner"),
    BlockedBy:      int64Slice(input, "blocked_by"),
}
```

- [ ] **Step 3: Wire `active_form` in `execUpdate`**

After `owner := strVal(input, "owner")`, add:

```go
activeForm := strVal(input, "active_form")
```

Update the empty-check condition:

```go
if subject == "" && activeForm == "" && description == "" && status == "" && owner == "" && blockedBy == nil {
```

Update the `store.Update` call:

```go
task, err := t.store.Update(t.conversationID, taskID, subject, activeForm, description, status, owner, blockedBy)
```

- [ ] **Step 4: Update `todoPromptSection` to mention `active_form`**

In the `<example>` block, show `active_form` usage:

```go
const todoPromptSection = `## Task Management (Todo tool)
...
**Rules:**
- Mark a task in_progress BEFORE beginning work on it
- Only ONE task in_progress at a time
- Mark completed IMMEDIATELY after finishing — do not batch completions
- Only mark completed when fully done; if blocked, keep in_progress and create a new task describing the blocker
- Provide active_form (present-continuous) when creating tasks, e.g. subject="Update TodoStore" active_form="Updating TodoStore"
...`
```

- [ ] **Step 5: Build and test**

```bash
go build ./...
go test ./internal/agent/... -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_todo_task.go
git commit -m "feat(agent): wire active_form through TodoTool create/update + prompt"
```

---

## Task 4: Backend — `turn_usage` SSE event

**Files:**
- Modify: `internal/agent/agent.go`

- [ ] **Step 1: Add `EventTurnUsage` constant**

In the `EventType` constants block (around line 31):

```go
EventTurnUsage       EventType = "turn_usage"
```

- [ ] **Step 2: Emit `turn_usage` after each LLM turn**

There are two "turn done" paths in `agent.go`:

**Path A** — no tool calls (final turn), around line 266:
```go
events <- Event{Type: EventDone}
```
Before this line, add:
```go
events <- Event{Type: EventTurnUsage, Content: map[string]any{
    "input_tokens":  usage.InputTokens,
    "output_tokens": usage.OutputTokens,
}}
```

**Path B** — turn with tool calls, around line 357 (after `log.Debug` "turn done"):
```go
// after: log.Debug().Int("turn", turn)...Msg("turn done")
events <- Event{Type: EventTurnUsage, Content: map[string]any{
    "input_tokens":  usage.InputTokens,
    "output_tokens": usage.OutputTokens,
}}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/agent.go
git commit -m "feat(agent): emit turn_usage SSE event after each LLM turn"
```

---

## Task 5: Frontend — API types

**Files:**
- Modify: `web/src/api/chat.ts`

- [ ] **Step 1: Add `turn_usage` to ChatEvent and `active_form` to Todo**

```typescript
export interface ChatEvent {
  type: 'text_delta' | 'tool_start' | 'tool_result' | 'confirm_required' | 'error' | 'done' | 'message' | 'todo_update' | 'turn_usage'
  content?: Record<string, any>
}

export interface Todo {
  id: number; conversation_id: string; subject: string; active_form?: string
  description?: string; status: string; owner?: string
  blocked_by?: number[]; created_at: string; updated_at: string
}
```

- [ ] **Step 2: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai
npm --prefix web run build 2>&1 | tail -5
```

Expected: no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/api/chat.ts
git commit -m "feat(api): add turn_usage event type and active_form to Todo"
```

---

## Task 6: Frontend — Todo panel rewrite

**Files:**
- Modify: `web/src/views/ChatView.vue`

This is the main frontend task. Make changes in sequence.

### Step 1: Add state refs

After `const todoTasksMap = ref<Record<string, Map<number, Todo>>>({})` (line 116), add:

- [ ] Add state:

```typescript
const completedFolded = ref(true)
const pendingFolded = ref(true)
const turnUsage = ref<{input: number; output: number} | null>(null)
const taskTimers = ref<Map<number, ReturnType<typeof setInterval>>>(new Map())
const taskElapsed = ref<Map<number, number>>(new Map())
```

### Step 2: Replace `sortedTasks` computed with grouped computed

- [ ] Replace lines 117-118:

```typescript
const inProgressTasks = computed(() =>
  allTasks.value.filter(t => t.status === 'in_progress').sort((a, b) => a.id - b.id)
)
const pendingTasks = computed(() =>
  allTasks.value.filter(t => t.status === 'pending').sort((a, b) => a.id - b.id)
)
const completedTasks = computed(() =>
  allTasks.value.filter(t => t.status === 'completed').sort((a, b) => a.id - b.id)
)
const allTasks = computed(() =>
  Array.from((todoTasksMap.value[activeConvId.value ?? ''] ?? new Map<number, Todo>()).values())
)
const visiblePending = computed(() =>
  pendingFolded.value ? pendingTasks.value.slice(0, 2) : pendingTasks.value
)
const hiddenPendingCount = computed(() =>
  pendingTasks.value.length - visiblePending.value.length
)
const visibleCompleted = computed(() =>
  completedFolded.value ? [] : completedTasks.value
)
const hasTasks = computed(() =>
  allTasks.value.length > 0
)
const panelHeader = computed(() => {
  const active = inProgressTasks.value[0]
  const tokenSuffix = turnUsage.value
    ? '  ↓ ' + fmtTokens(turnUsage.value.output)
    : ''
  if (active) {
    const label = active.active_form ?? active.subject
    const elapsed = taskElapsed.value.get(active.id) ?? 0
    return label + '… (' + fmtElapsed(elapsed) + ')' + tokenSuffix
  }
  const total = allTasks.value.length
  const done = completedTasks.value.length
  return `TASKS ${done}/${total}` + tokenSuffix
})
```

### Step 3: Add helper functions

- [ ] Replace `taskIcon`, `taskClass`, `completedCount` with:

```typescript
function taskIcon(t: Todo) {
  if (t.status === 'completed') return '✓'
  if (t.status === 'in_progress') return '●'
  return '○'
}
function taskClass(t: Todo) { return `todo-${t.status}` }

function fmtElapsed(seconds: number): string {
  if (seconds >= 60) {
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}m ${s}s`
  }
  return `${seconds}s`
}

function fmtTokens(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(n)
}

function startTimer(task: Todo) {
  if (taskTimers.value.has(task.id)) return
  const startTime = new Date(task.updated_at).getTime()
  const tick = () => {
    taskElapsed.value.set(task.id, Math.floor((Date.now() - startTime) / 1000))
    taskElapsed.value = new Map(taskElapsed.value)
  }
  tick()
  taskTimers.value.set(task.id, setInterval(tick, 1000))
}

function stopTimer(taskId: number) {
  const t = taskTimers.value.get(taskId)
  if (t) { clearInterval(t); taskTimers.value.delete(taskId) }
  taskElapsed.value.delete(taskId)
}

function clearAllTimers() {
  taskTimers.value.forEach(t => clearInterval(t))
  taskTimers.value.clear()
  taskElapsed.value.clear()
}
```

### Step 4: Handle timer lifecycle in `handleConvEvent`

- [ ] In `case 'todo_update':` (around line 444), after updating the map, add timer management:

```typescript
case 'todo_update': {
  const task = event.content as Todo
  if (!todoTasksMap.value[convId]) todoTasksMap.value[convId] = new Map()
  todoTasksMap.value[convId].set(task.id, task)
  todoTasksMap.value[convId] = todoTasksMap.value[convId]
  // timer management
  if (task.status === 'in_progress') {
    startTimer(task)
  } else {
    stopTimer(task.id)
  }
  break
}
```

- [ ] Add `turn_usage` case after `todo_update`:

```typescript
case 'turn_usage': {
  const u = event.content as { input_tokens: number; output_tokens: number }
  turnUsage.value = { input: u.input_tokens, output: u.output_tokens }
  break
}
```

### Step 5: Reset state on conversation switch and message send

- [ ] In `selectConversation`, after clearing `todoTasksMap` (or after setting new conv), add:

```typescript
clearAllTimers()
turnUsage.value = null
completedFolded.value = true
pendingFolded.value = true
// reinitialize timers for any in_progress tasks loaded from API
for (const task of (todoTasksMap.value[id] ?? new Map()).values()) {
  if (task.status === 'in_progress') startTimer(task)
}
```

- [ ] In `send()`, after setting `isStreaming = true`, add:

```typescript
turnUsage.value = null
```

### Step 6: Rewrite template

- [ ] Replace the todo panel template (around lines 843-849):

```html
<div v-if="hasTasks" class="todo-panel">
  <div class="todo-header">{{ panelHeader }}</div>

  <!-- in_progress tasks -->
  <div v-for="task in inProgressTasks" :key="task.id" class="todo-row todo-in_progress">
    <span class="todo-icon">●</span>
    <span class="todo-subject">{{ task.subject }}</span>
  </div>

  <!-- pending tasks (max 2 visible) -->
  <div v-for="task in visiblePending" :key="task.id" class="todo-row todo-pending">
    <span class="todo-icon">○</span>
    <span class="todo-subject">{{ task.subject }}</span>
  </div>
  <div v-if="hiddenPendingCount > 0" class="todo-row todo-fold" @click="pendingFolded = !pendingFolded">
    <span class="todo-icon">○</span>
    <span class="todo-subject">+{{ hiddenPendingCount }} more{{ pendingFolded ? '' : ' ▲' }}</span>
  </div>

  <!-- completed fold toggle -->
  <div v-if="completedTasks.length > 0" class="todo-row todo-fold" @click="completedFolded = !completedFolded">
    <span class="todo-icon">✓</span>
    <span class="todo-subject">+{{ completedTasks.length }} completed{{ completedFolded ? '' : ' ▲' }}</span>
  </div>
  <!-- expanded completed tasks -->
  <div v-for="task in visibleCompleted" :key="task.id" class="todo-row todo-completed todo-completed-indent">
    <span class="todo-icon">✓</span>
    <span class="todo-subject">{{ task.subject }}</span>
  </div>
</div>
```

### Step 7: Update CSS

- [ ] Add/update CSS (around line 998):

```css
.todo-panel { margin: 0 16px 8px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); font-family: 'SF Mono', monospace; font-size: 12px; overflow: hidden; }
.todo-header { padding: 5px 10px; color: var(--text-sub); font-size: 11px; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); }
.todo-row { display: flex; align-items: center; gap: 8px; padding: 4px 10px; color: var(--text-sub); }
.todo-row + .todo-row { border-top: 1px solid var(--border); }
.todo-icon { width: 14px; text-align: center; flex-shrink: 0; }
.todo-subject { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.todo-pending .todo-icon { color: var(--text-sub); }
.todo-in_progress { border-left: 2px solid var(--primary); background: rgba(99,102,241,0.06); }
.todo-in_progress .todo-icon { color: var(--primary); }
.todo-in_progress .todo-subject { color: var(--text); font-weight: 500; }
.todo-completed .todo-icon { color: var(--green, #4caf50); }
.todo-completed .todo-subject { color: var(--text-sub); text-decoration: line-through; }
.todo-completed-indent { padding-left: 20px; }
.todo-fold { cursor: pointer; }
.todo-fold:hover { background: var(--surface-2, rgba(255,255,255,0.04)); }
.todo-fold .todo-icon { color: var(--text-sub); }
```

- [ ] **Step 8: Build and verify**

```bash
npm --prefix web run build 2>&1 | tail -10
```

Expected: no TypeScript errors, build succeeds.

- [ ] **Step 9: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(frontend): todo panel — group sort, fold, highlight, timer, token display, active_form"
```

---

## Task 7: Integration test

- [ ] **Step 1: Start server**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: Open browser, start a conversation with a multi-step task**

Ask agent something that triggers 4+ todo tasks.

- [ ] **Step 3: Verify**

- [ ] `in_progress` task has left border + background tint
- [ ] Panel header shows `active_form… (Xs)` while task running
- [ ] Timer increments each second
- [ ] Pending tasks: only 2 shown, "+N more" clickable
- [ ] Completed tasks collapsed, "+N completed" clickable to expand
- [ ] After turn ends: header shows `TASKS N/M ↓ Xk`
- [ ] On conversation switch: timers reset, folds reset

- [ ] **Step 4: Commit if any fixes needed**

```bash
git add -A
git commit -m "fix(frontend): todo panel integration fixes"
```
