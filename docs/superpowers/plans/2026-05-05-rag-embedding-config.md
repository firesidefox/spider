# RAG Embedding 配置独立化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 RAG embedding 配置从 LLM 供应商中剥离，改为独立的 `rag_config` 表，前端在知识库页面单独配置。

**Architecture:** 新建 `rag_config` 单行表存储 embedding 配置（type/base_url/model/encrypted_api_key）；新建 `RagConfigStore`；`ragStore()` 改为从 `RagConfigStore` 读取；前端在 `KnowledgeView.vue` 加配置卡片，从 `ProfileView.vue` 移除 embedding 列。

**Tech Stack:** Go 1.22, SQLite (database/sql), Vue 3 + TypeScript

---

## File Map

| 操作 | 文件 | 说明 |
|------|------|------|
| Create | `internal/store/rag_config_store.go` | RagConfig struct + RagConfigStore |
| Modify | `internal/db/schema.go` | 新增 rag_config 表 migration |
| Modify | `internal/mcp/server.go` | App struct 加 RagConfigStore 字段 |
| Modify | `cmd/spider/main.go` | 初始化 RagConfigStore |
| Create | `internal/api/rag_config.go` | GET/PUT /api/v1/rag-config 处理函数 |
| Modify | `internal/api/documents.go` | ragStore() 改用 RagConfigStore |
| Modify | `internal/api/handler.go` | 注册 rag-config 路由，移除 embedding-model 路由 |
| Modify | `internal/models/provider.go` | 移除 EmbeddingModel 字段 |
| Modify | `internal/store/provider_store.go` | 移除 SetEmbeddingModel()，SQL 移除 embedding_model 列 |
| Modify | `internal/api/providers.go` | 移除 setProviderEmbeddingModel() |
| Modify | `web/src/views/KnowledgeView.vue` | 加 Embedding 配置卡片 |
| Modify | `web/src/views/ProfileView.vue` | 移除 Embedding 模型列 |

---

### Task 1: DB schema — 新增 rag_config 表

**Files:**
- Modify: `internal/db/schema.go:210`

- [ ] **Step 1: 在 schema.go 的 migrate() 末尾加建表语句**

找到文件末尾的 `db.Exec("ALTER TABLE providers ADD COLUMN embedding_model TEXT NOT NULL DEFAULT ''")`，在其后、`return nil` 之前插入：

```go
db.Exec(`CREATE TABLE IF NOT EXISTS rag_config (
    type              TEXT NOT NULL DEFAULT 'openai',
    base_url          TEXT NOT NULL DEFAULT '',
    model             TEXT NOT NULL DEFAULT '',
    encrypted_api_key TEXT NOT NULL DEFAULT ''
)`)
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出（编译通过）

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat: add rag_config table migration"
```

---

### Task 2: RagConfigStore

**Files:**
- Create: `internal/store/rag_config_store.go`

- [ ] **Step 1: 创建文件**

```go
package store

import (
	"database/sql"
	"fmt"

	"github.com/spiderai/spider/internal/crypto"
)

// RagConfig holds the embedding configuration for the RAG knowledge base.
type RagConfig struct {
	Type    string `json:"type"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"-"` // never serialized
}

// RagConfigStore manages the single-row rag_config table.
type RagConfigStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

func NewRagConfigStore(db *sql.DB, cm *crypto.Manager) *RagConfigStore {
	return &RagConfigStore{db: db, crypto: cm}
}

// Get returns the current RAG config, or nil if not yet configured.
func (s *RagConfigStore) Get() (*RagConfig, error) {
	row := s.db.QueryRow(
		`SELECT type, base_url, model, encrypted_api_key FROM rag_config LIMIT 1`,
	)
	var c RagConfig
	var encKey string
	if err := row.Scan(&c.Type, &c.BaseURL, &c.Model, &encKey); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("scan rag_config: %w", err)
	}
	if encKey != "" {
		var err error
		c.APIKey, err = s.crypto.Decrypt(encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt rag api key: %w", err)
		}
	}
	return &c, nil
}

// Save upserts the RAG config. If APIKey is empty, the existing key is preserved.
func (s *RagConfigStore) Save(cfg *RagConfig) error {
	// Preserve existing key if caller sent empty string
	if cfg.APIKey == "" {
		existing, err := s.Get()
		if err != nil {
			return err
		}
		if existing != nil {
			cfg.APIKey = existing.APIKey
		}
	}
	encKey := ""
	if cfg.APIKey != "" {
		var err error
		encKey, err = s.crypto.Encrypt(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt rag api key: %w", err)
		}
	}
	if _, err := s.db.Exec(`DELETE FROM rag_config`); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO rag_config (type, base_url, model, encrypted_api_key) VALUES (?, ?, ?, ?)`,
		cfg.Type, cfg.BaseURL, cfg.Model, encKey,
	)
	return err
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add internal/store/rag_config_store.go
git commit -m "feat: add RagConfigStore"
```

---

### Task 3: 注入 RagConfigStore 到 App

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: 在 App struct 加字段**

在 `internal/mcp/server.go` 的 `App` struct 中，在 `ProviderStore` 行后加：

```go
RagConfigStore *store.RagConfigStore
```

- [ ] **Step 2: 在 main.go 初始化**

在 `cmd/spider/main.go` 中，找到：
```go
app.ProviderStore = ps
```
在其后加：
```go
app.RagConfigStore = store.NewRagConfigStore(database, cm)
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat: inject RagConfigStore into App"
```

---

### Task 4: API 处理函数 rag_config.go

**Files:**
- Create: `internal/api/rag_config.go`

- [ ] **Step 1: 创建文件**

```go
package api

