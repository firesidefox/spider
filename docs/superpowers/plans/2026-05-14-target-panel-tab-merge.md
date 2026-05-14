# Target Panel Tab Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将右侧 TargetPanel 合并到左侧 sidebar，改为 Tab 切换（对话/目标），移除右侧面板，左侧宽度可拖拽调整。

**Architecture:** 左侧 sidebar 顶部加 Tab 栏，`sidebarTab` ref 控制显示对话列表还是设备列表。「目标」Tab 显示角标（failed 红色数字，executing 黄点）。左侧宽度改为可拖拽，范围 180–400px，存 localStorage。右侧 TargetPanel、drag-handle、startDrag 逻辑全部移除。

**Tech Stack:** Vue 3 Composition API, TypeScript, CSS scoped

---

## File Map

- Modify: `web/src/views/ChatView.vue` — 所有改动集中在此文件

---

### Task 1: 移除右侧面板，加左侧宽度状态

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: 删除 targetWidth ref 和 startDrag 函数**

找到并删除以下代码块（约第 141 行和第 204–226 行）：

```typescript
// 删除这行
const targetWidth = ref(parseInt(localStorage.getItem('spider-target-width') || '280'))

// 删除整个 startDrag 函数（约第 204–226 行）
function startDrag(e: MouseEvent) {
  isDragging.value = true
  const startX = e.clientX
  const startWidth = targetWidth.value
  function onMove(ev: MouseEvent) { ... }
  function onUp() { ... }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}
// 注：删除整个函数体，不是替换
```

- [ ] **Step 2: 添加左侧宽度 ref 和拖拽函数**

在 `const sidebarOpen` 附近添加：

```typescript
const sidebarWidth = ref(parseInt(localStorage.getItem('spider-sidebar-width') || '240'))

function startSidebarResize(e: MouseEvent) {
  isDragging.value = true
  const startX = e.clientX
  const startWidth = sidebarWidth.value

  function onMove(ev: MouseEvent) {
    const newWidth = Math.min(400, Math.max(180, startWidth + ev.clientX - startX))
    sidebarWidth.value = newWidth
  }

  function onUp() {
    isDragging.value = false
    localStorage.setItem('spider-sidebar-width', String(sidebarWidth.value))
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}
```

- [ ] **Step 3: 删除 TargetPanel import**

删除文件顶部两行：
```typescript
import TargetPanel from '../components/TargetPanel.vue'
import type { DeviceStatus } from '../components/TargetPanel.vue'
```

添加替代 import（只保留类型）：
```typescript
import type { DeviceStatus } from '../components/TargetPanel.vue'
```

- [ ] **Step 4: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无 TypeScript 错误，build 成功。

- [ ] **Step 5: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "refactor(chat): remove right TargetPanel, add sidebar width resize"
```

---

### Task 2: 添加 Tab 状态和角标计算

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: 添加 sidebarTab ref**

在 `const sidebarOpen` 附近添加：

```typescript
const sidebarTab = ref<'conv' | 'target'>(
  (localStorage.getItem('spider-sidebar-tab') as 'conv' | 'target') || 'conv'
)

function setSidebarTab(tab: 'conv' | 'target') {
  sidebarTab.value = tab
  localStorage.setItem('spider-sidebar-tab', tab)
}
```

- [ ] **Step 2: 添加角标计算 computed**

在 `devices` ref 附近添加：

```typescript
const targetBadge = computed(() => {
  const failed = devices.value.filter(d => d.status === 'failed').length
  const executing = devices.value.filter(d => d.status === 'executing').length
  if (failed > 0) return { type: 'failed' as const, count: failed }
  if (executing > 0) return { type: 'executing' as const, count: 0 }
  return null
})
```

- [ ] **Step 3: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无错误。

- [ ] **Step 4: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add sidebarTab state and targetBadge computed"
```

---

### Task 3: 更新 template — sidebar 加 Tab 栏

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: 替换 sidebar-header**

找到现有 sidebar-header：
```html
<div class="sidebar-header">
  <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
  <button class="sidebar-new" @click="createNewConversation()">+ New</button>
</div>
```

替换为：
```html
<div class="sidebar-header">
  <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
  <div class="sidebar-tabs">
    <button class="sidebar-tab" :class="{ active: sidebarTab === 'conv' }" @click="setSidebarTab('conv')">对话</button>
    <button class="sidebar-tab" :class="{ active: sidebarTab === 'target' }" @click="setSidebarTab('target')">
      目标
      <span v-if="targetBadge" class="tab-badge" :class="targetBadge.type">
        {{ targetBadge.type === 'failed' ? targetBadge.count : '' }}
      </span>
    </button>
  </div>
  <button v-if="sidebarTab === 'conv'" class="sidebar-new" @click="createNewConversation()">+</button>
</div>
```

- [ ] **Step 2: 替换 sidebar-body**

找到现有 sidebar-body：
```html
<div class="sidebar-body">
  <div v-for="c in conversations" ...>
    ...
  </div>
</div>
```

替换为（保留对话列表原有内容，加 TargetPanel 条件渲染）：
```html
<div class="sidebar-body">
  <template v-if="sidebarTab === 'conv'">
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
      <span v-if="c.status === 'processing'" class="conv-processing-dot" title="处理中"></span>
      <button class="conv-del" @click.stop="handleDeleteConversation(c.id)">×</button>
    </div>
  </template>
  <template v-else>
    <TargetPanel :devices="devices" />
  </template>
</div>
```

