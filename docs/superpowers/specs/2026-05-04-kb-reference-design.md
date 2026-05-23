# 运维智能体知识库检索设计

**状态：** 已实现 — SearchDocs 工具、三层检索（sections/entries/fetch/search）、sqlite-vec 向量搜索

## 目标

在智能运维对话中支持两种知识库检索路径：
1. 用户主动用 `@kb` / `@kb:分组名` / `@kb:分组名/文档名称` 显式引用知识库
2. LLM 自主调用 `search_docs` tool 兜底检索

## @ 语法规则

| 语法 | 含义 |
|------|------|
| `@kb` | 全局向量检索，不限分组 |
| `@kb:运维手册` | 按分组名向量检索 |
| `@kb:运维手册/nginx配置说明` | 精确引用指定文档（按文档名称查找，不走向量检索） |

支持一条消息中多个 `@` 引用，可混合三种语法。

## 前端交互流程

```
输入 @
  → 下拉显示分组列表 + "全局检索"选项

选分组（如"运维手册"）
  → 下拉切换为该分组的文档列表 + "全部文档"选项

选"全部文档"  → 插入 @kb:运维手册
选具体文档    → 插入 @kb:运维手册/nginx配置说明
选"全局检索"  → 插入 @kb
```

## 数据流

```
用户输入 "@kb:运维手册/nginx配置说明 这个配置怎么理解"
  ↓
前端：@ 触发两步下拉（分组 → 文档），插入 @kb:分组名/文档名称
  ↓
POST /api/v1/chat/conversations/:id/messages { content: "..." }
  ↓
后端 chatSendMessage 解析 @ 语法：
  @kb              → rag.Store.SearchByGroup(query, nil, topK=3)
  @kb:分组名        → rag.Store.SearchByGroup(query, groupID, topK=3)
  @kb:分组名/文档名  → store.DocumentStore.GetByGroupAndTitle(groupID, title)
  → 展开成结构化块，替换原消息中的 @ 引用
  ↓
Agent.Run() 收到展开后的消息
  → LLM 看到 [知识库: 运维手册/nginx配置说明 · 1条结果] + 内容
  ↓
LLM 也可自主调 search_docs tool（无 @ 时兜底）
```

## 消息展开格式

分组检索（向量）：
```
[知识库: 运维手册 · 3条结果]
---
## nginx 重启方法
systemctl restart nginx ...

## nginx 配置检查
nginx -t ...
---

nginx怎么重启
```

精确文档引用：
```
[知识库: 运维手册/nginx配置说明 · 1条结果]
---
## nginx配置说明
worker_processes auto; ...
---

这个配置怎么理解
```

原始 `@` 引用被替换为标注块，用户原始问题保留在末尾。

## 后端改动

### `internal/api/kb_ref.go`（新建）

- `parseKBRefs(message)` — 正则提取所有 `@kb`、`@kb:xxx`、`@kb:xxx/yyy`
- `expandKBRefs(message, groupLookup, docLookup, search)` — 展开所有引用
- `formatKBBlock(displayName, contents)` — 格式化结果块

### `internal/rag/store.go`

新增 `SearchByGroup(ctx, query string, groupID *int, topK int)` — group_id 过滤的向量检索。

### `internal/store/document_store.go`

新增 `GetByGroupAndTitle(groupID int, title string) (*models.Document, error)` — 精确文档查找。

### `internal/api/chat.go`

`chatSendMessage` 在调用 `Agent.Run()` 前调用 `expandKBRefs`。

### `internal/mcp/server.go`

`App` struct 加 `RagStore *rag.Store` 字段。

### `cmd/spider/main.go`

初始化 `RagStore` 并注入 `App`。

## 前端改动

### 对话输入框组件

- 输入 `@` → 弹出分组列表下拉（调 `GET /api/v1/document-groups`）
- 选分组 → 下拉切换为该分组文档列表（调 `GET /api/v1/documents?group_id=xxx`）
- 选文档 → 插入 `@kb:分组名/文档名称`
- 选"全部文档" → 插入 `@kb:分组名`
- 选"全局检索" → 插入 `@kb`

## 不改动

- `internal/agent/tools_docs.go` — `search_docs` tool 保持不变
- LLM 自主检索路径不变

## 成功标准

1. `@kb xxx` — 全局向量检索，LLM 回答基于结果
2. `@kb:运维手册 xxx` — 分组向量检索，只检索该分组
3. `@kb:运维手册/nginx配置说明 xxx` — 精确引用指定文档
4. 一条消息多个 `@`（可混合三种语法）均被展开
5. 不带 `@` 时，LLM 仍可自主调 `search_docs`
6. 前端两步下拉交互正常，插入正确语法