import (
	"encoding/json"
	"net/http"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/store"
)

type ragConfigResponse struct {
	Type      string `json:"type"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeySet bool   `json:"api_key_set"`
}

func getRagConfig(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	cfg, err := app.RagConfigStore.Get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusOK, ragConfigResponse{Type: "openai"})
		return
	}
	writeJSON(w, http.StatusOK, ragConfigResponse{
		Type:      cfg.Type,
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		APIKeySet: cfg.APIKey != "",
	})
}

func putRagConfig(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Type == "" {
		req.Type = "openai"
	}
	cfg := &store.RagConfig{
		Type:    req.Type,
		BaseURL: req.BaseURL,
		Model:   req.Model,
		APIKey:  req.APIKey,
	}
	if err := app.RagConfigStore.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	getRagConfig(app, w, r)
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add internal/api/rag_config.go
git commit -m "feat: add GET/PUT /api/v1/rag-config handlers"
```

---

### Task 5: 注册路由 + 移除 embedding-model 路由

**Files:**
- Modify: `internal/api/handler.go`

- [ ] **Step 1: 注册 /api/v1/rag-config 路由**

在 `handler.go` 中找到 `/api/v1/providers` 路由注册块附近，加：

```go
mux.HandleFunc("/api/v1/rag-config", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        getRagConfig(app, w, r)
    case http.MethodPut:
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            putRagConfig(app, w, r)
        })).ServeHTTP(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
```

- [ ] **Step 2: 移除 embedding-model 路由**

在 `handler.go` 中找到并删除：

```go
case action == "embedding-model" && r.Method == http.MethodPut:
    operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        setProviderEmbeddingModel(app, w, r, id)
    })).ServeHTTP(w, r)
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出（如果 `setProviderEmbeddingModel` 还存在则会有 unused 警告，下一 task 清理）

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler.go
git commit -m "feat: register rag-config routes, remove embedding-model route"
```

---

### Task 6: 修改 ragStore() 使用 RagConfigStore

**Files:**
- Modify: `internal/api/documents.go:13-37`

- [ ] **Step 1: 替换 ragStore() 函数体**

将 `internal/api/documents.go` 中的 `ragStore()` 函数替换为：

```go
func ragStore(app *mcppkg.App) (*rag.Store, error) {
	cfg, err := app.RagConfigStore.Get()
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.Model == "" {
		return nil, fmt.Errorf("RAG embedding 未配置，请在知识库页面设置 Embedding 配置")
	}
	embedder, err := rag.NewEmbedder(cfg.Type, cfg.APIKey, cfg.Model, cfg.BaseURL, 0)
	if err != nil {
		return nil, err
	}
	return rag.NewStore(app.DB, app.DocStore, embedder), nil
}
```

同时移除不再需要的 import（如果 `ProviderStore` 相关 import 变成 unused）。

- [ ] **Step 2: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add internal/api/documents.go
git commit -m "feat: ragStore() now reads from RagConfigStore"
```

---

### Task 7: 清理 provider embedding_model

**Files:**
- Modify: `internal/models/provider.go`
- Modify: `internal/store/provider_store.go`
- Modify: `internal/api/providers.go`

- [ ] **Step 1: 移除 Provider.EmbeddingModel 字段**

在 `internal/models/provider.go` 中删除：
```go
EmbeddingModel  string    `json:"embedding_model"`
```

- [ ] **Step 2: 更新 provider_store.go 的 SQL 查询**

在 `internal/store/provider_store.go` 中，所有 SELECT 语句都包含 `embedding_model`，需要移除该列。共有 3 处（GetActive、GetByID、List）：

将：
```sql
SELECT id, name, type, encrypted_api_key, base_url, selected_model, embedding_model, is_active, created_at, updated_at
```
全部替换为：
```sql
SELECT id, name, type, encrypted_api_key, base_url, selected_model, is_active, created_at, updated_at
```

- [ ] **Step 3: 更新 scanProvider 和 scanProviderRows**

在 `scanProvider`（约第 233 行）中，将：
```go
err := row.Scan(
    &p.ID, &p.Name, &p.Type, &p.EncryptedAPIKey,
    &p.BaseURL, &p.SelectedModel, &p.EmbeddingModel, &isActive,
    &p.CreatedAt, &p.UpdatedAt,
)
```
改为：
```go
err := row.Scan(
    &p.ID, &p.Name, &p.Type, &p.EncryptedAPIKey,
    &p.BaseURL, &p.SelectedModel, &isActive,
    &p.CreatedAt, &p.UpdatedAt,
)
```

同样更新 `scanProviderRows`（约第 250 行）中的 Scan 调用，移除 `&p.EmbeddingModel`。

- [ ] **Step 4: 移除 SetEmbeddingModel()**

在 `internal/store/provider_store.go` 中删除整个 `SetEmbeddingModel` 方法（约第 106-113 行）：
```go
// SetEmbeddingModel 设置 provider 的 embedding 模型。
func (s *ProviderStore) SetEmbeddingModel(id, model string) error {
    _, err := s.db.Exec(
        `UPDATE providers SET embedding_model = ?, updated_at = ? WHERE id = ?`,
        model, time.Now().UTC(), id,
    )
    return err
}
```

- [ ] **Step 5: 移除 setProviderEmbeddingModel()**

在 `internal/api/providers.go` 中删除整个 `setProviderEmbeddingModel` 函数（约第 217-230 行）：
```go
func setProviderEmbeddingModel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    ...
}
```

- [ ] **Step 6: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出

- [ ] **Step 7: Commit**

```bash
git add internal/models/provider.go internal/store/provider_store.go internal/api/providers.go
git commit -m "refactor: remove embedding_model from provider"
```

---

### Task 8: 前端 — ProfileView.vue 移除 Embedding 列

**Files:**
- Modify: `web/src/views/ProfileView.vue`

- [ ] **Step 1: 移除表头 Embedding 模型列**

在 `ProfileView.vue` 中找到：
```html
<thead><tr><th>名称</th><th>类型</th><th>请求地址</th><th>模型</th><th>Embedding 模型</th><th>状态</th><th>操作</th></tr></thead>
```
改为：
```html
<thead><tr><th>名称</th><th>类型</th><th>请求地址</th><th>模型</th><th>状态</th><th>操作</th></tr></thead>
```

- [ ] **Step 2: 移除编辑行中的空 Embedding 列**

找到编辑模式（`v-if="editingProviderId === p.id"`）中的：
```html
<td></td>
<td></td>
```
（第一个空 td 是 API key 占位，第二个是 embedding 占位）改为只保留一个：
```html
<td></td>
```

- [ ] **Step 3: 移除只读行中的 Embedding input**

找到并删除：
```html
<td>
  <input
    :value="p.embedding_model"
    @change="changeEmbeddingModel(p.id, ($event.target as HTMLInputElement).value)"
    class="input input-inline"
    placeholder="如 text-embedding-3-small"
  />
