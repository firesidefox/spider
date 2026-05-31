# Frontend Refactoring Design

## Overview

Refactor Spider.ai frontend to reduce complexity in large view files (ChatView 1803 lines, SettingView 1930 lines) by extracting composables and components. Goal: establish clear boundaries between business domains, isolate state, and improve maintainability.

**Root cause**: No encapsulation boundaries. 83 reactive declarations in ChatView share one scope. SettingView mixes 10 independent business domains (provider, RAG, tokens, SSH keys, etc.) in one component.

**Approach**: Gradual extraction. Extract logic first, move files later. Each phase independently verifiable and reversible.

## Constraints

- Zero behavior change in each phase
- Full verification before next phase
- Single developer workflow (no parallel coordination needed)
- Phase 4 (directory migration) is optional

## Success Criteria

- ChatView < 500 lines
- SettingView < 200 lines (becomes tab router)
- Each business domain has isolated state
- All tests pass after each phase
- No regressions in UI behavior

## Architecture

### Phase 1: API Client (1-2 days)

**Problem**: 13 API files repeat `fetch` + `authHeaders()` + error handling pattern.

**Solution**: Unified API client.

**File**: `shared/api/client.ts`

```typescript
class ApiClient {
  private baseURL = '/api/v1'
  
  async get<T>(path: string): Promise<T>
  async post<T>(path: string, body?: any): Promise<T>
  async patch<T>(path: string, body?: any): Promise<T>
  async delete<T>(path: string): Promise<T>
  async download(path: string): Promise<Blob>
  
  private async request<T>(method: string, path: string, body?: any): Promise<T> {
    const res = await fetch(`${this.baseURL}${path}`, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders(),
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    
    if (res.status === 401) {
      // Clear auth and redirect
      localStorage.removeItem('token')
      window.location.href = '/login'
      throw new ApiError(401, 'Unauthorized')
    }
    
    if (!res.ok) {
      const error = await res.json()
      throw new ApiError(res.status, error.error || 'Request failed')
    }
    
    return res.json()
  }
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

export const api = new ApiClient()
```

**Migration example** (`api/chat.ts`):

Before:
```typescript
export async function createConversation(title?: string): Promise<Conversation> {
  const res = await fetch('/api/v1/chat/conversations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: title ? JSON.stringify({ title }) : undefined,
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}
```

After:
```typescript
export async function createConversation(title?: string): Promise<Conversation> {
  return api.post('/chat/conversations', title ? { title } : undefined)
}
```

**Migration order**:
1. Write `client.ts`
2. Migrate `api/auth.ts` (simplest) to validate pattern
3. Migrate remaining 12 API files one by one
4. Run `npm run build` after each file

**Not changed**:
- `subscribeConversation` returns EventSource, doesn't use client
- SSE URL generation stays as-is

**Verification**:
- `vue-tsc --noEmit`
- `npm run build`
- Playwright: login → create conversation → send message → switch conversation → save provider settings
- Manual: test all API-dependent features

### Phase 2: SettingView Tab Extraction (2-3 days)

**Problem**: 1930 lines, 10 independent business domains share one component scope (76 reactive declarations).

**Solution**: Extract each tab into independent component.

**New directory**: `components/settings/`

**Components**:
```
components/settings/
  PasswordSettings.vue       # Password change form
  TokenSettings.vue          # Token CRUD
  SSHKeySettings.vue         # SSH key CRUD
  LogsViewer.vue             # Logs display
  ProviderSettings.vue       # Provider CRUD + model refresh
  RagSettings.vue            # RAG config + model fetch + validate
  AgentSettings.vue          # Agent settings + permission rules
  NotifyChannelSettings.vue  # Notify channel CRUD
  ChatThemeSettings.vue      # Chat theme selector
  AuditLogs.vue              # Audit logs display
```

**SettingView.vue becomes tab router** (~200 lines):

