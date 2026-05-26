# 知识库内嵌条目详情 + 原文视图 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将知识库三栏布局改为两栏，条目详情内嵌展开在卡片中，并新增 友好/原文 视图切换。

**Architecture:** 仅修改 `KnowledgeView.vue`。移除右侧 `kb-detail` 面板，条目面板改为 `flex:1`。新增 `entriesView` / `expandedEntries` / `entryDetails` 等 ref 管理视图状态，`toggleEntry()` 替代原 `selectEntry()`。

**Tech Stack:** Vue 3 Composition API，现有 `getEntry` API，Playwright e2e 测试

**Spec:** `docs/superpowers/specs/2026-05-26-kb-inline-entry-detail-design.md`  
**Mockup:** `.superpowers/brainstorm/90508-1779797634/content/inline-detail.html`

---

### Task 1: 移除右侧详情面板，条目面板改为全宽

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: 删除 `<section class="kb-detail">` 整块**

在 template 中找到并删除以下整段（约第 111–176 行）：

```html
<!-- 删除从这里 -->
<section class="kb-detail">
  ...（整个 detail 面板，含所有子元素）...
</section>
<!-- 到这里 -->
```

- [ ] **Step 2: 条目面板去掉固定宽度，改为 flex:1**

找到 `<section v-if="activeDoc" class="kb-entries" :style="{ width: entriesWidth + 'px' }">` 改为：

```html
<section v-if="activeDoc" class="kb-entries">
```

- [ ] **Step 3: 删除 `entriesWidth` 相关代码**

删除以下几处：
```ts
// 删除
const entriesWidth = ref(400)
const ENTRIES_MIN = 300, ENTRIES_MAX = 600
```

`startResize` 函数中删除 `entries` 分支：
```ts
// 删除
} else {
  entriesWidth.value = Math.min(ENTRIES_MAX, Math.max(ENTRIES_MIN, startW + dx))
}
```

`stopResize` 中删除：
```ts
// 删除
localStorage.setItem('kb_entries_width', String(entriesWidth.value))
```

`loadPersistence` 中删除：
```ts
// 删除
const ew = +(localStorage.getItem('kb_entries_width') ?? '0')
if (ew) entriesWidth.value = Math.min(ENTRIES_MAX, Math.max(ENTRIES_MIN, ew))
```

- [ ] **Step 4: 删除条目面板内的 resize-handle**

找到 `kb-entries` 中的：
```html
<div class="resize-handle" @mousedown.prevent="startResize('entries', $event)"></div>
```
删除这一行。

- [ ] **Step 5: 更新 `.kb-entries` CSS，改为 flex:1**

找到：
```css
.kb-entries {
  position: relative; flex-shrink: 0;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; overflow: hidden;
}
```
改为：
```css
.kb-entries {
  position: relative; flex: 1; min-width: 0;
  background: var(--panel);
  display: flex; flex-direction: column; overflow: hidden;
}
```

- [ ] **Step 6: 构建确认布局正常**

```bash
cd /Users/cw/fty.ai/spider.ai
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

打开 http://localhost:8002，进入知识库，确认：右侧详情面板已消失，条目面板占满宽度。

- [ ] **Step 7: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "refactor(kb): remove detail panel, entries panel takes full width"
```

---

### Task 2: 替换状态 refs

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: 删除旧的 detail 相关 refs**

删除以下 ref 声明：
```ts
// 全部删除
const activeEntryId = ref<number | null>(null)
const activeEntryDetail = ref<KnowledgeEntryDetail | null>(null)
const loadingDetail = ref(false)
const activeRespCode = ref('')
```

- [ ] **Step 2: 新增 refs**

在 `const focusedIdx = ref(-1)` 后面添加：

```ts
const expandedEntries = ref(new Set<number>())
const entryDetails = ref<Record<number, KnowledgeEntryDetail>>({})
const loadingEntries = ref(new Set<number>())
const entryRespCodes = ref<Record<number, string>>({})
const entriesView = ref<'friendly' | 'raw'>('friendly')
```

- [ ] **Step 3: 删除旧的 computed（responseTabs、activeRespBody）**

