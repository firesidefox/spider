# Agent Todo Context Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove `turn_id` and `blocked_by` from the todo system, inject active task state into LLM context on every `Run()` so the agent never recreates existing tasks after a mid-task user reply.

**Architecture:** Drop `turn_id` and `blocked_by` columns via SQLite table-recreation migration. Simplify `TodoStore.List()` to filter by conversation only. Add `todoStore` field to `Agent` and inject a synthetic `<system-reminder>` message into history before the first LLM call each turn.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), existing `internal/db`, `internal/store`, `internal/agent`, `internal/models` packages.

---

## File Map

| File | Change |
|------|--------|
| `internal/db/schema.go` | Add migration block: recreate `todo_tasks` without `turn_id`/`blocked_by` |
| `internal/models/todo_task.go` | Remove `TurnID` and `BlockedBy` fields |
| `internal/store/todo_task_store.go` | Remove `turn_id`/`blocked_by` from all queries; simplify `List()`; remove `ListByTurn()` |
| `internal/agent/tools_todo_task.go` | Remove `turnID` param; remove `blocked_by` from schema/create/update; fix `allTasksDone()` |
| `internal/agent/factory.go` | Remove `TurnID` field; pass `TodoStore` into `AgentConfig` |
| `internal/agent/agent.go` | Add `todoStore` to `Agent`/`AgentConfig`; inject task reminder into history |
| `internal/api/chat.go` | Remove `factory.TurnID = uuid.New().String()` |
| `internal/store/todo_task_store_test.go` | Remove `TestTodoStore_BlockedBy`; add `TestTodoStore_ListExcludesCompleted` |
| `internal/agent/tools_todo_task_test.go` | Update `newTestTodoTool` to remove `turnID` arg; add context injection test |

---

### Task 1: Schema migration — drop `turn_id` and `blocked_by`

**Files:**
- Modify: `internal/db/schema.go:402-405`

SQLite does not support dropping columns before v3.35. The migration recreates the table.

- [ ] **Step 1: Add migration block at end of `migrate()` in `schema.go`**

Replace the two existing `ALTER TABLE todo_tasks ADD COLUMN` lines (lines 402-403) with a table-recreation migration:

```go
	// Migrate todo_tasks: drop turn_id and blocked_by columns
	var hasTurnID int
	db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('todo_tasks') WHERE name='turn_id'`).Scan(&hasTurnID)
	if hasTurnID > 0 {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS todo_tasks_new (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT    NOT NULL,
			subject         TEXT    NOT NULL,
			active_form     TEXT    NOT NULL DEFAULT '',
			description     TEXT    NOT NULL DEFAULT '',
			status          TEXT    NOT NULL DEFAULT 'pending',
			owner           TEXT    NOT NULL DEFAULT '',
			created_at      DATETIME NOT NULL,
			updated_at      DATETIME NOT NULL,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		)`); err != nil {
			return err
		}
		if _, err := db.Exec(`INSERT INTO todo_tasks_new
			SELECT id, conversation_id, subject, active_form, description, status, owner, created_at, updated_at
			FROM todo_tasks`); err != nil {
			return err
		}
		if _, err := db.Exec(`DROP TABLE todo_tasks`); err != nil {
			return err
		}
		if _, err := db.Exec(`ALTER TABLE todo_tasks_new RENAME TO todo_tasks`); err != nil {
			return err
		}
	} else {
		// Fresh install: ensure active_form column exists (added in earlier migration)
		db.Exec(`ALTER TABLE todo_tasks ADD COLUMN active_form TEXT NOT NULL DEFAULT ''`)
	}
	db.Exec(`ALTER TABLE users ADD COLUMN ui_prefs TEXT NOT NULL DEFAULT '{}'`)
```

Note: the `active_form` and `ui_prefs` ALTER statements that were on lines 403-404 are now handled inside this block.

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): migrate todo_tasks — drop turn_id and blocked_by columns"
```

---

### Task 2: Update model — remove `TurnID` and `BlockedBy`

**Files:**
- Modify: `internal/models/todo_task.go`

- [ ] **Step 1: Rewrite the model**

```go
package models

