# Runtime Status Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Codex-style runtime status bar above ChatView input, showing single-line spinner + verb + context + elapsed + esc hint while agent is processing.

**Architecture:** New `RuntimeStatusBar.vue` component reads from existing `useAgentStatus` composable (extended with `hosts?` and `startedAt?` fields). `ChatView.vue` populates new fields when handling `tool_start` event, mounts component above input area, binds Esc to existing `cancelSend`.

**Tech Stack:** Vue 3 Composition API, TypeScript, scoped CSS. No test framework — verify via `npm run build` + Playwright manual checks (per project convention).

---

## File Structure

| File | Role |
|---|---|
| `web/src/composables/useAgentStatus.ts` | Add `hosts?: string[]`, `startedAt?: number` to `AgentStatus`. Set `startedAt` on phase transition. |
| `web/src/components/RuntimeStatusBar.vue` | New component: spinner + verb + context + elapsed + esc hint. |
| `web/src/views/ChatView.vue` | Extract hosts in `tool_start`, pass to `setStatus`. Mount `<RuntimeStatusBar>` above `.chat-input`. Bind `@keydown.escape` to `cancelSend`. |

Spec: [docs/spec-20260523-0254-runtime-status-bar.md](spec-20260523-0254-runtime-status-bar.md)

---

### Task 1: Extend `AgentStatus` interface

**Files:**
- Modify: `web/src/composables/useAgentStatus.ts:3-10`

- [ ] **Step 1: Add fields to interface**

Open `web/src/composables/useAgentStatus.ts`. Replace the `AgentStatus` interface:

```ts
export interface AgentStatus {
  conversationId: string
  title: string
  phase: 'thinking' | 'tool' | 'confirm' | 'done'
  toolName?: string
  toolInput?: string
  hosts?: string[]
  startedAt?: number
  updatedAt: number
}
```

- [ ] **Step 2: Update `updateAgentStatus` to set `startedAt` on phase transition**

Replace lines 19-46 with:

```ts
export function updateAgentStatus(update: Omit<AgentStatus, 'updatedAt' | 'startedAt'>) {
  const existing = doneTimers.get(update.conversationId)
  if (existing) {
    clearTimeout(existing)
    doneTimers.delete(update.conversationId)
  }

  const prev = statuses.value.get(update.conversationId)
  // Skip reactivity trigger if only phase is 'thinking' and it's already set —
  // text_delta fires per token; avoid a full Map copy on every token.
  if (update.phase === 'thinking' && prev?.phase === 'thinking') {
    return
  }

  const startedAt = prev && prev.phase === update.phase && prev.toolName === update.toolName
    ? prev.startedAt ?? Date.now()
    : Date.now()

  statuses.value.set(update.conversationId, {
    ...update,
    startedAt,
    updatedAt: Date.now(),
  })
  statuses.value = new Map(statuses.value)

  if (update.phase === 'done') {
    const timer = setTimeout(() => {
      removeAgentStatus(update.conversationId)
      doneTimers.delete(update.conversationId)
    }, 3000)
    doneTimers.set(update.conversationId, timer)
  }
}
```

- [ ] **Step 3: Build to verify no TS errors**

Run: `cd web && npm run build`
Expected: `vite build` succeeds with no type errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/composables/useAgentStatus.ts
git commit -m "feat(chat): extend AgentStatus with hosts and startedAt"
```

---

### Task 2: Pass `hosts` through `setStatus` in ChatView

**Files:**
- Modify: `web/src/views/ChatView.vue:547-555` (setStatus signature)
- Modify: `web/src/views/ChatView.vue:613` (tool_start call site)

- [ ] **Step 1: Update `setStatus` to accept hosts**

Replace lines 547-555:

```ts
function setStatus(phase: AgentStatus['phase'], toolName?: string, toolInput?: unknown, hosts?: string[]) {
  updateAgentStatus({
    conversationId: convId,
    title: getConvTitle(convId),
    phase,
    toolName,
    toolInput: toolInput ? JSON.stringify(toolInput) : undefined,
    hosts,
  })
}
```

- [ ] **Step 2: Extract hosts from tool_start event**

Replace line 613:

```ts
setStatus('tool', toolName, event.content?.input, event.content?.host_names)
```

- [ ] **Step 3: Build to verify**

Run: `cd web && npm run build`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): pass tool host_names through setStatus"
```

---

### Task 3: Create `RuntimeStatusBar.vue` component

**Files:**
- Create: `web/src/components/RuntimeStatusBar.vue`

- [ ] **Step 1: Write component file**

Create `web/src/components/RuntimeStatusBar.vue` with full content:

```vue
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import type { AgentStatus } from '../composables/useAgentStatus'

const props = defineProps<{
  status: AgentStatus | null
}>()

const SPINNER = ['✻', '✦', '✶', '✷', '✸', '✹']
const EXPLORE_TOOLS = new Set(['GetHosts', 'SearchDocs', 'Verify', 'GetTopology', 'invoke_skill'])

const spinnerIdx = ref(0)
const elapsedSec = ref(0)
let spinnerTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null

function recomputeElapsed() {
  const t = props.status?.startedAt
  elapsedSec.value = t ? Math.floor((Date.now() - t) / 1000) : 0
}

onMounted(() => {
  spinnerTimer = setInterval(() => {
    spinnerIdx.value = (spinnerIdx.value + 1) % SPINNER.length
  }, 120)
  elapsedTimer = setInterval(recomputeElapsed, 1000)
  recomputeElapsed()
})

onUnmounted(() => {
  if (spinnerTimer) clearInterval(spinnerTimer)
  if (elapsedTimer) clearInterval(elapsedTimer)
})

watch(() => props.status?.startedAt, () => recomputeElapsed())

const visible = computed(() => {
  const s = props.status
  return !!s && s.phase !== 'done'
})

const spinnerChar = computed(() => SPINNER[spinnerIdx.value])

function truncate(s: string, n = 50): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

function formatHosts(hosts: string[] | undefined): string {
  if (!hosts || hosts.length === 0) return ''
  if (hosts.length <= 3) return hosts.join(', ')
  return hosts.slice(0, 3).join(', ') + `, +${hosts.length - 3}`
}

function parseInput(inputJson?: string): Record<string, unknown> | null {
  if (!inputJson) return null
  try {
    const parsed = JSON.parse(inputJson)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null
  } catch { return null }
}

function firstStringValue(inp: Record<string, unknown>): string | null {
  const v = inp.command ?? inp.path ?? inp.query ?? Object.values(inp).find(x => typeof x === 'string')
  return typeof v === 'string' && v ? v : null
}

const verbAndContext = computed<{ verb: string; context: string }>(() => {
  const s = props.status
  if (!s) return { verb: '', context: '' }
  const inp = parseInput(s.toolInput)

  switch (s.phase) {
    case 'thinking':
      return { verb: 'Processing…', context: '' }
    case 'confirm':
      return { verb: 'Awaiting confirm', context: s.toolName ? `· ${s.toolName}` : '' }
    case 'tool': {
      const name = s.toolName || 'unknown'
      if (name === 'RunCommand') {
        const hosts = formatHosts(s.hosts)
        const verb = hosts ? `Running on ${hosts}` : 'Running'
        const cmd = inp ? firstStringValue(inp) : null
        return { verb, context: cmd ? `· ${truncate(cmd)}` : '' }
      }
      if (EXPLORE_TOOLS.has(name)) {
        const arg = inp ? firstStringValue(inp) : null
        return { verb: `Exploring · ${name}`, context: arg ? `· ${truncate(arg)}` : '' }
      }
      return { verb: `Working · ${name}`, context: '' }
    }
    default:
      return { verb: '', context: '' }
  }
})
</script>

<template>
  <div v-if="visible" class="runtime-status-bar">
    <span class="spinner">{{ spinnerChar }}</span>
    <span class="verb">{{ verbAndContext.verb }}</span>
    <span v-if="verbAndContext.context" class="context">{{ verbAndContext.context }}</span>
    <span class="spacer"></span>
    <span class="elapsed">{{ elapsedSec }}s</span>
    <span class="sep">·</span>
    <span class="esc">esc</span>
  </div>
</template>

<style scoped>
.runtime-status-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 16px;
  border-top: 1px solid var(--border);
  background: var(--nav);
  font-family: ui-monospace, 'SF Mono', monospace;
  font-size: 12px;
  color: var(--text-sub);
  overflow: hidden;
  white-space: nowrap;
}

.spinner {
  color: var(--primary);
  font-size: 13px;
  flex-shrink: 0;
}

.verb {
  color: var(--text);
  flex-shrink: 0;
}

.context {
  color: var(--text-sub);
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.spacer {
  flex: 1;
  min-width: 8px;
}

.elapsed,
.sep,
.esc {
  color: var(--muted, var(--text-sub));
  flex-shrink: 0;
  font-size: 11px;
}
</style>
```

- [ ] **Step 2: Build to verify**

