# Tool Call Intent Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `intent` field to ACT-class tools (RunCommand, RunCommandBatch, CallAPI) so the agent describes its goal in plain language, and display it in the UI as `(hostA, hostB +N台) -> <goal>`.

**Architecture:** Three-part change — (1) Go InputSchema adds `intent` field + Description updated + system prompt guidance added; (2) Go Execute() logs a warning when `intent` is missing; (3) Vue ChatMessage.vue renders the intent line above the INPUT/OUTPUT detail for ACT tools.

**Tech Stack:** Go 1.21, Vue 3 (Composition API), TypeScript

---

## File Map

| File | Change |
|------|--------|
| `internal/agent/tools_cli.go` | Add `intent` to InputSchema, update Description, warn on missing |
| `internal/agent/tools_batch.go` | Add `intent` to InputSchema, update Description, warn on missing |
| `internal/agent/tools_api.go` | Add `intent` to InputSchema, update Description, warn on missing |
| `internal/agent/factory.go` | Add intent guidance to `toolBehaviorPrompt` |
| `web/src/components/ChatMessage.vue` | Render intent line for ACT tools |

No new files needed.

---

### Task 1: Add `intent` to ExecuteCLITool (RunCommand)

**Files:**
- Modify: `internal/agent/tools_cli.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/tools_verify_test.go` — wait, CLI tool has no dedicated test file. Add a new test file `internal/agent/tools_cli_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"
)

func TestExecuteCLITool_InputSchema_HasIntent(t *testing.T) {
	tool := &ExecuteCLITool{}
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["intent"]; !ok {
		t.Error("intent field missing from InputSchema")
	}
	required, _ := schema["required"].([]string)
	for _, r := range required {
		if r == "intent" {
			t.Error("intent should NOT be in required (warn-only, not hard required)")
		}
	}
}

func TestExecuteCLITool_Description_MentionsIntent(t *testing.T) {
	tool := &ExecuteCLITool{}
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/ -run TestExecuteCLITool -v
```

Expected: FAIL — `intent field missing from InputSchema`

- [ ] **Step 3: Add `intent` to InputSchema and update Description**

In `internal/agent/tools_cli.go`, update `InputSchema()`:

```go
func (t *ExecuteCLITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_id":    map[string]any{"type": "string", "description": "Target host ID or name"},
			"command":    map[string]any{"type": "string", "description": "Shell command to execute"},
			"risk_level": map[string]any{"type": "string", "enum": []string{"L1", "L2", "L3", "L4"}, "description": "Risk level. L1=read-only, L2=standard change, L3=destructive, L4=critical"},
			"intent":     map[string]any{"type": "string", "description": "What you are trying to achieve with this command (goal only, no device names). Required for L2/L3/L4."},
		},
		"required": []string{"host_id", "command"},
	}
}
```

Update `Description()`:

```go
func (t *ExecuteCLITool) Description() string {
	return "Execute a CLI command on a remote host via SSH. Has side effects. Use only after confirming intent in Plan phase. Always set `intent` to a short goal description (e.g. \"重启 nginx 使配置生效\")."
}
```

- [ ] **Step 4: Add warn log in Execute() when intent missing**

In `Execute()`, after extracting `hostID` and `command`, add:

```go
intent, _ := input["intent"].(string)
if intent == "" {
	log.Printf("WARNING: RunCommand called without intent field (host=%s)", hostID)
}
```

Add `"log"` to imports if not present.

- [ ] **Step 5: Run tests**

```
go test ./internal/agent/ -run TestExecuteCLITool -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_cli.go internal/agent/tools_cli_test.go
git commit -m "feat: add intent field to ExecuteCLITool (RunCommand)"
```

---

### Task 2: Add `intent` to BatchExecuteTool (RunCommandBatch)

**Files:**
- Modify: `internal/agent/tools_batch.go`
- Create: `internal/agent/tools_batch_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/agent/tools_batch_test.go`:

```go
package agent

import (
	"strings"
	"testing"
)

func TestBatchExecuteTool_InputSchema_HasIntent(t *testing.T) {
	tool := &BatchExecuteTool{}
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["intent"]; !ok {
		t.Error("intent field missing from InputSchema")
	}
	required, _ := schema["required"].([]string)
	for _, r := range required {
		if r == "intent" {
			t.Error("intent should NOT be in required")
		}
	}
}

func TestBatchExecuteTool_Description_MentionsIntent(t *testing.T) {
	tool := &BatchExecuteTool{}
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/agent/ -run TestBatchExecuteTool -v
```

