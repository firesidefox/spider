# 知识库：内嵌条目详情 + 原文视图

**日期：** 2026-05-26  
**范围：** `web/src/views/KnowledgeView.vue`（前端仅）  
**Mockup：** `.superpowers/brainstorm/90508-1779797634/content/inline-detail.html`（可直接用浏览器打开）

---

## 目标

1. 条目详情从独立右侧面板改为在条目卡片内内嵌展开（多张可同时展开）。
2. 条目面板顶部新增 `[友好] [原文]` tab，`原文` 模式展示文档 raw_content。

---

## 布局变更

**现在（三栏）：** 侧边栏 | 条目面板 | 详情面板  
**改后（两栏）：** 侧边栏 | 条目面板（flex:1，占满剩余宽度）

删除 `<section class="kb-detail">` 整块。  
删除 `entriesWidth` ref 及其 resize 逻辑（条目面板不再有固定宽度）。  
侧边栏 resize 保留不变。

---

## 状态变更

### 删除

| 删除的 ref | 说明 |
|-----------|------|
| `activeEntryId` | 被 expandedEntries 取代 |
| `activeEntryDetail` | 被 entryDetails 取代 |
| `loadingDetail` | 被 loadingEntries 取代 |
| `activeRespCode` | 移入每张卡片的局部状态 |
| `entriesWidth` | 条目面板不再有固定宽度 |

### 新增

```ts
const expandedEntries = ref(new Set<number>())       // 展开中的条目 id
const entryDetails = ref<Record<number, KnowledgeEntryDetail>>({})  // 已加载的详情缓存
const loadingEntries = ref(new Set<number>())         // 正在加载详情的 id
const entryRespCodes = ref<Record<number, string>>({}) // 每张卡片当前选中的 resp tab
const entriesView = ref<'friendly' | 'raw'>('friendly') // 面板视图模式
```

---

## 友好视图（默认）

### 条目卡片展开逻辑

```
toggleEntry(id):
  if expandedEntries.has(id):
    expandedEntries.delete(id)
    return
  expandedEntries.add(id)
  if not entryDetails[id]:
    loadingEntries.add(id)
    entryDetails[id] = await getEntry(id)
    loadingEntries.delete(id)
    // 设置默认 resp tab
    entryRespCodes[id] = first 2xx tab or first tab
```

- 多张可同时展开，互不影响。
- 详情缓存：同一文档内不重复请求；切换文档时清空缓存（`watch(activeDoc)` 里重置 expandedEntries、entryDetails、loadingEntries、entryRespCodes）。

### 展开后卡片内容（与原详情面板一致）

1. 描述（description）
2. 参数表（parameters）
3. 响应 tab + 响应体（responses）
4. 无上述字段时显示原始 content

### Esc 键行为

收起所有展开的条目（`expandedEntries.clear()`）。原来 Esc 关闭详情面板，现在改为此行为。

### 键盘导航

ArrowUp / ArrowDown 移动 `focusedIdx`，Enter 触发 `toggleEntry`（替代原来的 `selectEntry`）。

---

## 原文视图

tab 切换至 `原文` 时：

- 搜索框和方法过滤器变为 `opacity: 0.4; pointer-events: none`（保留位置，不跳动）。
- 条目列表区域替换为 `<pre class="resp-body">{{ activeDoc.raw_content }}</pre>`（复用现有深色代码块样式）。
- 切换文档时视图重置为 `friendly`（在 `watch(activeDoc)` 中）。

---

## Mockup 对应关系

实现须与 mockup（`.superpowers/brainstorm/90508-1779797634/content/inline-detail.html`）保持一致：

| Mockup 元素 | 实现对应 |
|------------|---------|
| 右列"新方案"整体布局 | 两栏布局，条目面板 flex:1 |
| 顶部 `[友好] [原文]` tab，active 下划线蓝色 | `entriesView` ref + `.view-tab.active` 样式 |
| 搜索框+方法过滤在原文模式下 opacity 变灰 | opacity:0.4 + pointer-events:none |
| 条目卡片点击展开，再点收起，多张可同时展开 | `toggleEntry()` + `expandedEntries` Set |
| 展开区：描述 → 参数表 → 响应 tab + 响应体 | 与原详情面板内容一致 |
| 展开区底部"▲ 收起"按钮 | 点击触发 `toggleEntry(id)` |
| 收起状态：卡片仅显示 method badge + path + 摘要 | 折叠时隐藏 `.inline-detail` |

---

## 不变的部分

- 侧边栏（分组/文档管理）逻辑不动。
- 条目搜索、方法过滤（friendly 模式下）逻辑不动。
- API 层（`getEntry` 等）不动。
- 所有 modal（导入、移动、新建分组）不动。
