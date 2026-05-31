# Mid-Turn User Message Injection

**Date:** 2026-05-29

## Problem

Current queue behavior: user messages sent while agent is running wait until `EventDone` before being forwarded to the LLM. The LLM cannot act on them until the entire turn completes.

Claude Code reference: after each tool batch completes, queued messages (priority=`next`) are injected into the message history before the next API round-trip. The LLM sees them within the same agent loop.

## Goal

Inject queued user messages into the agent's history after each tool batch, so the LLM can respond to them in the next turn — without waiting for the full agent run to finish.

## Architecture

### Backend: inject channel

`Agent.Run` gains an `injectCh <-chan string` parameter (buffered, size 32).

After each tool batch completes, before the next LLM call, drain all pending messages:

```go
func (a *Agent) Run(
    ctx context.Context,
    convID, userMsg string,
    waiter *ConfirmationWaiter,
    injectCh <-chan string,
) (<-chan Event, error)
```

Drain logic (after `batches` loop, before `continue`):

```go
var parts []string
loop:
for {
    select {
    case msg, ok := <-injectCh:
        if !ok { break loop }
        parts = append(parts, msg)
    default:
        break loop
    }
}
if len(parts) > 0 {
    merged := strings.Join(parts, "\n\n")
    a.msgStore.Save(convID, "user", merged, "")
    history = append(history, llm.Message{Role: llm.RoleUser, Content: merged})
    events <- Event{Type: EventMidTurnUserMessage, Content: map[string]any{"text": merged}}
}
```

New event type:

```go
EventMidTurnUserMessage EventType = "mid_turn_user_message"
```

### Backend: API layer

`App` exposes three atomic operations on a mutex-protected map, eliminating two races:
- **double-start race**: two concurrent requests both see no agent and both try to start one
- **close + write race**: cleanup closes the channel while another request is writing to it

```go
// mcp/server.go — all three ops hold the same mutex

// TryClaimConv atomically checks if a conv is free, and if so registers a new
// inject channel for it. Returns (ch, true) if claimed, (nil, false) if already running.
func (a *App) TryClaimConv(id string) (ch chan string, claimed bool)

// TryInject atomically sends msg to the conv's inject channel.
// Returns queued=true on success, full=true if channel is at capacity, false/false if no agent running.
func (a *App) TryInject(id, msg string) (queued bool, full bool)

// ReleaseConv atomically removes and closes the inject channel.
// Safe to call even if TryInject is racing — close happens under the lock.
func (a *App) ReleaseConv(id string)
```

`chatSendMessage` usage:

```go
// Try to inject into running agent
if queued, full := app.TryInject(id, req.Content); queued || full {
    if full {
        writeError(w, 429, "message queue full")
        return
    }
    writeJSON(w, 202, map[string]string{"status": "queued"})
    return
}

// No agent running — claim the conv and start one
ch, claimed := app.TryClaimConv(id)
if !claimed {
    // Lost the race; another request just started an agent — inject instead
    if queued, full := app.TryInject(id, req.Content); queued {
        writeJSON(w, 202, map[string]string{"status": "queued"})
    } else if full {
        writeError(w, 429, "message queue full")
    } else {
        writeError(w, 503, "agent start conflict, retry")
    }
    return
}
// NOTE: ReleaseConv must be called in the agent goroutine's cleanup, not here.
// Run() is async — chatSendMessage returns immediately after launching the goroutine.

events, err := a.Run(ctx, id, content, waiter, ch)
// ...
go func() {
    defer app.ReleaseConv(id) // called when agent goroutine exits
    for ev := range events { ... }
}()
```

### Frontend: ChatView.vue

**Sending while streaming:** instead of pushing to local `queuedMessages`, send HTTP immediately. On 202 response, add to `queuedMessages` for display. On 429, show "队列已满" error.

**On `mid_turn_user_message` SSE event:**
1. Remove matching text from `queuedMessages`
2. Insert as a proper `DisplayMessage` with `role: 'user'` into the conversation

**`flushQueue` removed** — no longer needed. Queue is managed by backend channel.

## Data Flow

```
User types while streaming
  → send() → HTTP POST /conversations/{id}/messages
  → backend: injectCh <- msg (202) or full (429)
  → frontend: push to queuedMessages (display only)

Tool batch completes
  → backend drains injectCh
  → msgStore.Save("user", merged)
  → history append
  → emit EventMidTurnUserMessage

Frontend receives mid_turn_user_message SSE
  → remove from queuedMessages
  → insert DisplayMessage{role:"user"} into conversation

Next LLM turn
  → LLM sees injected user message in history
  → responds normally via text_delta
```

## Compatibility

- Works with all LLM providers (Claude, OpenAI-compatible). The inject channel operates at the `agent.go` layer; `llm.Client` interface is unchanged.
- Pure-text turns (no tool calls): drain `injectCh` before deciding to emit `EventDone`. If drain yields messages, append to history and **continue the turn loop** instead of exiting. Only emit `EventDone` when drain is empty.

## Edge Cases

| Case | Behavior |
|------|----------|
| Agent finishes before inject arrives | `ReleaseConv` called; next HTTP request starts new agent normally |
| Channel full (32 messages) | HTTP 429; frontend shows error |
| Agent cancelled mid-turn | `ReleaseConv` closes channel under lock; any concurrent `TryInject` sees no entry and returns false/false |
| Multiple tabs | Each tab sends HTTP independently; backend serializes via channel |

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/agent.go` | Add `injectCh` param, drain loop, `EventMidTurnUserMessage` |
| `internal/mcp/server.go` | Add `TryClaimConv`, `TryInject`, `ReleaseConv` |
| `internal/api/chat.go` | Check inject channel before starting new agent |
| `internal/scheduler/executor.go` | Pass `nil` as `injectCh` (headless runs need no injection) |
| `web/src/views/ChatView.vue` | Send HTTP on queue, handle `mid_turn_user_message` event, remove `flushQueue` |
