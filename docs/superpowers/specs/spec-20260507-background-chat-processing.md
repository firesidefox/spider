# Background Chat Processing — Design Spec

**Date:** 2026-05-07  
**Status:** 已实现 — scheduler/scheduler.go + executor.go，定时任务后台执行，对话在无用户在线时继续处理

## Problem

当前 LLM 处理生命周期绑定 HTTP 连接：`a.Run(r.Context(), ...)` 导致 SSE 连接断开时处理中止。三种场景均受影响：

1. 切到本系统其他页面再回来 → ChatView 卸载，流式中断
2. 切到其他会话再回来 → 原会话流式状态丢失
3. 关闭浏览器重开 → 处理中止，消息丢失

## Solution Overview

**后端**：用 `context.Background()` 替换 `r.Context()`，处理不再绑定连接。加 `status` 字段追踪处理状态。

**前端**：`<KeepAlive>` 保活 ChatView（场景 1）；重连时检查 status，轮询等待完成（场景 2、3）；localStorage 恢复上次会话（场景 3）。

---

## Backend Changes

### 1. DB Migration

`conversations` 表新增字段：

```sql
ALTER TABLE conversations ADD COLUMN status TEXT NOT NULL DEFAULT 'idle';
```

取值：`idle`（空闲）| `processing`（处理中）

### 2. Model & Store

`models.Conversation` 加 `Status string` 字段。

`ConversationStore` 加：

```go
func (s *ConversationStore) SetStatus(id, status string) error
```

### 3. chat handler — decouple context

`internal/api/chat.go` `chatSendMessage`：

```go
// 改前
events, err := a.Run(r.Context(), id, content, waiter)

// 改后
app.ConvStore.SetStatus(id, "processing")
bgCtx := context.Background()
events, err := a.Run(bgCtx, id, content, waiter)
```

SSE 转发循环结束后（`done` 事件或 channel 关闭）：

```go
app.ConvStore.SetStatus(id, "idle")
```

`defer app.RemoveChatWaiter(id)` 保持不变。

### 4. API response

`GET /api/v1/chat/conversations/:id` 返回的 `conversation` 对象包含 `status` 字段。

`GET /api/v1/chat/conversations`（列表）同样包含 `status`。

---

## Frontend Changes

### 1. App.vue — KeepAlive

```html
<!-- 改前 -->
<RouterView />

<!-- 改后 -->
<KeepAlive include="ChatView">
  <RouterView />
</KeepAlive>
```

### 2. ChatView.vue — 组件名 + 生命周期

```js
defineOptions({ name: 'ChatView' })
```

`onMounted` → `onActivated`，`onUnmounted` → `onDeactivated`。

`onActivated` 逻辑：

```
loadConversations()   // 刷新列表（含 status）
loadDevices()
if paramId && paramId !== activeConvId:
  selectConversation(paramId)
else if !paramId && localStorage has lastConvId:
  router.replace(`/chat/${lastConvId}`)
  selectConversation(lastConvId)
```

### 3. ChatView.vue — messages map

`messages` 从 `ref<DisplayMessage[]>` 改为 `ref<Map<string, DisplayMessage[]>>`。

```js
const messagesMap = ref(new Map<string, DisplayMessage[]>())
const messages = computed(() => messagesMap.value.get(activeConvId.value ?? '') ?? [])
```

流式回调在 `send()` 启动时捕获当前 conv 的数组引用，切换会话不影响进行中的流：

```js
const convId = activeConvId.value!
const convMsgs = getOrInitMessages(convId)  // 从 map 取或初始化
abortCtrl = sendMessage(convId, text, (event) => {
  // 操作 convMsgs，不读 messages.value
})
```

### 4. ChatView.vue — selectConversation 加 status 检查

```js
async function selectConversation(id: string) {
  const data = await getConversation(id)   // 含 status
  activeConvId.value = id
  localStorage.setItem('spider-last-conv', id)

  if (data.conversation.status === 'processing') {
    // 显示已有消息 + spinner，轮询直到 idle
    setMessagesFromDB(id, data.messages)
    pollUntilIdle(id)
  } else {
    setMessagesFromDB(id, data.messages)
  }
}
```

`pollUntilIdle`：每 2s 调用 `getConversation(id)`，直到 `status === 'idle'`，然后重载消息。

### 5. ChatView.vue — localStorage

- `selectConversation` 时：`localStorage.setItem('spider-last-conv', id)`
- `onActivated` 无 paramId 时：读取并跳转

---

## Data Flow

```
用户发消息
  → POST /messages
  → SetStatus("processing")
  → a.Run(context.Background(), ...)   ← 不绑定连接
  → SSE 转发给前端（如果连接存在）
  → done → SetStatus("idle") → 保存到 DB

用户切走（场景 1）
  → keep-alive，ChatView 存活，SSE 继续

用户切到其他会话（场景 2）
  → selectConversation(conv-B)
  → conv-A 的 goroutine 继续跑，写 DB
  → 回到 conv-A → selectConversation(conv-A)
  → status=processing → spinner + poll
  → idle → 从 DB 加载完整消息

用户关闭浏览器（场景 3）
  → SSE 连接断开，goroutine 继续（bgCtx）
  → 完成后 SetStatus("idle")，消息写 DB
  → 重开 → onActivated → localStorage → selectConversation
  → status=idle → 加载完整消息
```

---

## Out of Scope

- 流式重连（reconnect SSE 实时接收增量）：场景 2/3 重连后只展示 DB 最终结果，不重播流式事件
- confirm_required 跨连接恢复：关闭浏览器后待确认操作视为超时/取消（现有行为不变）
- 多标签页同步

---

## Files Changed

| 文件 | 改动 |
|------|------|
| `internal/db/schema.go` | 加 ALTER TABLE migration |
| `internal/models/conversation.go` | 加 Status 字段 |
| `internal/store/conversation.go` | 加 SetStatus()，GetByID/List 读 status |
| `internal/api/chat.go` | bgCtx，SetStatus 调用 |
| `web/src/App.vue` | KeepAlive 包裹 RouterView |
| `web/src/views/ChatView.vue` | defineOptions，onActivated/Deactivated，messages map，pollUntilIdle，localStorage |
| `web/src/api/chat.ts` | Conversation 类型加 status 字段 |
