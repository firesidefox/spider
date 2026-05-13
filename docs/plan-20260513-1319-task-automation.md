# Task Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build persistent, schedulable, cross-conversation automated tasks with headless Agent execution and notification support.

**Architecture:** Vertical slice approach — each slice delivers working functionality. Slice 1: manual trigger (DB + Store + Agent tool). Slice 2: cron scheduler. Slice 3: notifications (channels + LLM analysis). Slice 4: frontend UI.

**Tech Stack:** Go, SQLite, robfig/cron v3, Vue 3, existing spider.ai patterns (store/agent/api structure).

---

## Slice 1: Manual Task Execution

### Task 1: Database Schema & Models

**Files:**
- Modify: `internal/db/schema.go:350-355`
- Create: `internal/models/task.go`

- [ ] **Step 1: Add task tables to schema**

Add after topology tables in `internal/db/schema.go` (line ~350):

```go
// Task automation tables
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
	id                   INTEGER PRIMARY KEY AUTOINCREMENT,
	name                 TEXT NOT NULL,
	goal                 TEXT NOT NULL,
	host_ids             TEXT NOT NULL DEFAULT '[]',
	schedule             TEXT NOT NULL DEFAULT '',
	notify_mode          TEXT NOT NULL DEFAULT 'none',
	run_retention_days   INTEGER NOT NULL DEFAULT 30,
	timeout_minutes      INTEGER NOT NULL DEFAULT 30,
	status               TEXT NOT NULL DEFAULT 'active',
	created_at           DATETIME NOT NULL,
	updated_at           DATETIME NOT NULL,
	source_conv_id       TEXT NOT NULL DEFAULT ''
)`); err != nil {
	return err
}
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS task_runs (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id     INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	started_at  DATETIME NOT NULL,
	finished_at DATETIME,
	status      TEXT NOT NULL DEFAULT 'running',
	raw_output  TEXT NOT NULL DEFAULT '',
	summary     TEXT NOT NULL DEFAULT '',
	alerted     INTEGER NOT NULL DEFAULT 0
)`); err != nil {
	return err
}
if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_runs_task_id ON task_runs(task_id)`); err != nil {
	return err
}
```

- [ ] **Step 2: Verify schema compiles**

Run: `go build ./internal/db`
Expected: clean build

- [ ] **Step 3: Create Task and TaskRun models**

Create `internal/models/task.go`:

```go
package models

import "time"

type Task struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Goal             string    `json:"goal"`
	HostIDs          []int64   `json:"host_ids"`
	Schedule         string    `json:"schedule"`
	NotifyMode       string    `json:"notify_mode"`
	RunRetentionDays int       `json:"run_retention_days"`
	TimeoutMinutes   int       `json:"timeout_minutes"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	SourceConvID     string    `json:"source_conv_id"`
}

type TaskRun struct {
	ID         int64      `json:"id"`
	TaskID     int64      `json:"task_id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Status     string     `json:"status"`
	RawOutput  string     `json:"raw_output"`
	Summary    string     `json:"summary"`
	Alerted    bool       `json:"alerted"`
}
```

- [ ] **Step 4: Verify models compile**

Run: `go build ./internal/models`
Expected: clean build

- [ ] **Step 5: Commit schema and models**

```bash
git add internal/db/schema.go internal/models/task.go
git commit -m "feat(task): add Task and TaskRun schema and models"
```

---

### Task 2: TaskStore Implementation

**Files:**
- Create: `internal/store/task_store.go`
- Create: `internal/store/task_store_test.go`

- [ ] **Step 1: Write failing test for TaskStore.Create**

Create `internal/store/task_store_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
)

