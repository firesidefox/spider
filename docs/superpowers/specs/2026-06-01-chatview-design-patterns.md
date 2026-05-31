# ChatView 设计模式优化 Spec

## 概述

在现有 Phase 1-2 重构基础上（API client + SettingView 提取），完成 Phase 3：提取 ChatView composables，并应用设计模式强化架构。

**目标**：
- ChatView 从 1803 行降到 ~500 行
- 应用 Strategy Pattern 处理 SSE 事件
- 应用 State Pattern 管理会话状态
- 应用 Observer Pattern 统一事件分发

**当前状态**：
- ChatView: 1803 行（83 个 reactive 声明混在一个作用域）
- SSE 事件处理：300+ 行 switch-case
- 会话状态隐式（通过 `streamingConvIds` Set 判断）
- 回调注入方式耦合组件

## 架构设计

### 目录结构

```
web/src/composables/chat/
  useConversationList.ts       # 会话列表管理
  useChatStream.ts              # SSE 流 + 消息管理
  useTodoPanel.ts               # 任务面板
  
  states/                       # State Pattern
    ConversationState.ts        # 状态接口
    BaseConversationState.ts    # 基类
    IdleState.ts                # 空闲状态
    StreamingState.ts           # 流式状态
    WaitingConfirmState.ts      # 等待确认状态
    
  handlers/                     # Strategy Pattern
    EventHandler.ts             # 事件处理器接口
    EventHandlerRegistry.ts     # 注册表
    TextDeltaHandler.ts
    ToolStartHandler.ts
    ToolResultHandler.ts
    ConfirmRequiredHandler.ts
    ErrorHandler.ts
    DoneHandler.ts
    TodoUpdateHandler.ts
    TurnUsageHandler.ts
    MessageHandler.ts
    
  events/                       # Observer Pattern
    ChatEventBus.ts             # 事件总线
```

### 设计模式应用

#### 1. Strategy Pattern - SSE 事件处理

**问题**：当前 `handleConvEvent` 函数 300+ 行 switch-case，新增事件类型需修改核心逻辑。

**解决方案**：每种事件类型一个 handler 类。

**接口**：

```typescript
export interface EventHandlerContext {
  convId: string
  messagesMap: Ref<Record<string, DisplayMessage[]>>
  activeConvId: Ref<string | null>
  getConvTitle: (id: string) => string
  setConversationStreaming: (id: string, streaming: boolean) => void
  scrollToBottom: () => void
  markDevicesExecuting?: (hosts: string[]) => void
  markDevicesDone?: (hosts: string[], failed: boolean) => void
  updateAgentStatus?: (status: AgentStatusUpdate) => void
  clearRetryState?: () => void
  queuedMessages: Ref<Map<string, string[]>>
  todoTasksMap: Ref<Record<string, Map<number, Todo>>>
  startTimer?: (task: Todo) => void
  stopTimer?: (taskId: number) => void
  clearAllTimers?: () => void
  turnUsage: Ref<number | null>
  loadConversations?: () => Promise<void>
}

export interface EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void
}
```

**注册表**：

```typescript
export class EventHandlerRegistry {
  private handlers = new Map<string, EventHandler>()
  
  constructor() {
    this.register('text_delta', new TextDeltaHandler())
    this.register('tool_start', new ToolStartHandler())
    this.register('tool_result', new ToolResultHandler())
    this.register('confirm_required', new ConfirmRequiredHandler())
    this.register('error', new ErrorHandler())
    this.register('done', new DoneHandler())
    this.register('todo_update', new TodoUpdateHandler())
    this.register('turn_usage', new TurnUsageHandler())
    this.register('message', new MessageHandler())
  }
  
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const handler = this.handlers.get(event.type)
    if (handler) {
      try {
        handler.handle(event, context)
      } catch (error) {
        console.error(`Handler error for ${event.type}:`, error)
      }
    } else {
      console.warn(`No handler for event type: ${event.type}`)
    }
  }
}
```

