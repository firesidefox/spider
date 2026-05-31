# Design: Remove `pollUntilIdle`, SSE-Only Status Sync

**Date:** 2026-05-31  
**Scope:** Frontend only (`web/src/views/ChatView.vue`)  
**Risk:** Low

## Problem

When multiple conversations are processing simultaneously, each runs a `pollUntilIdle` loop that fires a `GET /api/v1/chat/conversations/:id` request every 2 seconds. This is redundant because the SSE stream (`subscribeConversation`) already delivers all events including `done`. The poll and SSE are uncoordinated: SSE `done` arrives and calls `setConversationStreaming(false)`, but `pollUntilIdle` continues until its next 2s tick.

## Architecture: Current vs Target

**Current:**
- `setConversationStreaming(true)` → start SSE + start `pollUntilIdle`
- SSE `done` → `setConversationStreaming(false)` (does NOT stop poll timer)
- Poll fires every 2s until it sees `status === 'idle'`

**Target:**
- `setConversationStreaming(true)` → start SSE only
- SSE `done` → `setConversationStreaming(false)` — done
- No polling, no timers to coordinate

## Reliability Analysis

Removing polling is safe because of the backend's in-flight SSE buffer:

| Scenario | Behavior |
|---|---|
| Normal flow | SSE `done` arrives, sets streaming=false |
| SSE disconnects mid-turn | EventSource auto-reconnects; backend `DrainSSEBuffer` replays up to 500 in-flight events including `done` |
| Buffer overflow (>500 events) | Rare; `done` on reconnect may be missing. `loadConversations()` called in `done` handler already refreshes list status. Page is eventually consistent. |
| Page refresh / new tab | `selectConversation` reads DB status; if `processing`, opens new EventSource which gets buffer replay |

The backend (`chat_stream.go`) reads `Last-Event-ID` from header or query param on reconnect, replays DB messages after cursor, then drains the in-flight buffer before subscribing to live updates.

## Changes

All changes are in `web/src/views/ChatView.vue`. No backend changes.

**Delete:**
- `let pollTimers: Map<string, ReturnType<typeof setTimeout>>` declaration
- `function clearPollTimer(convId: string)` — entire function
- `async function pollUntilIdle(convId: string)` — entire function
- In `setConversationStreaming`: the `if (!pollTimers.has(convId)) pollUntilIdle(convId)` branch
- In `selectConversation`: the `if (data.conversation.status === 'processing') { pollUntilIdle(id) }` block
- In `handleDeleteConversation`: `clearPollTimer(id)` call
- In `onDeactivated`: `pollTimers.forEach((t) => clearTimeout(t))` and `pollTimers.clear()` lines

**Keep unchanged:**
- `subscribeConversation` and `convSubscriptions` — SSE is the only path now
- `setConversationStreaming(false)` call sites in `done` and `error` handlers
- `loadConversations()` call in `done` handler — still needed to refresh sidebar status

## Success Criteria

1. Build passes (`npm run build` no errors)
2. Send message → response streams in → UI shows idle after `done` (no regression)
3. Two conversations processing simultaneously → no poll requests in DevTools Network tab
4. SSE disconnect simulation (disable network briefly) → reconnect → conversation state recovers
