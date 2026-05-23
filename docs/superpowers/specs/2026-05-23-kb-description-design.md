# KB Description — 手动生成与编辑

**日期**: 2026-05-23
**状态**: 设计完成，待实现
**作用域**: `internal/knowledge/` · `internal/api/` · `web/src/views/KnowledgeView.vue`

## 背景

`knowledge_documents` 和 `knowledge_groups` 目前无描述字段。agent 在决定是否调用 `SearchDocs` 时，只能看到 doc/group 名称，缺乏语义摘要。

本期加入 `description` 字段，支持手动触发 LLM 生成或用户直接编辑。不做解析完成后自动生成。

## 范围

| 项 | 状态 |
|----|------|
| DB 加 `description` 列（documents + groups） | 本期 |
| `POST /regenerate-description`（doc + group） | 本期 |
| `PUT` 接口支持编辑 description（doc + group） | 本期 |
| UI：doc 选中时 entries panel 顶部描述区 | 本期 |
| UI：group 选中时右侧 group detail 面板 | 本期 |
| 解析完成后自动生成 | 不做 |

## 数据模型

### DB Migration

```sql
ALTER TABLE knowledge_documents ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE knowledge_groups    ADD COLUMN description TEXT NOT NULL DEFAULT '';
```

### Struct 变更

`internal/knowledge/plugin.go`：

```go
type Document struct {
    // ...existing fields...
    Description string `json:"description"`
}

type Group struct {
    // ...existing fields...
    Description string `json:"description"`
}
```

### 规范化（写入前，服务端执行）

trim → 折叠换行/tab 为空格 → 压缩连续空白 → 截断 200 字符。

## Store 层

`internal/knowledge/store.go` 新增：

```go
func (s *Store) UpdateDocumentDescription(ctx context.Context, docID int, desc string) error
func (s *Store) UpdateGroupDescription(ctx context.Context, groupID int, desc string) error
```

`ListDocuments`、`GetDocument`、`ListGroups`、`GetGroupsByIDs` 的 SELECT 语句加 `description` 列。

## API

### 新增端点

| 方法 | 路径 | 行为 |
|------|------|------|
| `POST` | `/api/v1/knowledge-documents/:id/regenerate-description` | LLM 生成 doc description，写入，返回 `{"description":"..."}` |
| `POST` | `/api/v1/knowledge-groups/:id/regenerate-description` | LLM 生成 group description，写入，返回 `{"description":"..."}` |
| `PUT` | `/api/v1/knowledge-documents/:id` | 接受 `{"description":"..."}` 手动编辑 |
| `PUT` | `/api/v1/knowledge-groups/:id` | 同上（新接口） |

### LLM Prompt

**doc.description**：取该 doc 全量 entry（title + summary），调 LLM：

> 为知识库文档生成一句话描述。文档标题: `{name}`。条目列表: `[(title, summary)...]`。输出一句话（≤50 字）概括本文档主题。纯文本，不含换行与 Markdown。

**group.description**：取该 group 下所有 doc 的 `(name, description)`，调 LLM：

> 为知识库分组生成一句话描述。分组名: `{name}`。包含文档: `[(name, description)...]`。输出一句话（≤50 字）概括本组涵盖的知识范围。纯文本，不含换行与 Markdown。

### 错误码

| 条件 | 状态码 |
|------|--------|
| doc/group 不存在 | 404 |
| `AgentFactory` 为 nil / LLM 不可用 | 503 `"llm provider unavailable"` |
| group 无文档（regenerate-description） | 400 `"group has no documents"` |
| description 超 200 字（PUT 编辑） | 400 `"description too long"` |

### regenerate-description 执行流程

1. 查 doc/group，不存在 → 404
2. 检查 `AgentFactory.LLMClient`，nil → 503
3. 取 entries（doc）或 docs（group），空 group → 400
4. 调 `llmClient.Chat(ctx, req)`，超时由调用方 context 控制（建议 30s）
5. 规范化响应文本
6. 写入 DB
7. 返回 `{"description":"..."}`

## UI

### 视图 1：doc 选中时（entries panel 顶部）

entries panel header 下方、搜索框上方，加 `doc-desc-block`：

```
┌─────────────────────────────────────────┐
│ 来源: xxx · 块: 42 · 2026-05-20         │  ← 已有 entries-meta
├─────────────────────────────────────────┤
│ 文档描述                                 │  ← label
│ [textarea: 描述内容 / 暂无描述占位]      │
│ [✦ 生成]  [保存]              ≤200字    │
├─────────────────────────────────────────┤
│ 🔍 搜索条目...                           │  ← 已有 filter-input
└─────────────────────────────────────────┘
```

### 视图 2：group 选中（无 activeDoc）时

右侧 detail panel 替换空状态，显示 group detail：

```
📁 运维手册   3 篇文档

分组描述
[textarea]
[✦ 生成]  [保存]              ≤200字

─────────────────────────────────────────
文档列表
📄 nginx-ops.md                    ready
   Nginx 运维操作手册，涵盖配置调优...
📄 k8s-troubleshoot.md             ready
   暂无描述
```

文档列表中每条显示 doc.description 预览（单行截断，`暂无描述` 斜体灰色）。

### 交互规则

- textarea 始终可编辑（不需要点击展开）
- "生成" 按钮：POST regenerate-description，loading 期间按钮 disabled + spinner 文案 "生成中..."
- "保存" 按钮：PUT，成功后无需额外反馈（textarea 内容即最新值）
- 生成成功后 textarea 内容替换为返回的 description
- doc 切换时 textarea 内容跟随 activeDoc.description 重置

## 测试要点

- `UpdateDocumentDescription` / `UpdateGroupDescription` 写入 + 读回
- regenerate-description：正例、LLM 不可用 503、空 group 400、doc 不存在 404
- PUT 编辑：正例、超长 400、规范化（换行折叠）
- UI：生成 loading 状态、doc 切换后 textarea 重置