func TestTaskStore_Create(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)
	task := &models.Task{
		Name:             "test-task",
		Goal:             "test goal",
		HostIDs:          []int64{1, 2},
		Schedule:         "0 2 * * *",
		NotifyMode:       "none",
		RunRetentionDays: 30,
		TimeoutMinutes:   30,
		Status:           "active",
		SourceConvID:     "conv-123",
	}

	created, err := store.Create(task)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store -run TestTaskStore_Create -v`
Expected: FAIL with "undefined: NewTaskStore"

- [ ] **Step 3: Implement TaskStore.Create**

Create `internal/store/task_store.go`:

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

func (s *TaskStore) Create(task *models.Task) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	hostIDsJSON, err := json.Marshal(task.HostIDs)
	if err != nil {
		return nil, err
	}

	result, err := s.db.Exec(`
		INSERT INTO tasks (name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.Name, task.Goal, string(hostIDsJSON), task.Schedule, task.NotifyMode, task.RunRetentionDays, task.TimeoutMinutes, task.Status, now, now, task.SourceConvID)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	task.ID = id
	return task, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store -run TestTaskStore_Create -v`
Expected: PASS

- [ ] **Step 5: Write failing test for TaskStore.Get**

Add to `internal/store/task_store_test.go`:

```go
func TestTaskStore_Get(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)
	task := &models.Task{
		Name:             "test-task",
		Goal:             "test goal",
		HostIDs:          []int64{1, 2},
		Schedule:         "0 2 * * *",
		NotifyMode:       "none",
		RunRetentionDays: 30,
		TimeoutMinutes:   30,
		Status:           "active",
		SourceConvID:     "conv-123",
	}

	created, err := store.Create(task)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Name != task.Name {
		t.Errorf("expected name %q, got %q", task.Name, retrieved.Name)
	}
	if len(retrieved.HostIDs) != 2 {
		t.Errorf("expected 2 host IDs, got %d", len(retrieved.HostIDs))
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/store -run TestTaskStore_Get -v`
Expected: FAIL with "store.Get undefined"

- [ ] **Step 7: Implement TaskStore.Get**

Add to `internal/store/task_store.go`:

```go
func (s *TaskStore) Get(id int64) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var task models.Task
	var hostIDsJSON string

	err := s.db.QueryRow(`
		SELECT id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id
		FROM tasks WHERE id = ?
	`, id).Scan(&task.ID, &task.Name, &task.Goal, &hostIDsJSON, &task.Schedule, &task.NotifyMode, &task.RunRetentionDays, &task.TimeoutMinutes, &task.Status, &task.CreatedAt, &task.UpdatedAt, &task.SourceConvID)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(hostIDsJSON), &task.HostIDs); err != nil {
		return nil, err
	}

	return &task, nil
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/store -run TestTaskStore_Get -v`
Expected: PASS

- [ ] **Step 9: Write failing test for TaskStore.List**

Add to `internal/store/task_store_test.go`:

```go
func TestTaskStore_List(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)
	
	task1 := &models.Task{Name: "task1", Goal: "goal1", HostIDs: []int64{1}, Status: "active"}
	task2 := &models.Task{Name: "task2", Goal: "goal2", HostIDs: []int64{2}, Status: "paused"}
	
	if _, err := store.Create(task1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(task2); err != nil {
		t.Fatal(err)
	}

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/store -run TestTaskStore_List -v`
Expected: FAIL with "store.List undefined"

- [ ] **Step 11: Implement TaskStore.List**

Add to `internal/store/task_store.go`:

```go
func (s *TaskStore) List() ([]*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT id, name, goal, host_ids, schedule, notify_mode, run_retention_days, timeout_minutes, status, created_at, updated_at, source_conv_id
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var hostIDsJSON string

		if err := rows.Scan(&task.ID, &task.Name, &task.Goal, &hostIDsJSON, &task.Schedule, &task.NotifyMode, &task.RunRetentionDays, &task.TimeoutMinutes, &task.Status, &task.CreatedAt, &task.UpdatedAt, &task.SourceConvID); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(hostIDsJSON), &task.HostIDs); err != nil {
			return nil, err
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `go test ./internal/store -run TestTaskStore_List -v`
Expected: PASS

- [ ] **Step 13: Commit TaskStore**

```bash
git add internal/store/task_store.go internal/store/task_store_test.go
git commit -m "feat(task): add TaskStore with Create/Get/List"
```

---

### Task 3: TaskRunStore Implementation

**Files:**
- Create: `internal/store/task_run_store.go`
- Create: `internal/store/task_run_store_test.go`

- [ ] **Step 1: Write failing test for TaskRunStore.Create**

Create `internal/store/task_run_store_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
)

func TestTaskRunStore_Create(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	taskStore := NewTaskStore(database)
	task := &models.Task{Name: "test", Goal: "test", HostIDs: []int64{1}, Status: "active"}
	created, err := taskStore.Create(task)
	if err != nil {
		t.Fatal(err)
	}

	runStore := NewTaskRunStore(database)
	run := &models.TaskRun{
		TaskID:    created.ID,
		StartedAt: time.Now(),
		Status:    "running",
		RawOutput: "",
		Summary:   "",
		Alerted:   false,
	}

	createdRun, err := runStore.Create(run)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if createdRun.ID == 0 {
		t.Error("expected non-zero ID")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store -run TestTaskRunStore_Create -v`
Expected: FAIL with "undefined: NewTaskRunStore"

- [ ] **Step 3: Implement TaskRunStore.Create**

Create `internal/store/task_run_store.go`:

```go
package store

import (
	"database/sql"
	"sync"

	"github.com/spiderai/spider/internal/models"
)

type TaskRunStore struct {
	db *sql.DB
	mu sync.Mutex
}

func NewTaskRunStore(db *sql.DB) *TaskRunStore {
	return &TaskRunStore{db: db}
}

func (s *TaskRunStore) Create(run *models.TaskRun) (*models.TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		INSERT INTO task_runs (task_id, started_at, finished_at, status, raw_output, summary, alerted)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, run.TaskID, run.StartedAt, run.FinishedAt, run.Status, run.RawOutput, run.Summary, run.Alerted)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	run.ID = id
	return run, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store -run TestTaskRunStore_Create -v`
Expected: PASS

- [ ] **Step 5: Write failing test for TaskRunStore.Update**

Add to `internal/store/task_run_store_test.go`:

```go
func TestTaskRunStore_Update(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	taskStore := NewTaskStore(database)
	task := &models.Task{Name: "test", Goal: "test", HostIDs: []int64{1}, Status: "active"}
	created, err := taskStore.Create(task)
	if err != nil {
		t.Fatal(err)
	}

	runStore := NewTaskRunStore(database)
	run := &models.TaskRun{TaskID: created.ID, StartedAt: time.Now(), Status: "running"}
	createdRun, err := runStore.Create(run)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	createdRun.FinishedAt = &now
	createdRun.Status = "success"
	createdRun.Summary = "completed"
	createdRun.Alerted = false

	if err := runStore.Update(createdRun); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, err := runStore.Get(createdRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.Status != "success" {
		t.Errorf("expected status success, got %s", retrieved.Status)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/store -run TestTaskRunStore_Update -v`
Expected: FAIL with "runStore.Update undefined"

- [ ] **Step 7: Implement TaskRunStore.Update and Get**

Add to `internal/store/task_run_store.go`:

```go
func (s *TaskRunStore) Update(run *models.TaskRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE task_runs SET finished_at = ?, status = ?, raw_output = ?, summary = ?, alerted = ?
		WHERE id = ?
	`, run.FinishedAt, run.Status, run.RawOutput, run.Summary, run.Alerted, run.ID)
	return err
}