- [ ] **Step 3: 恢复 TargetPanel import（完整组件 import）**

将之前改为 type-only 的 import 改回：
```typescript
import TargetPanel from '../components/TargetPanel.vue'
import type { DeviceStatus } from '../components/TargetPanel.vue'
```

- [ ] **Step 4: 删除 template 底部的 drag-handle 和 TargetPanel**

找到并删除：
```html
<!-- Drag handle -->
<div class="drag-handle" @mousedown="startDrag">
  <div class="drag-indicator"></div>
</div>

<!-- Target panel -->
<TargetPanel :devices="devices" class="target-side" :style="{ flexBasis: targetWidth + 'px' }" />
```

- [ ] **Step 5: 在 sidebar 右边加拖拽手柄**

在 sidebar div 结束标签 `</div>` 之后、chat-main 之前插入：

```html
<div class="sidebar-resize-handle" @mousedown="startSidebarResize">
  <div class="drag-indicator"></div>
</div>
```

- [ ] **Step 6: sidebar 绑定宽度**

找到：
```html
<div class="sidebar" :class="{ collapsed: !sidebarOpen }">
```

改为：
```html
<div class="sidebar" :class="{ collapsed: !sidebarOpen }" :style="{ width: sidebarOpen ? sidebarWidth + 'px' : '0' }">
```

- [ ] **Step 7: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无错误。

- [ ] **Step 8: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(chat): sidebar tab UI — 对话/目标 tab with badge, resizable width"
```

---

### Task 4: 更新 CSS

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: 删除旧 CSS**

删除以下 CSS 规则：
```css
.target-side { min-width: 200px; max-width: 50vw; flex-shrink: 0; }
.drag-handle { width: 5px; cursor: col-resize; ... }
.drag-handle:hover, .chat-page.dragging .drag-handle { ... }
.drag-indicator { ... }
```

- [ ] **Step 2: 添加新 CSS**

在 `.sidebar` 规则附近添加：

```css
.sidebar { border-right: 1px solid var(--border); display: flex; flex-direction: column; background: var(--panel); transition: width 0.2s ease, opacity 0.2s ease; overflow: hidden; flex-shrink: 0; min-width: 0; }
.sidebar.collapsed { width: 0 !important; border-right: none; opacity: 0; }

.sidebar-tabs { display: flex; flex: 1; gap: 2px; }
.sidebar-tab { flex: 1; background: none; border: none; color: var(--text-sub); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; position: relative; white-space: nowrap; }
.sidebar-tab:hover { background: var(--row-hover); }
.sidebar-tab.active { color: var(--primary); background: var(--row-hover); }

.tab-badge { position: absolute; top: 1px; right: 2px; min-width: 14px; height: 14px; border-radius: 7px; font-size: 10px; display: flex; align-items: center; justify-content: center; padding: 0 3px; }
.tab-badge.failed { background: var(--red); color: #fff; }
.tab-badge.executing { background: var(--yellow); width: 7px; height: 7px; min-width: 0; border-radius: 50%; top: 3px; right: 3px; }

.sidebar-resize-handle { width: 5px; cursor: col-resize; background: transparent; display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: background 0.15s; }
.sidebar-resize-handle:hover, .chat-page.dragging .sidebar-resize-handle { background: rgba(108, 140, 255, 0.3); }
.drag-indicator { width: 2px; height: 32px; border-radius: 1px; background: var(--border); }
```

- [ ] **Step 3: 修改 sidebar 原有 width 规则**

找到：
```css
.sidebar { width: 240px; border-right: 1px solid var(--border); ... }
```

删除 `width: 240px`（宽度现在由 `:style` 绑定控制）。

- [ ] **Step 4: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: 无错误，无 CSS 警告。

- [ ] **Step 5: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "style(chat): update sidebar CSS for tab layout and resize handle"
```

---

### Task 5: 端到端验证

**Files:**
- Read: `web/src/views/ChatView.vue`

- [ ] **Step 1: 启动测试服务器**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data &
```

- [ ] **Step 2: 验证 Tab 切换**

用 Playwright 打开 `http://localhost:8002`，验证：
1. 左侧面板顶部有「对话」「目标」两个 Tab
2. 点击「目标」Tab，显示设备列表（TargetPanel 内容）
3. 点击「对话」Tab，显示对话列表
4. 右侧无 TargetPanel

```javascript
// Playwright 验证
const tabs = await page.locator('.sidebar-tab').allTextContents()
console.assert(tabs[0].includes('对话'), '对话 tab missing')
console.assert(tabs[1].includes('目标'), '目标 tab missing')
await page.click('.sidebar-tab:nth-child(2)')
const targetPanel = await page.locator('.target-panel').isVisible()
console.assert(targetPanel, 'TargetPanel not visible in target tab')
```

- [ ] **Step 3: 验证宽度拖拽**

拖拽 sidebar 右边框，确认宽度在 180–400px 范围内变化，刷新后宽度保持。

- [ ] **Step 4: 验证折叠**

点击 ≡ 按钮，sidebar 折叠为 0 宽度；再点击展开，宽度恢复。

- [ ] **Step 5: 验证角标（手动）**

在 DevTools console 执行：
```javascript
// 模拟 failed 设备触发角标
// 实际测试需要有 failed 状态的设备
```

角标逻辑通过 computed 验证，无需 mock。

- [ ] **Step 6: 停止测试服务器**

```bash
pkill -f spider-test
```

- [ ] **Step 7: Final commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(chat): target panel merged into sidebar tabs — complete"
```
