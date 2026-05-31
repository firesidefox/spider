# ChatView Design Patterns Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor ChatView from 1803 lines to ~500 lines by extracting composables and applying Strategy, State, and Observer patterns.

**Architecture:** Extract 3 composables (useConversationList, useChatStream, useTodoPanel). Apply Strategy Pattern for SSE event handling (9 handlers), State Pattern for conversation state machine (3 states), Observer Pattern for event bus.

**Tech Stack:** Vue 3 Composition API, TypeScript, EventSource (SSE)

**Spec:** `docs/superpowers/specs/2026-06-01-chatview-design-patterns.md`

---

## Implementation Strategy

分 5 个 Phase 实施，每个 Phase 独立验证和提交。Phase 内部按 TDD 原则：先写测试（如适用），再实现，频繁提交。

---

## Phase 1: 基础设施（1 天）

**目标**：创建设计模式基础设施，不修改 ChatView。

**Files:**
- Create: `web/src/composables/chat/events/ChatEventBus.ts`
- Create: `web/src/composables/chat/states/*.ts` (6 files)
- Create: `web/src/composables/chat/handlers/*.ts` (12 files)

### Task 1.1: Event Bus (Observer Pattern)

- [ ] **创建目录**

```bash
mkdir -p web/src/composables/chat/events
```

- [ ] **实现 ChatEventBus**

参考 spec 第 204-256 行，实现：
- `ChatEventBus` 类（on/off/emit/clear 方法）
- `chatEventBus` 单例
- `ChatEvents` 常量

文件：`web/src/composables/chat/events/ChatEventBus.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/events/
git commit -m "feat(chat): add event bus (Observer Pattern)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 1.2: State Pattern 接口

- [ ] **创建目录**

```bash
mkdir -p web/src/composables/chat/states
```

- [ ] **实现接口和基类**

参考 spec 第 130-153 行，实现：
- `ConversationState` 接口
- `ConversationStateContext` 接口
- `BaseConversationState` 抽象类（含 `safeTransition` 方法）

文件：
- `web/src/composables/chat/states/ConversationState.ts`
- `web/src/composables/chat/states/BaseConversationState.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/states/ConversationState.ts web/src/composables/chat/states/BaseConversationState.ts
git commit -m "feat(chat): add State Pattern interfaces

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 1.3: State Pattern 实现

- [ ] **实现 IdleState**

参考 spec 第 167-181 行。从 ChatView 复制 `sendMessage` 逻辑。

文件：`web/src/composables/chat/states/IdleState.ts`

- [ ] **实现 StreamingState**

参考 spec 第 183-196 行。从 ChatView 复制 `cancelSend` 逻辑。

文件：`web/src/composables/chat/states/StreamingState.ts`

- [ ] **实现 WaitingConfirmState**

从 ChatView 复制 `handleConfirm` 逻辑。

文件：`web/src/composables/chat/states/WaitingConfirmState.ts`

- [ ] **创建 barrel export**

```typescript
export * from './ConversationState'
export * from './BaseConversationState'
export * from './IdleState'
export * from './StreamingState'
export * from './WaitingConfirmState'
```

文件：`web/src/composables/chat/states/index.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/states/
git commit -m "feat(chat): implement State Pattern (3 states)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 1.4: Strategy Pattern 接口

- [ ] **创建目录**

```bash
mkdir -p web/src/composables/chat/handlers
```

- [ ] **实现接口**

参考 spec 第 64-86 行，实现：
- `EventHandlerContext` 接口
- `EventHandler` 接口

文件：`web/src/composables/chat/handlers/EventHandler.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/handlers/EventHandler.ts
git commit -m "feat(chat): add EventHandler interface (Strategy Pattern)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 1.5: Strategy Pattern - 9 个 Handler

从 ChatView.vue 的 `handleConvEvent` 函数（第 627-800 行）提取逻辑到独立 handler。

- [ ] **实现 TextDeltaHandler**

从 switch case 'text_delta' 提取（ChatView.vue:674-683）

文件：`web/src/composables/chat/handlers/TextDeltaHandler.ts`

- [ ] **实现 ToolStartHandler**

从 switch case 'tool_start' 提取（ChatView.vue:685-700）

文件：`web/src/composables/chat/handlers/ToolStartHandler.ts`

- [ ] **实现 ToolResultHandler**