import "time"

type Todo struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Subject        string    `json:"subject"`
	ActiveForm     string    `json:"active_form,omitempty"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: compile errors in `store` and `agent` packages referencing `TurnID`/`BlockedBy` — these are fixed in subsequent tasks.

- [ ] **Step 3: Commit when all tasks in this group compile**

Hold commit until Task 3 is done (model + store change together).

---

### Task 3: Update `TodoStore` — remove `turn_id`/`blocked_by`, simplify `List()`

**Files:**
- Modify: `internal/store/todo_task_store.go`
- Modify: `internal/store/todo_task_store_test.go`

- [ ] **Step 1: Rewrite `Create()` — remove `turn_id` and `blocked_by`**

Replace the `Create` method body:

```go
func (s *TodoStore) Create(task *models.Todo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO todo_tasks (conversation_id, subject, active_form, description, status, owner, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ConversationID, task.Subject, task.ActiveForm, task.Description,
		task.Status, task.Owner, now, now,
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

- [ ] **Step 2: Rewrite `Update()` — remove `blockedBy` param and `blocked_by` SET clause**

```go
func (s *TodoStore) Update(conversationID string, id int64, subject, activeForm, description, status, owner string) (*models.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	setClauses := []string{"updated_at = ?"}
	args := []any{now}

	if subject != "" {
		setClauses = append(setClauses, "subject = ?")
		args = append(args, subject)
	}
	if activeForm != "" {
		setClauses = append(setClauses, "active_form = ?")
		args = append(args, activeForm)
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

	args = append(args, id, conversationID)
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE todo_tasks SET %s WHERE id = ? AND conversation_id = ?", strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return nil, err
	}

	var t models.Todo
	err = s.db.QueryRow(
		`SELECT id, conversation_id, subject, active_form, description, status, owner, created_at, updated_at
		 FROM todo_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ConversationID, &t.Subject, &t.ActiveForm, &t.Description,
		&t.Status, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	logger.Global().Debug().Str("table", "todo_tasks").Str("op", "update").Int64("task_id", id).Str("status", status).Msg("store")
	return &t, nil
}
```

- [ ] **Step 3: Rewrite `List()` — simple conversation filter**

```go
func (s *TodoStore) List(conversationID string) ([]*models.Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, subject, active_form, description, status, owner, created_at, updated_at
		 FROM todo_tasks
		 WHERE conversation_id = ? AND status NOT IN ('completed', 'deleted')
		 ORDER BY id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Todo
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(&t.ID, &t.ConversationID, &t.Subject, &t.ActiveForm, &t.Description,
			&t.Status, &t.Owner, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	logger.Global().Debug().Str("table", "todo_tasks").Str("op", "select").Str("conv_id", conversationID).Int("count", len(tasks)).Msg("store")
	return tasks, nil
}
```

- [ ] **Step 4: Rewrite `Get()` — remove `turn_id`/`blocked_by` scan**

```go
func (s *TodoStore) Get(id int64) (*models.Todo, error) {
	var t models.Todo
	err := s.db.QueryRow(
		`SELECT id, conversation_id, subject, active_form, description, status, owner, created_at, updated_at
		 FROM todo_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ConversationID, &t.Subject, &t.ActiveForm, &t.Description,
		&t.Status, &t.Owner, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
```

- [ ] **Step 5: Delete `ListByTurn()` entirely** (no longer needed)

- [ ] **Step 6: Update store tests**

Replace `internal/store/todo_task_store_test.go` content:

```go
package store

import (
	"testing"

	"github.com/spiderai/spider/internal/models"
)

func TestTodoStore_CreateAndList(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	task := &models.Todo{ConversationID: "conv-1", Subject: "check device", Status: "pending"}
	if err := s.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == 0 {
		t.Fatal("expected ID to be set")
	}

	tasks, err := s.List("conv-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Subject != "check device" {
		t.Errorf("unexpected tasks: %+v", tasks)
	}
}

func TestTodoStore_Update(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	task := &models.Todo{ConversationID: "conv-1", Subject: "task1", Status: "pending"}
	s.Create(task)

	got, err := s.Update("conv-1", task.ID, "", "", "", "in_progress", "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Status != "in_progress" {
		t.Errorf("expected in_progress, got %s", got.Status)
	}
}

func TestTodoStore_ListExcludesDeleted(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	task := &models.Todo{ConversationID: "conv-1", Subject: "task1", Status: "pending"}
	s.Create(task)
	s.Update("conv-1", task.ID, "", "", "", "deleted", "")

	tasks, _ := s.List("conv-1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestTodoStore_ListExcludesCompleted(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	task := &models.Todo{ConversationID: "conv-1", Subject: "task1", Status: "pending"}
	s.Create(task)
	s.Update("conv-1", task.ID, "", "", "", "completed", "")

	tasks, _ := s.List("conv-1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after completion, got %d", len(tasks))
	}
}

func TestTodoStore_ListOnlyActiveConversation(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	s.Create(&models.Todo{ConversationID: "conv-1", Subject: "t1", Status: "pending"})
	s.Create(&models.Todo{ConversationID: "conv-2", Subject: "t2", Status: "pending"})

	tasks, _ := s.List("conv-1")
	if len(tasks) != 1 || tasks[0].Subject != "t1" {
		t.Errorf("unexpected tasks: %+v", tasks)
	}
}

func TestTodoStore_ConcurrentWrites(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			task := &models.Todo{ConversationID: "conv-1", Subject: "task", Status: "pending"}
			done <- s.Create(task)
		}()
	}
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Create: %v", err)
		}
	}
}
```

- [ ] **Step 7: Run store tests**

```bash
go test ./internal/store/... -run TestTodoStore -v
```
Expected: all 5 tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/models/todo_task.go internal/store/todo_task_store.go internal/store/todo_task_store_test.go
git commit -m "feat(store): remove turn_id and blocked_by from TodoStore"
```

---

### Task 4: Update `TodoTool` — remove `turnID` and `blocked_by`

**Files:**
- Modify: `internal/agent/tools_todo_task.go`
- Modify: `internal/agent/tools_todo_task_test.go`

- [ ] **Step 1: Update `TodoTool` struct and constructor**

```go
type TodoTool struct {
	store          *store.TodoStore
	broadcaster    SSEBroadcaster
	conversationID string
}

func NewTodoTool(s *store.TodoStore, broadcaster SSEBroadcaster, conversationID string) *TodoTool {
	return &TodoTool{store: s, broadcaster: broadcaster, conversationID: conversationID}
}
```

- [ ] **Step 2: Update `InputSchema()` — remove `blocked_by`**

```go
func (t *TodoTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"action"},
		"properties": map[string]any{
			"action":      map[string]any{"type": "string", "enum": []string{"create", "update", "list"}},
			"subject":     map[string]any{"type": "string"},
			"active_form": map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"status":      map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "deleted"}},
			"owner":       map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "integer"},
		},
	}
}
```

- [ ] **Step 3: Update `execCreate()` — remove `TurnID` and `BlockedBy`**

```go
func (t *TodoTool) execCreate(input map[string]any) (*ToolResult, error) {
	subject, _ := input["subject"].(string)
	if subject == "" {
		return &ToolResult{Content: "create requires subject", IsError: true, RiskLevel: RiskL1}, nil
	}
	task := &models.Todo{
		ConversationID: t.conversationID,
		Subject:        subject,
		ActiveForm:     strVal(input, "active_form"),
		Description:    strVal(input, "description"),
		Status:         "pending",
		Owner:          strVal(input, "owner"),
	}
	if err := t.store.Create(task); err != nil {
		return &ToolResult{Content: "create failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	t.broadcast(task)
	out, _ := json.Marshal(task)
	return &ToolResult{Content: string(out) + todoNudge(false), RiskLevel: RiskL1}, nil
}
```

- [ ] **Step 4: Update `execUpdate()` — remove `blockedBy` param**

```go
func (t *TodoTool) execUpdate(input map[string]any) (*ToolResult, error) {
	taskIDFloat, ok := input["task_id"].(float64)
	if !ok {
		return &ToolResult{Content: "update requires task_id", IsError: true, RiskLevel: RiskL1}, nil
	}
	taskID := int64(taskIDFloat)

	subject := strVal(input, "subject")
	activeForm := strVal(input, "active_form")
	description := strVal(input, "description")
	status := strVal(input, "status")
	owner := strVal(input, "owner")

	if subject == "" && activeForm == "" && description == "" && status == "" && owner == "" {
		return &ToolResult{Content: "update requires at least one field besides task_id", IsError: true, RiskLevel: RiskL1}, nil
	}

	task, err := t.store.Update(t.conversationID, taskID, subject, activeForm, description, status, owner)
	if err != nil {
		return &ToolResult{Content: "update failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	t.broadcast(task)
	out, _ := json.Marshal(task)

	tasks, allDone := t.allTasksDone()
	if allDone {
		t.broadcastSummary(tasks)
	}
	return &ToolResult{Content: string(out) + todoNudge(allDone), RiskLevel: RiskL1}, nil
}
```

- [ ] **Step 5: Fix `allTasksDone()` — use `List()` instead of `ListByTurn()`**

```go
func (t *TodoTool) allTasksDone() ([]*models.Todo, bool) {
	// List() only returns non-completed, non-deleted tasks.
	// If it returns empty, all tasks in this conversation are done.
	active, err := t.store.List(t.conversationID)
	if err != nil {
		logger.Global().Error().Err(err).Str("conv_id", t.conversationID).Msg("allTasksDone: List failed")
		return nil, false
	}
	if len(active) > 0 {
		return nil, false
	}
	// Fetch all tasks for the summary (including completed ones via Get is not available;
	// use a separate query via store.ListAll if needed — for now return nil tasks, summary skipped)
	return nil, true
}
```

Note: `broadcastSummary` needs tasks to render the list. Since `List()` now excludes completed tasks, we need a `ListAll` method or we skip the summary. The simplest fix: return `nil, true` and guard `broadcastSummary` against nil:

```go
tasks, allDone := t.allTasksDone()
if allDone {
	if tasks != nil {
		t.broadcastSummary(tasks)
	} else {
		t.broadcastEvent("todo_summary", "**All tasks completed.**")
	}
}
```

- [ ] **Step 6: Remove `int64Slice` helper** (no longer used)

Delete the `int64Slice` function at the bottom of `tools_todo_task.go`.

- [ ] **Step 7: Update `tools_todo_task_test.go` — remove `turnID` arg from `newTestTodoTool`**

```go
func newTestTodoTool(t *testing.T) (*TodoTool, *mockBroadcaster) {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	bc := &mockBroadcaster{}
	return NewTodoTool(store.NewTodoStore(database), bc, "conv-1"), bc
}
```

- [ ] **Step 8: Run tool tests**

```bash
go test ./internal/agent/... -run TestTodoTool -v
```
Expected: all existing TodoTool tests PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/agent/tools_todo_task.go internal/agent/tools_todo_task_test.go
git commit -m "feat(agent): remove turnID and blocked_by from TodoTool"
```

---

### Task 5: Wire `TodoStore` into `Agent` and inject task reminder

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/factory.go`
- Modify: `internal/api/chat.go`

- [ ] **Step 1: Add `todoStore` to `Agent` and `AgentConfig` in `agent.go`**

Add field to `Agent` struct:
```go
type Agent struct {
	llmClient     llm.Client
	registry      *ToolRegistry
	hooks         *HookChain
	msgStore      MessageStorer
	todoStore     *store.TodoStore
	systemPrompt  string
	maxTurns      int
	compactor     *Compactor
	skillManager  *SkillManager
	lastSkillHash string
}
```

Add field to `AgentConfig`:
```go
type AgentConfig struct {
	LLMClient    llm.Client
	Registry     *ToolRegistry
	Hooks        *HookChain
	MsgStore     MessageStorer
	TodoStore    *store.TodoStore
	SystemPrompt string
	MaxTurns     int
	Compactor    *Compactor
	SkillManager *SkillManager
}
```

Wire in `NewAgent()`:
```go
return &Agent{
	llmClient:    cfg.LLMClient,
	registry:     cfg.Registry,
	hooks:        cfg.Hooks,
	msgStore:     cfg.MsgStore,
	todoStore:    cfg.TodoStore,
	systemPrompt: epaSystemPromptPrefix + cfg.SystemPrompt,
	maxTurns:     maxTurns,
	compactor:    cfg.Compactor,
	skillManager: cfg.SkillManager,
}
```

- [ ] **Step 2: Add `buildTaskReminderMessage()` helper in `agent.go`**

Add after the `isContextLengthError` function:

```go
func buildTaskReminderMessage(tasks []*models.Todo) llm.Message {
	var sb strings.Builder
	sb.WriteString("<system-reminder>\nCurrent tasks for this conversation:\n")
	for _, t := range tasks {
		sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", t.ID, t.Status, t.Subject))
	}
	sb.WriteString("</system-reminder>")
	return llm.Message{Role: llm.RoleUser, Content: sb.String()}
}
```

- [ ] **Step 3: Inject task reminder into `Run()` — after history is built, before turn loop**

In `Run()`, after the history-building block (after the `if finalUserMessage != userMessage` block, before `toolDefs := a.registry.Definitions()`), add:

```go
if a.todoStore != nil {
	if tasks, err := a.todoStore.List(conversationID); err == nil && len(tasks) > 0 {
		history = append(history, buildTaskReminderMessage(tasks))
	}
}
```

- [ ] **Step 4: Add required import to `agent.go`**

Add `"github.com/spiderai/spider/internal/store"` and `"github.com/spiderai/spider/internal/models"` to imports if not already present.

- [ ] **Step 5: Update `factory.go` — remove `TurnID`, pass `TodoStore` into `AgentConfig`**

Remove `TurnID string` from `Factory` struct.

In `buildRegistry()`, change line:
```go
registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID, f.TurnID))
```
to:
```go
registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID))
```

In `NewAgent()`, add `TodoStore` to `AgentConfig`:
```go
return NewAgent(AgentConfig{
	LLMClient:    f.LLMClient,
	Registry:     registry,
	Hooks:        hooks,
	MsgStore:     f.MsgStore,
	TodoStore:    f.TodoStore,
	SystemPrompt: systemPrompt,
	MaxTurns:     15,
	Compactor:    compactor,
	SkillManager: NewSkillManager(f.DataDir),
})
```

- [ ] **Step 6: Update `chat.go` — remove `factory.TurnID = uuid.New().String()`**

Delete line 135: `factory.TurnID = uuid.New().String()`

Also remove the `"github.com/google/uuid"` import if it's now unused.

- [ ] **Step 7: Build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 8: Run all tests**

```bash
go test ./...
```
Expected: all tests PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/agent/agent.go internal/agent/factory.go internal/api/chat.go
git commit -m "feat(agent): inject todo task context into LLM history on each Run()"
```

---

### Task 6: Integration smoke test

**Files:** none (manual verification)

- [ ] **Step 1: Build and start server**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: Verify migration ran cleanly**

```bash
sqlite3 ~/.spider/data/spider.db ".schema todo_tasks"
```
Expected output should NOT contain `turn_id` or `blocked_by`:
```
CREATE TABLE todo_tasks(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  conversation_id TEXT NOT NULL,
  subject TEXT NOT NULL,
  active_form TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  owner TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
)
```

- [ ] **Step 3: Manual test — confirm no duplicate tasks**

1. Open a conversation in the browser at `http://localhost:8002`
2. Send a message that triggers multi-step tasks (e.g., "检查 local110 磁盘用量，清理 30 天前的日志")
3. When agent stops to ask confirmation, reply to confirm
4. Verify: task panel shows the same tasks (not duplicated), agent continues updating existing tasks

- [ ] **Step 4: Commit final**

```bash
git add -A
git commit -m "chore: agent todo context — all tasks complete"
```
