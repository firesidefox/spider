# Agent Status Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a global footer bar to App.vue that shows real-time agent conversation status across all pages.

**Architecture:** A module-level singleton composable (`useAgentStatus`) holds a `Map<conversationId, AgentStatus>`. ChatView updates it on EventSource events. `AgentStatusBar.vue` renders the footer — one row per active conversation, max 3 rows visible with vertical scroll.

**Tech Stack:** Vue 3 Composition API, TypeScript, CSS variables from existing theme system

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `web/src/composables/useAgentStatus.ts` | Create | Singleton state + update/remove functions |
| `web/src/components/AgentStatusBar.vue` | Create | Footer UI — renders sorted rows, handles click |
| `web/src/App.vue` | Modify | Mount `<AgentStatusBar>`, add footer layout CSS |
| `web/src/views/ChatView.vue` | Modify | Call `updateAgentStatus()` in `handleConvEvent` |

---

## Task 1: Create `useAgentStatus` composable

**Files:**
- Create: `web/src/composables/useAgentStatus.ts`

- [ ] **Step 1: Write the file**

```ts
import { ref, readonly } from 'vue'

export interface AgentStatus {
  conversationId: string
  title: string
  phase: 'thinking' | 'tool' | 'confirm' | 'done'
  toolName?: string
  toolInput?: string
  updatedAt: number
}

const statuses = ref<Map<string, AgentStatus>>(new Map())
const doneTimers = new Map<string, ReturnType<typeof setTimeout>>()

export function useAgentStatus() {
  return { statuses: readonly(statuses) }
}

export function updateAgentStatus(update: Omit<AgentStatus, 'updatedAt'>) {
  // Cancel any pending done-timer for this conversation
  const existing = doneTimers.get(update.conversationId)
  if (existing) {
    clearTimeout(existing)
    doneTimers.delete(update.conversationId)
  }

  statuses.value.set(update.conversationId, {
    ...update,
    updatedAt: Date.now(),
  })
  // Trigger reactivity — Map mutations don't auto-trigger
  statuses.value = new Map(statuses.value)

  if (update.phase === 'done') {
    const timer = setTimeout(() => {
      removeAgentStatus(update.conversationId)
      doneTimers.delete(update.conversationId)
    }, 3000)
    doneTimers.set(update.conversationId, timer)
  }
}

export function removeAgentStatus(conversationId: string) {
  const timer = doneTimers.get(conversationId)
  if (timer) {
    clearTimeout(timer)
    doneTimers.delete(conversationId)
  }
  statuses.value.delete(conversationId)
  statuses.value = new Map(statuses.value)
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

export function formatToolDetail(name: string, input: unknown): string {
  if (!input || typeof input !== 'object') return name
  const inp = input as Record<string, unknown>
  // Show most useful field: command > path > first string value
  const val = inp.command ?? inp.path ?? Object.values(inp).find(v => typeof v === 'string')
  if (!val) return name
  return `${name}: ${truncate(String(val), 40)}`
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add web/src/composables/useAgentStatus.ts
git commit -m "feat: useAgentStatus composable — singleton map, update/remove/format"
```

---

## Task 2: Create `AgentStatusBar.vue`

**Files:**
- Create: `web/src/components/AgentStatusBar.vue`

- [ ] **Step 1: Write the component (template + script)**

```vue
<template>
  <div v-if="sorted.length > 0" class="agent-status-bar">
    <div
      v-for="s in sorted"
      :key="s.conversationId"
      class="agent-status-row"
      :class="{ 'is-current': s.conversationId === currentConvId }"
      @click="handleClick(s.conversationId)"
    >
      <span class="status-dot" :class="dotClass(s.phase)" />
      <span class="status-title">{{ s.title }}</span>
      <span class="status-sep">·</span>
      <span class="status-detail" :class="{ monospace: s.phase === 'tool' || s.phase === 'confirm' }">
        {{ rowDetail(s) }}
      </span>
      <span class="status-arrow">→</span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Write the script section**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAgentStatus, removeAgentStatus, formatToolDetail, type AgentStatus } from '../composables/useAgentStatus'

const { statuses } = useAgentStatus()
const router = useRouter()
const route = useRoute()

const currentConvId = computed(() => route.query.id as string | undefined)

const sorted = computed<AgentStatus[]>(() => {
  const all = Array.from(statuses.value.values())
  return all.sort((a, b) => {
    if (a.conversationId === currentConvId.value) return -1
    if (b.conversationId === currentConvId.value) return 1
    return b.updatedAt - a.updatedAt
  })
})

function dotClass(phase: AgentStatus['phase']) {
  if (phase === 'thinking' || phase === 'tool') return 'dot-running'
  if (phase === 'confirm') return 'dot-confirm'
  return 'dot-done'
}

function rowDetail(s: AgentStatus): string {
  switch (s.phase) {
    case 'thinking': return '思考中'
    case 'tool': return s.toolName
      ? formatToolDetail(s.toolName, s.toolInput ? JSON.parse(s.toolInput) : {})
      : '执行中'
    case 'confirm': return `等待确认 · ${s.toolName
      ? formatToolDetail(s.toolName, s.toolInput ? JSON.parse(s.toolInput) : {})
      : ''}`
    case 'done': return '完成'
  }
}

function handleClick(convId: string) {
  removeAgentStatus(convId)
  router.push(`/chat?id=${convId}`)
}
</script>
```