```vue
<template>
  <div class="setting-view">
    <div class="tabs">
      <button
        v-for="tab in allowedTabs"
        :key="tab"
        :class="{ active: activeTab === tab }"
        @click="activeTab = tab"
      >
        {{ tabTitle[tab] }}
      </button>
    </div>
    
    <div class="tab-content">
      <PasswordSettings v-if="activeTab === 'info'" />
      <TokenSettings v-else-if="activeTab === 'tokens'" />
      <SSHKeySettings v-else-if="activeTab === 'ssh-keys'" />
      <LogsViewer v-else-if="activeTab === 'logs'" />
      <ChatThemeSettings v-else-if="activeTab === 'chat-theme'" />
      <ProviderSettings v-else-if="activeTab === 'settings'" />
      <RagSettings v-else-if="activeTab === 'kb'" />
      <AgentSettings v-else-if="activeTab === 'agent'" />
      <NotifyChannelSettings v-else-if="activeTab === 'notify'" />
      <AuditLogs v-else-if="activeTab === 'audit'" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const { isAdmin } = useAuth()
const route = useRoute()
const router = useRouter()

const allowedTabs = computed(() => {
  const base = ['info', 'tokens', 'ssh-keys', 'logs', 'chat-theme', 'settings', 'kb', 'agent', 'notify']
  return isAdmin.value ? [...base, 'audit'] : base
})

const queryTab = route.query.tab as string
const initialTab = allowedTabs.value.includes(queryTab) ? queryTab : 'info'
const activeTab = ref(initialTab)

const tabTitle = computed(() => ({
  info: '账户信息',
  tokens: 'API Tokens',
  'ssh-keys': 'SSH Keys',
  logs: '日志',
  'chat-theme': '聊天主题',
  settings: 'Provider 设置',
  kb: 'RAG 配置',
  agent: 'Agent 设置',
  notify: '通知渠道',
  audit: '审计日志',
}))

watch(activeTab, (tab) => {
  router.push({ query: { tab } })
})
</script>
```

**Each settings component**:
- Own `<script setup>` scope
- Own state (ref/computed)
- Own API calls via `api` client
- Own error handling
- No props (loads data from API)
- No emits (independent operations)

**Tab switching behavior**:
- Use `v-if` for each tab (component destroyed on switch)
- No state preservation needed (all tabs use "load → edit → save" pattern)
- URL query synced via `watch(activeTab)`

**Extraction order** (simple → complex):
1. `PasswordSettings.vue` — simplest, one form + one API call
2. `ChatThemeSettings.vue` — pure UI, no API
3. `TokenSettings.vue` — standard CRUD
4. `SSHKeySettings.vue` — standard CRUD
5. `LogsViewer.vue` — read-only list
6. `NotifyChannelSettings.vue` — CRUD + toggle
7. `ProviderSettings.vue` — complex: CRUD + model refresh + enable/disable
8. `RagSettings.vue` — complex: model fetch + validate
9. `AgentSettings.vue` — complex: permission rules CRUD
10. `AuditLogs.vue` — read-only list

**Verification after each component**:
- Tab switches correctly
- Component loads data
- CRUD operations work
- Error messages display
- No console errors

**Full verification after all components**:
- `vue-tsc --noEmit`
- `npm run build`
- Playwright: test each tab's primary operation
- Manual: verify all tabs functional

### Phase 3: ChatView Composable Extraction (3-4 days)

**Problem**: 1803 lines, 83 reactive declarations in one scope. Responsibilities mixed: conversation list, SSE streaming, message rendering, todo panel, KB dropdown, device status, sidebar drag, title editing, mode switching.

**Solution**: Extract 3 core composables.

#### 3.1 `composables/useConversationList.ts`

**Responsibility**: Conversation list management.

