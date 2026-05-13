# Task Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add persistent automated tasks — TodoTask→Todo rename, Task/TaskRun models, cron scheduler, CreateTask agent tool, REST API, and Vue frontend.

**Architecture:** Two independent subsystems. (1) Pure rename: TodoTask→Todo across Go types, DB table, SSE events, and frontend. (2) New Task feature: DB tables `tasks`+`task_runs`, TaskStore CRUD, REST API, in-process cron scheduler that runs headless agents, CreateTask agent tool, and TasksView.vue. Scheduler uses a simple built-in cron matcher (no new deps).

**Tech Stack:** Go (zerolog, modernc/sqlite), Vue 3 Composition API, existing llm.Client for post-run summaries.

---

## File Map

**Rename (Task 1):**
- Rename: `internal/models/todo_task.go` → `internal/models/todo.go` (struct `TodoTask`→`Todo`)
- Rename: `internal/store/todo_task_store.go` → `internal/store/todo_store.go` (struct `TodoTaskStore`→`TodoStore`)
- Rename: `internal/store/todo_task_store_test.go` → `internal/store/todo_store_test.go`
- Rename: `internal/agent/tools_todo_task.go` → `internal/agent/tools_todo.go` (struct `TodoTaskTool`→`TodoTool`)
- Rename: `internal/agent/tools_todo_task_test.go` → `internal/agent/tools_todo_test.go`
- Modify: `internal/db/schema.go` — add `todos` table, update migrate()
- Modify: `internal/mcp/server.go` — `TodoTaskStore`→`TodoStore`
- Modify: `internal/agent/factory.go` — `TodoTaskStore`→`TodoStore`, `TodoTaskTool`→`TodoTool`
- Modify: `cmd/spider/main.go` — `NewTodoTaskStore`→`NewTodoStore`
- Modify: `internal/api/chat.go` — `todo_tasks`→`todos` in response, `TodoTaskStore`→`TodoStore`
- Modify: `web/src/api/chat.ts` — `TodoTask`→`Todo`, `todo_tasks`→`todos`, event type
- Modify: `web/src/views/ChatView.vue` — `TodoTask`→`Todo`, `todo_tasks`→`todos`, event type

- [ ] **Step 1: Rename model file and struct**

Delete `internal/models/todo_task.go`. Create `internal/models/todo.go`:

```go
package models

import "time"

type Todo struct {
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

- [ ] **Step 2: Rename store file and struct**

Delete `internal/store/todo_task_store.go`. Create `internal/store/todo_store.go` — same content as `todo_task_store.go` but with these changes:
- `type TodoTaskStore` → `type TodoStore`
- `func NewTodoTaskStore` → `func NewTodoStore`
- `func (s *TodoTaskStore)` → `func (s *TodoStore)` (all methods)
- `models.TodoTask` → `models.Todo`
- All SQL `todo_tasks` → `todos`
- All log `"table", "todo_tasks"` → `"table", "todos"`

Full file content (continue from Create, add remaining methods):

```go
func (s *TodoStore) Update(conversationID string, id int64, subject, description, status, owner string, blockedBy []int64) (*models.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	setClauses := []string{"updated_at = ?"}
	args := []any{now}

	if subject != "" {
		setClauses = append(setClauses, "subject = ?")
		args = append(args, subject)
	}
	if description != "" {
		setClauses = append(setClauses, "description = ?")
		args = append(args, description)
	}
	if status != "" {
		setClauses = append(setClauses, "status = ?")
		args = append(args, status)
	}
	if owner != "" {
		setClauses = append(setClauses, "owner = ?")
		args = append(args, owner)
	}
	if blockedBy != nil {
		b, _ := json.Marshal(blockedBy)
		setClauses = append(setClauses, "blocked_by = ?")
		args = append(args, string(b))
	}

	args = append(args, id, conversationID)
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE todos SET %s WHERE id = ? AND conversation_id = ?", strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return nil, err
	}

	var t models.Todo
	var blockedByJSON string
	err = s.db.QueryRow(
		`SELECT id, conversation_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todos WHERE id = ?`, id,
	).Scan(&t.ID, &t.ConversationID, &t.Subject, &t.Description,
		&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy) //nolint:errcheck
	logger.Global().Debug().Str("table", "todos").Str("op", "update").Int64("task_id", id).Str("status", status).Msg("store")
	return &t, nil
}

