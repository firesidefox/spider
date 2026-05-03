package agent

import (
	"testing"
)

func TestHookChain_SafeTool(t *testing.T) {
	chain := NewHookChain()
	chain.AddBefore(DefaultRiskHook())

	result := chain.RunBefore("ping", nil, RiskL1)
	if result.Action != HookAllow {
		t.Errorf("expected HookAllow, got %q", result.Action)
	}
}

func TestHookChain_ModerateTool(t *testing.T) {
	chain := NewHookChain()
	chain.AddBefore(DefaultRiskHook())

	result := chain.RunBefore("restart", nil, RiskL2)
	if result.Action != HookRequireConfirm {
		t.Errorf("expected HookRequireConfirm, got %q", result.Action)
	}
}

func TestHookChain_DangerousTool(t *testing.T) {
	chain := NewHookChain()
	chain.AddBefore(DefaultRiskHook())

	result := chain.RunBefore("delete_all", nil, RiskL3)
	if result.Action != HookRequireConfirm {
		t.Errorf("expected HookRequireConfirm, got %q", result.Action)
	}
}

func TestHookChain_FirstDenyWins(t *testing.T) {
	chain := NewHookChain()
	chain.AddBefore(func(_ string, _ map[string]any, _ RiskLevel) *HookResult {
		return &HookResult{Action: HookDeny, Reason: "blocked by policy"}
	})
	chain.AddBefore(func(_ string, _ map[string]any, _ RiskLevel) *HookResult {
		return &HookResult{Action: HookAllow}
	})

	result := chain.RunBefore("any_tool", nil, RiskL1)
	if result.Action != HookDeny {
		t.Errorf("expected HookDeny, got %q", result.Action)
	}
	if result.Reason != "blocked by policy" {
		t.Errorf("expected reason %q, got %q", "blocked by policy", result.Reason)
	}
}

func TestHookChain_AfterHooksAllRun(t *testing.T) {
	chain := NewHookChain()
	count := 0
	chain.AddAfter(func(_ string, _ map[string]any, _ *ToolResult) { count++ })
	chain.AddAfter(func(_ string, _ map[string]any, _ *ToolResult) { count++ })
	chain.AddAfter(func(_ string, _ map[string]any, _ *ToolResult) { count++ })

	tr := &ToolResult{Content: "ok", RiskLevel: RiskL1}
	chain.RunAfter("ping", nil, tr)

	if count != 3 {
		t.Errorf("expected 3 after hooks to run, got %d", count)
	}
}
