# spec-20260503-chat-layout-redesign

## 概述

重构 Chat 页面布局：将会话列表从 dropdown 改为可折叠左侧边栏，聊天区与目标面板之间增加拖拽调整宽度功能。

## 当前状态

- 会话列表：hamburger 触发的 absolute dropdown，max-height 300px
- 布局：`.chat-area`(flex:7) + `.target-side`(flex:3) 水平排列
- 新建按钮：在 chat header 内
- 无拖拽调整功能
- 全 custom CSS + CSS variables，无第三方 UI 库

## 设计

### 左侧边栏

**展开态**（默认 240px 宽）：
- header：toggle 按钮 + 新建会话按钮，水平排列
- body：扁平会话列表，可滚动，点击切换会话
- 会话项：单击选中，双击重命名（保留现有行为）
- CSS transition 动画过渡展开/收起

**收起态**（width = 0，完全隐藏）：
- 侧边栏 DOM 保留，CSS 控制 width + overflow:hidden
- toggle + 新建按钮迁移到 chat header 左侧
- chat header 变为：toggle | 新建 | 会话标题 | model 显示

**状态控制**：
- `sidebarOpen: Ref<boolean>`，默认 true
- localStorage 持久化用户偏好

### 拖拽调整宽度

聊天区与目标面板之间增加 drag handle：
- 5px 宽竖条，hover 时高亮（primary color）
- cursor: col-resize
- mousedown → mousemove 更新目标面板 flex-basis
- 目标面板 min-width: 200px，max-width: 50vw
- 聊天区 min-width: 300px
- 拖拽结束 mouseup 清理事件监听
- 拖拽过程中 `user-select: none` 防止文字选中

### Chat Header 变化

**侧边栏展开时**：
- 移除原 hamburger toggle（已在侧边栏）
- 移除原 + 新建按钮（已在侧边栏）
- 保留：会话标题（可编辑）+ model 显示

**侧边栏收起时**：
- 左侧增加：toggle 按钮 + 新建按钮
- 保留：会话标题（可编辑）+ model 显示

### DOM 结构

```
.chat-page (flex row)
├── .sidebar (width: 240px | 0, transition)
│   ├── .sidebar-header (toggle + new btn)
│   └── .sidebar-body (conv list, overflow-y auto)
├── .chat-main (flex: 1, flex column)
│   ├── .chat-header (conditional toggle+new | title + model)
│   ├── .chat-messages
│   └── .chat-input
├── .drag-handle (5px, cursor col-resize)
└── .target-side (flex-basis: 280px, resizable)
```

### 实现方式

- 纯 CSS + mousedown 手写拖拽，零依赖
- 侧边栏折叠用 CSS transition（width + opacity）
- 所有新样式用项目现有 CSS variables
- 不引入第三方库

### 不做的事

- 不做会话分组/文件夹
- 不做侧边栏搜索
- 不做响应式/移动端适配
- 不改 TargetPanel 内部结构
