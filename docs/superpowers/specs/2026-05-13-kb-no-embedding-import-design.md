# 知识库无 Embedding 导入设计

## 背景

现有知识库导入（`ingestDocument`）强依赖 Embedding 模型：调用 `rag.Store.Ingest()` 时必须有可用的 embedder，否则返回 503。用户希望在没有配置 Embedding 模型时，也能导入文档，并让 Agent 通过"先看目录、再取全文"的方式使用这些文档。

## 目标

- 导入时可选跳过 Embedding，原文存入数据库
- Agent 可列出分组目录（ID + 标题），按需取全文发给 LLM
- 对 Agent 的工具调用模式友好：同一工具内完成目录浏览和全文获取

## 方案

### 1. 导入侧：`use_embedding` 参数

**`ingestDocument` API**（`internal/api/documents.go`）请求体加 `use_embedding bool`：

```go
var req struct {
    ...
    UseEmbedding *bool `json:"use_embedding"` // nil = true（向后兼容）
}
```

路由：
- `use_embedding` 为 nil 或 true → 走 `rs.Ingest()`（需要 embedder，现有逻辑不变）
- `use_embedding` 为 false → 直接调 `app.DocStore.Save(..., nil, ...)`，跳过 embedder

`use_embedding=false` 时不需要 ragStore，即使未配置 Embedding 模型也可导入成功。

### 2. Agent 侧：`SearchDocsTool` 加 `catalog` 参数

**`SearchDocsTool.InputSchema()`** 新增两个参数：

```json
"catalog": { "type": "boolean", "description": "List document titles in a group without fetching full content. Use with group_id to browse available documents before deciding which to read." },
"group_id": { "type": "integer", "description": "Group to list when catalog=true." }
```

**`SearchDocsTool.Execute()`** 新增分支（优先级最高，在 `doc_ids` 之前判断）：

```
catalog=true + group_id
  → DocStore.ListByGroup(group_id)
  → 返回 [{id, title, source_file}, ...]（不含 content）
```

Agent 两步流程：
1. `SearchDocs(catalog=true, group_id=X)` → 拿到目录
2. `SearchDocs(doc_ids=[id1, id2])` → 拿全文，塞进 context

### 3. 前端（`web/src/`）

**`web/src/api/documents.ts`**

```ts
export interface IngestRequest {
  vendor: string
  content: string
  source_file: string
  chunk_index: number
  group_id?: number | null
  use_embedding?: boolean  // 新增，默认 true
}
```

**`web/src/views/KnowledgeView.vue`**

导入表单加 checkbox：

```
[x] 使用 Embedding（语义搜索，需配置 Embedding 模型）
```

- 默认勾选（`useEmbedding: true`）
- 取消勾选时，`doIngest()` 传 `use_embedding: false`
- 不使用 Embedding 时，即使未配置 Embedding 模型也可导入成功

## 数据流

```
导入（use_embedding=false）
  → ingestDocument API
  → DocStore.Save(embedding=nil)
  → documents 表（embedding IS NULL）

Agent 使用无 Embedding 文档
  → SearchDocs(catalog=true, group_id=X)
      → DocStore.ListByGroup(X) → [{id, title}, ...]
  → SearchDocs(doc_ids=[id1, id2])
      → DocStore.GetByID(id) × N → 全文
  → 全文注入 LLM context
```

## 边界条件

- `catalog=true` 不过滤 embedding 状态，有无 embedding 的文档都会出现在目录中
- `doc_ids` 分支同样不过滤 embedding 状态，可取任意文档全文
- 向量搜索（`SearchByGroups`）仍只检索 `embedding IS NOT NULL` 的文档，行为不变

## 不在范围内

- 自动降级（有 embedder 时自动 embed，无时自动跳过）
- 分组级别的 embedding 配置
- FTS5 关键词搜索（本方案用目录+全文替代，不需要 FTS）
