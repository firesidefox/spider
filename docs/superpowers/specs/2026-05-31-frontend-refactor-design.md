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
  
  async get<T>(path: string, options?: RequestOptions): Promise<T>
  async post<T>(path: string, body?: any, options?: RequestOptions): Promise<T>
  async patch<T>(path: string, body?: any, options?: RequestOptions): Promise<T>
  async delete<T>(path: string, options?: RequestOptions): Promise<T>
  async download(path: string): Promise<Blob>
  async upload<T>(path: string, formData: FormData): Promise<T>
  
  private async request<T>(method: string, path: string, body?: any, options?: RequestOptions): Promise<T> {
    const headers: Record<string, string> = { ...authHeaders() }
    
    // Auto-detect body type
    let requestBody: any = body
    if (body && !(body instanceof FormData)) {
      headers['Content-Type'] = 'application/json'
      requestBody = JSON.stringify(body)
    }
    // FormData sets its own Content-Type with boundary
    
    // Merge custom headers
    if (options?.headers) {
      Object.assign(headers, options.headers)
    }
    
    const res = await fetch(`${this.baseURL}${path}`, {
      method,
      headers,
      body: requestBody,
    })
    
    if (res.status === 401) {
      // Clear auth state and redirect
      localStorage.removeItem('spider_token')
      // Trigger useAuth refresh to clear currentUser
      window.dispatchEvent(new Event('auth-expired'))
      window.location.href = '/login'
      throw new ApiError(401, 'Unauthorized')
    }
    
    if (!res.ok) {
      const contentType = res.headers.get('content-type')
      if (contentType?.includes('application/json')) {
        const error = await res.json()
        throw new ApiError(res.status, error.error || 'Request failed')
      }
      throw new ApiError(res.status, `HTTP ${res.status}`)
    }
    
    // Handle response based on type
    const responseType = options?.responseType || 'json'
    switch (responseType) {
      case 'void':
        return undefined as T
      case 'text':
        return (await res.text()) as T
      case 'blob':
        return (await res.blob()) as T
      case 'json':
      default:
        return res.json()
    }
  }
}

