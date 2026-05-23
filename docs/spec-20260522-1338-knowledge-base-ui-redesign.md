# 知识库 UI 重设计 Spec

## 目标

基于现有知识库后端架构，重新设计前端 UI，提升可用性和交互体验。

---

## 1. 布局架构

### 1.1 三栏布局

```
┌─────────────┬──────────────────┬────────────────────┐
│  左侧边栏    │   中间 Entry 面板  │   右侧详情面板      │
│  280px      │   400px          │   flex-1           │
│  可拖拽调整  │   可拖拽调整      │   自适应           │
│  200-400px  │   300-600px      │                    │
└─────────────┴──────────────────┴────────────────────┘
```

**参考 mockup**: `kb-mockup-both-resizable.png`

### 1.2 左侧边栏（Group/Document 导航）

**结构**:
```
┌─────────────────────────┐
│ 知识库                   │
│ [选择] [+ 分组] [+ 导入] │
├─────────────────────────┤
│ 🔍 搜索...         /     │  ← 快捷键提示
├─────────────────────────┤
│ ▼ AISG              3   │  ← Group（可折叠）
│   📄 aisg-api.yml  ●ready│  ← Document + 状态
│   📄 test-doc.md   ●ready│
│   📄 test-cli.md   ●error│
│                         │
│ ▶ F5 BIG-IP         1   │
│ ▶ Huawei            5   │
│                         │
│                    280px│  ← 宽度指示器
└─────────────────────────┘
```

**功能**:
- 2层树形导航：Group > Document
- Group 可折叠/展开（chevron 图标）
- Document 状态标签：
  - `ready` 绿色：索引完成
  - `error` 红色：索引失败
  - `indexing` 黄色：索引中
- 右侧拖拽手柄，范围 200-400px
- 宽度保存到 localStorage

**参考 mockup**: `kb-mockup-both-resizable.png`

### 1.3 中间 Entry 面板

**顶部搜索/过滤区**:
```
┌──────────────────────────────────────┐
│ 🔍 搜索 API 路径或描述...             │
│ [GET] [POST] [PUT] [DELETE]          │  ← HTTP 方法过滤器
│                               400px   │  ← 宽度指示器
└──────────────────────────────────────┘
```

**Entry 列表**:
```
┌──────────────────────────────────────┐
│ 来源: aisg-api.yml 块: 4  05/21 21:36│
│                                      │
│ ▼ All Entries              4 条目    │
│                                      │
│ ┌────────────────────────────────┐  │
│ │ GET  /api/v1/query         📋  │  │  ← 悬停显示复制按钮
│ │ 即时查询 - 对 Prometheus...    │  │
│ └────────────────────────────────┘  │
│ ┌────────────────────────────────┐  │
│ │ GET  /api/v1/query_range   📋  │  │  ← 选中状态（蓝色边框）
│ │ 范围查询 - 在时间范围内...     │  │
│ └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

**功能**:
- 搜索框：实时过滤 Entry（路径 + 描述）
- HTTP 方法过滤器：
  - GET 绿色、POST 蓝色、PUT 橙色、DELETE 红色
  - 点击切换激活/禁用
  - 支持多选
- Entry 卡片：
  - 左侧 HTTP 方法标签（颜色编码）
  - 标题 + 摘要
  - 悬停显示复制按钮
  - 选中状态蓝色边框
- 右侧拖拽手柄，范围 300-600px

**参考 mockup**: `kb-mockup-search-filter.png`, `kb-mockup-keyboard-copy.png`

### 1.4 右侧详情面板

**顶部**:
```
┌──────────────────────────────────────┐
│ [← 返回 Esc]  GET  /api/v1/query_range│
└──────────────────────────────────────┘
```

**内容区块**:
```
┌──────────────────────────────────────┐
│ 描述                                  │
│ 在时间范围内执行 PromQL 查询...       │
└──────────────────────────────────────┘

┌──────────────────────────────────────┐
│ 参数                                  │
│ ┌────────┬────────┬──────────────┐  │
│ │ 名称   │ 类型   │ 说明         │  │
│ ├────────┼────────┼──────────────┤  │
│ │ query* │ string │ PromQL 表达式│  │
│ │ start* │ string │ 开始时间戳   │  │
│ └────────┴────────┴──────────────┘  │
└──────────────────────────────────────┘

┌──────────────────────────────────────┐
│ 响应示例                              │
│ [✓ 200 成功] [✗ 400 参数错误] ...    │  ← 多 Tab
│ ┌────────────────────────────────┐  │
│ │ {                              │  │
│ │   "status": "success",         │  │
│ │   "data": { ... }              │  │
│ │ }                              │  │
│ └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

**功能**:
- 返回按钮（带 Esc 快捷键提示）
- HTTP 方法 + 路径标题
- 描述区块
- 参数表格（必填参数标 `*`）
- 响应示例多 Tab：
  - 成功状态带 ✓ 绿色图标
  - 错误状态带 ✗ 红色图标
  - 切换查看不同状态码

**参考 mockup**: `kb-mockup-entry-detail.png`, `kb-mockup-response-tabs.png`

---

## 2. 交互流程

### 2.1 导航流程

```
左侧选中 Group
    ↓
中间显示该 Group 下所有 Document 的 Entry 列表
    ↓
点击 Entry
    ↓
右侧显示详情面板
    ↓
点击返回 / 按 Esc
    ↓
右侧详情面板关闭
```

### 2.2 搜索/过滤流程

