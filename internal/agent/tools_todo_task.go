package agent

import (
	"context"
	"encoding/json"

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
	turnTaskIDs    []int64
}

func NewTodoTool(s *store.TodoStore, broadcaster SSEBroadcaster, conversationID string) *TodoTool {
	return &TodoTool{store: s, broadcaster: broadcaster, conversationID: conversationID}
}

func (t *TodoTool) Name() string                { return "Todo" }
func (t *TodoTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *TodoTool) Hidden() bool                { return true }

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
			"active_form": map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"status":      map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "deleted"}},
			"owner":       map[string]any{"type": "string"},
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
		Subject:        subject,
		ActiveForm:     strVal(input, "active_form"),
		Description:    strVal(input, "description"),
		Status:         "pending",
		Owner:          strVal(input, "owner"),
	}
	if err := t.store.Create(task); err != nil {
		return &ToolResult{Content: "create failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	t.turnTaskIDs = append(t.turnTaskIDs, task.ID)
	t.broadcast(task)
	out, _ := json.Marshal(task)
	return &ToolResult{Content: string(out), Nudge: todoNudge(false), RiskLevel: RiskL1}, nil
}

func (t *TodoTool) execUpdate(input map[string]any) (*ToolResult, error) {
	taskIDFloat, ok := input["task_id"].(float64)
	if !ok {
		return &ToolResult{Content: "update requires task_id", IsError: true, RiskLevel: RiskL1}, nil
	}
	taskID := int64(taskIDFloat)

	subject := strVal(input, "subject")
	activeForm := strVal(input, "active_form")
	description := strVal(input, "description")
	status := strVal(input, "status")
	owner := strVal(input, "owner")

	if subject == "" && activeForm == "" && description == "" && status == "" && owner == "" {
		return &ToolResult{Content: "update requires at least one field besides task_id", IsError: true, RiskLevel: RiskL1}, nil
	}

	task, err := t.store.Update(t.conversationID, taskID, subject, activeForm, description, status, owner)
	if err != nil {
		return &ToolResult{Content: "update failed: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	t.broadcast(task)
	out, _ := json.Marshal(task)

	allDone := false
	if status == "completed" || status == "deleted" {
		if tasks, err := t.store.GetByIDs(t.turnTaskIDs); err == nil && len(tasks) > 0 {
			allDone = tasksDone(tasks)
		}
	}
	return &ToolResult{Content: string(out), Nudge: todoNudge(allDone), RiskLevel: RiskL1}, nil
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

func tasksDone(tasks []*models.Todo) bool {
	for _, task := range tasks {
		if task.Status != "completed" && task.Status != "deleted" {
			return false
		}
	}
	return true
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
- Provide active_form (present-continuous) when creating tasks, e.g. subject="Update TodoStore" active_form="Updating TodoStore"

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