func (s *TodoStore) List(conversationID string) ([]*models.Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todos WHERE conversation_id = ? AND status != 'deleted' ORDER BY id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Todo
	for rows.Next() {
		var t models.Todo
		var blockedByJSON string
		if err := rows.Scan(&t.ID, &t.ConversationID, &t.Subject, &t.Description,
			&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy) //nolint:errcheck
		tasks = append(tasks, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	logger.Global().Debug().Str("table", "todos").Str("op", "select").Str("conv_id", conversationID).Int("count", len(tasks)).Msg("store")
	return tasks, nil
}

func (s *TodoStore) Get(id int64) (*models.Todo, error) {
	var t models.Todo
	var blockedByJSON string
	err := s.db.QueryRow(
		`SELECT id, conversation_id, subject, description, status, owner, blocked_by, created_at, updated_at
		 FROM todos WHERE id = ?`, id,
	).Scan(&t.ID, &t.ConversationID, &t.Subject, &t.Description,
		&t.Status, &t.Owner, &blockedByJSON, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(blockedByJSON), &t.BlockedBy) //nolint:errcheck
	return &t, nil
}
```

- [ ] **Step 3: Rename store test file**

Delete `internal/store/todo_task_store_test.go`. Create `internal/store/todo_store_test.go` — same content but replace all `TodoTaskStore` → `TodoStore`, `NewTodoTaskStore` → `NewTodoStore`, `TodoTask{` → `Todo{`, `models.TodoTask` → `models.Todo`, function names `TestTodoTaskStore_*` → `TestTodoStore_*`.

- [ ] **Step 4: Rename agent tool file**

Delete `internal/agent/tools_todo_task.go`. Create `internal/agent/tools_todo.go` with these changes from the original:
- `type TodoTaskTool struct` → `type TodoTool struct`
- `func NewTodoTaskTool` → `func NewTodoTool`
- `func (t *TodoTaskTool)` → `func (t *TodoTool)` (all methods)
- `store *store.TodoTaskStore` → `store *store.TodoStore`
- `func (t *TodoTool) Name() string { return "Todo" }` (was "TodoTask")
- `models.TodoTask` → `models.Todo`
- `[]*models.TodoTask{}` → `[]*models.Todo{}`
- SSE payload type: `"todotask_update"` → `"todo_update"`
- `const todoBaseNudge` — update text: replace "TodoTask tool" → "Todo tool"
- `todoTaskPromptSection` → `todoPromptSection`, update text: "TodoTask tool" → "Todo tool", "TodoTask" → "Todo"
- `func (t *TodoTool) SystemPromptSection() string { return todoPromptSection }`

- [ ] **Step 5: Rename agent tool test file**

Delete `internal/agent/tools_todo_task_test.go`. Create `internal/agent/tools_todo_test.go` — same content but:
- `newTestTodoTool` → `newTestTodoTool` (keep name, it's internal)
- `TodoTaskTool` → `TodoTool`
- `NewTodoTaskTool` → `NewTodoTool`
- `store.NewTodoTaskStore` → `store.NewTodoStore`
- `TestTodoTaskTool_*` → `TestTodoTool_*`

- [ ] **Step 6: Add `todos` table to schema.go**

In `internal/db/schema.go`, in `migrate()`, find the `todo_tasks` CREATE TABLE block and add a `todos` table immediately after it:

```go
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT    NOT NULL,
		subject         TEXT    NOT NULL,
		description     TEXT    NOT NULL DEFAULT '',
		status          TEXT    NOT NULL DEFAULT 'pending',
		owner           TEXT    NOT NULL DEFAULT '',
		blocked_by      TEXT    NOT NULL DEFAULT '[]',
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}
```

- [ ] **Step 7: Update mcp/server.go**

In `internal/mcp/server.go`:
- Change `TodoTaskStore  *store.TodoTaskStore` → `TodoStore  *store.TodoStore`
- Change `f.TodoTaskStore = a.TodoTaskStore` → `f.TodoStore = a.TodoStore`

- [ ] **Step 8: Update agent/factory.go**

In `internal/agent/factory.go`:
- Change `TodoTaskStore  *store.TodoTaskStore` → `TodoStore  *store.TodoStore`
- Change `if f.TodoTaskStore != nil {` → `if f.TodoStore != nil {`
- Change `registry.Register(NewTodoTaskTool(f.TodoTaskStore, f.SSEBroadcaster, conversationID))` → `registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID))`

- [ ] **Step 9: Update cmd/spider/main.go**

In `cmd/spider/main.go`:
- Change `app.TodoTaskStore = store.NewTodoTaskStore(database)` → `app.TodoStore = store.NewTodoStore(database)`

- [ ] **Step 10: Update internal/api/chat.go**

In `internal/api/chat.go`:
- Change `app.TodoTaskStore.List(id)` → `app.TodoStore.List(id)`
- Change `tasks = []*models.TodoTask{}` → `tasks = []*models.Todo{}`
- Change `"todo_tasks": tasks` → `"todos": tasks`
- Update import if needed: `models.TodoTask` → `models.Todo`

- [ ] **Step 11: Update web/src/api/chat.ts**

In `web/src/api/chat.ts`:
- Rename `interface TodoTask` → `interface Todo`
- Change `todo_tasks: TodoTask[]` → `todos: Todo[]` in `getConversation` return type
- Change event type in `ChatEvent`: `'todotask_update'` → `'todo_update'`

- [ ] **Step 12: Update web/src/views/ChatView.vue**

In `web/src/views/ChatView.vue`:
- Change import: `type TodoTask` → `type Todo`
- Change `todoTasksMap = ref<Record<string, Map<number, TodoTask>>>` → `todosMap = ref<Record<string, Map<number, Todo>>>`
- Change `sortedTasks = computed(...)` — update to use `todosMap` and `Todo` type
- Change `taskIcon(t: TodoTask)` → `taskIcon(t: Todo)`
- Change `taskClass(t: TodoTask)` → `taskClass(t: Todo)`
- Change `data.todo_tasks` → `data.todos`
- Change `case 'todotask_update':` → `case 'todo_update':`
- Change `event.content as TodoTask` → `event.content as Todo`

- [ ] **Step 13: Build and test**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./...
go test ./internal/store/... ./internal/agent/...
```

Expected: all pass, no compile errors.

- [ ] **Step 14: Commit**

```bash
git add internal/models/todo.go internal/store/todo_store.go internal/store/todo_store_test.go
git add internal/agent/tools_todo.go internal/agent/tools_todo_test.go
git add internal/db/schema.go internal/mcp/server.go internal/agent/factory.go
git add cmd/spider/main.go internal/api/chat.go
git add web/src/api/chat.ts web/src/views/ChatView.vue
git rm internal/models/todo_task.go internal/store/todo_task_store.go internal/store/todo_task_store_test.go
git rm internal/agent/tools_todo_task.go internal/agent/tools_todo_task_test.go
git commit -m "refactor: rename TodoTask→Todo across models, store, agent tool, and frontend"
```

---

## Task 2: Task + TaskRun models + DB schema

**Files:**
- Create: `internal/models/task.go`
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Create internal/models/task.go**

```go
package models

import "time"

type Task struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Goal         string    `json:"goal"`
	HostIDs      []int64   `json:"host_ids"`
	TriggerType  string    `json:"trigger_type"`  // "cron" | "manual"
	Schedule     string    `json:"schedule"`
	Status       string    `json:"status"`        // "active" | "paused" | "archived"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SourceConvID string    `json:"source_conv_id"`
}

type TaskRun struct {
	ID         int64      `json:"id"`
	TaskID     int64      `json:"task_id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Status     string     `json:"status"`     // "running" | "success" | "failed"
	RawOutput  string     `json:"raw_output"`
	Summary    string     `json:"summary"`
}
```

- [ ] **Step 2: Add tasks + task_runs tables to schema.go**

In `internal/db/schema.go`, in `migrate()`, after the `todos` CREATE TABLE block, add:

```go
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		name           TEXT    NOT NULL,
		goal           TEXT    NOT NULL DEFAULT '',
		host_ids       TEXT    NOT NULL DEFAULT '[]',
		trigger_type   TEXT    NOT NULL DEFAULT 'manual',
		schedule       TEXT    NOT NULL DEFAULT '',
		status         TEXT    NOT NULL DEFAULT 'active',
		source_conv_id TEXT    NOT NULL DEFAULT '',
		created_at     DATETIME NOT NULL,
		updated_at     DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS task_runs (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id     INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		started_at  DATETIME NOT NULL,
		finished_at DATETIME,
		status      TEXT    NOT NULL DEFAULT 'running',
		raw_output  TEXT    NOT NULL DEFAULT '',
		summary     TEXT    NOT NULL DEFAULT ''
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_runs_task_id ON task_runs(task_id)`); err != nil {
		return err
	}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: compiles clean.

- [ ] **Step 4: Commit**

```bash
git add internal/models/task.go internal/db/schema.go
git commit -m "feat(models): add Task, TaskRun structs and DB schema"
```

---

## Task 3: TaskStore CRUD

**Files:**
- Create: `internal/store/task_store.go`
- Create: `internal/store/task_store_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/store/task_store_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/spiderai/spider/internal/models"
)

func TestTaskStore_CreateAndList(t *testing.T) {
	s := NewTaskStore(setupTestDB(t))

	task := &models.Task{Name: "daily check", Goal: "check disk", TriggerType: "manual", Status: "active"}
	if err := s.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == 0 {
		t.Fatal("expected ID to be set")
	}

	tasks, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Name != "daily check" {
		t.Errorf("unexpected tasks: %+v", tasks)
	}
}

func TestTaskStore_GetAndUpdate(t *testing.T) {
	s := NewTaskStore(setupTestDB(t))

	task := &models.Task{Name: "t1", Goal: "g1", TriggerType: "cron", Schedule: "0 2 * * *", Status: "active"}
	s.Create(task)

	got, err := s.Get(task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Schedule != "0 2 * * *" {
		t.Errorf("expected schedule, got %s", got.Schedule)
	}

	got.Status = "paused"
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := s.Get(task.ID)
	if got2.Status != "paused" {
		t.Errorf("expected paused, got %s", got2.Status)
	}
}

func TestTaskStore_Delete(t *testing.T) {
	s := NewTaskStore(setupTestDB(t))

	task := &models.Task{Name: "t1", Goal: "g1", TriggerType: "manual", Status: "active"}
	s.Create(task)
	if err := s.Delete(task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	tasks, _ := s.List()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestTaskStore_TaskRuns(t *testing.T) {
	s := NewTaskStore(setupTestDB(t))

	task := &models.Task{Name: "t1", Goal: "g1", TriggerType: "manual", Status: "active"}
	s.Create(task)

	run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now(), Status: "running"}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.ID == 0 {
		t.Fatal("expected run ID to be set")
	}

	now := time.Now()
	run.FinishedAt = &now
	run.Status = "success"
	run.RawOutput = "output"
	run.Summary = "all good"
	if err := s.UpdateRun(run); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}

	runs, err := s.ListRuns(task.ID)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "success" {
		t.Errorf("unexpected runs: %+v", runs)
	}
}

func TestTaskStore_ListActive(t *testing.T) {
	s := NewTaskStore(setupTestDB(t))

	s.Create(&models.Task{Name: "active", Goal: "g", TriggerType: "cron", Schedule: "* * * * *", Status: "active"})
	s.Create(&models.Task{Name: "paused", Goal: "g", TriggerType: "cron", Schedule: "* * * * *", Status: "paused"})

	tasks, _ := s.ListActiveCron()
	if len(tasks) != 1 || tasks[0].Name != "active" {
		t.Errorf("expected 1 active cron task, got %+v", tasks)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/store/... -run TestTaskStore -v
```

Expected: FAIL — `NewTaskStore` undefined.

- [ ] **Step 3: Create internal/store/task_store.go**

```go
package store

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type TaskStore struct {
	db *sql.DB
	mu sync.Mutex
}

func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{db: db}
}

func (s *TaskStore) Create(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hostIDs, _ := json.Marshal(task.HostIDs)
	if hostIDs == nil {
		hostIDs = []byte("[]")
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO tasks (name, goal, host_ids, trigger_type, schedule, status, source_conv_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Name, task.Goal, string(hostIDs), task.TriggerType, task.Schedule,
		task.Status, task.SourceConvID, now, now,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now
	return nil
}

func (s *TaskStore) Get(id int64) (*models.Task, error) {
	var t models.Task
	var hostIDsJSON string
	err := s.db.QueryRow(
		`SELECT id, name, goal, host_ids, trigger_type, schedule, status, source_conv_id, created_at, updated_at
		 FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Goal, &hostIDsJSON, &t.TriggerType, &t.Schedule,
		&t.Status, &t.SourceConvID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(hostIDsJSON), &t.HostIDs) //nolint:errcheck
	return &t, nil
}

func (s *TaskStore) List() ([]*models.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, name, goal, host_ids, trigger_type, schedule, status, source_conv_id, created_at, updated_at
		 FROM tasks WHERE status != 'archived' ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanTasks(rows)
}

func (s *TaskStore) ListActiveCron() ([]*models.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, name, goal, host_ids, trigger_type, schedule, status, source_conv_id, created_at, updated_at
		 FROM tasks WHERE status = 'active' AND trigger_type = 'cron'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanTasks(rows)
}
```

Continue `task_store.go` (remaining methods):

```go
func (s *TaskStore) Update(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hostIDs, _ := json.Marshal(task.HostIDs)
	if hostIDs == nil {
		hostIDs = []byte("[]")
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE tasks SET name=?, goal=?, host_ids=?, trigger_type=?, schedule=?, status=?, updated_at=? WHERE id=?`,
		task.Name, task.Goal, string(hostIDs), task.TriggerType, task.Schedule, task.Status, now, task.ID,
	)
	if err != nil {
		return err
	}
	task.UpdatedAt = now
	return nil
}

func (s *TaskStore) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (s *TaskStore) CreateRun(run *models.TaskRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(
		`INSERT INTO task_runs (task_id, started_at, status) VALUES (?, ?, ?)`,
		run.TaskID, run.StartedAt, run.Status,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	run.ID = id
	return nil
}

func (s *TaskStore) UpdateRun(run *models.TaskRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`UPDATE task_runs SET finished_at=?, status=?, raw_output=?, summary=? WHERE id=?`,
		run.FinishedAt, run.Status, run.RawOutput, run.Summary, run.ID,
	)
	return err
}

func (s *TaskStore) ListRuns(taskID int64) ([]*models.TaskRun, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, started_at, finished_at, status, raw_output, summary
		 FROM task_runs WHERE task_id = ? ORDER BY id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*models.TaskRun
	for rows.Next() {
		var r models.TaskRun
		if err := rows.Scan(&r.ID, &r.TaskID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.RawOutput, &r.Summary); err != nil {
			return nil, err
		}
		runs = append(runs, &r)
	}
	return runs, rows.Err()
}

func (s *TaskStore) GetRun(id int64) (*models.TaskRun, error) {
	var r models.TaskRun
	err := s.db.QueryRow(
		`SELECT id, task_id, started_at, finished_at, status, raw_output, summary FROM task_runs WHERE id = ?`, id,
	).Scan(&r.ID, &r.TaskID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.RawOutput, &r.Summary)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *TaskStore) scanTasks(rows *sql.Rows) ([]*models.Task, error) {
	var tasks []*models.Task
	for rows.Next() {
		var t models.Task
		var hostIDsJSON string
		if err := rows.Scan(&t.ID, &t.Name, &t.Goal, &hostIDsJSON, &t.TriggerType, &t.Schedule,
			&t.Status, &t.SourceConvID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(hostIDsJSON), &t.HostIDs) //nolint:errcheck
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/store/... -run TestTaskStore -v
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/store/task_store.go internal/store/task_store_test.go
git commit -m "feat(store): add TaskStore with CRUD and run tracking"
```

---

## Task 4: Wire TaskStore into App + main.go

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Add TaskStore to App struct**

In `internal/mcp/server.go`, add after `TodoStore` field:

```go
TaskStore  *store.TaskStore
```

- [ ] **Step 2: Init TaskStore in main.go**

In `cmd/spider/main.go`, after `app.TodoStore = store.NewTodoStore(database)`, add:

```go
app.TaskStore = store.NewTaskStore(database)
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: compiles clean.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(app): wire TaskStore into App and main.go"
```

---

## Task 5: Task REST API

**Files:**
- Create: `internal/api/tasks.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Create internal/api/tasks.go**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/spiderai/spider/internal/models"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func listTasks(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	tasks, err := app.TaskStore.List()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}
	writeJSON(w, 200, tasks)
}

func createTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if task.Name == "" {
		writeError(w, 400, "name required")
		return
	}
	if task.TriggerType == "" {
		task.TriggerType = "manual"
	}
	if task.Status == "" {
		task.Status = "active"
	}
	if err := app.TaskStore.Create(&task); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, task)
}

func getTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	task, err := app.TaskStore.Get(id)
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	writeJSON(w, 200, task)
}

func updateTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	task, err := app.TaskStore.Get(id)
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	if err := json.NewDecoder(r.Body).Decode(task); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	task.ID = id
	if err := app.TaskStore.Update(task); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, task)
}

func deleteTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	if err := app.TaskStore.Delete(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func triggerTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	task, err := app.TaskStore.Get(id)
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	if app.Scheduler == nil {
		writeError(w, 503, "scheduler not available")
		return
	}
	run, err := app.Scheduler.RunNow(r.Context(), task)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 202, run)
}

func listTaskRuns(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	runs, err := app.TaskStore.ListRuns(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if runs == nil {
		runs = []*models.TaskRun{}
	}
	writeJSON(w, 200, runs)
}

func getTaskRun(app *mcppkg.App, w http.ResponseWriter, r *http.Request, runIDStr string) {
	id, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	run, err := app.TaskStore.GetRun(id)
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	writeJSON(w, 200, run)
}
```

- [ ] **Step 2: Register routes in handler.go**

In `internal/api/handler.go`, in `NewRouter()`, before the `return mux` (or before the auth middleware wrap), add:

```go
	mux.HandleFunc("/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTasks(app, w, r)
		case http.MethodPost:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createTask(app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/tasks/"):]
		id := rest
		sub := ""
		if idx := indexOf(rest, '/'); idx >= 0 {
			id = rest[:idx]
			sub = rest[idx+1:]
		}
		switch {
		case sub == "" && r.Method == http.MethodGet:
			getTask(app, w, r, id)
		case sub == "" && r.Method == http.MethodPut:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				updateTask(app, w, r, id)
			})).ServeHTTP(w, r)
		case sub == "" && r.Method == http.MethodDelete:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteTask(app, w, r, id)
			})).ServeHTTP(w, r)
		case sub == "trigger" && r.Method == http.MethodPost:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				triggerTask(app, w, r, id)
			})).ServeHTTP(w, r)
		case sub == "runs" && r.Method == http.MethodGet:
			listTaskRuns(app, w, r, id)
		default:
			// Check for /runs/:run_id
			if runID := ""; true {
				runID = sub
				if len(sub) > 5 && sub[:5] == "runs/" {
					runID = sub[5:]
					if r.Method == http.MethodGet {
						getTaskRun(app, w, r, runID)
						return
					}
				}
			}
			http.NotFound(w, r)
		}
	})
