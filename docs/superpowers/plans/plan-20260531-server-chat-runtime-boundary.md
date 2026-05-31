# Chat Runtime Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract chat, injection, cancellation, queued-message, and SSE runtime state from `mcp.App` into `internal/chatruntime` without changing external behavior.

**Architecture:** Add a focused `chatruntime.Runtime` that owns the mutexes and maps currently embedded in `mcp.App`. `mcp.App` keeps one `ChatRuntime *chatruntime.Runtime` field and retains only a small `BroadcastSSE` delegating method so `agent.Factory.SSEBroadcaster = a` remains unchanged.

**Tech Stack:** Go, `net/http`, `sync.Mutex`, standard `testing`, existing `internal/agent` confirmation waiter and SSE event contracts.

---

## Scope

This plan implements Phase 1 from `docs/superpowers/specs/spec-20260531-server-architecture-optimization.md`.

Do not touch unrelated frontend work. At the time this plan was written, `web/src/api/tokens.ts` had an unrelated working-tree change and must not be staged for this server task.

## File Structure

- Create: `internal/chatruntime/runtime.go`
  - Owns chat waiters, conversation cancellation, injection channels, queued messages, per-conversation SSE clients, per-conversation SSE buffers, and global SSE clients.
- Create: `internal/chatruntime/runtime_test.go`
  - Focused concurrency and lifecycle tests for the runtime package.
- Modify: `internal/mcp/server.go`
  - Remove runtime maps and mutexes from `App`.
  - Add `ChatRuntime *chatruntime.Runtime`.
  - Keep `BroadcastSSE` as a delegate to satisfy `agent.SSEBroadcaster`.
- Modify: `cmd/spider/main.go`
  - Initialize `ChatRuntime: chatruntime.New()` in the main `mcppkg.App` literal.
- Modify: `internal/api/chat.go`
  - Replace chat runtime calls with `app.ChatRuntime.*`.
- Modify: `internal/api/chat_stream.go`
  - Replace SSE client registration calls with `app.ChatRuntime.*`.
- Modify: `internal/api/monitor.go`
  - Replace global SSE calls with `app.ChatRuntime.*`.
- Modify: `internal/api/chat_send_test.go`
  - Initialize `ChatRuntime` in the test app helper.

## Task 1: Add Runtime Tests

**Files:**
- Create: `internal/chatruntime/runtime_test.go`

- [ ] **Step 1: Write failing tests for waiter, cancel, injection, queue, SSE, and global stream behavior**

Create `internal/chatruntime/runtime_test.go` with:

```go
package chatruntime

import (
	"context"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/agent"
)

func TestChatWaiterLifecycle(t *testing.T) {
	rt := New()
	waiter := agent.NewConfirmationWaiter()

	rt.StoreChatWaiter("conv-1", waiter)
	if got := rt.GetChatWaiter("conv-1"); got != waiter {
		t.Fatalf("expected stored waiter, got %#v", got)
	}

	rt.RemoveChatWaiter("conv-1")
	if got := rt.GetChatWaiter("conv-1"); got != nil {
		t.Fatalf("expected waiter removal, got %#v", got)
	}
}

func TestConversationCancelLifecycle(t *testing.T) {
	rt := New()
	called := make(chan struct{})
	cancel := func() { close(called) }

	rt.StoreConvCancel("conv-1", cancel)
	if !rt.CancelConv("conv-1") {
		t.Fatal("expected cancel to return true")
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("stored cancel was not called")
	}
	if rt.CancelConv("conv-1") {
		t.Fatal("expected second cancel to return false after removal")
	}
}

func TestTryClaimInjectConsumeAndRelease(t *testing.T) {
	rt := New()

	ch, ok := rt.TryClaimConv("conv-1")
	if !ok {
		t.Fatal("expected first claim to succeed")
	}
	if _, ok := rt.TryClaimConv("conv-1"); ok {
		t.Fatal("expected second claim to fail while conversation is running")
	}

	if queued, full := rt.TryInject("conv-1", "one"); !queued || full {
		t.Fatalf("expected first inject queued=true full=false, got queued=%v full=%v", queued, full)
	}
	if queued, full := rt.TryInject("conv-1", "two\n\nwith break"); !queued || full {
		t.Fatalf("expected second inject queued=true full=false, got queued=%v full=%v", queued, full)
	}
	if queued, full := rt.TryInject("conv-1", "three"); !queued || full {
		t.Fatalf("expected third inject queued=true full=false, got queued=%v full=%v", queued, full)
	}

	if got := rt.GetQueuedMsgs("conv-1"); len(got) != 3 || got[0] != "one" || got[1] != "two\n\nwith break" || got[2] != "three" {
		t.Fatalf("unexpected queued messages: %#v", got)
	}

	rt.ConsumeQueuedMsgs("conv-1", 2)
	if got := rt.GetQueuedMsgs("conv-1"); len(got) != 1 || got[0] != "three" {
		t.Fatalf("expected only third message after count-based consume, got %#v", got)
	}

	rt.ReleaseConv("conv-1")
	if queued, full := rt.TryInject("conv-1", "after-release"); queued || full {
		t.Fatalf("expected inject after release to miss running conversation, got queued=%v full=%v", queued, full)
	}
	if got := rt.GetQueuedMsgs("conv-1"); got != nil {
		t.Fatalf("expected release to clear queue, got %#v", got)
	}
	select {
	case _, open := <-ch:
		if open {
			t.Fatal("expected release to close injection channel")
		}
	case <-time.After(time.Second):
		t.Fatal("expected released injection channel to close")
	}
}

func TestTryInjectFull(t *testing.T) {
	rt := New()
	if _, ok := rt.TryClaimConv("conv-1"); !ok {
		t.Fatal("expected claim to succeed")
	}

	for i := 0; i < 32; i++ {
		if queued, full := rt.TryInject("conv-1", "msg"); !queued || full {
			t.Fatalf("inject %d expected queued=true full=false, got queued=%v full=%v", i, queued, full)
		}
	}
	if queued, full := rt.TryInject("conv-1", "overflow"); queued || !full {
		t.Fatalf("expected overflow to return queued=false full=true, got queued=%v full=%v", queued, full)
	}
}

func TestRegisterSSEClientAndDrain(t *testing.T) {
	rt := New()
	rt.BufferSSEEvent("conv-1", []byte("old-1"))
	rt.BufferSSEEvent("conv-1", []byte("old-2"))

	ch := make(chan []byte, 2)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", ch)
	if len(buffered) != 2 || string(buffered[0]) != "old-1" || string(buffered[1]) != "old-2" {
		t.Fatalf("unexpected drained buffer: %#v", buffered)
	}

	buffered = rt.RegisterSSEClientAndDrain("conv-1", make(chan []byte, 1))
	if len(buffered) != 0 {
		t.Fatalf("expected second drain to be empty, got %#v", buffered)
	}
}

func TestBufferAndBroadcastSSE(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)
	buffered := rt.RegisterSSEClientAndDrain("conv-1", ch)
	if len(buffered) != 0 {
		t.Fatalf("expected empty initial buffer, got %#v", buffered)
	}

	rt.BufferAndBroadcastSSE("conv-1", []byte("live-1"))
	select {
	case got := <-ch:
		if string(got) != "live-1" {
			t.Fatalf("unexpected live event: %q", string(got))
		}
	case <-time.After(time.Second):
		t.Fatal("expected live event")
	}

	reconnect := make(chan []byte, 1)
	buffered = rt.RegisterSSEClientAndDrain("conv-1", reconnect)
	if len(buffered) != 1 || string(buffered[0]) != "live-1" {
		t.Fatalf("expected buffered live event for reconnect, got %#v", buffered)
	}
}

func TestUnregisterSSEClient(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)
	rt.RegisterSSEClientAndDrain("conv-1", ch)
	rt.UnregisterSSEClient("conv-1", ch)

	rt.BroadcastSSE("conv-1", []byte("ignored"))
	select {
	case got := <-ch:
		t.Fatalf("unregistered client received event %q", string(got))
	default:
	}
}

func TestGlobalSSEClients(t *testing.T) {
	rt := New()
	ch := make(chan []byte, 1)

	rt.AddGlobalSSEClient(ch)
	rt.BroadcastGlobalSSE([]byte("global-1"))
	select {
	case got := <-ch:
		if string(got) != "global-1" {
			t.Fatalf("unexpected global event: %q", string(got))
		}
	case <-time.After(time.Second):
		t.Fatal("expected global event")
	}

	rt.RemoveGlobalSSEClient(ch)
	rt.BroadcastGlobalSSE([]byte("global-2"))
	select {
	case got := <-ch:
		t.Fatalf("removed global client received event %q", string(got))
	default:
	}
}

func TestCancelRemoveWithoutCalling(t *testing.T) {
	rt := New()
	ctx, cancel := context.WithCancel(context.Background())
	rt.StoreConvCancel("conv-1", cancel)
	rt.RemoveConvCancel("conv-1")

	select {
	case <-ctx.Done():
		t.Fatal("RemoveConvCancel must not call cancel")
	default:
	}
	if rt.CancelConv("conv-1") {
		t.Fatal("expected removed cancel to be absent")
	}
}
```