func (s *TaskRunStore) Get(id int64) (*models.TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var run models.TaskRun
	err := s.db.QueryRow(`
		SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
		FROM task_runs WHERE id = ?
	`, id).Scan(&run.ID, &run.TaskID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.RawOutput, &run.Summary, &run.Alerted)
	if err != nil {
		return nil, err
	}
	return &run, nil
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/store -run TestTaskRunStore_Update -v`
Expected: PASS

- [ ] **Step 9: Write failing test for TaskRunStore.ListByTask**

Add to `internal/store/task_run_store_test.go`:

```go
func TestTaskRunStore_ListByTask(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	taskStore := NewTaskStore(database)
	task := &models.Task{Name: "test", Goal: "test", HostIDs: []int64{1}, Status: "active"}
	created, err := taskStore.Create(task)
	if err != nil {
		t.Fatal(err)
	}

	runStore := NewTaskRunStore(database)
	run1 := &models.TaskRun{TaskID: created.ID, StartedAt: time.Now(), Status: "running"}
	run2 := &models.TaskRun{TaskID: created.ID, StartedAt: time.Now().Add(time.Minute), Status: "success"}
	
	if _, err := runStore.Create(run1); err != nil {
		t.Fatal(err)
	}
	if _, err := runStore.Create(run2); err != nil {
		t.Fatal(err)
	}

	runs, err := runStore.ListByTask(created.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListByTask failed: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/store -run TestTaskRunStore_ListByTask -v`
Expected: FAIL with "runStore.ListByTask undefined"

- [ ] **Step 11: Implement TaskRunStore.ListByTask**

Add to `internal/store/task_run_store.go`:

```go
func (s *TaskRunStore) ListByTask(taskID int64, limit, offset int) ([]*models.TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
		FROM task_runs WHERE task_id = ?
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`, taskID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*models.TaskRun
	for rows.Next() {
		var run models.TaskRun
		if err := rows.Scan(&run.ID, &run.TaskID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.RawOutput, &run.Summary, &run.Alerted); err != nil {
			return nil, err
		}
		runs = append(runs, &run)
	}

	return runs, rows.Err()
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `go test ./internal/store -run TestTaskRunStore_ListByTask -v`
Expected: PASS

- [ ] **Step 13: Commit TaskRunStore**

```bash
git add internal/store/task_run_store.go internal/store/task_run_store_test.go
git commit -m "feat(task): add TaskRunStore with Create/Update/Get/ListByTask"
```

---

### Task 4: CreateTask Agent Tool

**Files:**
- Create: `internal/agent/tools_task.go`
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Write CreateTask tool skeleton**

Create `internal/agent/tools_task.go`:

```go
package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type CreateTaskTool struct {
	taskStore *store.TaskStore
}

func NewCreateTaskTool(taskStore *store.TaskStore) *CreateTaskTool {
	return &CreateTaskTool{taskStore: taskStore}
}

func (t *CreateTaskTool) Name() string {
	return "CreateTask"
}

func (t *CreateTaskTool) Description() string {
	return "Save a confirmed automated task. Has side effects. Call only after user has confirmed all fields."
}

type CreateTaskInput struct {
	Name             string  `json:"name" jsonschema:"required,description=Task name"`
	Goal             string  `json:"goal" jsonschema:"required,description=Natural language goal"`
	HostIDs          []int64 `json:"host_ids" jsonschema:"required,description=Target device IDs"`
	Schedule         string  `json:"schedule" jsonschema:"description=Cron expression (empty = manual only)"`
	NotifyMode       string  `json:"notify_mode" jsonschema:"description=none|failure|complete|anomaly,default=none"`
	RunRetentionDays int     `json:"run_retention_days" jsonschema:"description=TaskRun retention days,default=30"`
	TimeoutMinutes   int     `json:"timeout_minutes" jsonschema:"description=Execution timeout minutes,default=30"`
}

func (t *CreateTaskTool) InputSchema() any {
	return &CreateTaskInput{}
}

- [ ] **Step 2: Implement CreateTask.Execute**

Add to `internal/agent/tools_task.go`:

```go
func (t *CreateTaskTool) Execute(ctx ToolContext, inputJSON string) (string, error) {
	var input CreateTaskInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.NotifyMode == "" {
		input.NotifyMode = "none"
	}
	if input.RunRetentionDays == 0 {
		input.RunRetentionDays = 30
	}
	if input.TimeoutMinutes == 0 {
		input.TimeoutMinutes = 30
	}

	task := &models.Task{
		Name:             input.Name,
		Goal:             input.Goal,
		HostIDs:          input.HostIDs,
		Schedule:         input.Schedule,
		NotifyMode:       input.NotifyMode,
		RunRetentionDays: input.RunRetentionDays,
		TimeoutMinutes:   input.TimeoutMinutes,
		Status:           "active",
		SourceConvID:     ctx.ConversationID,
	}

	created, err := t.taskStore.Create(task)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	return fmt.Sprintf("Task created: ID=%d, Name=%s", created.ID, created.Name), nil
}
```

- [ ] **Step 3: Register CreateTask tool in Factory**

Add to `internal/agent/factory.go` after existing tool registrations:

```go
if f.TaskStore != nil {
	tools = append(tools, NewCreateTaskTool(f.TaskStore))
}
```

- [ ] **Step 4: Add TaskStore field to Factory**

Modify `internal/agent/factory.go` Factory struct:

```go
type Factory struct {
	// ... existing fields ...
	TaskStore *store.TaskStore
}

- [ ] **Step 5: Wire TaskStore in main.go**

Modify `cmd/spider/main.go` after creating other stores:

```go
taskStore := store.NewTaskStore(database)
app.TaskStore = taskStore
```

And in AgentFactory initialization:

```go
if agentFactory != nil {
	agentFactory.TaskStore = taskStore
}
```

- [ ] **Step 6: Add TaskStore to App struct**

Modify `internal/mcp/server.go` App struct:

```go
type App struct {
	// ... existing fields ...
	TaskStore *store.TaskStore
}
```

- [ ] **Step 7: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 8: Commit CreateTask tool**

```bash
git add internal/agent/tools_task.go internal/agent/factory.go cmd/spider/main.go internal/mcp/server.go
git commit -m "feat(task): add CreateTask agent tool"
```

---

### Task 5: Manual Task Trigger API

**Files:**
- Create: `internal/api/task_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write task handler skeleton**

Create `internal/api/task_handler.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/spiderai/spider/internal/store"
)

type TaskHandler struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
}

func NewTaskHandler(taskStore *store.TaskStore, taskRunStore *store.TaskRunStore) *TaskHandler {
	return &TaskHandler{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
	}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.taskStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

- [ ] **Step 2: Add Get handler**

Add to `internal/api/task_handler.go`:

```go
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	task, err := h.taskStore.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}
```

- [ ] **Step 3: Add Trigger handler stub**

Add to `internal/api/task_handler.go`:

```go
func (h *TaskHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	// TODO: implement manual trigger in Slice 1 Task 6
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "triggered", "task_id": idStr})
}
```

- [ ] **Step 4: Register routes**

Modify `internal/api/router.go` in NewRouter function:

```go
taskHandler := NewTaskHandler(app.TaskStore, app.TaskRunStore)
mux.HandleFunc("GET /api/tasks", taskHandler.List)
mux.HandleFunc("GET /api/tasks/{id}", taskHandler.Get)
mux.HandleFunc("POST /api/tasks/{id}/trigger", taskHandler.Trigger)
```

- [ ] **Step 5: Add TaskRunStore to App**

Modify `internal/mcp/server.go` App struct:

```go
type App struct {
	// ... existing fields ...
	TaskRunStore *store.TaskRunStore
}
```

- [ ] **Step 6: Wire TaskRunStore in main.go**

Add to `cmd/spider/main.go`:

```go
taskRunStore := store.NewTaskRunStore(database)
app.TaskRunStore = taskRunStore
```

- [ ] **Step 7: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 8: Commit API handlers**

```bash
git add internal/api/task_handler.go internal/api/router.go internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(task): add task API endpoints (list/get/trigger stub)"
```

---

### Task 6: Manual Trigger Execution

**Files:**
- Create: `internal/scheduler/executor.go`
- Modify: `internal/api/task_handler.go`

- [ ] **Step 1: Write executor skeleton**

Create `internal/scheduler/executor.go`:

```go
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type Executor struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	hostStore    *store.HostStore
	agentFactory *agent.Factory
}

func NewExecutor(
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	hostStore *store.HostStore,
	agentFactory *agent.Factory,
) *Executor {
	return &Executor{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
		hostStore:    hostStore,
		agentFactory: agentFactory,
	}
}
```

- [ ] **Step 2: Implement Execute method**

Add to `internal/scheduler/executor.go`:

```go
func (e *Executor) Execute(ctx context.Context, taskID int64) (*models.TaskRun, error) {
	task, err := e.taskStore.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	run := &models.TaskRun{
		TaskID:    taskID,
		StartedAt: time.Now(),
		Status:    "running",
		RawOutput: "",
		Summary:   "",
		Alerted:   false,
	}

	created, err := e.taskRunStore.Create(run)
	if err != nil {
		return nil, fmt.Errorf("failed to create task run: %w", err)
	}

	go e.executeAsync(ctx, task, created)

	return created, nil
}

