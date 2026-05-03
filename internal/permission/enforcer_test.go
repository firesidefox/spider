package permission

import "testing"

func TestEnforcer_Decide(t *testing.T) {
	e := NewEnforcer()
	tests := []struct {
		mode  Mode
		level RiskLevel
		want  Decision
	}{
		// readonly
		{ModeReadonly, L1Read, DecisionAllow},
		{ModeReadonly, L2Write, DecisionDeny},
		{ModeReadonly, L3Dangerous, DecisionDeny},
		{ModeReadonly, L4Destroy, DecisionDeny},
		// ask
		{ModeAsk, L1Read, DecisionAllow},
		{ModeAsk, L2Write, DecisionAllow},
		{ModeAsk, L3Dangerous, DecisionPending},
		{ModeAsk, L4Destroy, DecisionPending},
		// auto
		{ModeAuto, L1Read, DecisionAllow},
		{ModeAuto, L2Write, DecisionAllow},
		{ModeAuto, L3Dangerous, DecisionAllow},
		{ModeAuto, L4Destroy, DecisionPending},
		// plan
		{ModePlan, L1Read, DecisionPlan},
		{ModePlan, L2Write, DecisionPlan},
		{ModePlan, L3Dangerous, DecisionPlan},
		{ModePlan, L4Destroy, DecisionPlan},
	}
	for _, tt := range tests {
		got := e.Decide(tt.mode, tt.level)
		if got != tt.want {
			t.Errorf("Decide(%s, L%d) = %s, want %s", tt.mode, tt.level, got, tt.want)
		}
	}
}

func TestEnforcer_Decide_UnknownMode(t *testing.T) {
	e := NewEnforcer()
	// unknown mode falls back to ask behavior
	if got := e.Decide("unknown", L1Read); got != DecisionAllow {
		t.Errorf("unknown mode L1 = %s, want Allow", got)
	}
	if got := e.Decide("unknown", L3Dangerous); got != DecisionPending {
		t.Errorf("unknown mode L3 = %s, want Pending", got)
	}
}
