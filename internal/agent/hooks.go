package agent

import "github.com/spiderai/spider/internal/permission"

type HookAction string

const (
	HookAllow          HookAction = "allow"
	HookRequireConfirm HookAction = "require_confirm"
	HookDeny           HookAction = "deny"
)

type HookResult struct {
	Action    HookAction
	RiskLevel RiskLevel
	Reason    string
}

type BeforeToolHook func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult
type AfterToolHook func(toolName string, input map[string]any, result *ToolResult)

type HookChain struct {
	before []BeforeToolHook
	after  []AfterToolHook
}

func NewHookChain() *HookChain {
	return &HookChain{}
}

func (h *HookChain) AddBefore(hook BeforeToolHook) {
	h.before = append(h.before, hook)
}

func (h *HookChain) AddAfter(hook AfterToolHook) {
	h.after = append(h.after, hook)
}

func (h *HookChain) RunBefore(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
	for _, hook := range h.before {
		if r := hook(toolName, input, riskLevel); r.Action != HookAllow {
			return r
		}
	}
	return &HookResult{Action: HookAllow}
}

func (h *HookChain) RunAfter(toolName string, input map[string]any, result *ToolResult) {
	for _, hook := range h.after {
		hook(toolName, input, result)
	}
}

// DefaultRiskHook is the fallback hook when no Enforcer is available.
// L1 is auto-allowed; L2+ requires confirmation.
func DefaultRiskHook() BeforeToolHook {
	return func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
		switch riskLevel {
		case RiskL1:
			return &HookResult{Action: HookAllow, RiskLevel: riskLevel}
		case RiskL2, RiskL3, RiskL4:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: riskLevel}
		default:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: RiskL2, Reason: "unknown risk level"}
		}
	}
}

// PermissionHook delegates tool execution decisions to the permission Enforcer.
func PermissionHook(enforcer *permission.Enforcer, mode permission.Mode) BeforeToolHook {
	return func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
		decision := enforcer.Decide(mode, riskLevel)
		switch decision {
		case permission.DecisionAllow:
			return &HookResult{Action: HookAllow, RiskLevel: riskLevel}
		case permission.DecisionPending:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: riskLevel}
		case permission.DecisionDeny:
			return &HookResult{Action: HookDeny, RiskLevel: riskLevel, Reason: "denied by permission mode"}
		case permission.DecisionPlan:
			return &HookResult{Action: HookDeny, RiskLevel: riskLevel, Reason: "plan mode: execution not allowed"}
		default:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: riskLevel}
		}
	}
}
