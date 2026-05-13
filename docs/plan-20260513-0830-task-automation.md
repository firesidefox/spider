# Task Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a persistent, schedulable, cross-conversation task automation system where users create tasks via conversation, a headless Agent executes them on schedule, and results are summarized with optional anomaly alerting.

**Architecture:** New DB tables (tasks, task_runs, task_alerts, notify_channels) + stores + scheduler goroutine in main.go + headless Agent execution + REST API + Vue frontend TaskView. The `CreateTask` agent tool lets the LLM save a confirmed task. Post-execution LLM call generates summary and optionally creates alerts.

**Tech Stack:** Go (SQLite/database/sql, goroutine scheduler), Vue 3 (Composition API, same left-list + right-detail pattern as HostsView), existing crypto.Manager for sensitive field encryption.

---

## File Map

**New files:**
- `internal/models/task.go` — Task, TaskRun, TaskAlert, NotifyChannel structs
- `internal/store/task_store.go` — TaskStore (CRUD for tasks, task_runs, task_alerts)
- `internal/store/notify_channel_store.go` — NotifyChannelStore (CRUD + encrypt/decrypt)
- `internal/store/task_store_test.go` — TaskStore tests
- `internal/store/notify_channel_store_test.go` — NotifyChannelStore tests
- `internal/agent/tools_create_task.go` — CreateTask agent tool
- `internal/agent/tools_create_task_test.go` — CreateTask tool tests
- `internal/scheduler/scheduler.go` — DB-polling cron scheduler
- `internal/api/tasks.go` — task/run/alert/notify-channel HTTP handlers
- `web/src/api/tasks.ts` — TypeScript API client
- `web/src/views/TaskView.vue` — Task management page

**Modified files:**
- `internal/db/schema.go` — add 4 new tables in migrate()
- `internal/mcp/server.go` — add TaskStore, NotifyChannelStore fields to App; wire in NewAgentFactory
- `internal/agent/factory.go` — add TaskStore field; register CreateTask tool in buildRegistry
- `internal/api/handler.go` — add /api/v1/tasks, /api/v1/alerts, /api/v1/notify-channels routes
- `cmd/spider/main.go` — init stores, start scheduler goroutine
- `web/src/main.ts` — add /tasks route
- `web/src/App.vue` — add 任务 nav link

---

### Task 1: DB Schema — 4 new tables

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add tables to migrate()**

At the end of `migrate()` in `internal/db/schema.go`, before the final `return nil`, add:

```go
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    goal            TEXT    NOT NULL,
    host_ids        TEXT    NOT NULL DEFAULT '[]',
    schedule        TEXT    NOT NULL DEFAULT '',
    alert_on_anomaly INTEGER NOT NULL DEFAULT 0,
    status          TEXT    NOT NULL DEFAULT 'active',
    source_conv_id  TEXT    NOT NULL DEFAULT '',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
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
    summary     TEXT    NOT NULL DEFAULT '',
    alerted     INTEGER NOT NULL DEFAULT 0
)`); err != nil {
    return err
}
if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_runs_task_id ON task_runs(task_id)`); err != nil {
    return err
}
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS task_alerts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id     INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    task_run_id INTEGER NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    summary     TEXT    NOT NULL DEFAULT '',
    status      TEXT    NOT NULL DEFAULT 'open',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME
)`); err != nil {
    return err
}
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS notify_channels (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    type                TEXT    NOT NULL,
    name                TEXT    NOT NULL,
    encrypted_config    TEXT    NOT NULL DEFAULT '',
    enabled             INTEGER NOT NULL DEFAULT 1,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
    return err
}
```

- [ ] **Step 2: Build and verify migration runs**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./internal/db/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add tasks, task_runs, task_alerts, notify_channels tables"
```

---

### Task 2: Models

**Files:**
- Create: `internal/models/task.go`

- [ ] **Step 1: Write models**

Create `internal/models/task.go`:

```go
package models

import "time"

type Task struct {
    ID             int64     `json:"id"`
    Name           string    `json:"name"`
    Goal           string    `json:"goal"`
    HostIDs        []int64   `json:"host_ids"`
    Schedule       string    `json:"schedule"`
    AlertOnAnomaly bool      `json:"alert_on_anomaly"`
    Status         string    `json:"status"`
    SourceConvID   string    `json:"source_conv_id"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
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

type TaskAlert struct {
    ID         int64      `json:"id"`
    TaskID     int64      `json:"task_id"`
    TaskRunID  int64      `json:"task_run_id"`
    Summary    string     `json:"summary"`
    Status     string     `json:"status"`
    CreatedAt  time.Time  `json:"created_at"`
    ResolvedAt *time.Time `json:"resolved_at"`
}

