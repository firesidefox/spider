package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func (m *mockLLMClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
	return "summary", nil
}

func (m *mockLLMClient) CountTokens(_ context.Context, msgs []llm.Message) (int, error) {
	total := 0
	for _, msg := range msgs {
		if s, ok := msg.Content.(string); ok {
			total += llm.EstimateTokens(s)
		}
	}
	return total, nil
}

// capturingLLMClient calls onStream for each request, allowing tests to inspect history.
type capturingLLMClient struct {
	onStream func(req *llm.ChatRequest) (<-chan llm.StreamEvent, error)
}

func (c *capturingLLMClient) ChatStream(_ context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return c.onStream(req)
}

func (c *capturingLLMClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
	return "summary", nil
}

func (c *capturingLLMClient) CountTokens(_ context.Context, msgs []llm.Message) (int, error) {
	total := 0
	for _, msg := range msgs {
		if s, ok := msg.Content.(string); ok {
			total += llm.EstimateTokens(s)
		}
	}
	return total, nil
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

func (m *mockMsgStore) ListAfterMessage(convID, _ string) ([]*models.Message, error) {
	return m.ListByConversation(convID)
}

type mockResultTool struct {
	name   string
	result *ToolResult
}

func (t *mockResultTool) Name() string                    { return t.name }
func (t *mockResultTool) Description() string             { return t.name }
func (t *mockResultTool) InputSchema() map[string]any     { return map[string]any{} }
func (t *mockResultTool) DefaultRiskLevel() RiskLevel              { return RiskL1 }
func (t *mockResultTool) IsConcurrencySafe(_ map[string]any) bool { return false }
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

	ch, err := agent.Run(context.Background(), "conv1", "hi", nil, nil)
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
	reg.Register(&mockResultTool{name: "echo", result: &ToolResult{Content: "hi", RiskLevel: RiskL1}})

	agent := newTestAgent(client, store, reg)
	ch, err := agent.Run(context.Background(), "conv1", "run echo", nil, nil)
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
	reg.Register(&mockResultTool{name: "echo", result: &ToolResult{Content: "ok", RiskLevel: RiskL1}})

	agent := NewAgent(AgentConfig{
		LLMClient: client,
		Registry:  reg,
		Hooks:     NewHookChain(),
		MsgStore:  store,
		MaxTurns:  3,
	})

	ch, err := agent.Run(context.Background(), "conv1", "loop", nil, nil)
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

func TestReadOnlyToolDescriptionsContainExploreHint(t *testing.T) {
	tools := []Tool{
		NewGetHostsTool(nil, nil),
		NewSearchDocsTool(nil, nil),
		NewVerifyTool(nil, nil, nil, nil),
	}
	for _, tool := range tools {
		desc := tool.Description()
		if !strings.Contains(desc, "Read-only") {
			t.Errorf("tool %q description should contain 'Read-only', got: %q", tool.Name(), desc)
		}
	}
}

func TestActToolDescriptionsContainSideEffectHint(t *testing.T) {
	tools := []Tool{
		NewExecuteCLITool(nil, nil, nil, nil, nil),
		NewBatchExecuteTool(nil, nil, nil, nil, nil),
		NewCallRESTAPITool(nil),
	}
	for _, tool := range tools {
		desc := tool.Description()
		if !strings.Contains(desc, "side effects") {
			t.Errorf("tool %q description should contain 'side effects', got: %q", tool.Name(), desc)
		}
	}
}

func TestDrainInjectCh(t *testing.T) {
	ch := make(chan string, 32)
	ch <- "hello"
	ch <- "world"

	parts := drainInjectCh(ch)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0] != "hello" || parts[1] != "world" {
		t.Fatalf("unexpected parts: %v", parts)
	}

	// nil channel returns empty
	parts = drainInjectCh(nil)
	if len(parts) != 0 {
		t.Fatalf("expected 0 parts for nil ch, got %d", len(parts))
	}

	// closed channel exits cleanly
	ch2 := make(chan string, 4)
	ch2 <- "a"
	close(ch2)
	parts = drainInjectCh(ch2)
	if len(parts) != 1 || parts[0] != "a" {
		t.Fatalf("unexpected parts from closed ch: %v", parts)
	}
}

