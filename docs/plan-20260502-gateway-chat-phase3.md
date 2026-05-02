# Gateway Chat Phase 3: API Handlers + Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire up the Agent Engine (Phase 2) to HTTP API endpoints and build the Vue frontend ChatView with terminal-style UI, target panel, and SSE streaming.

**Architecture:** Backend registers chat routes on `mux`, handlers create Agent per request, stream events as SSE via `text/event-stream`. Frontend uses `fetch` + `ReadableStream` for POST-based SSE (not EventSource, which is GET-only). Vue SFC with `<script setup lang="ts">`.

**Tech Stack:** Go 1.23, existing `internal/api` patterns, Vue 3.4, vue-router 4, marked + shiki (already in deps)

**Spec Reference:** `docs/spec-20260502-gateway-chat.md` sections 7, 9, 10, 12

**Phase 1+2 Dependencies:**
- `internal/agent` — Agent, AgentConfig, ConfirmationWaiter, NewAgent, tool constructors
- `internal/llm` — Client, NewClient
- `internal/rag` — NewStore, NewEmbedder
- `internal/store` — ConversationStore, MessageStore, DocumentStore
- `internal/mcp` — App struct (needs extension with new stores + agent deps)
- `web/src/api/auth.ts` — authHeaders()
- `web/src/composables/useAuth.ts` — useAuth()

---

### Task 1: Extend App Struct + Agent Factory

**Files:**
- Modify: `internal/mcp/server.go` (App struct)
- Create: `internal/agent/factory.go`

<!-- PLACEHOLDER_TASK1_CONTINUE -->

- [ ] **Step 1: Add chat stores to App struct**

In `internal/mcp/server.go`, add fields to App:
```go
ConvStore    *store.ConversationStore
MsgStore     *store.MessageStore
DocStore     *store.DocumentStore
AgentFactory *agent.Factory  // nil if LLM not configured
```

Initialize after DB open:
```go
app.ConvStore = store.NewConversationStore(app.DB)
app.MsgStore = store.NewMessageStore(app.DB)
app.DocStore = store.NewDocumentStore(app.DB)
```

- [ ] **Step 2: Create agent factory**

`internal/agent/factory.go` — Factory struct holds shared deps (LLMClient, RAGStore, Hosts, SSHPool, SSHKeys, Logs, MsgStore). `NewFactory(cfg, db, hosts, sshPool, sshKeys, logs, msgStore, docStore)` creates LLM client from active model, optionally creates RAG store if embedding configured. `NewAgent()` method creates fresh Agent with all tools registered + DefaultRiskHook + system prompt.

- [ ] **Step 3: Implement BuildSystemPrompt**

In factory.go — query hosts.List(""), count by vendor, build prompt describing available hosts, tools, and capabilities. Template:
```
You are a network operations assistant managing {N} gateway devices.
Vendors: {vendor: count, ...}
Available tools: get_device_info, execute_cli, batch_execute, verify, search_docs, call_rest_api
...
```

- [ ] **Step 4: Initialize factory in App setup**

After stores init, attempt `agent.NewFactory(...)`. If LLM not configured, log warning, leave AgentFactory nil. Chat handlers check nil before use.

- [ ] **Step 5: Build + commit**

Run: `go build ./...`
Commit: `feat(agent): add factory with tool registration and system prompt`

---

### Task 2: Chat API Handlers — CRUD + SSE Streaming

**Files:**
- Create: `internal/api/chat.go`
- Modify: `internal/api/handler.go` (register routes)

- [ ] **Step 1: Register chat routes**

In `handler.go`, add inside `NewRouter`:
```go
mux.HandleFunc("POST /api/v1/chat/conversations", chatCreateConversation(app))
mux.HandleFunc("GET /api/v1/chat/conversations", chatListConversations(app))
mux.HandleFunc("GET /api/v1/chat/conversations/{id}", chatGetConversation(app))
mux.HandleFunc("DELETE /api/v1/chat/conversations/{id}", chatDeleteConversation(app))
mux.HandleFunc("PATCH /api/v1/chat/conversations/{id}", chatUpdateTitle(app))
mux.HandleFunc("POST /api/v1/chat/conversations/{id}/messages", chatSendMessage(app))
mux.HandleFunc("POST /api/v1/chat/conversations/{id}/confirm/{requestId}", chatConfirm(app))
```

- [ ] **Step 2: Implement CRUD handlers**

`internal/api/chat.go`:

`chatCreateConversation` — decode `{title}`, get userID from `authmw.GetUser(r.Context())`, call `app.ConvStore.Create(userID, title)`, return JSON.

