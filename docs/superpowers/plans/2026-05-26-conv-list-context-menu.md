# 对话列表项上下文菜单 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除侧边栏对话列表项的 × 删除按钮，替换为 ⋯ 更多图标，点击弹出菜单（重命名 / 批量管理 / 删除），并实现批量管理模式（checkbox 多选 + 全选 + 批量删除）。

**Architecture:** 所有改动集中在 `ChatView.vue` 单文件。新增三个 ref 管理菜单/批量状态，复用现有 `startEditConvTitle` 和 `handleDeleteConversation` 逻辑，新增批量删除函数。模板中侧边栏 header 和列表项根据 `batchMode` 条件渲染两套 UI。

**Tech Stack:** Vue 3 Composition API, TypeScript, scoped CSS（无新依赖）

---

## File Map

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/src/views/ChatView.vue` | Modify | 唯一改动文件：state、template、style |

---

### Task 1: 新增状态 ref

**Files:**
- Modify: `web/src/views/ChatView.vue:427-428`（在 `editingConvId` 附近插入）

- [ ] **Step 1: 在现有 ref 声明区域（约第 427 行）后插入三个新 ref**

找到：
```ts
const editingConvId = ref<string | null>(null)
const editTitleText = ref('')
```

改为：
```ts
const editingConvId = ref<string | null>(null)
const editTitleText = ref('')
const menuOpenConvId = ref<string | null>(null)
const batchMode = ref(false)
const selectedConvIds = ref<Set<string>>(new Set())
```

- [ ] **Step 2: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出（无错误）

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add menu/batch mode state refs"
```

---

### Task 2: 新增菜单控制函数和批量操作函数

**Files:**
- Modify: `web/src/views/ChatView.vue`（在 `cancelEdit` 函数后插入，约第 461 行）

- [ ] **Step 1: 在 `cancelEdit` 函数后插入以下函数**

```ts
function openConvMenu(id: string) {
  menuOpenConvId.value = menuOpenConvId.value === id ? null : id
}

function closeConvMenu() {
  menuOpenConvId.value = null
}

function enterBatchMode() {
  menuOpenConvId.value = null
  batchMode.value = true
  selectedConvIds.value = new Set()
}

function exitBatchMode() {
  batchMode.value = false
  selectedConvIds.value = new Set()
}

function toggleSelectConv(id: string) {
  const s = new Set(selectedConvIds.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selectedConvIds.value = s
}

function toggleSelectAll() {
  if (selectedConvIds.value.size === conversations.value.length) {
    selectedConvIds.value = new Set()
  } else {
    selectedConvIds.value = new Set(conversations.value.map(c => c.id))
  }
}

async function handleBatchDelete() {
  const ids = Array.from(selectedConvIds.value)
  for (const id of ids) {
    await handleDeleteConversation(id)
  }
  exitBatchMode()
}
```

- [ ] **Step 2: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add conv menu and batch operation functions"
```

---

### Task 3: 替换侧边栏 header 模板

**Files:**
- Modify: `web/src/views/ChatView.vue:1188-1194`

- [ ] **Step 1: 找到现有 sidebar-header，替换为条件渲染版本**

找到（约第 1188 行）：
```html
      <div class="sidebar-header">
        <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <div class="sidebar-tabs">
          <button class="sidebar-tab active">对话</button>
        </div>
        <button class="sidebar-new" @click="createNewConversation()">+</button>
      </div>
```

替换为：
```html
      <div class="sidebar-header">
        <template v-if="!batchMode">
          <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
          <div class="sidebar-tabs">
            <button class="sidebar-tab active">对话</button>
          </div>
          <button class="sidebar-new" @click="createNewConversation()">+</button>
        </template>
        <template v-else>
          <span class="batch-mode-label">批量管理</span>
          <span style="flex:1"></span>
          <button class="batch-select-all" @click="toggleSelectAll">{{ selectedConvIds.size === conversations.length ? '取消全选' : '全选' }}</button>
          <button class="batch-cancel" @click="exitBatchMode">取消</button>
        </template>
      </div>
```

- [ ] **Step 2: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add batch mode header to sidebar"
```

---

### Task 4: 替换列表项模板（⋯ 菜单 + 批量 checkbox）

**Files:**
- Modify: `web/src/views/ChatView.vue:1196-1209`

- [ ] **Step 1: 找到现有 conv-item 循环，替换为新版本**

找到（约第 1196 行）：
```html
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
```

