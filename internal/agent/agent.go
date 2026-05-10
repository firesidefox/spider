package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/permission"
)

const epaSystemPromptPrefix = `## Behavioral Constraints

Process tasks in the following order:

Explore: Use read-only tools to gather information first. Do not perform any side-effecting operations until you have a clear understanding of the current state.
Plan: Based on exploration results, reason through a complete execution plan internally. Clarify the purpose and expected outcome of each step.
Act: Execute the plan step by step, verifying results after each step before continuing. If anything unexpected occurs, re-enter Explore — do not proceed blindly.

`

type EventType string

const (
	EventTextDelta       EventType = "text_delta"
	EventToolStart       EventType = "tool_start"
	EventToolResult      EventType = "tool_result"
	EventConfirmRequired EventType = "confirm_required"
	EventError           EventType = "error"
	EventDone            EventType = "done"
)

type Event struct {
	Type    EventType      `json:"type"`
	Content map[string]any `json:"content,omitempty"`
}

type ToolCallRecord struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Input      map[string]any `json:"input,omitempty"`
	Result     string         `json:"result"`
	IsError    bool           `json:"is_error"`
	RiskLevel  string         `json:"risk_level"`
	DurationMs int64          `json:"duration_ms"`
}

type MessageStorer interface {
	Save(conversationID, role, content, toolCalls string) error
	ListByConversation(conversationID string) ([]*models.Message, error)
	ListAfterMessage(conversationID, messageID string) ([]*models.Message, error)
}

type Agent struct {
	llmClient    llm.Client
	registry     *ToolRegistry
	hooks        *HookChain
	msgStore     MessageStorer
	systemPrompt string
	maxTurns     int
	compactor    *Compactor
}

type AgentConfig struct {
	LLMClient    llm.Client
	Registry     *ToolRegistry
	Hooks        *HookChain
	MsgStore     MessageStorer
	SystemPrompt string
	MaxTurns     int
	Compactor    *Compactor
}

func NewAgent(cfg AgentConfig) *Agent {
	maxTurns := cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = 10
	}
	return &Agent{
		llmClient:    cfg.LLMClient,
		registry:     cfg.Registry,
		hooks:        cfg.Hooks,
		msgStore:     cfg.MsgStore,
		systemPrompt: epaSystemPromptPrefix + cfg.SystemPrompt,
		maxTurns:     maxTurns,
		compactor:    cfg.Compactor,
	}
}

type ConfirmationWaiter struct {
	pending map[string]chan bool
	mu      sync.Mutex
}

func NewConfirmationWaiter() *ConfirmationWaiter {
	return &ConfirmationWaiter{pending: make(map[string]chan bool)}
}

func (w *ConfirmationWaiter) Wait(requestID string, timeout time.Duration) (bool, error) {
	ch := make(chan bool, 1)
	w.mu.Lock()
	w.pending[requestID] = ch
	w.mu.Unlock()

	select {
	case approved := <-ch:
		return approved, nil
	case <-time.After(timeout):
		w.mu.Lock()
		delete(w.pending, requestID)
		w.mu.Unlock()
		return false, context.DeadlineExceeded
	}
}

func (w *ConfirmationWaiter) Resolve(requestID string, approved bool) {
	w.mu.Lock()
	ch, ok := w.pending[requestID]
	if ok {
		delete(w.pending, requestID)
	}
	w.mu.Unlock()
	if ok {
		ch <- approved
	}
}

