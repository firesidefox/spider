# Mid-Turn User Message Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Inject user messages queued during agent execution into the LLM history after each tool batch, so the LLM can respond within the same agent loop instead of waiting for the full run to finish.

**Architecture:** `Agent.Run` gains an `injectCh <-chan string` parameter. After each tool batch (and before emitting `EventDone` on pure-text turns), the agent drains the channel and appends merged messages to history. The API layer uses three mutex-protected operations (`TryClaimConv`, `TryInject`, `ReleaseConv`) to route incoming HTTP requests: inject into the running agent if one exists, otherwise start a new one. The frontend sends HTTP immediately on queue (instead of buffering locally) and promotes messages to conversation display on `mid_turn_user_message` SSE events.

**Tech Stack:** Go 1.22+, Vue 3 (Composition API), TypeScript

---

## File Map

| File | Change |
|------|--------|
| `internal/agent/agent.go` | Add `injectCh <-chan string` param to `Run`; add drain helper; add `EventMidTurnUserMessage` |
| `internal/mcp/server.go` | Add `convInjectChs map[string]chan string` + mutex; add `TryClaimConv`, `TryInject`, `ReleaseConv` |
| `internal/api/chat.go` | Use `TryInject`/`TryClaimConv` routing; move `ReleaseConv` into goroutine cleanup |
| `internal/scheduler/executor.go` | Pass `nil` as `injectCh` |
| `internal/agent/agent_test.go` | Update existing `Run` call sites to pass `nil` |
| `web/src/api/chat.ts` | Update `ChatEvent` type; update `sendMessage` return to expose status code |
| `web/src/views/ChatView.vue` | Send HTTP on queue; handle `mid_turn_user_message`; remove `flushQueue` |

---

## Task 1: Add `EventMidTurnUserMessage` and drain helper to `agent.go`

**Files:**
- Modify: `internal/agent/agent.go`
- Test: `internal/agent/agent_test.go`

- [ ] **Step 1: Add the new event type constant**

In `internal/agent/agent.go`, add after `EventRetrying`:

```go
EventMidTurnUserMessage EventType = "mid_turn_user_message"
```

- [ ] **Step 2: Write a failing test for drain behavior**