func TestMidTurnInjection(t *testing.T) {
	toolCall := &llm.ToolCall{ID: "t1", Name: "echo"}

	// capturingLLMClient records each ChatRequest so we can verify history contents.
	type capturedCall struct{ msgs []llm.Message }
	var captured []capturedCall
	responses := [][]llm.StreamEvent{
		// First turn: tool call → triggers drain-after-tool-batch path
		{
			{Type: "tool_start", ToolCall: toolCall},
			{Type: "tool_input_delta", Text: `{"msg":"hi"}`},
			{Type: "message_stop"},
		},
		// Second turn: pure text → agent done
		{
			{Type: "text_delta", Text: "got it"},
			{Type: "message_stop"},
		},
	}
	callIdx := 0
	capClient := &capturingLLMClient{
		onStream: func(req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
			captured = append(captured, capturedCall{msgs: req.Messages})
			ch := make(chan llm.StreamEvent, 32)
			evs := responses[callIdx]
			callIdx++
			go func() {
				defer close(ch)
				for _, e := range evs {
					ch <- e
				}
			}()
			return ch, nil
		},
	}

	injectCh := make(chan string, 32)
	injectCh <- "please stop"

	reg := NewToolRegistry()
	reg.Register(&mockResultTool{name: "echo", result: &ToolResult{Content: "ok", RiskLevel: RiskL1}})

	store := &mockMsgStore{}
	ag := newTestAgent(capClient, store, reg)

	events, err := ag.Run(context.Background(), "conv1", "start", nil, injectCh)
	if err != nil {
		t.Fatal(err)
	}

	var eventTypes []string
	for ev := range events {
		eventTypes = append(eventTypes, string(ev.Type))
	}

	// Must see mid_turn_user_message
	found := false
	for _, et := range eventTypes {
		if et == string(EventMidTurnUserMessage) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mid_turn_user_message event, got: %v", eventTypes)
	}

	// Second LLM call must include the injected message in its history
	if len(captured) < 2 {
		t.Fatalf("expected at least 2 LLM calls, got %d", len(captured))
	}
	hasInjected := false
	for _, m := range captured[1].msgs {
		if s, ok := m.Content.(string); ok && s == "please stop" {
			hasInjected = true
		}
	}
	if !hasInjected {
		t.Fatalf("injected message not found in second LLM call history: %+v", captured[1].msgs)
	}

	// Injected message must also be persisted to the store
	savedToStore := false
	for _, msg := range store.messages {
		if msg.role == "user" && msg.content == "please stop" {
			savedToStore = true
		}
	}
	if !savedToStore {
		t.Fatalf("injected message not saved to msgStore: %+v", store.messages)
	}
}