func (a *Agent) Run(ctx context.Context, conversationID string, userMessage string, waiter *ConfirmationWaiter) (<-chan Event, error) {
	events := make(chan Event, 64)
	go func() {
		defer close(events)

		log := logger.FromContext(ctx).With().Str("module", "agent").Str("conv_id", conversationID).Logger()
		ctx = logger.WithContext(ctx, &log)
		log.Info().Msg("agent started")

		a.msgStore.Save(conversationID, "user", userMessage, "")

		var history []llm.Message
		var err error
		if a.compactor != nil {
			history, err = a.compactor.BuildHistory(ctx, conversationID, false)
			if err != nil {
				events <- Event{Type: EventError, Content: map[string]any{"error": err.Error()}}
				return
			}
		} else {
			stored, err := a.msgStore.ListByConversation(conversationID)
			if err != nil {
				events <- Event{Type: EventError, Content: map[string]any{"error": err.Error()}}
				return
			}
			history = toLLMMessages(stored)
		}

		toolDefs := a.registry.Definitions()

		for turn := 0; turn < a.maxTurns; turn++ {
			stream, err := a.llmClient.ChatStream(ctx, &llm.ChatRequest{
				System:    a.systemPrompt,
				Messages:  history,
				Tools:     toolDefs,
				MaxTokens: 4096,
			})
			if err != nil {
				if a.compactor != nil && isContextLengthError(err) {
					forced, ferr := a.compactor.BuildHistory(ctx, conversationID, true)
					if ferr == nil {
						history = forced
						stream, err = a.llmClient.ChatStream(ctx, &llm.ChatRequest{
							System:    a.systemPrompt,
							Messages:  history,
							Tools:     toolDefs,
							MaxTokens: 4096,
						})
					}
				}
				if err != nil {
					events <- Event{Type: EventError, Content: map[string]any{"error": err.Error()}}
					return
				}
			}

			var assistantText string
			var toolCalls []llm.ToolCall
			var currentToolInput string

			finishToolInput := func() {
				if len(toolCalls) > 0 && currentToolInput != "" {
					var input map[string]any
					json.Unmarshal([]byte(currentToolInput), &input) //nolint:errcheck
					toolCalls[len(toolCalls)-1].Input = input
					currentToolInput = ""
					tc := toolCalls[len(toolCalls)-1]
					events <- Event{Type: EventToolStart, Content: map[string]any{"id": tc.ID, "name": tc.Name, "input": tc.Input}}
				}
			}

			for ev := range stream {
				switch ev.Type {
				case "text_delta":
					assistantText += ev.Text
					events <- Event{Type: EventTextDelta, Content: map[string]any{"text": ev.Text}}
				case "tool_start":
					finishToolInput()
					toolCalls = append(toolCalls, *ev.ToolCall)
				case "tool_input_delta":
					currentToolInput += ev.Text
				case "message_stop":
					finishToolInput()
				}
			}

			if len(toolCalls) == 0 {
				if assistantText != "" {
					a.msgStore.Save(conversationID, "assistant", assistantText, "")
				}
				log.Info().Int("turn", turn).Msg("agent done")
				events <- Event{Type: EventDone}
				return
			}

			history = append(history, llm.Message{Role: llm.RoleAssistant, Content: assistantText})

			var tcRecords []ToolCallRecord
			for _, tc := range toolCalls {
				tool, ok := a.registry.Get(tc.Name)
				if !ok {
					events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "tool not found", "is_error": true}}
					history = append(history, llm.Message{Role: llm.RoleUser, Content: "Tool " + tc.Name + " not found"})
					tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "tool not found", IsError: true})
					continue
				}

				riskLevel := tool.DefaultRiskLevel()
				if rl, ok2 := tc.Input["risk_level"].(string); ok2 {
					riskLevel = permission.ParseRiskLevel(rl)
				}

				hookResult := a.hooks.RunBefore(tc.Name, tc.Input, riskLevel)

				if hookResult.Action == HookRequireConfirm && waiter != nil {
					requestID := uuid.New().String()
					events <- Event{Type: EventConfirmRequired, Content: map[string]any{
						"request_id": requestID, "tool": tc.Name,
						"input": tc.Input, "risk_level": hookResult.RiskLevel.String(),
					}}
					approved, err := waiter.Wait(requestID, 5*time.Minute)
					if err != nil || !approved {
						events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "denied by user", "is_error": true}}
						history = append(history, llm.Message{Role: llm.RoleUser, Content: "operation denied by user"})
						tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "denied by user", RiskLevel: hookResult.RiskLevel.String()})
						continue
					}
				} else if hookResult.Action == HookDeny {
					events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "denied: " + hookResult.Reason, "is_error": true}}
					history = append(history, llm.Message{Role: llm.RoleUser, Content: "Tool denied: " + hookResult.Reason})
					tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "denied: " + hookResult.Reason, RiskLevel: hookResult.RiskLevel.String()})
					continue
				} else if hookResult.Action == HookPlan {
					inputJSON, _ := json.Marshal(tc.Input)
					planMsg := fmt.Sprintf("[PLAN] Would execute tool %s with input: %s", tc.Name, inputJSON)
					events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": planMsg, "is_error": false}}
					history = append(history, llm.Message{Role: llm.RoleUser, Content: planMsg})
					tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: planMsg, RiskLevel: hookResult.RiskLevel.String()})
					continue
				}

				start := time.Now()
				log.Debug().Str("tool", tc.Name).Msg("tool call start")
				result, err := tool.Execute(ctx, tc.Input)
				durationMs := time.Since(start).Milliseconds()
				if err != nil {
					result = &ToolResult{Content: err.Error(), IsError: true, RiskLevel: riskLevel}
					log.Error().Err(err).Str("tool", tc.Name).Int64("duration_ms", durationMs).Msg("tool call error")
				} else {
					log.Debug().Str("tool", tc.Name).Int64("duration_ms", durationMs).Bool("is_error", result.IsError).Msg("tool call done")
				}
				a.hooks.RunAfter(tc.Name, tc.Input, result)

				events <- Event{Type: EventToolResult, Content: map[string]any{
					"id": tc.ID, "tool": tc.Name, "input": tc.Input, "result": result.Content, "is_error": result.IsError, "duration_ms": durationMs,
				}}
				history = append(history, llm.Message{Role: llm.RoleUser, Content: result.Content})
				tcRecords = append(tcRecords, ToolCallRecord{
					ID: tc.ID, Name: tc.Name, Input: tc.Input,
					Result: result.Content, IsError: result.IsError,
					RiskLevel: result.RiskLevel.String(), DurationMs: durationMs,
				})
			}

			tcJSON, _ := json.Marshal(tcRecords)
			a.msgStore.Save(conversationID, "assistant", assistantText, string(tcJSON))
		}

		events <- Event{Type: EventError, Content: map[string]any{"error": "max turns exceeded"}}
		log.Error().Int("max_turns", a.maxTurns).Msg("agent max turns exceeded")
	}()

	return events, nil
}

func isContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "context_length_exceeded") ||
		strings.Contains(s, "context length") ||
		strings.Contains(s, "too many tokens") ||
		strings.Contains(s, "maximum context")
}
