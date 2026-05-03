# Chat Layout Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor Chat page from dropdown conversation list to collapsible left sidebar, add drag-resize between chat and target panel.

**Architecture:** Single-file change to `ChatView.vue` — restructure template into sidebar + chat-main + drag-handle + target-side. Sidebar state persisted to localStorage. Drag resize via mousedown/mousemove handlers updating flex-basis.

**Tech Stack:** Vue 3 Composition API, custom CSS with CSS variables, zero dependencies.

---

## File Structure

- Modify: `web/src/views/ChatView.vue` — template restructure, new CSS, drag logic, sidebar state

No new files needed. All changes contained in ChatView.vue.

---

### Task 1: Add sidebar state and drag refs

**Files:**
- Modify: `web/src/views/ChatView.vue:27-35` (refs section)

- [ ] **Step 1: Add new refs for sidebar and drag**

In the `<script setup>` section, after line 35 (`let abortCtrl`), add:

```typescript
const sidebarOpen = ref(localStorage.getItem('spider-sidebar') !== 'closed')
const targetWidth = ref(parseInt(localStorage.getItem('spider-target-width') || '280'))
const isDragging = ref(false)
const chatPageRef = ref<HTMLElement | null>(null)
```

Replace `showConvList` ref (line 33) — it becomes unused. Remove:
```typescript
const showConvList = ref(false)
```

- [ ] **Step 2: Add sidebar toggle function**

After the `cancelEdit` function (line 82), add:

```typescript
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
  localStorage.setItem('spider-sidebar', sidebarOpen.value ? 'open' : 'closed')
}
```

- [ ] **Step 3: Add drag handlers**

After `toggleSidebar`, add:

```typescript
function startDrag(e: MouseEvent) {
  isDragging.value = true
  const startX = e.clientX
  const startWidth = targetWidth.value

  function onMove(ev: MouseEvent) {
    const delta = startX - ev.clientX
    const newWidth = Math.min(
      window.innerWidth * 0.5,
      Math.max(200, startWidth + delta)
    )
    targetWidth.value = newWidth
  }

  function onUp() {
    isDragging.value = false
    localStorage.setItem('spider-target-width', String(targetWidth.value))
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}
```

- [ ] **Step 4: Verify no TypeScript errors**

Run: `cd /Users/cw/fty.ai/spider.ai/web && npx vue-tsc --noEmit 2>&1 | head -20`

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(web): add sidebar state, drag resize refs and handlers"
```

<!-- PLAN_CONTINUES -->

---

### Task 2: Restructure template — sidebar

**Files:**
- Modify: `web/src/views/ChatView.vue:314-391` (template section)

- [ ] **Step 1: Replace template opening and add sidebar**

Replace the current template (lines 314-391) entirely. Start with the outer wrapper and sidebar:

Replace:
```html
<template>
  <div class="chat-page">
    <div class="chat-area">
      <div class="chat-header">
        <button class="conv-toggle" @click="showConvList = !showConvList">≡</button>
        <input v-if="editingHeaderTitle" class="conv-title-input"
               v-model="editTitleText"
               @keydown.enter="saveHeaderTitle"
               @keydown.escape="cancelEdit"
               @blur="saveHeaderTitle"
               @vue:mounted="($event: any) => $event.el.focus()" />
        <span v-else class="conv-title" @click="startEditHeaderTitle">{{ activeConv?.title || '新对话' }}</span>
        <span class="current-model" v-if="currentModelName">{{ currentModelName }}</span>
        <button class="new-conv-btn" @click="createNewConversation()">+</button>
      </div>

      <div v-if="showConvList" class="conv-dropdown">
        <div v-for="c in conversations" :key="c.id" class="conv-item"
             :class="{ active: c.id === activeConvId }"
             @click="selectConversation(c.id); showConvList = false">
          <input v-if="editingConvId === c.id" class="conv-item-input"
                 v-model="editTitleText"
                 @keydown.enter="saveConvTitle(c.id)"
                 @keydown.escape="cancelEdit"
                 @blur="saveConvTitle(c.id)"
                 @click.stop
                 @vue:mounted="($event: any) => $event.el.focus()" />
          <span v-else class="conv-item-title" @dblclick.stop="startEditConvTitle(c.id, c.title)">{{ c.title || '未命名对话' }}</span>
          <button class="conv-del" @click.stop="handleDeleteConversation(c.id)">×</button>
        </div>
      </div>