```

Note: The `runs/:run_id` sub-path handling above has a logic issue. Replace the `default` case with:

```go
		default:
			if strings.HasPrefix(sub, "runs/") && r.Method == http.MethodGet {
				getTaskRun(app, w, r, sub[5:])
				return
			}
			http.NotFound(w, r)
```

- [ ] **Step 3: Add Scheduler field to App struct**

In `internal/mcp/server.go`, add a `Scheduler` field. First, create a minimal interface so `mcp` package doesn't import `scheduler` package (avoids circular deps):

In `internal/mcp/server.go`, add before the `App` struct:

```go
// TaskScheduler is the interface for triggering task runs on demand.
type TaskScheduler interface {
	RunNow(ctx context.Context, task *models.Task) (*models.TaskRun, error)
}
```

Then add to `App` struct:

```go
Scheduler TaskScheduler
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: compiles clean.

- [ ] **Step 5: Commit**

```bash
git add internal/api/tasks.go internal/api/handler.go internal/mcp/server.go
git commit -m "feat(api): add Task CRUD and run endpoints"
```

---

## Task 6: CreateTask agent tool

**Files:**
- Create: `internal/agent/tools_create_task.go`
- Create: `internal/agent/tools_create_task_test.go`
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Write failing test**

Create `internal/agent/tools_create_task_test.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/store"
)

func newTestCreateTaskTool(t *testing.T) *CreateTaskTool {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewCreateTaskTool(store.NewTaskStore(database), "conv-1")
}

func TestCreateTaskTool_Create(t *testing.T) {
	tool := newTestCreateTaskTool(t)
	res, err := tool.Execute(context.Background(), map[string]any{
		"name":         "daily disk check",
		"goal":         "check disk usage on all hosts",
		"trigger_type": "cron",
		"schedule":     "0 2 * * *",
	})
	if err != nil || res.IsError {
		t.Fatalf("unexpected error: %v / %s", err, res.Content)
	}
	var out map[string]any
	json.Unmarshal([]byte(res.Content), &out)
	if out["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestCreateTaskTool_MissingName(t *testing.T) {
	tool := newTestCreateTaskTool(t)
	res, _ := tool.Execute(context.Background(), map[string]any{"goal": "check disk"})
	if !res.IsError {
		t.Error("expected error for missing name")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/agent/... -run TestCreateTaskTool -v
```