type NotifyChannel struct {
    ID        int64     `json:"id"`
    Type      string    `json:"type"`
    Name      string    `json:"name"`
    Config    string    `json:"config"`
    Enabled   bool      `json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/models/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/task.go
git commit -m "feat(models): add Task, TaskRun, TaskAlert, NotifyChannel"
```

---

### Task 3: TaskStore

**Files:**
- Create: `internal/store/task_store.go`
- Create: `internal/store/task_store_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/store/task_store_test.go`:

```go
package store_test

import (
    "testing"
    "time"

    "github.com/spiderai/spider/internal/db"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

func newTestTaskStore(t *testing.T) *store.TaskStore {
    t.Helper()
    database, err := db.Open(t.TempDir())
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { database.Close() })
    return store.NewTaskStore(database)
}

func TestTaskStore_CreateAndGet(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{
        Name:           "weekly check",
        Goal:           "check disk usage",
        HostIDs:        []int64{1, 2},
        Schedule:       "0 2 * * 3",
        AlertOnAnomaly: true,
        Status:         "active",
        SourceConvID:   "conv-abc",
    }
    if err := s.Create(task); err != nil {
        t.Fatal(err)
    }
    if task.ID == 0 {
        t.Fatal("expected non-zero ID")
    }
    got, err := s.Get(task.ID)
    if err != nil {
        t.Fatal(err)
    }
    if got.Name != "weekly check" {
        t.Errorf("got name %q", got.Name)
    }
    if len(got.HostIDs) != 2 {
        t.Errorf("got host_ids %v", got.HostIDs)
    }
    if !got.AlertOnAnomaly {
        t.Error("expected alert_on_anomaly true")
    }
}

func TestTaskStore_List(t *testing.T) {
    s := newTestTaskStore(t)
    for i := 0; i < 3; i++ {
        s.Create(&models.Task{Name: "t", Goal: "g", Status: "active"})
    }
    tasks, err := s.List()
    if err != nil {
        t.Fatal(err)
    }
    if len(tasks) != 3 {
        t.Errorf("expected 3, got %d", len(tasks))
    }
}

func TestTaskStore_Update(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "old", Goal: "g", Status: "active"}
    s.Create(task)
    task.Name = "new"
    task.Status = "paused"
    if err := s.Update(task); err != nil {
        t.Fatal(err)
    }
    got, _ := s.Get(task.ID)
    if got.Name != "new" || got.Status != "paused" {
        t.Errorf("update failed: %+v", got)
    }
}

func TestTaskStore_CreateRun(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "t", Goal: "g", Status: "active"}
    s.Create(task)
    run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "running"}
    if err := s.CreateRun(run); err != nil {
        t.Fatal(err)
    }
    if run.ID == 0 {
        t.Fatal("expected non-zero run ID")
    }
}

func TestTaskStore_ListRuns(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "t", Goal: "g", Status: "active"}
    s.Create(task)
    now := time.Now().UTC()
    s.CreateRun(&models.TaskRun{TaskID: task.ID, StartedAt: now, Status: "success"})
    s.CreateRun(&models.TaskRun{TaskID: task.ID, StartedAt: now, Status: "failed"})
    runs, err := s.ListRuns(task.ID)
    if err != nil {
        t.Fatal(err)
    }
    if len(runs) != 2 {
        t.Errorf("expected 2 runs, got %d", len(runs))
    }
}

func TestTaskStore_UpdateRun(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "t", Goal: "g", Status: "active"}
    s.Create(task)
    run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "running"}
    s.CreateRun(run)
    fin := time.Now().UTC()
    run.FinishedAt = &fin
    run.Status = "success"
    run.RawOutput = "output"
    run.Summary = "all good"
    if err := s.UpdateRun(run); err != nil {
        t.Fatal(err)
    }
    got, _ := s.GetRun(run.ID)
    if got.Status != "success" || got.Summary != "all good" {
        t.Errorf("update run failed: %+v", got)
    }
}

func TestTaskStore_CreateAlert(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "t", Goal: "g", Status: "active"}
    s.Create(task)
    run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "success"}
    s.CreateRun(run)
    alert := &models.TaskAlert{TaskID: task.ID, TaskRunID: run.ID, Summary: "disk 95%", Status: "open"}
    if err := s.CreateAlert(alert); err != nil {
        t.Fatal(err)
    }
    if alert.ID == 0 {
        t.Fatal("expected non-zero alert ID")
    }
}

func TestTaskStore_ListAlerts(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "t", Goal: "g", Status: "active"}
    s.Create(task)
    run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "success"}
    s.CreateRun(run)
    s.CreateAlert(&models.TaskAlert{TaskID: task.ID, TaskRunID: run.ID, Summary: "a", Status: "open"})
    s.CreateAlert(&models.TaskAlert{TaskID: task.ID, TaskRunID: run.ID, Summary: "b", Status: "open"})
    alerts, err := s.ListAlerts()
    if err != nil {
        t.Fatal(err)
    }
    if len(alerts) != 2 {
        t.Errorf("expected 2 alerts, got %d", len(alerts))
    }
}

func TestTaskStore_ResolveAlert(t *testing.T) {
    s := newTestTaskStore(t)
    task := &models.Task{Name: "t", Goal: "g", Status: "active"}
    s.Create(task)
    run := &models.TaskRun{TaskID: task.ID, StartedAt: time.Now().UTC(), Status: "success"}
    s.CreateRun(run)
    alert := &models.TaskAlert{TaskID: task.ID, TaskRunID: run.ID, Summary: "x", Status: "open"}
    s.CreateAlert(alert)
    if err := s.ResolveAlert(alert.ID); err != nil {
        t.Fatal(err)
    }
    alerts, _ := s.ListAlerts()
    for _, a := range alerts {
        if a.ID == alert.ID && a.Status != "resolved" {
            t.Error("expected resolved")
        }
    }
}

func TestTaskStore_DueActiveTasks(t *testing.T) {
    s := newTestTaskStore(t)
    // manual task (empty schedule) should not appear in due tasks
    s.Create(&models.Task{Name: "manual", Goal: "g", Status: "active", Schedule: ""})
    tasks, err := s.DueActiveTasks(time.Now())
    if err != nil {
        t.Fatal(err)
    }
    // no cron tasks, so result should be empty
    if len(tasks) != 0 {
        t.Errorf("expected 0 due tasks, got %d", len(tasks))
    }
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/store/... -run TestTaskStore 2>&1 | head -20
```
Expected: compile error "undefined: store.TaskStore"

- [ ] **Step 3: Implement TaskStore**

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

func (s *TaskStore) Create(t *models.Task) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    hostIDs, _ := json.Marshal(t.HostIDs)
    now := time.Now().UTC()
    res, err := s.db.Exec(
        `INSERT INTO tasks (name, goal, host_ids, schedule, alert_on_anomaly, status, source_conv_id, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        t.Name, t.Goal, string(hostIDs), t.Schedule, boolToInt(t.AlertOnAnomaly),
        t.Status, t.SourceConvID, now, now,
    )
    if err != nil {
        return err
    }
    id, _ := res.LastInsertId()
    t.ID = id
    t.CreatedAt = now
    t.UpdatedAt = now
    return nil
}

func (s *TaskStore) Get(id int64) (*models.Task, error) {
    return s.scanTask(s.db.QueryRow(
        `SELECT id, name, goal, host_ids, schedule, alert_on_anomaly, status, source_conv_id, created_at, updated_at
         FROM tasks WHERE id = ?`, id,
    ))
}

func (s *TaskStore) List() ([]*models.Task, error) {
    rows, err := s.db.Query(
        `SELECT id, name, goal, host_ids, schedule, alert_on_anomaly, status, source_conv_id, created_at, updated_at
         FROM tasks WHERE status != 'archived' ORDER BY id DESC`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var tasks []*models.Task
    for rows.Next() {
        t, err := s.scanTask(rows)
        if err != nil {
            return nil, err
        }
        tasks = append(tasks, t)
    }
    return tasks, rows.Err()
}

func (s *TaskStore) Update(t *models.Task) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    hostIDs, _ := json.Marshal(t.HostIDs)
    now := time.Now().UTC()
    _, err := s.db.Exec(
        `UPDATE tasks SET name=?, goal=?, host_ids=?, schedule=?, alert_on_anomaly=?, status=?, updated_at=? WHERE id=?`,
        t.Name, t.Goal, string(hostIDs), t.Schedule, boolToInt(t.AlertOnAnomaly), t.Status, now, t.ID,
    )
    t.UpdatedAt = now
    return err
}

func (s *TaskStore) Delete(id int64) error {
    _, err := s.db.Exec(`UPDATE tasks SET status='archived' WHERE id=?`, id)
    return err
}

// DueActiveTasks returns active cron tasks whose next fire time <= now.
// Simple approach: return all active tasks with non-empty schedule; scheduler
// checks cron expression itself.
func (s *TaskStore) DueActiveTasks(now time.Time) ([]*models.Task, error) {
    rows, err := s.db.Query(
        `SELECT id, name, goal, host_ids, schedule, alert_on_anomaly, status, source_conv_id, created_at, updated_at
         FROM tasks WHERE status = 'active' AND schedule != ''`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var tasks []*models.Task
    for rows.Next() {
        t, err := s.scanTask(rows)
        if err != nil {
            return nil, err
        }
        tasks = append(tasks, t)
    }
    return tasks, rows.Err()
}
```

- [ ] **Step 4: Add run/alert methods (append to task_store.go)**

```go
func (s *TaskStore) CreateRun(r *models.TaskRun) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    res, err := s.db.Exec(
        `INSERT INTO task_runs (task_id, started_at, status) VALUES (?, ?, ?)`,
        r.TaskID, r.StartedAt, r.Status,
    )
    if err != nil {
        return err
    }
    id, _ := res.LastInsertId()
    r.ID = id
    return nil
}

func (s *TaskStore) GetRun(id int64) (*models.TaskRun, error) {
    return s.scanRun(s.db.QueryRow(
        `SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
         FROM task_runs WHERE id = ?`, id,
    ))
}

func (s *TaskStore) ListRuns(taskID int64) ([]*models.TaskRun, error) {
    rows, err := s.db.Query(
        `SELECT id, task_id, started_at, finished_at, status, raw_output, summary, alerted
         FROM task_runs WHERE task_id = ? ORDER BY id DESC`, taskID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var runs []*models.TaskRun
    for rows.Next() {
        r, err := s.scanRun(rows)
        if err != nil {
            return nil, err
        }
        runs = append(runs, r)
    }
    return runs, rows.Err()
}

func (s *TaskStore) UpdateRun(r *models.TaskRun) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    _, err := s.db.Exec(
        `UPDATE task_runs SET finished_at=?, status=?, raw_output=?, summary=?, alerted=? WHERE id=?`,
        r.FinishedAt, r.Status, r.RawOutput, r.Summary, boolToInt(r.Alerted), r.ID,
    )
    return err
}

func (s *TaskStore) CreateAlert(a *models.TaskAlert) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC()
    res, err := s.db.Exec(
        `INSERT INTO task_alerts (task_id, task_run_id, summary, status, created_at) VALUES (?, ?, ?, ?, ?)`,
        a.TaskID, a.TaskRunID, a.Summary, a.Status, now,
    )
    if err != nil {
        return err
    }
    id, _ := res.LastInsertId()
    a.ID = id
    a.CreatedAt = now
    return nil
}

func (s *TaskStore) ListAlerts() ([]*models.TaskAlert, error) {
    rows, err := s.db.Query(
        `SELECT id, task_id, task_run_id, summary, status, created_at, resolved_at
         FROM task_alerts ORDER BY id DESC`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var alerts []*models.TaskAlert
    for rows.Next() {
        var a models.TaskAlert
        var resolvedAt sql.NullTime
        if err := rows.Scan(&a.ID, &a.TaskID, &a.TaskRunID, &a.Summary, &a.Status, &a.CreatedAt, &resolvedAt); err != nil {
            return nil, err
        }
        if resolvedAt.Valid {
            a.ResolvedAt = &resolvedAt.Time
        }
        alerts = append(alerts, &a)
    }
    return alerts, rows.Err()
}

func (s *TaskStore) ResolveAlert(id int64) error {
    now := time.Now().UTC()
    _, err := s.db.Exec(`UPDATE task_alerts SET status='resolved', resolved_at=? WHERE id=?`, now, id)
    return err
}

// scanTask scans a row into *models.Task. Works with both *sql.Row and *sql.Rows.
type taskScanner interface {
    Scan(dest ...any) error
}

func (s *TaskStore) scanTask(row taskScanner) (*models.Task, error) {
    var t models.Task
    var hostIDsJSON string
    var alertInt int
    err := row.Scan(&t.ID, &t.Name, &t.Goal, &hostIDsJSON, &t.Schedule,
        &alertInt, &t.Status, &t.SourceConvID, &t.CreatedAt, &t.UpdatedAt)
    if err != nil {
        return nil, err
    }
    json.Unmarshal([]byte(hostIDsJSON), &t.HostIDs) //nolint:errcheck
    t.AlertOnAnomaly = alertInt != 0
    return &t, nil
}

func (s *TaskStore) scanRun(row taskScanner) (*models.TaskRun, error) {
    var r models.TaskRun
    var finishedAt sql.NullTime
    var alertedInt int
    err := row.Scan(&r.ID, &r.TaskID, &r.StartedAt, &finishedAt, &r.Status, &r.RawOutput, &r.Summary, &alertedInt)
    if err != nil {
        return nil, err
    }
    if finishedAt.Valid {
        r.FinishedAt = &finishedAt.Time
    }
    r.Alerted = alertedInt != 0
    return &r, nil
}

func boolToInt(b bool) int {
    if b {
        return 1
    }
    return 0
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/store/... -run TestTaskStore -v 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/task_store.go internal/store/task_store_test.go
git commit -m "feat(store): TaskStore with tasks, runs, alerts CRUD"
```

---

### Task 4: NotifyChannelStore

**Files:**
- Create: `internal/store/notify_channel_store.go`
- Create: `internal/store/notify_channel_store_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/store/notify_channel_store_test.go`:

```go
package store_test

import (
    "testing"

    "github.com/spiderai/spider/internal/crypto"
    "github.com/spiderai/spider/internal/db"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

func newTestNotifyChannelStore(t *testing.T) *store.NotifyChannelStore {
    t.Helper()
    dir := t.TempDir()
    database, err := db.Open(dir)
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { database.Close() })
    cm, err := crypto.NewManager(dir)
    if err != nil {
        t.Fatal(err)
    }
    return store.NewNotifyChannelStore(database, cm)
}

func TestNotifyChannelStore_CreateAndGet(t *testing.T) {
    s := newTestNotifyChannelStore(t)
    ch := &models.NotifyChannel{
        Type:    "dingtalk",
        Name:    "ops bot",
        Config:  `{"webhook_url":"https://oapi.dingtalk.com/robot/send?access_token=xxx","secret":"mysecret"}`,
        Enabled: true,
    }
    if err := s.Create(ch); err != nil {
        t.Fatal(err)
    }
    if ch.ID == 0 {
        t.Fatal("expected non-zero ID")
    }
    got, err := s.Get(ch.ID)
    if err != nil {
        t.Fatal(err)
    }
    if got.Name != "ops bot" {
        t.Errorf("got name %q", got.Name)
    }
    // Config should be decrypted back to original
    if got.Config != ch.Config {
        t.Errorf("config mismatch: got %q", got.Config)
    }
}

func TestNotifyChannelStore_List(t *testing.T) {
    s := newTestNotifyChannelStore(t)
    s.Create(&models.NotifyChannel{Type: "email", Name: "a", Config: "{}", Enabled: true})
    s.Create(&models.NotifyChannel{Type: "webhook", Name: "b", Config: "{}", Enabled: false})
    channels, err := s.List()
    if err != nil {
        t.Fatal(err)
    }
    if len(channels) != 2 {
        t.Errorf("expected 2, got %d", len(channels))
    }
}

func TestNotifyChannelStore_Update(t *testing.T) {
    s := newTestNotifyChannelStore(t)
    ch := &models.NotifyChannel{Type: "email", Name: "old", Config: "{}", Enabled: true}
    s.Create(ch)
    ch.Name = "new"
    ch.Enabled = false
    if err := s.Update(ch); err != nil {
        t.Fatal(err)
    }
    got, _ := s.Get(ch.ID)
    if got.Name != "new" || got.Enabled {
        t.Errorf("update failed: %+v", got)
    }
}

func TestNotifyChannelStore_Delete(t *testing.T) {
    s := newTestNotifyChannelStore(t)
    ch := &models.NotifyChannel{Type: "webhook", Name: "x", Config: "{}", Enabled: true}
    s.Create(ch)
    if err := s.Delete(ch.ID); err != nil {
        t.Fatal(err)
    }
    channels, _ := s.List()
    if len(channels) != 0 {
        t.Errorf("expected 0 after delete, got %d", len(channels))
    }
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/store/... -run TestNotifyChannelStore 2>&1 | head -10
```
Expected: compile error "undefined: store.NotifyChannelStore"

- [ ] **Step 3: Implement NotifyChannelStore**

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

func (s *NotifyChannelStore) Create(ch *models.NotifyChannel) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    enc, err := s.cm.Encrypt(ch.Config)
    if err != nil {
        return err
    }
    now := time.Now().UTC()
    res, err := s.db.Exec(
        `INSERT INTO notify_channels (type, name, encrypted_config, enabled, created_at) VALUES (?, ?, ?, ?, ?)`,
        ch.Type, ch.Name, enc, boolToInt(ch.Enabled), now,
    )
    if err != nil {
        return err
    }
    id, _ := res.LastInsertId()
    ch.ID = id
    ch.CreatedAt = now
    return nil
}

func (s *NotifyChannelStore) Get(id int64) (*models.NotifyChannel, error) {
    var ch models.NotifyChannel
    var enc string
    var enabledInt int
    err := s.db.QueryRow(
        `SELECT id, type, name, encrypted_config, enabled, created_at FROM notify_channels WHERE id = ?`, id,
    ).Scan(&ch.ID, &ch.Type, &ch.Name, &enc, &enabledInt, &ch.CreatedAt)
    if err != nil {
        return nil, err
    }
    ch.Enabled = enabledInt != 0
    ch.Config, err = s.cm.Decrypt(enc)
    return &ch, err
}

func (s *NotifyChannelStore) List() ([]*models.NotifyChannel, error) {
    rows, err := s.db.Query(
        `SELECT id, type, name, encrypted_config, enabled, created_at FROM notify_channels ORDER BY id ASC`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var channels []*models.NotifyChannel
    for rows.Next() {
        var ch models.NotifyChannel
        var enc string
        var enabledInt int
        if err := rows.Scan(&ch.ID, &ch.Type, &ch.Name, &enc, &enabledInt, &ch.CreatedAt); err != nil {
            return nil, err
        }
        ch.Enabled = enabledInt != 0
        ch.Config, _ = s.cm.Decrypt(enc)
        channels = append(channels, &ch)
    }
    return channels, rows.Err()
}

func (s *NotifyChannelStore) Update(ch *models.NotifyChannel) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    enc, err := s.cm.Encrypt(ch.Config)
    if err != nil {
        return err
    }
    _, err = s.db.Exec(
        `UPDATE notify_channels SET type=?, name=?, encrypted_config=?, enabled=? WHERE id=?`,
        ch.Type, ch.Name, enc, boolToInt(ch.Enabled), ch.ID,
    )
    return err
}

func (s *NotifyChannelStore) Delete(id int64) error {
    _, err := s.db.Exec(`DELETE FROM notify_channels WHERE id=?`, id)
    return err
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/store/... -run TestNotifyChannelStore -v 2>&1 | tail -15
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/notify_channel_store.go internal/store/notify_channel_store_test.go
git commit -m "feat(store): NotifyChannelStore with encrypted config"
```

---

### Task 5: CreateTask Agent Tool

**Files:**
- Create: `internal/agent/tools_create_task.go`
- Create: `internal/agent/tools_create_task_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/agent/tools_create_task_test.go`:

```go
package agent_test

import (
    "context"
    "testing"

    "github.com/spiderai/spider/internal/agent"
    "github.com/spiderai/spider/internal/db"
    "github.com/spiderai/spider/internal/store"
)

func newTestCreateTaskTool(t *testing.T) *agent.CreateTaskTool {
    t.Helper()
    database, err := db.Open(t.TempDir())
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { database.Close() })
    return agent.NewCreateTaskTool(store.NewTaskStore(database))
}

func TestCreateTaskTool_Name(t *testing.T) {
    tool := newTestCreateTaskTool(t)
    if tool.Name() != "CreateTask" {
        t.Errorf("got %q", tool.Name())
    }
}

func TestCreateTaskTool_Execute_Basic(t *testing.T) {
    tool := newTestCreateTaskTool(t)
    result, err := tool.Execute(context.Background(), map[string]any{
        "name":             "weekly disk check",
        "goal":             "check disk usage on all hosts",
        "host_ids":         []any{float64(1), float64(2)},
        "schedule":         "0 2 * * 3",
        "alert_on_anomaly": true,
    })
    if err != nil {
        t.Fatal(err)
    }
    if result.IsError {
        t.Errorf("unexpected error: %s", result.Content)
    }
}

func TestCreateTaskTool_Execute_ManualOnly(t *testing.T) {
    tool := newTestCreateTaskTool(t)
    result, err := tool.Execute(context.Background(), map[string]any{
        "name":  "one-time check",
        "goal":  "verify config",
        "host_ids": []any{float64(1)},
    })
    if err != nil {
        t.Fatal(err)
    }
    if result.IsError {
        t.Errorf("unexpected error: %s", result.Content)
    }
}

func TestCreateTaskTool_Execute_MissingRequired(t *testing.T) {
    tool := newTestCreateTaskTool(t)
    result, err := tool.Execute(context.Background(), map[string]any{
        "name": "no goal",
    })
    if err != nil {
        t.Fatal(err)
    }
    if !result.IsError {
        t.Error("expected error for missing goal")
    }
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/agent/... -run TestCreateTaskTool 2>&1 | head -10
```
Expected: compile error "undefined: agent.CreateTaskTool"

- [ ] **Step 3: Implement CreateTask tool**

Create `internal/agent/tools_create_task.go`:

```go
package agent

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

type CreateTaskTool struct {
    store *store.TaskStore
}

func NewCreateTaskTool(s *store.TaskStore) *CreateTaskTool {
    return &CreateTaskTool{store: s}
}

func (t *CreateTaskTool) Name() string { return "CreateTask" }

func (t *CreateTaskTool) Description() string {
    return "Save a confirmed automated task. Has side effects. Call only after user has confirmed all fields."
}

func (t *CreateTaskTool) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "name":             map[string]any{"type": "string", "description": "任务名称"},
            "goal":             map[string]any{"type": "string", "description": "自然语言目标"},
            "host_ids":         map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "目标设备 ID 列表"},
            "schedule":         map[string]any{"type": "string", "description": "cron 表达式，空 = manual only"},
            "alert_on_anomaly": map[string]any{"type": "boolean", "description": "执行后是否判断异常并告警"},
            "source_conv_id":   map[string]any{"type": "string", "description": "创建来源对话 ID"},
        },
        "required": []string{"name", "goal", "host_ids"},
    }
}

