package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/permission"
	"github.com/spiderai/spider/internal/store"
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
	EventTurnUsage       EventType = "turn_usage"
	EventRetrying        EventType = "retrying"
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
	Summary    string         `json:"summary,omitempty"`
	HostNames  []string       `json:"host_names,omitempty"`
}

type MessageStorer interface {
	Save(conversationID, role, content, toolCalls string) error
	ListByConversation(conversationID string) ([]*models.Message, error)
	ListAfterMessage(conversationID, messageID string) ([]*models.Message, error)
}

// noopMessageStorer discards all messages. Used for headless task runs.
type noopMessageStorer struct{}

func (noopMessageStorer) Save(_, _, _, _ string) error                          { return nil }
func (noopMessageStorer) ListByConversation(_ string) ([]*models.Message, error) { return nil, nil }
func (noopMessageStorer) ListAfterMessage(_, _ string) ([]*models.Message, error) { return nil, nil }

type Agent struct {
	llmClient     llm.Client
	registry      *ToolRegistry
	hooks         *HookChain
	msgStore      MessageStorer
	todoStore     *store.TodoStore
	hosts         *store.HostStore
	systemPrompt  string
	maxTurns      int
	compactor     *Compactor
	skillManager  *SkillManager
	lastSkillHash string
	dataDir                      string
	perToolResultMaxChars        int
	perMessageToolResultMaxChars int
	replacementState             *ContentReplacementState
}

type AgentConfig struct {
	LLMClient    llm.Client
	Registry     *ToolRegistry
	Hooks        *HookChain
	MsgStore     MessageStorer
	TodoStore    *store.TodoStore
	Hosts        *store.HostStore
	SystemPrompt string
	MaxTurns     int
	Compactor    *Compactor
	SkillManager *SkillManager
	DataDir                      string
	PerToolResultMaxChars        int
	PerMessageToolResultMaxChars int
	ReplacementState             *ContentReplacementState
}

