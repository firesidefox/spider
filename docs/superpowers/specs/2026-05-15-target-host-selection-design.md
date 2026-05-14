# Target Host Selection — Design Spec

**Date:** 2026-05-15  
**Status:** Approved

## Overview

Allow users to select target hosts before starting a conversation. Selected hosts are stored on the Conversation and injected into the agent's system prompt as context. The agent uses this as a soft constraint — it prioritizes selected hosts but is not hard-blocked from others.

## User Flow

1. User optionally adjusts host selection in TargetPanel (defaults to all hosts).
2. User clicks "新建对话". `createConversation` is called with the current `selectedHostIds`.
3. User can click **编辑** at any time to change the selection — even mid-conversation.
4. Each message send uses the current selection at send time. Agent receives selected host IDs via system prompt injection on every turn.

## TargetPanel UI

### Two zones with draggable divider (top to bottom)

The two zones are separated by a draggable resize handle. The status zone (top) auto-expands to fit its content, with a maximum that allows the selection zone to be fully collapsed (i.e., status zone can grow until selection zone height = 0). When content exceeds that maximum, status zone scrolls internally. User can drag the handle to adjust the split freely.

**Zone 1 — Status (heat matrix, read-only)**
- Shows all hosts as colored cells (online=blue, offline=gray, failed=red, executing=yellow)
- In view mode: all cells visible
- In edit mode: unselected host cells are hidden (transparent placeholder, layout stable); selected cells remain visible

**Zone 2 — Host selection**

View mode:
- Header: "目标主机" + badge + "编辑" button
- Badge: green "全部 N 台" when all selected; blue "已选 M / N" when partial
- Body: "全部主机（AI 自行选择目标）" label when all selected; chip list of selected hosts (name + IP) when partial, truncated with "+N 台" if > 5
- Stats bar at bottom: online / offline / failed counts

Edit mode:
- Header: "目标主机" + badge + "完成" button
- Tag filter bar: all distinct tags as chips, multi-select; selecting a tag auto-checks all hosts with that tag
- Search input: filters list by name or IP
- Bulk row: "过滤结果 N 台 · 全选 · 清空"
- Device list: checkbox + status dot + name + IP per row; checked rows highlighted with left border + faint blue bg; unchecked rows dimmed
- Stats bar at bottom

### Default state

On new conversation: `selectedHostIds = null` (meaning "all"). Displayed as "全部 N 台".

## Data Model

Host selection is **global frontend state** — it persists across conversations and is not stored per-conversation. Each message send passes the current selection at send time. No DB schema change needed.

### API

`POST /api/v1/chat/conversations/:id/messages`  
Request body adds optional field:
```json
{ "content": "...", "host_ids": ["id1", "id2"] }
```

`null` / omitted = all hosts.

`createConversation` requires no change.

## Agent System Prompt Injection

In `BuildSystemPrompt()`, when `TargetHostIDs` is non-empty:

```
用户已预选以下主机作为操作目标：
- web-prod-01 (10.0.1.1)
- web-prod-02 (10.0.1.2)

优先对这些主机执行操作。若用户明确指定其他主机，以用户指令为准。
```

When `TargetHostIDs` is empty/null: no injection (agent sees all hosts, decides freely).

Host names are resolved at prompt-build time via the host store.

## Frontend

### TargetPanel.vue changes

New props:
```ts
props: {
  devices: DeviceStatus[]        // existing — runtime status
  allHosts: Host[]               // new — full host list for selection
  modelValue: string[] | null    // null = all selected
}
emits: ['update:modelValue']
```

Internal state:
- `editMode: boolean`
- `activeTags: string[]`
- `search: string`

Computed:
- `allTags`: distinct tags from `allHosts`
- `filteredHosts`: allHosts filtered by activeTags + search
- `isAllSelected`: modelValue === null

### ChatView.vue changes

- `selectedHostIds: string[] | null` moves to app-level global state (e.g. a composable or Pinia store), not local to ChatView
- TargetPanel reads and writes this global state
- On `sendMessage()`: pass current `selectedHostIds`

### chat.ts changes

```ts
export function sendMessage(conversationId: string, content: string, hostIds?: string[]): AbortController
```

Adds `host_ids` to request body when provided.

## Out of Scope

- Hard-blocking agent from using non-selected hosts
- Per-tool-call host override