找到并删除：
```ts
// 删除整段
const responseTabs = computed(() => {
  ...
})
const activeRespBody = computed(() => {
  ...
})
watch(responseTabs, t => {
  ...
})
```

- [ ] **Step 4: 新增 per-entry 辅助函数**

在删除位置后添加：

```ts
interface RespTab { code: string; ok: boolean; description: string; example: any }

function entryRespTabs(detail: KnowledgeEntryDetail): RespTab[] {
  const r = detail.responses
  if (!r) return []
  return Object.keys(r).sort().map(code => {
    const v = r[code]
    return { code, ok: code.startsWith('2'), description: v.description || '', example: v.example }
  })
}

function entryRespBody(detail: KnowledgeEntryDetail, code: string): string {
  const tab = entryRespTabs(detail).find(t => t.code === code)
  if (!tab) return ''
  if (tab.example == null) return '(无示例)'
  if (typeof tab.example === 'string') return tab.example
  try { return JSON.stringify(tab.example, null, 2) } catch { return String(tab.example) }
}
```

- [ ] **Step 5: 更新 `watch(activeDoc)` 重置逻辑**

找到原来的 `watch(activeDoc, async d => { ... })`，在重置部分加入新 refs，并重置 entriesView：

```ts
watch(activeDoc, async d => {
  expandedEntries.value = new Set()
  entryDetails.value = {}
  loadingEntries.value = new Set()
  entryRespCodes.value = {}
  entriesView.value = 'friendly'
  if (!d) { sections.value = []; entriesBySection.value = {}; return }
  // 其余加载 sections/entries 逻辑不变
  loadingSections.value = true
  try {
    const ss = await getSections(d.id)
    sections.value = ss
    entriesBySection.value = {}
    await Promise.all(ss.map(async s => {
      try {
        entriesBySection.value = { ...entriesBySection.value, [s.id]: await getEntries(s.id) }
      } catch { entriesBySection.value = { ...entriesBySection.value, [s.id]: [] } }
    }))
    focusedIdx.value = filteredEntries.value.length ? 0 : -1
  } finally { loadingSections.value = false }
})
```

- [ ] **Step 6: 新增 `toggleEntry` 函数，删除旧 `selectEntry` / `closeDetail`**

删除 `selectEntry` 和 `closeDetail` 函数，替换为：

```ts
async function toggleEntry(e: KnowledgeEntry) {
  const id = e.id
  const next = new Set(expandedEntries.value)
  if (next.has(id)) {
    next.delete(id)
    expandedEntries.value = next
    return
  }
  next.add(id)
  expandedEntries.value = next
  focusedIdx.value = filteredEntries.value.findIndex(x => x.id === id)
  if (!entryDetails.value[id]) {
    const loading = new Set(loadingEntries.value)
    loading.add(id)
    loadingEntries.value = loading
    try {
      const detail = await getEntry(id)
      entryDetails.value = { ...entryDetails.value, [id]: detail }
      const tabs = entryRespTabs(detail)
      entryRespCodes.value = {
        ...entryRespCodes.value,
        [id]: tabs.length ? (tabs.find(t => t.ok) ?? tabs[0]).code : ''
      }
    } finally {
      const loading2 = new Set(loadingEntries.value)
      loading2.delete(id)
      loadingEntries.value = loading2
    }
  }
}
```

- [ ] **Step 7: 确认编译无报错**

```bash
cd /Users/cw/fty.ai/spider.ai/web
npm run build 2>&1 | tail -20
```

Expected: 构建成功，无 TypeScript 错误。

- [ ] **Step 8: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "refactor(kb): replace detail panel state with inline expansion refs"
```

---

### Task 3: 新增 友好/原文 tab UI + 原文视图

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: 在 entries-toolbar 末尾加 tab 行**

找到 entries-toolbar 内 `<div class="method-filters">` 块之后，`</div>` 关闭 entries-toolbar 之前，插入：

```html
<div class="view-tabs">
  <button class="view-tab" :class="{ active: entriesView === 'friendly' }"
    @click="entriesView = 'friendly'">友好</button>
  <button class="view-tab" :class="{ active: entriesView === 'raw' }"
    @click="entriesView = 'raw'">原文</button>