- [ ] **Step 2: Run tests to verify the new package does not compile yet**

Run:

```sh
go test ./internal/chatruntime
```

Expected result:

```text
FAIL    github.com/spiderai/spider/internal/chatruntime [setup failed]
```

The package should fail because `runtime.go` and `New` do not exist yet.

## Task 2: Implement `internal/chatruntime.Runtime`

**Files:**
- Create: `internal/chatruntime/runtime.go`
- Test: `internal/chatruntime/runtime_test.go`

- [ ] **Step 1: Create the runtime implementation**

Create `internal/chatruntime/runtime.go` with:

```go
package chatruntime

import (
	"context"
	"sync"

	"github.com/spiderai/spider/internal/agent"
)

const maxSSEBufferEvents = 500

type Runtime struct {
	chatWaiters   map[string]*agent.ConfirmationWaiter
	chatWaitersMu sync.Mutex

	convCancels   map[string]context.CancelFunc
	convCancelsMu sync.Mutex

	convInjectChs   map[string]chan string
	convQueuedMsgs  map[string][]string
	convInjectChsMu sync.Mutex

	sseClients   map[string][]chan []byte
	sseClientsMu sync.Mutex

	sseBuffers   map[string][][]byte
	sseBuffersMu sync.Mutex

	globalSSEClients   []chan []byte
	globalSSEClientsMu sync.Mutex
}

func New() *Runtime {
	return &Runtime{}
}

func (r *Runtime) StoreChatWaiter(convID string, w *agent.ConfirmationWaiter) {
	r.chatWaitersMu.Lock()
	defer r.chatWaitersMu.Unlock()
	if r.chatWaiters == nil {
		r.chatWaiters = make(map[string]*agent.ConfirmationWaiter)
	}
	r.chatWaiters[convID] = w
}

func (r *Runtime) GetChatWaiter(convID string) *agent.ConfirmationWaiter {
	r.chatWaitersMu.Lock()
	defer r.chatWaitersMu.Unlock()
	return r.chatWaiters[convID]
}

func (r *Runtime) RemoveChatWaiter(convID string) {
	r.chatWaitersMu.Lock()
	defer r.chatWaitersMu.Unlock()
	delete(r.chatWaiters, convID)
}

func (r *Runtime) StoreConvCancel(convID string, cancel context.CancelFunc) {
	r.convCancelsMu.Lock()
	defer r.convCancelsMu.Unlock()
	if r.convCancels == nil {
		r.convCancels = make(map[string]context.CancelFunc)
	}
	r.convCancels[convID] = cancel
}

func (r *Runtime) CancelConv(convID string) bool {
	r.convCancelsMu.Lock()
	defer r.convCancelsMu.Unlock()
	cancel, ok := r.convCancels[convID]
	if ok {
		cancel()
		delete(r.convCancels, convID)
	}
	return ok
}

func (r *Runtime) RemoveConvCancel(convID string) {
	r.convCancelsMu.Lock()
	defer r.convCancelsMu.Unlock()
	delete(r.convCancels, convID)
}

func (r *Runtime) TryClaimConv(convID string) (chan string, bool) {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	if r.convInjectChs == nil {
		r.convInjectChs = make(map[string]chan string)
	}
	if _, running := r.convInjectChs[convID]; running {
		return nil, false
	}
	ch := make(chan string, 32)
	r.convInjectChs[convID] = ch
	return ch, true
}

func (r *Runtime) TryInject(convID, msg string) (queued bool, full bool) {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	ch, ok := r.convInjectChs[convID]
	if !ok {
		return false, false
	}
	select {
	case ch <- msg:
		if r.convQueuedMsgs == nil {
			r.convQueuedMsgs = make(map[string][]string)
		}
		r.convQueuedMsgs[convID] = append(r.convQueuedMsgs[convID], msg)
		return true, false
	default:
		return false, true
	}
}

func (r *Runtime) ConsumeQueuedMsgs(convID string, n int) {
	if n <= 0 {
		return
	}
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	queue := r.convQueuedMsgs[convID]
	if len(queue) == 0 {
		return
	}
	if n >= len(queue) {
		delete(r.convQueuedMsgs, convID)
	} else {
		r.convQueuedMsgs[convID] = queue[n:]
	}
}

func (r *Runtime) GetQueuedMsgs(convID string) []string {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	q := r.convQueuedMsgs[convID]
	if len(q) == 0 {
		return nil
	}
	out := make([]string, len(q))
	copy(out, q)
	return out
}

func (r *Runtime) ReleaseConv(convID string) {
	r.convInjectChsMu.Lock()
	defer r.convInjectChsMu.Unlock()
	if ch, ok := r.convInjectChs[convID]; ok {
		delete(r.convInjectChs, convID)
		close(ch)
	}
	delete(r.convQueuedMsgs, convID)
}

func (r *Runtime) UnregisterSSEClient(convID string, ch chan []byte) {
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	clients := r.sseClients[convID]
	for i, c := range clients {
		if c == ch {
			r.sseClients[convID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(r.sseClients[convID]) == 0 {
		delete(r.sseClients, convID)
	}
}

func (r *Runtime) BroadcastSSE(convID string, data []byte) {
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	for _, ch := range r.sseClients[convID] {
		select {
		case ch <- data:
		default:
		}
	}
}

func (r *Runtime) BufferAndBroadcastSSE(convID string, data []byte) {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	if r.sseBuffers == nil {
		r.sseBuffers = make(map[string][][]byte)
	}
	buf := r.sseBuffers[convID]
	if len(buf) >= maxSSEBufferEvents {
		r.sseBuffers[convID] = append(buf[1:], data)
	} else {
		r.sseBuffers[convID] = append(buf, data)
	}
	for _, ch := range r.sseClients[convID] {
		select {
		case ch <- data:
		default:
		}
	}
}

func (r *Runtime) BufferSSEEvent(convID string, data []byte) {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	if r.sseBuffers == nil {
		r.sseBuffers = make(map[string][][]byte)
	}
	buf := r.sseBuffers[convID]
	if len(buf) >= maxSSEBufferEvents {
		r.sseBuffers[convID] = append(buf[1:], data)
		return
	}
	r.sseBuffers[convID] = append(buf, data)
}

func (r *Runtime) RegisterSSEClientAndDrain(convID string, ch chan []byte) [][]byte {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	r.sseClientsMu.Lock()
	defer r.sseClientsMu.Unlock()
	if r.sseClients == nil {
		r.sseClients = make(map[string][]chan []byte)
	}
	r.sseClients[convID] = append(r.sseClients[convID], ch)
	buf := r.sseBuffers[convID]
	delete(r.sseBuffers, convID)
	return buf
}

func (r *Runtime) ClearSSEBuffer(convID string) {
	r.sseBuffersMu.Lock()
	defer r.sseBuffersMu.Unlock()
	delete(r.sseBuffers, convID)
}

func (r *Runtime) AddGlobalSSEClient(ch chan []byte) {
	r.globalSSEClientsMu.Lock()
	defer r.globalSSEClientsMu.Unlock()
	r.globalSSEClients = append(r.globalSSEClients, ch)
}

func (r *Runtime) RemoveGlobalSSEClient(ch chan []byte) {
	r.globalSSEClientsMu.Lock()
	defer r.globalSSEClientsMu.Unlock()
	clients := make([]chan []byte, 0, len(r.globalSSEClients))
	for _, c := range r.globalSSEClients {
		if c != ch {
			clients = append(clients, c)
		}
	}
	r.globalSSEClients = clients
}

func (r *Runtime) BroadcastGlobalSSE(data []byte) {
	r.globalSSEClientsMu.Lock()
	defer r.globalSSEClientsMu.Unlock()
	for _, ch := range r.globalSSEClients {
		select {
		case ch <- data:
		default:
		}
	}
}
```