```

With:
```html
<template>
  <div class="chat-page" ref="chatPageRef" :class="{ dragging: isDragging }">
    <!-- Sidebar -->
    <div class="sidebar" :class="{ collapsed: !sidebarOpen }">
      <div class="sidebar-header">
        <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <button class="sidebar-new" @click="createNewConversation()">+ New</button>
      </div>
      <div class="sidebar-body">
        <div v-for="c in conversations" :key="c.id" class="conv-item"
             :class="{ active: c.id === activeConvId }"
             @click="selectConversation(c.id)">
          <input v-if="editingConvId === c.id" class="conv-item-input"
                 v-model="editTitleText"
                 @keydown.enter="saveConvTitle(c.id)"
                 @keydown.escape="cancelEdit"
                 @blur="saveConvTitle(c.id)"
                 @click.stop
                 @vue:mounted="($event: any) => $event.el.focus()" />
          <span v-else class="conv-item-title" @dblclick.stop="startEditConvTitle(c.id, c.title)">{{ c.title || '未命名对话' }}</span>
          <button class="conv-del" @click.stop="handleDeleteConversation(c.id)">×</button>
        </div>
      </div>
    </div>

    <!-- Chat main -->
    <div class="chat-main">
      <div class="chat-header">
        <button v-if="!sidebarOpen" class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <button v-if="!sidebarOpen" class="header-new-btn" @click="createNewConversation()">+</button>
        <input v-if="editingHeaderTitle" class="conv-title-input"
               v-model="editTitleText"
               @keydown.enter="saveHeaderTitle"
               @keydown.escape="cancelEdit"
               @blur="saveHeaderTitle"
               @vue:mounted="($event: any) => $event.el.focus()" />
        <span v-else class="conv-title" @click="startEditHeaderTitle">{{ activeConv?.title || '新对话' }}</span>
        <span class="current-model" v-if="currentModelName">{{ currentModelName }}</span>
      </div>
```

- [ ] **Step 2: Replace chat body and target panel section**

Replace the rest of the template (from `<div class="chat-messages"` through `</template>`):

```html
      <div class="chat-messages" ref="messagesRef">
        <ChatMessage
          v-for="msg in messages" :key="msg.id"
          :role="msg.role" :blocks="msg.blocks"
          :confirm="msg.confirm"
          :is-streaming="msg.isStreaming"
          @confirm="handleConfirm"
        />
        <div v-if="messages.length === 0" class="empty-state">
          输入消息开始对话...
        </div>
      </div>

      <div v-if="showModelPicker" class="model-picker">
        <div class="model-picker-header">
          <span>当前模型: <strong>{{ currentModel || '未选择' }}</strong></span>
          <button class="btn btn-sm" @click="showModelPicker = false">关闭</button>
        </div>
        <div class="model-picker-list">
          <div v-for="m in availableModels" :key="m.id"
               class="model-picker-item"
               :class="{ active: m.id === currentModel }"
               @click="selectModel(m.id)">
            <span>{{ m.display_name || m.id }}</span>
            <span v-if="m.id === currentModel" class="model-check">✓ 当前</span>
          </div>
        </div>
      </div>

      <div class="chat-input">
        <textarea
          v-model="inputText"
          @keydown.enter.exact.prevent="send"
          placeholder="输入运维指令..."
          :disabled="isStreaming"
          rows="1"
        ></textarea>
        <button @click="send" :disabled="isStreaming || !inputText.trim()" class="send-btn">
          {{ isStreaming ? '...' : '发送' }}
        </button>
      </div>
    </div>

    <!-- Drag handle -->
    <div class="drag-handle" @mousedown="startDrag">
      <div class="drag-indicator"></div>
    </div>

    <!-- Target panel -->
    <TargetPanel :devices="devices" class="target-side" :style="{ flexBasis: targetWidth + 'px' }" />
  </div>
</template>
```

- [ ] **Step 3: Verify template compiles**

Run: `cd /Users/cw/fty.ai/spider.ai/web && npx vue-tsc --noEmit 2>&1 | head -20`

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(web): restructure chat template with sidebar and drag handle"
```

---

### Task 3: Replace CSS styles

**Files:**
- Modify: `web/src/views/ChatView.vue:393-434` (style section)

- [ ] **Step 1: Replace all scoped styles**

