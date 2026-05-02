package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBatchExecuteTool_Interface(t *testing.T) {
	tool := NewBatchExecuteTool(nil, nil, nil, nil)
	if tool.Name() != "batch_execute" {
		t.Errorf("got name %q, want batch_execute", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Error("schema type should be object")
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"host_ids", "tag", "command"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
}

func TestVerifyTool_Interface(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil)
	if tool.Name() != "verify" {
		t.Errorf("got name %q, want verify", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Error("schema type should be object")
	}
}

func TestParseChecks_Valid(t *testing.T) {
	input := map[string]any{
		"checks": []any{
			map[string]any{"host_id": "h1", "command": "uptime", "expect": "load"},
			map[string]any{"host_id": "h2", "command": "echo ok", "expect": "ok"},
		},
	}
	checks, err := parseChecks(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}
	if checks[0].HostID != "h1" || checks[0].Command != "uptime" || checks[0].Expect != "load" {
		t.Errorf("check[0] mismatch: %+v", checks[0])
	}
}

func TestParseChecks_Invalid(t *testing.T) {
	_, err := parseChecks(map[string]any{"checks": "not-an-array"})
	if err == nil {
		t.Error("expected error for non-array checks")
	}
}

func TestVerifyTool_Execute_NoChecks(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil)
	res, err := tool.Execute(context.Background(), map[string]any{
		"checks": "bad",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for bad checks input")
	}
}

func TestBatchExecuteTool_Execute_NoCommand(t *testing.T) {
	tool := NewBatchExecuteTool(nil, nil, nil, nil)
	res, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true when command is missing")
	}
}

func TestBatchExecuteTool_Execute_NoHosts(t *testing.T) {
	tool := NewBatchExecuteTool(nil, nil, nil, nil)
	res, err := tool.Execute(context.Background(), map[string]any{
		"command":  "echo hi",
		"host_ids": []any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true when no hosts selected")
	}
}

func TestVerifyTool_InputSchema_Checks(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil)
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"checks", "timeout", "interval"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
}

func TestVerifyTool_ToolResultIsJSON(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil)
	res, _ := tool.Execute(context.Background(), map[string]any{"checks": "bad"})
	if res == nil {
		t.Fatal("nil result")
	}
	_ = res.Content
}

func TestBatchExecuteTool_RiskLevel(t *testing.T) {
	tool := NewBatchExecuteTool(nil, nil, nil, nil)
	res, _ := tool.Execute(context.Background(), map[string]any{})
	if res.RiskLevel != RiskDangerous {
		t.Errorf("expected RiskDangerous, got %q", res.RiskLevel)
	}
}

func TestVerifyTool_RiskLevel(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil)
	res, _ := tool.Execute(context.Background(), map[string]any{"checks": "bad"})
	if res.RiskLevel != RiskSafe {
		t.Errorf("expected RiskSafe, got %q", res.RiskLevel)
	}
}

func TestVerifyTool_Timeout_EmptyChecks(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil)
	res, err := tool.Execute(context.Background(), map[string]any{
		"checks":   []any{},
		"timeout":  1,
		"interval": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if jsonErr := json.Unmarshal([]byte(res.Content), &out); jsonErr != nil {
		t.Fatalf("result not valid JSON: %v", jsonErr)
	}
	if out["status"] != "ok" {
		t.Errorf("expected status ok for empty checks, got %v", out["status"])
	}
}
