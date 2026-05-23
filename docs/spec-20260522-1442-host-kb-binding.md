# 主机 KB 绑定

## 1. 背景

本需求让**主机管理**能持久关联 KB scope，使 agent 处理该主机相关任务时优先按绑定 scope 检索知识库。

### 1.1 与现有体系的关系

代码侧已存在 `access_face.knowledge_sources` 字段（`internal/models/host.go:54`），数组形式 `[]KnowledgeSourceRef{Type,ID}`，Type ∈ "group"|"doc"。agent SearchDocs 引导（`tools_docs.go:63`）+ chat（`chat.go:262`）已在用。

`host.knowledge_sources` 字段 + `host_knowledge_sources` 表（`internal/models/host.go:98`、`internal/db/schema.go:266`）存在但**无任何代码读写**——死代码，本期删除。

→ 本期方向：**复用 `access_face.knowledge_sources`**，删 `host.knowledge_sources`，主机管理 UI 编辑 face 层 KB 绑定。同时清理"sentinel hack"（`[{type:"none",id:0}]` 标禁用），引入显式 `kb_mode` 字段。

### 1.2 `@kb` 语法变更

- `@kb`（裸，无参数）：**移除**。
- `@kb:组名`、`@kb:组名/文档`：保留。
- face KB 绑定：face 持久属性，scope 为 group / doc 数组，不支持全局。

## 2. 范围

| 项 | 状态 |
|---|---|
| `access_face.kb_mode` 字段（"none" / "specific"）| 新增 |
| `access_face.knowledge_sources` 数组多选（group + doc 混合）| 沿用 |
| 主机详情页编辑每 face 的 KB 绑定 | 实现 |
| `host.knowledge_sources` 字段 + `host_knowledge_sources` 表 | 删除 |
| sentinel `[{type:"none",id:0}]` | 移除（迁移到 `kb_mode='none'`） |
| `@kb` 裸语法 | 移除（`kb_ref.go` 删无参形态） |
| `@kb:组名` / `@kb:组名/文档` | 保留 |
| `knowledge_groups.description` 列 | 新增（阶段 2） |
| `knowledge_documents.description` 列 | 新增（阶段 2） |
| 新增 agent tool | 不做（复用 SearchDocs） |
| SearchDocs schema/Execute 逻辑 | 不改 |
| SearchDocs / GetHostsTool system prompt | 微调（kb_mode + enriched sources 引导） |
| host group 绑定 | 不做 |
| 绑定历史 / 审计 | 不做 |

## 3. 数据模型

### 3.1 access_face