func (t *CreateTaskTool) DefaultRiskLevel() RiskLevel { return RiskL2 }

func (t *CreateTaskTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
    name, _ := input["name"].(string)
    goal, _ := input["goal"].(string)
    if name == "" || goal == "" {
        return &ToolResult{Content: "name and goal are required", IsError: true}, nil
    }

    var hostIDs []int64
    if raw, ok := input["host_ids"]; ok {
        switch v := raw.(type) {
        case []any:
            for _, item := range v {
                if f, ok := item.(float64); ok {
                    hostIDs = append(hostIDs, int64(f))
                }
            }
        }
    }

    schedule, _ := input["schedule"].(string)
    alertOnAnomaly, _ := input["alert_on_anomaly"].(bool)
    sourceConvID, _ := input["source_conv_id"].(string)

    task := &models.Task{
        Name:           name,
        Goal:           goal,
        HostIDs:        hostIDs,
        Schedule:       schedule,
        AlertOnAnomaly: alertOnAnomaly,
        Status:         "active",
        SourceConvID:   sourceConvID,
    }
    if err := t.store.Create(task); err != nil {
        return &ToolResult{Content: fmt.Sprintf("failed to create task: %v", err), IsError: true}, nil
    }

    out, _ := json.Marshal(map[string]any{
        "id":      task.ID,
        "name":    task.Name,
        "status":  task.Status,
        "message": "任务已创建",
    })
    return &ToolResult{Content: string(out)}, nil
}