</div>
```

- [ ] **Step 2: 搜索框和方法过滤在原文模式下变灰**

为 entries-toolbar 内的 search input 和 method-filters 添加条件样式：

```html
<input v-model="entryQuery" class="filter-input"
  placeholder="🔍 搜索 API 路径或描述..."
  :style="entriesView === 'raw' ? 'opacity:0.4;pointer-events:none' : ''" />

<div class="method-filters"
  :style="entriesView === 'raw' ? 'opacity:0.4;pointer-events:none' : ''">
  ...
</div>
```

- [ ] **Step 3: 条目列表区域加原文视图分支**

找到 `<div class="entries-body">` 内的内容，在最外层加条件：

```html
<div class="entries-body">
  <!-- 原文视图 -->
  <pre v-if="entriesView === 'raw'" class="resp-body raw-source">{{ activeDoc?.raw_content }}</pre>

  <!-- 友好视图（原有内容，用 v-else 包裹） -->
  <template v-else>
    <div v-if="loadingSections" class="entries-loading">加载中...</div>
    <div v-else-if="!sections.length && !flatEntries.length" class="entries-empty">无条目</div>
    <div v-else class="entries-list">
      ...（原有条目卡片循环，下一个 Task 替换）...
    </div>
  </template>
</div>
```

- [ ] **Step 4: 新增 tab CSS**

在 style 块末尾添加：

```css
.view-tabs {
  display: flex;
  border-top: 1px solid var(--border);
  margin: 0 -12px;
}
.view-tab {
  flex: 1; text-align: center;
  font-size: 12px; font-weight: 600;
  padding: 7px 0; cursor: pointer;
  color: var(--muted); background: none; border: none;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}
.view-tab:hover { color: var(--text-sub); }
.view-tab.active { color: var(--primary); border-bottom-color: var(--primary); }
.raw-source {
  flex: 1; margin: 0; white-space: pre-wrap; word-break: break-all;
  overflow-y: auto;
}
```

- [ ] **Step 5: 构建 + 浏览器验证**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-test ./cmd/spider && /tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

打开 http://localhost:8002，进入知识库，选中文档，确认：
- 顶部出现 `[友好] [原文]` tab
- 点击"原文"：搜索框变灰、显示 raw YAML/JSON 内容
- 点击"友好"：恢复正常条目列表

- [ ] **Step 6: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat(kb): add friendly/raw view tab toggle"
```

---

### Task 4: 条目卡片内嵌展开详情

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: 替换条目卡片 template**

找到 entries-list 内的条目卡片循环：
```html
<div v-for="(entry, idx) in filteredEntries" :key="entry.id"
  class="entry-card" :class="{ active: activeEntryId === entry.id, focused: focusedIdx === idx }"
  @click="selectEntry(entry)">
  <div class="entry-row">
    <span class="method-badge" :class="entryMethod(entry).toLowerCase()">
      {{ entryMethod(entry) || '·' }}
    </span>
    <span class="entry-path">{{ entryPath(entry) }}</span>
    <button class="copy-btn" :title="'复制 ' + entryPath(entry)" @click.stop="copy(entryPath(entry))">📋</button>
  </div>
  <div class="entry-summary">{{ entry.summary }}</div>
</div>
```

替换为：