- [ ] **Step 3: Write the style section**

```vue
<style scoped>
.agent-status-bar {
  border-top: 1px solid var(--border);
  background: var(--nav);
  max-height: 84px;
  overflow-y: auto;
  scrollbar-width: thin;
}

.agent-status-row {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 16px;
  cursor: pointer;
  font-size: 12px;
  color: var(--text-sub);
  transition: background 0.15s;
}

.agent-status-row:hover {
  background: var(--row-hover);
}

.agent-status-row.is-current {
  background: var(--row-alt);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-running {
  background: var(--primary);
  animation: status-pulse 1.2s ease-in-out infinite;
}

.dot-confirm {
  background: var(--yellow);
}

.dot-done {
  background: var(--green);
}

@keyframes status-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.status-title {
  color: var(--text);
  font-weight: 500;
  flex-shrink: 0;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-sep {
  flex-shrink: 0;
}

.status-detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.status-detail.monospace {
  font-family: ui-monospace, monospace;
  font-size: 11px;
}

.status-arrow {
  flex-shrink: 0;
  margin-left: auto;
  color: var(--text-sub);
  font-size: 11px;
}
</style>
```

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add web/src/components/AgentStatusBar.vue
git commit -m "feat: AgentStatusBar component — multi-row, sorted, scrollable"
```

---

## Task 3: Wire AgentStatusBar into App.vue

**Files:**
- Modify: `web/src/App.vue`

- [ ] **Step 1: Add import and component to template**

In `web/src/App.vue`, add the import after the existing imports in `<script setup>`:

```ts
import AgentStatusBar from './components/AgentStatusBar.vue'
```

In the template, add `<AgentStatusBar />` between `</main>` and `</div>`:

```html
    </main>
    <AgentStatusBar v-if="route.path !== '/login'" />
  </div>
```

- [ ] **Step 2: Verify build passes**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: `✓ built in` with no errors

- [ ] **Step 3: Commit**

```bash
git add web/src/App.vue
git commit -m "feat: mount AgentStatusBar in App.vue"
```

---

## Task 4: Wire ChatView into useAgentStatus

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Add import**

In `web/src/views/ChatView.vue`, add to the existing imports block:

```ts
import { updateAgentStatus, formatToolDetail } from '../composables/useAgentStatus'
```

- [ ] **Step 2: Add status updates in `handleConvEvent`**

Find the `switch (event.type)` block in `handleConvEvent`. Add status update calls inside each relevant case. The function signature is `handleConvEvent(convId: string, event: ChatEvent)`.

First, find where conversation title is available. Add a helper at the top of `<script setup>`:

```ts
function getConvTitle(convId: string): string {
  return conversations.value.find(c => c.id === convId)?.title || convId.slice(0, 8)
}
```

Then add calls inside the switch cases:

```ts
case 'text_delta': {
  // ... existing code ...
  updateAgentStatus({
    conversationId: convId,
    title: getConvTitle(convId),
    phase: 'thinking',
  })
  break
}

case 'tool_start': {
  // ... existing code ...
  const toolName = event.content?.name || 'unknown'
  const toolInput = event.content?.input
  updateAgentStatus({
    conversationId: convId,
    title: getConvTitle(convId),
    phase: 'tool',
    toolName,
    toolInput: toolInput ? JSON.stringify(toolInput) : undefined,
  })
  break
}

case 'confirm_required': {
  // ... existing code ...
  const tool = event.content?.tool || ''
  const input = event.content?.input
  updateAgentStatus({
    conversationId: convId,
    title: getConvTitle(convId),
    phase: 'confirm',
    toolName: tool,
    toolInput: input ? JSON.stringify(input) : undefined,
  })
  break
}

case 'done': {
  // ... existing code ...
  updateAgentStatus({
    conversationId: convId,
    title: getConvTitle(convId),
    phase: 'done',
  })
  break
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```
Expected: no errors

- [ ] **Step 4: Build**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: `✓ built in` with no errors

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat: wire ChatView events into useAgentStatus"
```

---

## Task 5: Verify live in browser

- [ ] **Step 1: Start dev server**

```bash
cd /Users/cw/fty.ai/spider.ai && go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: Open browser and start a conversation**

Navigate to `http://localhost:8002/chat`, start a new conversation, send a message that triggers tool use (e.g., "列出 local110 上的 nginx 进程").

- [ ] **Step 3: Navigate away while agent runs**

While the agent is streaming, click "主机管理" in the nav. Verify:
- Status bar appears at bottom with the conversation title
- Dot pulses purple
- Tool detail updates as tools execute
- Clicking the status bar row navigates back to the conversation

- [ ] **Step 4: Verify done state**

After agent finishes, verify:
- Dot turns green, text shows "完成"
- After 3 seconds, the row disappears
- Footer hides when no active conversations

- [ ] **Step 5: Commit if any fixes needed, then final commit**

```bash
git add -p
git commit -m "fix: agent status bar — live verification fixes"
```
