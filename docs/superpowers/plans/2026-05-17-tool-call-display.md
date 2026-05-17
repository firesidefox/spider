# Tool Call Display Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current INPUT/OUTPUT block tool call display with a compact `ToolName(args) → summary` collapsed view that expands on click.

**Architecture:** Add a `Summary` field to `ToolResult` in Go; emit it in the SSE `tool_result` event; each tool fills in a human-readable summary string; frontend renders collapsed `ToolName(args) → summary` row with click-to-expand for full INPUT/OUTPUT.

**Tech Stack:** Go (backend tools), Vue 3 (ChatMessage.vue), SSE events

---

## Files

- Modify: `internal/agent/tools.go` — add `Summary string` to `ToolResult`
- Modify: `internal/agent/agent.go` — emit `summary` in SSE `tool_result` event
- Modify: `internal/agent/tools_skill.go` — set Summary on skill results
- Modify: `internal/agent/tools_cli.go` — set Summary on RunCommand results
- Modify: `internal/agent/tools_batch.go` — set Summary on batch results
- Modify: `internal/agent/tools_list_hosts.go` — set Summary on list results
- Modify: `internal/agent/tools_docs.go` — set Summary on search results
- Modify: `internal/agent/tools_verify.go` — set Summary on verify results
- Modify: `web/src/components/ChatMessage.vue` — new collapsed display + fallback truncation
- Modify: `web/src/views/ChatView.vue` — pass `summary` from SSE event to ToolCallBlock

---

### Task 1: Add Summary field to ToolResult and emit in SSE

**Files:**
- Modify: `internal/agent/tools.go`
- Modify: `internal/agent/agent.go`

- [ ] **Step 1: Add Summary field to ToolResult struct**

In `internal/agent/tools.go`, change the struct:

```go
type ToolResult struct {
	Content     string          `json:"content"`
	IsError     bool            `json:"is_error"`
	RiskLevel   RiskLevel       `json:"risk_level"`
	Summary     string          `json:"-"`
	Nudge       string          `json:"-"`
	NewMessages []InjectMessage `json:"-"`
}
```

- [ ] **Step 2: Emit summary in SSE tool_result event**

In `internal/agent/agent.go`, find the `EventToolResult` emit (around line 415). Change:

```go
events <- Event{Type: EventToolResult, Content: map[string]any{
    "id": tc.ID, "tool": tc.Name, "input": tc.Input,
    "result": result.Content, "is_error": result.IsError,
    "duration_ms": durationMs, "summary": result.Summary,
}}
```

