# SSE Event Buffer on Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a user refreshes the page during an active agent run, the frontend reconnects via SSE and replays all streaming events (text_delta, tool_start, tool_result, done) that have already been broadcast, restoring the in-progress streaming view.

**Architecture:** Add an in-memory per-conversation event buffer (`sseEventBuf`) to `App`. Every `BroadcastSSE` call appends to the buffer. When a new SSE client connects to a processing conversation, `chatStreamGet` replays the buffer before subscribing to live events. The buffer is cleared when the conversation transitions to idle (done, cancel, or error).

**Tech Stack:** Go, existing `internal/mcp/server.go`, `internal/api/chat_stream.go`, `internal/api/chat.go`

---

### Task 1: Add event buffer to App and wire BroadcastSSE

**Files:**
- Modify: `internal/mcp/server.go`

- [ ] **Step 1: Add buffer field and mutex to App struct**

In `internal/mcp/server.go`, add two fields after the `sseClients` block (around line 70):

```go
sseEventBuf   map[string][]json.RawMessage // convID -> buffered events
sseEventBufMu sync.Mutex
```

Also add `"encoding/json"` to the import block — it is not currently imported in `server.go`.

- [ ] **Step 2: Add ClearSSEBuf and ReplayBufAndRegister methods**

Add after `BroadcastSSE` (around line 220):

```go
// ClearSSEBuf discards the event buffer for a conversation (call when idle).
func (a *App) ClearSSEBuf(convID string) {
	a.sseEventBufMu.Lock()
	defer a.sseEventBufMu.Unlock()
	delete(a.sseEventBuf, convID)
}

// ReplayBufAndRegister replays buffered events via fn, then registers ch as a live
// SSE client — all while holding sseEventBufMu so no event can slip between replay
// and registration. BroadcastSSE also holds sseEventBufMu, so there is no race.
func (a *App) ReplayBufAndRegister(convID string, ch chan []byte, fn func([]byte)) {
	a.sseEventBufMu.Lock()
	defer a.sseEventBufMu.Unlock()
	for _, raw := range a.sseEventBuf[convID] {
		fn(raw)
	}
	a.RegisterSSEClient(convID, ch)
}
```

- [ ] **Step 3: Update BroadcastSSE to append to buffer**

Replace the existing `BroadcastSSE` method body (around line 210):

```go
func (a *App) BroadcastSSE(convID string, data []byte) {
	// Append to replay buffer
	a.sseEventBufMu.Lock()
	if a.sseEventBuf == nil {
		a.sseEventBuf = make(map[string][]json.RawMessage)
	}
	a.sseEventBuf[convID] = append(a.sseEventBuf[convID], json.RawMessage(data))
	a.sseEventBufMu.Unlock()

	// Broadcast to live clients
	a.sseClientsMu.Lock()
	defer a.sseClientsMu.Unlock()
	for _, ch := range a.sseClients[convID] {
		select {
		case ch <- data:
		default:
		}
	}
}
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go
git commit -m "feat(sse): add per-conv event buffer to App"
```

---

### Task 2: Clear buffer when conversation goes idle

**Files:**
- Modify: `internal/api/chat.go`

The buffer must be cleared at every point where `SetStatus("idle")` is called, so replaying stale events from a previous run is impossible.

- [ ] **Step 1: Identify the three idle transitions**

In `internal/api/chat.go`, the three locations are:
- Line ~200: after agent run completes normally (inside `chatSendMessage`)
- Line ~230: early error path (inside `chatSendMessage`)
- Line ~294: `chatCancel`

- [ ] **Step 2: Add ClearSSEBuf calls at all three locations**

At line ~200 (normal completion — after the `for ev := range events` loop):
```go
app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
app.ClearSSEBuf(id)
```

At line ~230 (error path — `if err != nil` block after `a.Run`):
```go
app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
app.ClearSSEBuf(id)
writeError(w, 500, err.Error())
return
```

At line ~294 (cancel — inside `chatCancel`):
```go
app.CancelConv(id)
app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
app.ClearSSEBuf(id)
writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
```