**State**:
```typescript
const conversations = ref<Conversation[]>([])
const activeConvId = ref<string | null>(null)
const batchMode = ref(false)
const selectedConvIds = ref<Set<string>>(new Set())
const editingConvId = ref<string | null>(null)
const editTitleText = ref('')
const menuOpenConvId = ref<string | null>(null)
```

**Interface**:
```typescript
export function useConversationList() {
  return {
    // State
    conversations: Readonly<Ref<Conversation[]>>
    activeConvId: Readonly<Ref<string | null>>
    batchMode: Readonly<Ref<boolean>>
    selectedConvIds: Readonly<Ref<Set<string>>>
    
    // Operations
    loadConversations(): Promise<void>
    selectConversation(id: string): Promise<void>
    createConversation(title?: string): Promise<string>
    deleteConversation(id: string): Promise<void>
    updateTitle(id: string, title: string): Promise<void>
    
    // Batch operations
    enterBatchMode(): void
    exitBatchMode(): void
    toggleSelectConv(id: string): void
    toggleSelectAll(): void
    batchDelete(): Promise<void>
    
    // Title editing
    startEditConvTitle(id: string, title: string): void
    saveConvTitle(id: string): Promise<void>
    cancelEdit(): void
    editingConvId: Readonly<Ref<string | null>>
    editTitleText: Ref<string>
    
    // Menu
    openConvMenu(id: string): void
    closeConvMenu(): void
    menuOpenConvId: Readonly<Ref<string | null>>
  }
}
```

**Key logic**:
- `loadConversations`: fetch from API, sort by updated_at
- `selectConversation`: set activeConvId, load messages via `useChatStream`
- `createConversation`: POST to API, add to list, select new conversation
- Batch mode: toggle selection, delete multiple conversations

#### 3.2 `composables/useChatStream.ts`

**Responsibility**: SSE streaming, event handling, message management.

**State**:
```typescript
const messagesMap = ref<Record<string, DisplayMessage[]>>({})
const streamingConvIds = ref<Set<string>>(new Set())
const queuedMessages = ref<Map<string, string[]>>(new Map())
const toolCallsCache = new Map<string, any[]>()
const retryState = ref<RetryState | null>(null)
const globalEventSource = ref<EventSource | null>(null)
```

**Interface**:
```typescript
export function useChatStream(activeConvId: Ref<string | null>) {
  return {
    // State
    messages: ComputedRef<DisplayMessage[]>  // Current conversation messages
    isStreaming: ComputedRef<boolean>
    queuedMessages: Readonly<Ref<Map<string, string[]>>>
    retryState: Readonly<Ref<RetryState | null>>
    
    // Operations
    sendMessage(text: string, convId: string): Promise<void>
    cancelSend(convId: string): Promise<void>
    handleConfirm(requestId: string, approved: boolean): Promise<void>
    
    // Internal (called by view)
    startGlobalSSE(): void
    handleConvEvent(convId: string, event: ChatEvent): void
    
    // Lifecycle
    cleanup(): void  // Close EventSource, clear timers
  }
}
```

**Key logic**:
- `messagesMap`: indexed by convId, supports multiple conversations
- `startGlobalSSE`: open persistent EventSource for all conversations
- `handleConvEvent`: dispatch by event type (text_delta, tool_start, tool_result, done, error, todo_update, etc.)
- `buildDisplayMessages`: parse tool_calls JSON, merge into DisplayMessage
- `sendMessage`: POST to API, add user message, start streaming
- Retry logic: exponential backoff on error, countdown display

**Event handling**:
```typescript
function handleConvEvent(convId: string, event: ChatEvent) {
  switch (event.type) {
    case 'text_delta':
      appendTextDelta(convId, event.content.delta)
      break
    case 'tool_start':
      addToolCall(convId, event.content)
      break
    case 'tool_result':
      updateToolResult(convId, event.content)
      break
    case 'done':
      setConversationStreaming(convId, false)
      break
    case 'error':
      handleError(convId, event.content)
      break
    case 'todo_update':
      // Delegate to useTodoPanel
      break
    case 'message':
      addMessage(convId, event.content)
      break
  }
}
```