- [ ] **Step 3: Implement executeAsync stub**

Add to `internal/scheduler/executor.go`:

```go
func (e *Executor) executeAsync(ctx context.Context, task *models.Task, run *models.TaskRun) {
	// Filter valid host IDs
	validHostIDs := e.filterValidHosts(task.HostIDs)
	if len(validHostIDs) == 0 {
		run.Status = "failed"
		run.RawOutput = fmt.Sprintf("all hosts invalid: %v", task.HostIDs)
		run.Alerted = true
		now := time.Now()
		run.FinishedAt = &now
		e.taskRunStore.Update(run)
		return
	}

	// Create context with timeout
	execCtx := ctx
	if task.TimeoutMinutes > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutMinutes)*time.Minute)
		defer cancel()
	}

	// Build system prompt with task goal and host info
	hostInfo := ""
	for _, id := range validHostIDs {
		if host, err := e.hostStore.Get(id); err == nil {
			hostInfo += fmt.Sprintf("- Host %d: %s (%s)\n", host.ID, host.Name, host.IP)
		}
	}
	systemPrompt := fmt.Sprintf("Task: %s\n\nTarget hosts:\n%s\nExecute the task and report results.", task.Goal, hostInfo)

	// Execute headless agent
	output := ""
	agent := e.agentFactory.NewAgent(execCtx, "", systemPrompt)
	
	// Run agent (simplified - actual implementation needs message loop)
	select {
	case <-execCtx.Done():
		// Timeout
		run.Status = "failed"
		run.RawOutput = fmt.Sprintf("execution timeout after %dm", task.TimeoutMinutes)
		run.Alerted = true
		now := time.Now()
		run.FinishedAt = &now
		e.taskRunStore.Update(run)
		e.sendNotifications(task, run)
		return
	default:
		// Execution completed
		output = "execution output placeholder"
	}

	run.RawOutput = output
	
	// LLM analysis for summary
	run.Summary = e.generateSummary(output)
	
	// LLM anomaly detection if needed
	if task.NotifyMode == "anomaly" {
		run.Alerted = e.detectAnomaly(output)
	}

	run.Status = "success"
	now := time.Now()
	run.FinishedAt = &now
	e.taskRunStore.Update(run)
	e.sendNotifications(task, run)
}

func (e *Executor) generateSummary(output string) string {
	// TODO: call LLM with system default provider
	return "Summary: " + output[:min(len(output), 100)]
}

func (e *Executor) detectAnomaly(output string) bool {
	// TODO: call LLM to judge if anomalous
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *Executor) filterValidHosts(hostIDs []int64) []int64 {
	var valid []int64
	for _, id := range hostIDs {
		if _, err := e.hostStore.Get(id); err == nil {
			valid = append(valid, id)
		}
	}
	return valid
}
```

- [ ] **Step 4: Wire Executor in TaskHandler**

Modify `internal/api/task_handler.go` struct:

```go
type TaskHandler struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	executor     *scheduler.Executor
}

func NewTaskHandler(taskStore *store.TaskStore, taskRunStore *store.TaskRunStore, executor *scheduler.Executor) *TaskHandler {
	return &TaskHandler{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
		executor:     executor,
	}
}
```

- [ ] **Step 5: Implement Trigger handler**

Replace Trigger method in `internal/api/task_handler.go`:

```go
func (h *TaskHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}

	run, err := h.executor.Execute(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"run_id": run.ID, "status": "started"})
}

- [ ] **Step 6: Wire Executor in router**

Modify `internal/api/router.go`:

```go
executor := scheduler.NewExecutor(app.TaskStore, app.TaskRunStore, app.HostStore, app.AgentFactory)
taskHandler := NewTaskHandler(app.TaskStore, app.TaskRunStore, executor)
```

- [ ] **Step 7: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 8: Commit executor**

```bash
git add internal/scheduler/executor.go internal/api/task_handler.go internal/api/router.go
git commit -m "feat(task): add manual trigger executor with host validation"
```

---

## Slice 2: Cron Scheduling

### Task 7: Cron Scheduler

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Write scheduler skeleton**

Create `internal/scheduler/scheduler.go`:

```go
package scheduler

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/store"
)

type Scheduler struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	executor     *Executor
	db           *sql.DB
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewScheduler(
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	executor *Executor,
	db *sql.DB,
) *Scheduler {
	return &Scheduler{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
		executor:     executor,
		db:           db,
		stopCh:       make(chan struct{}),
	}
}
```

- [ ] **Step 2: Implement Start method**

Add to `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.run(ctx)
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

- [ ] **Step 3: Implement run loop**

Add to `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	tasks, err := s.taskStore.List()
	if err != nil {
		logger.Global().Error().Err(err).Msg("scheduler: failed to list tasks")
		return
	}

	for _, task := range tasks {
		if task.Status != "active" || task.Schedule == "" {
			continue
		}

		if s.isDue(task) {
			s.tryTrigger(ctx, task)
		}
	}
}
```

- [ ] **Step 4: Implement isDue and tryTrigger**

Add to `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) isDue(task *models.Task) bool {
	schedule, err := cron.ParseStandard(task.Schedule)
	if err != nil {
		logger.Global().Warn().Err(err).Int64("task_id", task.ID).Msg("invalid cron schedule")
		return false
	}

	next := schedule.Next(task.UpdatedAt)
	return time.Now().After(next)
}

func (s *Scheduler) tryTrigger(ctx context.Context, task *models.Task) {
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM task_runs WHERE task_id = ? AND status = 'running'", task.ID).Scan(&count)
	if err != nil || count > 0 {
		logger.Global().Info().Int64("task_id", task.ID).Msg("skipped: previous run still running")
		return
	}

	if err := tx.Commit(); err != nil {
		return
	}

	if _, err := s.executor.Execute(ctx, task.ID); err != nil {
		logger.Global().Error().Err(err).Int64("task_id", task.ID).Msg("failed to trigger task")
	}
}

- [ ] **Step 5: Wire scheduler in main.go**

Add to `cmd/spider/main.go` after creating executor:

```go
executor := scheduler.NewExecutor(taskStore, taskRunStore, hs, agentFactory)
sched := scheduler.NewScheduler(taskStore, taskRunStore, executor, database)
sched.Start(shutdownCtx)
defer sched.Stop()
```

- [ ] **Step 6: Add robfig/cron dependency**

Run: `go get github.com/robfig/cron/v3`
Expected: dependency added

- [ ] **Step 7: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 8: Commit scheduler**

```bash
git add internal/scheduler/scheduler.go cmd/spider/main.go go.mod go.sum
git commit -m "feat(task): add cron scheduler with minute-level polling"
```

---

## Slice 3: Notification System

### Task 8: NotifyChannel Model and Store

**Files:**
- Modify: `internal/db/schema.go`
- Create: `internal/models/notify_channel.go`
- Create: `internal/store/notify_channel_store.go`

- [ ] **Step 1: Add notify_channels table**

Add to `internal/db/schema.go` after task_runs table:

```go
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS notify_channels (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	type       TEXT NOT NULL,
	name       TEXT NOT NULL,
	config     TEXT NOT NULL,
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME NOT NULL
)`); err != nil {
	return err
}
```

