---
title: Tool Result Budgeting
date: 2026-05-16
status: draft
---

# Tool Result Budgeting

## Problem

Tool results can be arbitrarily large (e.g., `cat` on a big file, `grep` across many files). Without limits, a single turn can inject hundreds of KB into the LLM context, wasting tokens and risking context overflow. The compactor handles overflow after the fact; budgeting prevents it proactively.

## Goals

1. Cap individual tool results before they enter LLM history.
2. Cap the aggregate tool result size per message turn.
3. Preserve full content on disk; give LLM a preview + file path.
4. Keep decisions stable across history rebuilds (same tool_use_id → same preview).

## Non-Goals

- Prompt cache optimization (spider.ai has no caching today).
- UI for browsing persisted tool results.
- Per-tool custom thresholds (use defaults for now).

---

## Architecture

Two independent layers applied in sequence:

```
Tool executes
    │
    ▼
[Layer 1] Per-tool limit  (in agent.go, after tool.Execute)
    result > 50 000 chars?
    → persist to file, replace with preview message
    │
    ▼  (result is now either original or preview)
stored to msgStore as "tool_result" row
    │
    ▼
[Layer 2] Per-message aggregate limit  (in compactor.go, inside toLLMMessages)
    sum of all tool_result blocks in this turn > 200 000 chars?
    → persist largest ones, replace with preview messages
    │
    ▼
LLM receives history
```

---

## Layer 1: Per-Tool Limit

### Threshold

`PerToolResultMaxChars = 50_000` (configurable via `config.AgentConfig`)

### Trigger point

`agent.go` — immediately after `tool.Execute()` returns, before appending to `toolResults` slice.

### Behavior

1. Measure `len(result.Content)`.
2. If under threshold: no-op.
3. If over threshold:
   - Write full content to `{dataDir}/tool-results/{conversationID}/{toolUseID}.txt`.
   - Replace `result.Content` with preview message (see format below).
   - Set `result.Persisted = true` on the record (for logging).

### Preview format

```
[Output too large: 123 456 chars. Full output saved to: /path/to/file.txt]

Preview (first 2 000 chars):
<first 2000 chars of original content, cut at last newline within limit>
...
```

### File layout

```
{dataDir}/tool-results/
  {conversationID}/
    {toolUseID}.txt
```

Files are written once and never modified. Cleanup is out of scope (manual or future GC task).

---

## Layer 2: Per-Message Aggregate Limit

### Threshold

`PerMessageToolResultMaxChars = 200_000` (configurable via `config.AgentConfig`)

### Trigger point

`compactor.go` — inside `toLLMMessages()`, when assembling the `[]ContentBlock` for a `tool_result` group.

### ContentReplacementState

```go
type ContentReplacementState struct {
    mu           sync.Mutex
    replacements map[string]string  // toolUseID → preview string (frozen once set)
    seen         map[string]bool    // toolUseID → ever processed
}
```

- Owned by `Agent`, one instance per conversation session.
- Passed into `toLLMMessages()` on every history rebuild.
- Decisions are frozen: once a toolUseID is in `replacements`, the same preview is reused forever.
- `seen` tracks IDs that were processed but NOT replaced (budget was fine that turn); these are frozen as "do not replace" to avoid retroactive changes.

### Algorithm (per turn group)

```
Collect all ContentBlocks in this turn's tool_result group.

Partition into:
  mustReapply  = IDs in replacements map  → reuse cached preview
  frozen       = IDs in seen but not replacements  → leave as-is
  fresh        = IDs not in seen  → eligible for new decisions

totalSize = sum(len(block.Content)) for frozen + fresh blocks

if totalSize <= PerMessageToolResultMaxChars:
    mark all fresh IDs as seen (no replacement)
    return blocks as-is (with mustReapply substituted)

else:
    sort fresh blocks by size descending
    greedily select largest until totalSize <= threshold
    for each selected:
        persist to file (same path scheme as Layer 1)
        build preview string
        store in replacements[id]
    mark remaining fresh IDs as seen
    return blocks with replacements applied
```

### Interaction with Layer 1

If Layer 1 already replaced a result, its content is already a short preview (~2 000 chars). Layer 2 sees the preview size, not the original. This means Layer 1 results effectively don't count toward the aggregate budget — correct behavior.

---

## New Files

| File | Purpose |
|------|---------|
| `internal/agent/tool_result_budget.go` | `ContentReplacementState`, `persistToolResult()`, `generatePreview()`, `enforcePerMessageBudget()` |

## Modified Files

| File | Change |
|------|--------|
| `internal/agent/agent.go` | Apply Layer 1 after `tool.Execute()`; add `replacementState` field to `Agent` |
| `internal/agent/compactor.go` | Pass `replacementState` into `toLLMMessages()`; call `enforcePerMessageBudget()` |
| `internal/config/config.go` | Add `PerToolResultMaxChars`, `PerMessageToolResultMaxChars` to `AgentConfig` |

---

## Configuration

```go
type AgentConfig struct {
    // existing fields ...
    PerToolResultMaxChars     int    // default 50_000; 0 = disabled
    PerMessageToolResultMaxChars int // default 200_000; 0 = disabled
}
```

Zero value disables the respective layer (opt-out for tests).

---

## Error Handling

- File write failure: log warning, do NOT replace result (pass original through). Better to send large content than silently lose it.
- Directory creation failure: same — log and pass through.

---

## Testing

1. **Unit: Layer 1** — result > 50K → file written, preview returned; result ≤ 50K → no-op.
2. **Unit: Layer 2** — aggregate > 200K → largest replaced; stable across two calls with same state.
3. **Unit: preview generation** — cuts at last newline, handles content shorter than preview size.
4. **Unit: ContentReplacementState** — frozen decisions not overwritten; mustReapply returns cached string.
5. **Integration** — full agent run with a tool returning 100K content; verify LLM history contains preview, file exists on disk.
