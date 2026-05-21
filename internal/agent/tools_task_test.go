package agent

import (
	"context"
	"strings"
	"testing"
)

func TestCreateTaskToolNilStore(t *testing.T) {
	tool := NewCreateTaskTool(nil, "test-conv")
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "test task",
		"goal": "test goal",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true when store is nil")
	}
	if !strings.Contains(result.Content, "not configured") {
		t.Errorf("Expected 'not configured' in content, got: %s", result.Content)
	}
}
