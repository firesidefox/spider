# 知识库重设计 Spec

## 目标

重新设计 spider.ai 的知识库系统，支持结构化文档（OpenAPI、Markdown CLI）的高质量导入、索引和检索，使 agent 能以 99%+ 召回率找到正确的 API endpoint 或 CLI 命令。

---

## 1. 数据层级

```
Group（组）
  └── Document（文档）
        ├── Section（章节）← LLM 聚类生成，检索导航用
        └── Entry（条目）← 实际索引单元
```

| 层级 | 说明 | 示例 |
|------|------|------|
| Group | 按厂商/项目分组的顶层容器 | AISG、F5 BIG-IP、Huawei |
| Document | 一个上传的文件 | 监控 API、CLI 手册 |
| Section | LLM 生成的语义章节，用于分层检索 | "查询接口"、"配置管理" |
| Entry | 一个 endpoint 或 CLI 命令 | GET /api/v1/query |

### 1.1 Access Face 绑定

access face 可绑定到任意层级：

| 绑定目标 | 搜索范围 |
|----------|----------|
| Group | 该组下所有文档 |
| Document | 单个文档 |

绑定关系存储在 `access_faces.knowledge_sources`，格式：

```json
[
  {"type": "group",    "id": 1},
  {"type": "document", "id": 7}
]
```

---

## 2. 支持的文档类型

| 类型 | 扩展名 | 解析策略 |
|------|--------|----------|
| OpenAPI | .yaml, .yml, .json | 按 path+method 拆分为 Entry |
| Markdown CLI | .md | LLM 驱动解析，识别语义边界 |
| 其他 | .pdf, .docx 等 | 拒绝，提示转换为 Markdown |

拒绝提示示例：
> "不支持 .pdf 格式。请将文档转换为 Markdown (.md) 后重新上传。"

---

## 3. 导入流程

### 3.1 流程步骤

```
上传文件（支持批量）
    ↓
文件类型检测
    ├── 不支持 → 立即报错，显示转换提示
    └── 支持 → 继续
    ↓
结构解析（Parser）
    ├── OpenAPI → 提取所有 path+method
    └── Markdown → LLM 驱动解析，识别语义边界
    ↓
生成 Entry（parent + child）
    ├── parent = 完整内容（method/path/params/response 或命令全文）
    └── child  = title + summary（一句话）
    ↓
章节聚类（LLM）
    → 对所有 child summary 聚类，生成语义化章节名和章节 summary
    ↓
向量化（OpenAI text-embedding-3-small）
    → 只对 child.summary 生成 embedding
    ↓
写入 DB
    ↓
返回 summary（条目数、章节数、耗时）
```

### 3.2 OpenAPI 解析规则

每个 `path + method` 生成一个 Entry：

```
parent.title   = "GET /api/v1/query_range"
parent.content = method + path + summary + description + parameters + requestBody + responses（完整 JSON）
child.title    = "GET /api/v1/query_range"
child.summary  = operationId 或 summary 字段，若无则用 description 前 100 字
```

### 3.3 Markdown 解析规则

使用 LLM 驱动解析，不预设标题层级：

```
输入：Markdown 全文
输出：[{title, content, summary}] 列表
```

LLM 负责：
- 识别语义边界（一个命令/主题 = 一个 Entry）
- 提取每个 Entry 的 title（命令名或主题）
- 生成 summary（一句话描述）
- content = 该 Entry 的完整原文（含代码块、选项、示例）

适应任意格式：标题层级、粗体分隔、无结构纯文本均可处理。

**分块策略**：文档过长时（> 8000 tokens）先按 `##` 或空行分段，逐段送入 LLM，结果合并。

### 3.4 章节聚类（LLM）

导入完成后，对该文档所有 child.summary 调用 LLM 聚类：

- 输入：所有 entry 的 title + summary 列表
- 输出：章节列表，每章节含 name（中文）、summary（一句话）、entry_ids
- 章节数量：自动决定，通常 3-15 个
- 原始 tag（OpenAPI）作为聚类参考输入，但不直接使用

### 3.5 导入 UI（弹窗）

1. 拖拽或选择多个文件
2. 文件列表 + 每个文件的解析进度条
3. 全部完成后显示 summary：
   - X 个文件成功 / Y 个失败
   - 共 N 个条目、M 个章节
   - 失败文件及原因
4. 关闭按钮

支持批量删除文档。

---

## 4. 检索架构（Hierarchical Retrieval）

### 4.1 三层检索

```
Level 1: 章节 catalog
  → SearchDocs catalog=true, scope={type,id}
  → 返回：[{section_id, name, summary, entry_count}]
  → LLM 选择相关章节

Level 2: 条目 catalog
  → SearchDocs catalog=true, section_id=N
  → 返回：[{entry_id, title, summary}]
  → LLM 选择相关条目

Level 3: 条目全文
  → SearchDocs entry_ids=[...]
  → 返回：完整 parent 内容
```

### 4.2 向量搜索（fallback）

SearchDocs 在以下情况自动建议 LLM 使用 `mode=search`：
- `sections` 返回的 `total_entries ≥ 500`
- LLM 自行判断章节 catalog 无法定位目标

```
SearchDocs mode=search, query="查 CPU 趋势", scope_type="group", scope_id=3
  → 向量搜索所有 child embedding
  → 返回 top-5 匹配条目的 parent 内容
```

### 4.3 搜索范围（scope）

scope 对应 access face 的绑定目标：