Run: `cd web && npm run build`
Expected: build succeeds, no TS errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/RuntimeStatusBar.vue
git commit -m "feat(chat): add RuntimeStatusBar component"
```

---

### Task 4: Mount `RuntimeStatusBar` in ChatView

**Files:**
- Modify: `web/src/views/ChatView.vue` (script imports + template above `.chat-input`)

- [ ] **Step 1: Add component import + computed**

Find the `import ChatMessage from ...` line near line 5 and add right after:

```ts
import RuntimeStatusBar from '../components/RuntimeStatusBar.vue'
```

Find the `useAgentStatus` import block (around line 20). Update to:

```ts
import { updateAgentStatus, useAgentStatus, type AgentStatus } from '../composables/useAgentStatus'
```

Add a computed below the existing `messages` computed (around line 77):

```ts
const { statuses: agentStatuses } = useAgentStatus()
const currentAgentStatus = computed(() => {
  const id = activeConvId.value
  return id ? agentStatuses.value.get(id) ?? null : null
})
```

- [ ] **Step 2: Mount component above `.chat-input`**

Find `<div class="chat-input">` (line 1291). Insert directly above:

```vue
<RuntimeStatusBar v-if="isStreaming" :status="currentAgentStatus" />
```

- [ ] **Step 3: Bind Esc to cancelSend**

Find the `<textarea` block (line 1301-1309). Add `@keydown.escape.stop="cancelSend"`:

```vue
<textarea
  ref="textareaRef"
  v-model="inputText"
  @keydown.enter.exact.prevent="send()"
  @keydown="onTextareaKeydown"
  @keydown.escape.stop="isStreaming && cancelSend()"
  @input="onTextareaInput"
  :placeholder="isStreaming ? '排队发送...' : '输入运维指令...'"
  rows="1"
></textarea>
```

- [ ] **Step 4: Build to verify**

Run: `cd web && npm run build`
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): mount RuntimeStatusBar above input area"
```

---

### Task 5: Build Go binary and verify in browser

**Files:** none (verification only)

- [ ] **Step 1: Force-rebuild Go binary with embedded frontend**

Run from project root:

```bash
go build -a -o /tmp/spider-test ./cmd/spider
```

Expected: binary at `/tmp/spider-test`. Per CLAUDE.md, `go build -a` is required because Go's incremental compile does not track `//go:embed` changes.

- [ ] **Step 2: Verify embedded HTML references new JS bundle**

Run:

```bash
ls web/dist/assets/index-*.js | head -1
```

Then start the server in background:

```bash
/tmp/spider-test serve --addr :8003 --data-dir ~/.spider/data
```

Then verify HTML matches:

```bash
curl -s http://localhost:8003/ | grep -o 'index-[a-z0-9]*\.js'
```

Expected: filename matches `web/dist/assets/index-*.js`. If mismatch, repeat `go build -a`.

- [ ] **Step 3: Manual visual verification with Playwright**

Open `http://localhost:8003/`, log in (`admin` / `12345qwer`), open or create a conversation, send a message that triggers a `RunCommand` (e.g. "查看 local201 的 nginx 状态").

Verify in the browser:

| State | Expected display |
|---|---|
| Sending → thinking | `✻ Processing…  Ns · esc` (spinner cycling) |
| RunCommand on 1 host | `✻ Running on local201 · <cmd> ··· Ns · esc` |
| Explore (GetHosts) | `✻ Exploring · GetHosts ··· Ns · esc` |
| Awaiting confirm | `✻ Awaiting confirm · RunCommand ··· Ns · esc` |
| Done | bar disappears within 1s |

Press Esc while streaming → conversation cancels.

- [ ] **Step 4: Stop test server**

Find PID and kill:

```bash
lsof -ti:8003 | xargs kill
```

- [ ] **Step 5: No commit (verification only)**

If issues found, fix in prior tasks and re-verify. If all pass, proceed to merge.

---

## Self-Review

**Spec coverage:**

| Spec section | Implemented in |
|---|---|
| Verb mapping (Processing/Running on/Exploring/Working/Awaiting confirm) | Task 3 `verbAndContext` computed |
| Spinner char cycle 120ms | Task 3 `spinnerTimer` |
| Multi-host truncate (B) | Task 3 `formatHosts` |
| 50-char truncate | Task 3 `truncate` |
| Elapsed seconds 1000ms | Task 3 `elapsedTimer` |
| Esc → cancelSend | Task 4 Step 3 |
| `hosts` field | Task 1 (interface), Task 2 (populate) |
| `startedAt` reset on phase transition | Task 1 Step 2 |
| Render condition: `isStreaming && phase !== 'done'` | Task 4 Step 2 (`v-if="isStreaming"`) + Task 3 `visible` |
| `text_delta` reactivity optimization preserved | Task 1 Step 2 keeps `if (thinking && prev=thinking) return` |
| invoke_skill in Explore set | Task 3 `EXPLORE_TOOLS` |

**Placeholder scan:** none.

**Type consistency:** `AgentStatus` extended once in Task 1, consumed in Task 3 prop type and Task 4 computed. `setStatus` 4th arg `hosts?: string[]` matches `updateAgentStatus` Omit signature. `EXPLORE_TOOLS` set name local to component, no cross-task collision.