从 switch case 'tool_result' 提取（ChatView.vue:702-733）

文件：`web/src/composables/chat/handlers/ToolResultHandler.ts`

- [ ] **实现 ConfirmRequiredHandler**

从 switch case 'confirm_required' 提取（ChatView.vue:735-742）

文件：`web/src/composables/chat/handlers/ConfirmRequiredHandler.ts`

- [ ] **实现 ErrorHandler**

从 switch case 'error' 提取（ChatView.vue:744-763）

文件：`web/src/composables/chat/handlers/ErrorHandler.ts`

- [ ] **实现 DoneHandler**

从 switch case 'done' 提取（ChatView.vue:765-781）

文件：`web/src/composables/chat/handlers/DoneHandler.ts`

- [ ] **实现 TodoUpdateHandler**

从 switch case 'todo_update' 提取（ChatView.vue:782-792）

文件：`web/src/composables/chat/handlers/TodoUpdateHandler.ts`

- [ ] **实现 TurnUsageHandler**

从 switch case 'turn_usage' 提取（ChatView.vue:794-799）

文件：`web/src/composables/chat/handlers/TurnUsageHandler.ts`

- [ ] **实现 MessageHandler**

从 switch case 'message' 提取（ChatView.vue:642-649）

文件：`web/src/composables/chat/handlers/MessageHandler.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/handlers/*Handler.ts
git commit -m "feat(chat): implement 9 event handlers (Strategy Pattern)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 1.6: EventHandlerRegistry

- [ ] **实现注册表**

参考 spec 第 92-119 行，实现：
- `EventHandlerRegistry` 类
- `register` 方法
- `handle` 方法（含错误隔离）
- 构造函数中注册 9 个 handler

文件：`web/src/composables/chat/handlers/EventHandlerRegistry.ts`

- [ ] **创建 barrel export**

```typescript
export * from './EventHandler'
export * from './EventHandlerRegistry'
export * from './TextDeltaHandler'
// ... 其他 8 个
```

文件：`web/src/composables/chat/handlers/index.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/handlers/
git commit -m "feat(chat): add EventHandlerRegistry

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 1.7: Phase 1 验证

- [ ] **类型检查**

```bash
cd web && npx vue-tsc --noEmit
```

Expected: No errors

- [ ] **构建**

```bash
cd web && npm run build
```

Expected: Success

- [ ] **标记 Phase 1 完成**

```bash
git tag phase1-infrastructure
```

---

## Phase 2: useConversationList（1 天）

**目标**：提取会话列表管理逻辑到 composable。

**Files:**
- Create: `web/src/composables/chat/useConversationList.ts`
- Modify: `web/src/views/ChatView.vue`

### Task 2.1: 提取 useConversationList

- [ ] **实现 composable**

参考 spec 第 262-302 行。从 ChatView.vue 提取：
- 会话列表状态（conversations, activeConvId, batchMode 等）
- 会话操作（loadConversations, selectConversation, createConversation 等）
- 批量操作（enterBatchMode, batchDelete 等）
- 标题编辑（startEditConvTitle, saveConvTitle 等）
- 菜单（openConvMenu, closeConvMenu）

源代码位置：
- 状态：ChatView.vue:65-69, 423-428
- 操作：ChatView.vue:532-598
- 批量：ChatView.vue:471-503
- 标题：ChatView.vue:430-461
- 菜单：ChatView.vue:463-469

文件：`web/src/composables/chat/useConversationList.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/useConversationList.ts
git commit -m "feat(chat): extract useConversationList composable

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 2.2: 更新 ChatView 使用 useConversationList

- [ ] **导入 composable**

在 ChatView.vue `<script setup>` 顶部添加：

```typescript
import { useConversationList } from '../composables/chat/useConversationList'
```

- [ ] **替换状态和方法**

删除原有会话列表相关代码，替换为：

```typescript
const conversationList = useConversationList({
  onConversationSelected: async (convId) => {
    // 加载消息逻辑（暂时保留在 ChatView）
    await selectConversation(convId)
  }
})
```

- [ ] **更新模板引用**

将模板中的 `conversations` 替换为 `conversationList.conversations`，其他类似。

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **构建**

```bash
cd web && npm run build
```

- [ ] **提交**

```bash
git add web/src/views/ChatView.vue
git commit -m "refactor(chat): use useConversationList in ChatView

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 2.3: Phase 2 验证

