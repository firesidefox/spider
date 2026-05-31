# Remove pollUntilIdle — SSE-Only Status Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the redundant `pollUntilIdle` polling mechanism from `ChatView.vue`, relying entirely on the existing SSE stream for conversation status updates.

**Architecture:** All conversation status (streaming start/stop) is already driven by SSE events (`done`, `error`) via `subscribeConversation`. The poll loop is a vestigial fallback that fires every 2s but is never coordinated with SSE — it continues running after SSE `done` arrives. Removing it eliminates redundant HTTP requests without changing any observable behavior.

**Tech Stack:** Vue 3, TypeScript — frontend only, no backend changes.

---

### Task 1: Delete polling code from ChatView.vue

**Files:**
- Modify: `web/src/views/ChatView.vue`

This is a pure deletion task. No new code is written. All changes trace directly to removing `pollTimers`, `clearPollTimer`, and `pollUntilIdle`.

- [ ] **Step 1: Delete `pollTimers` declaration and `clearPollTimer` function (lines 123–128)**

In `web/src/views/ChatView.vue`, remove these lines:

```ts
// DELETE these lines:
let pollTimers: Map<string, ReturnType<typeof setTimeout>> = new Map()

function clearPollTimer(convId: string) {
  const t = pollTimers.get(convId)
  if (t !== undefined) { clearTimeout(t); pollTimers.delete(convId) }
}
```

- [ ] **Step 2: Delete `pollUntilIdle` function (lines 141–161)**

Remove the entire function:

```ts
// DELETE this entire function:
async function pollUntilIdle(convId: string) {
  const check = async () => {
    try {
      const data = await getConversation(convId)
      if (data.conversation.status === 'idle') {
        pollTimers.delete(convId)
        messagesMap.value[convId] = buildDisplayMessages(data.messages)
        setConversationStreaming(convId, false)
        if (activeConvId.value === convId) {
          await nextTick()
          scrollToBottom()
        }
      } else {
        pollTimers.set(convId, setTimeout(check, 2000))
      }
    } catch {
      pollTimers.set(convId, setTimeout(check, 2000))
    }
  }
  pollTimers.set(convId, setTimeout(check, 2000))
}
```

- [ ] **Step 3: Remove poll start from `setConversationStreaming` (line 178)**

In `setConversationStreaming`, the `streaming` branch currently reads:

```ts
  if (streaming) {
    next.add(convId)
    if (!pollTimers.has(convId)) pollUntilIdle(convId)  // DELETE this line
  } else next.delete(convId)
```

After edit:

```ts
  if (streaming) {
    next.add(convId)
  } else next.delete(convId)
```

- [ ] **Step 4: Remove poll start from `selectConversation` (lines 576–578)**

In `selectConversation`, remove the standalone `pollUntilIdle` call. The block currently reads:

```ts
    messagesMap.value[id] = buildDisplayMessages(data.messages)
    setConversationStreaming(id, data.conversation.status === 'processing')
    if (data.conversation.status === 'processing') {
      pollUntilIdle(id)   // DELETE this block (the if + the call)
    }
```

After edit:

```ts
    messagesMap.value[id] = buildDisplayMessages(data.messages)
    setConversationStreaming(id, data.conversation.status === 'processing')
```

- [ ] **Step 5: Remove `clearPollTimer` call from `handleDeleteConversation` (line 1080)**

```ts
async function handleDeleteConversation(id: string) {
  await deleteConversation(id)
  conversations.value = conversations.value.filter(c => c.id !== id)
  clearPollTimer(id)   // DELETE this line
  const unsub = convSubscriptions.get(id)
```

- [ ] **Step 6: Remove `pollTimers` cleanup from `onDeactivated` (lines 1329–1330)**

```ts
onDeactivated(() => {
  clearAllTimers()
  pollTimers.forEach((t) => clearTimeout(t))   // DELETE
  pollTimers.clear()                            // DELETE
  document.removeEventListener('click', closeModeDropdown)
  window.removeEventListener('keydown', handleEscCancel)
})
```

After edit:

```ts
onDeactivated(() => {
  clearAllTimers()
  document.removeEventListener('click', closeModeDropdown)
  window.removeEventListener('keydown', handleEscCancel)
})
```

- [ ] **Step 7: Verify no remaining references to deleted symbols**

Run:
```bash
grep -n "pollUntilIdle\|clearPollTimer\|pollTimers" web/src/views/ChatView.vue
```

Expected: no output (zero matches).

- [ ] **Step 8: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build
```

Expected: build completes with no TypeScript errors.

- [ ] **Step 9: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "refactor(frontend): remove pollUntilIdle; rely on SSE for status sync

SSE stream already delivers all events including done/error.
Poll loop was redundant and uncoordinated — it kept firing after
SSE done arrived. Backend DrainSSEBuffer (up to 500 events) handles
reconnect replay, so no fallback needed."
```

---

### Task 2: Verify in browser (manual smoke test)

**Files:** none

- [ ] **Step 1: Start dev server**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: Send a message and watch Network tab**

Open DevTools → Network → filter by `XHR/Fetch`. Send a message in a conversation.

Expected: you see **one** SSE connection to `/api/v1/chat/conversations/:id/stream` for the conversation. You do **NOT** see repeated GET requests to `/api/v1/chat/conversations/:id` every 2 seconds during streaming.

- [ ] **Step 3: Verify conversation reaches idle state**

After the assistant response completes, confirm the UI exits streaming state (send button returns, RuntimeStatusBar disappears). No manual action needed — this should happen automatically when SSE `done` fires.

- [ ] **Step 4: Open same conversation in second tab**

Copy the conversation URL, open in new tab. Confirm messages are visible and the conversation is in the correct state (idle or streaming, matching the first tab).

- [ ] **Step 5: Done**

No further commits needed for this task — it is verification only.
