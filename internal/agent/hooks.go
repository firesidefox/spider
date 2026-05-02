package agent

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

func DefaultRiskHook() BeforeToolHook {
	return func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
		switch riskLevel {
		case RiskSafe:
			return &HookResult{Action: HookAllow, RiskLevel: riskLevel}
		case RiskModerate, RiskDangerous:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: riskLevel}
		default:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: RiskModerate, Reason: "unknown risk level"}
		}
	}
}