func NewAgent(cfg AgentConfig) *Agent {
	maxTurns := cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = 10
	}
	return &Agent{
		llmClient:                    cfg.LLMClient,
		registry:                     cfg.Registry,
		hooks:                        cfg.Hooks,
		msgStore:                     cfg.MsgStore,
		todoStore:                    cfg.TodoStore,
		hosts:                        cfg.Hosts,
		systemPrompt:                 epaSystemPromptPrefix + cfg.SystemPrompt,
		maxTurns:                     maxTurns,
		compactor:                    cfg.Compactor,
		skillManager:                 cfg.SkillManager,
		dataDir:                      cfg.DataDir,
		perToolResultMaxChars:        cfg.PerToolResultMaxChars,
		perMessageToolResultMaxChars: cfg.PerMessageToolResultMaxChars,
		replacementState:             cfg.ReplacementState,
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

		finalUserMessage := userMessage
		if a.skillManager != nil {
			currentHash, _ := a.skillManager.ComputeHash()
			if currentHash != a.lastSkillHash {
				entries, _ := a.skillManager.LoadSkills()
				list := a.skillManager.RenderList(entries)
				if list != "" {
					finalUserMessage = fmt.Sprintf("<skills>\nNOTE: Replaces any earlier skill list in this conversation.\n%s\n</skills>\n\n%s", list, userMessage)
				}
				a.lastSkillHash = currentHash
			}
		}
		a.msgStore.Save(conversationID, "user", userMessage, "")

		var history []llm.Message
		var err error
		if a.compactor != nil {
			history, err = a.compactor.BuildHistory(ctx, conversationID, false)
			if err != nil {
				log.Error().Err(err).Msg("agent exit: build history failed")
				events <- Event{Type: EventError, Content: map[string]any{"error": err.Error()}}
				return
			}
		} else {
			stored, err := a.msgStore.ListByConversation(conversationID)
			if err != nil {
				log.Error().Err(err).Msg("agent exit: list messages failed")
				events <- Event{Type: EventError, Content: map[string]any{"error": err.Error()}}
				return
			}
			history = toLLMMessages(stored, toolResultBudget{
					maxChars: a.perMessageToolResultMaxChars,
					dataDir:  a.dataDir,
					convID:   conversationID,
					state:    a.replacementState,
				})
		}

		// Replace last message with skill-injected version for LLM only
		if finalUserMessage != userMessage && len(history) > 0 {
			history[len(history)-1].Content = finalUserMessage
		}

		history = a.injectTaskReminder(history, conversationID)

		toolDefs := a.registry.Definitions()

		for turn := 0; turn < a.maxTurns; turn++ {
			turnStart := time.Now()
			log.Debug().Int("turn", turn).Int("history", len(history)).Msg("turn start")
			chatReq := &llm.ChatRequest{
				System:    a.systemPrompt,
				Messages:  history,
				Tools:     toolDefs,
				MaxTokens: 4096,
			}
			var stream <-chan llm.StreamEvent
			for attempt := range llmMaxRetries {
				if attempt > 0 {
					delay := llmRetryDelay(attempt - 1)
					log.Warn().Err(err).Int("turn", turn).Int("attempt", attempt).Dur("backoff", delay).Msg("llm transient error, retrying")
					events <- Event{Type: EventRetrying, Content: map[string]any{
						"attempt":      attempt,
						"max_retries":  llmMaxRetries,
						"error":        err.Error(),
						"retry_in_ms":  delay.Milliseconds(),
					}}
					select {
					case <-ctx.Done():
						log.Error().Err(ctx.Err()).Int("turn", turn).Msg("llm chat stream failed")
						events <- Event{Type: EventError, Content: map[string]any{"error": "llm: " + ctx.Err().Error()}}
						return
					case <-time.After(delay):
					}
				}
				stream, err = a.llmClient.ChatStream(ctx, chatReq)
				if err == nil {
					break
				}
				if ctx.Err() != nil || !isTransientLLMError(err) {
					break
				}
			}
			if err != nil {
				if a.compactor != nil && isContextLengthError(err) {
					forced, ferr := a.compactor.BuildHistory(ctx, conversationID, true)
					if ferr == nil {
						history = a.injectTaskReminder(forced, conversationID)
						chatReq.Messages = history
						stream, err = a.llmClient.ChatStream(ctx, chatReq)
					}
				}
				if err != nil {
					log.Error().Err(err).Int("turn", turn).Msg("llm chat stream failed")
					events <- Event{Type: EventError, Content: map[string]any{"error": "llm: " + err.Error()}}
					return
				}
			}

			var assistantText string
			var toolCalls []llm.ToolCall
			var currentToolInput string
			var usage llm.Usage

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
				case "usage":
					if ev.Usage != nil {
						if ev.Usage.InputTokens > 0 {
							usage.InputTokens = ev.Usage.InputTokens
						}
						if ev.Usage.OutputTokens > 0 {
							usage.OutputTokens = ev.Usage.OutputTokens
						}
					}
				case "message_stop":
					finishToolInput()
				}
			}

			if len(toolCalls) == 0 {
				if assistantText != "" {
					a.msgStore.Save(conversationID, "assistant", assistantText, "")
				}
				log.Debug().Int("turn", turn).Int64("duration_ms", time.Since(turnStart).Milliseconds()).Int("input_tokens", usage.InputTokens).Int("output_tokens", usage.OutputTokens).Str("response", assistantText).Msg("turn done")
				log.Info().Int("turn", turn).Str("reason", "no_tool_calls").Msg("agent done")
				events <- Event{Type: EventTurnUsage, Content: map[string]any{
					"input_tokens":  usage.InputTokens,
					"output_tokens": usage.OutputTokens,
				}}
				events <- Event{Type: EventDone}
				return
			}

			if assistantText != "" {
				history = append(history, llm.Message{Role: llm.RoleAssistant, Content: assistantText})
			}

			var tcRecords []ToolCallRecord
			var pendingToolResults []string // toolID\x00content, saved after assistant
			for _, tc := range toolCalls {
				tool, ok := a.registry.Get(tc.Name)
				if !ok {
					events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "tool not found", "is_error": true}}
					history = append(history, llm.Message{Role: llm.RoleUser, Content: "Tool " + tc.Name + " not found"})
					pendingToolResults = append(pendingToolResults, tc.ID+"\x00Tool "+tc.Name+" not found")
					tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "tool not found", IsError: true})
					continue
				}

				hidden := false
				if h, ok2 := tool.(HiddenTool); ok2 {
					hidden = h.Hidden()
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
						if !hidden {
							pendingToolResults = append(pendingToolResults, tc.ID+"\x00operation denied by user")
							tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "denied by user", RiskLevel: hookResult.RiskLevel.String()})
						}
						continue
					}
				} else if hookResult.Action == HookDeny {
					events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "denied: " + hookResult.Reason, "is_error": true}}
					history = append(history, llm.Message{Role: llm.RoleUser, Content: "Tool denied: " + hookResult.Reason})
					if !hidden {
						pendingToolResults = append(pendingToolResults, tc.ID+"\x00Tool denied: "+hookResult.Reason)
						tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "denied: " + hookResult.Reason, RiskLevel: hookResult.RiskLevel.String()})
					}
					continue
				} else if hookResult.Action == HookPlan {
					inputJSON, _ := json.Marshal(tc.Input)
					planMsg := fmt.Sprintf("[PLAN] Would execute tool %s with input: %s", tc.Name, inputJSON)
					events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": planMsg, "is_error": false}}
					history = append(history, llm.Message{Role: llm.RoleUser, Content: planMsg})
					if !hidden {
						pendingToolResults = append(pendingToolResults, tc.ID+"\x00"+planMsg)
						tcRecords = append(tcRecords, ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: planMsg, RiskLevel: hookResult.RiskLevel.String()})
					}
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
					log.Debug().Str("tool", tc.Name).Int64("duration_ms", durationMs).Bool("is_error", result.IsError).Interface("input", tc.Input).Str("output", result.Content).Msg("tool call done")
				}
				a.hooks.RunAfter(tc.Name, tc.Input, result)

				// Layer 1: per-tool result size limit
				if a.perToolResultMaxChars > 0 && len(result.Content) > a.perToolResultMaxChars && a.replacementState != nil {
					filePath, ferr := persistToolResult(a.dataDir, conversationID, tc.ID, result.Content)
					if ferr != nil {
						log.Warn().Err(ferr).Str("tool", tc.Name).Msg("failed to persist large tool result; passing through")
					} else {
						preview := generatePreview(result.Content, filePath)
						a.replacementState.setReplacement(tc.ID, preview)
						result = &ToolResult{Content: preview, IsError: result.IsError, RiskLevel: result.RiskLevel}
					}
				}

				events <- Event{Type: EventToolResult, Content: map[string]any{
					"id": tc.ID, "tool": tc.Name, "input": tc.Input, "result": result.Content, "is_error": result.IsError, "duration_ms": durationMs, "summary": result.Summary,
				}}
				for _, msg := range result.NewMessages {
					history = append(history, llm.Message{Role: llm.RoleUser, Content: msg.Content})
				}
				history = append(history, llm.Message{Role: llm.RoleUser, Content: result.Content + result.Nudge})
				if !hidden {
					pendingToolResults = append(pendingToolResults, tc.ID+"\x00"+result.Content)
					tcRecords = append(tcRecords, ToolCallRecord{
						ID: tc.ID, Name: tc.Name, Input: tc.Input,
						Result: result.Content, IsError: result.IsError,
						RiskLevel: result.RiskLevel.String(), DurationMs: durationMs,
						Summary:   result.Summary,
						HostNames: a.resolveHostNames(tc.Input),
					})
				}
			}

			tcJSON, _ := json.Marshal(tcRecords)
			a.msgStore.Save(conversationID, "assistant", assistantText, string(tcJSON))
			for _, tr := range pendingToolResults {
				a.msgStore.Save(conversationID, "tool_result", tr, "")
			}
			log.Debug().Int("turn", turn).Int64("duration_ms", time.Since(turnStart).Milliseconds()).Int("input_tokens", usage.InputTokens).Int("output_tokens", usage.OutputTokens).Int("tools", len(tcRecords)).Str("response", assistantText).Msg("turn done")
			events <- Event{Type: EventTurnUsage, Content: map[string]any{
				"input_tokens":  usage.InputTokens,
				"output_tokens": usage.OutputTokens,
			}}
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