- [ ] **Step 2: Run runtime tests**

Run:

```sh
go test ./internal/chatruntime
```

Expected result:

```text
ok  	github.com/spiderai/spider/internal/chatruntime
```

- [ ] **Step 3: Commit runtime package**

Run:

```sh
git add internal/chatruntime/runtime.go internal/chatruntime/runtime_test.go
git commit -m "refactor(chat): add runtime package"
```

## Task 3: Wire Runtime Into `mcp.App`

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`
- Test: `internal/mcp` compile through package tests

- [ ] **Step 1: Update `internal/mcp/server.go` imports**

Remove:

```go
	"sync"
```

Add:

```go
	"github.com/spiderai/spider/internal/chatruntime"
```

- [ ] **Step 2: Replace runtime fields on `App`**

In `type App struct`, delete these fields:

```go
	chatWaiters   map[string]*agent.ConfirmationWaiter
	chatWaitersMu sync.Mutex

	convCancels   map[string]context.CancelFunc
	convCancelsMu sync.Mutex

	convInjectChs    map[string]chan string
	convQueuedMsgs   map[string][]string
	convInjectChsMu  sync.Mutex

	sseClients   map[string][]chan []byte
	sseClientsMu sync.Mutex

	sseBuffers   map[string][][]byte
	sseBuffersMu sync.Mutex

	globalSSEClients   []chan []byte
	globalSSEClientsMu sync.Mutex
