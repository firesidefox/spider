# Agent Status Bar — Design Spec

**Date:** 2026-05-13  
**Status:** Approved

## Overview

A global 28px footer bar in App.vue that shows the current agent conversation status in real time. Visible on all pages except `/login`. Disappears when idle.

## Goals

- User can see agent progress without staying on ChatView
- Shows current tool being executed (like Claude Code's status line)
- Zero friction: no new dependencies, fits existing composable pattern

## Architecture

```
useAgentStatus.ts          ← module-level singleton, state + updaters
AgentStatusBar.vue         ← footer UI component
App.vue                    ← mounts AgentStatusBar, adds footer layout
ChatView.vue               ← calls updateAgentStatus() on EventSource events
```

## State

```ts
interface AgentStatus {
  conversationId: string
  title: string
  phase: 'thinking' | 'tool' | 'confirm' | 'done'
  toolName?: string    // from tool_start event
  toolInput?: string   // truncated to ~40 chars
  updatedAt: number    // Date.now(), used for ordering
}
```

Module-level `ref<Map<string, AgentStatus>>` — keyed by `conversationId`. Max 3 entries; when a 4th arrives, evict the oldest `done` entry first, then oldest by `updatedAt`.

Footer hidden when map is empty.

## Event Mapping

| EventSource event | phase | Display |
|---|---|---|
| `text_delta` | `thinking` | `对话名 · 思考中` |
| `tool_start` | `tool` | `对话名 · bash: systemctl status nginx` |
| `confirm_required` | `confirm` | `对话名 · 等待确认 · bash: cmd` |
| `done` | `done` | `对话名 · 完成` → idle after 3s |

## Entry Lifecycle

An entry is added to the map when a conversation starts streaming. It is removed when:
1. `done` event received + 3 seconds elapsed (auto-remove)
2. User clicks the entry in the status bar (navigates to conversation, then remove)

Navigating away from ChatView does NOT remove the entry — agent keeps running in background.

## UI

```
┌──────────────────────────────────────────────────────────────────┐
│ header (nav)                                                     │
├──────────────────────────────────────────────────────────────────┤
│ main (router-view)                                               │
├──────────────────────────────────────────────────────────────────┤
│ ● 排查 nginx 502 · bash: systemctl status nginx  →  ● 部署前端 · 思考中  → │  ← 28px, scrollable
└──────────────────────────────────────────────────────────────────┘
```

**Ordering:** Current route's conversation always first. Others ordered by `updatedAt` descending.

**Layout:** Single row, `overflow-x: auto`, `scrollbar-width: none` (hidden scrollbar). Items separated by `|` divider. Max 3 items.

**Dot colors:**
- Purple pulsing — `thinking` / `tool`
- Amber static — `confirm` (needs attention)
- Green static — `done`

**Tool detail:** `toolName: truncate(toolInput, 40)` in monospace, dimmed.

**Click item:** navigates to `/chat?id={conversationId}`, removes entry from map.

**Footer hidden:** when map is empty (`v-if="statuses.size > 0"`).

## Files to Change

| File | Change |
|---|---|
| `web/src/composables/useAgentStatus.ts` | New file — state + updaters |
| `web/src/components/AgentStatusBar.vue` | New file — footer UI |
| `web/src/App.vue` | Add `<AgentStatusBar>` + footer CSS |
| `web/src/views/ChatView.vue` | Call `updateAgentStatus()` in EventSource handler |

## Out of Scope

- Exec command status (separate feature)
- Persistent status across page reload