func (t *CreateTaskTool) SystemPromptSection() string {
    return createTaskPrompt
}

const createTaskPrompt = `## CreateTask Tool

**When to use:** Only after the user has explicitly confirmed all task fields (name, goal, devices, schedule, alert preference). Never call speculatively.

**When NOT to use:**
- User is still describing what they want
- Any field is unclear or unconfirmed
- User has not seen and approved the extracted fields

**Workflow:**
1. Extract from conversation: name, goal, host_ids, schedule (cron or empty), alert_on_anomaly
2. Present extracted fields to user in a summary
3. Wait for explicit confirmation ("确认" / "好的" / "是的")
4. Only then call CreateTask

**Rules:**
- schedule: standard 5-field cron (e.g. "0 2 * * 3" = Wednesday 2am). Empty string = manual only.
- alert_on_anomaly: default false. Set true only if user mentions monitoring, alerting, or anomaly detection.
- host_ids: must be numeric IDs from ListDevices. Never pass names.`
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/agent/... -run TestCreateTaskTool -v 2>&1 | tail -15
```
Expected: all PASS.

- [ ] **Step 5: Wire into factory**

In `internal/agent/factory.go`, add `TaskStore *store.TaskStore` field to `Factory` struct (after `TopologyStore`):

```go
TaskStore  *store.TaskStore
```

In `buildRegistry()`, after the TopologyStore block:

```go
if f.TaskStore != nil {
    registry.Register(NewCreateTaskTool(f.TaskStore))
}
```

- [ ] **Step 6: Build**

```bash
go build ./internal/agent/...
```
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/agent/tools_create_task.go internal/agent/tools_create_task_test.go internal/agent/factory.go
git commit -m "feat(agent): CreateTask tool + wire into factory"
```

---

### Task 6: Scheduler

**Files:**
- Create: `internal/scheduler/scheduler.go`

The scheduler polls the DB every minute, checks which active cron tasks are due (using a simple cron parser), and runs them as headless Agents.

- [ ] **Step 1: Add robfig/cron dependency**

```bash
cd /Users/cw/fty.ai/spider.ai
go get github.com/robfig/cron/v3@v3.0.1
```
Expected: go.mod and go.sum updated.

- [ ] **Step 2: Implement scheduler**

Create `internal/scheduler/scheduler.go`:

```go
package scheduler

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/robfig/cron/v3"
    "github.com/spiderai/spider/internal/agent"
    "github.com/spiderai/spider/internal/logger"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

// Executor runs a headless agent for a task and writes the TaskRun result.
type Executor interface {
    RunHeadless(ctx context.Context, task *models.Task) (string, error)
}

// Scheduler polls the DB every minute and fires due tasks.
type Scheduler struct {
    taskStore *store.TaskStore
    executor  Executor
    ticker    *time.Ticker
    done      chan struct{}
    // track last-fired minute per task to avoid double-firing
    lastFired map[int64]time.Time
}

func New(taskStore *store.TaskStore, executor Executor) *Scheduler {
    return &Scheduler{
        taskStore: taskStore,
        executor:  executor,
        done:      make(chan struct{}),
        lastFired: make(map[int64]time.Time),
    }
}

func (s *Scheduler) Start(ctx context.Context) {
    s.ticker = time.NewTicker(time.Minute)
    go func() {
        for {
            select {
            case t := <-s.ticker.C:
                s.tick(ctx, t)
            case <-ctx.Done():
                s.ticker.Stop()
                return
            case <-s.done:
                s.ticker.Stop()
                return
            }
        }
    }()
}

func (s *Scheduler) Stop() { close(s.done) }

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
    tasks, err := s.taskStore.DueActiveTasks(now)
    if err != nil {
        logger.Global().Error().Err(err).Msg("scheduler: list tasks")
        return
    }
    for _, task := range tasks {
        if !s.isDue(task, now) {
            continue
        }
        s.lastFired[task.ID] = now.Truncate(time.Minute)
        go s.runTask(ctx, task)
    }
}

func (s *Scheduler) isDue(task *models.Task, now time.Time) bool {
    if task.Schedule == "" {
        return false
    }
    // Prevent double-firing within the same minute
    minute := now.Truncate(time.Minute)
    if last, ok := s.lastFired[task.ID]; ok && last.Equal(minute) {
        return false
    }
    sched, err := cron.ParseStandard(task.Schedule)
    if err != nil {
        logger.Global().Warn().Err(err).Int64("task_id", task.ID).Msg("scheduler: invalid cron")
        return false
    }
    // Check if the cron would have fired in the last minute
    prev := sched.Next(now.Add(-time.Minute))
    return !prev.After(now)
}

func (s *Scheduler) runTask(ctx context.Context, task *models.Task) {
    log := logger.Global().With().Int64("task_id", task.ID).Str("task_name", task.Name).Logger()
    log.Info().Msg("scheduler: starting task run")

    run := &models.TaskRun{
        TaskID:    task.ID,
        StartedAt: time.Now().UTC(),
        Status:    "running",
    }
    if err := s.taskStore.CreateRun(run); err != nil {
        log.Error().Err(err).Msg("scheduler: create run")
        return
    }

    rawOutput, execErr := s.executor.RunHeadless(ctx, task)
    fin := time.Now().UTC()
    run.FinishedAt = &fin
    run.RawOutput = rawOutput
    if execErr != nil {
        run.Status = "failed"
        log.Error().Err(execErr).Msg("scheduler: task run failed")
    } else {
        run.Status = "success"
    }

    if err := s.taskStore.UpdateRun(run); err != nil {
        log.Error().Err(err).Msg("scheduler: update run")
    }
    log.Info().Str("status", run.Status).Msg("scheduler: task run complete")
}

// TriggerNow runs a task immediately (manual trigger). Returns the run ID.
func (s *Scheduler) TriggerNow(ctx context.Context, task *models.Task) (int64, error) {
    run := &models.TaskRun{
        TaskID:    task.ID,
        StartedAt: time.Now().UTC(),
        Status:    "running",
    }
    if err := s.taskStore.CreateRun(run); err != nil {
        return 0, err
    }
    go s.runTask(ctx, task)
    return run.ID, nil
}
```

- [ ] **Step 3: Implement headless executor in agent package**

Add to `internal/agent/factory.go` (new method after `BuildSystemPrompt`):

```go
// RunHeadless executes a task goal using a headless Agent (no conversation history).
// Returns the concatenated text output from the agent run.
func (f *Factory) RunHeadless(ctx context.Context, task *models.Task) (string, error) {
    // Build a focused system prompt for this task
    hostSummary := ""
    if len(task.HostIDs) > 0 {
        var names []string
        for _, id := range task.HostIDs {
            h, err := f.Hosts.GetByID(fmt.Sprintf("%d", id))
            if err == nil {
                names = append(names, h.Name)
            }
        }
        if len(names) > 0 {
            hostSummary = fmt.Sprintf("\n\nTarget devices: %s", strings.Join(names, ", "))
        }
    }
    systemPrompt := fmt.Sprintf(
        "You are Spider, executing an automated task.\n\nTask: %s%s\n\nComplete the task autonomously. Be concise in output.",
        task.Goal, hostSummary,
    )

    a := f.NewAgent(systemPrompt, fmt.Sprintf("task-%d", task.ID))

    var output strings.Builder
    events := make(chan Event, 64)
    go func() {
        a.Run(ctx, task.Goal, events)
        close(events)
    }()
    for ev := range events {
        if ev.Type == EventTextDelta {
            if text, ok := ev.Content["text"].(string); ok {
                output.WriteString(text)
            }
        }
    }
    return output.String(), nil
}
```

Note: `RunHeadless` needs `"fmt"` and `"strings"` imports already present in factory.go.

- [ ] **Step 4: Check agent.Run signature**

```bash
grep -n "func.*Run(" /Users/cw/fty.ai/spider.ai/internal/agent/agent.go | head -5
```

If the signature differs from `Run(ctx context.Context, userMsg string, events chan<- Event)`, adjust the `RunHeadless` implementation to match.

- [ ] **Step 5: Build**

```bash
go build ./internal/scheduler/... ./internal/agent/...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/scheduler/scheduler.go internal/agent/factory.go go.mod go.sum
git commit -m "feat(scheduler): DB-polling cron scheduler + headless agent executor"
```

---

### Task 7: Wire stores and scheduler into main

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Add fields to App struct**

In `internal/mcp/server.go`, add to the `App` struct after `TodoStore`:

```go
TaskStore           *store.TaskStore
NotifyChannelStore  *store.NotifyChannelStore
```

In `NewAgentFactory()`, after `f.TodoStore = a.TodoStore`:

```go
f.TaskStore = a.TaskStore
```

- [ ] **Step 2: Init stores and start scheduler in main.go**

In `cmd/spider/main.go`, after `app.TodoStore = store.NewTodoStore(database)`:

```go
app.TaskStore = store.NewTaskStore(database)
app.NotifyChannelStore = store.NewNotifyChannelStore(database, cm)
```

After the `agentFactory` block (around line 225), add scheduler startup:

```go
if agentFactory != nil {
    sched := scheduler.New(app.TaskStore, agentFactory)
    sched.Start(shutdownCtx)
    defer sched.Stop()
}
```

Add import: `"github.com/spiderai/spider/internal/scheduler"`

- [ ] **Step 3: Build**

```bash
go build ./cmd/spider/...
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(main): wire TaskStore, NotifyChannelStore, start scheduler"
```

---

### Task 8: Post-execution LLM analysis (summary + alert)

**Files:**
- Modify: `internal/scheduler/scheduler.go`

After a task run completes, call the LLM to generate a summary and optionally create an alert.

- [ ] **Step 1: Add LLM summarizer to Scheduler**

The `Executor` interface already returns raw output. We need the factory's LLM client to do the post-run analysis. Update `Scheduler` to accept an `llm.Client`:

In `internal/scheduler/scheduler.go`, update the struct and constructor:

```go
import (
    "github.com/spiderai/spider/internal/llm"
    "github.com/spiderai/spider/internal/store"
)

type Scheduler struct {
    taskStore    *store.TaskStore
    notifyStore  *store.NotifyChannelStore
    executor     Executor
    llmClient    llm.Client
    llmModel     string
    ticker       *time.Ticker
    done         chan struct{}
    lastFired    map[int64]time.Time
}

func New(taskStore *store.TaskStore, notifyStore *store.NotifyChannelStore, executor Executor, llmClient llm.Client, llmModel string) *Scheduler {
    return &Scheduler{
        taskStore:   taskStore,
        notifyStore: notifyStore,
        executor:    executor,
        llmClient:   llmClient,
        llmModel:    llmModel,
        done:        make(chan struct{}),
        lastFired:   make(map[int64]time.Time),
    }
}
```

- [ ] **Step 2: Add summarize() method**

Append to `internal/scheduler/scheduler.go`:

```go
func (s *Scheduler) summarize(ctx context.Context, run *models.TaskRun, task *models.Task) {
    if s.llmClient == nil {
        return
    }
    prompt := fmt.Sprintf(
        "Task: %s\nGoal: %s\n\nExecution output:\n%s\n\nWrite a concise summary (2-3 sentences) of what happened.",
        task.Name, task.Goal, truncate(run.RawOutput, 4000),
    )
    msgs := []llm.Message{{Role: "user", Content: prompt}}
    resp, err := s.llmClient.Chat(ctx, msgs)
    if err != nil {
        logger.Global().Warn().Err(err).Int64("run_id", run.ID).Msg("scheduler: summarize failed")
        return
    }
    run.Summary = resp
    if task.AlertOnAnomaly {
        s.checkAnomaly(ctx, run, task)
    }
    s.taskStore.UpdateRun(run) //nolint:errcheck
}

func (s *Scheduler) checkAnomaly(ctx context.Context, run *models.TaskRun, task *models.Task) {
    prompt := fmt.Sprintf(
        "Task: %s\nGoal: %s\n\nExecution summary:\n%s\n\nIs there an anomaly or problem that requires attention? Reply with JSON: {\"anomaly\": true/false, \"reason\": \"...\"}",
        task.Name, task.Goal, run.Summary,
    )
    msgs := []llm.Message{{Role: "user", Content: prompt}}
    resp, err := s.llmClient.Chat(ctx, msgs)
    if err != nil {
        return
    }
    var result struct {
        Anomaly bool   `json:"anomaly"`
        Reason  string `json:"reason"`
    }
    if err := json.Unmarshal([]byte(extractJSON(resp)), &result); err != nil || !result.Anomaly {
        return
    }
    alert := &models.TaskAlert{
        TaskID:    task.ID,
        TaskRunID: run.ID,
        Summary:   result.Reason,
        Status:    "open",
    }
    if err := s.taskStore.CreateAlert(alert); err != nil {
        logger.Global().Error().Err(err).Msg("scheduler: create alert")
        return
    }
    run.Alerted = true
    s.sendNotifications(ctx, alert, task)
}

func (s *Scheduler) sendNotifications(ctx context.Context, alert *models.TaskAlert, task *models.Task) {
    channels, err := s.notifyStore.List()
    if err != nil || len(channels) == 0 {
        return
    }
    for _, ch := range channels {
        if !ch.Enabled {
            continue
        }
        if err := sendNotification(ctx, ch, alert, task); err != nil {
            logger.Global().Warn().Err(err).Str("channel", ch.Name).Msg("scheduler: notify failed")
        }
    }
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max] + "...[truncated]"
}

// extractJSON finds the first {...} block in a string (LLM may wrap JSON in prose).
func extractJSON(s string) string {
    start := strings.Index(s, "{")
    end := strings.LastIndex(s, "}")
    if start < 0 || end < start {
        return s
    }
    return s[start : end+1]
}
```

- [ ] **Step 3: Add sendNotification helper**

Append to `internal/scheduler/scheduler.go`:

```go
import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "net/http"
    "net/smtp"
    "strconv"
)

func sendNotification(ctx context.Context, ch *models.NotifyChannel, alert *models.TaskAlert, task *models.Task) error {
    var cfg map[string]any
    if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
        return err
    }
    msg := fmt.Sprintf("[Spider Alert] Task: %s\n%s", task.Name, alert.Summary)
    switch ch.Type {
    case "dingtalk":
        return sendDingTalk(ctx, cfg, msg)
    case "email":
        return sendEmail(cfg, msg)
    case "webhook":
        return sendWebhook(ctx, cfg, msg)
    }
    return nil
}

func sendDingTalk(ctx context.Context, cfg map[string]any, msg string) error {
    webhookURL, _ := cfg["webhook_url"].(string)
    if webhookURL == "" {
        return fmt.Errorf("dingtalk: missing webhook_url")
    }
    // Optional HMAC signing
    if secret, _ := cfg["secret"].(string); secret != "" {
        ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
        sign := ts + "\n" + secret
        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write([]byte(sign))
        sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
        webhookURL += "&timestamp=" + ts + "&sign=" + sig
    }
    body, _ := json.Marshal(map[string]any{
        "msgtype": "text",
        "text":    map[string]string{"content": msg},
    })
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    resp.Body.Close()
    return nil
}

func sendEmail(cfg map[string]any, msg string) error {
    to, _ := cfg["to"].([]any)
    smtpHost, _ := cfg["smtp_host"].(string)
    smtpPortF, _ := cfg["smtp_port"].(float64)
    username, _ := cfg["username"].(string)
    password, _ := cfg["password"].(string)
    if smtpHost == "" || len(to) == 0 {
        return fmt.Errorf("email: missing smtp_host or to")
    }
    var toAddrs []string
    for _, t := range to {
        if s, ok := t.(string); ok {
            toAddrs = append(toAddrs, s)
        }
    }
    addr := fmt.Sprintf("%s:%d", smtpHost, int(smtpPortF))
    auth := smtp.PlainAuth("", username, password, smtpHost)
    body := fmt.Sprintf("Subject: [Spider Alert]\r\n\r\n%s", msg)
    return smtp.SendMail(addr, auth, username, toAddrs, []byte(body))
}

func sendWebhook(ctx context.Context, cfg map[string]any, msg string) error {
    url, _ := cfg["url"].(string)
    method, _ := cfg["method"].(string)
    if url == "" {
        return fmt.Errorf("webhook: missing url")
    }
    if method == "" {
        method = "POST"
    }
    body, _ := json.Marshal(map[string]string{"message": msg})
    req, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    if headers, ok := cfg["headers"].(map[string]any); ok {
        for k, v := range headers {
            if s, ok := v.(string); ok {
                req.Header.Set(k, s)
            }
        }
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    resp.Body.Close()
    return nil
}
```

- [ ] **Step 4: Call summarize() in runTask()**

In `runTask()`, after `run.Status = "success"` / `run.Status = "failed"`, before `UpdateRun`:

```go
// Replace the existing UpdateRun call with:
if err := s.taskStore.UpdateRun(run); err != nil {
    log.Error().Err(err).Msg("scheduler: update run")
}
go s.summarize(ctx, run, task)
```

Remove the old `s.taskStore.UpdateRun(run)` call that was there before.

- [ ] **Step 5: Update main.go to pass llmClient to scheduler**

In `cmd/spider/main.go`, update the scheduler init:

```go
if agentFactory != nil {
    sched := scheduler.New(app.TaskStore, app.NotifyChannelStore, agentFactory, agentFactory.LLMClient, agentFactory.LLMModel)
    sched.Start(shutdownCtx)
    defer sched.Stop()
}
```

- [ ] **Step 6: Build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/scheduler/scheduler.go cmd/spider/main.go
git commit -m "feat(scheduler): post-run LLM summary, anomaly check, notifications"
```

---

### Task 9: REST API

**Files:**
- Create: `internal/api/tasks.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Implement task handlers**

Create `internal/api/tasks.go`:

```go
package api

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"

    mcppkg "github.com/spiderai/spider/internal/mcp"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/scheduler"
)

// --- Tasks ---

func listTasks(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    tasks, err := app.TaskStore.List()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    if tasks == nil {
        tasks = []*models.Task{}
    }
    writeJSON(w, http.StatusOK, tasks)
}

func createTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    var task models.Task
    if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }
    if task.Name == "" || task.Goal == "" {
        writeError(w, http.StatusBadRequest, "name and goal required")
        return
    }
    task.Status = "active"
    if err := app.TaskStore.Create(&task); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    writeJSON(w, http.StatusCreated, task)
}

func getTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    task, err := app.TaskStore.Get(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "task not found")
        return
    }
    writeJSON(w, http.StatusOK, task)
}

func updateTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    task, err := app.TaskStore.Get(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "task not found")
        return
    }
    if err := json.NewDecoder(r.Body).Decode(task); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }
    task.ID = id
    if err := app.TaskStore.Update(task); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    writeJSON(w, http.StatusOK, task)
}

func deleteTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    if err := app.TaskStore.Delete(id); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func triggerTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    task, err := app.TaskStore.Get(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "task not found")
        return
    }
    if app.Scheduler == nil {
        writeError(w, http.StatusServiceUnavailable, "scheduler not available")
        return
    }
    runID, err := app.Scheduler.TriggerNow(r.Context(), task)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    writeJSON(w, http.StatusAccepted, map[string]int64{"run_id": runID})
}

func listTaskRuns(app *mcppkg.App, w http.ResponseWriter, r *http.Request, taskID int64) {
    runs, err := app.TaskStore.ListRuns(taskID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    if runs == nil {
        runs = []*models.TaskRun{}
    }
    // Truncate raw_output for list view
    for _, run := range runs {
        if len(run.RawOutput) > 10240 {
            run.RawOutput = run.RawOutput[:10240] + "...[truncated]"
        }
    }
    writeJSON(w, http.StatusOK, runs)
}

func getTaskRun(app *mcppkg.App, w http.ResponseWriter, r *http.Request, runID int64) {
    run, err := app.TaskStore.GetRun(runID)
    if err != nil {
        writeError(w, http.StatusNotFound, "run not found")
        return
    }
    if len(run.RawOutput) > 10240 {
        run.RawOutput = run.RawOutput[:10240] + "...[truncated]"
    }
    writeJSON(w, http.StatusOK, run)
}

// --- Alerts ---

func listAlerts(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    alerts, err := app.TaskStore.ListAlerts()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    if alerts == nil {
        alerts = []*models.TaskAlert{}
    }
    writeJSON(w, http.StatusOK, alerts)
}

func resolveAlert(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    if err := app.TaskStore.ResolveAlert(id); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// --- NotifyChannels ---

func listNotifyChannels(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    channels, err := app.NotifyChannelStore.List()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    if channels == nil {
        channels = []*models.NotifyChannel{}
    }
    // Mask config secrets in list response
    for _, ch := range channels {
        ch.Config = maskConfig(ch.Config)
    }
    writeJSON(w, http.StatusOK, channels)
}

func createNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    var ch models.NotifyChannel
    if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }
    if ch.Type == "" || ch.Name == "" {
        writeError(w, http.StatusBadRequest, "type and name required")
        return
    }
    if err := app.NotifyChannelStore.Create(&ch); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    ch.Config = maskConfig(ch.Config)
    writeJSON(w, http.StatusCreated, ch)
}

func updateNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    ch, err := app.NotifyChannelStore.Get(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "channel not found")
        return
    }
    if err := json.NewDecoder(r.Body).Decode(ch); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }
    ch.ID = id
    if err := app.NotifyChannelStore.Update(ch); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    ch.Config = maskConfig(ch.Config)
    writeJSON(w, http.StatusOK, ch)
}

func deleteNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    if err := app.NotifyChannelStore.Delete(id); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func testNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
    ch, err := app.NotifyChannelStore.Get(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "channel not found")
        return
    }
    testAlert := &models.TaskAlert{Summary: "Spider 通知渠道测试消息"}
    testTask := &models.Task{Name: "Test"}
    if err := scheduler.SendNotificationPublic(r.Context(), ch, testAlert, testTask); err != nil {
        writeError(w, http.StatusBadGateway, err.Error())
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// maskConfig replaces sensitive values in config JSON with "***".
func maskConfig(config string) string {
    var m map[string]any
    if err := json.Unmarshal([]byte(config), &m); err != nil {
        return config
    }
    for _, key := range []string{"secret", "password"} {
        if _, ok := m[key]; ok {
            m[key] = "***"
        }
    }
    if headers, ok := m["headers"].(map[string]any); ok {
        for k := range headers {
            headers[k] = "***"
        }
    }
    b, _ := json.Marshal(m)
    return string(b)
}

func parseTaskID(s string) (int64, bool) {
    id, err := strconv.ParseInt(s, 10, 64)
    return id, err == nil
}
```

Note: `scheduler.SendNotificationPublic` requires exporting `sendNotification` in scheduler.go — rename it to `SendNotificationPublic` and export it.

- [ ] **Step 2: Add routes to handler.go**

In `internal/api/handler.go`, before the `// log-level endpoint` comment, add:

```go
// Tasks
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
    // /api/v1/tasks/:id
    // /api/v1/tasks/:id/trigger
    // /api/v1/tasks/:id/runs
    // /api/v1/tasks/:id/runs/:run_id
    id := rest
    sub := ""
    if idx := indexOf(rest, '/'); idx >= 0 {
        id = rest[:idx]
        sub = rest[idx+1:]
    }
    taskID, ok := parseTaskID(id)
    if !ok {
        http.Error(w, "invalid task id", http.StatusBadRequest)
        return
    }
    switch sub {
    case "":
        switch r.Method {
        case http.MethodGet:
            getTask(app, w, r, taskID)
        case http.MethodPut:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                updateTask(app, w, r, taskID)
            })).ServeHTTP(w, r)
        case http.MethodDelete:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                deleteTask(app, w, r, taskID)
            })).ServeHTTP(w, r)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    case "trigger":
        if r.Method == http.MethodPost {
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                triggerTask(app, w, r, taskID)
            })).ServeHTTP(w, r)
        }
    case "runs":
        listTaskRuns(app, w, r, taskID)
    default:
        // /api/v1/tasks/:id/runs/:run_id
        if strings.HasPrefix(sub, "runs/") {
            runIDStr := sub[len("runs/"):]
            runID, ok := parseTaskID(runIDStr)
            if !ok {
                http.Error(w, "invalid run id", http.StatusBadRequest)
                return
            }
            getTaskRun(app, w, r, runID)
        }
    }
})

// Alerts
mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        listAlerts(app, w, r)
    }
})
mux.HandleFunc("/api/v1/alerts/", func(w http.ResponseWriter, r *http.Request) {
    rest := r.URL.Path[len("/api/v1/alerts/"):]
    id := rest
    sub := ""
    if idx := indexOf(rest, '/'); idx >= 0 {
        id = rest[:idx]
        sub = rest[idx+1:]
    }
    alertID, ok := parseTaskID(id)
    if !ok {
        http.Error(w, "invalid alert id", http.StatusBadRequest)
        return
    }
    if sub == "resolve" && r.Method == http.MethodPut {
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            resolveAlert(app, w, r, alertID)
        })).ServeHTTP(w, r)
    }
})

// NotifyChannels
mux.HandleFunc("/api/v1/notify-channels", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        listNotifyChannels(app, w, r)
    case http.MethodPost:
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            createNotifyChannel(app, w, r)
        })).ServeHTTP(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
mux.HandleFunc("/api/v1/notify-channels/", func(w http.ResponseWriter, r *http.Request) {
    rest := r.URL.Path[len("/api/v1/notify-channels/"):]
    id := rest
    sub := ""
    if idx := indexOf(rest, '/'); idx >= 0 {
        id = rest[:idx]
        sub = rest[idx+1:]
    }
    chID, ok := parseTaskID(id)
    if !ok {
        http.Error(w, "invalid channel id", http.StatusBadRequest)
        return
    }
    switch sub {
    case "":
        switch r.Method {
        case http.MethodPut:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                updateNotifyChannel(app, w, r, chID)
            })).ServeHTTP(w, r)
        case http.MethodDelete:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                deleteNotifyChannel(app, w, r, chID)
            })).ServeHTTP(w, r)
        }
    case "test":
        if r.Method == http.MethodPost {
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                testNotifyChannel(app, w, r, chID)
            })).ServeHTTP(w, r)
        }
    }
})
```

- [ ] **Step 3: Add Scheduler field to App struct**

In `internal/mcp/server.go`, add to App struct:

```go
Scheduler *scheduler.Scheduler
```

Add import: `"github.com/spiderai/spider/internal/scheduler"`

In `cmd/spider/main.go`, update scheduler init to store reference:

```go
if agentFactory != nil {
    sched := scheduler.New(app.TaskStore, app.NotifyChannelStore, agentFactory, agentFactory.LLMClient, agentFactory.LLMModel)
    sched.Start(shutdownCtx)
    app.Scheduler = sched
    defer sched.Stop()
}
```

- [ ] **Step 4: Build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Smoke test API**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data &
sleep 2
curl -s http://localhost:8002/api/v1/tasks | head -5
```
Expected: `[]` (empty array, no auth error since auth may be disabled in dev).

Kill the test server after.

- [ ] **Step 6: Commit**

```bash
git add internal/api/tasks.go internal/api/handler.go internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(api): task, alert, notify-channel REST endpoints"
```

---

### Task 10: Frontend API client

**Files:**
- Create: `web/src/api/tasks.ts`

- [ ] **Step 1: Write TypeScript API client**

Create `web/src/api/tasks.ts`:

```typescript
import { authHeaders } from './auth'

export interface Task {
  id: number
  name: string
  goal: string
  host_ids: number[]
  schedule: string
  alert_on_anomaly: boolean
  status: string
  source_conv_id: string
  created_at: string
  updated_at: string
}

export interface TaskRun {
  id: number
  task_id: number
  started_at: string
  finished_at: string | null
  status: string
  raw_output: string
  summary: string
  alerted: boolean
}

