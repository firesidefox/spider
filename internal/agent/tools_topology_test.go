package agent

import (
	"context"
	"strings"
	"testing"
)

func TestGetTopologyToolNilStore(t *testing.T) {
	tool := NewGetTopologyTool(nil)
	result, err := tool.Execute(context.Background(), map[string]any{
		"topology_id": "1",
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

func TestGetTopologyContextToolNilStore(t *testing.T) {
	tool := NewGetTopologyContextTool(nil)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_name": "test-host",
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
