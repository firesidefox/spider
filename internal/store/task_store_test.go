package store

import (
	"testing"

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
		HostIDs:          []string{"host-1", "host-2"},
		Schedule:         "0 2 * * *",
		NotifyMode:       models.NotifyNone,
		RunRetentionDays: 30,
		TimeoutMinutes:   30,
		Status:           models.TaskStatusActive,
		SourceConvID:     "conv-123",
	}

	created, err := store.Create(task)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

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
		HostIDs:          []string{"host-1", "host-2"},
		Schedule:         "0 2 * * *",
		NotifyMode:       models.NotifyNone,
		RunRetentionDays: 30,
		TimeoutMinutes:   30,
		Status:           models.TaskStatusActive,
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

func TestTaskStore_List(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)

	task1 := &models.Task{Name: "task1", Goal: "goal1", HostIDs: []string{"host-1"}, Status: models.TaskStatusActive}
	task2 := &models.Task{Name: "task2", Goal: "goal2", HostIDs: []string{"host-2"}, Status: models.TaskStatusPaused}

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

func TestTaskStore_Get_NotFound(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)

	_, err = store.Get("non-existent-id")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskStore_Create_EmptyName(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)
	task := &models.Task{
		Name:    "",
		Goal:    "test goal",
		HostIDs: []string{"host-1"},
		Status:  models.TaskStatusActive,
	}

	_, err = store.Create(task)
	if err == nil {
		t.Error("expected error for empty name, got nil")
	}
	if err.Error() != "task name cannot be empty" {
		t.Errorf("expected 'task name cannot be empty', got %v", err)
	}
}

func TestTaskStore_Create_EmptyHostIDs(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)
	task := &models.Task{
		Name:    "test-task",
		Goal:    "test goal",
		HostIDs: []string{},
		Status:  models.TaskStatusActive,
	}

	_, err = store.Create(task)
	if err == nil {
		t.Error("expected error for empty host IDs, got nil")
	}
	if err.Error() != "task must have at least one host" {
		t.Errorf("expected 'task must have at least one host', got %v", err)
	}
}

func TestTaskStore_List_Empty(t *testing.T) {
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := NewTaskStore(database)

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