`chatListConversations` — get userID, call `app.ConvStore.ListByUser(userID)`, return JSON array.

`chatGetConversation` — get id from `r.PathValue("id")`, call `app.ConvStore.GetByID(id)`, also `app.MsgStore.ListByConversation(id)`, return `{conversation, messages}`.

`chatDeleteConversation` — delete messages first, then conversation.

`chatUpdateTitle` — decode `{title}`, call `app.ConvStore.UpdateTitle(id, title)`.

- [ ] **Step 3: Implement SSE streaming handler**

`chatSendMessage` — the core handler:
```go
func chatSendMessage(app *mcppkg.App) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if app.AgentFactory == nil {
            writeError(w, 503, "LLM not configured")
            return
        }
        id := r.PathValue("id")
        var req struct{ Content string `json:"content"` }
        json.NewDecoder(r.Body).Decode(&req)

        agent := app.AgentFactory.NewAgent()
        waiter := agent.NewConfirmationWaiter()

        // Store waiter for confirm endpoint
        app.StoreWaiter(id, waiter)
        defer app.RemoveWaiter(id)

        events, err := agent.Run(r.Context(), id, req.Content, waiter)
        if err != nil {
            writeError(w, 500, err.Error())
            return
        }

        // SSE headers
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        flusher, _ := w.(http.Flusher)

        for ev := range events {
            data, _ := json.Marshal(ev)
            fmt.Fprintf(w, "data: %s\n\n", data)
            if flusher != nil {
                flusher.Flush()
            }
        }
    }
}
```

- [ ] **Step 4: Implement confirm handler**

`chatConfirm` — decode `{approved: bool}`, get requestId from path, call `app.GetWaiter(id).Resolve(requestId, approved)`.

- [ ] **Step 5: Add waiter management to App**

Add to App struct:
```go
waiters   map[string]*agent.ConfirmationWaiter
waitersMu sync.Mutex
```
Methods: `StoreWaiter(convID, waiter)`, `GetWaiter(convID)`, `RemoveWaiter(convID)`.

- [ ] **Step 6: Build + test + commit**

Run: `go build ./...`
Commit: `feat(api): add chat API handlers with SSE streaming`

---

### Task 3: Frontend API Client — chat.ts

**Files:**
- Create: `web/src/api/chat.ts`

- [ ] **Step 1: Define types**

```typescript
export interface Conversation {
  id: string; user_id: string; title: string
  created_at: string; updated_at: string
}
export interface ChatMessage {
  id: string; conversation_id: string; role: string
  content: string; created_at: string
}
export interface ChatEvent {
  type: 'text_delta' | 'tool_start' | 'tool_result' |
        'confirm_required' | 'error' | 'done'
  content?: Record<string, any>
}
```

- [ ] **Step 2: Implement CRUD functions**

Follow existing pattern from `hosts.ts`:
```typescript
export async function createConversation(title: string): Promise<Conversation>
export async function listConversations(): Promise<Conversation[]>
export async function getConversation(id: string): Promise<{conversation: Conversation, messages: ChatMessage[]}>
export async function deleteConversation(id: string): Promise<void>
export async function updateTitle(id: string, title: string): Promise<void>
```

All use `fetch` + `authHeaders()` from auth.ts.

- [ ] **Step 3: Implement SSE streaming via fetch + ReadableStream**

```typescript
export function sendMessage(
  conversationId: string,
  content: string,
  onEvent: (event: ChatEvent) => void,
): AbortController {
  const ctrl = new AbortController()
  const run = async () => {
    const res = await fetch(
      `/api/v1/chat/conversations/${conversationId}/messages`,
      {
        method: 'POST', signal: ctrl.signal,
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ content }),
      }
    )
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop()!
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          onEvent(JSON.parse(line.slice(6)))
        }
      }
    }
  }
  run().catch(() => {})
  return ctrl
}
```

- [ ] **Step 4: Implement confirm function**

```typescript
export async function confirmAction(
  conversationId: string, requestId: string, approved: boolean
): Promise<void>
```

- [ ] **Step 5: Commit**

Commit: `feat(web): add chat API client with SSE streaming`

---

### Task 4: ChatView.vue — Main Page Layout

**Files:**
- Create: `web/src/views/ChatView.vue`
- Modify: `web/src/main.ts` (add routes)
- Modify: `web/src/App.vue` (add nav link)

- [ ] **Step 1: Add routes**

In `main.ts`, add:
```typescript
{ path: '/chat', component: () => import('./views/ChatView.vue') },
{ path: '/chat/:id', component: () => import('./views/ChatView.vue') },
```

- [ ] **Step 2: Add nav link**

In `App.vue`, add `<RouterLink to="/chat">智能运维</RouterLink>` to nav-links.

