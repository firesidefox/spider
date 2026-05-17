package agent

import (
	"context"
	"fmt"
)

type InvokeSkillTool struct {
	manager *SkillManager
}

func NewInvokeSkillTool(dataDir string) *InvokeSkillTool {
	return &InvokeSkillTool{manager: NewSkillManager(dataDir)}
}

func (t *InvokeSkillTool) Name() string                { return "invoke_skill" }
func (t *InvokeSkillTool) DefaultRiskLevel() RiskLevel { return RiskL1 }

func (t *InvokeSkillTool) Description() string {
	return `Load a skill's full instructions into context. Call this ONCE per skill per turn when you need to execute a skill listed in <skills>. If <loaded-skill name=X> is already present in this conversation, do NOT call invoke_skill("X") again — use the loaded instructions directly.`
}

func (t *InvokeSkillTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill name exactly as listed in <skills>",
			},
		},
	}
}

func (t *InvokeSkillTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return &ToolResult{Content: "missing required field: name", IsError: true, RiskLevel: RiskL1, Summary: "failed to load skill: missing name"}, nil
	}

	entry, err := t.manager.LookupSkill(name)
	if err != nil {
		return &ToolResult{
			Content:   fmt.Sprintf("skill %q not found", name),
			IsError:   true,
			RiskLevel: RiskL1,
			Summary:   fmt.Sprintf("failed to load skill %q", name),
		}, nil
	}

	body, _ := entry.Body()
	return &ToolResult{
		Content:   fmt.Sprintf("skill %q loaded", name),
		RiskLevel: RiskL1,
		Summary:   fmt.Sprintf("skill %q loaded", name),
		NewMessages: []InjectMessage{
			{Content: fmt.Sprintf("<loaded-skill name=%q>\n%s\n</loaded-skill>", name, body)},
		},
	}, nil
}