```go
type KnowledgeSourceRef struct {
    Type string `json:"type"` // "group" | "doc"
    ID   int    `json:"id"`
    // 以下字段仅出现在响应 DTO 中，不入库
}

// 响应专用 DTO，不污染存储 model
type KnowledgeSourceRefEnriched struct {
    Type        string `json:"type"`
    ID          int    `json:"id"`
    Name        string `json:"name,omitempty"`        // type=group
    Description string `json:"description,omitempty"` // group 或 doc（阶段 2 填充）
    Title       string `json:"title,omitempty"`        // type=doc
    GroupID     int    `json:"group_id,omitempty"`     // type=doc
    GroupName   string `json:"group_name,omitempty"`   // type=doc
}

type AccessFace struct {
    // ... 既有字段
    KBMode           string               `json:"kb_mode"`           // "none" | "specific"
    KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"` // 存储用
}
```

语义：

| kb_mode | knowledge_sources | 含义 |
|---|---|---|
| none | 必空 | 该 face 禁用 KB |
| specific | 必须 ≥1 | agent 按数组绑定检索 |

`kb_mode` 默认 `"none"`。

### 3.2 host

删除字段：`Host.KnowledgeSources`。

### 3.3 知识库（阶段 2）

`knowledge_groups` 加列：`description TEXT NOT NULL DEFAULT ''`
`knowledge_documents` 加列：`description TEXT NOT NULL DEFAULT ''`

description 规范化（写入前服务端处理）：trim + 折叠换行/tab 为空格 + 压缩连续空白 + 截 200 字符。纯文本约束由 prompt 保证，服务端不做 strip markdown。

## 4. API

沿用 `/api/v1/...` + kebab-plural 风格。

### 4.1 access face 更新

`PUT /api/v1/hosts/:host_id/access-faces/:id`

**合并语义**：先合并现有值与请求值，再校验最终状态。

| 请求字段 | 行为 |
|---|---|
| 不传 `kb_mode` | 保留原值 |
| 不传 `knowledge_sources` | 保留原值 |
| `kb_mode=none` | 后端自动清空 `knowledge_sources`（无需客户端传空数组） |
| `kb_mode=specific` | 使用请求中的 `knowledge_sources`（或保留原值） |

**最终状态校验**（合并后，两层）：

1. **store 层**（`access_face_store.go`）：格式与语义校验
2. **API 层**（`hosts.go:validateKnowledgeRefs`）：DB 存在性校验（group/doc id 是否存在）

| kb_mode | knowledge_sources | 校验 |
|---|---|---|
| none | 任意（后端已清空） | 通过 |
| specific | 空 | 400 `"kb_mode=specific requires at least one knowledge_source"` |
| specific | ≥1 项 | 每项 type ∈ {"group","doc"}，id 存在；不存在 → 400 `"knowledge group not found"` / `"knowledge document not found"` |
| specific | >10 项 | 400 `"knowledge_sources exceeds limit of 10"` |
| 其他 | — | 400 `"invalid kb_mode"` |
| specific | type 非法 | 400 `"invalid knowledge_source type"` |
| specific | id ≤ 0 | 400 `"invalid knowledge_source id"` |

`POST /api/v1/hosts/:host_id/access-faces`（新增 face）同样执行上述两层校验。

### 4.2 主机响应

`GET /api/v1/hosts`、`GET /api/v1/hosts/:id`、`POST /api/v1/hosts/:host_id/access-faces`、`PUT /api/v1/hosts/:host_id/access-faces/:id` 每 face 响应均使用 `KnowledgeSourceRefEnriched` DTO：

```json
"access_faces": [
  {
    "id": "...",
    "kb_mode": "specific",
    "knowledge_sources": [
      {"type": "group", "id": 3, "name": "运维手册"},
      {"type": "doc",   "id": 17, "title": "nginx 配置说明", "group_id": 3, "group_name": "运维手册"}
    ]
  }
]
```

`description` 字段阶段 2 填充，阶段 1 省略。`kb_mode='none'`：`knowledge_sources` 输出 `[]`。

### 4.3 知识库描述生成（阶段 2）

#### 4.3.1 doc.description

- **解析阶段自动生成**：doc 状态转 `ready` 后，启动**后台 goroutine**，超时 30s，失败记 warning log，不重试，不影响 doc status，description 留空。
- **手动重生**: `POST /api/v1/knowledge-documents/:id/regenerate-description`（同步）
  - 成功：200 + `{"description": "..."}`
  - LLM provider 不可用：503 `"llm provider unavailable"`
  - 其他失败：500
- **Prompt**（≈）：
  > 为知识库文档生成一句话描述。文档标题: `{title}`。条目摘要: `[entry summaries]`。输出一句话（≤50 字）概括本文档主题。纯文本，不含换行与 Markdown。
- **写入**：服务端规范化后 `UPDATE knowledge_documents SET description=?`。
- **覆盖语义**：手动重生总覆盖用户编辑值。
- **用户编辑**: `PUT /api/v1/knowledge-documents/:id` 入参 `description` 字段，规范化同上，超 200 字符 → 400。
- **权限**：admin。

#### 4.3.2 group.description

- **创建时**：默认空串，无自动生成。
- **手动按需生成**: `POST /api/v1/knowledge-groups/:id/regenerate-description`（同步）
  - 成功：200 + `{"description": "..."}`
  - LLM provider 不可用：503
  - 空 group：400 `"group has no documents"`
  - 其他失败：500
- **Prompt**（≈）：
  > 为知识库分组生成一句话描述。分组名: `{name}`。包含文档: `[(title, description)...]`。输出一句话（≤50 字）概括本组涵盖的知识范围。纯文本，不含换行与 Markdown。
- **写入**：规范化后 `UPDATE knowledge_groups SET description=?`。
- **覆盖语义**：同 doc。
- **用户编辑**: `PUT /api/v1/knowledge-groups/:id` 入参 `description` 字段，规则同 doc。
- **权限**：admin。

## 5. 服务端集成

### 5.1 主机响应预取

`GET /api/v1/hosts` 序列化时批量预取 group/doc 元信息（避免 N+1）：

```go
// 收集所有 face.knowledge_sources 中的 group_id / doc_id
// 两次 IN 查 → 两个 map → 序列化时 O(1) 回填
// doc 项：doc.group_id 同步加入 groupIDs，一次 IN 查覆盖
```

store 加：
- `GetGroupsByIDs(ctx, ids []int) ([]Group, error)`
- `GetDocumentsByIDs(ctx, ids []int) ([]Document, error)`

`GET /api/v1/hosts/:id`：face 数量小，直接查。

### 5.2 store 改动清单

| 文件 | 改动 |
|---|---|
| `internal/models/host.go` | `AccessFace` 加 `KBMode string`；删 `Host.KnowledgeSources`；新增 `KnowledgeSourceRefEnriched` DTO；`AddAccessFaceRequest` / `UpdateAccessFaceRequest` 加 `KBMode` |
| `internal/store/access_face_store.go` | INSERT/UPDATE/SELECT SQL 加 `kb_mode`；`UpdateAccessFaceRequest` 加 `KBMode *string`；新增 `normalizeKBMode` / `normalizeKnowledgeSources` / `validateAccessFaceKB` 辅助函数（在 scan 时自动规范化） |
| `internal/knowledge/store.go` | `DeleteGroup` / `DeleteDocument` / `DeleteGroups` 加事务联动（§5.3）；新增 `GetGroupsByIDs` / `GetDocumentsByIDs`；`SearchByQuery` 重命名为 `Search`，原 `Search`（embedding 入参）重命名为 `searchByEmbedding`（私有） |
| `internal/api/chat.go` | sentinel gate 改 `f.KBMode != "none"`（§9 步骤 4）；`allFacesDisableKB` 改为 stub `return false`（保留函数签名，不删调用点） |
| `internal/api/kb_ref.go` | 删 `@kb` 裸形态（无参数匹配），保留 group/doc 形态 |
| `internal/api/hosts.go` | 新增 `hostResponse` / `accessFaceResponse` enriched DTO；`listHosts` / `getHost` / `addAccessFace` / `updateAccessFace` 均返回 enriched 响应；新增 `validateKnowledgeRefs`（DB 存在性校验）、`enrichHosts` / `enrichHost` / `enrichAccessFace` / `buildKnowledgeRefCache` / `makeHostResponse` / `makeAccessFaceResponse` / `enrichKnowledgeSources` 辅助函数；新增 `accessFaceErrorStatus` 将 store 层错误映射到 HTTP 状态码 |
| `internal/agent/tools_docs.go` | `scope_type` enum 移除 `"kb"`，仅保留 `"group"` / `"document"`；`executeSearch` 调用改为 `store.Search`（新签名） |

### 5.3 删除联动下沉到 knowledge store

`DeleteGroup` / `DeleteDocument` / `DeleteGroups` 事务内先清理 face 引用，再删除。

```go
// DeleteGroup(groupID) 事务内：
// 1. SELECT id FROM knowledge_documents WHERE group_id=groupID → docIDs
// 2. SELECT id, knowledge_sources, kb_mode FROM access_faces
//    WHERE knowledge_sources LIKE '%"id":groupID%'
//       OR knowledge_sources LIKE '%"id":docID1%' OR ...  ← 粗筛，覆盖 group + 所有子 doc
// 3. Go 端 json.Unmarshal → 过滤掉 {type:"group",id:groupID}
//    以及所有 {type:"doc",id:X} where X ∈ docIDs
// 4. 若过滤后 sources 为空 且 kb_mode='specific' → kb_mode='none'
// 5. UPDATE access_faces SET knowledge_sources=?, kb_mode=? WHERE id=?
// 6. DELETE FROM knowledge_groups WHERE id=groupID
//    （ON DELETE CASCADE 自动删 docs / entries / sections）
// 步骤 5 与 6 同事务，中途失败回滚

