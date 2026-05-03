package permission

// Enforcer decides whether to allow, deny, pause, or plan a command
// based on the current permission mode and risk level.
type Enforcer struct{}

// NewEnforcer creates a new Enforcer.
func NewEnforcer() *Enforcer {
	return &Enforcer{}
}

// Decide returns the execution decision for the given mode and risk level.
// Unknown modes fall back to ask behavior (conservative default).
func (e *Enforcer) Decide(mode Mode, level RiskLevel) Decision {
	switch mode {
	case ModeReadonly:
		if level == L1Read {
			return DecisionAllow
		}
		return DecisionDeny
	case ModeAsk:
		if level <= L2Write {
			return DecisionAllow
		}
		return DecisionPending
	case ModeAuto:
		if level <= L3Dangerous {
			return DecisionAllow
		}
		return DecisionPending
	case ModePlan:
		return DecisionPlan
	default:
		// Unknown mode → ask behavior (conservative)
		if level <= L2Write {
			return DecisionAllow
		}
		return DecisionPending
	}
}