</td>
```

- [ ] **Step 4: 修复 colspan**

找到：
```html
<td colspan="7" class="dim" style="text-align:center;padding:24px">暂无供应商配置</td>
```
改为：
```html
<td colspan="6" class="dim" style="text-align:center;padding:24px">暂无供应商配置</td>
```

- [ ] **Step 5: 移除 TypeScript 中的 embedding_model 字段和 changeEmbeddingModel 函数**

在 `Provider` interface 中删除：
```typescript
embedding_model: string;
```

删除整个 `changeEmbeddingModel` 函数：
```typescript
async function changeEmbeddingModel(providerId: string, model: string) {
  await fetch(`/api/v1/providers/${providerId}/embedding-model`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ model }),
  })
  const p = providers.value.find(x => x.id === providerId)
  if (p) p.embedding_model = model
}
```

- [ ] **Step 6: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "refactor: remove embedding model column from provider table"
```

---

### Task 9: 前端 — KnowledgeView.vue 加 Embedding 配置卡片

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: 在 `<script setup lang="ts">` 中加状态和函数**

在 KnowledgeView.vue 的 script 区域，加以下响应式状态（放在其他 ref 声明附近）：

```typescript
// Embedding 配置
const ragConfig = ref({ type: 'openai', base_url: '', model: '', api_key_set: false })
const ragConfigForm = ref({ type: 'openai', base_url: '', model: '', api_key: '' })
const ragConfigSaving = ref(false)
const ragConfigError = ref('')
const ragConfigOk = ref(false)
```

