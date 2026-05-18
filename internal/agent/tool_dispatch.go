package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/permission"
)

type toolExecResult struct {
	historyMessages []llm.Message
	pendingResult   string          // toolID\x00content; empty if hidden
	record          *ToolCallRecord // nil if hidden
}

type toolBatch struct {
	concurrent bool
	calls      []llm.ToolCall
}

func partitionToolCalls(calls []llm.ToolCall, registry *ToolRegistry) []toolBatch {
	var batches []toolBatch
	for _, tc := range calls {
		tool, ok := registry.Get(tc.Name)
		safe := ok && tool.IsConcurrencySafe(tc.Input)
		last := len(batches) - 1
		if safe && last >= 0 && batches[last].concurrent {
			batches[last].calls = append(batches[last].calls, tc)
		} else {
			batches = append(batches, toolBatch{concurrent: safe, calls: []llm.ToolCall{tc}})
		}
	}
	return batches
}

func (a *Agent) executeOne(
	ctx context.Context,
	tc llm.ToolCall,
	conversationID string,
	waiter *ConfirmationWaiter,
	events chan<- Event,
) toolExecResult {
	log := logger.FromContext(ctx)
	res := toolExecResult{}

	tool, ok := a.registry.Get(tc.Name)
	if !ok {
		events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "tool not found", "is_error": true}}
		res.historyMessages = []llm.Message{{Role: llm.RoleUser, Content: "Tool " + tc.Name + " not found"}}
		res.pendingResult = tc.ID + "\x00Tool " + tc.Name + " not found"
		res.record = &ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "tool not found", IsError: true}
		return res
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
			res.historyMessages = []llm.Message{{Role: llm.RoleUser, Content: "operation denied by user"}}
			if !hidden {
				res.pendingResult = tc.ID + "\x00operation denied by user"
				res.record = &ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "denied by user", RiskLevel: hookResult.RiskLevel.String()}
			}
			return res
		}
	} else if hookResult.Action == HookDeny {
		events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": "denied: " + hookResult.Reason, "is_error": true}}
		res.historyMessages = []llm.Message{{Role: llm.RoleUser, Content: "Tool denied: " + hookResult.Reason}}
		if !hidden {
			res.pendingResult = tc.ID + "\x00Tool denied: " + hookResult.Reason
			res.record = &ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: "denied: " + hookResult.Reason, RiskLevel: hookResult.RiskLevel.String()}
		}
		return res
	} else if hookResult.Action == HookPlan {
		inputJSON, _ := json.Marshal(tc.Input)
		planMsg := fmt.Sprintf("[PLAN] Would execute tool %s with input: %s", tc.Name, inputJSON)
		events <- Event{Type: EventToolResult, Content: map[string]any{"id": tc.ID, "tool": tc.Name, "result": planMsg, "is_error": false}}
		res.historyMessages = []llm.Message{{Role: llm.RoleUser, Content: planMsg}}
		if !hidden {
			res.pendingResult = tc.ID + "\x00" + planMsg
			res.record = &ToolCallRecord{ID: tc.ID, Name: tc.Name, Input: tc.Input, Result: planMsg, RiskLevel: hookResult.RiskLevel.String()}
		}
		return res
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

	var msgs []llm.Message
	for _, msg := range result.NewMessages {
		msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: msg.Content})
	}
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: result.Content + result.Nudge})
	res.historyMessages = msgs

	if !hidden {
		res.pendingResult = tc.ID + "\x00" + result.Content
		res.record = &ToolCallRecord{
			ID: tc.ID, Name: tc.Name, Input: tc.Input,
			Result: result.Content, IsError: result.IsError,
			RiskLevel: result.RiskLevel.String(), DurationMs: durationMs,
			Summary:   result.Summary,
			HostNames: a.resolveHostNames(tc.Input),
		}
	}
	return res
}

func (a *Agent) executeConcurrent(
	ctx context.Context,
	calls []llm.ToolCall,
	conversationID string,
	waiter *ConfirmationWaiter,
	events chan<- Event,
) []toolExecResult {
	results := make([]toolExecResult, len(calls))
	var wg sync.WaitGroup
	sem := make(chan struct{}, a.maxToolConcurrency)
	for i, tc := range calls {
		wg.Add(1)
		go func(i int, tc llm.ToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = a.executeOne(ctx, tc, conversationID, waiter, events)
		}(i, tc)
	}
	wg.Wait()
	return results
}
