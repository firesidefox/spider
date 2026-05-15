# Todo Panel UX Enhancement Spec

**Date:** 2026-05-14  
**Status:** Draft

## Background

Current todo panel shows all tasks in a flat list sorted by id. Inspired by Claude Code's task UI, we want to improve visual hierarchy, reduce noise from completed tasks, and surface real-time progress.

## Goals

1. Fold completed tasks — reduce noise, keep focus on remaining work
2. Group sort + fold pending — in_progress → pending (max 2 visible) → completed (collapsed)
3. Highlight active task — stronger visual weight for in_progress
4. Timer on active task — show elapsed time while task is running
5. Real-time broadcast — broadcast `todo_update` on every TodoTool call, not just turn end
6. Token display — show turn token consumption in todo panel header (requires backend support)
7. `active_form` field — present-continuous description shown while task is in_progress

---

## Feature Specs

### 1. Fold Completed Tasks

Completed tasks collapse into a single summary row at the bottom.

**Behavior:**
- If 0 completed: no summary row
- If ≥1 completed: show `✓ +N completed` row at bottom, collapsed by default
- Click to toggle expand/collapse
- Expanded: show completed tasks below the summary row with strikethrough style (existing)

---

### 2. Group Sort and Folding

Display order: `in_progress` → `pending` (up to 2) → `+N more pending` → `completed` (collapsed).

Within each group, sort by `id` ascending.

**Pending folding:**
- Show first 2 pending tasks
- If >2 pending: show `○ +N more` row below, clickable to expand all pending
- Expanded: show all pending tasks, row changes to `○ +N more ▲`

**UI (collapsed, 6 pending tasks):**
```
● Task 3: in_progress
○ Task 4: pending
○ Task 5: pending
○ +4 more
✓ +2 completed
```

---

### 3. In-Progress Row Highlight

`in_progress` task gets stronger visual treatment:

- Left border: 2px solid `var(--primary)`
- Subject text: `var(--text)` (full brightness, not subdued)
- Icon: `●` in `var(--primary)` (already done)
- Background: subtle tint, e.g. `rgba(var(--primary-rgb), 0.06)` or `var(--surface-2)` if available

No change to pending or completed row styles.

---

### 4. Timer on Active Task

While a task is `in_progress`, show elapsed time since `created_at` (or `updated_at` when status changed to in_progress — use `updated_at` as proxy since that's what we have).

**Display:** right-aligned in the row, e.g. `2m 14s`

**Behavior:**
- Timer starts when task enters `in_progress` (frontend receives `todo_update` with status=in_progress)
- Timer ticks every second via `setInterval`
- Timer stops when task leaves `in_progress` (completed/deleted/pending)
- Multiple in_progress tasks: each shows its own timer
- On conversation switch: clear all timers, reinitialize from `updated_at` of any in_progress tasks loaded from API

**Implementation note:** Use `task.updated_at` as the start time reference. This is a proxy — it's the last update time, not strictly the in_progress start time — but it's good enough and requires no schema change.

**Format:** `Xm Ys` if ≥60s, else `Xs`

---

### 5. Real-Time Broadcast (Backend)

`broadcast(task)` must be called after every `todo_task` tool execution (create/update).

**Verify:** Confirm `execCreate` calls `t.broadcast(task)`. If missing, add it.

No new SSE event type needed. Frontend already handles `todo_update`.

---

### 6. Token Display

Show input+output token count for the current turn in the todo panel header.

**Backend:**
- After each LLM turn completes in `agent.go`, broadcast a new SSE event:
  ```json
  {"type": "turn_usage", "content": {"input_tokens": 1234, "output_tokens": 567}}
  ```
- Broadcast via `events <- Event{Type: EventTurnUsage, Content: ...}` then picked up by `chat.go` SSE loop

**Frontend:**
- `ChatEvent.type` gains `'turn_usage'`
- `handleConvEvent` case `turn_usage`: store `turnUsage` ref `{input: number, output: number}`
- Header shows: `TASKS 2/5  ↓ 1.2k` — `↓` = output tokens (abbreviated); input tokens not displayed
- Reset `turnUsage` to null on new user message send

**Abbreviation:** `< 1000` → show as-is; `≥ 1000` → `Xk` rounded to 1 decimal

---

### 7. `active_form` Field

Inspired by Claude Code's task model. Tasks have two text fields:
- `subject` — imperative title, e.g. `"Update TodoStore — turn_id support"`
- `active_form` — present-continuous, shown while in_progress, e.g. `"Updating TodoStore"`

**Backend:**
- Add `ActiveForm string` to `models.Todo` (JSON: `"active_form,omitempty"`)
- DB: add `active_form TEXT` column to `todo_tasks` table (migration required)
- `TodoTool` `create` action: accept optional `active_form` parameter
- `TodoTool` `update` action: accept optional `active_form` parameter

**Frontend:**
- `Todo` interface gains `active_form?: string`
- Panel header display logic:
  - Any task `in_progress`: header = `active_form ?? subject` + `(Xm Ys)` + `↓ Xk` (if token data available)
  - No task `in_progress`: header = `TASKS N/M` + `↓ Xk` (if token data available)
- Row itself: still shows `subject`

**Agent prompt:** Update `todoTaskPrompt` to instruct agent to provide `active_form` when creating tasks. Example:
```
create: { subject: "Update TodoStore", active_form: "Updating TodoStore", ... }
```

---

## Files Affected

| File | Change | Features |
|------|--------|----------|
| `web/src/views/ChatView.vue` | computed group sort, completed fold, pending fold (max 2), timer setInterval, active_form header, token display, CSS | 1,2,3,4,6,7 |
| `web/src/api/chat.ts` | add `'turn_usage'` to `ChatEvent.type`; add `active_form?: string` to `Todo` | 6,7 |
| `internal/agent/agent.go` | add `EventTurnUsage` constant; emit `turn_usage` event after each LLM turn completes | 6 |
| `internal/agent/tools_todo_task.go` | verify `execCreate` calls `t.broadcast`; add `active_form` param to create/update | 5,7 |
| `internal/models/todo_task.go` | add `ActiveForm string` field | 7 |
| `internal/db/schema.go` | add `ALTER TABLE todo_tasks ADD COLUMN active_form TEXT NOT NULL DEFAULT ''` in migrate() | 7 |
| `internal/store/todo_task_store.go` | include `active_form` in Create INSERT, Update SET, and scan in List/Get | 7 |

---

## Non-Goals

- No drag-to-reorder tasks
- No token display per-task (only per-turn total)