```html
<div v-for="(entry, idx) in filteredEntries" :key="entry.id"
  class="entry-card"
  :class="{
    expanded: expandedEntries.has(entry.id),
    focused: focusedIdx === idx
  }"
  @click="toggleEntry(entry)">

  <!-- 卡片头：始终显示 -->
  <div class="entry-row">
    <span class="method-badge" :class="entryMethod(entry).toLowerCase()">
      {{ entryMethod(entry) || '·' }}
    </span>
    <span class="entry-path">{{ entryPath(entry) }}</span>
    <button class="copy-btn" :title="'复制 ' + entryPath(entry)"
      @click.stop="copy(entryPath(entry))">📋</button>
  </div>
  <div class="entry-summary">{{ entry.summary }}</div>

  <!-- 内嵌详情：展开时显示 -->
  <div v-if="expandedEntries.has(entry.id)" class="inline-detail" @click.stop>

    <!-- 加载中 -->
    <div v-if="loadingEntries.has(entry.id)" class="inline-loading">加载中...</div>

    <template v-else-if="entryDetails[entry.id]">
      <!-- 描述 -->
      <div v-if="entryDetails[entry.id].description" class="inline-section">
        <h5>描述</h5>
        <p>{{ entryDetails[entry.id].description }}</p>
      </div>

      <!-- 参数 -->
      <div v-if="entryDetails[entry.id].parameters?.length" class="inline-section">
        <h5>参数</h5>
        <table class="inline-param-table">
          <thead><tr><th>名称</th><th>位置</th><th>类型</th><th>说明</th></tr></thead>
          <tbody>
            <tr v-for="p in entryDetails[entry.id].parameters" :key="p.name + (p.in || '')">
              <td>
                <code>{{ p.name }}</code>
                <span v-if="p.required" class="required-mark">*</span>
              </td>
              <td><span class="param-in">{{ p.in || '-' }}</span></td>
              <td><span class="param-type">{{ p.type || '-' }}</span></td>
              <td>{{ p.description || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 响应 -->
      <div v-if="entryRespTabs(entryDetails[entry.id]).length" class="inline-section">
        <h5>响应示例</h5>
        <div class="resp-tabs">
          <button v-for="t in entryRespTabs(entryDetails[entry.id])" :key="t.code"
            class="resp-tab"
            :class="{ active: entryRespCodes[entry.id] === t.code, ok: t.ok, err: !t.ok }"
            @click.stop="entryRespCodes = { ...entryRespCodes, [entry.id]: t.code }">
            <span class="resp-icon">{{ t.ok ? '✓' : '✗' }}</span>
            <span>{{ t.code }}</span>
            <span class="resp-desc">{{ t.description }}</span>
          </button>
        </div>
        <pre class="resp-body"><code>{{ entryRespBody(entryDetails[entry.id], entryRespCodes[entry.id]) }}</code></pre>
      </div>

      <!-- fallback: 原始内容 -->
      <div v-if="!entryDetails[entry.id].description
                 && !entryDetails[entry.id].parameters?.length
                 && !entryRespTabs(entryDetails[entry.id]).length"
           class="inline-section">
        <h5>原始内容</h5>
        <pre class="resp-body"><code>{{ entryDetails[entry.id].content }}</code></pre>
      </div>
    </template>

    <!-- 收起按钮 -->
    <div class="collapse-btn" @click.stop="toggleEntry(entry)">▲ 收起</div>
  </div>
</div>
```

- [ ] **Step 2: 修复 `entryRespCodes` 响应式更新**

在 `toggleEntry` 中已用对象展开赋值，template 里的点击也要用同样方式。检查 Task 4 Step 1 中 `@click.stop` 的写法——`entryRespCodes = { ...entryRespCodes, [entry.id]: t.code }` 会报错，因为 `entryRespCodes` 是 `ref`。

修正为：
```html
@click.stop="entryRespCodes.value = { ...entryRespCodes.value, [entry.id]: t.code }"
```

但在 template 中 `.value` 会自动 unwrap，改为：
```html
@click.stop="entryRespCodes = { ...entryRespCodes, [entry.id]: t.code }"
```

这在 template 中是正确的（Vue 自动 unwrap ref）。

- [ ] **Step 3: 新增内嵌详情 CSS**

```css
/* Inline detail */
.entry-card.expanded {
  border-color: var(--primary);
  background: rgba(99,102,241,0.04);
}
.inline-detail {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}
.inline-loading {
  padding: 12px 0;
  font-size: 12px;
  color: var(--muted);
  text-align: center;
}
.inline-section {
  margin-bottom: 14px;
}
.inline-section h5 {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-sub);
  letter-spacing: 0.5px;
  text-transform: uppercase;
  margin-bottom: 8px;
}
.inline-section p {
  font-size: 12px;
  color: var(--text);
  line-height: 1.6;
  margin: 0;
}
.inline-param-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.inline-param-table th, .inline-param-table td {
  padding: 6px 8px;
  text-align: left;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.inline-param-table th {
  font-weight: 600;
  color: var(--text-sub);
  background: var(--surface);
  font-size: 11px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}
.collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
  font-size: 11px;
  color: var(--muted);
  cursor: pointer;
  gap: 4px;
}
.collapse-btn:hover { color: var(--text-sub); }
```