#### 3.3 `composables/useTodoPanel.ts`

**Responsibility**: Todo tasks and timers.

**State**:
```typescript
const todoTasksMap = ref<Record<string, Map<number, Todo>>>({})
const taskTimers = ref<Map<number, ReturnType<typeof setInterval>>>(new Map())
const taskElapsed = ref<Map<number, number>>(new Map())
const completedFolded = ref(true)
const pendingFolded = ref(true)
const turnUsage = ref<number | null>(null)
```

**Interface**:
```typescript
export function useTodoPanel(activeConvId: Ref<string | null>) {
  return {
    // State
    allTasks: ComputedRef<Todo[]>
    inProgressTasks: ComputedRef<Todo[]>
    pendingTasks: ComputedRef<Todo[]>
    completedTasks: ComputedRef<Todo[]>
    visiblePending: ComputedRef<Todo[]>
    visibleCompleted: ComputedRef<Todo[]>
    hiddenPendingCount: ComputedRef<number>
    
    completedFolded: Ref<boolean>
    pendingFolded: Ref<boolean>
    taskElapsed: Readonly<Ref<Map<number, number>>>
    turnUsage: Readonly<Ref<number | null>>
    
    // Operations
    updateTodoFromEvent(convId: string, todos: Todo[]): void
    setTurnUsage(tokens: number | null): void
    
    // Internal
    startTimer(task: Todo): void
    stopTimer(taskId: number): void
    clearAllTimers(): void
    
    // Formatting
    fmtElapsed(seconds: number): string
    fmtTokens(n: number): string
  }
}
```

**Key logic**:
- `todoTasksMap`: indexed by convId
- `allTasks`: computed from current conversation's todo map
- `startTimer`: setInterval every 1s, update `taskElapsed`
- `stopTimer`: clear interval, remove from map
- `clearAllTimers`: called on unmount
- `updateTodoFromEvent`: called by `useChatStream` on `todo_update` event

#### 3.4 ChatView.vue Remaining Content (~500 lines)

**Keep in view layer**:
- Template layout (sidebar, messages, composer, todo panel)
- KB dropdown logic (tightly coupled to textarea)
- Device status display (interacts with host panel)
- Sidebar drag resize (pure UI)
- Title editing UI (header title)
- Mode dropdown (permission mode)
- Slash commands parsing
- Layout transition animation
- Export menu

**Composable composition**:
```typescript
const { 
  conversations, 
  activeConvId, 
  loadConversations, 
  selectConversation,
  createConversation,
  deleteConversation,
  updateTitle,
  // ... other methods
} = useConversationList()

const { 
  messages, 
  isStreaming, 
  sendMessage, 
  cancelSend,
  handleConfirm,
  startGlobalSSE,
  handleConvEvent,
  cleanup: cleanupStream,
} = useChatStream(activeConvId)

const { 
  allTasks, 
  inProgressTasks, 
  completedTasks,
  completedFolded,
  pendingFolded,
  taskElapsed,
  turnUsage,
  updateTodoFromEvent,
  setTurnUsage,
  clearAllTimers,
} = useTodoPanel(activeConvId)

onMounted(() => {
  loadConversations()
  startGlobalSSE()
})

onUnmounted(() => {
  cleanupStream()
  clearAllTimers()
})
```

**Extraction order**:
1. `useConversationList` — most independent, zero dependencies
2. `useTodoPanel` — independent, only depends on activeConvId
3. `useChatStream` — most complex, depends on activeConvId, calls `updateTodoFromEvent`

**Verification after each composable**:
- Extract composable
- Update ChatView imports
- Test related functionality
- No console errors

**Full verification**:
- `vue-tsc --noEmit`
- `npm run build`
- Playwright: send message → SSE streaming → todo updates → switch conversation → multi-tab sync
- Manual: test all chat features

