package agent

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/llm"
)

type concurrencyMockTool struct {
	name string
	safe bool
}

func (t *concurrencyMockTool) Name() string                            { return t.name }
func (t *concurrencyMockTool) Description() string                     { return t.name }
func (t *concurrencyMockTool) InputSchema() map[string]any             { return map[string]any{} }
func (t *concurrencyMockTool) DefaultRiskLevel() RiskLevel             { return RiskL1 }
func (t *concurrencyMockTool) IsConcurrencySafe(_ map[string]any) bool { return t.safe }
func (t *concurrencyMockTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	return &ToolResult{Content: "ok", RiskLevel: RiskL1}, nil
}

func TestPartitionToolCalls_AllSafe(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&concurrencyMockTool{name: "a", safe: true})
	reg.Register(&concurrencyMockTool{name: "b", safe: true})

	calls := []llm.ToolCall{
		{ID: "1", Name: "a", Input: map[string]any{}},
		{ID: "2", Name: "b", Input: map[string]any{}},
		{ID: "3", Name: "a", Input: map[string]any{}},
	}
	batches := partitionToolCalls(calls, reg)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if !batches[0].concurrent {
		t.Error("expected batch to be concurrent")
	}
	if len(batches[0].calls) != 3 {
		t.Errorf("expected 3 calls in batch, got %d", len(batches[0].calls))
	}
}

func TestPartitionToolCalls_AllUnsafe(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&concurrencyMockTool{name: "x", safe: false})

	calls := []llm.ToolCall{
		{ID: "1", Name: "x", Input: map[string]any{}},
		{ID: "2", Name: "x", Input: map[string]any{}},
	}
	batches := partitionToolCalls(calls, reg)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	for i, b := range batches {
		if b.concurrent {
			t.Errorf("batch %d should not be concurrent", i)
		}
	}
}

func TestPartitionToolCalls_Mixed(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&concurrencyMockTool{name: "safe", safe: true})
	reg.Register(&concurrencyMockTool{name: "unsafe", safe: false})

	calls := []llm.ToolCall{
		{ID: "1", Name: "safe", Input: map[string]any{}},
		{ID: "2", Name: "safe", Input: map[string]any{}},
		{ID: "3", Name: "unsafe", Input: map[string]any{}},
		{ID: "4", Name: "safe", Input: map[string]any{}},
	}
	batches := partitionToolCalls(calls, reg)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	if !batches[0].concurrent || len(batches[0].calls) != 2 {
		t.Errorf("batch 0: want concurrent with 2 calls, got concurrent=%v len=%d", batches[0].concurrent, len(batches[0].calls))
	}
	if batches[1].concurrent || len(batches[1].calls) != 1 {
		t.Errorf("batch 1: want serial with 1 call, got concurrent=%v len=%d", batches[1].concurrent, len(batches[1].calls))
	}
	if !batches[2].concurrent || len(batches[2].calls) != 1 {
		t.Errorf("batch 2: want concurrent with 1 call, got concurrent=%v len=%d", batches[2].concurrent, len(batches[2].calls))
	}
}

func TestPartitionToolCalls_UnknownTool(t *testing.T) {
	reg := NewToolRegistry()
	calls := []llm.ToolCall{
		{ID: "1", Name: "missing", Input: map[string]any{}},
	}
	batches := partitionToolCalls(calls, reg)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if batches[0].concurrent {
		t.Error("unknown tool should not be concurrent")
	}
}