Expected: FAIL

- [ ] **Step 3: Update InputSchema and Description**

In `internal/agent/tools_batch.go`, update `InputSchema()`:

```go
func (t *BatchExecuteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of host IDs to target (use instead of tag)"},
			"tag":        map[string]any{"type": "string", "description": "Target all hosts with this tag (use instead of host_ids)"},
			"command":    map[string]any{"type": "string", "description": "Shell command to execute on all hosts"},
			"risk_level": map[string]any{"type": "string", "enum": []string{"L1", "L2", "L3", "L4"}, "description": "Risk level. L1=read-only, L2=standard change, L3=destructive, L4=critical"},
			"intent":     map[string]any{"type": "string", "description": "What you are trying to achieve (goal only, no device names). Required for L2/L3/L4."},
		},
		"required": []string{"command"},
	}
}
```

Update `Description()`:

```go
func (t *BatchExecuteTool) Description() string {
	return "Execute a CLI command on multiple hosts in parallel. Has side effects. Use only after confirming intent in Plan phase. Always set `intent` to a short goal description (e.g. \"重启 nginx 使配置生效\")."
}
```

- [ ] **Step 4: Add warn log in Execute()**

After `command, _ := input["command"].(string)`, add:

```go
intent, _ := input["intent"].(string)
if intent == "" {
	log.Printf("WARNING: RunCommandBatch called without intent field")
}
```

Add `"log"` to imports.

- [ ] **Step 5: Run tests**

```
go test ./internal/agent/ -run TestBatchExecuteTool -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_batch.go internal/agent/tools_batch_test.go
git commit -m "feat: add intent field to BatchExecuteTool (RunCommandBatch)"
```

---

### Task 3: Add `intent` to CallRESTAPITool (CallAPI)

**Files:**
- Modify: `internal/agent/tools_api.go`

- [ ] **Step 1: Write failing test**

In `internal/agent/tools_api_test.go` (file already exists — check first, then add):

```go
func TestCallRESTAPITool_InputSchema_HasIntent(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["intent"]; !ok {
		t.Error("intent field missing from InputSchema")
	}
}

func TestCallRESTAPITool_Description_MentionsIntent(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/agent/ -run TestCallRESTAPITool_InputSchema_HasIntent -v
```

Expected: FAIL

- [ ] **Step 3: Update InputSchema and Description**

In `internal/agent/tools_api.go`, update `InputSchema()` to add `intent`:

```go
func (t *CallRESTAPITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":     map[string]any{"type": "string", "description": "Full URL to call"},
			"method":  map[string]any{"type": "string", "description": "HTTP method", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
			"headers": map[string]any{"type": "object", "description": "HTTP headers"},
			"body":    map[string]any{"type": "string", "description": "Request body"},
			"face_id": map[string]any{"type": "string", "description": "Optional. Access face ID. If provided, auth headers are injected automatically from the stored credentials."},
			"intent":  map[string]any{"type": "string", "description": "What you are trying to achieve with this API call (goal only). Required for POST/PUT/DELETE/PATCH."},
		},
		"required": []string{"method"},
	}
}
```

Update `Description()`:

```go
func (t *CallRESTAPITool) Description() string {
	return "Call a REST API endpoint on a gateway device. Has side effects for POST/PUT/DELETE methods. Use GET freely in Explore phase; use mutating methods only in Act phase after confirming intent. Always set `intent` for mutating calls."
}
```

- [ ] **Step 4: Add warn log in Execute()**

After `method, _ := input["method"].(string)`, add:

```go
intent, _ := input["intent"].(string)
if intent == "" && method != "GET" {
	log.Printf("WARNING: CallAPI called without intent field (method=%s)", method)
}
```

Add `"log"` to imports.

- [ ] **Step 5: Run tests**

```
go test ./internal/agent/ -run TestCallRESTAPITool -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_api.go internal/agent/tools_api_test.go
git commit -m "feat: add intent field to CallRESTAPITool (CallAPI)"
```

---

### Task 4: Add intent guidance to system prompt

**Files:**
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Add intent guidance block to `toolBehaviorPrompt`**

In `internal/agent/factory.go`, append to the end of `toolBehaviorPrompt` (before the closing backtick):

