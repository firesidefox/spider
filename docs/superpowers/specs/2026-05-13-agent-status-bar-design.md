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
  phase: 'thinking' | 'tool' | 'confirm' | 'done' | 'idle'
  toolName?: string    // from tool_start event
  toolInput?: string   // truncated to ~40 chars
}
```

Module-level `ref<AgentStatus>` — shared across all component imports, no provide/inject needed.

## Event Mapping

| EventSource event | phase | Display |
|---|---|---|
| `text_delta` | `thinking` | `对话名 · 思考中` |
| `tool_start` | `tool` | `对话名 · bash: systemctl status nginx` |
| `confirm_required` | `confirm` | `对话名 · 等待确认 · bash: cmd` |
| `done` | `done` | `对话名 · 完成` → idle after 3s |

## Idle Transitions

Status goes `idle` (footer hidden) only when:
1. `done` event received + 3 seconds elapsed
2. User clicks the status bar (navigates to the conversation)

Navigating away from ChatView does NOT clear the status — agent keeps running in background.

## UI

```
┌─────────────────────────────────────────────────────────────┐
│ header (nav)                                                │
├─────────────────────────────────────────────────────────────┤
│ main (router-view)                                          │
├─────────────────────────────────────────────────────────────┤
│ ● 排查 nginx 502 错误 · bash: systemctl status nginx  → │  ← 28px footer
└─────────────────────────────────────────────────────────────┘
```

**Dot colors:**
- Purple pulsing — `thinking` / `tool`
- Amber static — `confirm` (needs attention)
- Green static — `done`

**Tool detail:** `toolName: truncate(toolInput, 40)` in monospace, dimmed.

**Click:** navigates to `/chat?id={conversationId}`.

**Idle:** `v-if="status.phase !== 'idle'"` — footer element removed from DOM, no height.

## Files to Change

| File | Change |
|---|---|
| `web/src/composables/useAgentStatus.ts` | New file — state + updaters |
| `web/src/components/AgentStatusBar.vue` | New file — footer UI |
| `web/src/App.vue` | Add `<AgentStatusBar>` + footer CSS |
| `web/src/views/ChatView.vue` | Call `updateAgentStatus()` in EventSource handler |

## Out of Scope

- Exec command status (separate feature)
- Multiple concurrent conversations
- Persistent status across page reload