### Phase 4: Directory Migration (Optional, 1-2 days)

**Condition**: Only if Phase 1-3 complete and directory structure still causes navigation issues.

**Target structure**:
```
web/src/
  app/
    router.ts
    App.vue
  
  shared/
    api/
      client.ts
      auth.ts
      chat.ts
      hosts.ts
      knowledge.ts
      ...
    components/
      CodeBlock.vue
      CodeEditor.vue
      RuntimeStatusBar.vue
      AgentStatusBar.vue
    composables/
      useAuth.ts
      useAgentStatus.ts
      useHighlight.ts
  
  features/
    chat/
      views/ChatView.vue
      components/
        ChatMessage.vue
        TargetPanel.vue
      composables/
        useConversationList.ts
        useChatStream.ts
        useTodoPanel.ts
        useTargetHosts.ts
    
    settings/
      views/SettingView.vue
      components/
        PasswordSettings.vue
        TokenSettings.vue
        ...
    
    knowledge/
      views/KnowledgeView.vue
    
    hosts/
      views/HostsView.vue
    
    topology/
      views/TopologyView.vue
    
    tasks/
      views/TasksView.vue
    
    users/
      views/UsersView.vue
    
    auth/
      views/LoginView.vue
    
    install/
      views/InstallView.vue
  
  main.ts
  theme.ts
  chatTheme.ts
```

**Migration steps**:
1. Create directory structure
2. Move files one feature at a time
3. Update all import paths
4. Update router paths
5. Verify `npm run build`
6. Run full test suite

**Skip Phase 4 if**:
- Current structure sufficient after Phase 1-3
- Single maintainer familiar with structure
- Migration cost > benefit

## Error Handling

### API Client

**401 Unauthorized**:
- Clear local auth token
- Redirect to `/login`
- Don't throw (avoid duplicate handling)

**Other errors**:
- Throw `ApiError` with `status` and `message`
- Caller catches and displays error

**Network errors**:
- Wrap `TypeError` from `fetch` into `ApiError`
- Message: "网络错误，请检查连接"

### SettingView Components

**Shared state**:
- `currentUser` and `isAdmin` from `useAuth()`, no props needed
- No other cross-tab state

**Tab switching**:
- Components destroyed on `v-if` switch
- No state preservation needed (all tabs use load → edit → save pattern)

**URL sync**:
- Watch `activeTab`, update `router.push({ query: { tab } })`
- Restore from `route.query.tab` on mount

### ChatView Composables

**Multi-conversation isolation**:
- `messagesMap` indexed by convId
- `todoTasksMap` indexed by convId
- Switching conversations doesn't clear other conversation state

**SSE reconnection**:
- `startGlobalSSE()` called in `onMounted`
- EventSource auto-reconnects (browser behavior)
- Use `last_event_id` to avoid duplicate messages

**Memory leak prevention**:
- `useTodoPanel`: call `clearAllTimers()` in `onUnmounted`
- `useChatStream`: close EventSource in `cleanup()`
- `useConversationList`: no cleanup needed

**Composable dependencies**:
- `useChatStream` and `useTodoPanel` both depend on `activeConvId`
- Pass as parameter, don't call `useConversationList` inside composables
- Maintain unidirectional dependency, avoid cycles

## Testing Strategy

### Phase 1 Verification

**Type check**:
```bash
vue-tsc --noEmit
```

**Build**:
```bash
npm run build
```

**Playwright core path**:
```typescript
test('API client - login and create conversation', async ({ page }) => {
  await page.goto('http://localhost:8002/login')
  await page.fill('input[type="text"]', 'admin')
  await page.fill('input[type="password"]', '12345qwer')
  await page.click('button[type="submit"]')
  
  await page.waitForURL('**/chat')
  await page.click('text=新建对话')
  await expect(page.locator('.conversation-item')).toHaveCount(1)
})
```