Replace the entire `<style scoped>` block (lines 393-434) with new styles. The new styles cover: sidebar, chat-main, drag-handle, target-side, and preserve existing chat-messages/input/model-picker styles.

Replace:
```css
<style scoped>
.chat-page { display: flex; height: 100%; gap: 0; }
.chat-area { flex: 7; display: flex; flex-direction: column; min-width: 0; position: relative; }
.target-side { flex: 3; min-width: 280px; max-width: 400px; }
```

With:
```css
<style scoped>
.chat-page { display: flex; height: 100%; gap: 0; }
.chat-page.dragging { user-select: none; cursor: col-resize; }

/* Sidebar */
.sidebar { width: 240px; border-right: 1px solid var(--border); display: flex; flex-direction: column; background: var(--panel); transition: width 0.2s ease, opacity 0.2s ease; overflow: hidden; flex-shrink: 0; }
.sidebar.collapsed { width: 0; border-right: none; opacity: 0; }
.sidebar-header { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.sidebar-toggle { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 14px; flex-shrink: 0; }
.sidebar-toggle:hover { background: var(--row-hover); }
.sidebar-new { flex: 1; background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 13px; font-family: 'SF Mono', monospace; }
.sidebar-new:hover { background: var(--row-hover); }
.sidebar-body { flex: 1; overflow-y: auto; padding: 8px; }

/* Chat main */
.chat-main { flex: 1; display: flex; flex-direction: column; min-width: 300px; position: relative; }

/* Target side */
.target-side { min-width: 200px; max-width: 50vw; flex-shrink: 0; }
```

- [ ] **Step 2: Replace header and conv-item styles**

Replace:
```css
.chat-header { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid var(--border); background: var(--panel); }
.conv-toggle { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 14px; }
.conv-toggle:hover { background: var(--row-hover); }
```

With:
```css
.chat-header { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid var(--border); background: var(--panel); }
.header-new-btn { background: none; border: 1px solid var(--border); color: var(--text); width: 28px; height: 28px; border-radius: 4px; cursor: pointer; font-size: 16px; flex-shrink: 0; }
.header-new-btn:hover { background: var(--row-hover); }
```

- [ ] **Step 3: Remove old dropdown styles, add drag-handle styles**

Remove these lines entirely:
```css
.conv-dropdown { position: absolute; top: 48px; left: 16px; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; z-index: 10; max-height: 300px; overflow-y: auto; min-width: 250px; }
```

And remove:
```css
.new-conv-btn { background: var(--primary); color: #fff; border: none; width: 28px; height: 28px; border-radius: 4px; cursor: pointer; font-size: 16px; }
.new-conv-btn:hover { background: var(--primary-hover); }
```

Add before `</style>`:
```css
/* Drag handle */
.drag-handle { width: 5px; cursor: col-resize; background: transparent; display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: background 0.15s; }
.drag-handle:hover, .chat-page.dragging .drag-handle { background: var(--primary); opacity: 0.3; }
.drag-indicator { width: 2px; height: 32px; border-radius: 1px; background: var(--border); }
```

- [ ] **Step 4: Start dev server and verify in browser**

Run: `cd /Users/cw/fty.ai/spider.ai/web && npm run dev`

Open browser, navigate to chat page. Verify:
1. Sidebar visible on left with conversation list
2. Toggle button collapses/expands sidebar
3. Drag handle visible between chat and target panel
4. Drag handle resizes target panel width
5. Chat header shows toggle + new button only when sidebar collapsed

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(web): chat layout with collapsible sidebar and drag resize"
```

---

### Task 4: Polish and verify

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Test sidebar persistence**

1. Collapse sidebar → refresh page → sidebar should stay collapsed
2. Expand sidebar → refresh page → sidebar should stay expanded

- [ ] **Step 2: Test drag persistence**

1. Drag target panel to ~350px → refresh page → width should persist
2. Verify min-width (200px) and max-width (50vw) constraints work

- [ ] **Step 3: Test conversation interactions**

1. Click conversation in sidebar → loads messages
2. Double-click conversation → inline rename works
3. Click delete (×) → conversation removed
4. Click "+ New" in sidebar → creates new conversation
5. When sidebar collapsed, click "+" in header → creates new conversation
6. Click title in header → inline edit works

- [ ] **Step 4: Fix any visual issues found during testing**

Address spacing, alignment, or transition issues discovered in steps 1-3.

- [ ] **Step 5: Final commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "fix(web): chat layout polish and edge case fixes"
```