export interface TaskAlert {
  id: number
  task_id: number
  task_run_id: number
  summary: string
  status: string
  created_at: string
  resolved_at: string | null
}

export interface NotifyChannel {
  id: number
  type: string
  name: string
  config: string
  enabled: boolean
  created_at: string
}

export async function listTasks(): Promise<Task[]> {
  const res = await fetch('/api/v1/tasks', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createTask(task: Omit<Task, 'id' | 'created_at' | 'updated_at'>): Promise<Task> {
  const res = await fetch('/api/v1/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(task),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function getTask(id: number): Promise<Task> {
  const res = await fetch(`/api/v1/tasks/${id}`, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function updateTask(id: number, task: Partial<Task>): Promise<Task> {
  const res = await fetch(`/api/v1/tasks/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(task),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteTask(id: number): Promise<void> {
  const res = await fetch(`/api/v1/tasks/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function triggerTask(id: number): Promise<{ run_id: number }> {
  const res = await fetch(`/api/v1/tasks/${id}/trigger`, { method: 'POST', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function listTaskRuns(taskId: number): Promise<TaskRun[]> {
  const res = await fetch(`/api/v1/tasks/${taskId}/runs`, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function listAlerts(): Promise<TaskAlert[]> {
  const res = await fetch('/api/v1/alerts', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function resolveAlert(id: number): Promise<void> {
  const res = await fetch(`/api/v1/alerts/${id}/resolve`, { method: 'PUT', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function listNotifyChannels(): Promise<NotifyChannel[]> {
  const res = await fetch('/api/v1/notify-channels', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createNotifyChannel(ch: Omit<NotifyChannel, 'id' | 'created_at'>): Promise<NotifyChannel> {
  const res = await fetch('/api/v1/notify-channels', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(ch),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function updateNotifyChannel(id: number, ch: Partial<NotifyChannel>): Promise<NotifyChannel> {
  const res = await fetch(`/api/v1/notify-channels/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(ch),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteNotifyChannel(id: number): Promise<void> {
  const res = await fetch(`/api/v1/notify-channels/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function testNotifyChannel(id: number): Promise<void> {
  const res = await fetch(`/api/v1/notify-channels/${id}/test`, { method: 'POST', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}
```

- [ ] **Step 2: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -10
```
Expected: no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/api/tasks.ts
git commit -m "feat(web): tasks API client"
```

---

### Task 11: Frontend TaskView

**Files:**
- Create: `web/src/views/TaskView.vue`
- Modify: `web/src/main.ts`
- Modify: `web/src/App.vue`

Layout: left list (task names + status badges) + right panel (config summary at top, execution records below). Matches HostsView pattern.

- [ ] **Step 1: Add route and nav link**

In `web/src/main.ts`, add after the topology route:

```typescript
{ path: '/tasks', component: () => import('./views/TaskView.vue') },
```

In `web/src/App.vue`, add after the topology nav link:

```html
<RouterLink to="/tasks" class="nav-item">任务</RouterLink>
```

- [ ] **Step 2: Create TaskView.vue (template section)**

Create `web/src/views/TaskView.vue` with the template:

```vue
<template>
  <div class="task-view">
    <!-- Left sidebar -->
    <aside class="task-sidebar">
      <div class="sidebar-header">
        <span class="sidebar-title">任务</span>
        <button class="btn-primary btn-sm" @click="openCreateModal">新建</button>
      </div>
      <div v-if="tasks.length === 0" class="empty-hint">暂无任务</div>
      <ul class="task-list">
        <li
          v-for="task in tasks"
          :key="task.id"
          class="task-item"
          :class="{ active: selectedTask?.id === task.id }"
          @click="selectTask(task)"
        >
          <div class="task-item-name">
            <span v-if="task.alert_on_anomaly" class="alert-icon" title="告警已启用">🔔</span>
            {{ task.name }}
          </div>
          <div class="task-item-meta">
            <span class="badge" :class="statusClass(task.status)">{{ task.status }}</span>
            <span class="task-schedule">{{ task.schedule || '手动' }}</span>
          </div>
        </li>
      </ul>
    </aside>

    <!-- Right panel -->
    <main class="task-main" v-if="selectedTask">
      <!-- Config summary -->
      <div class="task-header">
        <div class="task-header-top">
          <h2>{{ selectedTask.name }}</h2>
          <span class="badge" :class="statusClass(selectedTask.status)">{{ selectedTask.status }}</span>
        </div>
        <div class="task-actions">
          <button class="btn-primary btn-sm" @click="triggerTask" :disabled="triggering">
            {{ triggering ? '执行中...' : '立即执行' }}
          </button>
          <button class="btn-secondary btn-sm" @click="openEditModal">编辑</button>
          <button class="btn-secondary btn-sm" @click="togglePause">
            {{ selectedTask.status === 'paused' ? '恢复' : '暂停' }}
          </button>
          <button class="btn-danger btn-sm" @click="confirmDelete">删除</button>
        </div>
        <div class="task-meta">
          <div class="meta-row"><span class="meta-label">目标</span><span>{{ selectedTask.goal }}</span></div>
          <div class="meta-row"><span class="meta-label">调度</span><span>{{ selectedTask.schedule || '手动触发' }}</span></div>
          <div class="meta-row"><span class="meta-label">设备</span><span>{{ selectedTask.host_ids.join(', ') || '无' }}</span></div>
          <div class="meta-row"><span class="meta-label">告警</span><span>{{ selectedTask.alert_on_anomaly ? '已启用' : '关闭' }}</span></div>
        </div>
      </div>

      <!-- Execution records -->
      <div class="runs-section">
        <h3>执行记录</h3>
        <div v-if="runs.length === 0" class="empty-hint">暂无执行记录</div>
        <div v-for="run in runs" :key="run.id" class="run-card">
          <div class="run-header" @click="toggleRun(run.id)">
            <span class="run-status-icon">{{ runIcon(run.status) }}</span>
            <span class="run-time">{{ formatTime(run.started_at) }}</span>
            <span class="run-duration">{{ duration(run) }}</span>
            <span v-if="run.alerted" class="alert-icon" title="触发告警">🔔</span>
            <span class="run-expand">{{ expandedRuns.has(run.id) ? '▲' : '▼' }}</span>
          </div>
          <div v-if="expandedRuns.has(run.id)" class="run-body">
            <div v-if="run.summary" class="run-summary">{{ run.summary }}</div>
            <pre v-if="run.raw_output" class="run-output">{{ run.raw_output }}</pre>
          </div>
        </div>
      </div>
    </main>

    <div v-else class="task-main task-main--empty">
      <p>选择左侧任务查看详情</p>
    </div>

    <!-- Create/Edit modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <h3>{{ editingTask ? '编辑任务' : '新建任务' }}</h3>
        <form @submit.prevent="saveTask">
          <div class="form-group">
            <label>名称</label>
            <input v-model="form.name" required placeholder="任务名称" />
          </div>
          <div class="form-group">
            <label>目标</label>
            <textarea v-model="form.goal" required placeholder="自然语言描述任务目标" rows="3" />
          </div>
          <div class="form-group">
            <label>调度 (cron)</label>
            <input v-model="form.schedule" placeholder="0 2 * * 3 (空=手动)" />
          </div>
          <div class="form-group form-group--inline">
            <label>发现异常时告警</label>
            <input type="checkbox" v-model="form.alert_on_anomaly" />
          </div>
          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="closeModal">取消</button>
            <button type="submit" class="btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Add script section to TaskView.vue**

Append to `web/src/views/TaskView.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  type Task, type TaskRun,
  listTasks, createTask, updateTask, deleteTask,
  triggerTask as apiTriggerTask, listTaskRuns,
} from '../api/tasks'

const tasks = ref<Task[]>([])
const selectedTask = ref<Task | null>(null)
const runs = ref<TaskRun[]>([])
const expandedRuns = ref(new Set<number>())
const showModal = ref(false)
const editingTask = ref<Task | null>(null)
const triggering = ref(false)

const form = ref({
  name: '',
  goal: '',
  schedule: '',
  alert_on_anomaly: false,
  host_ids: [] as number[],
})

onMounted(loadTasks)

async function loadTasks() {
  tasks.value = await listTasks()
}

async function selectTask(task: Task) {
  selectedTask.value = task
  expandedRuns.value.clear()
  runs.value = await listTaskRuns(task.id)
}

function openCreateModal() {
  editingTask.value = null
  form.value = { name: '', goal: '', schedule: '', alert_on_anomaly: false, host_ids: [] }
  showModal.value = true
}

function openEditModal() {
  if (!selectedTask.value) return
  editingTask.value = selectedTask.value
  form.value = {
    name: selectedTask.value.name,
    goal: selectedTask.value.goal,
    schedule: selectedTask.value.schedule,
    alert_on_anomaly: selectedTask.value.alert_on_anomaly,
    host_ids: [...selectedTask.value.host_ids],
  }
  showModal.value = true
}

function closeModal() { showModal.value = false }

async function saveTask() {
  if (editingTask.value) {
    const updated = await updateTask(editingTask.value.id, form.value)
    const idx = tasks.value.findIndex(t => t.id === updated.id)
    if (idx >= 0) tasks.value[idx] = updated
    selectedTask.value = updated
  } else {
    const created = await createTask({ ...form.value, status: 'active', source_conv_id: '' })
    tasks.value.unshift(created)
    selectedTask.value = created
  }
  closeModal()
}

async function triggerTask() {
  if (!selectedTask.value) return
  triggering.value = true
  try {
    await apiTriggerTask(selectedTask.value.id)
    setTimeout(() => selectTask(selectedTask.value!), 1000)
  } finally {
    triggering.value = false
  }
}

async function togglePause() {
  if (!selectedTask.value) return
  const newStatus = selectedTask.value.status === 'paused' ? 'active' : 'paused'
  const updated = await updateTask(selectedTask.value.id, { status: newStatus })
  selectedTask.value = updated
  const idx = tasks.value.findIndex(t => t.id === updated.id)
  if (idx >= 0) tasks.value[idx] = updated
}

async function confirmDelete() {
  if (!selectedTask.value) return
  if (!confirm(`删除任务 "${selectedTask.value.name}"？`)) return
  await deleteTask(selectedTask.value.id)
  tasks.value = tasks.value.filter(t => t.id !== selectedTask.value!.id)
  selectedTask.value = null
  runs.value = []
}

function toggleRun(id: number) {
  if (expandedRuns.value.has(id)) expandedRuns.value.delete(id)
  else expandedRuns.value.add(id)
}

function statusClass(status: string) {
  return { 'badge-ok': status === 'active', 'badge-warn': status === 'paused', 'badge-err': status === 'archived' }
}

function runIcon(status: string) {
  return { running: '⏳', success: '✅', failed: '❌' }[status] ?? '❓'
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleString('zh-CN')
}

function duration(run: TaskRun) {
  if (!run.finished_at) return '进行中'
  const ms = new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()
  return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`
}
</script>
```

- [ ] **Step 4: Add styles to TaskView.vue**

Append to `web/src/views/TaskView.vue`:

```vue
<style scoped>
.task-view { display: flex; height: 100%; overflow: hidden; }
.task-sidebar { width: 260px; min-width: 200px; border-right: 1px solid var(--border); display: flex; flex-direction: column; overflow: hidden; }
.sidebar-header { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.sidebar-title { font-weight: 600; font-size: 15px; }
.task-list { flex: 1; overflow-y: auto; list-style: none; margin: 0; padding: 0; }
.task-item { padding: 10px 16px; cursor: pointer; border-bottom: 1px solid var(--border-light, var(--border)); }
.task-item:hover, .task-item.active { background: var(--bg-hover, var(--bg-secondary)); }
.task-item-name { font-size: 14px; font-weight: 500; margin-bottom: 4px; }
.task-item-meta { display: flex; gap: 8px; align-items: center; font-size: 12px; color: var(--text-secondary); }
.task-schedule { font-family: monospace; }
.alert-icon { font-size: 12px; }
.task-main { flex: 1; overflow-y: auto; padding: 20px 24px; }
.task-main--empty { display: flex; align-items: center; justify-content: center; color: var(--text-secondary); }
.task-header { margin-bottom: 24px; }
.task-header-top { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.task-header-top h2 { margin: 0; font-size: 18px; }
.task-actions { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
.task-meta { display: flex; flex-direction: column; gap: 6px; }
.meta-row { display: flex; gap: 12px; font-size: 14px; }
.meta-label { color: var(--text-secondary); min-width: 60px; }
.runs-section h3 { font-size: 15px; margin-bottom: 12px; }
.run-card { border: 1px solid var(--border); border-radius: 6px; margin-bottom: 8px; overflow: hidden; }
.run-header { display: flex; align-items: center; gap: 10px; padding: 10px 14px; cursor: pointer; background: var(--bg-secondary); font-size: 13px; }
.run-header:hover { background: var(--bg-hover, var(--bg-secondary)); }
.run-time { flex: 1; }
.run-duration { color: var(--text-secondary); }
.run-expand { color: var(--text-secondary); }
.run-body { padding: 12px 14px; }
.run-summary { font-size: 13px; margin-bottom: 8px; color: var(--text-secondary); }
.run-output { font-size: 12px; background: var(--bg-code, #1e1e1e); color: var(--text-code, #d4d4d4); padding: 10px; border-radius: 4px; overflow-x: auto; white-space: pre-wrap; max-height: 300px; overflow-y: auto; }
.empty-hint { padding: 20px; text-align: center; color: var(--text-secondary); font-size: 13px; }
.badge { font-size: 11px; padding: 2px 7px; border-radius: 10px; font-weight: 500; }
.badge-ok { background: var(--badge-ok-bg, #d1fae5); color: var(--badge-ok-text, #065f46); }
.badge-warn { background: var(--badge-warn-bg, #fef3c7); color: var(--badge-warn-text, #92400e); }
.badge-err { background: var(--badge-err-bg, #fee2e2); color: var(--badge-err-text, #991b1b); }
.btn-sm { padding: 4px 12px; font-size: 13px; border-radius: 4px; cursor: pointer; border: none; }
.btn-primary { background: var(--primary, #3b82f6); color: white; }
.btn-secondary { background: var(--bg-secondary); color: var(--text); border: 1px solid var(--border); }
.btn-danger { background: #ef4444; color: white; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg); border-radius: 8px; padding: 24px; width: 480px; max-width: 90vw; }
.modal h3 { margin: 0 0 16px; }
.form-group { margin-bottom: 14px; display: flex; flex-direction: column; gap: 4px; }
.form-group--inline { flex-direction: row; align-items: center; gap: 10px; }
.form-group label { font-size: 13px; color: var(--text-secondary); }
.form-group input, .form-group textarea { padding: 7px 10px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg); color: var(--text); font-size: 14px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
```

- [ ] **Step 5: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -15
```
Expected: no errors.

- [ ] **Step 6: Build Go binary and verify**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data &
sleep 2
```

Open browser at http://localhost:8002/tasks — verify:
- "任务" nav link appears
- Left sidebar shows "暂无任务"
- "新建" button opens modal
- Create a test task, verify it appears in list
- Click task, verify right panel shows config summary

Kill test server after.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/TaskView.vue web/src/main.ts web/src/App.vue
git commit -m "feat(web): TaskView — left list + right detail + create/edit modal"
```

---

### Task 12: NotifyChannel settings in ProfileView

**Files:**
- Modify: `web/src/views/ProfileView.vue`

Add a "通知渠道" section to the profile/settings page.

- [ ] **Step 1: Read current ProfileView structure**

```bash
grep -n "section\|<h\|<div class" /Users/cw/fty.ai/spider.ai/web/src/views/ProfileView.vue | head -30
```

Identify where to add the new section (after existing sections).

- [ ] **Step 2: Add notify channels section**

In `ProfileView.vue`, add a new section for notify channels. The section should:
- List existing channels (type badge, name, enabled toggle)
- "新建渠道" button opens a form
- Form fields: 类型 (select: dingtalk/email/webhook), 名称, 配置 (JSON textarea), 启用
- Each channel row has 编辑, 测试, 删除 buttons

Import from `../api/tasks`: `listNotifyChannels, createNotifyChannel, updateNotifyChannel, deleteNotifyChannel, testNotifyChannel, type NotifyChannel`

The exact implementation depends on ProfileView's current structure — follow the existing section pattern.

- [ ] **Step 3: Build and verify**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -10
```

Open http://localhost:8002/profile — verify notify channels section renders.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(web): notify channel management in profile settings"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Task model (id, name, goal, host_ids, schedule, alert_on_anomaly, status, source_conv_id) | Task 2 |
| TaskRun model | Task 2 |
| TaskAlert model | Task 2 |
| NotifyChannel model (dingtalk/email/webhook, encrypted config) | Task 2, Task 4 |
| DB tables: tasks, task_runs, task_alerts, notify_channels | Task 1 |
| TaskStore CRUD | Task 3 |
| NotifyChannelStore with encryption | Task 4 |
| CreateTask agent tool | Task 5 |
| CreateTask system prompt guidance | Task 5 |
| Wire CreateTask into factory | Task 5 |
| DB-polling scheduler (every minute) | Task 6 |
| Headless Agent execution | Task 6 |
| Post-execution LLM summary | Task 8 |
| Anomaly check + TaskAlert creation | Task 8 |
| DingTalk / Email / Webhook notifications | Task 8 |
| Wire stores + scheduler into main | Task 7 |
| REST API: /api/v1/tasks CRUD + trigger + runs | Task 9 |
| REST API: /api/v1/alerts list + resolve | Task 9 |
| REST API: /api/v1/notify-channels CRUD + test | Task 9 |
| Frontend TaskView (left list + right detail) | Task 11 |
| Frontend: create/edit modal | Task 11 |
| Frontend: execution records expandable | Task 11 |
| Frontend: nav link | Task 11 |
| Frontend: notify channel settings in profile | Task 12 |
| Frontend API client | Task 10 |

All spec requirements covered.

**Type consistency check:** `TaskStore`, `NotifyChannelStore`, `CreateTaskTool`, `Scheduler` — all names consistent across tasks. `boolToInt` defined in task_store.go and reused in notify_channel_store.go — note: `boolToInt` will be a duplicate if both files are in the same package. Move it to a shared helper or define it only in task_store.go and reference it from notify_channel_store.go (same package, so it's accessible).

**Fix:** In Task 4 (notify_channel_store.go), do NOT redefine `boolToInt` — it's already defined in task_store.go in the same `store` package.

**Scheduler.New() signature:** Updated in Task 8 to include `notifyStore` and `llmClient`. Task 7 uses the old signature — **fix:** Task 7 step 2 should use the final signature from Task 8. Since Task 8 comes after Task 7, the implementer should apply Task 8's constructor change when doing Task 7, or update in Task 8. The plan notes this in Task 8 step 5.

**`scheduler.SendNotificationPublic`:** Task 9 references this exported function. Task 8 defines `sendNotification` (unexported). The implementer must rename it to `SendNotificationPublic` when implementing Task 8.

**`agent.Run` signature:** Task 6 step 4 explicitly tells the implementer to check the actual signature before using it. This is correct — the plan cannot assume the exact signature without reading the full agent.go.