// DeleteDocument(docID) 事务内：
// 1. SELECT id, knowledge_sources, kb_mode FROM access_faces
//    WHERE knowledge_sources LIKE '%"id":docID%'
// 2. Go 端过滤掉 {type:"doc",id:docID}；必要时降级 kb_mode
// 3. UPDATE access_faces ... ; DELETE FROM knowledge_documents WHERE id=docID
```

LIKE 粗筛可能误匹配（如 id:3 匹配 id:30），Go 端精确过滤兜底。

### 5.4 SearchDocs / GetHostsTool system prompt

`tools_docs.go` `SystemPromptSection()` 追加段（不改 schema 与 Execute）：

```
**Face KB Bindings**

Each access face has `kb_mode` and `knowledge_sources`. When kb_mode='specific',
the face exposes one or more KB scopes (group or doc). Each entry includes
`name` (group) or `title` + `group_name` (doc).

When solving tasks for selected hosts, prefer SearchDocs scoped to the bound
group/doc. Multiple sources may bind to different scopes — call SearchDocs
separately per scope as needed.

- type=group → scope_type=group, scope_id=source.id
- type=doc   → scope_type=document, scope_id=source.id
- kb_mode=none → no binding signal
```

`GetHostsTool` system prompt 同步更新，说明返回值含 `kb_mode` + `knowledge_sources`（enriched）。

## 6. UI

### 6.1 主机管理 — face KB 绑定

主机详情 / 编辑页的 face 卡片内增"KB 绑定"区：

- `kb_mode` 二态选择器：`不使用 KB` / `指定 KB`
- `kb_mode=specific` 时展开多选区：
  - 多选 group（来自 `GET /api/v1/knowledge-groups`）
  - 多选 doc（先选 group → 列出该 group 的 doc → 多选）
  - group 与 doc 可混合
  - 已选项展示 name / title
  - 上限：≤ 10 项（超出禁止继续添加，提示"最多绑定 10 个 KB 来源"）
- 切换 `kb_mode=none` → 前端清空 `knowledge_sources`（后端也会自动清，双重保障）
- 校验：`specific` 且 `knowledge_sources` 为空 → 提交按钮禁用

### 6.2 知识库管理（阶段 2）

- 组列表 / 详情页：展示 `description`，含 `自动生成` 按钮 + 编辑框。空 group 时按钮禁用。
- 文档列表 / 详情页：同上。
- regen 返回 503 → 前端提示 "LLM provider 不可用，请检查配置"。
- 状态显示规则：
  - `doc.status != 'ready'` → 灰文案 "解析中"，按钮禁用
  - `doc.status == 'ready'` 且 `description == ''` → 灰文案 "未生成"，按钮可点
  - `doc.status == 'ready'` 且 `description != ''` → 显示描述，按钮可点（覆盖）
  - group 无 status，仅按 description 是否空区分

## 7. 测试要点

- 单元：
  - `kb_mode` 校验（none + 非空 sources 自动清空 / specific + 空 sources 400 / specific + >10 项 400 / 非法 mode 400）
  - description 规范化（trim / 折叠换行 / 压缩空白 / 截断）
  - `kb_ref.go`：`@kb` 裸形态不再匹配；`@kb:组名` / `@kb:组名/文档` 仍匹配
- API：
  - PUT access face：kb_mode 切换 + knowledge_sources 编辑正反例
  - PUT 不传 kb_mode → 保留原值；不传 knowledge_sources → 保留原值
  - PUT kb_mode=none → knowledge_sources 自动清空
  - regenerate-description（group / doc）正例 + 空 group 反例 + provider 不可用反例
  - PUT 编辑 description 正例 + 超长反例
- 联动：
  - 删 group → 引用该组及其下属 doc 的 face.knowledge_sources 全部清理；sources 空且 kb_mode=specific → 自动变 none
  - 删 doc → 引用该 doc 的 face 同上
  - 事务原子性（中途失败回滚）
- 序列化：主机响应每 face 含 `kb_mode` + enriched `knowledge_sources`（name / title / group_name）
- 预取：N 主机 ListHosts 触发 ≤2 次知识库相关查询
- doc 移组：face 绑定该 doc，doc 被 `MoveDocuments` 移到新组 → 响应中 `group_id` / `group_name` 跟随更新
- **KB 注入开关**：face kb_mode=specific → chat 注入 KB；kb_mode=none → 不注入
- **allFacesDisableKB 移除**：SearchDocs 始终注册，不受 face 状态影响
- 迁移：sentinel `[{type:"none",id:0}]` → `kb_mode='none'`、`knowledge_sources=[]`；已有有效 sources → `kb_mode='specific'`；`host_knowledge_sources` 表数据合并到 face 层后删表
- `scope_type` enum：SearchDocs 不再接受 `"kb"`，仅 `"group"` / `"document"` 有效
- 阶段 2：doc 解析后台 goroutine 失败 → doc 仍 ready，description 空；KB 注入开关迁移正确

## 8. 非目标

详见 §2。

## 9. 迁移

`internal/db/schema.go` 既有幂等 ALTER 模式（启动时执行）。追加：

```sql
ALTER TABLE access_faces ADD COLUMN kb_mode TEXT NOT NULL DEFAULT 'none';
ALTER TABLE knowledge_groups    ADD COLUMN description TEXT NOT NULL DEFAULT '';  -- 阶段 2
ALTER TABLE knowledge_documents ADD COLUMN description TEXT NOT NULL DEFAULT '';  -- 阶段 2
```

数据迁移（Go 启动逻辑，幂等）：

1. **sentinel 转换**：Go 解析 `access_faces.knowledge_sources`，含 `{type:"none",id:0}` → `UPDATE access_faces SET kb_mode='none', knowledge_sources='[]' WHERE id=?`

2. **已有绑定升级**：Go 解析，数组含有效 group/doc 项（type ∈ {"group","doc"} 且 id > 0）→ `UPDATE access_faces SET kb_mode='specific' WHERE id=? AND kb_mode='none'`（幂等）

3. **host_knowledge_sources 数据迁移**（`migrateHostKnowledgeSources`）：若 `host_knowledge_sources` 表存在，将其数据合并到对应主机的第一个 face（优先 ssh face）的 `knowledge_sources`，并设 `kb_mode='specific'`；已有 face 绑定的 id 不重复添加。迁移完成后：
   ```sql
   DROP TABLE IF EXISTS host_knowledge_sources;
   ```
   `Host.KnowledgeSources` 从 model 删除（Go 代码层）。

4. **更新 KB 注入 gate**（`internal/api/chat.go`）：
   ```go
   // 旧
   if len(f.KnowledgeSources) == 0 || f.KnowledgeSources[0].Type != "none" {
   // 新
   if f.KBMode != "none" {
   ```

5. **allFacesDisableKB stub**（`internal/api/chat.go`）：函数体改为 `return false`，保留函数签名（不删调用点）。SearchDocs 始终注册。

6. **删 `@kb` 裸形态**（`internal/api/kb_ref.go`）：正则 `@kb(?::...)` 改为要求 `:组名` 必填，无参数形态不再匹配。

注：SQLite ALTER ADD COLUMN 不支持 REFERENCES，FK 由应用层守（§5.3 store 层联动）。

## 10. 实施阶段

| 阶段 | 内容 | 依赖 |
|---|---|---|
| **阶段 1** | kb_mode + knowledge_sources + 删 host 层 + sentinel 迁移 + `@kb` 裸形态移除 + UI + agent prompt | 无 |
| **阶段 2** | description 列 + regen API + 解析链路后台生成 + KB 管理 UI | 阶段 1 完成 |

阶段 1 可独立上线验证，阶段 2 不阻塞主机 KB 绑定核心功能。
