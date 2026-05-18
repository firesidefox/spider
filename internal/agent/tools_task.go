package agent

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// CreateTaskTool saves a confirmed automated task to the database.
type CreateTaskTool struct {
	taskStore      *store.TaskStore
	conversationID string
}

// NewCreateTaskTool creates a new CreateTaskTool.
func NewCreateTaskTool(taskStore *store.TaskStore, conversationID string) *CreateTaskTool {
	return &CreateTaskTool{taskStore: taskStore, conversationID: conversationID}
}

// Name returns the tool name.
func (t *CreateTaskTool) Name() string { return "CreateTask" }

// DefaultRiskLevel returns L2 because this tool writes to the database.
func (t *CreateTaskTool) DefaultRiskLevel() RiskLevel              { return RiskL2 }
func (t *CreateTaskTool) IsConcurrencySafe(_ map[string]any) bool { return false }

// Description returns the tool description.
func (t *CreateTaskTool) Description() string {
	return "Save a confirmed automated task to the database. Has side effects."
}

const createTaskPrompt = `## CreateTask

**When to use:** Call only after the user has explicitly confirmed all task fields (name, goal, host_ids, schedule, notify_mode).

**When NOT to use:**
- Do not call speculatively or before user confirmation
- Do not call if any required field is unclear

**Rules:**
- Extract fields from conversation context, present them to the user, wait for confirmation, then call
- schedule is a 5-field cron expression (e.g. "0 2 * * 3") or empty string for manual-only tasks
- notify_mode: "none" | "failure" | "complete" | "anomaly" (default: "none")`

// SystemPromptSection returns the system prompt section for this tool.
func (t *CreateTaskTool) SystemPromptSection() string {
	return createTaskPrompt
}

// Execute creates the task and returns a confirmation string.
func (t *CreateTaskTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	name, _ := input["name"].(string)
	goal, _ := input["goal"].(string)

	if name == "" {
		return &ToolResult{Content: "name is required", IsError: true, RiskLevel: RiskL2}, nil
	}
	if goal == "" {
		return &ToolResult{Content: "goal is required", IsError: true, RiskLevel: RiskL2}, nil
	}

	var hostIDs []string
	if raw, ok := input["host_ids"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				hostIDs = append(hostIDs, s)
			}
		}
	}

	if len(hostIDs) == 0 {
		return &ToolResult{Content: "host_ids must not be empty", IsError: true, RiskLevel: RiskL2}, nil
	}

	notifyMode, _ := input["notify_mode"].(string)
	if notifyMode == "" {
		notifyMode = string(models.NotifyNone)
	}

	runRetentionDays := intVal(input, "run_retention_days")
	if runRetentionDays == 0 {
		runRetentionDays = 30
	}
	timeoutMinutes := intVal(input, "timeout_minutes")
	if timeoutMinutes == 0 {
		timeoutMinutes = 30
	}

	schedule := strVal(input, "schedule")
	if schedule != "" {
		if _, err := cron.ParseStandard(schedule); err != nil {
			return &ToolResult{Content: fmt.Sprintf("invalid cron expression %q: %v", schedule, err), IsError: true, RiskLevel: RiskL2}, nil
		}
	}

	task := &models.Task{
		Name:             name,
		Goal:             goal,
		HostIDs:          hostIDs,
		Schedule:         schedule,
		NotifyMode:       models.NotifyMode(notifyMode),
		RunRetentionDays: runRetentionDays,
		TimeoutMinutes:   timeoutMinutes,
		Status:           models.TaskStatusActive,
		SourceConvID:     t.conversationID,
	}

	created, err := t.taskStore.Create(task)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("failed to create task: %v", err), IsError: true, RiskLevel: RiskL2}, nil
	}
	return &ToolResult{
		Content:   fmt.Sprintf("Task created: ID=%s, Name=%s", created.ID, created.Name),
		RiskLevel: RiskL2,
	}, nil
}

// InputSchema returns the JSON schema for the tool input.
func (t *CreateTaskTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name", "goal", "host_ids"},
		"properties": map[string]any{
			"name":               map[string]any{"type": "string", "description": "Task name"},
			"goal":               map[string]any{"type": "string", "description": "Natural language goal"},
			"host_ids":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Target device IDs"},
			"schedule":           map[string]any{"type": "string", "description": "Cron expression (empty = manual only)"},
			"notify_mode":        map[string]any{"type": "string", "description": "none|failure|complete|anomaly"},
			"run_retention_days": map[string]any{"type": "integer", "description": "TaskRun retention days, default 30"},
			"timeout_minutes":    map[string]any{"type": "integer", "description": "Execution timeout minutes, default 30"},
		},
	}
}

func intVal(input map[string]any, key string) int {
	if f, ok := input[key].(float64); ok {
		return int(f)
	}
	return 0
}
