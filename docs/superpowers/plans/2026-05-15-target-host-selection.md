# Target Host Selection — Implementation Plan

**Spec:** docs/superpowers/specs/2026-05-15-target-host-selection-design.md  
**Date:** 2026-05-15

## Tasks

### 1. Backend — sendMessage accepts host_ids

- `internal/agent/agent.go`: add `TargetHostIDs []string` to the Run() input or message struct
- `internal/agent/tools_api.go` or message handler: read `host_ids` from request body, pass to agent
- `internal/agent/agent.go` `BuildSystemPrompt()`: inject host list when TargetHostIDs non-empty
- Resolve host names via host store at prompt-build time

### 2. Frontend — global selection state

- `web/src/composables/useTargetHosts.ts` (new): holds `selectedHostIds: Ref<string[] | null>`, default null
- Export from a single place so TargetPanel and ChatView share the same ref

### 3. Frontend — TargetPanel.vue

- Add props: `allHosts: Host[]`, `modelValue: string[] | null`
- Add emits: `update:modelValue`
- Internal state: `editMode`, `activeTags`, `search`
- Split into two zones with draggable resize handle (CSS: flex-direction column, resize via mousedown on divider)
- Status zone: auto-height, max = panel height (selection zone min-height: 0), overflow-y scroll
- Heat matrix: per-cell color + animation per spec; blue outline for selected; transparent for unselected when partial
- Selection zone view mode: badge + chip list or "全部" label + stats bar
- Selection zone edit mode: tag filter + search + bulk row + device list + stats bar

### 4. Frontend — ChatView.vue

- Import `useTargetHosts`
- Pass `allHosts` (from `listHosts()`) and `v-model` to TargetPanel
- Pass `selectedHostIds` to `sendMessage()`

### 5. Frontend — chat.ts

- Update `sendMessage()` signature: add `hostIds?: string[]`
- Include `host_ids` in SSE request body when provided

## Order

1 → 2 → 3 → 4 → 5 (backend first so frontend can test end-to-end)
