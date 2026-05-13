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

Module-level `ref<Map<string, AgentStatus>>` — keyed by `conversationId`. No hard cap on entries.

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
│ ● 排查 nginx 502 · bash: systemctl status nginx           →  │ ← current conv, row 1
│ ● 部署前端 · 思考中                                         →  │ ← row 2
│ ● 数据库迁移 · 等待确认 · bash: migrate.sh                 →  │ ← row 3
│ ↕ scroll for more                                               │ ← if >3
└──────────────────────────────────────────────────────────────────┘
```

**Ordering:** Current route's conversation always first. Others ordered by `updatedAt` descending.

**Layout:** Each conversation gets its own 28px row. Max visible height = 3 rows (84px). If more than 3 active conversations, container scrolls vertically (`overflow-y: auto`). No hard cap on entry count.

**Dot colors:**
- Purple pulsing — `thinking` / `tool`
- Amber static — `confirm` (needs attention)
- Green static — `done`

**Tool detail:** `toolName: truncate(toolInput, 40)` in monospace, dimmed.

**Click item:** navigates to `/chat?id={conversationId}`, removes entry from map.

**Footer hidden:** when map is empty (`v-if="statuses.size > 0"`).

**Footer height:** `min-height: 28px`, `max-height: 84px` (3 rows), `overflow-y: auto`.

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