- [ ] **Step 2: Verify schema compiles**

Run: `go build ./internal/db`
Expected: clean build

- [ ] **Step 3: Create NotifyChannel model**

Create `internal/models/notify_channel.go`:

```go
package models

import "time"

type NotifyChannel struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Config    string    `json:"config"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 4: Verify model compiles**

Run: `go build ./internal/models`
Expected: clean build

- [ ] **Step 5: Create NotifyChannelStore**

Create `internal/store/notify_channel_store.go`:

```go
package store

import (
	"database/sql"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

type NotifyChannelStore struct {
	db *sql.DB
	cm *crypto.Manager
	mu sync.Mutex
}

func NewNotifyChannelStore(db *sql.DB, cm *crypto.Manager) *NotifyChannelStore {
	return &NotifyChannelStore{db: db, cm: cm}
}

func (s *NotifyChannelStore) Create(ch *models.NotifyChannel) (*models.NotifyChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	encrypted, err := s.cm.Encrypt([]byte(ch.Config))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ch.CreatedAt = now

	result, err := s.db.Exec(`
		INSERT INTO notify_channels (type, name, config, enabled, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, ch.Type, ch.Name, encrypted, ch.Enabled, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	ch.ID = id
	return ch, nil
}

func (s *NotifyChannelStore) List() ([]*models.NotifyChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT id, type, name, config, enabled, created_at FROM notify_channels ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*models.NotifyChannel
	for rows.Next() {
		var ch models.NotifyChannel
		var encryptedConfig []byte
		var enabled int

		if err := rows.Scan(&ch.ID, &ch.Type, &ch.Name, &encryptedConfig, &enabled, &ch.CreatedAt); err != nil {
			return nil, err
		}

		decrypted, err := s.cm.Decrypt(encryptedConfig)
		if err != nil {
			return nil, err
		}
		ch.Config = string(decrypted)
		ch.Enabled = enabled == 1

		channels = append(channels, &ch)
	}

	return channels, rows.Err()
}
```

- [ ] **Step 6: Build and verify**

Run: `go build ./internal/store`
Expected: clean build

- [ ] **Step 7: Commit NotifyChannel**

```bash
git add internal/db/schema.go internal/models/notify_channel.go internal/store/notify_channel_store.go
git commit -m "feat(task): add NotifyChannel model and store with encryption"
```

---

### Task 9: Notification Sender

**Files:**
- Create: `internal/notify/sender.go`
- Create: `internal/notify/dingtalk.go`

- [ ] **Step 1: Write sender interface**

Create `internal/notify/sender.go`:

```go
package notify

import (
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/models"
)

type Sender interface {
	Send(message string) error
}

func NewSender(ch *models.NotifyChannel) (Sender, error) {
	switch ch.Type {
	case "dingtalk":
		return NewDingTalkSender(ch.Config)
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}
```

- [ ] **Step 2: Implement DingTalk sender**

Create `internal/notify/dingtalk.go`:

```go
package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DingTalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

type DingTalkSender struct {
	config DingTalkConfig
}

func NewDingTalkSender(configJSON string) (*DingTalkSender, error) {
	var cfg DingTalkConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, err
	}
	return &DingTalkSender{config: cfg}, nil
}