替换为：
```html
        <div v-for="c in conversations" :key="c.id" class="conv-item"
             :class="{ active: c.id === activeConvId, 'batch-selected': batchMode && selectedConvIds.has(c.id) }"
             @click="batchMode ? toggleSelectConv(c.id) : selectConversation(c.id)">
          <input type="checkbox" v-if="batchMode" class="conv-checkbox"
                 :checked="selectedConvIds.has(c.id)"
                 @click.stop="toggleSelectConv(c.id)" />
          <input v-else-if="editingConvId === c.id" class="conv-item-input"
                 v-model="editTitleText"
                 @keydown.enter="saveConvTitle(c.id)"
                 @keydown.escape="cancelEdit"
                 @blur="saveConvTitle(c.id)"
                 @click.stop
                 @vue:mounted="($event: any) => $event.el.focus()" />
          <span v-else class="conv-item-title" @dblclick.stop="startEditConvTitle(c.id, c.title)">{{ c.title || '未命名对话' }}</span>
          <span v-if="c.status === 'processing'" class="conv-processing-dot" title="处理中"></span>
          <div v-if="!batchMode" class="conv-menu-wrap">
            <button class="conv-more" @click.stop="openConvMenu(c.id)" title="更多">⋯</button>
            <div v-if="menuOpenConvId === c.id" class="conv-menu" @click.stop>
              <button class="conv-menu-item" @click="startEditConvTitle(c.id, c.title); closeConvMenu()">✏ 重命名</button>
              <button class="conv-menu-item" @click="enterBatchMode()">☑ 批量管理</button>
              <div class="conv-menu-divider"></div>
              <button class="conv-menu-item conv-menu-item--danger" @click="handleDeleteConversation(c.id); closeConvMenu()">✕ 删除</button>
            </div>
          </div>
        </div>
```

- [ ] **Step 2: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): replace delete button with context menu and batch checkboxes"
```

---

### Task 5: 新增批量操作底部栏

**Files:**
- Modify: `web/src/views/ChatView.vue`（在 `</div><!-- sidebar-body -->` 后，`</div><!-- sidebar -->` 前插入）

- [ ] **Step 1: 找到 sidebar-body 结束标签（约第 1210 行），在其后插入批量操作栏**

找到：
```html
      </div>
    </div>
    <div class="sidebar-resize-handle" @mousedown="startDrag">
```

替换为：
```html
      </div>
      <div v-if="batchMode" class="batch-action-bar">
        <span class="batch-count">已选 {{ selectedConvIds.size }}</span>
        <button class="batch-delete-btn" :disabled="selectedConvIds.size === 0" @click="handleBatchDelete">删除选中</button>
      </div>
    </div>
    <div class="sidebar-resize-handle" @mousedown="startDrag">
```

- [ ] **Step 2: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add batch action bar to sidebar"
```

---

### Task 6: 新增 CSS 样式

**Files:**
- Modify: `web/src/views/ChatView.vue`（在 `.conv-del` 样式行附近，约第 1433 行）

- [ ] **Step 1: 找到 `.conv-del` 样式行，删除它并插入新样式**

找到并删除：
```css
.conv-del { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 16px; padding: 0 4px; flex-shrink: 0; }
.conv-del:hover { color: var(--red); }
```

在 `.conv-processing-dot` 行之前插入：
```css
.conv-menu-wrap { position: relative; flex-shrink: 0; }
.conv-more { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 16px; padding: 0 4px; opacity: 0; transition: opacity 0.1s; }
.conv-item:hover .conv-more, .conv-item.active .conv-more { opacity: 1; }
.conv-more:hover { color: var(--text); }
.conv-menu { position: absolute; right: 0; top: 100%; background: var(--panel); border: 1px solid var(--border); border-radius: 6px; min-width: 140px; z-index: 100; padding: 4px 0; box-shadow: 0 4px 12px rgba(0,0,0,0.3); }
.conv-menu-item { display: block; width: 100%; background: none; border: none; color: var(--text); text-align: left; padding: 6px 14px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; }
.conv-menu-item:hover { background: var(--row-hover); }
.conv-menu-item--danger { color: var(--red); }
.conv-menu-divider { height: 1px; background: var(--border); margin: 2px 0; }
.conv-checkbox { accent-color: var(--primary); width: 13px; height: 13px; flex-shrink: 0; margin-right: 4px; cursor: pointer; }
.conv-item.batch-selected { background: var(--row-hover); }
.batch-mode-label { color: var(--primary); font-size: 12px; font-family: 'SF Mono', monospace; }
.batch-select-all { background: none; border: 1px solid var(--border); color: var(--text); padding: 2px 8px; border-radius: 4px; font-size: 11px; font-family: 'SF Mono', monospace; cursor: pointer; }
.batch-select-all:hover { background: var(--row-hover); }
.batch-cancel { background: none; border: none; color: var(--text-sub); padding: 2px 6px; font-size: 11px; font-family: 'SF Mono', monospace; cursor: pointer; }
.batch-cancel:hover { color: var(--text); }
.batch-action-bar { display: flex; align-items: center; padding: 8px 10px; border-top: 1px solid var(--border); flex-shrink: 0; }
.batch-count { color: var(--text-sub); font-size: 12px; font-family: 'SF Mono', monospace; flex: 1; }
.batch-delete-btn { background: var(--red); border: none; color: #fff; padding: 3px 10px; border-radius: 4px; font-size: 11px; font-family: 'SF Mono', monospace; cursor: pointer; }
.batch-delete-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.batch-delete-btn:not(:disabled):hover { opacity: 0.85; }
```

