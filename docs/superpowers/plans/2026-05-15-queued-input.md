# Queued Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agent 运行时输入框保持可用，提交的消息进入队列，run 完成或取消后合并发出。

**Architecture:** 纯前端改动。`ChatView.vue` 新增 `queuedMessages: string[]` ref。`send()` 在 streaming 时入队而非发送。Run 完成/取消时合并队列消息触发新 run。排队消息以 dim 样式渲染在消息列表末尾。

**Tech Stack:** Vue 3 Composition API，无新依赖。

---

## File Structure

- Modify: `web/src/views/ChatView.vue` — 唯一改动文件

---

### Task 1: 新增队列 ref，修改 send() 入队逻辑

**Files:**
- Modify: `web/src/views/ChatView.vue:559-618`

- [ ] **Step 1: 在 `isStreaming` ref 附近新增队列 ref**

在 `ChatView.vue` 中找到 `isStreaming` 的声明（约第 110 行附近），在其后添加：

```ts
const queuedMessages = ref<string[]>([])
```

- [ ] **Step 2: 修改 `send()` 函数，streaming 时入队**

将现有 `send()` 函数（第 559-618 行）中的逻辑修改：

```ts
async function send(overrideText?: string) {
  const text = (overrideText ?? inputText.value).trim()
  if (!text) return

  // slash commands 只在非 streaming 时处理
  if (!overrideText) {
    if (text === '/model') {
      inputText.value = ''
      await handleModelCommand()
      return
    }
    if (text === '/export' || text.startsWith('/export ')) {
      const fmt = parseExportFormat(text)
      if (fmt === 'invalid') {
        addSystemMessage('用法：/export [md|json] 或 /export --format [md|json]')
        return
      }
      inputText.value = ''
      if (!activeConvId.value) {
        addSystemMessage('没有活跃的会话')
        return
      }
      await exportConversation(activeConvId.value, fmt === 'default' ? 'md' : fmt)
      return
    }
  }

  // streaming 时入队
  if (isStreaming.value && !overrideText) {
    queuedMessages.value.push(text)
    inputText.value = ''
    nextTick(() => {
      if (textareaRef.value) textareaRef.value.style.height = 'auto'
    })
    return
  }

  if (!overrideText) {
    inputText.value = ''
    nextTick(() => {
      if (textareaRef.value) textareaRef.value.style.height = 'auto'
    })
  }

  if (!activeConvId.value) {
    await createNewConversation()
  }

  const convId = activeConvId.value!
  const convMsgs = getOrInitMessages(convId)

  if (!convSubscriptions.has(convId)) {
    const unsub = subscribeConversation(convId, (event) => handleConvEvent(convId, event), -1)
    convSubscriptions.set(convId, unsub)
  }

  const userMsg: DisplayMessage = {
    id: `u-${Date.now()}`, role: 'user', blocks: [{ type: 'text', content: text }],
  }
  convMsgs.push(userMsg)

  const assistantMsg: DisplayMessage = {
    id: `a-${Date.now()}`, role: 'assistant',
    blocks: [], isStreaming: true,
  }
  convMsgs.push(assistantMsg)
  isStreaming.value = true
  turnUsage.value = null
  await nextTick()
  scrollToBottom()

  abortCtrl = sendMessage(convId, text)
}
```

- [ ] **Step 3: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无 TypeScript 错误，build 成功。

- [ ] **Step 4: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(frontend): add queuedMessages ref, send() enqueues when streaming"
```

---

### Task 2: Run 完成/取消时消费队列

**Files:**
- Modify: `web/src/views/ChatView.vue` — `handleConvEvent()` 和 `cancelSend()`

- [ ] **Step 1: 新增 flushQueue() 辅助函数**

在 `send()` 函数之后添加：

```ts
function flushQueue() {
  if (queuedMessages.value.length === 0) return
  const merged = queuedMessages.value.join('\n\n')
  queuedMessages.value = []
  send(merged)
}
```

- [ ] **Step 2: 在 run 完成时调用 flushQueue()**

在 `handleConvEvent()` 中找到设置 `isStreaming.value = false` 的位置（EventDone 和 EventError 分支），在每处 `isStreaming.value = false` 之后添加 `flushQueue()`。

找到 `EventDone` 处理（约第 500-520 行附近）：

```ts
case 'done':
  isStreaming.value = false
  flushQueue()   // ← 添加这行
  // ... 其余逻辑不变
  break
