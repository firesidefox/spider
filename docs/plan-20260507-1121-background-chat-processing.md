# Background Chat Processing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple LLM processing from SSE connection lifetime so chat continues when user navigates away or closes the browser.

**Architecture:** Backend uses `context.Background()` for agent execution and a `status` field on conversations to track processing state. Frontend uses `<KeepAlive>` for in-app navigation, a per-conversation messages map to preserve state across conversation switches, and localStorage + polling for browser-close recovery.

**Tech Stack:** Go (backend), Vue 3 Composition API, TypeScript (frontend)

---

## File Map

| File | Change |
|------|--------|
| `internal/db/schema.go` | Add `ALTER TABLE conversations ADD COLUMN status` migration |
| `internal/models/conversation.go` | Add `Status string` field |
| `internal/store/conversation.go` | Add `SetStatus()`, update `GetByID`/`ListByUser` to scan `status` |
| `internal/store/conversation_test.go` | Add `TestConversationStore_SetStatus` |
| `internal/api/chat.go` | Use `context.Background()`, call `SetStatus` before/after SSE loop |
| `web/src/api/chat.ts` | Add `status` to `Conversation` interface |
| `web/src/App.vue` | Wrap `<RouterView>` with `<KeepAlive include="ChatView">` |
| `web/src/views/ChatView.vue` | `defineOptions`, `onActivated`/`onDeactivated`, messages map, `pollUntilIdle`, localStorage |

---

## Task 1: DB migration, model, store

**Files:**
- Modify: `internal/db/schema.go:222`
- Modify: `internal/models/conversation.go`
- Modify: `internal/store/conversation.go`
- Test: `internal/store/conversation_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/store/conversation_test.go`:

```go
func TestConversationStore_SetStatus(t *testing.T) {
	database := setupTestDB(t)
	s := NewConversationStore(database)

	conv, err := s.Create("user-1", "test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.GetByID(conv.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "idle" {
		t.Errorf("default Status = %q, want idle", got.Status)
	}
	if err := s.SetStatus(conv.ID, "processing"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	got, err = s.GetByID(conv.ID)
	if err != nil {
		t.Fatalf("GetByID after SetStatus: %v", err)
	}
	if got.Status != "processing" {
		t.Errorf("Status = %q, want processing", got.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/... -run TestConversationStore_SetStatus -v
```
Expected: FAIL — `s.SetStatus undefined` or `got.Status` field missing.

- [ ] **Step 3: Add DB migration**

In `internal/db/schema.go`, after line 222 (after the `validated_at` migration), before `return nil`:

```go
db.Exec("ALTER TABLE conversations ADD COLUMN status TEXT NOT NULL DEFAULT 'idle'")
```

- [ ] **Step 4: Add Status field to model**

In `internal/models/conversation.go`, update `Conversation` struct:

```go
type Conversation struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Title          string    `json:"title"`
	Status         string    `json:"status"`
	PermissionMode string    `json:"permission_mode,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

- [ ] **Step 5: Update store — SetStatus + scan status in queries**

Replace the full content of `internal/store/conversation.go` with the updated version.

`GetByID` — update SQL and Scan:
```go
func (s *ConversationStore) GetByID(id string) (*models.Conversation, error) {
	row := s.db.QueryRow(
		"SELECT id, user_id, title, status, permission_mode, created_at, updated_at FROM conversations WHERE id = ?", id,
	)
	var c models.Conversation
	err := row.Scan(&c.ID, &c.UserID, &c.Title, &c.Status, &c.PermissionMode, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("scan conversation: %w", err)
	}
	return &c, nil
}
```

`ListByUser` — update SQL and Scan:
```go
func (s *ConversationStore) ListByUser(userID string) ([]*models.Conversation, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, title, status, permission_mode, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	var list []*models.Conversation
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.Status, &c.PermissionMode, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan conversation row: %w", err)
		}
		list = append(list, &c)
	}
	return list, nil
}
```

Add `SetStatus` at end of file:
```go
func (s *ConversationStore) SetStatus(id, status string) error {
	_, err := s.db.Exec(
		"UPDATE conversations SET status = ?, updated_at = ? WHERE id = ?",
		status, time.Now().UTC(), id,
	)
	return err
}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/... -run TestConversationStore_SetStatus -v
```
Expected: PASS

- [ ] **Step 7: Run all store tests**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/... -v
```
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/db/schema.go internal/models/conversation.go internal/store/conversation.go internal/store/conversation_test.go
git commit -m "feat(store): add conversation status field and SetStatus()"
```

---

