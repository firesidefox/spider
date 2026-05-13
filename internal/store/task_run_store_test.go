package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
)

func setupTaskRunTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return database
}

func TestTaskRunStore_Create(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	// Create a task first
	taskStore := NewTaskStore(database)
	task := &models.Task{
		Name:    "test-task",
		Goal:    "test goal",
		HostIDs: []string{"host1"},
		Status:  models.TaskStatusActive,
	}
	createdTask, err := taskStore.Create(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create a task run
	now := time.Now()
	taskRun := &models.TaskRun{
		TaskID:    createdTask.ID,
		StartedAt: now,
		Status:    models.TaskRunStatusRunning,
	}

	result, err := store.Create(taskRun)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if result.ID == "" {
		t.Error("Create() returned empty ID")
	}
	if result.TaskID != createdTask.ID {
		t.Errorf("Create() TaskID = %v, want %v", result.TaskID, createdTask.ID)
	}
	if result.Status != models.TaskRunStatusRunning {
		t.Errorf("Create() Status = %v, want %v", result.Status, models.TaskRunStatusRunning)
	}
}

func TestTaskRunStore_Create_EmptyTaskID(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	taskRun := &models.TaskRun{
		TaskID:    "",
		StartedAt: time.Now(),
		Status:    models.TaskRunStatusRunning,
	}

	_, err := store.Create(taskRun)
	if err == nil {
		t.Error("Create() expected error for empty TaskID, got nil")
	}
}

func TestTaskRunStore_Create_EmptyStatus(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	// "task-123" is intentionally fake — this test exercises the in-memory
	// Status guard (checked before any DB insert), so a real task ID is not needed.
	taskRun := &models.TaskRun{
		TaskID:    "task-123",
		StartedAt: time.Now(),
		Status:    "",
	}

	_, err := store.Create(taskRun)
	if err == nil {
		t.Error("Create() expected error for empty Status, got nil")
	}
}

func TestTaskRunStore_Get(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	// Create a task first
	taskStore := NewTaskStore(database)
	task := &models.Task{
		Name:    "test-task",
		Goal:    "test goal",
		HostIDs: []string{"host1"},
		Status:  models.TaskStatusActive,
	}
	createdTask, err := taskStore.Create(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create a task run
	now := time.Now()
	taskRun := &models.TaskRun{
		TaskID:    createdTask.ID,
		StartedAt: now,
		Status:    models.TaskRunStatusRunning,
		RawOutput: "test output",
	}

	created, err := store.Create(taskRun)
	if err != nil {
		t.Fatalf("failed to create task run: %v", err)
	}

	// Get the task run
	result, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result.ID != created.ID {
		t.Errorf("Get() ID = %v, want %v", result.ID, created.ID)
	}
	if result.TaskID != createdTask.ID {
		t.Errorf("Get() TaskID = %v, want %v", result.TaskID, createdTask.ID)
	}
	if result.Status != models.TaskRunStatusRunning {
		t.Errorf("Get() Status = %v, want %v", result.Status, models.TaskRunStatusRunning)
	}
	if result.RawOutput != "test output" {
		t.Errorf("Get() RawOutput = %v, want %v", result.RawOutput, "test output")
	}
}

func TestTaskRunStore_Get_NotFound(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	_, err := store.Get("nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("Get() error = %v, want %v", err, ErrNotFound)
	}
}

func TestTaskRunStore_ListByTaskID(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	// Create a task first
	taskStore := NewTaskStore(database)
	task := &models.Task{
		Name:    "test-task",
		Goal:    "test goal",
		HostIDs: []string{"host1"},
		Status:  models.TaskStatusActive,
	}
	createdTask, err := taskStore.Create(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create multiple task runs (completed so unique-running index doesn't block)
	for i := 0; i < 3; i++ {
		taskRun := &models.TaskRun{
			TaskID:    createdTask.ID,
			StartedAt: time.Now().Add(time.Duration(i) * time.Second),
			Status:    models.TaskRunStatusCompleted,
		}
		_, err := store.Create(taskRun)
		if err != nil {
			t.Fatalf("failed to create task run %d: %v", i, err)
		}
	}

	// List task runs
	results, err := store.ListByTaskID(createdTask.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("ListByTaskID() returned %d runs, want 3", len(results))
	}

	// Verify ordering (newest first)
	for i := 0; i < len(results)-1; i++ {
		if results[i].StartedAt.Before(results[i+1].StartedAt) {
			t.Error("ListByTaskID() results not ordered by started_at DESC")
		}
	}
}

func TestTaskRunStore_ListByTaskID_Empty(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	results, err := store.ListByTaskID("nonexistent-task-id", 100, 0)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("ListByTaskID() returned %d runs, want 0", len(results))
	}
}

func TestTaskRunStore_Update(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	// Create a task first
	taskStore := NewTaskStore(database)
	task := &models.Task{
		Name:    "test-task",
		Goal:    "test goal",
		HostIDs: []string{"host1"},
		Status:  models.TaskStatusActive,
	}
	createdTask, err := taskStore.Create(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create a task run
	now := time.Now()
	taskRun := &models.TaskRun{
		TaskID:    createdTask.ID,
		StartedAt: now,
		Status:    models.TaskRunStatusRunning,
		RawOutput: "initial output",
	}

	created, err := store.Create(taskRun)
	if err != nil {
		t.Fatalf("failed to create task run: %v", err)
	}

	// Update the task run
	finishedAt := time.Now()
	created.FinishedAt = &finishedAt
	created.Status = models.TaskRunStatusCompleted
	created.RawOutput = "updated output"
	created.Summary = "test summary"
	created.Alerted = true

	err = store.Update(created)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify the update
	result, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result.Status != models.TaskRunStatusCompleted {
		t.Errorf("Update() Status = %v, want %v", result.Status, models.TaskRunStatusCompleted)
	}
	if result.RawOutput != "updated output" {
		t.Errorf("Update() RawOutput = %v, want %v", result.RawOutput, "updated output")
	}
	if result.Summary != "test summary" {
		t.Errorf("Update() Summary = %v, want %v", result.Summary, "test summary")
	}
	if !result.Alerted {
		t.Error("Update() Alerted = false, want true")
	}
	if result.FinishedAt == nil {
		t.Error("Update() FinishedAt = nil, want non-nil")
	}

	// Verify immutable fields were not changed
	if result.ID != created.ID {
		t.Errorf("Update() changed ID from %v to %v", created.ID, result.ID)
	}
	if result.TaskID != created.TaskID {
		t.Errorf("Update() changed TaskID from %v to %v", created.TaskID, result.TaskID)
	}
	if !result.StartedAt.Equal(created.StartedAt) {
		t.Errorf("Update() changed StartedAt from %v to %v", created.StartedAt, result.StartedAt)
	}
}

func TestTaskRunStore_Update_NotFound(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	taskRun := &models.TaskRun{
		ID:     "nonexistent-id",
		Status: models.TaskRunStatusCompleted,
	}

	err := store.Update(taskRun)
	if err != ErrNotFound {
		t.Errorf("Update() error = %v, want %v", err, ErrNotFound)
	}
}

func TestTaskRunStore_ListByTaskID_Pagination(t *testing.T) {
	database := setupTaskRunTestDB(t)
	defer database.Close()

	store := NewTaskRunStore(database)

	// Create a task first
	taskStore := NewTaskStore(database)
	task := &models.Task{
		Name:    "test-task",
		Goal:    "test goal",
		HostIDs: []string{"host1"},
		Status:  models.TaskStatusActive,
	}
	createdTask, err := taskStore.Create(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create 5 task runs (completed so unique-running index doesn't block)
	for i := 0; i < 5; i++ {
		taskRun := &models.TaskRun{
			TaskID:    createdTask.ID,
			StartedAt: time.Now().Add(time.Duration(i) * time.Second),
			Status:    models.TaskRunStatusCompleted,
		}
		_, err := store.Create(taskRun)
		if err != nil {
			t.Fatalf("failed to create task run %d: %v", i, err)
		}
	}

	// Test limit
	results, err := store.ListByTaskID(createdTask.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListByTaskID(limit=2) returned %d runs, want 2", len(results))
	}

	// Test offset
	results, err = store.ListByTaskID(createdTask.ID, 2, 2)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListByTaskID(limit=2, offset=2) returned %d runs, want 2", len(results))
	}

	// Test offset beyond available rows
	results, err = store.ListByTaskID(createdTask.ID, 10, 3)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListByTaskID(limit=10, offset=3) returned %d runs, want 2", len(results))
	}
}