**扩展性**：新增事件类型只需实现 `EventHandler` 接口并注册，无需修改现有代码。

#### 2. State Pattern - 会话状态机

**问题**：会话状态隐式（通过 `streamingConvIds` Set 判断），状态转换逻辑分散在多处。

**解决方案**：显式状态类，每个状态定义允许的操作。

**状态接口**：

```typescript
export interface ConversationStateContext {
  convId: string
  messagesMap: Ref<Record<string, DisplayMessage[]>>
  queuedMessages: Ref<Map<string, string[]>>
  streamingConvIds: Ref<Set<string>>
  setConversationStreaming: (id: string, streaming: boolean) => void
  scrollToBottom: () => void
  updateAgentStatus?: (status: AgentStatusUpdate) => void
  getConvTitle: (id: string) => string
}

export interface ConversationState {
  readonly name: 'idle' | 'streaming' | 'waiting_confirm'
  
  send(text: string, context: ConversationStateContext): Promise<void>
  cancel(context: ConversationStateContext): Promise<void>
  confirm(requestId: string, approved: boolean, context: ConversationStateContext): Promise<void>
  
  transitionTo(newState: ConversationState, context: ConversationStateContext): void
}
```

**状态转换图**：

```
idle ──send()──> streaming ──done/error──> idle
                     │
                     └──confirm_required──> waiting_confirm ──confirm(true)──> streaming
                                                              └──confirm(false)──> idle
```

**实现示例**：

```typescript
export class IdleState extends BaseConversationState {
  readonly name = 'idle' as const
  
  async send(text: string, context: ConversationStateContext): Promise<void> {
    // 添加用户消息 + 流式占位
    // 发送 API 请求
    // 转换到 StreamingState
    this.transitionTo(new StreamingState(), context)
    context.setConversationStreaming(context.convId, true)
  }
  
  async cancel(context: ConversationStateContext): Promise<void> {
    throw new Error('Cannot cancel in idle state')
  }
}

export class StreamingState extends BaseConversationState {
  readonly name = 'streaming' as const
  
  async send(text: string, context: ConversationStateContext): Promise<void> {
    throw new Error('Cannot send while streaming. Cancel first.')
  }
  
  async cancel(context: ConversationStateContext): Promise<void> {
    await cancelConversation(context.convId)
    // 清理 + 重新加载消息
    this.transitionTo(new IdleState(), context)
    context.setConversationStreaming(context.convId, false)
  }
}
```

**收益**：
- 状态转换显式，易调试
- 非法操作在编译时/运行时明确拒绝
- 可导出状态图用于文档/可视化

#### 3. Observer Pattern - 事件总线

**问题**：当前用回调注入（`onScrollToBottom`, `onDeviceStatusUpdate` 等），组件间耦合。

**解决方案**：全局事件总线，发布-订阅解耦。

```typescript
export class ChatEventBus {
  private listeners = new Map<string, Set<EventCallback>>()
  
  on(event: string, callback: EventCallback): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set())
    }
    this.listeners.get(event)!.add(callback)
    return () => this.off(event, callback)
  }
  
  emit(event: string, data?: any): void {
    this.listeners.get(event)?.forEach(cb => cb(data))
  }
  
  clear(): void {
    this.listeners.clear()
  }
}

export const chatEventBus = new ChatEventBus()

export const ChatEvents = {
  SCROLL_TO_BOTTOM: 'scroll:bottom',
  DEVICE_STATUS_UPDATE: 'device:status',
  AGENT_STATUS_UPDATE: 'agent:status',
  TODO_UPDATE: 'todo:update',
  CONVERSATION_SELECTED: 'conversation:selected',
  MESSAGE_SENT: 'message:sent',
} as const
```

**使用**：