```

Add this field near `ShutdownCtx`:

```go
	ChatRuntime *chatruntime.Runtime
```

- [ ] **Step 3: Remove moved runtime methods from `internal/mcp/server.go`**

Delete the `App` methods that are now owned by `chatruntime.Runtime`:

```go
StoreChatWaiter
GetChatWaiter
RemoveChatWaiter
StoreConvCancel
CancelConv
RemoveConvCancel
TryClaimConv
TryInject
ConsumeQueuedMsgs
GetQueuedMsgs
ReleaseConv
UnregisterSSEClient
BufferAndBroadcastSSE
BufferSSEEvent
RegisterSSEClientAndDrain
ClearSSEBuffer
AddGlobalSSEClient
RemoveGlobalSSEClient
BroadcastGlobalSSE
```

Keep `BroadcastSSE` on `mcp.App`, but replace its body with:

```go
func (a *App) BroadcastSSE(convID string, data []byte) {
	if a.ChatRuntime == nil {
		return
	}
	a.ChatRuntime.BroadcastSSE(convID, data)
}
```

Do not change `NewAgentFactory`; it should keep:

```go
	f.SSEBroadcaster = a
```

- [ ] **Step 4: Initialize the runtime in `cmd/spider/main.go`**

Add the import:

```go
	"github.com/spiderai/spider/internal/chatruntime"
```

In the `app := &mcppkg.App{...}` literal, add:

```go
		ChatRuntime:     chatruntime.New(),
