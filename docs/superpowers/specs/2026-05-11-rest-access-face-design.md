# REST API 操作面优化 — 设计文档

**日期：** 2026-05-11  
**状态：** 待实现

---

## 背景

Agent 通过 `CallAPI` 工具调用设备 REST API。工具已支持 `face_id` 自动注入认证头、拼接 base URL。但两个问题未解决：

1. 前端表单字段不随认证方式动态显示，`apikey` 认证缺少 `header_name` 字段
2. Agent 无法查询 face 绑定的 API 文档（知识库未接通）

---

## 数据链路

```
Host
└── AccessFace (type=restapi)
    ├── base_url          — Agent 调用时自动拼接相对路径
    ├── rest_auth_type    — bearer / basic / apikey / none
    ├── rest_username     — basic 认证用
    ├── header_name       — apikey 认证时的 header 名称
    ├── credential        — 加密存储
    └── knowledge_sources — API 文档来源
        ├── {type:"group", id:N}  — 文档组，向量搜索
        ├── {type:"doc",   id:N}  — 具体文档，返回全文
        └── []                    — 全局搜索（无过滤）
```

---

## Agent 工作流

```
1. GetDeviceInfo(host_id)
   → 返回 host，含 access_faces
   → face.base_url + face.knowledge_sources 可见

2. SearchDocs(query, group_id | doc_id | 无参数)
   → 查询 face 绑定的 API 文档
   → Agent 从 face.knowledge_sources 自行决定传哪个参数

3. CallAPI(face_id, url="/api/v1/...", method, intent)
   → 框架自动注入认证头
   → base_url + 相对路径自动拼接
```

Agent 从 `GetDeviceInfo` 返回数据中直接读取 `knowledge_sources`，无需额外 system prompt 规则。

---

## 改动范围

### 后端

#### 1. `SearchDocs` 工具扩展

`InputSchema` 新增两个可选参数：

```json
"group_id": {
  "type": "integer",
  "description": "Search within this document group only. Get from face.knowledge_sources where type=group."
},
"doc_id": {
  "type": "integer",
  "description": "Fetch full content of a specific document by ID. Get from face.knowledge_sources where type=doc."
}
```

`Execute` 优先级：
1. 有 `doc_id` → `DocumentStore.GetByID(doc_id)`，返回全文，不做向量搜索
2. 有 `group_id` → `rag.Store.SearchByGroup(query, &group_id, topK)`
3. 都没有 → `rag.Store.SearchByGroup(query, nil, topK)`（全局）

#### 2. `DocumentStore.GetByID`

新增方法，按主键取单条文档。

```go
func (s *DocumentStore) GetByID(id int) (*models.Document, error)
```

#### 3. `rag.Store.GetDocByID`

新增方法，包装 `DocumentStore.GetByID`，供 `SearchDocsTool` 调用。

```go
func (s *Store) GetDocByID(ctx context.Context, id int) (*models.Document, error)
```

---

### 前端

#### REST API face 表单字段动态显示

按 `rest_auth_type` 显示字段：

| auth_type | 显示字段 |
|-----------|---------|
| `none`    | 无额外字段 |
| `bearer`  | 凭据（token） |
| `basic`   | 用户名 + 凭据（密码） |
| `apikey`  | Header Name + 凭据（key 值） |

当前问题：
- `rest_username` 对所有类型都显示
- `header_name` 字段完全缺失
- `credential` textarea 无条件显示

修复：用 `v-if` 按 `faceForm.rest_auth_type` 控制每个字段的显示。

#### 知识库绑定 UI 扩展

当前：face 表单只支持选文档组（checkbox dropdown）。

扩展为三种模式，用 radio 切换：

```
● 全局（不过滤）
● 文档组  [下拉多选]
● 具体文档 [下拉多选]
```

- 全局：`knowledge_sources = []`
- 文档组：`[{type:"group", id}, ...]`
- 具体文档：`[{type:"doc", id}, ...]`

具体文档下拉需要加载 `GET /api/documents`（已有接口）。

---

## 不在范围内

- REST face 连通性测试（ping）
- `AccessFace` 重命名
- `CallAPI` 自动预加载文档（由 Agent 主动查询，不由框架注入）