```typescript
// 发布
chatEventBus.emit(ChatEvents.SCROLL_TO_BOTTOM)

// 订阅
onMounted(() => {
  const unsub = chatEventBus.on(ChatEvents.TODO_UPDATE, ({ convId, task }) => {
    todoPanel.updateTodoFromEvent(convId, task)
  })
  onUnmounted(unsub)
})
```

### Composables 接口

#### useConversationList

```typescript
export function useConversationList(options?: {
  onConversationSelected?: (convId: string) => void
}) {
  return {
    // State (readonly)
    conversations: Readonly<Ref<Conversation[]>>
    activeConvId: Readonly<Ref<string | null>>
    batchMode: Readonly<Ref<boolean>>
    selectedConvIds: Readonly<Ref<Set<string>>>
    editingConvId: Readonly<Ref<string | null>>
    editTitleText: Ref<string>
    menuOpenConvId: Readonly<Ref<string | null>>
    
    // Operations
    loadConversations(): Promise<void>
    selectConversation(id: string): Promise<void>
    createConversation(title?: string): Promise<string>
    deleteConversation(id: string): Promise<void>
    updateTitle(id: string, title: string): Promise<void>
    
    // Batch
    enterBatchMode(): void
    exitBatchMode(): void
    toggleSelectConv(id: string): void
    toggleSelectAll(): void
    batchDelete(): Promise<void>
    
    // Title editing
    startEditConvTitle(id: string, title: string): void
    saveConvTitle(id: string): Promise<void>
    cancelEdit(): void
    
    // Menu
    openConvMenu(id: string): void
    closeConvMenu(): void
    
    // Utils
    getConvTitle(convId: string): string
  }
}
```

#### useChatStream

```typescript
export function useChatStream(options: {
  activeConvId: Ref<string | null>
  getConvTitle: (id: string) => string
  onScrollToBottom?: () => void
  onDeviceStatusUpdate?: (hostName: string, status: string) => void
  onAgentStatusUpdate?: (status: AgentStatusUpdate) => void
}) {
  return {
    // State
    messages: ComputedRef<DisplayMessage[]>
    isStreaming: ComputedRef<boolean>
    queuedMessages: Readonly<Ref<Map<string, string[]>>>
    retryState: Readonly<Ref<RetryState | null>>
    
    // Operations
    loadConversationMessages(convId: string): Promise<void>
    sendMessage(text: string): Promise<void>
    cancelSend(): Promise<void>
    handleConfirm(requestId: string, approved: boolean): Promise<void>
    
    // Lifecycle
    cleanup(): void
  }
}
```

#### useTodoPanel

```typescript
export function useTodoPanel(options: {
  activeConvId: Ref<string | null>
}) {
  return {
    // State
    allTasks: ComputedRef<Todo[]>
    inProgressTasks: ComputedRef<Todo[]>
    pendingTasks: ComputedRef<Todo[]>
    completedTasks: ComputedRef<Todo[]>
    visiblePending: ComputedRef<Todo[]>
    visibleCompleted: ComputedRef<Todo[]>
    hiddenPendingCount: ComputedRef<number>
    hasTasks: ComputedRef<boolean>
    panelHeader: ComputedRef<string>
    completedFolded: Ref<boolean>
    pendingFolded: Ref<boolean>
    taskElapsed: Readonly<Ref<Map<number, number>>>
    turnUsage: Readonly<Ref<number | null>>
    
    // Operations
    updateTodoFromEvent(convId: string, task: Todo): void
    loadTodoTasks(convId: string, tasks: Todo[]): void
    setTurnUsage(tokens: number | null): void
    clearAllTimers(): void
    
    // Utils
    fmtElapsed(seconds: number): string
    fmtTokens(n: number): string
  }
}
```

### ChatView 组合

