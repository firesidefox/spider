package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type SSEBroadcaster interface {
	BroadcastSSE(conversationID string, data []byte)
}

type TodoTool struct {
	store          *store.TodoStore
	broadcaster    SSEBroadcaster
	conversationID string
	turnID         string
}

func NewTodoTool(s *store.TodoStore, broadcaster SSEBroadcaster, conversationID, turnID string) *TodoTool {
	return &TodoTool{store: s, broadcaster: broadcaster, conversationID: conversationID, turnID: turnID}
}

func (t *TodoTool) Name() string                    { return "Todo" }
func (t *TodoTool) DefaultRiskLevel() RiskLevel     { return RiskL1 }
func (t *TodoTool) Hidden() bool                    { return true }

func (t *TodoTool) Description() string {
	return "Update the todo task list for the current conversation. Use proactively to track progress on complex tasks. Actions: create (required: subject), update (required: task_id + at least one field), list. Status values: pending, in_progress, completed, deleted."
}

func (t *TodoTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"action"},
		"properties": map[string]any{
			"action":      map[string]any{"type": "string", "enum": []string{"create", "update", "list"}},
			"subject":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"status":      map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "deleted"}},
			"owner":       map[string]any{"type": "string"},
			"blocked_by":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"task_id":     map[string]any{"type": "integer"},
		},
	}
}

func (t *TodoTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	action, _ := input["action"].(string)
	switch action {
	case "create":
		return t.execCreate(input)
	case "update":
		return t.execUpdate(input)
	case "list":
		return t.execList()
	default:
		return &ToolResult{Content: "unknown action: " + action, IsError: true, RiskLevel: RiskL1}, nil
	}
}

func (t *TodoTool) execCreate(input map[string]any) (*ToolResult, error) {
	subject, _ := input["subject"].(string)
	if subject == "" {
		return &ToolResult{Content: "create requires subject", IsError: true, RiskLevel: RiskL1}, nil
	}
	task := &models.Todo{
		ConversationID: t.conversationID,
		TurnID:         t.turnID,
		Subject:        subject,
		Description:    strVal(input, "description"),
		Status:         "pending",
		Owner:          strVal(input, "owner"),
		BlockedBy:      int64Slice(input, "blocked_by"),
	}
	if err := t.store.Create(task); err != nil {
		return &ToolResult{Content: "create failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	t.broadcast(task)
	out, _ := json.Marshal(task)
	return &ToolResult{Content: string(out) + todoNudge(false), RiskLevel: RiskL1}, nil
}

func (t *TodoTool) execUpdate(input map[string]any) (*ToolResult, error) {
	taskIDFloat, ok := input["task_id"].(float64)
	if !ok {
		return &ToolResult{Content: "update requires task_id", IsError: true, RiskLevel: RiskL1}, nil
	}
	taskID := int64(taskIDFloat)

	subject := strVal(input, "subject")
	description := strVal(input, "description")
	status := strVal(input, "status")
	owner := strVal(input, "owner")
	var blockedBy []int64
	if _, has := input["blocked_by"]; has {
		blockedBy = int64Slice(input, "blocked_by")
		if blockedBy == nil {
			blockedBy = []int64{}
		}
	}

	if subject == "" && description == "" && status == "" && owner == "" && blockedBy == nil {
		return &ToolResult{Content: "update requires at least one field besides task_id", IsError: true, RiskLevel: RiskL1}, nil
	}

	task, err := t.store.Update(t.conversationID, taskID, subject, description, status, owner, blockedBy)
	if err != nil {
		return &ToolResult{Content: "update failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	t.broadcast(task)
	out, _ := json.Marshal(task)

	tasks, allDone := t.allTasksDone()
	if allDone {
		t.broadcastSummary(tasks)
	}
	return &ToolResult{Content: string(out) + todoNudge(allDone), RiskLevel: RiskL1}, nil
}

func (t *TodoTool) execList() (*ToolResult, error) {
	tasks, err := t.store.List(t.conversationID)
	if err != nil {
		return &ToolResult{Content: "list failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	if tasks == nil {
		tasks = []*models.Todo{}
	}
	out, _ := json.Marshal(tasks)
	return &ToolResult{Content: string(out), RiskLevel: RiskL1}, nil
}

func (t *TodoTool) broadcast(task *models.Todo) {
	t.broadcastEvent("todo_update", task)
}

func (t *TodoTool) broadcastEvent(eventType string, content any) {
	if t.broadcaster == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{"type": eventType, "content": content})
	t.broadcaster.BroadcastSSE(t.conversationID, payload)
}

func (t *TodoTool) allTasksDone() ([]*models.Todo, bool) {
	tasks, err := t.store.ListByTurn(t.turnID)
	if err != nil {
		logger.Global().Error().Err(err).Str("turn_id", t.turnID).Msg("allTasksDone: ListByTurn failed")
		return nil, false
	}
	if len(tasks) == 0 {
		return nil, false
	}
	for _, task := range tasks {
		if task.Status != "completed" && task.Status != "deleted" {
			return nil, false
		}
	}
	return tasks, true
}

func (t *TodoTool) broadcastSummary(tasks []*models.Todo) {
	var sb strings.Builder
	sb.WriteString("**Tasks completed:**\n")
	n := 0
	for _, task := range tasks {
		n++
		dur := task.UpdatedAt.Sub(task.CreatedAt).Round(time.Second)
		fmt.Fprintf(&sb, "%d. %s (%s)\n", n, task.Subject, dur)
	}
	t.broadcastEvent("todo_summary", sb.String())
}

const todoBaseNudge = "\n\nTodo list updated. Continue using the Todo tool to track remaining work — mark each task in_progress before starting and completed immediately when done."
const todoAllDoneNudge = "\n\nAll tasks are complete. Before finishing, verify your work by producing a concrete artifact (test output, build log, or command result) that confirms the changes are correct. Do not self-assess — let the output speak."
const execNudge = "\n\nCommand executed. Update your todo list if this completes a task, then verify the result before proceeding to the next step."
const apiMutateNudge = "\n\nAPI call completed. Check status_code in the response. Update your todo list if this completes a task."

func todoNudge(allDone bool) string {
	if allDone {
		return todoAllDoneNudge
	}
	return todoBaseNudge
}

func strVal(input map[string]any, key string) string {
	v, _ := input[key].(string)
	return v
}

const todoPromptSection = `## Task Management (Todo tool)

Use the Todo tool proactively to track progress on complex tasks.

**When to use:**
- Task requires 3 or more distinct steps
- User provides multiple tasks to complete

**When NOT to use:**
- Single, straightforward task
- Purely conversational or informational response

**Rules:**
- Mark a task in_progress BEFORE beginning work on it
- Only ONE task in_progress at a time
- Mark completed IMMEDIATELY after finishing — do not batch completions
- Only mark completed when fully done; if blocked, keep in_progress and create a new task describing the blocker

<example>
User: Check disk usage on all web servers, clean up logs older than 30 days, and restart nginx if free space is below 20%.
Assistant: Creates tasks: 1) Check disk usage 2) Clean up logs 3) Restart nginx if space < 20%
</example>

<example>
User: What is the IP address of host web-01?
Assistant: Calls ListHosts directly. No todo list.
</example>`

func (t *TodoTool) SystemPromptSection() string {
	return todoPromptSection
}

func int64Slice(input map[string]any, key string) []int64 {
	raw, ok := input[key].([]any)
	if !ok {
		return nil
	}
	out := make([]int64, 0, len(raw))
	for _, v := range raw {
		if f, ok := v.(float64); ok {
			out = append(out, int64(f))
		}
	}
	return out
}