加以下函数（放在其他 async function 附近）：

```typescript
async function loadRagConfig() {
  try {
    const res = await fetch('/api/v1/rag-config', { headers: authHeaders() })
    if (!res.ok) return
    const data = await res.json()
    ragConfig.value = data
    ragConfigForm.value = { type: data.type || 'openai', base_url: data.base_url || '', model: data.model || '', api_key: '' }
  } catch {}
}

async function saveRagConfig() {
  ragConfigSaving.value = true
  ragConfigError.value = ''
  ragConfigOk.value = false
  try {
    const res = await fetch('/api/v1/rag-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(ragConfigForm.value),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: '保存失败' }))
      ragConfigError.value = err.error || '保存失败'
      return
    }
    const data = await res.json()
    ragConfig.value = data
    ragConfigForm.value.api_key = ''
    ragConfigOk.value = true
    setTimeout(() => { ragConfigOk.value = false }, 3000)
  } catch {
    ragConfigError.value = '网络错误'
  } finally {
    ragConfigSaving.value = false
  }
}
```

在 `onMounted` 中加 `loadRagConfig()` 调用（与其他 load 函数并列）。

- [ ] **Step 2: 在模板中加 Embedding 配置卡片**

在 KnowledgeView.vue 的主内容区（右侧面板，`activeDoc` 为 null 时的空状态区域附近），加一个配置卡片。找到右侧主内容区的根元素，在文档详情展示区域之外（比如在 `<div class="kb-main">` 内、文档详情 `v-if="activeDoc"` 之后）加：

```html
<!-- Embedding 配置 -->
<div class="edit-card" style="margin:16px;max-width:520px">
  <div style="font-weight:600;margin-bottom:12px">Embedding 配置</div>
  <p class="dim" style="font-size:13px;margin-bottom:16px">
    用于知识库文档向量化和检索，需要支持 OpenAI 兼容 embedding 接口的供应商。
  </p>
  <div style="display:flex;flex-direction:column;gap:10px">
    <div>
      <label class="label">请求地址</label>
      <input v-model="ragConfigForm.base_url" class="input" placeholder="留空使用 https://api.openai.com" />
    </div>
    <div>
      <label class="label">模型</label>
      <input v-model="ragConfigForm.model" class="input" placeholder="如 text-embedding-3-small" />
    </div>
    <div>
      <label class="label">API Key</label>
      <input
        v-model="ragConfigForm.api_key"
        class="input"
        type="password"
        :placeholder="ragConfig.api_key_set ? '已设置，留空保留原值' : '输入 API Key'"
      />
    </div>
    <div v-if="ragConfigError" class="err" style="font-size:13px">{{ ragConfigError }}</div>
    <div v-if="ragConfigOk" style="font-size:13px;color:var(--color-ok,#4caf50)">已保存</div>
    <div>
      <button class="btn btn-primary btn-sm" @click="saveRagConfig" :disabled="ragConfigSaving">
        {{ ragConfigSaving ? '保存中...' : '保存' }}
      </button>
    </div>
  </div>
</div>
```

- [ ] **Step 3: 前端构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in ...`（无 TypeScript 错误）

- [ ] **Step 4: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat: add Embedding config card to KnowledgeView"
```

---

### Task 10: 后端完整构建 + 前端构建

- [ ] **Step 1: 后端完整构建**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无输出

- [ ] **Step 2: 前端完整构建**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in ...`

- [ ] **Step 3: 手动验证流程**

启动服务后：
1. 打开知识库页面，应看到"Embedding 配置"卡片
2. 填入 base_url、model、api_key，点保存，应返回 `api_key_set: true`
3. 刷新页面，api_key 输入框 placeholder 应显示"已设置，留空保留原值"
4. 打开供应商配置页，表格应无"Embedding 模型"列
5. 尝试导入文档，未配置时应报错"RAG embedding 未配置"，配置后应正常

- [ ] **Step 4: 最终 Commit（如有遗漏文件）**

```bash
cd /Users/cw/fty.ai/spider.ai && git status
# 确认无遗漏，按需 add + commit
```
