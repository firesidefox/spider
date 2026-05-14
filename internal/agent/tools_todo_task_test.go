package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/store"
)

func jsonPart(s string) string {
	if i := strings.Index(s, "\n\n"); i != -1 {
		return s[:i]
	}
	return s
}

type mockBroadcaster struct {
	broadcasts [][]byte
}

func (m *mockBroadcaster) BroadcastSSE(_ string, data []byte) {
	m.broadcasts = append(m.broadcasts, data)
}

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

func TestTodoTool_Create(t *testing.T) {
	tool, bc := newTestTodoTool(t)
	res, err := tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "check device"})
	if err != nil || res.IsError {
		t.Fatalf("unexpected error: %v / %s", err, res.Content)
	}
	var out map[string]any
	json.Unmarshal([]byte(jsonPart(res.Content)), &out)
	if out["id"] == nil {
		t.Error("expected id in response")
	}
	if out["subject"] == nil {
		t.Error("expected subject in response")
	}
	if len(bc.broadcasts) != 1 {
		t.Errorf("expected 1 broadcast, got %d", len(bc.broadcasts))
	}
}

func TestTodoTool_CreateMissingSubject(t *testing.T) {
	tool, _ := newTestTodoTool(t)
	res, _ := tool.Execute(context.Background(), map[string]any{"action": "create"})
	if !res.IsError {
		t.Error("expected error for missing subject")
	}
}

func TestTodoTool_Update(t *testing.T) {
	tool, bc := newTestTodoTool(t)
	res, _ := tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "task1"})
	var created map[string]any
	json.Unmarshal([]byte(jsonPart(res.Content)), &created)
	id := created["id"].(float64)

	res, err := tool.Execute(context.Background(), map[string]any{"action": "update", "task_id": id, "status": "in_progress"})
	if err != nil || res.IsError {
		t.Fatalf("update failed: %v / %s", err, res.Content)
	}
	var task map[string]any
	json.Unmarshal([]byte(jsonPart(res.Content)), &task)
	if task["status"] != "in_progress" {
		t.Errorf("expected in_progress, got %v", task["status"])
	}
	if len(bc.broadcasts) != 2 {
		t.Errorf("expected 2 broadcasts, got %d", len(bc.broadcasts))
	}
}

func TestTodoTool_UpdateEmptyFields(t *testing.T) {
	tool, _ := newTestTodoTool(t)
	res, _ := tool.Execute(context.Background(), map[string]any{"action": "update", "task_id": float64(1)})
	if !res.IsError {
		t.Error("expected error for empty update")
	}
}

func TestTodoTool_UpdateMissingTaskID(t *testing.T) {
	tool, _ := newTestTodoTool(t)
	res, _ := tool.Execute(context.Background(), map[string]any{"action": "update", "status": "completed"})
	if !res.IsError {
		t.Error("expected error for missing task_id")
	}
}

func TestTodoTool_List(t *testing.T) {
	tool, _ := newTestTodoTool(t)
	tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "t1"})
	tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "t2"})

	res, err := tool.Execute(context.Background(), map[string]any{"action": "list"})
	if err != nil || res.IsError {
		t.Fatalf("list failed: %v / %s", err, res.Content)
	}
	var tasks []map[string]any
	json.Unmarshal([]byte(res.Content), &tasks)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTodoTool_UnknownAction(t *testing.T) {
	tool, _ := newTestTodoTool(t)
	res, _ := tool.Execute(context.Background(), map[string]any{"action": "delete"})
	if !res.IsError {
		t.Error("expected error for unknown action")
	}
}

func TestTodoTool_SummaryBroadcastOnTurnComplete(t *testing.T) {
	tool, bc := newTestTodoTool(t)

	// create two tasks in same turn
	tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "task A"})
	tool.Execute(context.Background(), map[string]any{"action": "create", "subject": "task B"})

	// complete first — no summary yet
	tool.Execute(context.Background(), map[string]any{"action": "update", "task_id": float64(1), "status": "completed"})
	for _, p := range bc.broadcasts {
		var m map[string]any
		json.Unmarshal(p, &m)
		if m["type"] == "todo_summary" {
			t.Fatal("should not broadcast summary after first task")
		}
	}

	// complete second — summary must fire
	bc.broadcasts = nil
	tool.Execute(context.Background(), map[string]any{"action": "update", "task_id": float64(2), "status": "completed"})
	found := false
	for _, p := range bc.broadcasts {
		var m map[string]any
		json.Unmarshal(p, &m)
		if m["type"] == "todo_summary" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected todo_summary broadcast after all tasks complete")
	}
}