- [ ] **Step 3: Build to verify no compile errors**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/tools.go internal/agent/agent.go
git commit -m "feat(agent): add Summary field to ToolResult, emit in SSE tool_result"
```

---

### Task 2: Set Summary in tools_skill.go

**Files:**
- Modify: `internal/agent/tools_skill.go`

- [ ] **Step 1: Find where skill tool returns ToolResult**

```bash
grep -n "ToolResult{" /Users/cw/fty.ai/spider.ai/internal/agent/tools_skill.go
```

- [ ] **Step 2: Add Summary to successful skill load result**

Find the success return in `tools_skill.go` and add `Summary`:

```go
return &ToolResult{
    Content: content,
    Summary: fmt.Sprintf("skill %q loaded", skillName),
}, nil
```

For error cases, set a brief error summary:

```go
return &ToolResult{
    Content: err.Error(),
    IsError: true,
    Summary: "failed to load skill",
}, nil
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/agent/tools_skill.go
git commit -m "feat(tools): add Summary to skill tool results"
```

---

### Task 3: Set Summary in tools_cli.go (RunCommand)

**Files:**
- Modify: `internal/agent/tools_cli.go`

- [ ] **Step 1: Find RunCommand result construction**

```bash
grep -n "ToolResult{" /Users/cw/fty.ai/spider.ai/internal/agent/tools_cli.go
```

- [ ] **Step 2: Add a helper to build CLI summary**

Add this helper near the top of the file (after imports):

```go
func cliSummary(exitCode int, stderr string) string {
    if exitCode == 0 {
        return "exit 0"
    }
    firstLine := stderr
    if idx := strings.IndexByte(stderr, '\n'); idx >= 0 {
        firstLine = stderr[:idx]
    }
    if len(firstLine) > 60 {
        firstLine = firstLine[:60] + "…"
    }
    if firstLine == "" {
        return fmt.Sprintf("exit %d", exitCode)
    }
    return fmt.Sprintf("exit %d: %s", exitCode, firstLine)
}
```

- [ ] **Step 3: Set Summary on RunCommand results**

Find where `ToolResult` is returned after command execution and set:

```go
return &ToolResult{
    Content: output,
    IsError: exitCode != 0,
    Summary: cliSummary(exitCode, stderrStr),
}, nil
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools_cli.go
git commit -m "feat(tools): add Summary to RunCommand results"
```

---

### Task 4: Set Summary in tools_batch.go, tools_list_hosts.go, tools_docs.go, tools_verify.go

**Files:**
- Modify: `internal/agent/tools_batch.go`
- Modify: `internal/agent/tools_list_hosts.go`
- Modify: `internal/agent/tools_docs.go`
- Modify: `internal/agent/tools_verify.go`

- [ ] **Step 1: Find result construction in each file**

```bash
grep -n "ToolResult{" \
  /Users/cw/fty.ai/spider.ai/internal/agent/tools_batch.go \
  /Users/cw/fty.ai/spider.ai/internal/agent/tools_list_hosts.go \
  /Users/cw/fty.ai/spider.ai/internal/agent/tools_docs.go \
  /Users/cw/fty.ai/spider.ai/internal/agent/tools_verify.go
```

- [ ] **Step 2: tools_batch.go — batch summary**

After counting successes/failures, set:

```go
// okCount = number of hosts with exit 0
// failCount = number of hosts with non-zero exit
var summary string
if failCount == 0 {
    summary = fmt.Sprintf("%d hosts ok", okCount)
} else {
    summary = fmt.Sprintf("%d ok, %d failed", okCount, failCount)
}
return &ToolResult{Content: output, IsError: failCount > 0, Summary: summary}, nil
```

- [ ] **Step 3: tools_list_hosts.go — host count summary**

```go
return &ToolResult{
    Content: output,
    Summary: fmt.Sprintf("%d hosts", len(hosts)),
}, nil
```

- [ ] **Step 4: tools_docs.go — search result count**

```go
return &ToolResult{
    Content: output,
    Summary: fmt.Sprintf("found %d results", resultCount),
}, nil
```

- [ ] **Step 5: tools_verify.go — ok/failed**

```go
// success case
return &ToolResult{Content: output, Summary: "ok"}, nil

// error case
return &ToolResult{Content: output, IsError: true, Summary: "failed"}, nil
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/agent/tools_batch.go internal/agent/tools_list_hosts.go \
        internal/agent/tools_docs.go internal/agent/tools_verify.go
git commit -m "feat(tools): add Summary to batch, list, docs, verify tool results"
```

---

### Task 5: Frontend — pass summary from SSE to ToolCallBlock

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Add summary to ToolCallBlock interface**

In `web/src/components/ChatMessage.vue`, find the `ToolCallBlock` interface and add:

```ts
export interface ToolCallBlock {
  id: string
  name: string
  input?: Record<string, any>
  result?: string
  summary?: string      // add this
  isError?: boolean
  durationMs?: number
  hostNames?: string[]
}
```

- [ ] **Step 2: Pass summary in SSE tool_result handler**

In `web/src/views/ChatView.vue`, find the `case 'tool_result':` block (around line 568). Add `summary` to the updated block:

```ts
case 'tool_result': {
  const idx = toolIndex.get(event.content?.id || '')
  if (idx !== undefined && idx < blocks.length) {
    const old = (blocks[idx] as { type: 'tool'; call: ToolCallBlock }).call
    blocks[idx] = { type: 'tool', call: {
      ...old,
      input: event.content?.input ?? old.input,
      result: event.content?.result,
      summary: event.content?.summary,   // add this
      isError: event.content?.is_error,
      durationMs: event.content?.duration_ms,
    }}
    // ... rest unchanged
  }
  break
}
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -5
```

Expected: no TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue web/src/components/ChatMessage.vue
git commit -m "feat(frontend): add summary field to ToolCallBlock, wire from SSE"
```

