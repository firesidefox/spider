package agent

import (
	"context"
	"fmt"
	"path/filepath"
)

type InvokeSkillTool struct {
	skillsDir string
}

func NewInvokeSkillTool(skillsDir string) *InvokeSkillTool {
	return &InvokeSkillTool{skillsDir: skillsDir}
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
		return &ToolResult{Content: "missing required field: name", IsError: true, RiskLevel: RiskL1}, nil
	}

	mdPath := filepath.Join(t.skillsDir, filepath.FromSlash(name), "SKILL.md")
	entry := SkillEntry{Name: name, bodyPath: mdPath}
	body, err := entry.Body()
	if err != nil {
		return &ToolResult{
			Content:   fmt.Sprintf("skill %q not found", name),
			IsError:   true,
			RiskLevel: RiskL1,
		}, nil
	}

	return &ToolResult{
		Content:   fmt.Sprintf("skill %q loaded", name),
		RiskLevel: RiskL1,
		NewMessages: []InjectMessage{
			{Content: fmt.Sprintf("<loaded-skill name=%q>\n%s\n</loaded-skill>", name, body)},
		},
	}, nil
}