## Task 2: Decouple chat handler from request context

**Files:**
- Modify: `internal/api/chat.go:174-191`

- [ ] **Step 1: Update chatSendMessage**

In `internal/api/chat.go`, replace lines 174–191:

```go
// Before (line 174):
events, err := a.Run(r.Context(), id, content, waiter)
if err != nil {
    writeError(w, 500, err.Error())
    return
}

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
```

Replace with:

```go
app.ConvStore.SetStatus(id, "processing") //nolint:errcheck
events, err := a.Run(context.Background(), id, content, waiter)
if err != nil {
    app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
    writeError(w, 500, err.Error())
    return
}

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
app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
```

- [ ] **Step 2: Build to verify**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/api/chat.go
git commit -m "feat(api): decouple LLM processing from SSE connection context"
```

---

## Task 3: Frontend type update + App.vue KeepAlive

**Files:**
- Modify: `web/src/api/chat.ts`
- Modify: `web/src/App.vue:26`

- [ ] **Step 1: Add status to Conversation type**

In `web/src/api/chat.ts`, update `Conversation` interface:

```ts
export interface Conversation {
  id: string
  user_id: string
  title: string
  status: string
  permission_mode?: string
  created_at: string
  updated_at: string
}
```

- [ ] **Step 2: Add KeepAlive to App.vue**

In `web/src/App.vue`, replace line 26:
```html
<!-- before -->
<RouterView />

<!-- after -->
<KeepAlive include="ChatView">
  <RouterView />
</KeepAlive>
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: no TypeScript errors

- [ ] **Step 4: Commit**

```bash
git add web/src/api/chat.ts web/src/App.vue
git commit -m "feat(frontend): add conversation status type, keep ChatView alive"
```

---

## Task 4: ChatView — messages map + helpers

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Replace messages ref with messages map**

In `ChatView.vue`, replace line 31:
```ts
// before
const messages = ref<DisplayMessage[]>([])

// after
const messagesMap = ref<Record<string, DisplayMessage[]>>({})
const messages = computed(() => messagesMap.value[activeConvId.value ?? ''] ?? [])
```

- [ ] **Step 2: Add helpers after the messages declaration**

Add after the `messages` computed (after line 31 area):

```ts
function getOrInitMessages(convId: string): DisplayMessage[] {
  if (!messagesMap.value[convId]) {
    messagesMap.value[convId] = []
  }
  return messagesMap.value[convId]
}

function buildDisplayMessages(msgs: ChatMsg[]): DisplayMessage[] {
  return msgs.map(m => {
    const blocks: MessageBlock[] = []
    if (m.content) blocks.push({ type: 'text', content: m.content })
    if (m.tool_calls) {
      try {
        for (const tc of JSON.parse(m.tool_calls)) {
          blocks.push({ type: 'tool', call: {
            id: tc.id, name: tc.name, input: tc.input,
            result: tc.result, isError: tc.is_error, durationMs: tc.duration_ms,
          }})
        }
      } catch { /* ignore malformed */ }
    }
    return { id: m.id, role: m.role, blocks } as DisplayMessage
  })
}
```

- [ ] **Step 3: Update selectConversation to use messagesMap**

Replace `selectConversation` (lines 132–153):

```ts
async function selectConversation(id: string) {
  const data = await getConversation(id)
  activeConvId.value = id
  localStorage.setItem('spider-last-conv', id)
  messagesMap.value[id] = buildDisplayMessages(data.messages)
  isStreaming.value = data.conversation.status === 'processing'
  router.replace(`/chat/${id}`)
  if (data.conversation.status === 'processing') {
    pollUntilIdle(id)
  }
  await nextTick()
  scrollToBottom()
}
```

- [ ] **Step 4: Update createNewConversation**

Replace `messages.value = []` in `createNewConversation` (line 159):
```ts
// before
messages.value = []
// after
messagesMap.value[conv.id] = []
```

- [ ] **Step 5: Update handleDeleteConversation**

Replace `messages.value = []` in `handleDeleteConversation` (line 320 area):
```ts
// before
activeConvId.value = null
messages.value = []
router.replace('/chat')

// after
activeConvId.value = null
delete messagesMap.value[id]
router.replace('/chat')
```

- [ ] **Step 6: Update send() to capture convMsgs**

In `send()`, replace the section that pushes messages and starts streaming (lines 186–200):