**Manual**:
- Login
- Create conversation
- Send message
- Switch conversation
- Save provider settings

### Phase 2 Verification

**Per-tab test**:
```typescript
test('ProviderSettings - add provider', async ({ page }) => {
  await loginAsAdmin(page)
  await page.goto('http://localhost:8002/setting?tab=settings')
  
  await page.click('text=添加 Provider')
  await page.fill('input[name="name"]', 'test-provider')
  await page.selectOption('select[name="type"]', 'anthropic')
  await page.fill('input[name="api_key"]', 'sk-test')
  await page.click('text=保存')
  
  await expect(page.locator('text=test-provider')).toBeVisible()
})
```

**Regression**:
- All tabs switch correctly
- URL query syncs
- Permission control (non-admin can't see users tab)

### Phase 3 Verification

**SSE streaming**:
```typescript
test('ChatView - send message and receive streaming', async ({ page }) => {
  await loginAsAdmin(page)
  await page.goto('http://localhost:8002/chat')
  
  await page.fill('textarea', 'hello')
  await page.press('textarea', 'Enter')
  
  await expect(page.locator('.message.assistant')).toBeVisible({ timeout: 5000 })
  await expect(page.locator('.streaming-indicator')).toBeHidden({ timeout: 30000 })
})
```

**Multi-tab sync**:
```typescript
test('ChatView - multi-tab sync', async ({ browser }) => {
  const context = await browser.newContext()
  const page1 = await context.newPage()
  const page2 = await context.newPage()
  
  await loginAsAdmin(page1)
  await page1.goto('http://localhost:8002/chat')
  
  await loginAsAdmin(page2)
  await page2.goto('http://localhost:8002/chat')
  
  await page1.fill('textarea', 'test message')
  await page1.press('textarea', 'Enter')
  
  await expect(page2.locator('text=test message')).toBeVisible({ timeout: 5000 })
})
```

**Todo panel**:
```typescript
test('ChatView - todo panel updates', async ({ page }) => {
  await loginAsAdmin(page)
  await page.goto('http://localhost:8002/chat')
  
  await page.fill('textarea', '/loop 1m echo hello')
  await page.press('textarea', 'Enter')
  
  await expect(page.locator('.todo-item')).toBeVisible({ timeout: 5000 })
  
  const elapsed1 = await page.locator('.todo-elapsed').textContent()
  await page.waitForTimeout(2000)
  const elapsed2 = await page.locator('.todo-elapsed').textContent()
  expect(elapsed1).not.toBe(elapsed2)
})
```

### Phase 4 Verification

**Import paths**:
```bash
npm run build
vue-tsc --noEmit
```

**Full regression**:
- Re-run all Phase 1-3 tests
- Manual verification of all pages

### Test Environment

**Start test server**:
```bash
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

**Playwright config**:
```typescript
// playwright.config.ts
export default {
  baseURL: 'http://localhost:8002',
  use: {
    headless: false,
  },
}
```

## Rollback Strategy

If verification fails in any phase:
1. `git reset --hard` to phase start
2. Analyze failure cause
3. Adjust design or implementation
4. Re-execute phase

No "partial completion" allowed. Each phase must fully pass verification before proceeding.

## Timeline

- Phase 1: 1-2 days
- Phase 2: 2-3 days
- Phase 3: 3-4 days
- Phase 4: 1-2 days (optional)

Total: 7-11 days (6-9 days if skipping Phase 4)

## Non-Goals

- Introduce Pinia or other state management library (current composable singleton pattern sufficient)
- Add unit tests for composables (focus on integration tests via Playwright)
- Refactor unrelated code (surgical changes only)
- Change UI design or behavior
- Optimize performance (unless regression detected)

## Future Considerations

After this refactoring:
- Consider Pinia if state dependencies become complex
- Add Vitest for composable unit tests
- Extract more shared UI components (Button, Modal, Dropdown) if duplication emerges
- Evaluate TypeScript strict mode
