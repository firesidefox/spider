package agent

import (
	"context"

	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/permission"
)

type RiskLevel = permission.RiskLevel

const (
	RiskL1 = permission.L1Read
	RiskL2 = permission.L2Write
	RiskL3 = permission.L3Dangerous
	RiskL4 = permission.L4Destroy
)

type InjectMessage struct {
	Content string
}

type ToolResult struct {
	Content     string          `json:"content"`
	IsError     bool            `json:"is_error"`
	RiskLevel   RiskLevel       `json:"risk_level"`
	NewMessages []InjectMessage `json:"-"`
}

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	DefaultRiskLevel() RiskLevel
	Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
}

// SystemPromptSection is implemented by tools that contribute behavior
// guidance to the system prompt. BuildSystemPrompt collects sections from
// all registered tools in registration order and appends them as Layer 2.
type SystemPromptSection interface {
	SystemPromptSection() string
}

type ToolRegistry struct {
	tools []Tool
	index map[string]int
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{index: make(map[string]int)}
}

func (r *ToolRegistry) Register(t Tool) {
	r.index[t.Name()] = len(r.tools)
	r.tools = append(r.tools, t)
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	i, ok := r.index[name]
	if !ok {
		return nil, false
	}
	return r.tools[i], true
}

func (r *ToolRegistry) All() []Tool {
	return r.tools
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
