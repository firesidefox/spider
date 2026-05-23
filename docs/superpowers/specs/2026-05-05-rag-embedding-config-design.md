# RAG Embedding 配置独立化设计

**状态：** 已实现 — RAG config UI、POST /api/v1/rag-config/validate、embedding provider 独立配置（ProfileView.vue）

## 目标

将知识库 RAG 的 embedding 配置从 LLM 供应商（provider）中剥离，改为系统级独立配置。原因：不是所有 LLM 供应商都提供 embedding API，两者应独立管理。

## 架构

### 数据层

新建 `rag_config` 表（单行，upsert 替换）：

```sql
CREATE TABLE IF NOT EXISTS rag_config (
    type              TEXT NOT NULL DEFAULT 'openai',
    base_url          TEXT NOT NULL DEFAULT '',
    model             TEXT NOT NULL DEFAULT '',
    encrypted_api_key TEXT NOT NULL DEFAULT ''
)
```

- `type`：embedding provider 类型，目前只支持 `openai`（OpenAI 兼容接口）
- `base_url`：API 地址，留空使用 `https://api.openai.com`
- `model`：embedding 模型名，如 `text-embedding-3-small`
- `encrypted_api_key`：加密存储的 API key

`providers` 表的 `embedding_model` 列保留（已迁移，不回滚），但不再使用。

### 后端

**新增 `store.RagConfigStore`**（`internal/store/rag_config_store.go`）

```go
type RagConfig struct {
    Type    string `json:"type"`
    BaseURL string `json:"base_url"`
    Model   string `json:"model"`
    APIKey  string `json:"-"`
}

func (s *RagConfigStore) Get() (*RagConfig, error)
func (s *RagConfigStore) Save(cfg *RagConfig) error
```

- `Get()` 返回当前配置，未配置时返回 `nil, nil`
- `Save()` 加密 API key 后 upsert

**修改 `App` struct**（`internal/mcp/server.go`）

加 `RagConfigStore *store.RagConfigStore` 字段。

**修改 `ragStore()`**（`internal/api/documents.go`）

从 `app.RagConfigStore.Get()` 读配置，不再依赖 active provider。未配置时返回 `fmt.Errorf("RAG embedding not configured")`。

**新增 API**（`internal/api/rag_config.go`）

| 端点 | 方法 | 说明 |
|------|------|------|
| `GET /api/v1/rag-config` | GET | 返回当前配置（API key 脱敏） |
| `PUT /api/v1/rag-config` | PUT | 保存配置（api_key 为空时保留原值） |

响应格式：
```json
{
  "type": "openai",
  "base_url": "https://api.minimaxi.com",
  "model": "text-embedding-3-small",
  "api_key_set": true
}
```

**清理 provider embedding_model**

- `models/provider.go`：移除 `EmbeddingModel` 字段
- `store/provider_store.go`：移除 `SetEmbeddingModel()`，所有 SQL 查询移除 `embedding_model` 列
- `api/providers.go`：移除 `setProviderEmbeddingModel()`
- `api/handler.go`：移除 `/embedding-model` 路由

### 前端

**`KnowledgeView.vue`**：加"Embedding 配置"卡片，字段：
- 类型（固定 openai，只读或 select）
- 请求地址（input，placeholder: 留空使用 OpenAI 默认）
- 模型（input，placeholder: 如 text-embedding-3-small）
- API Key（password input，placeholder: 已设置时显示 ****xxxx）
- 保存按钮

**`ProfileView.vue`**：移除供应商表格中的"Embedding 模型"列。

### `main.go`

初始化 `RagConfigStore` 并注入 `App`：

```go
app.RagConfigStore = store.NewRagConfigStore(database, cm)
```

## 数据流

```
用户在知识库页面配置 Embedding
  → PUT /api/v1/rag-config
  → RagConfigStore.Save()
  → rag_config 表（API key 加密）

导入文档 / @kb 检索
  → ragStore()
  → RagConfigStore.Get()
  → rag.NewEmbedder(type, apiKey, model, baseURL, 0)
  → OpenAI 兼容 /v1/embeddings
```

## 不改动

- `rag.Store`、`rag.Embedder` 接口不变
- `documents` 表结构不变
- LLM 供应商配置流程不变
