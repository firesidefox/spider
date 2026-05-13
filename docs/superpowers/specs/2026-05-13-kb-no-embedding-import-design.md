# 知识库无 Embedding 导入设计

## 背景

现有知识库导入（`ingestDocument`）强依赖 Embedding 模型：调用 `rag.Store.Ingest()` 时必须有可用的 embedder，否则返回 503。用户希望在没有配置 Embedding 模型时，也能导入文档并通过关键词搜索使用。

## 目标

- 导入时可选跳过 Embedding，原文存入数据库
- 无 Embedding 的文档通过 SQLite FTS5 全文检索
- Agent 的 `SearchDocsTool` 自动合并两种结果，对 Agent 透明

## 方案：FTS5 虚拟表 + 显式 use_embedding 参数

### 1. Schema 变更（`internal/db/schema.go`）

在 `migrate()` 末尾（`return nil` 前）追加：

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts
USING fts5(title, content, content=documents, content_rowid=id);

CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
  INSERT INTO documents_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
  INSERT INTO documents_fts(documents_fts, rowid, title, content)
    VALUES('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
  INSERT INTO documents_fts(documents_fts, rowid, title, content)
    VALUES('delete', old.id, old.title, old.content);
  INSERT INTO documents_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
END;
```

FTS5 content table 模式：索引指向 `documents` 表，不重复存储原文。触发器保持 FTS 索引与主表同步。

### 2. `rag.Store` 新增 `IngestRaw()`（`internal/rag/store.go`）

```go
func (s *Store) IngestRaw(ctx context.Context, vendor string, tags []string, title, content, sourceFile string, chunkIndex int, groupID *int) error {
    return s.docs.Save(vendor, tags, title, content, nil, sourceFile, chunkIndex, groupID)
}
```

跳过 embedder，`embedding` 存 nil。

### 3. `store.DocumentStore` 新增 `SearchByKeyword()`（`internal/store/document.go`）

```go
func (s *DocumentStore) SearchByKeyword(query string, groupID *int, topK int) ([]*models.Document, error)
```

实现：
1. FTS5 查询：`SELECT rowid FROM documents_fts WHERE documents_fts MATCH ? ORDER BY rank LIMIT ?`
2. 按 rowid 批量取完整文档（`GetByID`）
3. 若 `groupID != nil`，在 FTS 查询中加 `AND rowid IN (SELECT id FROM documents WHERE group_id = ?)`

### 4. `rag.Store` 新增 `SearchByKeyword()`（`internal/rag/store.go`）

委托给 `store.DocumentStore.SearchByKeyword()`，供 `SearchDocsTool` 调用。

### 5. `ingestDocument` API（`internal/api/documents.go`）

请求体加 `use_embedding bool`（默认 true）：

```go
var req struct {
    ...
    UseEmbedding *bool `json:"use_embedding"` // nil = true（向后兼容）
}
```

路由：
- `use_embedding` 为 nil 或 true → 走 `rs.Ingest()`（需要 embedder）
- `use_embedding` 为 false → 走 `rs.IngestRaw()`（无需 embedder）

### 6. `SearchDocsTool.Execute()`（`internal/agent/tools_docs.go`）

搜索路径变更（`doc_ids` 分支不变）：

```
query
  → 向量搜索：SearchByGroup(query, groupID, topK)  [embedding IS NOT NULL]
  → FTS5 搜索：SearchByKeyword(query, groupID, topK)
  → 合并去重（按 doc.ID），返回
```

两路并行或串行均可，结果按 ID 去重后返回给 Agent。Agent 无需感知文档是否有 embedding。

### 7. 前端（`web/src/`）

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
  → rag.Store.IngestRaw()
  → store.DocumentStore.Save(embedding=nil)
  → documents 表（embedding IS NULL）
  → 触发器 → documents_fts 索引

搜索（SearchDocsTool）
  → 向量搜索（embedding IS NOT NULL）→ 结果 A
  → FTS5 搜索（embedding IS NULL）   → 结果 B
  → 合并去重 → 返回 Agent
```

## 边界条件

- 已有文档（无 embedding）不会自动加入 FTS 索引，需手动回填或重新导入。回填 SQL：`INSERT INTO documents_fts(rowid, title, content) SELECT id, title, content FROM documents`
- FTS5 MATCH 语法：使用 SQLite FTS5 默认分词器（unicode61），中文分词效果有限，但基本关键词匹配可用
- `SearchByKeyword` 的 `topK` 与向量搜索的 `topK` 独立，合并后总数可能超过 topK，可在合并后截断

## 不在范围内

- 自动降级（有 embedder 时自动 embed，无时自动跳过）
- 分组级别的 embedding 配置
- 中文分词优化（jieba 等）
