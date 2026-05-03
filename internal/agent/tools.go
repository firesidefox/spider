package agent

import (
	"context"

	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/permission"
)

// RiskLevel is an alias for permission.RiskLevel, unifying the agent tool
// risk system with the L1-L4 permission levels.
type RiskLevel = permission.RiskLevel

const (
	RiskL1 = permission.L1Read
	RiskL2 = permission.L2Write
	RiskL3 = permission.L3Dangerous
	RiskL4 = permission.L4Destroy
)

type ToolResult struct {
	Content   string    `json:"content"`
	IsError   bool      `json:"is_error"`
	RiskLevel RiskLevel `json:"risk_level"`
}

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	DefaultRiskLevel() RiskLevel
	Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) Definitions() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}