```
输入搜索关键词
    ↓
实时过滤 Entry 列表（路径 + 描述匹配）
    ↓
点击 HTTP 方法过滤器
    ↓
只显示选中方法的 Entry
```

### 2.3 键盘导航

| 快捷键 | 功能 |
|--------|------|
| `/` | 聚焦搜索框 |
| `↑` `↓` | 切换 Entry（蓝色光晕边框） |
| `Enter` | 打开详情 |
| `Esc` | 关闭详情 |

**参考 mockup**: `kb-mockup-keyboard-copy.png`

---

## 3. 状态管理

### 3.1 URL 路由

```
/knowledge                          # 默认视图（无选中）
/knowledge/group/:groupId           # 选中 Group
/knowledge/doc/:docId               # 选中 Document
/knowledge/doc/:docId/entry/:entryId # 选中 Entry（显示详情）
```

### 3.2 本地存储

```typescript
localStorage.setItem('kb_sidebar_width', '280')
localStorage.setItem('kb_entries_width', '400')
localStorage.setItem('kb_method_filters', '["GET","POST"]')
```

---

## 4. 数据接口

### 4.1 获取 Group 列表

```
GET /api/knowledge/groups
Response: [
  {
    id: 1,
    name: "AISG",
    doc_count: 3
  }
]
```

### 4.2 获取 Document 列表

```
GET /api/knowledge/groups/:groupId/documents
Response: [
  {
    id: 7,
    name: "aisg-monitor-api.yml",
    status: "ready",
    entry_count: 4,
    updated_at: "2026-05-21T21:36:00Z"
  }
]
```

### 4.3 获取 Entry 列表

```
GET /api/knowledge/documents/:docId/entries
Query params:
  - search: string (可选)
  - methods: string[] (可选，如 ["GET","POST"])

Response: [
  {
    id: 42,
    method: "GET",
    path: "/api/v1/query_range",
    summary: "范围查询 - 在时间范围内执行 PromQL 查询"
  }
]
```

### 4.4 获取 Entry 详情

```
GET /api/knowledge/entries/:entryId
Response: {
  id: 42,
  method: "GET",
  path: "/api/v1/query_range",
  description: "在时间范围内执行 PromQL 查询...",
  parameters: [
    {
      name: "query",
      type: "string",
      required: true,
      description: "PromQL 表达式"
    }
  ],
  responses: {
    "200": {
      description: "成功",
      example: { ... }
    },
    "400": {
      description: "参数错误",
      example: { ... }
    }
  }
}
```

---

## 5. 主题支持

### 5.1 明亮主题（默认）

- 背景：白色 `#ffffff`
- 边框：浅灰 `#e5e5e5`
- 文字：深灰 `#111827`
- 代码块：深色背景 `#1e293b`

### 5.2 暗色主题

- 背景：深灰 `#1e293b`
- 边框：中灰 `#334155`
- 文字：浅灰 `#e2e8f0`
- 代码块：更深背景 `#0f172a`

主题切换通过全局 CSS 变量实现：

```css
:root {
  --kb-bg: #ffffff;
  --kb-border: #e5e5e5;
  --kb-text: #111827;
}

[data-theme="dark"] {
  --kb-bg: #1e293b;
  --kb-border: #334155;
  --kb-text: #e2e8f0;
}
```

---

## 6. 实现优先级

### P0（核心功能）
- [x] 三栏布局
- [ ] 左侧 Group/Document 导航
- [ ] 中间 Entry 列表
- [ ] 右侧详情面板
- [ ] 基础路由

### P1（增强体验）
- [ ] 双面板可拖拽调整
- [ ] 搜索/过滤功能
- [ ] HTTP 方法颜色标签
- [ ] 键盘导航
- [ ] 快捷复制按钮

### P2（高级功能）
- [ ] 响应示例多 Tab
- [ ] 暗色主题支持
- [ ] 宽度持久化

---

## 7. 技术栈

- **框架**: Vue 3 + TypeScript
- **路由**: Vue Router
- **状态**: Pinia
- **样式**: Tailwind CSS
- **图标**: Heroicons

---

## 8. 验证标准

### 8.1 功能验证

- [ ] 左侧导航可折叠/展开
- [ ] 点击 Document 显示 Entry 列表
- [ ] 点击 Entry 显示详情
- [ ] 搜索框实时过滤
- [ ] HTTP 方法过滤器工作
- [ ] 键盘导航响应
- [ ] 复制按钮复制路径

### 8.2 性能验证

- [ ] Entry 列表渲染 < 100ms（100 条）
- [ ] 搜索过滤响应 < 50ms
- [ ] 详情面板打开 < 200ms

### 8.3 兼容性验证

- [ ] Chrome 最新版
- [ ] Firefox 最新版
- [ ] Safari 最新版
- [ ] 1920x1080 分辨率
- [ ] 1366x768 分辨率

---

## 9. Mockup 参考

| 文件 | 说明 |
|------|------|
| `kb-mockup-both-resizable.png` | 双面板可拖拽 |
| `kb-mockup-entry-detail.png` | Entry 详情面板 |
| `kb-mockup-search-filter.png` | 搜索/过滤功能 |
| `kb-mockup-response-tabs.png` | 响应示例多 Tab |
| `kb-mockup-keyboard-copy.png` | 键盘导航 + 复制按钮 |

---

## 10. 后续优化

- 面包屑导航（Document > Section > Entry）
- 参数表格展开/折叠（嵌套参数）
- Entry 右键菜单（复制 curl 命令）
- "在原文档中定位"按钮
- "试一试"API 测试工具