---

### Task 6: Frontend — render new collapsed display in ChatMessage.vue

**Files:**
- Modify: `web/src/components/ChatMessage.vue`

- [ ] **Step 1: Add fallback summary helper**

In the `<script setup>` section of `ChatMessage.vue`, add after the existing helpers:

```ts
function toolSummary(call: ToolCallBlock): string {
  if (call.summary) return call.summary
  if (!call.result) return ''
  const first = call.result.split('\n')[0]
  return first.length > 60 ? first.slice(0, 60) + '…' : first
}
```

- [ ] **Step 2: Update the Act tool header template**

Find the `<div class="tool-header"` block (around line 150) and replace with:

```html
<div class="tool-header" @click="toggleTool(item.call.id)">
  <span class="tool-arrow">{{ expandedTools.has(item.call.id) ? '▼' : '▶' }}</span>
  <span class="tool-badge" :class="{ 'tool-badge-error': item.call.isError }">tool</span>
  <span class="tool-name">{{ item.call.name }}</span>
  <span v-if="item.call.input && Object.keys(item.call.input).length" class="tool-args"
    >({{ exploreParam(item.call) }})</span>
  <template v-if="!expandedTools.has(item.call.id) && item.call.durationMs != null">
    <span v-if="toolSummary(item.call)" class="tool-summary-arrow">→</span>
    <span v-if="toolSummary(item.call)"
      class="tool-summary" :class="{ 'tool-summary-error': item.call.isError }"
    >{{ toolSummary(item.call) }}</span>
  </template>
  <span v-if="formatTargets(item.call)" class="tool-targets">{{ formatTargets(item.call) }}</span>
  <span v-if="item.call.durationMs != null" class="tool-duration" style="margin-left:auto">{{ formatDuration(item.call.durationMs) }}</span>
  <span v-else class="act-streaming">···</span>
</div>
```

- [ ] **Step 3: Add CSS for new elements**

In the `<style>` section, add:

```css
.tool-badge { background: #1f6feb22; color: #58a6ff; font-size: 10px; padding: 1px 6px; border-radius: 3px; border: 1px solid #1f6feb44; text-transform: uppercase; letter-spacing: 0.5px; }
.tool-badge-error { background: #f8514922; color: #f85149; border-color: #f8514944; }
.tool-args { color: var(--text-sub); font-size: 12px; }
.tool-summary-arrow { color: var(--border); margin: 0 4px; font-size: 11px; }
.tool-summary { color: var(--text-sub); font-size: 12px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tool-summary-error { color: #f85149; }
```

- [ ] **Step 4: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -5
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ChatMessage.vue
git commit -m "feat(frontend): render collapsed tool call with summary arrow"
```

---

### Task 7: Verify end-to-end in browser

**Files:** none (verification only)

- [ ] **Step 1: Build full binary**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-test ./cmd/spider
```

- [ ] **Step 2: Start test server**

```bash
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: Open browser and send a message that triggers tool calls**

Navigate to `http://localhost:8002`. Send a message like "list all hosts". Verify:
- Collapsed: `ListHosts() → N hosts  0ms`
- Click expands to show INPUT/OUTPUT blocks
- Error tools show red summary text

- [ ] **Step 4: Verify skill tool**

Send a message that triggers `invoke_skill`. Verify collapsed shows `invoke_skill("name") → skill "name" loaded`.

- [ ] **Step 5: Stop test server and clean up**

```bash
pkill -f spider-test
rm /tmp/spider-test
```
