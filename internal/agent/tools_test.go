package agent

import (
	"context"
	"testing"
)

type mockTool struct {
	name        string
	description string
	schema      map[string]any
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return m.description }
func (m *mockTool) InputSchema() map[string]any { return m.schema }
func (m *mockTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	return &ToolResult{Content: "ok", RiskLevel: RiskSafe}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewToolRegistry()
	tool := &mockTool{name: "ping", description: "ping tool", schema: map[string]any{}}
	r.Register(tool)

	got, ok := r.Get("ping")
	if !ok {
		t.Fatal("expected to find registered tool")
	}
	if got.Name() != "ping" {
		t.Errorf("got name %q, want %q", got.Name(), "ping")
	}

	_, ok = r.Get("missing")
	if ok {
		t.Fatal("expected not to find unregistered tool")
	}
}

func TestRegistry_Definitions(t *testing.T) {
	r := NewToolRegistry()
	schema := map[string]any{"type": "object"}
	r.Register(&mockTool{name: "tool1", description: "first", schema: schema})

	defs := r.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	d := defs[0]
	if d.Name != "tool1" {
		t.Errorf("got name %q, want %q", d.Name, "tool1")
	}
	if d.Description != "first" {
		t.Errorf("got description %q, want %q", d.Description, "first")
	}
}

func TestRegistry_MultipleTools(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&mockTool{name: "a", description: "tool a", schema: nil})
	r.Register(&mockTool{name: "b", description: "tool b", schema: nil})
	r.Register(&mockTool{name: "c", description: "tool c", schema: nil})

	defs := r.Definitions()
	if len(defs) != 3 {
		t.Fatalf("expected 3 definitions, got %d", len(defs))
	}

	for _, name := range []string{"a", "b", "c"} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("expected to find tool %q", name)
		}
	}
}