- [ ] **启动测试服务器**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **手动测试**

1. 访问 http://localhost:8002/chat
2. 创建新会话
3. 切换会话
4. 编辑会话标题
5. 批量删除会话

Expected: 所有功能正常

- [ ] **标记 Phase 2 完成**

```bash
git tag phase2-conversation-list
```

---

## Phase 3: useTodoPanel（1 天）

**目标**：提取 todo 面板逻辑到 composable。

**Files:**
- Create: `web/src/composables/chat/useTodoPanel.ts`
- Modify: `web/src/views/ChatView.vue`

### Task 3.1: 提取 useTodoPanel

- [ ] **实现 composable**

参考 spec 第 336-366 行。从 ChatView.vue 提取：
- Todo 状态（todoTasksMap, taskTimers, taskElapsed 等）
- 计算属性（allTasks, inProgressTasks, panelHeader 等）
- 操作（updateTodoFromEvent, loadTodoTasks, setTurnUsage）
- 计时器（startTimer, stopTimer, clearAllTimers）
- 格式化（fmtElapsed, fmtTokens）

源代码位置：
- 状态：ChatView.vue:196-199, 230-231
- 计算属性：ChatView.vue:233-270
- 操作：ChatView.vue:782-792
- 计时器：ChatView.vue:286-308
- 格式化：ChatView.vue:272-284

文件：`web/src/composables/chat/useTodoPanel.ts`

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/useTodoPanel.ts
git commit -m "feat(chat): extract useTodoPanel composable

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 3.2: 更新 ChatView 使用 useTodoPanel

- [ ] **导入 composable**

```typescript
import { useTodoPanel } from '../composables/chat/useTodoPanel'
```

- [ ] **替换状态和方法**

```typescript
const todoPanel = useTodoPanel({
  activeConvId: conversationList.activeConvId
})
```

- [ ] **更新模板引用**

将模板中的 `allTasks` 替换为 `todoPanel.allTasks`，其他类似。

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **构建**

```bash
cd web && npm run build
```

- [ ] **提交**

```bash
git add web/src/views/ChatView.vue
git commit -m "refactor(chat): use useTodoPanel in ChatView

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 3.3: Phase 3 验证

- [ ] **手动测试**

1. 发送消息触发 todo 任务
2. 验证任务计时器
3. 验证任务状态更新
4. 验证 token 使用量显示

Expected: Todo 面板功能正常

- [ ] **标记 Phase 3 完成**

```bash
git tag phase3-todo-panel
```

---

## Phase 4: useChatStream（2 天）

**目标**：提取 SSE 流和消息管理逻辑，集成 State Pattern 和 Strategy Pattern。

**Files:**
- Create: `web/src/composables/chat/useChatStream.ts`
- Modify: `web/src/views/ChatView.vue`

### Task 4.1: 提取 useChatStream

- [ ] **实现 composable**

参考 spec 第 307-331 行。从 ChatView.vue 提取：
- 消息状态（messagesMap, queuedMessages, streamingConvIds 等）
- SSE 订阅管理（convSubscriptions）
- 状态管理（conversationStates Map）
- 事件处理器注册表
- 操作（loadConversationMessages, sendMessage, cancelSend, handleConfirm）
- 事件处理（handleConvEvent，使用 EventHandlerRegistry）

源代码位置：
- 状态：ChatView.vue:80-82, 136-145, 201-209
- SSE：ChatView.vue:350-351, 572-576
- 操作：ChatView.vue:611-625, 627-800
- 消息构建：ChatView.vue:98-121

文件：`web/src/composables/chat/useChatStream.ts`

**关键集成点**：
1. 使用 `EventHandlerRegistry` 替换 switch-case
2. 使用 `ConversationState` 管理状态转换
3. 在 `handleConvEvent` 中触发状态转换

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **提交**

```bash
git add web/src/composables/chat/useChatStream.ts
git commit -m "feat(chat): extract useChatStream with design patterns

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 4.2: 更新 ChatView 使用 useChatStream

- [ ] **导入 composable**

```typescript
import { useChatStream } from '../composables/chat/useChatStream'
```

- [ ] **替换状态和方法**

参考 spec 第 372-410 行：

```typescript
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
```

- [ ] **连接事件总线**

