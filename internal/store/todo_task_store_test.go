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