```typescript
// views/ChatView.vue <script setup>
const conversationList = useConversationList({
  onConversationSelected: async (convId) => {
    await chatStream.loadConversationMessages(convId)
    router.replace(`/chat/${convId}`)
    await nextTick()
    scrollToBottom()
  }
})

const todoPanel = useTodoPanel({
  activeConvId: conversationList.activeConvId
})

const chatStream = useChatStream({
  activeConvId: conversationList.activeConvId,
  getConvTitle: conversationList.getConvTitle,
  onScrollToBottom: scrollToBottom,
  onDeviceStatusUpdate: (hostName, status) => {
    // 更新设备状态
  },
  onAgentStatusUpdate: (status) => {
    updateAgentStatus(status)
  },
})

onMounted(() => {
  chatEventBus.on(ChatEvents.TODO_UPDATE, ({ convId, task }) => {
    todoPanel.updateTodoFromEvent(convId, task)
  })
  
  conversationList.loadConversations()
})

onUnmounted(() => {
  chatStream.cleanup()
  todoPanel.clearAllTimers()
  chatEventBus.clear()
})
```

## 错误处理

### State Pattern 错误处理

```typescript
export abstract class BaseConversationState implements ConversationState {
  protected async safeTransition(
    action: () => Promise<void>,
    context: ConversationStateContext,
    errorState: ConversationState = new IdleState()
  ): Promise<void> {
    try {
      await action()
    } catch (error) {
      console.error(`[${context.convId}] State ${this.name} error:`, error)
      this.transitionTo(errorState, context)
      throw error
    }
  }
}
```

### Event Handler 错误隔离

```typescript
export class EventHandlerRegistry {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const handler = this.handlers.get(event.type)
    if (!handler) {
      console.warn(`No handler for event type: ${event.type}`)
      return
    }
    
    try {
      handler.handle(event, context)
    } catch (error) {
      console.error(`Handler error for ${event.type}:`, error)
      // 错误不传播，避免影响其他事件
      chatEventBus.emit('handler:error', { event, error })
    }
  }
}
```

### SSE 重连策略

```typescript
interface RetryConfig {
  maxRetries: number
  baseDelayMs: number
  maxDelayMs: number
}

const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxRetries: 5,
  baseDelayMs: 1000,
  maxDelayMs: 30000,
}

function calculateRetryDelay(attempt: number, config: RetryConfig): number {
  const delay = Math.min(
    config.baseDelayMs * Math.pow(2, attempt),
    config.maxDelayMs
  )
  // 添加 jitter 避免雷群
  return delay + Math.random() * 1000
}
```

## 测试策略

### 单元测试 - State Pattern

```typescript
// states/__tests__/ConversationState.test.ts
describe('ConversationState', () => {
  it('IdleState can send message', async () => {
    const state = new IdleState()
    const context = createMockContext()
    
    await state.send('hello', context)
    
    expect(context.messagesMap.value[context.convId]).toHaveLength(2)
    expect(context.setConversationStreaming).toHaveBeenCalledWith(context.convId, true)
  })
  
  it('StreamingState cannot send message', async () => {
    const state = new StreamingState()
    const context = createMockContext()
    
    await expect(state.send('hello', context)).rejects.toThrow('Cannot send while streaming')
  })
})
```

### 单元测试 - Event Handlers

```typescript
// handlers/__tests__/TextDeltaHandler.test.ts
describe('TextDeltaHandler', () => {
  it('appends text to existing text block', () => {
    const handler = new TextDeltaHandler()
    const context = createMockContext()
    context.messagesMap.value['conv-1'] = [{
      id: 'a-1',
      role: 'assistant',
      blocks: [{ type: 'text', content: 'Hello' }],
      isStreaming: true,
      toolIndex: new Map(),
    }]
    
    const event: ChatEvent = {
      type: 'text_delta',
      content: { text: ' world' },
    }
    
    handler.handle(event, context)
    
    const msg = context.messagesMap.value['conv-1'][0]
    expect(msg.blocks[0]).toEqual({ type: 'text', content: 'Hello world' })
  })
})
```