```typescript
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

- [ ] **更新模板引用**

将模板中的 `messages` 替换为 `chatStream.messages`，其他类似。

- [ ] **验证编译**

```bash
cd web && npx vue-tsc --noEmit
```

- [ ] **构建**

```bash
cd web && npm run build
```

- [ ] **提交**

```bash
git add web/src/views/ChatView.vue
git commit -m "refactor(chat): use useChatStream in ChatView

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 4.3: Phase 4 验证

- [ ] **手动测试核心流程**

1. 发送消息 → 验证流式响应
2. 按 ESC 取消 → 验证状态转换
3. 触发确认操作 → 验证 WaitingConfirmState
4. 切换会话 → 验证多会话隔离
5. 打开多个 tab → 验证 SSE 广播

Expected: 所有功能正常，状态转换日志正确

- [ ] **检查 ChatView 行数**

```bash
wc -l web/src/views/ChatView.vue
```

Expected: ~500 行（从 1803 降低）

- [ ] **标记 Phase 4 完成**

```bash
git tag phase4-chat-stream
```

---

## Phase 5: 全量测试（1 天）

**目标**：全量验证，性能测试，文档更新。

### Task 5.1: 类型检查和构建

- [ ] **类型检查**

```bash
cd web && npx vue-tsc --noEmit
```

Expected: No errors

- [ ] **构建**

```bash
cd web && npm run build
```

Expected: Success

- [ ] **Go 构建**

```bash
go build -a -o /tmp/spider-final ./cmd/spider
```

Expected: Success

### Task 5.2: 手动回归测试

- [ ] **启动服务器**

```bash
/tmp/spider-final serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **测试清单**

1. [ ] 登录
2. [ ] 创建会话
3. [ ] 发送消息
4. [ ] SSE 流式响应
5. [ ] 取消发送（ESC）
6. [ ] 确认操作
7. [ ] 切换会话
8. [ ] 多 tab 同步
9. [ ] Todo 面板更新
10. [ ] 批量删除会话
11. [ ] 编辑会话标题
12. [ ] 设备状态更新
13. [ ] Agent 状态显示

Expected: 所有功能正常，无回归

### Task 5.3: 性能验证

- [ ] **Chrome DevTools Performance**

1. 打开 Chrome DevTools → Performance
2. 录制发送消息流程
3. 检查事件处理耗时

Expected: 每个事件处理 < 10ms

- [ ] **内存泄漏检查**

1. Chrome DevTools → Memory
2. 创建 10 个会话
3. 切换会话 20 次
4. Take heap snapshot
5. 检查 detached DOM nodes

Expected: 无明显内存泄漏

### Task 5.4: 更新文档

- [ ] **更新 CLAUDE.md**

添加设计模式说明：

```markdown
## 前端架构

### ChatView 设计模式

- **Strategy Pattern**: SSE 事件处理（9 个 handler）
- **State Pattern**: 会话状态机（idle/streaming/waiting_confirm）
- **Observer Pattern**: 事件总线（chatEventBus）

新增事件类型：实现 `EventHandler` 接口并在 `EventHandlerRegistry` 注册。
新增会话状态：继承 `BaseConversationState` 并实现状态转换逻辑。
```

- [ ] **提交**

```bash
git add CLAUDE.md
git commit -m "docs: update ChatView architecture documentation

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

### Task 5.5: 最终验证

- [ ] **确认目标达成**

1. [ ] ChatView < 500 行
2. [ ] 所有 SSE 事件类型有独立 handler
3. [ ] 会话状态转换显式（State Pattern）
4. [ ] `vue-tsc --noEmit` 通过
5. [ ] `npm run build` 成功
6. [ ] 手动回归测试通过
7. [ ] 无性能回归

- [ ] **标记完成**

```bash
git tag phase5-complete
git push origin main --tags
```

---

## 回滚策略

每个 Phase 有独立 tag，失败时回滚到上一个 Phase：

```bash
# 回滚到 Phase 3
git reset --mixed phase3-todo-panel

# 或创建反向 commit
git revert HEAD
```

---

## 预期收益

- **代码行数**: ChatView 1803 → ~500 行（-72%）
- **可维护性**: 新增事件/状态只需加类，无需改现有代码
- **可测试性**: 单元测试覆盖率 0% → 80%+（后续可加）
- **扩展性**: 支持插件式 handler、状态机可视化、事件重放