- [ ] **Step 4: 构建确认**

```bash
cd /Users/cw/fty.ai/spider.ai/web
npm run build 2>&1 | tail -20
```

Expected: 无 TypeScript 错误。

- [ ] **Step 5: 浏览器验证**

```bash
go build -a -o /tmp/spider-test ./cmd/spider && /tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

打开 http://localhost:8002，进入知识库，验证：
- 点击条目卡片 → 内嵌展开显示描述/参数/响应
- 再点同一张卡片 → 收起
- 点击多张卡片 → 多张同时展开
- 点击"▲ 收起" → 该卡片收起
- 响应 tab 可点击切换
- "友好/原文" tab 切换正常

- [ ] **Step 6: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat(kb): inline entry detail expansion with multi-open support"
```

---

### Task 5: 修复键盘交互

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: 更新 `onKeydown` 中 Esc 和 Enter 行为**

找到 `onKeydown` 函数，修改如下：

```ts
function onKeydown(e: KeyboardEvent) {
  const tag = (e.target as HTMLElement)?.tagName
  const isInput = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
  if (e.key === '/' && !isInput) {
    e.preventDefault()
    searchInputRef.value?.focus()
    return
  }
  if (e.key === 'Escape') {
    // 有展开的条目：全部收起
    if (expandedEntries.value.size > 0) {
      expandedEntries.value = new Set()
      return
    }
    if (isInput) (e.target as HTMLElement).blur()
    return
  }
  if (isInput) return
  if (!activeDoc.value || !filteredEntries.value.length) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    focusedIdx.value = Math.min(filteredEntries.value.length - 1, focusedIdx.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    focusedIdx.value = Math.max(0, focusedIdx.value - 1)
  } else if (e.key === 'Enter' && focusedIdx.value >= 0) {
    e.preventDefault()
    toggleEntry(filteredEntries.value[focusedIdx.value])
  }
}
```

- [ ] **Step 2: 构建 + 键盘验证**

```bash
go build -a -o /tmp/spider-test ./cmd/spider && /tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

验证：
- 选中文档后，ArrowDown/Up 移动高亮，Enter 展开/收起对应卡片
- Esc 一次收起所有展开卡片

- [ ] **Step 3: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "fix(kb): update keyboard shortcuts for inline entry expansion"
```

---

### Task 6: Playwright 端到端验证

**Files:**
- 参考现有测试目录结构

- [ ] **Step 1: 找到现有 Playwright 测试位置**

```bash
find /Users/cw/fty.ai/spider.ai -name "*.spec.ts" -o -name "playwright.config*" 2>/dev/null | head -10
```

- [ ] **Step 2: 运行现有测试确认无回归**

```bash
cd /Users/cw/fty.ai/spider.ai
# 根据 Step 1 找到的测试命令运行
npx playwright test 2>&1 | tail -30
```

Expected: 现有测试全部通过（或与本次改动无关的失败数不变）。

- [ ] **Step 3: Final build 验证**

```bash
go build -a -o /tmp/spider-final ./cmd/spider && /tmp/spider-final serve --addr :8002 --data-dir ~/.spider/data
```

完整验证清单：
- [ ] 三栏变两栏，无右侧详情面板残留
- [ ] 友好/原文 tab 切换正常，active 样式正确
- [ ] 原文模式：搜索框变灰，raw_content 正确显示
- [ ] 切换文档：视图重置为友好模式，展开状态清空
- [ ] 条目卡片点击展开/收起
- [ ] 多张卡片同时展开
- [ ] "▲ 收起"按钮正常
- [ ] 响应 tab 独立切换（每张卡片互不影响）
- [ ] Esc 收起全部展开卡片
- [ ] ArrowUp/Down + Enter 键盘导航正常

- [ ] **Step 4: Final commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat(kb): knowledge base inline entry detail and raw view complete"
```