func (s *DingTalkSender) Send(message string) error {
	timestamp := time.Now().UnixMilli()
	sign := s.sign(timestamp)

	url := fmt.Sprintf("%s&timestamp=%d&sign=%s", s.config.WebhookURL, timestamp, sign)

	payload := map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": message,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *DingTalkSender) sign(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, s.config.Secret)
	h := hmac.New(sha256.New, []byte(s.config.Secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

- [ ] **Step 3: Build and verify**

Run: `go build ./internal/notify`
Expected: clean build

- [ ] **Step 4: Commit notification sender**

```bash
git add internal/notify/sender.go internal/notify/dingtalk.go
git commit -m "feat(task): add notification sender with DingTalk support"
```

---

### Task 10: Integrate Notifications into Executor

**Files:**
- Modify: `internal/scheduler/executor.go`

- [ ] **Step 1: Add NotifyChannelStore to Executor**

Modify `internal/scheduler/executor.go` struct:

```go
type Executor struct {
	taskStore          *store.TaskStore
	taskRunStore       *store.TaskRunStore
	hostStore          *store.HostStore
	agentFactory       *agent.Factory
	notifyChannelStore *store.NotifyChannelStore
}

func NewExecutor(
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	hostStore *store.HostStore,
	agentFactory *agent.Factory,
	notifyChannelStore *store.NotifyChannelStore,
) *Executor {
	return &Executor{
		taskStore:          taskStore,
		taskRunStore:       taskRunStore,
		hostStore:          hostStore,
		agentFactory:       agentFactory,
		notifyChannelStore: notifyChannelStore,
	}
}
```

- [ ] **Step 2: Add sendNotifications method**

Add to `internal/scheduler/executor.go`:

```go
func (e *Executor) sendNotifications(task *models.Task, run *models.TaskRun) {
	if task.NotifyMode == "none" {
		return
	}

	shouldNotify := false
	switch task.NotifyMode {
	case "failure":
		shouldNotify = run.Status == "failed"
	case "complete":
		shouldNotify = true
	case "anomaly":
		shouldNotify = run.Alerted
	}

	if !shouldNotify {
		return
	}

	channels, err := e.notifyChannelStore.List()
	if err != nil {
		logger.Global().Error().Err(err).Msg("failed to list notify channels")
		return
	}

	message := fmt.Sprintf("Task: %s\nStatus: %s\nSummary: %s", task.Name, run.Status, run.Summary)

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		sender, err := notify.NewSender(ch)
		if err != nil {
			logger.Global().Error().Err(err).Str("channel", ch.Name).Msg("failed to create sender")
			continue
		}

		if err := sender.Send(message); err != nil {
			logger.Global().Error().Err(err).Str("channel", ch.Name).Msg("failed to send notification")
		}
	}
}
```

- [ ] **Step 3: Call sendNotifications from executeAsync**

Modify `executeAsync` in `internal/scheduler/executor.go` to call sendNotifications after updating TaskRun:

```go
func (e *Executor) executeAsync(ctx context.Context, task *models.Task, run *models.TaskRun) {
	// Filter valid host IDs
	validHostIDs := e.filterValidHosts(task.HostIDs)
	if len(validHostIDs) == 0 {
		run.Status = "failed"
		run.RawOutput = fmt.Sprintf("all hosts invalid: %v", task.HostIDs)
		run.Alerted = true
		now := time.Now()
		run.FinishedAt = &now
		e.taskRunStore.Update(run)
		e.sendNotifications(task, run)
		return
	}

	// TODO: implement headless agent execution in next step
	run.Status = "success"
	run.RawOutput = "execution placeholder"
	run.Summary = "completed"
	now := time.Now()
	run.FinishedAt = &now
	e.taskRunStore.Update(run)
	e.sendNotifications(task, run)
}
```

- [ ] **Step 4: Wire NotifyChannelStore in main.go**

Add to `cmd/spider/main.go` after creating other stores:

```go
notifyChannelStore := store.NewNotifyChannelStore(database, cm)
app.NotifyChannelStore = notifyChannelStore
```

And update executor creation:

```go
executor := scheduler.NewExecutor(taskStore, taskRunStore, hs, agentFactory, notifyChannelStore)
```

- [ ] **Step 5: Add NotifyChannelStore to App**

Modify `internal/mcp/server.go` App struct:

```go
type App struct {
	// ... existing fields ...
	NotifyChannelStore *store.NotifyChannelStore
}
```

- [ ] **Step 6: Update router.go executor creation**

Modify `internal/api/router.go`:

```go
executor := scheduler.NewExecutor(app.TaskStore, app.TaskRunStore, app.HostStore, app.AgentFactory, app.NotifyChannelStore)
```

- [ ] **Step 7: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 8: Commit notification integration**

```bash
git add internal/scheduler/executor.go cmd/spider/main.go internal/mcp/server.go internal/api/router.go
git commit -m "feat(task): integrate notifications into executor"
```

---

### Task 14: Headless Agent Execution

**Files:**
- Modify: `internal/scheduler/executor.go`

- [ ] **Step 1: Replace executeAsync stub with full implementation**

Replace the executeAsync method in `internal/scheduler/executor.go`:

```go
func (e *Executor) executeAsync(ctx context.Context, task *models.Task, run *models.TaskRun) {
	// Filter valid host IDs
	validHostIDs := e.filterValidHosts(task.HostIDs)
	if len(validHostIDs) == 0 {
		run.Status = "failed"
		run.RawOutput = fmt.Sprintf("all hosts invalid: %v", task.HostIDs)
		run.Alerted = true
		now := time.Now()
		run.FinishedAt = &now
		e.taskRunStore.Update(run)
		e.sendNotifications(task, run)
		return
	}

	// Create context with timeout
	execCtx := ctx
	if task.TimeoutMinutes > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutMinutes)*time.Minute)
		defer cancel()
	}

	// Build system prompt with task goal and host info
	hostInfo := ""
	for _, id := range validHostIDs {
		if host, err := e.hostStore.Get(id); err == nil {
			hostInfo += fmt.Sprintf("- Host %d: %s (%s)\n", host.ID, host.Name, host.IP)
		}
	}
	systemPrompt := fmt.Sprintf("Task: %s\n\nTarget hosts:\n%s\nExecute the task and report results.", task.Goal, hostInfo)

	// Execute headless agent
	ag := e.agentFactory.NewAgent(execCtx, "", systemPrompt)
	output, err := ag.Run()
	
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			// Timeout
			run.Status = "failed"
			run.RawOutput = fmt.Sprintf("execution timeout after %dm", task.TimeoutMinutes)
			run.Alerted = true
		} else {
			// Other error
			run.Status = "failed"
			run.RawOutput = fmt.Sprintf("execution error: %v", err)
			run.Alerted = true
		}
		now := time.Now()
		run.FinishedAt = &now
		e.taskRunStore.Update(run)
		e.sendNotifications(task, run)
		return
	}

	run.RawOutput = output
	
	// LLM analysis for summary
	run.Summary = e.generateSummary(output)
	
	// LLM anomaly detection if needed
	if task.NotifyMode == "anomaly" {
		run.Alerted = e.detectAnomaly(output)
	}

	run.Status = "success"
	now := time.Now()
	run.FinishedAt = &now
	e.taskRunStore.Update(run)
	e.sendNotifications(task, run)
}
```

- [ ] **Step 2: Add generateSummary stub**

Add to `internal/scheduler/executor.go`:

```go
func (e *Executor) generateSummary(output string) string {
	// TODO: implement LLM summary generation in Task 15
	if len(output) > 100 {
		return "Summary: " + output[:100] + "..."
	}
	return "Summary: " + output
}
```

- [ ] **Step 3: Add detectAnomaly stub**

Add to `internal/scheduler/executor.go`:

```go
func (e *Executor) detectAnomaly(output string) bool {
	// TODO: implement LLM anomaly detection in Task 15
	return false
}
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 5: Commit headless agent execution**

```bash
git add internal/scheduler/executor.go
git commit -m "feat(task): implement headless agent execution with timeout"
```

---

### Task 15: LLM Analysis for Summary and Anomaly Detection

**Files:**
- Modify: `internal/scheduler/executor.go`

- [ ] **Step 1: Implement generateSummary with LLM**

Replace generateSummary in `internal/scheduler/executor.go`:

```go
func (e *Executor) generateSummary(output string) string {
	if e.agentFactory == nil {
		return "Summary: " + truncate(output, 100)
	}

	prompt := fmt.Sprintf("Summarize this task execution output in 2-3 sentences:\n\n%s", output)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	ag := e.agentFactory.NewAgent(ctx, "", "You are a task execution summarizer.")
	summary, err := ag.RunSinglePrompt(prompt)
	if err != nil {
		logger.Global().Error().Err(err).Msg("failed to generate summary")
		return "Summary: " + truncate(output, 100)
	}
	
	return summary
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

- [ ] **Step 2: Implement detectAnomaly with LLM**

Replace detectAnomaly in `internal/scheduler/executor.go`:

```go
func (e *Executor) detectAnomaly(output string) bool {
	if e.agentFactory == nil {
		return false
	}

	prompt := fmt.Sprintf(`Analyze this task execution output and determine if it indicates an anomaly (errors, failures, unexpected values).

Output:
%s

Respond with only "YES" if anomalous, or "NO" if normal.`, output)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	ag := e.agentFactory.NewAgent(ctx, "", "You are a task execution anomaly detector.")
	response, err := ag.RunSinglePrompt(prompt)
	if err != nil {
		logger.Global().Error().Err(err).Msg("failed to detect anomaly")
		return false
	}
	
	return strings.Contains(strings.ToUpper(response), "YES")
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 4: Commit LLM analysis**

```bash
git add internal/scheduler/executor.go
git commit -m "feat(task): implement LLM summary generation and anomaly detection"
```

---

### Task 16: TaskRun Retention Cleanup

**Files:**
- Modify: `internal/scheduler/scheduler.go`

- [ ] **Step 1: Add cleanup goroutine to Start**

Modify Start method in `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(2)
	go s.run(ctx)
	go s.runCleanup(ctx)
}
```

- [ ] **Step 2: Implement runCleanup**

Add to `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) runCleanup(ctx context.Context) {
	defer s.wg.Done()
	
	// Calculate next midnight
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-timer.C:
			s.cleanupOldRuns()
			// Reset timer for next midnight
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer.Reset(time.Until(next))
		}
	}
}
```

- [ ] **Step 3: Implement cleanupOldRuns**

Add to `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) cleanupOldRuns() {
	tasks, err := s.taskStore.List()
	if err != nil {
		logger.Global().Error().Err(err).Msg("cleanup: failed to list tasks")
		return
	}

	for _, task := range tasks {
		if task.RunRetentionDays == 0 {
			continue // Permanent retention
		}

		cutoff := time.Now().AddDate(0, 0, -task.RunRetentionDays)
		result, err := s.db.Exec(
			"DELETE FROM task_runs WHERE task_id = ? AND started_at < ?",
			task.ID, cutoff,
		)
		if err != nil {
			logger.Global().Error().Err(err).Int64("task_id", task.ID).Msg("cleanup: failed to delete old runs")
			continue
		}

		rows, _ := result.RowsAffected()
		if rows > 0 {
			logger.Global().Info().Int64("task_id", task.ID).Int64("deleted", rows).Msg("cleanup: deleted old task runs")
		}
	}
}
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 5: Commit cleanup**