Expected: FAIL — `CreateTaskTool` undefined.

- [ ] **Step 3: Create internal/agent/tools_create_task.go**

```go
package agent

import (
	"context"
	"encoding/json"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type CreateTaskTool struct {
	store          *store.TaskStore
	conversationID string
}

func NewCreateTaskTool(s *store.TaskStore, conversationID string) *CreateTaskTool {
	return &CreateTaskTool{store: s, conversationID: conversationID}
}

func (t *CreateTaskTool) Name() string             { return "CreateTask" }
func (t *CreateTaskTool) DefaultRiskLevel() RiskLevel { return RiskL2 }

func (t *CreateTaskTool) Description() string {
	return "Create a persistent automated task from conversation context. Has side effects. Use only after user confirms intent."
}

func (t *CreateTaskTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name", "goal"},
		"properties": map[string]any{
			"name":         map[string]any{"type": "string"},
			"goal":         map[string]any{"type": "string"},
			"host_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"trigger_type": map[string]any{"type": "string", "enum": []string{"cron", "manual"}},
			"schedule":     map[string]any{"type": "string"},
		},
	}
}

func (t *CreateTaskTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	name := strVal(input, "name")
	if name == "" {
		return &ToolResult{Content: "name required", IsError: true, RiskLevel: RiskL2}, nil
	}
	triggerType := strVal(input, "trigger_type")
	if triggerType == "" {
		triggerType = "manual"
	}
	task := &models.Task{
		Name:         name,
		Goal:         strVal(input, "goal"),
		HostIDs:      int64Slice(input, "host_ids"),
		TriggerType:  triggerType,
		Schedule:     strVal(input, "schedule"),
		Status:       "active",
		SourceConvID: t.conversationID,
	}
	if err := t.store.Create(task); err != nil {
		return &ToolResult{Content: "create failed: " + err.Error(), IsError: true, RiskLevel: RiskL2}, nil
	}
	out, _ := json.Marshal(task)
	return &ToolResult{Content: string(out), RiskLevel: RiskL2}, nil
}

const createTaskPromptSection = `## Task Automation (CreateTask tool)