```go
### Intent Field (RunCommand / RunCommandBatch / CallAPI)

Always set the `intent` field when calling RunCommand, RunCommandBatch, or CallAPI. This field is shown to the user in the UI so they understand what you are doing.

**Rules:**
- Write the goal only — do not include device names (the UI adds those automatically)
- Keep it short: 10 Chinese characters or fewer is ideal
- Use plain language the user can read at a glance

<example>
Good: `"重启 nginx 使配置生效"`
Good: `"清理 30 天前的日志"`
Good: `"推送新 ACL 规则"`
Bad: `"在 local110 和 local201 上重启 nginx"` — device names belong in host_ids, not intent
Bad: `"execute the restart command"` — too vague, not user-facing language
</example>
```

- [ ] **Step 2: Build to verify no compile errors**

```
go build ./internal/agent/
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/agent/factory.go
git commit -m "feat: add intent field guidance to agent system prompt"
```

---

### Task 5: Render intent line in ChatMessage.vue

**Files:**
- Modify: `web/src/components/ChatMessage.vue`

- [ ] **Step 1: Add `formatIntent` helper function**

In the `<script setup>` section of `ChatMessage.vue`, add after `formatDuration`:

```typescript
const ACT_TOOLS_WITH_HOSTS = new Set(['RunCommand', 'RunCommandBatch'])

function formatTargets(call: ToolCallBlock): string {
  if (!call.input) return ''
  // Single host
  const hostId = call.input['host_id']
  if (typeof hostId === 'string' && hostId) return `(${hostId})`
  // Multiple hosts
  const hostIds = call.input['host_ids']
  if (Array.isArray(hostIds) && hostIds.length > 0) {
    const shown = hostIds.slice(0, 2).join(', ')
    const extra = hostIds.length - 2
    return extra > 0 ? `(${shown} +${extra}台)` : `(${shown})`
  }
  return ''
}

function formatIntentLine(call: ToolCallBlock): string {
  const intent = call.input?.['intent']
  if (!intent) return ''
  const targets = formatTargets(call)
  const line = targets ? `${targets} -> ${intent}` : `-> ${intent}`
  return line.length > 60 ? line.slice(0, 60) + '...' : line
}

function fullIntentLine(call: ToolCallBlock): string {
  const intent = call.input?.['intent']
  if (!intent) return ''
  const targets = formatTargets(call)
  return targets ? `${targets} -> ${intent}` : `-> ${intent}`
}
```

- [ ] **Step 2: Add expand state for intent lines**

Add a new ref after `collapsedGroups`:

```typescript
const expandedIntents = ref<Set<string>>(new Set())
function toggleIntent(id: string) { toggle(expandedIntents.value, id) }
```

- [ ] **Step 3: Add intent line to ACT tool template**

In the `<!-- Act tool -->` section, add the intent line inside `.tool-call`, before `.tool-header`:

```html
<!-- Intent line -->
<div v-if="formatIntentLine(item.call)" class="tool-intent">
  <span class="intent-text" @click="toggleIntent(item.call.id)">
    {{ expandedIntents.has(item.call.id) ? fullIntentLine(item.call) : formatIntentLine(item.call) }}
  </span>
</div>
```

- [ ] **Step 4: Add CSS for intent line**

In `<style scoped>`, add:

```css
.tool-intent { padding: 4px 8px 2px 8px; }
.intent-text {
  font-size: 12px;
  color: var(--text-muted, #888);
  cursor: pointer;
  font-family: 'SF Mono', 'Fira Code', monospace;
}
.intent-text:hover { color: var(--text); }
```

- [ ] **Step 5: Build frontend**

```
cd /Users/cw/fty.ai/spider.ai/web && npm run build
```

Expected: no errors

- [ ] **Step 6: Verify in browser**

```
go build -a -o /tmp/spider-test ./cmd/spider && /tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

Open http://localhost:8002, send a message that triggers RunCommand or RunCommandBatch. Verify:
1. Intent line appears above the tool header: `(hostA) -> <goal>`
2. If intent > 60 chars, it truncates with `...`
3. Clicking the truncated line expands to full text
4. If agent omits `intent`, no intent line appears (no crash)

- [ ] **Step 7: Commit**

```bash
git add web/src/components/ChatMessage.vue
git commit -m "feat: render intent line in ACT tool call UI"
```

---

## Acceptance Checklist

- [ ] `go test ./internal/agent/ -v` passes
- [ ] `npm run build` in `web/` passes
- [ ] RunCommand/RunCommandBatch show `(hostA, hostB +N台) -> <goal>` in UI
- [ ] CallAPI shows `-> <goal>` (no host prefix)
- [ ] Intent > 60 chars truncates, click expands
- [ ] Missing intent: no crash, no intent line shown
- [ ] Backend logs `WARNING` when ACT tool called without intent
