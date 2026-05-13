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

	// Create multiple task runs
	for i := 0; i < 3; i++ {
		taskRun := &models.TaskRun{
			TaskID:    createdTask.ID,
			StartedAt: time.Now().Add(time.Duration(i) * time.Second),
			Status:    models.TaskRunStatusRunning,
		}
		_, err := store.Create(taskRun)
		if err != nil {
			t.Fatalf("failed to create task run %d: %v", i, err)
		}
	}

	// List task runs
	results, err := store.ListByTaskID(createdTask.ID)
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

	results, err := store.ListByTaskID("nonexistent-task-id")
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("ListByTaskID() returned %d runs, want 0", len(results))
	}
}