Use CreateTask to persist a recurring or one-off automated task when the user explicitly asks to schedule or automate something.

**When to use:**
- User says "schedule", "automate", "every week", "remind me to", "set up a recurring task"
- User confirms intent after you describe what will be created

**When NOT to use:**
- User is asking a question, not requesting automation
- User has not confirmed — always describe the task first, then call CreateTask

**Rules:**
- Always show the task details (name, goal, schedule) to the user before calling CreateTask
- Wait for explicit confirmation before creating
- For cron tasks, validate the schedule expression is a valid 5-field cron`

func (t *CreateTaskTool) SystemPromptSection() string { return createTaskPromptSection }
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/agent/... -run TestCreateTaskTool -v
```

Expected: all pass.

- [ ] **Step 5: Register in factory.go**

In `internal/agent/factory.go`, add `TaskStore *store.TaskStore` field to `Factory` struct:

```go
TaskStore  *store.TaskStore
```

In `buildRegistry()`, after the `TodoTool` registration block, add:

```go
	if f.TaskStore != nil {
		registry.Register(NewCreateTaskTool(f.TaskStore, conversationID))
	}
```

In `internal/mcp/server.go`, in `NewAgentFactory()`, after `f.TodoStore = a.TodoStore`, add:

```go
f.TaskStore = a.TaskStore
```

- [ ] **Step 6: Build and test**

```bash
go build ./...
go test ./internal/agent/...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/agent/tools_create_task.go internal/agent/tools_create_task_test.go
git add internal/agent/factory.go internal/mcp/server.go
git commit -m "feat(agent): add CreateTask tool for persistent task automation"
```

---

## Task 7: Scheduler

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Modify: `cmd/spider/main.go`
- Modify: `internal/mcp/server.go`

- [ ] **Step 1: Create internal/scheduler/scheduler.go**

```go
package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type Scheduler struct {
	taskStore *store.TaskStore
	factory   *agent.Factory
	mu        sync.Mutex
	running   map[int64]bool
}

func New(taskStore *store.TaskStore, factory *agent.Factory) *Scheduler {
	return &Scheduler{
		taskStore: taskStore,
		factory:   factory,
		running:   make(map[int64]bool),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			s.tick(ctx, t)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	tasks, err := s.taskStore.ListActiveCron()
	if err != nil {
		logger.Global().Error().Err(err).Msg("scheduler: list active cron tasks")
		return
	}
	for _, task := range tasks {
		if matchesCron(task.Schedule, now) {
			s.mu.Lock()
			if s.running[task.ID] {
				s.mu.Unlock()
				continue
			}
			s.running[task.ID] = true
			s.mu.Unlock()
			go func(t *models.Task) {
				defer func() {
					s.mu.Lock()
					delete(s.running, t.ID)
					s.mu.Unlock()
				}()
				s.execute(ctx, t)
			}(task)
		}
	}
}

// RunNow triggers a task immediately (manual trigger). Implements mcp.TaskScheduler.
func (s *Scheduler) RunNow(ctx context.Context, task *models.Task) (*models.TaskRun, error) {
	run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "running"}
	if err := s.taskStore.CreateRun(run); err != nil {
		return nil, err
	}
	go s.executeRun(ctx, task, run)
	return run, nil
}

func (s *Scheduler) execute(ctx context.Context, task *models.Task) {
	run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "running"}
	if err := s.taskStore.CreateRun(run); err != nil {
		logger.Global().Error().Err(err).Int64("task_id", task.ID).Msg("scheduler: create run")
		return
	}
	s.executeRun(ctx, task, run)
}

func (s *Scheduler) executeRun(ctx context.Context, task *models.Task, run *models.TaskRun) {
	log := logger.Global().With().Int64("task_id", task.ID).Int64("run_id", run.ID).Logger()
	log.Info().Msg("scheduler: executing task")

	output, execErr := s.runAgent(ctx, task)

	now := time.Now().UTC()
	run.FinishedAt = &now
	run.RawOutput = output
	if execErr != nil {
		run.Status = "failed"
		run.RawOutput += "\n\nError: " + execErr.Error()
	} else {
		run.Status = "success"
	}

	if s.factory != nil && s.factory.LLMClient != nil {
		run.Summary = s.summarize(ctx, task, output)
	}

	if err := s.taskStore.UpdateRun(run); err != nil {
		log.Error().Err(err).Msg("scheduler: update run")
	}
	log.Info().Str("status", run.Status).Msg("scheduler: task complete")
}
```

Continue `scheduler.go` (runAgent, summarize, cron helpers):