func TestMidTurnInjection_PureTextPath(t *testing.T) {
	// Agent returns pure text (no tool calls) on turn 1.
	// Injected message causes a continue; turn 2 returns pure text and agent exits.
	type capturedCall struct{ msgs []llm.Message }
	var captured []capturedCall
	responses := [][]llm.StreamEvent{
		// Turn 1: pure text, no tool calls
		{
			{Type: "text_delta", Text: "thinking..."},
			{Type: "message_stop"},
		},
		// Turn 2: pure text, agent exits
		{
			{Type: "text_delta", Text: "done"},
			{Type: "message_stop"},
		},
	}
	callIdx := 0
	capClient := &capturingLLMClient{
		onStream: func(req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
			captured = append(captured, capturedCall{msgs: req.Messages})
			ch := make(chan llm.StreamEvent, 32)
			evs := responses[callIdx]
			callIdx++
			go func() {
				defer close(ch)
				for _, e := range evs {
					ch <- e
				}
			}()
			return ch, nil
		},
	}

	injectCh := make(chan string, 32)
	injectCh <- "stop now"

	store := &mockMsgStore{}
	ag := newTestAgent(capClient, store, NewToolRegistry())

	events, err := ag.Run(context.Background(), "conv1", "start", nil, injectCh)
	if err != nil {
		t.Fatal(err)
	}

	var eventTypes []string
	for ev := range events {
		eventTypes = append(eventTypes, string(ev.Type))
	}

	// Must see mid_turn_user_message
	found := false
	for _, et := range eventTypes {
		if et == string(EventMidTurnUserMessage) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mid_turn_user_message event in pure-text path, got: %v", eventTypes)
	}

	// Must end with done
	if len(eventTypes) == 0 || eventTypes[len(eventTypes)-1] != string(EventDone) {
		t.Fatalf("expected EventDone as last event, got: %v", eventTypes)
	}

	// Second LLM call must include injected message
	if len(captured) < 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(captured))
	}
	hasInjected := false
	for _, m := range captured[1].msgs {
		if s, ok := m.Content.(string); ok && s == "stop now" {
			hasInjected = true
		}
	}
	if !hasInjected {
		t.Fatalf("injected message not in second LLM call history: %+v", captured[1].msgs)
	}

	// EventMidTurnUserMessage must come before EventTurnUsage (ordering consistency)
	midIdx, usageIdx := -1, -1
	for i, et := range eventTypes {
		if et == string(EventMidTurnUserMessage) && midIdx == -1 {
			midIdx = i
		}
		if et == string(EventTurnUsage) && usageIdx == -1 {
			usageIdx = i
		}
	}
	if midIdx == -1 || usageIdx == -1 {
		t.Fatalf("missing mid_turn_user_message or turn_usage in events: %v", eventTypes)
	}
	if midIdx > usageIdx {
		t.Fatalf("EventMidTurnUserMessage (idx %d) must come before EventTurnUsage (idx %d)", midIdx, usageIdx)
	}
}

func TestAgent_PerToolResultLimit_Truncates(t *testing.T) {
	dataDir := t.TempDir()
	bigContent := strings.Repeat("a", 60_000)

	toolCall := &llm.ToolCall{ID: "tu1", Name: "big_tool"}
	toolCallResp := []llm.StreamEvent{
		{Type: "tool_start", ToolCall: toolCall},
		{Type: "tool_input_delta", Text: `{}`},
		{Type: "message_stop"},
	}
	doneResp := []llm.StreamEvent{
		{Type: "text_delta", Text: "done"},
		{Type: "message_stop"},
	}
	client := &mockLLMClient{responses: [][]llm.StreamEvent{toolCallResp, doneResp}}

	reg := NewToolRegistry()
	reg.Register(&mockResultTool{name: "big_tool", result: &ToolResult{Content: bigContent, RiskLevel: RiskL1}})

	a := NewAgent(AgentConfig{
		LLMClient:             client,
		Registry:              reg,
		Hooks:                 NewHookChain(),
		MsgStore:              &mockMsgStore{},
		MaxTurns:              3,
		DataDir:               dataDir,
		PerToolResultMaxChars: 50_000,
		ReplacementState:      newContentReplacementState(),
	})

	ch, err := a.Run(context.Background(), "conv1", "go", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for e := range ch {
		if e.Type == EventToolResult {
			result, _ := e.Content["result"].(string)
			if len(result) >= 60_000 {
				t.Errorf("tool result not truncated: len=%d", len(result))
			}
			if !strings.Contains(result, "too large") {
				t.Errorf("expected truncation notice in result, got: %s", result[:min(100, len(result))])
			}
		}
	}
	entries, _ := os.ReadDir(filepath.Join(dataDir, "tool-results", "conv1"))
	if len(entries) != 1 {
		t.Errorf("expected 1 persisted file, got %d", len(entries))
	}
}