```

找到 `EventError` 处理：

```ts
case 'error':
  isStreaming.value = false
  flushQueue()   // ← 添加这行
  // ... 其余逻辑不变
  break
```

- [ ] **Step 3: 修改 cancelSend() 在取消后消费队列**

找到 `cancelSend()` 函数，在取消操作后添加 `flushQueue()`：

```ts
async function cancelSend() {
  if (abortCtrl) {
    abortCtrl.abort()
    abortCtrl = null
  }
  if (activeConvId.value) {
    await cancelConversation(activeConvId.value)
  }
  isStreaming.value = false
  flushQueue()   // ← 添加这行
}
```

- [ ] **Step 4: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(frontend): flush queued messages on run complete or cancel"
```

---

### Task 3: Dim 消息渲染

**Files:**
- Modify: `web/src/views/ChatView.vue` — template 和 style

- [ ] **Step 1: 在消息列表末尾渲染 dim 消息**

找到消息列表的 `v-for` 循环（渲染 `messages` 的部分），在其后、`chat-input` div 之前插入：

```html
<!-- Queued messages — dim style, shown while streaming -->
<div
  v-for="(qm, i) in queuedMessages"
  :key="`queued-${i}`"
  class="queued-message"
>
  <span class="queued-message-text">{{ qm }}</span>
</div>
```

- [ ] **Step 2: 添加 dim 样式**

在 `<style scoped>` 中添加：

```css
.queued-message { padding: 8px 16px; opacity: 0.45; }
.queued-message-text { font-family: 'SF Mono', monospace; font-size: 13px; color: var(--text); white-space: pre-wrap; word-break: break-word; }
```

- [ ] **Step 3: 修改输入框 — 移除 streaming 时的 disabled，更新按钮**

找到 textarea（约第 1039-1048 行）：

```html
<textarea
  ref="textareaRef"
  v-model="inputText"
  @keydown.enter.exact.prevent="send()"
  @keydown="onTextareaKeydown"
  @input="onTextareaInput"
  :placeholder="isStreaming ? '排队发送...' : '输入运维指令...'"
  rows="1"
></textarea>
```

（移除 `:disabled="isStreaming"`，改 placeholder）

找到发送/取消按钮区域（约第 1050-1051 行），改为：

```html
<button v-if="isStreaming" @click="cancelSend" class="send-btn cancel-btn">取消</button>
<button v-if="isStreaming" @click="send()" :disabled="!inputText.trim()" class="send-btn queue-btn">排队</button>
<button v-if="!isStreaming" @click="send()" :disabled="!inputText.trim()" class="send-btn">发送</button>
```

添加样式：

```css
.queue-btn { background: var(--text-sub); }
.queue-btn:hover:not(:disabled) { background: var(--text); }
```

- [ ] **Step 4: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(frontend): render queued messages dim, unlock textarea during streaming"
```

---

### Task 4: 端到端验证

- [ ] **Step 1: 启动测试服务器**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: 用 Playwright 验证黄金路径**

```bash
cd /Users/cw/fty.ai/spider.ai
npx playwright test --headed 2>&1 | tail -30
```

手动验证步骤：
1. 打开 http://localhost:8002，进入一个对话
2. 发送一条消息，触发 streaming
3. streaming 期间在输入框输入第二条消息，点"排队"
4. 确认：dim 消息出现在列表末尾
5. streaming 完成后：dim 消息消失，第二条消息作为新 user message 发出，新 run 开始
6. 再次 streaming 期间，排队两条消息，确认两条都显示为 dim
7. streaming 完成后，两条消息合并为一条（`\n\n` 分隔）发出
8. 测试取消路径：streaming 期间排队一条消息，点"取消"，确认排队消息立即发出

- [ ] **Step 3: 最终 commit（如有遗留修复）**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "fix(frontend): queued input edge case fixes"
```