```go
// memMsgStore is an in-memory MessageStorer for headless agent runs.
type memMsgStore struct {
	mu   sync.Mutex
	msgs []*models.Message
}

func (m *memMsgStore) Save(convID, role, content, toolCalls string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, &models.Message{Role: role, Content: content})
	return nil
}

func (m *memMsgStore) ListByConversation(_ string) ([]*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*models.Message, len(m.msgs))
	copy(out, m.msgs)
	return out, nil
}

func (m *memMsgStore) ListAfterMessage(_ string, _ string) ([]*models.Message, error) {
	return m.ListByConversation("")
}

func (m *memMsgStore) output() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var parts []string
	for _, msg := range m.msgs {
		if msg.Role == "assistant" {
			parts = append(parts, msg.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func (s *Scheduler) runAgent(ctx context.Context, task *models.Task) (string, error) {
	if s.factory == nil {
		return "", fmt.Errorf("agent factory not available")
	}
	msgStore := &memMsgStore{}
	f := *s.factory
	f.MsgStore = msgStore

	systemPrompt := fmt.Sprintf("You are executing an automated task.\n\nTask: %s\nGoal: %s\n\nComplete the task autonomously.", task.Name, task.Goal)
	ag := f.NewAgent(systemPrompt, fmt.Sprintf("task-%d", task.ID))

	events, err := ag.Run(ctx, fmt.Sprintf("task-%d", task.ID), "Execute the task now.", nil)
	if err != nil {
		return "", err
	}
	for range events {
		// drain events
	}
	return msgStore.output(), nil
}

func (s *Scheduler) summarize(ctx context.Context, task *models.Task, output string) string {
	if output == "" {
		return ""
	}
	prompt := fmt.Sprintf("Summarize the following task execution output in 2-3 sentences. Task: %s\n\nOutput:\n%s", task.Goal, output)
	summary, err := s.factory.LLMClient.Chat(ctx, &llm.ChatRequest{
		System:    "You are a concise summarizer. Respond in the same language as the task goal.",
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 256,
	})
	if err != nil {
		return ""
	}
	return summary
}

func matchesCron(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	return matchField(fields[0], t.Minute(), 0, 59) &&
		matchField(fields[1], t.Hour(), 0, 23) &&
		matchField(fields[2], t.Day(), 1, 31) &&
		matchField(fields[3], int(t.Month()), 1, 12) &&
		matchField(fields[4], int(t.Weekday()), 0, 6)
}

func matchField(field string, val, min, _ int) bool {
	if field == "*" {
		return true
	}
	if strings.HasPrefix(field, "*/") {
		n, err := strconv.Atoi(field[2:])
		if err != nil || n <= 0 {
			return false
		}
		return (val-min)%n == 0
	}
	for _, part := range strings.Split(field, ",") {
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 == nil && err2 == nil && val >= lo && val <= hi {
				return true
			}
		} else {
			n, err := strconv.Atoi(part)
			if err == nil && n == val {
				return true
			}
		}
	}
	return false
}
```

Note: `llm.RoleUser` is `llm.Role("user")` — use `llm.RoleUser` constant. The `Chat()` method returns `(string, error)` directly, no need to drain a channel.

- [ ] **Step 2: Check LLM stream event type**

`internal/llm/client.go` confirms: `Client.Chat(ctx, req)` returns `(string, error)`. Use `Chat()` for summarize (simpler than streaming). `llm.RoleUser` is the constant for the user role.

- [ ] **Step 3: Wire Scheduler into main.go**

In `cmd/spider/main.go`, after `agentFactory` is set up, add:

```go
	sched := scheduler.New(store.NewTaskStore(database), agentFactory)
	app.Scheduler = sched
	go sched.Start(shutdownCtx)
```

Add import: `"github.com/spiderai/spider/internal/scheduler"`

Note: `agentFactory` may be nil if no provider is configured. The scheduler handles nil factory gracefully (returns error from RunNow).

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: compiles clean. Fix any type mismatches in `summarize()` based on actual LLM stream API.

- [ ] **Step 5: Commit**

```bash
git add internal/scheduler/scheduler.go cmd/spider/main.go
git commit -m "feat(scheduler): add cron scheduler with headless agent execution"
```

---

## Task 8: Frontend — TasksView

**Files:**
- Create: `web/src/api/tasks.ts`
- Create: `web/src/views/TasksView.vue`
- Modify: `web/src/main.ts`
- Modify: `web/src/App.vue`

- [ ] **Step 1: Create web/src/api/tasks.ts**