const (
	llmMaxRetries    = 10
	llmBaseDelayMs   = 500
	llmMaxDelayMs    = 32_000
)

func llmRetryDelay(attempt int) time.Duration {
	base := float64(llmBaseDelayMs) * math.Pow(2, float64(attempt))
	if base > llmMaxDelayMs {
		base = llmMaxDelayMs
	}
	jitter := rand.Float64() * 0.25 * base
	return time.Duration(base+jitter) * time.Millisecond
}

func isTransientLLMError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Connection-level transient errors
	if strings.Contains(s, "timeout awaiting response headers") ||
		strings.Contains(s, "connection reset by peer") ||
		strings.HasSuffix(s, "EOF") {
		return true
	}
	// HTTP status codes: 408 timeout, 409 lock, 429 rate limit, 5xx server errors
	for _, prefix := range []string{"claude API error ", "openai API error "} {
		if strings.HasPrefix(s, prefix) {
			var code int
			if _, scanErr := fmt.Sscanf(s[len(prefix):], "%d", &code); scanErr == nil {
				return code == 408 || code == 409 || code == 429 || code >= 500
			}
		}
	}
	return false
}

func (a *Agent) injectTaskReminder(history []llm.Message, conversationID string) []llm.Message {
	if a.todoStore == nil {
		return history
	}
	tasks, err := a.todoStore.List(conversationID)
	if err != nil || len(tasks) == 0 {
		return history
	}
	return append(history, buildTaskReminderMessage(tasks))
}

func buildTaskReminderMessage(tasks []*models.Todo) llm.Message {
	var sb strings.Builder
	sb.WriteString("<system-reminder>\nCurrent tasks for this conversation:\n")
	for _, t := range tasks {
		fmt.Fprintf(&sb, "[%d] %s: %s\n", t.Seq, t.Status, t.Subject)
	}
	sb.WriteString("</system-reminder>")
	return llm.Message{Role: llm.RoleUser, Content: sb.String()}
}

func (a *Agent) resolveHostNames(input map[string]any) []string {
	if a.hosts == nil {
		return nil
	}
	return a.hosts.ResolveNames(input)
}