```bash
git add internal/scheduler/scheduler.go
git commit -m "feat(task): implement TaskRun retention cleanup"
```

---

## Plan Complete

### Task 11: Task Management Page

**Files:**
- Create: `web/src/views/TaskView.vue`
- Create: `web/src/api/task.ts`
- Modify: `web/src/router/index.ts`

- [ ] **Step 1: Write task API client**

Create `web/src/api/task.ts`:

```typescript
import { authHeaders } from './auth'

export interface Task {
  id: number
  name: string
  goal: string
  host_ids: number[]
  schedule: string
  notify_mode: string
  run_retention_days: number
  timeout_minutes: number
  status: string
  created_at: string
  updated_at: string
  source_conv_id: string
}

export interface TaskRun {
  id: number
  task_id: number
  started_at: string
  finished_at?: string
  status: string
  raw_output: string
  summary: string
  alerted: boolean
}

export async function listTasks(): Promise<Task[]> {
  const res = await fetch('/api/tasks', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function getTask(id: number): Promise<Task> {
  const res = await fetch(`/api/tasks/${id}`, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function triggerTask(id: number): Promise<{ run_id: number; status: string }> {
  const res = await fetch(`/api/tasks/${id}/trigger`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}
```

- [ ] **Step 2: Build and verify API client**

Run: `cd web && npm run build`
Expected: clean build

- [ ] **Step 3: Write TaskView skeleton**

Create `web/src/views/TaskView.vue`:

```vue
<template>
  <div class="task-view">
    <div class="left-panel">
      <div class="header">
        <h2>任务</h2>
      </div>
      <div class="task-list">
        <div
          v-for="task in tasks"
          :key="task.id"
          class="task-item"
          :class="{ active: selectedTaskId === task.id }"
          @click="selectTask(task.id)"
        >
          <div class="task-name">{{ task.name }}</div>
          <div class="task-meta">
            <span class="badge" :class="task.status">{{ task.status }}</span>
            <span v-if="task.schedule" class="schedule">{{ task.schedule }}</span>
            <span v-else class="schedule">手动</span>
          </div>
        </div>
      </div>
    </div>

    <div class="right-panel" v-if="selectedTask">
      <div class="task-header">
        <h3>{{ selectedTask.name }}</h3>
        <button @click="handleTrigger" class="btn-primary">立即执行</button>
      </div>
      <div class="task-details">
        <div class="detail-row">
          <span class="label">目标:</span>
          <span class="value">{{ selectedTask.goal }}</span>
        </div>
        <div class="detail-row">
          <span class="label">调度:</span>
          <span class="value">{{ selectedTask.schedule || '手动触发' }}</span>
        </div>
        <div class="detail-row">
          <span class="label">通知模式:</span>
          <span class="value">{{ selectedTask.notify_mode }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTasks, getTask, triggerTask, type Task } from '@/api/task'

const tasks = ref<Task[]>([])
const selectedTaskId = ref<number | null>(null)
const selectedTask = ref<Task | null>(null)

async function loadTasks() {
  tasks.value = await listTasks()
  if (tasks.value.length > 0 && !selectedTaskId.value) {
    selectTask(tasks.value[0].id)
  }
}

async function selectTask(id: number) {
  selectedTaskId.value = id
  selectedTask.value = await getTask(id)
}

async function handleTrigger() {
  if (!selectedTaskId.value) return
  await triggerTask(selectedTaskId.value)
  alert('任务已触发')
}

onMounted(() => {
  loadTasks()
})
</script>

<style scoped>
.task-view {
  display: flex;
  height: 100%;
}

.left-panel {
  width: 300px;
  border-right: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
}

.header {
  padding: 16px;
  border-bottom: 1px solid #e0e0e0;
}

.task-list {
  flex: 1;
  overflow-y: auto;
}

.task-item {
  padding: 12px 16px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
}

.task-item:hover {
  background: #f5f5f5;
}

.task-item.active {
  background: #e3f2fd;
}

.task-name {
  font-weight: 500;
  margin-bottom: 4px;
}

.task-meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: #666;
}

.badge {
  padding: 2px 6px;
  border-radius: 3px;
  background: #e0e0e0;
}

.badge.active {
  background: #4caf50;
  color: white;
}

.right-panel {
  flex: 1;
  padding: 16px;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.task-details {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-row {
  display: flex;
  gap: 8px;
}

.label {
  font-weight: 500;
  min-width: 80px;
}

.btn-primary {
  padding: 8px 16px;
  background: #1976d2;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-primary:hover {
  background: #1565c0;
}
</style>
```

- [ ] **Step 4: Register route**

Add to `web/src/router/index.ts`:

```typescript
{
  path: '/tasks',
  name: 'tasks',
  component: () => import('@/views/TaskView.vue'),
}
```

- [ ] **Step 5: Build and verify**

Run: `cd web && npm run build`
Expected: clean build

- [ ] **Step 6: Test in browser**

Run: `go build -a -o /tmp/spider-test ./cmd/spider && /tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data`
Navigate to: `http://localhost:8002/tasks`
Expected: Task list page renders

- [ ] **Step 7: Commit task management page**

```bash
git add web/src/views/TaskView.vue web/src/api/task.ts web/src/router/index.ts
git commit -m "feat(task): add task management page with list and trigger"
```