```

- [ ] **Step 5: Run package tests to catch compile errors**

Run:

```sh
go test ./internal/mcp ./cmd/spider
```

Expected result:

```text
?   	github.com/spiderai/spider/internal/mcp	[no test files]
ok  	github.com/spiderai/spider/cmd/spider
```

If `cmd/spider` reports `[no test files]`, that is acceptable.

- [ ] **Step 6: Commit `mcp.App` wiring**

Run:

```sh
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "refactor(chat): move runtime state out of app"
```

## Task 4: Update API Call Sites

**Files:**
- Modify: `internal/api/chat.go`
- Modify: `internal/api/chat_stream.go`
- Modify: `internal/api/monitor.go`
- Modify: `internal/api/chat_send_test.go`

- [ ] **Step 1: Update `chat.go` runtime calls**

Replace these calls:

```go
app.GetQueuedMsgs(id)
app.TryInject(id, content)
app.TryClaimConv(id)
app.ReleaseConv(id)
app.StoreChatWaiter(id, waiter)
app.RemoveChatWaiter(id)
app.StoreConvCancel(id, cancel)
app.RemoveConvCancel(id)
app.ClearSSEBuffer(id)
app.ConsumeQueuedMsgs(id, count)
app.BufferAndBroadcastSSE(id, data)
app.CancelConv(id)
app.GetChatWaiter(convID)
```

with:

```go
app.ChatRuntime.GetQueuedMsgs(id)
app.ChatRuntime.TryInject(id, content)
app.ChatRuntime.TryClaimConv(id)
app.ChatRuntime.ReleaseConv(id)
app.ChatRuntime.StoreChatWaiter(id, waiter)
app.ChatRuntime.RemoveChatWaiter(id)
app.ChatRuntime.StoreConvCancel(id, cancel)
app.ChatRuntime.RemoveConvCancel(id)
app.ChatRuntime.ClearSSEBuffer(id)
app.ChatRuntime.ConsumeQueuedMsgs(id, count)
app.ChatRuntime.BufferAndBroadcastSSE(id, data)
app.ChatRuntime.CancelConv(id)
app.ChatRuntime.GetChatWaiter(convID)
```

The cancel handler must keep the same ordering:

```go
app.ChatRuntime.CancelConv(id)
app.ChatRuntime.ReleaseConv(id)
app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
```

- [ ] **Step 2: Update `chat_stream.go` runtime calls**

Replace:

```go
buffered := app.RegisterSSEClientAndDrain(id, ch)
defer app.UnregisterSSEClient(id, ch)
```

with:

```go
buffered := app.ChatRuntime.RegisterSSEClientAndDrain(id, ch)
defer app.ChatRuntime.UnregisterSSEClient(id, ch)
```

- [ ] **Step 3: Update `monitor.go` global stream calls**

Replace:

```go
app.AddGlobalSSEClient(ch)
defer app.RemoveGlobalSSEClient(ch)
```

with:

```go
app.ChatRuntime.AddGlobalSSEClient(ch)
defer app.ChatRuntime.RemoveGlobalSSEClient(ch)
```

- [ ] **Step 4: Initialize `ChatRuntime` in `newChatSendTestApp`**

In `internal/api/chat_send_test.go`, add the import:

```go
	"github.com/spiderai/spider/internal/chatruntime"
```

In the returned `mcppkg.App` literal, add:

```go
		ChatRuntime:     chatruntime.New(),