- [ ] **Step 3: Create ChatView layout**

Two-panel layout: left chat area (70%) + right target panel (30%).

```vue
<template>
  <div class="chat-page">
    <div class="chat-area">
      <div class="chat-header">
        <!-- conversation selector dropdown + title -->
      </div>
      <div class="chat-messages" ref="messagesRef">
        <!-- message list -->
      </div>
      <div class="chat-input">
        <!-- textarea + send button -->
      </div>
    </div>
    <div class="target-panel">
      <!-- TargetPanel component -->
    </div>
  </div>
</template>
```

- [ ] **Step 4: Implement conversation management**

State: `conversations`, `activeConversation`, `messages`, `isStreaming`.
On mount: load conversations, select from route param or create new.
Conversation dropdown: list + create new button.

- [ ] **Step 5: Implement message sending + SSE consumption**

Call `sendMessage()`, accumulate `text_delta` into current assistant message, handle `tool_start`/`tool_result` as tool call entries, handle `confirm_required` by showing ConfirmBar, handle `done` to finalize.

- [ ] **Step 6: Style with terminal theme**

Dark terminal background, monospace font, `❯` prompt for user messages, code blocks with shiki highlighting. Use CSS variables from theme.ts.

- [ ] **Step 7: Commit**

Commit: `feat(web): add ChatView with conversation management and SSE streaming`

---

### Task 5: ChatMessage.vue — Message Rendering

**Files:**
- Create: `web/src/components/ChatMessage.vue`

- [ ] **Step 1: Create message component**

Props: `message: { role, content, toolCalls? }`.

Render based on role:
- `user`: show with `❯` prompt prefix, plain text
- `assistant`: render markdown via `marked`, syntax highlight code blocks via `shiki`
- Tool calls: collapsible sections showing tool name, input, result

- [ ] **Step 2: Implement tool call rendering**

Tool calls shown as expandable cards:
```
▶ execute_cli on GW-01
  command: display interface brief
  [collapsed result]
```

Click to expand shows full stdout/stderr.

- [ ] **Step 3: Implement ConfirmBar inline**

When `confirm_required` event arrives, show inline confirm/cancel buttons with risk level badge (green/yellow/red).

```vue
<div class="confirm-bar" :class="riskClass">
  <span>{{ toolName }} — {{ riskLevel }}</span>
  <button @click="confirm(true)">确认执行</button>
  <button @click="confirm(false)">取消</button>
</div>
```

- [ ] **Step 4: Commit**

Commit: `feat(web): add ChatMessage component with markdown and tool rendering`

---

### Task 6: TargetPanel.vue — Device Status Panel

**Files:**
- Create: `web/src/components/TargetPanel.vue`

- [ ] **Step 1: Create target panel component**

Three sections stacked vertically:
1. **Stats bar**: online/offline/executing/failed counts
2. **Heat matrix**: grid of small colored squares (one per device)
3. **Device list**: scrollable list with search filter

- [ ] **Step 2: Implement stats bar**

```vue
<div class="stats-bar">
  <span class="stat online">{{ onlineCount }} 在线</span>
  <span class="stat offline">{{ offlineCount }} 离线</span>
  <span class="stat running">{{ runningCount }} 执行中</span>
  <span class="stat failed">{{ failedCount }} 失败</span>
</div>
```

- [ ] **Step 3: Implement heat matrix**

Grid of 10-12px squares, color-coded:
- Green: success/online
- Gray: idle/offline
- Yellow: executing
- Red: failed

Tooltip on hover shows device name + status.

- [ ] **Step 4: Implement device list with search**

Filterable list showing: name, IP, vendor, status. Failed devices sorted to top. Click device → show detail (last command, output).

- [ ] **Step 5: Wire up device_update events**

TargetPanel receives events from parent ChatView via props/emits. On `device_update` SSE event, update device status in panel.

- [ ] **Step 6: Commit**

Commit: `feat(web): add TargetPanel with stats, heat matrix, and device list`

---

### Task 7: Full Build + Integration Test

- [ ] **Step 1: Backend build**

Run: `go build ./...`

- [ ] **Step 2: Backend tests**

Run: `go test ./... -v`

- [ ] **Step 3: Frontend build**

Run: `cd web && npm run build`

- [ ] **Step 4: Start dev server and test in browser**

Run: `cd web && npm run dev`
Navigate to `/chat`, verify:
- Conversation create/list/switch works
- Message input sends and streams response (needs LLM configured)
- Target panel renders with host data
- Navigation link visible

- [ ] **Step 5: Commit any fixes**

Phase 3 complete. Full chat system operational.