```typescript
import { authHeaders } from './auth'

export interface Task {
  id: number
  name: string
  goal: string
  host_ids: number[]
  trigger_type: 'cron' | 'manual'
  schedule: string
  status: 'active' | 'paused' | 'archived'
  source_conv_id: string
  created_at: string
  updated_at: string
}

export interface TaskRun {
  id: number
  task_id: number
  started_at: string
  finished_at: string | null
  status: 'running' | 'success' | 'failed'
  raw_output: string
  summary: string
}

export async function listTasks(): Promise<Task[]> {
  const res = await fetch('/api/v1/tasks', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createTask(data: Partial<Task>): Promise<Task> {
  const res = await fetch('/api/v1/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function updateTask(id: number, data: Partial<Task>): Promise<Task> {
  const res = await fetch(`/api/v1/tasks/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteTask(id: number): Promise<void> {
  const res = await fetch(`/api/v1/tasks/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function triggerTask(id: number): Promise<TaskRun> {
  const res = await fetch(`/api/v1/tasks/${id}/trigger`, { method: 'POST', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function listTaskRuns(taskId: number): Promise<TaskRun[]> {
  const res = await fetch(`/api/v1/tasks/${taskId}/runs`, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}
```

- [ ] **Step 2: Create web/src/views/TasksView.vue (part 1 — template)**

```vue
<template>
  <div class="tasks-layout">
    <aside class="tasks-sidebar">
      <div class="sidebar-header">
        <h2>任务</h2>
        <button class="btn-primary" @click="showCreate = true">新建</button>
      </div>
      <div class="task-list">
        <div
          v-for="task in tasks"
          :key="task.id"
          class="task-item"
          :class="{ active: selectedTask?.id === task.id }"
          @click="selectTask(task)"
        >
          <div class="task-item-name">{{ task.name }}</div>
          <div class="task-item-meta">
            <span class="badge" :class="`badge-${task.status}`">{{ task.status }}</span>
            <span class="task-schedule">{{ task.trigger_type === 'cron' ? task.schedule : '手动' }}</span>
          </div>
        </div>
        <div v-if="tasks.length === 0" class="empty-hint">暂无任务</div>
      </div>
    </aside>

    <main class="tasks-detail" v-if="selectedTask">
      <div class="detail-header">
        <div class="detail-title">
          <h2>{{ selectedTask.name }}</h2>
          <span class="badge" :class="`badge-${selectedTask.status}`">{{ selectedTask.status }}</span>
        </div>
        <div class="detail-actions">
          <button class="btn-secondary" @click="triggerNow" :disabled="triggering">立即执行</button>
          <button class="btn-secondary" @click="toggleStatus">{{ selectedTask.status === 'active' ? '暂停' : '激活' }}</button>
          <button class="btn-danger" @click="confirmDelete">删除</button>
        </div>
      </div>

      <div class="detail-meta">
        <div class="meta-row"><span class="meta-label">目标</span><span>{{ selectedTask.goal }}</span></div>
        <div class="meta-row"><span class="meta-label">触发</span><span>{{ selectedTask.trigger_type === 'cron' ? selectedTask.schedule : '手动' }}</span></div>
        <div class="meta-row" v-if="selectedTask.source_conv_id"><span class="meta-label">来源对话</span><span>{{ selectedTask.source_conv_id }}</span></div>
      </div>

      <div class="runs-section">
        <h3>执行记录</h3>
        <div v-if="runs.length === 0" class="empty-hint">暂无执行记录</div>
        <div v-for="run in runs" :key="run.id" class="run-item">
          <div class="run-header" @click="toggleRun(run.id)">
            <span class="run-status-icon">{{ runIcon(run) }}</span>
            <span class="run-time">{{ formatTime(run.started_at) }}</span>
            <span class="run-duration" v-if="run.finished_at">{{ duration(run) }}</span>
            <span class="run-status badge" :class="`badge-${run.status}`">{{ run.status }}</span>
          </div>
          <div v-if="expandedRuns.has(run.id)" class="run-detail">
            <div v-if="run.summary" class="run-summary">{{ run.summary }}</div>
            <pre v-if="run.raw_output" class="run-output">{{ run.raw_output }}</pre>
          </div>
        </div>
      </div>
    </main>

    <main class="tasks-detail tasks-empty" v-else>
      <p>选择一个任务查看详情</p>
    </main>

    <!-- Create dialog -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建任务</h3>
        <label>名称 <input v-model="form.name" placeholder="任务名称" /></label>
        <label>目标 <textarea v-model="form.goal" placeholder="自然语言描述任务目标" /></label>
        <label>触发类型
          <select v-model="form.trigger_type">
            <option value="manual">手动</option>
            <option value="cron">定时 (cron)</option>
          </select>
        </label>
        <label v-if="form.trigger_type === 'cron'">Cron 表达式 <input v-model="form.schedule" placeholder="0 2 * * *" /></label>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showCreate = false">取消</button>
          <button class="btn-primary" @click="submitCreate" :disabled="creating">创建</button>
        </div>
        <p v-if="createError" class="error-msg">{{ createError }}</p>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Create web/src/views/TasksView.vue (part 2 — script + style)**

Append to `TasksView.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTasks, createTask, updateTask, deleteTask, triggerTask, listTaskRuns, type Task, type TaskRun } from '../api/tasks'

const tasks = ref<Task[]>([])
const selectedTask = ref<Task | null>(null)
const runs = ref<TaskRun[]>([])
const expandedRuns = ref<Set<number>>(new Set())
const showCreate = ref(false)
const creating = ref(false)
const triggering = ref(false)
const createError = ref('')
const form = ref({ name: '', goal: '', trigger_type: 'manual' as 'manual' | 'cron', schedule: '' })

onMounted(async () => { tasks.value = await listTasks() })

async function selectTask(task: Task) {
  selectedTask.value = task
  runs.value = await listTaskRuns(task.id)
  expandedRuns.value = new Set()
}

async function submitCreate() {
  createError.value = ''
  if (!form.value.name) { createError.value = '名称不能为空'; return }
  creating.value = true
  try {
    const task = await createTask(form.value)
    tasks.value.unshift(task)
    showCreate.value = false
    form.value = { name: '', goal: '', trigger_type: 'manual', schedule: '' }
  } catch (e: any) {
    createError.value = e.message
  } finally {
    creating.value = false
  }
}

async function toggleStatus() {
  if (!selectedTask.value) return
  const newStatus = selectedTask.value.status === 'active' ? 'paused' : 'active'
  const updated = await updateTask(selectedTask.value.id, { ...selectedTask.value, status: newStatus })
  selectedTask.value = updated
  const idx = tasks.value.findIndex(t => t.id === updated.id)
  if (idx >= 0) tasks.value[idx] = updated
}

async function triggerNow() {
  if (!selectedTask.value) return
  triggering.value = true
  try {
    await triggerTask(selectedTask.value.id)
    runs.value = await listTaskRuns(selectedTask.value.id)
  } finally {
    triggering.value = false
  }
}

async function confirmDelete() {
  if (!selectedTask.value || !confirm(`删除任务 "${selectedTask.value.name}"？`)) return
  await deleteTask(selectedTask.value.id)
  tasks.value = tasks.value.filter(t => t.id !== selectedTask.value!.id)
  selectedTask.value = null
  runs.value = []
}

function toggleRun(id: number) {
  if (expandedRuns.value.has(id)) expandedRuns.value.delete(id)
  else expandedRuns.value.add(id)
}

function runIcon(run: TaskRun) {
  return run.status === 'success' ? '✓' : run.status === 'failed' ? '✗' : '⟳'
}

function formatTime(s: string) {
  return new Date(s).toLocaleString('zh-CN')
}

function duration(run: TaskRun) {
  if (!run.finished_at) return ''
  const ms = new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()
  return ms < 60000 ? `${Math.round(ms / 1000)}s` : `${Math.round(ms / 60000)}m`
}
</script>

<style scoped>
.tasks-layout { display: flex; height: 100%; overflow: hidden; }
.tasks-sidebar { width: 280px; border-right: 1px solid var(--border); display: flex; flex-direction: column; }
.sidebar-header { display: flex; justify-content: space-between; align-items: center; padding: 16px; border-bottom: 1px solid var(--border); }
.sidebar-header h2 { margin: 0; font-size: 16px; }
.task-list { flex: 1; overflow-y: auto; }
.task-item { padding: 12px 16px; cursor: pointer; border-bottom: 1px solid var(--border-light); }
.task-item:hover, .task-item.active { background: var(--bg-hover); }
.task-item-name { font-weight: 500; margin-bottom: 4px; }
.task-item-meta { display: flex; gap: 8px; align-items: center; font-size: 12px; color: var(--text-muted); }
.tasks-detail { flex: 1; overflow-y: auto; padding: 24px; }
.tasks-empty { display: flex; align-items: center; justify-content: center; color: var(--text-muted); }
.detail-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; }
.detail-title { display: flex; align-items: center; gap: 12px; }
.detail-title h2 { margin: 0; }
.detail-actions { display: flex; gap: 8px; }
.detail-meta { background: var(--bg-secondary); border-radius: 8px; padding: 16px; margin-bottom: 24px; }
.meta-row { display: flex; gap: 12px; padding: 4px 0; }
.meta-label { font-weight: 500; min-width: 80px; color: var(--text-muted); }
.runs-section h3 { margin-bottom: 12px; }
.run-item { border: 1px solid var(--border); border-radius: 6px; margin-bottom: 8px; overflow: hidden; }
.run-header { display: flex; align-items: center; gap: 12px; padding: 10px 14px; cursor: pointer; }
.run-header:hover { background: var(--bg-hover); }
.run-time { flex: 1; font-size: 13px; }
.run-detail { padding: 12px 14px; border-top: 1px solid var(--border); }
.run-summary { margin-bottom: 8px; color: var(--text-secondary); }
.run-output { font-size: 12px; background: var(--bg-code); padding: 8px; border-radius: 4px; overflow-x: auto; white-space: pre-wrap; max-height: 300px; overflow-y: auto; }
.badge { padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 500; }
.badge-active { background: #dcfce7; color: #166534; }
.badge-paused { background: #fef9c3; color: #854d0e; }
.badge-archived { background: var(--bg-secondary); color: var(--text-muted); }
.badge-success { background: #dcfce7; color: #166534; }
.badge-failed { background: #fee2e2; color: #991b1b; }
.badge-running { background: #dbeafe; color: #1e40af; }
.empty-hint { padding: 24px; text-align: center; color: var(--text-muted); }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg); border-radius: 12px; padding: 24px; width: 480px; display: flex; flex-direction: column; gap: 12px; }
.modal h3 { margin: 0; }
.modal label { display: flex; flex-direction: column; gap: 4px; font-size: 14px; }
.modal input, .modal textarea, .modal select { padding: 8px; border: 1px solid var(--border); border-radius: 6px; background: var(--bg-input); color: var(--text); }
.modal textarea { min-height: 80px; resize: vertical; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }
.error-msg { color: var(--error); font-size: 13px; }
</style>
```

- [ ] **Step 4: Add route in main.ts**

In `web/src/main.ts`, add to the routes array:

```typescript
{ path: '/tasks', component: () => import('./views/TasksView.vue') },
```

- [ ] **Step 5: Add nav link in App.vue**

In `web/src/App.vue`, add after the `知识库` nav link:

```html
<RouterLink to="/tasks" class="nav-item">任务</RouterLink>
```

- [ ] **Step 6: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build
```

Expected: builds clean, no TypeScript errors.

- [ ] **Step 7: Build Go binary and verify**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data &
sleep 2
curl -s http://localhost:8002/api/v1/tasks -H "Authorization: Bearer $(cat ~/.spider/data/token 2>/dev/null || echo test)" | head -c 200
```

Expected: returns JSON array (may be empty `[]`).

- [ ] **Step 8: Commit**

```bash
git add web/src/api/tasks.ts web/src/views/TasksView.vue web/src/main.ts web/src/App.vue
git commit -m "feat(web): add TasksView with list, detail, and run history"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| TodoTask → Todo rename | Task 1 |
| Task model (ID, Name, Goal, HostIDs, TriggerType, Schedule, Status, SourceConvID) | Task 2 |
| TaskRun model (ID, TaskID, StartedAt, FinishedAt, Status, RawOutput, Summary) | Task 2 |
| DB tables tasks + task_runs | Task 2 |
| TaskStore CRUD | Task 3 |
| cron + manual trigger | Task 7 |
| headless Agent execution | Task 7 |
| LLM summary after run | Task 7 |
| CreateTask agent tool | Task 6 |
| REST API (GET/POST/PUT/DELETE tasks, trigger, runs) | Task 5 |
| Frontend left list + right detail + runs | Task 8 |
| New/edit dialog | Task 8 |

**Gaps found:**
- `models.Message` is used in `memMsgStore` — verify it's in `internal/models/` (it is, in `internal/store/message.go` or similar). Check and import correctly.
- LLM stream API: `summarize()` uses `llm.EventTextDelta` and `ev.Content` — verify against actual `internal/llm/models.go` types before implementing.
- `app.Scheduler` field type is `TaskScheduler` interface — ensure `*scheduler.Scheduler` implements it (it does via `RunNow` method).
- `agentFactory` in main.go may be nil — scheduler handles this gracefully.
- The `runs/:run_id` route in handler.go uses `strings.HasPrefix` — ensure `strings` is imported in `handler.go` (it already is per the existing code).


**Create (Tasks 2–8):**
- Create: `internal/models/task.go` — Task, TaskRun structs
- Create: `internal/store/task_store.go` — TaskStore CRUD
- Create: `internal/store/task_store_test.go` — tests
- Create: `internal/api/tasks.go` — HTTP handlers
- Create: `internal/agent/tools_create_task.go` — CreateTask tool
- Create: `internal/agent/tools_create_task_test.go` — tests
- Create: `internal/scheduler/scheduler.go` — cron scheduler + headless runner
- Create: `web/src/api/tasks.ts` — API client
- Create: `web/src/views/TasksView.vue` — left list + right detail with runs

**Modify (Tasks 2–8):**
- Modify: `internal/db/schema.go` — add tasks, task_runs tables
- Modify: `internal/mcp/server.go` — add TaskStore field
- Modify: `internal/api/handler.go` — register /api/v1/tasks routes
- Modify: `cmd/spider/main.go` — init TaskStore, start Scheduler
- Modify: `internal/agent/factory.go` — register CreateTask tool
- Modify: `web/src/main.ts` — add /tasks route
- Modify: `web/src/App.vue` — add 任务 nav link

---

## Task 1: TodoTask → Todo rename

**Files:**
- Rename: `internal/models/todo_task.go` → `internal/models/todo.go`
- Rename: `internal/store/todo_task_store.go` → `internal/store/todo_store.go`
- Rename: `internal/store/todo_task_store_test.go` → `internal/store/todo_store_test.go`
- Rename: `internal/agent/tools_todo_task.go` → `internal/agent/tools_todo.go`
- Rename: `internal/agent/tools_todo_task_test.go` → `internal/agent/tools_todo_test.go`
- Modify: `internal/db/schema.go`, `internal/mcp/server.go`, `internal/agent/factory.go`, `cmd/spider/main.go`, `internal/api/chat.go`, `web/src/api/chat.ts`, `web/src/views/ChatView.vue`