In `internal/agent/agent_test.go`, add this test (the function `drainInjectCh` doesn't exist yet):

```go
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
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/ -run TestDrainInjectCh -v
```

Expected: compile error — `drainInjectCh undefined`

- [ ] **Step 4: Add `drainInjectCh` to `agent.go`**

Add this function at the bottom of `internal/agent/agent.go` (before the closing of the file):

```go
func drainInjectCh(ch <-chan string) []string {
	if ch == nil {
		return nil
	}
	var parts []string
loop:
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				break loop
			}
			parts = append(parts, msg)
		default:
			break loop
		}
	}
	return parts
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/agent/ -run TestDrainInjectCh -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/agent/agent.go internal/agent/agent_test.go
git commit -m "feat(agent): add EventMidTurnUserMessage and drainInjectCh helper"
```

---

## Task 2: Wire `injectCh` into `Agent.Run`

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/agent_test.go`
- Modify: `internal/scheduler/executor.go`

- [ ] **Step 1: Update `Run` signature**

In `internal/agent/agent.go`, change line 169:

```go
func (a *Agent) Run(ctx context.Context, conversationID string, userMessage string, waiter *ConfirmationWaiter, injectCh <-chan string) (<-chan Event, error) {
```

- [ ] **Step 2: Fix compile errors — update all `Run` call sites**

`internal/scheduler/executor.go` line 134 — pass `nil`:

```go
events, err := ag.Run(execCtx, convID, task.Goal, nil, nil)
```

`internal/api/chat.go` line 215 — temporarily pass `nil` (will be fixed in Task 4):

```go
events, err := a.Run(ctx, id, content, waiter, nil)
```

`internal/agent/agent_test.go` — update all three `Run` calls to pass `nil` as fifth arg:

```go
// line 118
ch, err := agent.Run(context.Background(), "conv1", "hi", nil, nil)
// line 156
ch, err := agent.Run(context.Background(), "conv1", "run echo", nil, nil)
// line 215
ch, err := agent.Run(context.Background(), "conv1", "loop", nil, nil)
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Add drain after tool batches**

In `internal/agent/agent.go`, after the `for _, tr := range pendingToolResults` block (around line 362) and before `events <- Event{Type: EventTurnUsage, ...}`, add:

```go
if parts := drainInjectCh(injectCh); len(parts) > 0 {
    merged := strings.Join(parts, "\n\n")
    a.msgStore.Save(conversationID, "user", merged, "")
    history = append(history, llm.Message{Role: llm.RoleUser, Content: merged})
    events <- Event{Type: EventMidTurnUserMessage, Content: map[string]any{"text": merged}}
}
```

- [ ] **Step 5: Add drain before `EventDone` on pure-text turns**

In `internal/agent/agent.go`, find the `if len(toolCalls) == 0 {` block (around line 317). Replace:

```go
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
```

With:

```go
if len(toolCalls) == 0 {
    if assistantText != "" {
        a.msgStore.Save(conversationID, "assistant", assistantText, "")
    }
    // Drain queued user messages before deciding to exit.
    // If any arrived, continue the loop so LLM can respond to them.
    if parts := drainInjectCh(injectCh); len(parts) > 0 {
        merged := strings.Join(parts, "\n\n")
        a.msgStore.Save(conversationID, "user", merged, "")
        if assistantText != "" {
            history = append(history, llm.Message{Role: llm.RoleAssistant, Content: assistantText})
        }
        history = append(history, llm.Message{Role: llm.RoleUser, Content: merged})
        events <- Event{Type: EventTurnUsage, Content: map[string]any{
            "input_tokens":  usage.InputTokens,
            "output_tokens": usage.OutputTokens,
        }}
        events <- Event{Type: EventMidTurnUserMessage, Content: map[string]any{"text": merged}}
        continue
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
```

- [ ] **Step 6: Write a test for mid-turn injection**

In `internal/agent/agent_test.go`, add:

```go
func TestMidTurnInjection(t *testing.T) {
	// Agent that calls one tool, then returns text.
	// We inject a user message after the tool call.
	callCount := 0
	var capturedHistory []llm.Message

	mockLLM := &mockLLMClient{
		streamFn: func(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
			callCount++
			capturedHistory = req.Messages
			ch := make(chan llm.StreamEvent, 4)
			if callCount == 1 {
				// First turn: emit a tool call
				ch <- llm.StreamEvent{Type: "tool_start", ToolCall: &llm.ToolCall{ID: "t1", Name: "echo", Input: map[string]any{"msg": "hi"}}}
				ch <- llm.StreamEvent{Type: "message_stop"}
			} else {
				// Second turn: pure text response
				ch <- llm.StreamEvent{Type: "text_delta", Text: "got it"}
				ch <- llm.StreamEvent{Type: "message_stop"}
			}
			close(ch)
			return ch, nil
		},
	}

	injectCh := make(chan string, 32)
	injectCh <- "please stop"

	reg := NewToolRegistry()
	reg.Register(echoTool()) // assumes echoTool() helper exists in test file
	ag := &Agent{
		llmClient: mockLLM,
		registry:  reg,
		msgStore:  noopMessageStorer{},
		maxTurns:  5,
	}

	events, err := ag.Run(context.Background(), "conv1", "start", nil, injectCh)
	if err != nil {
		t.Fatal(err)
	}

	var eventTypes []string
	for ev := range events {
		eventTypes = append(eventTypes, string(ev.Type))
	}

	// Must see mid_turn_user_message before done
	found := false
	for _, et := range eventTypes {
		if et == string(EventMidTurnUserMessage) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mid_turn_user_message event, got: %v", eventTypes)
	}

	// Second LLM call must include the injected message in history
	hasInjected := false
	for _, m := range capturedHistory {
		if s, ok := m.Content.(string); ok && s == "please stop" {
			hasInjected = true
		}
	}
	if !hasInjected {
		t.Fatalf("injected message not found in second LLM call history")
	}
}
```

- [ ] **Step 7: Run all agent tests**

```bash
go test ./internal/agent/ -v -timeout 30s
```

Expected: all pass (TestMidTurnInjection may need mock helpers — fix any compile errors before proceeding)

- [ ] **Step 8: Commit**

```bash
git add internal/agent/agent.go internal/agent/agent_test.go internal/scheduler/executor.go
git commit -m "feat(agent): wire injectCh into Run for mid-turn user message injection"
```

---

## Task 3: Add `TryClaimConv`, `TryInject`, `ReleaseConv` to `App`

**Files:**
- Modify: `internal/mcp/server.go`

- [ ] **Step 1: Add the map and mutex fields to `App` struct**

In `internal/mcp/server.go`, find the `App` struct. After `convCancelsMu sync.Mutex`, add:

```go
convInjectChs   map[string]chan string
convInjectChsMu sync.Mutex
```

- [ ] **Step 2: Add the three methods**

Add after the existing `RemoveConvCancel` method:

```go
// TryClaimConv atomically registers a new inject channel for convID.
// Returns (ch, true) if the conv was free; (nil, false) if already running.
func (a *App) TryClaimConv(convID string) (chan string, bool) {
	a.convInjectChsMu.Lock()
	defer a.convInjectChsMu.Unlock()
	if a.convInjectChs == nil {
		a.convInjectChs = make(map[string]chan string)
	}
	if _, running := a.convInjectChs[convID]; running {
		return nil, false
	}
	ch := make(chan string, 32)
	a.convInjectChs[convID] = ch
	return ch, true
}

// TryInject atomically sends msg to the running agent for convID.
// Returns queued=true on success, full=true if channel at capacity, false/false if no agent running.
func (a *App) TryInject(convID, msg string) (queued bool, full bool) {
	a.convInjectChsMu.Lock()
	defer a.convInjectChsMu.Unlock()
	ch, ok := a.convInjectChs[convID]
	if !ok {
		return false, false
	}
	select {
	case ch <- msg:
		return true, false
	default:
		return false, true
	}
}

// ReleaseConv atomically removes and closes the inject channel for convID.
// Must be called from the agent goroutine's cleanup (not from chatSendMessage).
func (a *App) ReleaseConv(convID string) {
	a.convInjectChsMu.Lock()
	defer a.convInjectChsMu.Unlock()
	if ch, ok := a.convInjectChs[convID]; ok {
		delete(a.convInjectChs, convID)
		close(ch)
	}
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go
git commit -m "feat(mcp): add TryClaimConv/TryInject/ReleaseConv for atomic inject channel management"
```

---

## Task 4: Update `chatSendMessage` to use inject routing

**Files:**
- Modify: `internal/api/chat.go`

- [ ] **Step 1: Replace the `Run` call and goroutine cleanup**

Find `chatSendMessage` in `internal/api/chat.go`. Replace from `a := factory.NewAgent(...)` through the end of the goroutine with:

```go
// Try to inject into a running agent first
if queued, full := app.TryInject(id, req.Content); queued || full {
    if full {
        writeError(w, 429, "message queue full")
        return
    }
    writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
    return
}

// No agent running — try to claim the conv
injectCh, claimed := app.TryClaimConv(id)
if !claimed {
    // Lost the race to another concurrent request — try inject again
    if queued, full := app.TryInject(id, req.Content); queued {
        writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
    } else if full {
        writeError(w, 429, "message queue full")
    } else {
        writeError(w, 503, "agent start conflict, retry")
    }
    return
}

a := factory.NewAgent(id, req.HostIDs)
waiter := agent.NewConfirmationWaiter()
app.StoreChatWaiter(id, waiter)
goroutineLaunched := false
defer func() {
    if !goroutineLaunched {
        app.RemoveChatWaiter(id)
        app.ReleaseConv(id)
    }
}()

app.ConvStore.SetStatus(id, "processing") //nolint:errcheck
parent := app.ShutdownCtx
if parent == nil {
    parent = context.Background()
}
ctx, cancel := context.WithCancel(parent)
app.StoreConvCancel(id, cancel)
events, err := a.Run(ctx, id, content, waiter, injectCh)
if err != nil {
    cancel()
    app.RemoveConvCancel(id)
    app.RemoveChatWaiter(id)
    app.ReleaseConv(id)
    app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
    writeError(w, 500, err.Error())
    return
}

goroutineLaunched = true
go func() {
    defer func() {
        cancel()
        app.RemoveConvCancel(id)
        app.RemoveChatWaiter(id)
        app.ReleaseConv(id)
        app.ClearSSEBuffer(id)
        app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
    }()
    for ev := range events {
        if ev.Type == agent.EventToolStart {
            injectHostNames(app, ev.Content)
        }
        if ev.Type == agent.EventToolStart || ev.Type == agent.EventToolResult {
            name, _ := ev.Content["name"].(string)
            if name == "" {
                name, _ = ev.Content["tool"].(string)
            }
            if name == "Todo" {
                continue
            }
        }
        data, _ := json.Marshal(ev)
        app.BufferSSEEvent(id, data)
        app.BroadcastSSE(id, data)
    }
}()
writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 3: Run all backend tests**

```bash
go test ./... -timeout 60s
```

Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add internal/api/chat.go
git commit -m "feat(api): route queued messages to running agent via TryInject/TryClaimConv"
```

---

## Task 5: Update frontend API types and `sendMessage`

**Files:**
- Modify: `web/src/api/chat.ts`

- [ ] **Step 1: Add `mid_turn_user_message` to `ChatEvent` type**

In `web/src/api/chat.ts`, update the `ChatEvent` interface:

```typescript
export interface ChatEvent {
  type: 'text_delta' | 'tool_start' | 'tool_result' | 'confirm_required' | 'error' | 'done' | 'message' | 'todo_update' | 'turn_usage' | 'mid_turn_user_message'
  content?: Record<string, any>
}
```

- [ ] **Step 2: Update `sendMessage` to return HTTP status**

Replace the existing `sendMessage` function:

```typescript
export function sendMessage(
  conversationId: string,
  content: string,
  hostIds?: string[] | null,
): { controller: AbortController; request: Promise<{ status: 'accepted' | 'queued' }> } {
  const ctrl = new AbortController()
  const body: Record<string, unknown> = { content }
  if (hostIds && hostIds.length > 0) body.host_ids = hostIds
  const request = fetch(`/api/v1/chat/conversations/${conversationId}/messages`, {
    method: 'POST',
    signal: ctrl.signal,
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  }).then(async (res) => {
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw Object.assign(new Error(err.error || res.statusText), { status: res.status })
    }
    const data = await res.json()
    return data as { status: 'accepted' | 'queued' }
  })
  return { controller: ctrl, request }
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/src/api/chat.ts
git commit -m "feat(api-client): add mid_turn_user_message type and expose status from sendMessage"
```

---

## Task 6: Update `ChatView.vue` — queue via HTTP, handle `mid_turn_user_message`

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Update `send()` to send HTTP when streaming**

Find the enqueue block in `send()` (around line 872):

```typescript
// enqueue when streaming (only for direct user input, not flush)
if (isStreaming.value && !overrideText) {
  queuedMessages.value.push(text)
  inputText.value = ''
  nextTick(() => {
    if (textareaRef.value) textareaRef.value.style.height = 'auto'
  })
  return
}
```

Replace with:

```typescript
// When streaming, send HTTP immediately — backend queues it in the running agent
if (isStreaming.value && !overrideText) {
  inputText.value = ''
  nextTick(() => {
    if (textareaRef.value) textareaRef.value.style.height = 'auto'
  })
  const convId = activeConvId.value
  if (!convId) return
  const req = sendMessage(convId, text, selectedHostIds.value)
  req.request
    .then(() => {
      queuedMessages.value.push(text)
    })
    .catch((e: any) => {
      if (e?.status === 429) {
        addSystemMessage('队列已满，请稍后再试')
      } else if (e?.name !== 'AbortError') {
        addSystemMessage(`排队失败：${e?.message || '未知错误'}`)
      }
    })
  return
}
```

- [ ] **Step 2: Add `mid_turn_user_message` handler in `handleConvEvent`**

In `handleConvEvent`, add a new case after `case 'done':` (around line 803):

```typescript
case 'mid_turn_user_message': {
  const text = event.content?.text as string | undefined
  if (!text) break
  // Remove from queued display (match first occurrence)
  const idx = queuedMessages.value.indexOf(text)
  if (idx !== -1) queuedMessages.value.splice(idx, 1)
  // Insert as proper user message in conversation
  const userMsg: DisplayMessage = {
    id: `u-injected-${Date.now()}`,
    role: 'user',
    blocks: [{ type: 'text', content: text }],
  }
  convMsgs.push(userMsg)
  if (activeConvId.value === convId) nextTick(() => scrollToBottom())
  break
}
```

- [ ] **Step 3: Remove `flushQueue` and its call sites**

Delete the `flushQueue` function (lines 943–948):

```typescript
function flushQueue() {
  if (queuedMessages.value.length === 0) return
  const merged = queuedMessages.value.join('\n\n')
  queuedMessages.value = []
  send(merged)
}
```

Replace the two `flushQueue()` call sites (in `case 'error':` and `case 'done':`) with nothing — remove those lines entirely.

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(ui): send queued messages via HTTP, handle mid_turn_user_message SSE event"
```

---

## Task 7: Build and verify end-to-end

**Files:** none (verification only)

- [ ] **Step 1: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build
```

Expected: no errors

- [ ] **Step 2: Build backend with forced embed**

```bash
cd /Users/cw/fty.ai/spider.ai && go build -a -o /tmp/spider-test ./cmd/spider
```

Expected: no errors

- [ ] **Step 3: Start test server**

```bash
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 4: Open browser and test golden path**

Navigate to `http://localhost:8002`. Send a message that triggers multiple tool calls (e.g., a command that runs several SSH operations). While the agent is running:

1. Type a second message and click "排队" — verify it appears in the queued messages list
2. Wait for a tool batch to complete — verify the queued message disappears from the queue and appears as a user message in the conversation
3. Verify the agent responds to the injected message in the next turn

- [ ] **Step 5: Test queue-full behavior**

With an agent running, rapidly send 33+ messages. Verify the 33rd returns a "队列已满" error in the UI.

- [ ] **Step 6: Test cancel behavior**

Start a long-running agent, queue a message, then click cancel. Verify no panic in server logs and the conversation returns to idle state.

- [ ] **Step 7: Run full test suite**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./... -timeout 60s
```

Expected: all pass

- [ ] **Step 8: Final commit**

```bash
git add -p  # stage any remaining changes
git commit -m "chore: verify mid-turn injection end-to-end"
```
