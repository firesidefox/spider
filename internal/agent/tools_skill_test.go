package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvokeSkillTool_Name(t *testing.T) {
	tool := NewInvokeSkillTool("/tmp")
	if tool.Name() != "invoke_skill" {
		t.Errorf("expected name 'invoke_skill', got %q", tool.Name())
	}
}

func TestInvokeSkillTool_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "deploy")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Use when deploying.\n---\n\n# Deploy Steps\n\nRun make deploy."), 0o644)

	tool := NewInvokeSkillTool(dir)
	result, err := tool.Execute(context.Background(), map[string]any{"name": "deploy"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Content)
	}
	if result.Content == "" {
		t.Error("expected non-empty content")
	}
	if len(result.NewMessages) != 1 {
		t.Fatalf("expected 1 NewMessage, got %d", len(result.NewMessages))
	}
	if !strings.Contains(result.NewMessages[0].Content, "Deploy Steps") {
		t.Errorf("NewMessage missing skill body content: %q", result.NewMessages[0].Content)
	}
}

func TestInvokeSkillTool_Execute_NotFound(t *testing.T) {
	dir := t.TempDir()
	tool := NewInvokeSkillTool(dir)
	result, err := tool.Execute(context.Background(), map[string]any{"name": "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing skill")
	}
}

func TestInvokeSkillTool_Execute_MissingName(t *testing.T) {
	tool := NewInvokeSkillTool("/tmp")
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing name")
	}
}