- [ ] **Step 2: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add context menu and batch mode CSS"
```

---

### Task 7: 点击外部关闭菜单

**Files:**
- Modify: `web/src/views/ChatView.vue`（chat-main div 的 @click handler，约第 1217 行）

- [ ] **Step 1: 找到 chat-main 的 @click，追加 closeConvMenu()**

找到：
```html
    <div class="chat-main" @click="showExportMenu = false; showModeDropdown = false">
```

替换为：
```html
    <div class="chat-main" @click="showExportMenu = false; showModeDropdown = false; closeConvMenu()">
```

- [ ] **Step 2: 在 sidebar-body 的 @click 也关闭菜单（点击列表项空白区域）**

找到：
```html
      <div class="sidebar-body">
```

替换为：
```html
      <div class="sidebar-body" @click="closeConvMenu()">
```

- [ ] **Step 3: 验证 TypeScript 编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```

Expected: 无输出

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): close conv menu on outside click"
```

---

### Task 8: 构建并验证

**Files:**
- No file changes — build and verify only

- [ ] **Step 1: 全量构建前端**

```bash
cd /Users/cw/fty.ai/spider.ai && go build -a -o /tmp/spider-test ./cmd/spider
```

Expected: 无错误输出，生成 `/tmp/spider-test`

- [ ] **Step 2: 启动测试服务器**

```bash
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: 打开浏览器验证以下场景**

1. 侧边栏对话列表项 hover → 出现 ⋯ 图标，× 按钮消失
2. 点击 ⋯ → 弹出菜单（重命名 / 批量管理 / 删除）
3. 点击菜单外部 → 菜单关闭
4. 点击「重命名」→ 标题变为输入框（与双击行为一致）
5. 点击「删除」→ 对话被删除
6. 点击「批量管理」→ 进入批量模式：header 变为「批量管理 全选 取消」，每项出现 checkbox，底部出现操作栏
7. 勾选若干对话 → 底部「已选 N」更新
8. 点击「全选」→ 全部勾选；再点「取消全选」→ 全部取消
9. 点击「删除选中」→ 批量删除，退出批量模式
10. 点击「取消」→ 退出批量模式，恢复正常

- [ ] **Step 4: 停止测试服务器（Ctrl+C）**

- [ ] **Step 5: 最终 commit（如有遗漏修复）**

```bash
git add web/src/views/ChatView.vue
git commit -m "fix(chat): post-verification fixes"
```

---

## 变量/函数名速查

| 名称 | 类型 | 说明 |
|------|------|------|
| `menuOpenConvId` | `ref<string\|null>` | 当前展开菜单的对话 id |
| `batchMode` | `ref<boolean>` | 批量管理模式开关 |
| `selectedConvIds` | `ref<Set<string>>` | 批量模式已选 id 集合 |
| `openConvMenu(id)` | function | 切换指定对话的菜单 |
| `closeConvMenu()` | function | 关闭当前菜单 |
| `enterBatchMode()` | function | 进入批量模式 |
| `exitBatchMode()` | function | 退出批量模式，清空选中 |
| `toggleSelectConv(id)` | function | 切换单条对话选中状态 |
| `toggleSelectAll()` | function | 全选/取消全选 |
| `handleBatchDelete()` | async function | 批量删除已选对话 |