---

### Task 12: NotifyChannel Management Page

**Files:**
- Create: `web/src/views/SettingsView.vue`
- Create: `web/src/api/notify.ts`
- Modify: `web/src/router/index.ts`

- [ ] **Step 1: Write notify API client**

Create `web/src/api/notify.ts`:

```typescript
import { authHeaders } from './auth'

export interface NotifyChannel {
  id: number
  type: string
  name: string
  config: string
  enabled: boolean
  created_at: string
}

export async function listNotifyChannels(): Promise<NotifyChannel[]> {
  const res = await fetch('/api/notify-channels', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createNotifyChannel(ch: Omit<NotifyChannel, 'id' | 'created_at'>): Promise<NotifyChannel> {
  const res = await fetch('/api/notify-channels', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(ch),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteNotifyChannel(id: number): Promise<void> {
  const res = await fetch(`/api/notify-channels/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}
```

- [ ] **Step 2: Write SettingsView skeleton**

Create `web/src/views/SettingsView.vue`:

```vue
<template>
  <div class="settings-view">
    <h2>通知渠道设置</h2>
    <button @click="showAddDialog = true" class="btn-primary">添加渠道</button>

    <div class="channel-list">
      <div v-for="ch in channels" :key="ch.id" class="channel-item">
        <div class="channel-info">
          <div class="channel-name">{{ ch.name }}</div>
          <div class="channel-type">{{ ch.type }}</div>
        </div>
        <button @click="handleDelete(ch.id)" class="btn-danger">删除</button>
      </div>
    </div>

    <div v-if="showAddDialog" class="dialog-overlay" @click="showAddDialog = false">
      <div class="dialog" @click.stop>
        <h3>添加通知渠道</h3>
        <form @submit.prevent="handleAdd">
          <label>
            名称:
            <input v-model="newChannel.name" required />
          </label>
          <label>
            类型:
            <select v-model="newChannel.type" required>
              <option value="dingtalk">钉钉</option>
            </select>
          </label>
          <label>
            Webhook URL:
            <input v-model="webhookUrl" required />
          </label>
          <label>
            Secret:
            <input v-model="secret" required />
          </label>
          <div class="dialog-actions">
            <button type="submit" class="btn-primary">添加</button>
            <button type="button" @click="showAddDialog = false" class="btn-secondary">取消</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listNotifyChannels, createNotifyChannel, deleteNotifyChannel, type NotifyChannel } from '@/api/notify'

const channels = ref<NotifyChannel[]>([])
const showAddDialog = ref(false)
const newChannel = ref({ name: '', type: 'dingtalk', enabled: true })
const webhookUrl = ref('')
const secret = ref('')

async function loadChannels() {
  channels.value = await listNotifyChannels()
}

async function handleAdd() {
  const config = JSON.stringify({ webhook_url: webhookUrl.value, secret: secret.value })
  await createNotifyChannel({ ...newChannel.value, config })
  showAddDialog.value = false
  newChannel.value = { name: '', type: 'dingtalk', enabled: true }
  webhookUrl.value = ''
  secret.value = ''
  loadChannels()
}

async function handleDelete(id: number) {
  if (!confirm('确认删除?')) return
  await deleteNotifyChannel(id)
  loadChannels()
}

onMounted(() => {
  loadChannels()
})
</script>

<style scoped>
.settings-view {
  padding: 16px;
}

.channel-list {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.channel-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
}

.channel-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.channel-name {
  font-weight: 500;
}

.channel-type {
  font-size: 12px;
  color: #666;
}

.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}

.dialog {
  background: white;
  padding: 24px;
  border-radius: 8px;
  min-width: 400px;
}

.dialog form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.dialog label {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dialog input,
.dialog select {
  padding: 8px;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
}

.dialog-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.btn-primary {
  padding: 8px 16px;
  background: #1976d2;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-primary:hover {
  background: #1565c0;
}

.btn-secondary {
  padding: 8px 16px;
  background: #e0e0e0;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-danger {
  padding: 8px 16px;
  background: #d32f2f;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-danger:hover {
  background: #c62828;
}
</style>
```

- [ ] **Step 3: Register route**

Add to `web/src/router/index.ts`:

```typescript
{
  path: '/settings',
  name: 'settings',
  component: () => import('@/views/SettingsView.vue'),
}
```

- [ ] **Step 4: Build and verify**

Run: `cd web && npm run build`
Expected: clean build

- [ ] **Step 5: Test in browser**

Navigate to: `http://localhost:8002/settings`
Expected: Settings page renders with channel list

- [ ] **Step 6: Commit settings page**

```bash
git add web/src/views/SettingsView.vue web/src/api/notify.ts web/src/router/index.ts
git commit -m "feat(task): add notification channel settings page"
```

---

### Task 13: NotifyChannel API Handlers

**Files:**
- Create: `internal/api/notify_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write notify handler**

Create `internal/api/notify_handler.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type NotifyHandler struct {
	notifyChannelStore *store.NotifyChannelStore
}

func NewNotifyHandler(notifyChannelStore *store.NotifyChannelStore) *NotifyHandler {
	return &NotifyHandler{notifyChannelStore: notifyChannelStore}
}

func (h *NotifyHandler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.notifyChannelStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (h *NotifyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var ch models.NotifyChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	created, err := h.notifyChannelStore.Create(&ch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(created)
}

func (h *NotifyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid channel ID", http.StatusBadRequest)
		return
	}

	if err := h.notifyChannelStore.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Add Delete method to NotifyChannelStore**

Add to `internal/store/notify_channel_store.go`:

```go
func (s *NotifyChannelStore) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM notify_channels WHERE id = ?", id)
	return err
}
```

- [ ] **Step 3: Register routes**

Add to `internal/api/router.go`:

```go
notifyHandler := NewNotifyHandler(app.NotifyChannelStore)
mux.HandleFunc("GET /api/notify-channels", notifyHandler.List)
mux.HandleFunc("POST /api/notify-channels", notifyHandler.Create)
mux.HandleFunc("DELETE /api/notify-channels/{id}", notifyHandler.Delete)
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/spider`
Expected: clean build

- [ ] **Step 5: Test API**

Run: `curl http://localhost:8002/api/notify-channels`
Expected: JSON array response

- [ ] **Step 6: Commit notify handlers**

```bash
git add internal/api/notify_handler.go internal/store/notify_channel_store.go internal/api/router.go
git commit -m "feat(task): add notification channel API handlers"
```

---

## Plan Complete

All tasks defined. Implementation ready.

**Next steps:**
1. Execute tasks in order
2. Run tests after each task
3. Verify build passes
4. Commit frequently

**Execution approach:**
- Use superpowers:subagent-driven-development (recommended) for fresh subagent per task with review between tasks
- Or use superpowers:executing-plans for inline execution with checkpoints