```

- [ ] **Step 5: Run API tests**

Run:

```sh
go test ./internal/api
```

Expected result:

```text
ok  	github.com/spiderai/spider/internal/api
```

- [ ] **Step 6: Commit API call-site migration**

Run:

```sh
git add internal/api/chat.go internal/api/chat_stream.go internal/api/monitor.go internal/api/chat_send_test.go
git commit -m "refactor(api): use chat runtime"
```

## Task 5: Verify Runtime Boundary and Race-Sensitive Paths

**Files:**
- Verify: `internal/mcp/server.go`
- Verify: `internal/chatruntime/runtime.go`
- Verify: `internal/api/chat.go`
- Verify: `internal/api/chat_stream.go`
- Verify: `internal/api/monitor.go`

- [ ] **Step 1: Confirm `mcp.App` no longer owns runtime mutexes or maps**

Run:

```sh
rg -n "chatWaiters|convCancels|convInjectChs|convQueuedMsgs|sseClients|sseBuffers|globalSSEClients|sync\\.Mutex" internal/mcp/server.go
```

Expected result:

```text
```

No matches should remain in `internal/mcp/server.go`.

- [ ] **Step 2: Confirm runtime methods live in the new package**

Run:

```sh
rg -n "func \\(r \\*Runtime\\) (TryClaimConv|TryInject|ConsumeQueuedMsgs|BufferAndBroadcastSSE|RegisterSSEClientAndDrain|AddGlobalSSEClient)" internal/chatruntime/runtime.go
```

Expected result includes all six method declarations:

```text
internal/chatruntime/runtime.go:...
```

- [ ] **Step 3: Run focused tests**

Run:

```sh
go test ./internal/chatruntime ./internal/api ./internal/agent ./internal/mcp
```

Expected result:

```text
ok  	github.com/spiderai/spider/internal/chatruntime
ok  	github.com/spiderai/spider/internal/api
ok  	github.com/spiderai/spider/internal/agent
?   	github.com/spiderai/spider/internal/mcp	[no test files]
```

- [ ] **Step 4: Run race tests for runtime and API**

Run:

```sh
go test -race ./internal/chatruntime ./internal/api
```

Expected result:

```text
ok  	github.com/spiderai/spider/internal/chatruntime
ok  	github.com/spiderai/spider/internal/api
```

- [ ] **Step 5: Run full test suite**

Run:

```sh
go test ./...
```

Expected result:

```text
ok  	github.com/spiderai/spider/...
```

Some packages may print `[no test files]`; that is acceptable.

- [ ] **Step 6: Commit any verification-only fixes**

If the verification commands forced small compile or test fixes, commit only those touched files:

```sh
git add internal/chatruntime internal/mcp/server.go cmd/spider/main.go internal/api/chat.go internal/api/chat_stream.go internal/api/monitor.go internal/api/chat_send_test.go
git commit -m "fix(chat): complete runtime extraction"
```

If there are no changes after verification, skip this commit.

## Task 6: Final Status Check

**Files:**
- Verify: repository status

- [ ] **Step 1: Check final status**

Run:

```sh
git status --short
```

Expected result:

```text
 M web/src/api/tokens.ts
?? .claude/skills/
?? docs/superpowers/plans/2026-05-29-mid-turn-user-message-injection.md
?? docs/superpowers/plans/2026-05-31-remove-polling-sse-only.md
?? docs/superpowers/specs/2026-05-29-mid-turn-user-message-injection-design.md
```

The exact unrelated files may differ if the user changed the workspace during execution. The important condition is that no `internal/chatruntime`, `internal/mcp`, `internal/api`, or `cmd/spider` changes remain unstaged or uncommitted.

- [ ] **Step 2: Report completion**

Report:

```text
Phase 1 complete. Chat/SSE runtime state now lives in internal/chatruntime, mcp.App delegates BroadcastSSE for agent compatibility, and API handlers use app.ChatRuntime directly. Tests run: go test ./internal/chatruntime ./internal/api ./internal/agent ./internal/mcp; go test -race ./internal/chatruntime ./internal/api; go test ./...
```

## Self-Review

- Spec coverage: Phase 1 package, API shape, count-based queue consumption, atomic `BufferAndBroadcastSSE`, global SSE names, `mcp.App.BroadcastSSE` delegation, and tests are covered.
- Placeholder scan: This plan uses concrete file paths, commands, expected outputs, and code snippets. It avoids open-ended implementation instructions.
- Type consistency: Method names match the corrected spec: `ConsumeQueuedMsgs(convID string, n int)`, `BufferSSEEvent`, `BufferAndBroadcastSSE`, `AddGlobalSSEClient`, `RemoveGlobalSSEClient`, and `BroadcastGlobalSSE`.