```ts
// before
const userMsg: DisplayMessage = {
  id: `u-${Date.now()}`, role: 'user', blocks: [{ type: 'text', content: text }],
}
messages.value.push(userMsg)

const assistantMsg: DisplayMessage = {
  id: `a-${Date.now()}`, role: 'assistant',
  blocks: [], isStreaming: true,
}
messages.value.push(assistantMsg)
isStreaming.value = true
await nextTick()
scrollToBottom()

abortCtrl = sendMessage(activeConvId.value!, text, (event: ChatEvent) => {
  const last = messages.value[messages.value.length - 1]

// after
const convId = activeConvId.value!
const convMsgs = getOrInitMessages(convId)

const userMsg: DisplayMessage = {
  id: `u-${Date.now()}`, role: 'user', blocks: [{ type: 'text', content: text }],
}
convMsgs.push(userMsg)

const assistantMsg: DisplayMessage = {
  id: `a-${Date.now()}`, role: 'assistant',
  blocks: [], isStreaming: true,
}
convMsgs.push(assistantMsg)
isStreaming.value = true
await nextTick()
scrollToBottom()

abortCtrl = sendMessage(convId, text, (event: ChatEvent) => {
  const last = convMsgs[convMsgs.length - 1]
```

- [ ] **Step 7: Update done/error handlers to guard isStreaming by convId**

In the stream callback's `done` and `error` cases, guard `isStreaming.value = false` with a convId check:

```ts
case 'error': {
  const errText = `\n\n**Error:** ${event.content?.error || 'unknown error'}`
  const lastBlk = last.blocks[last.blocks.length - 1]
  if (lastBlk?.type === 'text') {
    lastBlk.content += errText
  } else {
    last.blocks.push({ type: 'text', content: errText })
  }
  last.isStreaming = false
  if (activeConvId.value === convId) isStreaming.value = false
  break
}
case 'done':
  last.isStreaming = false
  if (activeConvId.value === convId) isStreaming.value = false
  loadConversations()
  break
```

- [ ] **Step 8: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: no TypeScript errors

- [ ] **Step 9: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): per-conversation messages map, preserve state across conv switches"
```

---

## Task 5: ChatView — lifecycle hooks + pollUntilIdle + localStorage

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Add defineOptions and import onActivated/onDeactivated**

At the top of `<script setup>`, add `defineOptions`:
```ts
defineOptions({ name: 'ChatView' })
```

Update the Vue import (line 2) to include `onActivated` and `onDeactivated`:
```ts
import { ref, onActivated, onDeactivated, nextTick, watch, computed } from 'vue'
```

- [ ] **Step 2: Add pollTimer and pollUntilIdle**

Add after the `abortCtrl` declaration (line 36 area):

```ts
let pollTimer: ReturnType<typeof setTimeout> | null = null

async function pollUntilIdle(convId: string) {
  const check = async () => {
    try {
      const data = await getConversation(convId)
      if (data.conversation.status === 'idle') {
        messagesMap.value[convId] = buildDisplayMessages(data.messages)
        if (activeConvId.value === convId) {
          isStreaming.value = false
          await nextTick()
          scrollToBottom()
          loadConversations()
        }
      } else {
        pollTimer = setTimeout(check, 2000)
      }
    } catch {
      pollTimer = setTimeout(check, 2000)
    }
  }
  pollTimer = setTimeout(check, 2000)
}
```

- [ ] **Step 3: Replace onMounted with onActivated**

Replace `onMounted` (lines 487–501) with:

```ts
onActivated(async () => {
  await Promise.all([loadConversations(), loadDevices()])
  getActiveModel().then(m => { currentModelName.value = m.model })
  const paramId = route.params.id as string | undefined
  if (paramId) {
    if (paramId !== activeConvId.value) {
      await selectConversation(paramId)
    }
  } else {
    const lastConvId = localStorage.getItem('spider-last-conv')
    if (lastConvId) {
      router.replace(`/chat/${lastConvId}`)
      await selectConversation(lastConvId)
    }
  }
  try {
    const res = await fetch('/api/v1/settings', { headers: authHeaders() })
    const data = await res.json()
    globalMode.value = data.permission_mode || 'ask'
  } catch (_) { /* use default */ }
  document.addEventListener('click', closeModeDropdown)
})
```

- [ ] **Step 4: Replace onUnmounted with onDeactivated**

Replace `onUnmounted` (lines 503–505) with:

```ts
onDeactivated(() => {
  if (pollTimer) { clearTimeout(pollTimer); pollTimer = null }
  document.removeEventListener('click', closeModeDropdown)
})
```

- [ ] **Step 5: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: no TypeScript errors

- [ ] **Step 6: Build backend**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): background processing resilience — keep-alive, poll, localStorage restore"
```
