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

	got, err := s.Update("conv-1", task.ID, "", "", "", "in_progress", "", nil)
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
	s.Update("conv-1", task.ID, "", "", "", "deleted", "", nil)

	tasks, _ := s.List("conv-1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestTodoStore_BlockedBy(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	t1 := &models.Todo{ConversationID: "conv-1", Subject: "t1", Status: "pending"}
	t2 := &models.Todo{ConversationID: "conv-1", Subject: "t2", Status: "pending"}
	s.Create(t1)
	s.Create(t2)

	s.Update("conv-1", t2.ID, "", "", "", "", "", []int64{t1.ID})

	got, _ := s.Get(t2.ID)
	if len(got.BlockedBy) != 1 || got.BlockedBy[0] != t1.ID {
		t.Errorf("unexpected blocked_by: %v", got.BlockedBy)
	}
}

func TestTodoStore_ConcurrentWrites(t *testing.T) {
	s := NewTodoStore(setupTestDB(t))

	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(i int) {
			task := &models.Todo{ConversationID: "conv-1", Subject: "task", Status: "pending"}
			done <- s.Create(task)
		}(i)
	}
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Create: %v", err)
		}
	}
}