### 集成测试 - Playwright

```typescript
// e2e/chat-stream.spec.ts
test('send message transitions to streaming state', async ({ page }) => {
  await page.goto('http://localhost:8002/chat')
  await page.fill('textarea', 'hello')
  await page.press('textarea', 'Enter')
  
  await expect(page.locator('.streaming-indicator')).toBeVisible()
  await expect(page.locator('textarea')).toBeDisabled()
  
  await expect(page.locator('.streaming-indicator')).toBeHidden({ timeout: 30000 })
  await expect(page.locator('textarea')).toBeEnabled()
})

test('cancel during streaming returns to idle', async ({ page }) => {
  await page.goto('http://localhost:8002/chat')
  await page.fill('textarea', 'long running task')
  await page.press('textarea', 'Enter')
  
  await expect(page.locator('.streaming-indicator')).toBeVisible()
  await page.keyboard.press('Escape')
  
  await expect(page.locator('.streaming-indicator')).toBeHidden()
  await expect(page.locator('textarea')).toBeEnabled()
})
```

## 实施阶段

### Phase 1: 基础设施（1 天）

1. 创建目录结构
2. 实现 EventHandler 接口 + 9 个具体 handler
3. 实现 ConversationState 接口 + 3 个状态类
4. 实现 ChatEventBus
5. 验证：`npm run type-check && npm run build`

### Phase 2: useConversationList（1 天）

1. 提取 composable
2. 更新 ChatView 导入
3. 验证：Playwright 测试会话列表功能

### Phase 3: useTodoPanel（1 天）

1. 提取 composable
2. 更新 ChatView 导入
3. 验证：Playwright 测试 todo 面板

### Phase 4: useChatStream（2 天）

1. 提取 composable
2. 集成 State Pattern + Event Handlers
3. 更新 ChatView 组合三个 composables
4. 验证：Playwright 全量测试

### Phase 5: 全量测试（1 天）

1. 类型检查：`vue-tsc --noEmit`
2. 构建：`npm run build && go build -a`
3. Playwright 全量测试
4. 手动回归测试
5. 性能验证（Chrome DevTools）

## 回滚策略

每个 Phase 独立 commit：

```bash
git add web/src/composables/chat/{states,handlers,events}
git commit -m "feat(chat): add event handlers and state pattern infrastructure"

git add web/src/composables/chat/useConversationList.ts web/src/views/ChatView.vue
git commit -m "refactor(chat): extract useConversationList composable"

# ... 其他 Phase
```

失败回滚：`git reset --mixed HEAD~1` 或 `git revert HEAD`

## 预期收益

### 代码行数

- ChatView: 1803 → ~500 行（-72%）
- 新增 composables: ~800 行
- 新增 handlers: ~600 行
- 新增 states: ~300 行
- 净增: ~300 行（+17%），但职责清晰

### 可维护性

- 新增事件类型：只需加 handler，无需改 switch-case
- 新增状态：只需加 state 类，无需改现有状态
- 单元测试覆盖率：0% → 80%+

### 扩展性

- 支持插件式事件处理器
- 支持状态机可视化（导出状态图）
- 支持事件重放（调试）

## 非目标

- 不引入 Pinia（当前 composable 单例模式足够）
- 不添加 Vitest 单元测试（先用 Playwright 集成测试）
- 不重构无关代码（surgical changes only）
- 不改变 UI 设计或行为
- 不优化性能（除非检测到回归）

## 成功标准

- [ ] ChatView < 500 行
- [ ] 所有 SSE 事件类型有独立 handler
- [ ] 会话状态转换显式（State Pattern）
- [ ] `vue-tsc --noEmit` 通过
- [ ] `npm run build` 成功
- [ ] Playwright 测试全部通过
- [ ] 手动回归测试无问题
- [ ] 无性能回归（Chrome DevTools 验证）