```json
{"type": "group",    "id": 1}   // 搜索整个组
{"type": "document", "id": 7}   // 搜索单个文档
```

### 4.4 性能目标

| 路径 | P95 延迟 |
|------|----------|
| catalog 路径 | < 100ms |
| 向量搜索路径 | < 500ms |
| 缓存命中 | < 10ms |

向量化使用 OpenAI text-embedding-3-small，维度 1536。

### 4.5 查询缓存

相同 query + scope 的向量搜索结果缓存 1 小时，key = hash(query + scope)。

---

## 5. 插件接口

知识库作为进程内插件，通过 Go interface 与 spider.ai 核心解耦：

```go
type KnowledgePlugin interface {
    // Group management
    CreateGroup(ctx context.Context, name string) (*Group, error)
    ListGroups(ctx context.Context) ([]Group, error)
    DeleteGroup(ctx context.Context, groupID int) error
    DeleteGroups(ctx context.Context, groupIDs []int) error

    // Document management
    ListDocuments(ctx context.Context, groupID int) ([]Document, error)
    GetDocument(ctx context.Context, docID int) (*Document, error)
    DeleteDocuments(ctx context.Context, docIDs []int) error
    MoveDocuments(ctx context.Context, docIDs []int, targetGroupID int) error

    // 检索
    CatalogSections(ctx context.Context, scope Scope) ([]Section, error)
    CatalogEntries(ctx context.Context, sectionID int) ([]EntrySummary, error)
    FetchEntries(ctx context.Context, entryIDs []int) ([]Entry, error)
    Search(ctx context.Context, query string, scope Scope, topK int, embedder rag.Embedder) ([]Entry, error)

    // 导入
    ImportDocument(ctx context.Context, req ImportRequest) (*ImportResult, error)
    ReindexDocument(ctx context.Context, docID int, req ImportRequest) (*ImportResult, error)
}

type Scope struct {
    Type string // "group" | "document"
    ID   int
}

type Section struct {
    ID         int
    Name       string
    Summary    string
    EntryCount int
}

type EntrySummary struct {
    ID      int
    Title   string
    Summary string
}

type Entry struct {
    ID      int
    Title   string
    Content string // parent 完整内容
}

type ImportRequest struct {
    GroupID  int
    Name     string
    Content  []byte
    Filename string
}

type ImportResult struct {
    DocumentID   int
    EntryCount   int
    SectionCount int
    Sections     []Section
}
```

---

## 6. DB Schema

```sql
-- 组（顶层容器）
CREATE TABLE knowledge_groups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

-- 文档
CREATE TABLE knowledge_documents (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id    INTEGER NOT NULL REFERENCES knowledge_groups(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    doc_type    TEXT NOT NULL CHECK(doc_type IN ('openapi','markdown')),
    raw_content TEXT NOT NULL DEFAULT '',
    filename    TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','indexing','ready','error')),
    error_msg   TEXT NOT NULL DEFAULT '',
    entry_count INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- 章节（LLM 生成）
CREATE TABLE knowledge_sections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL DEFAULT 0
);

-- 条目（parent）
CREATE TABLE knowledge_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    section_id  INTEGER REFERENCES knowledge_sections(id) ON DELETE SET NULL,
    title       TEXT NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL,
    embedding   BLOB,
    position    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_kb_docs_group_id ON knowledge_documents(group_id);
CREATE INDEX idx_kb_sections_doc_id ON knowledge_sections(document_id);
CREATE INDEX idx_kb_entries_doc_id ON knowledge_entries(document_id);
CREATE INDEX idx_kb_entries_section_id ON knowledge_entries(section_id);
```

`access_faces.knowledge_sources` 字段格式：

```json
[{"type":"group","id":1}, {"type":"document","id":7}]
```

---

## 7. SearchDocs 工具接口（agent 侧）

```
SearchDocs 输入参数：

mode         string   "sections" | "entries" | "fetch" | "search"
scope_type   string   "group" | "document"（mode=sections/search 时必填）
scope_id     int      对应 ID（mode=sections/search 时必填）
section_id   int      mode=entries 时必填
entry_ids    []int    mode=fetch 时必填
query        string   mode=search 时必填
```

四种模式：

| mode | 输入 | 输出 | 说明 |
|------|------|------|------|
| `sections` | scope_type + scope_id | [{section_id, name, summary, entry_count}] | Level 1：列出章节 |
| `entries` | section_id | [{entry_id, title, summary}] | Level 2：列出章节内条目 |
| `fetch` | entry_ids | [{title, content}] | Level 3：拉取条目全文 |
| `search` | query + scope_type + scope_id | [{title, content}] | Fallback：向量搜索 |

`sections` 返回结果包含 `total_entries` 字段，SearchDocs 内部据此自动决定是否建议 LLM 使用向量搜索（total_entries ≥ 500 时在返回中附加提示）。

---

## 8. LLM 依赖

知识库插件复用 spider.ai 已有的 provider 配置，不引入新的 LLM 客户端。

| 用途 | 调用时机 | 模型要求 |
|------|----------|----------|
| Markdown 解析 | 导入时，每个分段一次调用 | 支持长上下文，推荐 claude-sonnet-4-6 |
| 章节聚类 | 导入完成后，每个文档一次调用 | 同上 |

provider 通过 `KnowledgePlugin` 初始化时注入，插件本身不持有 provider 配置。

---

## 9. 不在范围内

- URL 自动拉取 OpenAPI spec
- PDF/Word 支持
- 跨知识库搜索
- 文档版本管理
- 自动重新索引
