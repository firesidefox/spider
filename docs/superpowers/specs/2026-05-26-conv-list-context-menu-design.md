---
title: 对话列表项上下文菜单
date: 2026-05-26
status: approved
---

# 对话列表项上下文菜单

## 目标

移除侧边栏对话列表项右侧的 × 删除按钮，替换为 ⋯「更多」图标，点击弹出菜单，支持重命名、批量管理、删除。

## 范围

仅修改 `web/src/views/ChatView.vue`，不涉及后端。

---

## 正常模式

### 列表项结构

```
[ 对话标题（截断）]  [ ⋯ ]
```

- ⋯ 图标默认隐藏，hover 列表项时显示
- 点击 ⋯ 弹出下拉菜单，点击外部关闭
- 活跃对话（active）始终显示 ⋯

### 下拉菜单项

| 项 | 图标 | 行为 |
|----|------|------|
| 重命名 | ✏ | 触发 `startEditConvTitle(id, title)`（复用现有逻辑） |
| 批量管理 | ☑ | 进入批量模式 |
| （分隔线） | — | — |
| 删除 | ✕ | 触发 `handleDeleteConversation(id)`（复用现有逻辑） |

菜单样式：贴近 ⋯ 图标右对齐，`z-index` 覆盖其他元素，点击外部或 Escape 关闭。

---

## 批量管理模式

### 进入/退出

- 点击菜单「批量管理」进入
- 点击「取消」退出，清空已选集合

### 侧边栏 header 变化

正常模式 header（对话 tab + 新建按钮）替换为：

```
[ 批量管理 ]  [ 全选 ]  [ 取消 ]
```

- 「全选」：选中所有对话；若已全选则取消全选（toggle）
- 「取消」：退出批量模式

### 列表项变化

每项左侧出现 checkbox，点击整行切换选中状态（不触发 `selectConversation`）。

### 底部操作栏

侧边栏底部固定：

```
已选 N        [ 删除选中 ]
```

- N = 0 时「删除选中」禁用
- 点击「删除选中」：批量调用 `handleDeleteConversation`，完成后退出批量模式

---

## 状态

新增三个 ref（在现有 state 区域添加）：

```ts
const menuOpenConvId = ref<string | null>(null)   // 当前展开菜单的对话 id
const batchMode = ref(false)                       // 是否处于批量管理模式
const selectedConvIds = ref<Set<string>>(new Set()) // 批量模式已选 id
```

---

## 样式规范（参照 mockup）

- ⋯ 图标：`color: var(--muted)`，hover `color: var(--text)`，`font-size: 16px`
- 下拉菜单：`background: var(--panel)`，`border: 1px solid var(--border)`，`border-radius: 6px`，`min-width: 140px`
- 菜单项：`padding: 6px 14px`，hover `background: var(--row-hover)`
- 删除项：`color: var(--red)`
- 批量模式 header：与正常 header 等高，保持 sidebar 宽度不变
- 底部操作栏：`border-top: 1px solid var(--border)`，`padding: 8px 10px`
- Checkbox：`accent-color: var(--primary)`

---

## 不在范围内

- 批量重命名
- 拖拽排序
- 知识库 / 其他列表的类似改造
