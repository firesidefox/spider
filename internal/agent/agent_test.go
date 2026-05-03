package agent

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
)

type mockLLMClient struct {
	responses [][]llm.StreamEvent
	callIdx   int
}

func (m *mockLLMClient) ChatStream(_ context.Context, _ *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 32)
	events := m.responses[m.callIdx]
	m.callIdx++
	go func() {
		defer close(ch)
		for _, e := range events {
			ch <- e
		}
	}()
	return ch, nil
}

type mockMsgStore struct {
	messages []struct{ convID, role, content, toolCalls string }
}

func (m *mockMsgStore) Save(convID, role, content, toolCalls string) error {
	m.messages = append(m.messages, struct{ convID, role, content, toolCalls string }{convID, role, content, toolCalls})
	return nil
}

func (m *mockMsgStore) ListByConversation(convID string) ([]*models.Message, error) {
	var out []*models.Message
	for _, msg := range m.messages {
		if msg.convID == convID {
			out = append(out, &models.Message{
				ConversationID: msg.convID,
				Role:           msg.role,
				Content:        msg.content,
			})
		}
	}
	return out, nil
}

type mockResultTool struct {
	name   string
	result *ToolResult
}

func (t *mockResultTool) Name() string                    { return t.name }
func (t *mockResultTool) Description() string             { return t.name }
func (t *mockResultTool) InputSchema() map[string]any     { return map[string]any{} }
func (t *mockResultTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	return t.result, nil
}

func newTestAgent(client llm.Client, store *mockMsgStore, reg *ToolRegistry) *Agent {
	return NewAgent(AgentConfig{
		LLMClient: client,
		Registry:  reg,
		Hooks:     NewHookChain(),
		MsgStore:  store,
		MaxTurns:  3,
	})
}

func collectEvents(ch <-chan Event) []Event {
	var out []Event
	for e := range ch {
		out = append(out, e)
	}
	return out
}

func TestSimpleTextResponse(t *testing.T) {
	client := &mockLLMClient{
		responses: [][]llm.StreamEvent{
			{
				{Type: "text_delta", Text: "Hello"},
				{Type: "text_delta", Text: " world"},
				{Type: "message_stop"},
			},
		},
	}
	store := &mockMsgStore{}
	agent := newTestAgent(client, store, NewToolRegistry())

	ch, err := agent.Run(context.Background(), "conv1", "hi", nil)
	if err != nil {
		t.Fatal(err)
	}
	events := collectEvents(ch)

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}
	if events[0].Type != EventTextDelta {
		t.Errorf("expected text_delta, got %s", events[0].Type)
	}
	last := events[len(events)-1]
	if last.Type != EventDone {
		t.Errorf("expected done, got %s", last.Type)
	}
}

func TestToolCallResponse(t *testing.T) {
	toolCall := &llm.ToolCall{ID: "tc1", Name: "echo"}
	client := &mockLLMClient{
		responses: [][]llm.StreamEvent{
			{
				{Type: "tool_start", ToolCall: toolCall},
				{Type: "tool_input_delta", Text: `{"msg":"hi"}`},
				{Type: "message_stop"},
			},
			{
				{Type: "text_delta", Text: "done"},
				{Type: "message_stop"},
			},
		},
	}
	store := &mockMsgStore{}
	reg := NewToolRegistry()
	reg.Register(&mockResultTool{name: "echo", result: &ToolResult{Content: "hi", RiskLevel: RiskSafe}})

	agent := newTestAgent(client, store, reg)
	ch, err := agent.Run(context.Background(), "conv1", "run echo", nil)
	if err != nil {
		t.Fatal(err)
	}
	events := collectEvents(ch)

	types := make([]EventType, 0, len(events))
	for _, e := range events {
		types = append(types, e.Type)
	}

	hasToolStart := false
	hasToolResult := false
	hasDone := false
	for _, et := range types {
		switch et {
		case EventToolStart:
			hasToolStart = true
		case EventToolResult:
			hasToolResult = true
		case EventDone:
			hasDone = true
		}
	}
	if !hasToolStart {
		t.Error("expected EventToolStart")
	}
	if !hasToolResult {
		t.Error("expected EventToolResult")
	}
	if !hasDone {
		t.Error("expected EventDone")
	}
}

func TestMaxTurnsExceeded(t *testing.T) {
	toolCall := &llm.ToolCall{ID: "tc1", Name: "echo"}
	// Always return a tool call — never terminates naturally.
	turnEvents := []llm.StreamEvent{
		{Type: "tool_start", ToolCall: toolCall},
		{Type: "message_stop"},
	}
	responses := make([][]llm.StreamEvent, 5)
	for i := range responses {
		responses[i] = turnEvents
	}
	client := &mockLLMClient{responses: responses}
	store := &mockMsgStore{}
	reg := NewToolRegistry()
	reg.Register(&mockResultTool{name: "echo", result: &ToolResult{Content: "ok", RiskLevel: RiskSafe}})

	agent := NewAgent(AgentConfig{
		LLMClient: client,
		Registry:  reg,
		Hooks:     NewHookChain(),
		MsgStore:  store,
		MaxTurns:  3,
	})

	ch, err := agent.Run(context.Background(), "conv1", "loop", nil)
	if err != nil {
		t.Fatal(err)
	}
	events := collectEvents(ch)

	last := events[len(events)-1]
	if last.Type != EventError {
		t.Errorf("expected EventError, got %s", last.Type)
	}
	if msg, _ := last.Content["error"].(string); msg != "max turns exceeded" {
		t.Errorf("expected 'max turns exceeded', got %q", msg)
	}
}