- [ ] **Step 3: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/api/chat.go
git commit -m "feat(sse): clear event buffer when conversation goes idle"
```

---

### Task 3: Replay buffer in chatStreamGet

**Files:**
- Modify: `internal/api/chat_stream.go`

Replace the separate `RegisterSSEClient` call with `ReplayBufAndRegister`. This atomically replays buffered events and registers the channel, closing the race window where an event could be broadcast after replay but before registration.

- [ ] **Step 1: Replace RegisterSSEClient with ReplayBufAndRegister**

Replace this block (lines 70-73):

```go
// Subscribe to live updates
ch := make(chan []byte, 10)
app.RegisterSSEClient(id, ch)
defer app.UnregisterSSEClient(id, ch)
```

With:

```go
// Replay buffer and register atomically to avoid missing events between replay and subscribe.
ch := make(chan []byte, 10)
app.ReplayBufAndRegister(id, ch, func(raw []byte) {
    fmt.Fprintf(w, "data: %s\n\n", raw)
    if flusher != nil {
        flusher.Flush()
    }
})
defer app.UnregisterSSEClient(id, ch)
```

The full function after the change:

```go
func chatStreamGet(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    _, err := verifyConvOwner(app, r, id)
    if err != nil {
        writeError(w, 404, "conversation not found")
        return
    }

    lastEventID := 0
    rawID := r.URL.Query().Get("last_event_id")
    if rawID == "" {
        rawID = r.Header.Get("Last-Event-ID")
    }
    if rawID != "" {
        if n, err := strconv.Atoi(rawID); err == nil {
            lastEventID = n
        }
    }

    msgs, err := app.MsgStore.ListByConversation(id)
    if err != nil {
        writeError(w, 500, err.Error())
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher, _ := w.(http.Flusher)

    // Replay DB messages after last_event_id
    for i, msg := range msgs {
        if i <= lastEventID {
            continue
        }
        event := map[string]any{
            "type": "message",
            "content": map[string]any{
                "id":              msg.ID,
                "conversation_id": msg.ConversationID,
                "role":            msg.Role,
                "content":         msg.Content,
                "tool_calls":      msg.ToolCalls,
                "created_at":      msg.CreatedAt,
            },
        }
        data, _ := json.Marshal(event)
        fmt.Fprintf(w, "id: %d\ndata: %s\n\n", i, data)
        if flusher != nil {
            flusher.Flush()
        }
    }

    // Replay in-memory buffer and register atomically — no event can slip between.
    ch := make(chan []byte, 10)
    app.ReplayBufAndRegister(id, ch, func(raw []byte) {
        fmt.Fprintf(w, "data: %s\n\n", raw)
        if flusher != nil {
            flusher.Flush()
        }
    })
    defer app.UnregisterSSEClient(id, ch)

    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case data := <-ch:
            fmt.Fprintf(w, "data: %s\n\n", data)
            if flusher != nil {
                flusher.Flush()
            }
        case <-ticker.C:
            fmt.Fprintf(w, ": keepalive\n\n")
            if flusher != nil {
                flusher.Flush()
            }
        }
    }
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/api/chat_stream.go
git commit -m "feat(sse): replay event buffer on reconnect for refresh resilience"
```

---

### Task 4: Manual verification

- [ ] **Step 1: Build and start server**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: Start a long-running agent turn**

Open `http://localhost:8002`, send a message that triggers a multi-step agent run (e.g., a command that takes several seconds).

- [ ] **Step 3: Refresh mid-stream**

While text is streaming or a tool call is in progress, press Cmd+R (hard refresh).

Expected: page reloads, reconnects, and immediately shows all text and tool calls that had already been output — streaming continues from where it left off.

- [ ] **Step 4: Verify buffer clears after done**

Wait for the agent run to complete. Refresh again.

Expected: no duplicate streaming events — only the final DB messages are shown (via the normal `message` replay path).

- [ ] **Step 5: Verify cancel clears buffer**

Start another run, cancel it mid-stream, then refresh.

Expected: no streaming events replayed — only the messages saved to DB before cancel.