interface RequestOptions {
  headers?: Record<string, string>
  responseType?: 'json' | 'text' | 'blob' | 'void'
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

export const api = new ApiClient()
```

**Usage examples**:

```typescript
// JSON request/response (default)
await api.post('/chat/conversations', { title: 'New chat' })

// FormData upload
const formData = new FormData()
formData.append('file', file)
await api.upload('/knowledge/documents', formData)

// Custom headers (e.g., YAML)
await api.post('/topology/import', yamlContent, {
  headers: { 'Content-Type': 'application/x-yaml' }
})

// Blob download
const blob = await api.download('/topology/export')

// Void response (delete, logout, prefs update)
await api.delete('/chat/conversations/123', { responseType: 'void' })
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
  UsersPanel.vue             # User management (admin only)
  InstallPanel.vue           # Install panel (admin only)
  SkillsPanel.vue            # Skills management (admin only)
  PrometheusDataSourcesPanel.vue  # Prometheus datasources (admin only)
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
      <UsersPanel v-else-if="activeTab === 'users'" />
      <InstallPanel v-else-if="activeTab === 'install'" />
      <SkillsPanel v-else-if="activeTab === 'skills'" />
      <PrometheusDataSourcesPanel v-else-if="activeTab === 'datasources'" />
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
  return isAdmin.value ? [...base, 'audit', 'users', 'install', 'skills', 'datasources'] : base
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
  users: '用户管理',
  install: '安装',
  skills: 'Skills',
  datasources: 'Prometheus 数据源',
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
7. `UsersPanel.vue` — user management (already exists as separate component, move to settings/)
8. `InstallPanel.vue` — install panel (already exists, move to settings/)
9. `SkillsPanel.vue` — skills management (already exists, move to settings/)
10. `PrometheusDataSourcesPanel.vue` — datasources (already exists, move to settings/)
11. `ProviderSettings.vue` — complex: CRUD + model refresh + enable/disable
12. `RagSettings.vue` — complex: model fetch + validate
13. `AgentSettings.vue` — complex: permission rules CRUD
14. `AuditLogs.vue` — read-only list (already exists as AuditView, move to settings/)

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
- `selectConversation`: set activeConvId (ChatView watches this and loads messages)
- `createConversation`: POST to API, add to list, select new conversation
- Batch mode: toggle selection, delete multiple conversations

**Note**: `selectConversation` only updates `activeConvId`. Message loading is handled by ChatView watching `activeConvId` and calling `useChatStream` methods. This avoids circular dependency between composables.

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
export function useChatStream(
  activeConvId: Ref<string | null>,
  callbacks?: {
    onAgentStatusUpdate?: (status: any) => void
    onDeviceStatusUpdate?: (hostName: string, status: string) => void
    onTodoUpdate?: (convId: string, todos: Todo[]) => void
    onScrollToBottom?: () => void
  }
) {
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
    
    // Lifecycle
    cleanup(): void  // Close EventSource, clear timers
  }
}
```

**Key logic**:
- `messagesMap`: indexed by convId, supports multiple conversations
- `startGlobalSSE`: open persistent EventSource to `/api/v1/stream` (global stream for all conversations)
- Event handling: dispatch by event type, delegate side effects via callbacks
- `buildDisplayMessages`: parse tool_calls JSON, merge into DisplayMessage
- `sendMessage`: POST to API, add user message, start streaming
- Retry logic: exponential backoff on error, countdown display

**Event handling pattern**:
```typescript
function handleConvEvent(convId: string, event: ChatEvent) {
  switch (event.type) {
    case 'text_delta':
      appendTextDelta(convId, event.content.delta)
      callbacks?.onScrollToBottom?.()
      break
    case 'tool_start':
      addToolCall(convId, event.content)
      if (SSH_TOOLS.has(event.content.tool_name)) {
        callbacks?.onDeviceStatusUpdate?.(event.content.host, 'executing')
      }
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
      callbacks?.onTodoUpdate?.(convId, event.content.todos)
      break
    case 'message':
      addMessage(convId, event.content)
      break
    case 'agent_status':
      callbacks?.onAgentStatusUpdate?.(event.content)
      break
  }
}
```

**SSE stream model**: Uses global `/api/v1/stream` EventSource (no `last_event_id` parameter). Per-conversation subscriptions removed in recent refactor. Messages loaded from DB on conversation select, then global stream provides real-time updates.

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

const { 
  messages, 
  isStreaming, 
  sendMessage, 
  cancelSend,
  handleConfirm,
  startGlobalSSE,
  cleanup: cleanupStream,
} = useChatStream(activeConvId, {
  onTodoUpdate: updateTodoFromEvent,
  onAgentStatusUpdate: (status) => {
    // Update agent status display
  },
  onDeviceStatusUpdate: (hostName, status) => {
    // Update device status in host panel
  },
  onScrollToBottom: () => {
    // Scroll messages to bottom
  },
})

// Watch activeConvId to load messages when conversation changes
watch(activeConvId, async (newId) => {
  if (newId) {
    const { messages } = await getConversation(newId)
    // useChatStream will update messagesMap
  }
})

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
1. `useConversationList` — most independent, only manages list and activeConvId
2. `useTodoPanel` — independent, only depends on activeConvId
3. `useChatStream` — most complex, depends on activeConvId, uses callbacks for side effects

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
- Clear local auth token (`spider_token`)
- Dispatch `auth-expired` event to trigger `useAuth` state refresh
- Redirect to `/login`
- Don't throw (avoid duplicate handling)

**Implementation**:
```typescript
if (res.status === 401) {
  localStorage.removeItem('spider_token')
  window.dispatchEvent(new Event('auth-expired'))
  window.location.href = '/login'
  throw new ApiError(401, 'Unauthorized')
}
```

**Note**: Current auth uses cookies (auto-sent by browser). `spider_token` in localStorage is for display state only. The `auth-expired` event ensures `useAuth` composable clears `currentUser` state.

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
- Opens `/api/v1/stream` (global stream, no per-conversation subscriptions)
- EventSource auto-reconnects (browser behavior)
- No `last_event_id` parameter (messages loaded from DB on conversation select)

**Memory leak prevention**:
- `useTodoPanel`: call `clearAllTimers()` in `onUnmounted`
- `useChatStream`: close EventSource in `cleanup()`
- `useConversationList`: no cleanup needed

**Composable dependencies**:
- `useChatStream` and `useTodoPanel` both depend on `activeConvId`
- Pass as parameter, don't call `useConversationList` inside composables
- Side effects (agent status, device status, scroll) injected via callbacks
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
1. Create a temporary branch or commit before starting each phase
2. On failure, use `git reset --mixed HEAD~1` to unstage changes while preserving working directory
3. Or create a reverse patch: `git diff > /tmp/phase-N.patch` then `git apply -R /tmp/phase-N.patch`
4. Analyze failure cause
5. Adjust design or implementation
6. Re-execute phase

**Never use `git reset --hard`** — it destroys uncommitted work and violates safety constraints.

Each phase should be committed incrementally (e.g., after extracting each settings component). Small commits make rollback surgical.

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
