# Model Provider DB Migration + OpenAI Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate model provider config from config.yaml to DB, add OpenAI-compatible LLM client, simplify provider setup to "add and go", clean up Embedding config.

**Architecture:** New `providers` + `provider_models` DB tables with ProviderStore for CRUD. API handlers rewritten to use DB instead of config. Factory reads active provider from DB. OpenAI client implements same `llm.Client` interface as Claude. Frontend ProfileView uses new CRUD API with per-row edit + model dropdown.

**Tech Stack:** Go 1.23, SQLite (modernc.org/sqlite), Vue 3.4, existing crypto.Manager for API key encryption

**Spec Reference:** `docs/spec-20260503-model-provider.md`

---

### Task 1: DB Schema + Models

**Files:**
- Modify: `internal/db/schema.go`
- Create: `internal/models/provider.go`

- [ ] **Step 1: Add providers + provider_models tables to schema**

In `internal/db/schema.go`, add to `schemaSQL` before the closing backtick:

```sql
CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    encrypted_api_key TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    selected_model TEXT NOT NULL DEFAULT '',
    is_active INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_provider_models_provider_id ON provider_models(provider_id);
```

- [ ] **Step 2: Create Provider and ProviderModel structs**

`internal/models/provider.go`:

```go
package models

import "time"

type Provider struct {
    ID              string    `json:"id"`
    Name            string    `json:"name"`
    Type            string    `json:"type"`
    EncryptedAPIKey string    `json:"-"`
    BaseURL         string    `json:"base_url"`
    SelectedModel   string    `json:"selected_model"`
    IsActive        bool      `json:"is_active"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type ProviderModel struct {
    ID          int       `json:"id"`
    ProviderID  string    `json:"provider_id"`
    ModelID     string    `json:"model_id"`
    DisplayName string    `json:"display_name"`
    CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 3: Build + commit**

Run: `go build ./...`
Commit: `feat: add providers and provider_models DB schema + model structs`

---

### Task 2: ProviderStore — CRUD + Model Storage

**Files:**
- Create: `internal/store/provider_store.go`

- [ ] **Step 1: Create ProviderStore**

`internal/store/provider_store.go` — follows same pattern as HostStore:

```go
type ProviderStore struct {
    db     *sql.DB
    crypto *crypto.Manager
}
func NewProviderStore(db *sql.DB, cm *crypto.Manager) *ProviderStore
```

Methods:
- `Create(name, providerType, apiKey, baseURL string) (*models.Provider, error)` — generate UUID, encrypt API key, INSERT, return provider
- `GetByID(id string) (*models.Provider, error)` — SELECT by id
- `List() ([]*models.Provider, error)` — SELECT all ORDER BY created_at
- `Update(id string, name, providerType *string, apiKey, baseURL *string) (*models.Provider, error)` — partial update, encrypt new API key if provided
- `Delete(id string) error` — DELETE (cascade deletes provider_models)
- `Activate(id string) error` — SET is_active=0 for all, then SET is_active=1 for id
- `SetSelectedModel(id, model string) error` — UPDATE selected_model
- `GetActive() (*models.Provider, error)` — SELECT WHERE is_active=1
- `DecryptAPIKey(p *models.Provider) (string, error)` — decrypt encrypted_api_key
- `CountAll() (int, error)` — SELECT COUNT(*)

Model methods:
- `SaveModels(providerID string, models []llm.ModelInfo) error` — DELETE old + INSERT new
- `ListModels(providerID string) ([]*models.ProviderModel, error)` — SELECT by provider_id

- [ ] **Step 2: Build + test + commit**

Run: `go build ./internal/store/`
Commit: `feat: add ProviderStore with CRUD and model storage`

---

### Task 3: OpenAI-Compatible LLM Client

**Files:**
- Create: `internal/llm/openai.go`
- Modify: `internal/llm/client.go`

- [ ] **Step 1: Create OpenAI client**

`internal/llm/openai.go`:

```go
const defaultOpenAIBaseURL = "https://api.openai.com"

type OpenAIClient struct {
    apiKey  string
    model   string
    baseURL string
    http    *http.Client
}

func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient
```

`ChatStream` implementation:
- POST `{baseURL}/v1/chat/completions` with `stream: true`
- Header: `Authorization: Bearer {apiKey}`, `Content-Type: application/json`
- Body: `{"model": model, "messages": [...], "tools": [...], "stream": true, "max_tokens": maxTokens}`
- Messages format: `{"role": "user"/"assistant", "content": "..."}`
- Tools format: OpenAI function calling — `{"type": "function", "function": {"name": ..., "description": ..., "parameters": inputSchema}}`
- SSE parsing: `data: {"choices":[{"delta":{"content":"text"}}]}` → `StreamEvent{Type: "text_delta", Text: text}`
- Tool calls: `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_xxx","function":{"name":"...","arguments":"..."}}]}}]}`
  → accumulate arguments across deltas, emit `tool_start` + `tool_input_delta`
- `data: [DONE]` → emit `message_stop`

- [ ] **Step 2: Update NewClient factory**

In `internal/llm/client.go`, add openai case:
```go
case "openai":
    return NewOpenAIClient(apiKey, model, baseURL), nil
```

- [ ] **Step 3: Build + test + commit**

Run: `go build ./internal/llm/` and `go test ./internal/llm/ -v`
Commit: `feat(llm): add OpenAI-compatible LLM client with streaming`

---

### Task 4: Config Cleanup — Remove Model/Embedding Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Remove ModelConfig, ProviderConfig, EmbeddingConfig from config.go**

In `internal/config/config.go`:
- Remove `Model ModelConfig` and `Embedding EmbeddingConfig` from Config struct
- Remove `ProviderConfig`, `ModelConfig`, `EmbeddingModelConfig`, `EmbeddingConfig` types
- Remove `GetActiveProvider()`, `GetProvider()`, `ResolveAPIKey()` on ProviderConfig
- Remove `ActiveModel()`, `ResolveAPIKey()` on EmbeddingConfig/EmbeddingModelConfig
- Config struct becomes: DataDir, LogLevel, SSH, SSE, Auth only

- [ ] **Step 2: Update config_test.go**

Remove `TestLoadModelConfig`, `TestGetActiveProvider`, `TestProviderResolveAPIKey` tests. Keep SSH/SSE/Auth tests.

- [ ] **Step 3: Fix all compilation errors**

Files that reference removed types will break. Fix each:
- `internal/api/settings.go` — remove `saveConfig` (moved to providers.go), remove Model from settingsResponse
- `internal/rag/embedder.go` — remove `NewEmbedder(cfg *config.EmbeddingModelConfig)`, keep interface
- Any other references

- [ ] **Step 4: Build + test + commit**

Run: `go build ./...` and `go test ./... -v`
Commit: `refactor: remove Model/Embedding config from config.yaml`

---

### Task 5: API Handlers — Provider CRUD via DB

**Files:**
- Rewrite: `internal/api/providers.go`
- Modify: `internal/api/handler.go`
- Modify: `internal/api/settings.go`

- [ ] **Step 1: Rewrite providers.go — DB-backed CRUD**

Replace entire `internal/api/providers.go`. All handlers use `app.ProviderStore` instead of `app.Config.Model`.

Handlers:
- `listProviders(app, w, r)` — `app.ProviderStore.List()` + `app.ProviderStore.ListModels(p.ID)` for each, return JSON array with models nested
- `createProvider(app, w, r)` — decode request, validate type (`anthropic`/`openai`), call `app.ProviderStore.Create(...)`, then auto-fetch models via `llm.ListModels()` + `app.ProviderStore.SaveModels()`, auto-select first model, auto-activate if first provider
- `updateProvider(app, w, r, id)` — decode partial update, call `app.ProviderStore.Update(...)`
- `deleteProvider(app, w, r, id)` — call `app.ProviderStore.Delete(id)`
- `refreshModels(app, w, r, id)` — get provider, decrypt API key, call `llm.ListModels()`, save to DB, return models
- `activateProvider(app, w, r, id)` — call `app.ProviderStore.Activate(id)`, if no selected_model auto-select first
- `setProviderModel(app, w, r, id)` — decode `{model}`, call `app.ProviderStore.SetSelectedModel(id, model)`
- `listProviderModels(app, w, r, id)` — `app.ProviderStore.ListModels(id)`, return JSON array

- [ ] **Step 2: Update handler.go routes**

Replace existing provider routes with:
```
GET    /api/v1/providers              → listProviders
POST   /api/v1/providers              → createProvider
PUT    /api/v1/providers/{id}         → updateProvider
DELETE /api/v1/providers/{id}         → deleteProvider
POST   /api/v1/providers/{id}/refresh → refreshModels
PUT    /api/v1/providers/{id}/activate → activateProvider
PUT    /api/v1/providers/{id}/model   → setProviderModel
GET    /api/v1/providers/{id}/models  → listProviderModels
```

Remove `/api/v1/providers/active` route.

- [ ] **Step 3: Clean settings.go**

Remove `saveConfig`, `maskedProvider`, `maskedModelConfig`, `maskKey` (move maskKey to providers.go if still needed). Remove `Model` from settingsResponse. Settings only handles SSH/SSE config. Use `saveConfig` helper that writes config.yaml (SSH/SSE only).

- [ ] **Step 4: Build + commit**

Run: `go build ./...`
Commit: `feat(api): rewrite provider handlers to use DB store`

---

### Task 6: Agent Factory — Read from DB

**Files:**
- Modify: `internal/agent/factory.go`
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Update App struct**

In `internal/mcp/server.go`:
- Add `ProviderStore *store.ProviderStore` to App
- Remove `ConfigMu sync.RWMutex` (no longer needed, DB handles concurrency)

- [ ] **Step 2: Rewrite agent Factory**

`internal/agent/factory.go` — change `NewFactory` to accept `ProviderStore` instead of reading from config:

```go
func NewFactory(
    providerStore *store.ProviderStore,
    database *sql.DB,
    hosts *store.HostStore,
    pool *ssh.Pool,
    keys *store.SSHKeyStore,
    logs *store.LogStore,
    msgs MessageStorer,
) (*Factory, error) {
    provider, err := providerStore.GetActive()
    if err != nil || provider == nil {
        return nil, fmt.Errorf("no active provider")
    }
    apiKey, err := providerStore.DecryptAPIKey(provider)
    if err != nil {
        return nil, fmt.Errorf("decrypt API key: %w", err)
    }
    llmClient, err := llm.NewClient(provider.Type, apiKey, provider.SelectedModel, provider.BaseURL)
    if err != nil {
        return nil, fmt.Errorf("create LLM client: %w", err)
    }
    // ... rest unchanged, remove RAG/Embedding setup
}
```

Remove `cfg *config.Config`, `docStore`, and Embedding RAG setup from Factory.

- [ ] **Step 3: Update main.go**

In `cmd/spider/main.go`:
- Initialize `ProviderStore`: `ps := store.NewProviderStore(database, cm)`
- Set `app.ProviderStore = ps`
- Update `agent.NewFactory(ps, database, hs, pool, ks, ls, app.MsgStore)` call
- Remove `app.DocStore` if no longer used by factory

- [ ] **Step 4: Build + test + commit**

Run: `go build ./...` and `go test ./... -v`
Commit: `refactor: agent factory reads provider from DB instead of config`

---

### Task 7: Frontend — Provider Management UI

**Files:**
- Modify: `web/src/views/ProfileView.vue`
- Modify: `web/src/api/chat.ts`
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Rewrite ProfileView provider tab**

Replace the 模型供应商 template section. New table columns: 名称 | 类型 | 请求地址 | 模型(下拉框) | 状态 | 操作

Read-only row:
```
| My Claude | Anthropic 兼容 | 默认 | [claude-sonnet ▾] | 已启用 | 编辑 获取模型 |
```

Edit row (when editingProviderId matches):
```
| [input name] | [select type] | [input base_url] | [input api_key] | | 保存 取消 删除 |
```

Model dropdown: `<select>` populated from provider's models array, `@change` calls `PUT /providers/{id}/model`.

Type options: `Anthropic 兼容` (value: `anthropic`) / `OpenAI 兼容` (value: `openai`).

- [ ] **Step 2: Update provider script functions**

Replace all provider functions to use new API:
- `loadProviders()` — `GET /api/v1/providers`, response is array with nested models
- `addProvider()` — `POST /api/v1/providers` with `{name, type, api_key, base_url}`, returns provider with auto-fetched models
- `saveProvider(p)` — `PUT /api/v1/providers/{id}` with partial update
- `removeProvider(id)` — `DELETE /api/v1/providers/{id}`
- `refreshModels(id)` — `POST /api/v1/providers/{id}/refresh`, update local models
- `enableProvider(id)` — `PUT /api/v1/providers/{id}/activate`
- `changeModel(id, model)` — `PUT /api/v1/providers/{id}/model` with `{model}`

Remove old `settingsEditing`-based flow for providers. Each row has independent edit state.

- [ ] **Step 3: Update chat.ts**

Replace `getActiveModel` and `setActiveModel`:
```typescript
export async function getActiveModel(): Promise<{provider_id: string, model: string}> {
  const res = await fetch('/api/v1/providers', { headers: authHeaders() })
  const providers = await res.json()
  const active = providers.find((p: any) => p.is_active)
  return { provider_id: active?.id || '', model: active?.selected_model || '' }
}

export async function setActiveModel(providerId: string, model: string): Promise<void> {
  await fetch(`/api/v1/providers/${providerId}/model`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ model }),
  })
}
```

- [ ] **Step 4: Add model name to Chat header**

In `ChatView.vue`, add current model display in chat-header:
```html
<span class="current-model">{{ currentModelName }}</span>
```
Load on mount via `getActiveModel()`.

- [ ] **Step 5: Build + commit**

Run: `cd web && npx vite build`
Commit: `feat(web): provider management with model dropdown and chat model display`

---

### Task 8: Full Build + Verification

- [ ] **Step 1: Full Go build**

Run: `go build ./...`

- [ ] **Step 2: Full Go tests**

Run: `go test ./... -v`

- [ ] **Step 3: Frontend build**

Run: `cd web && npx vite build`

- [ ] **Step 4: Start server + browser test**

1. Start server, go to 个人设置 → 模型供应商
2. Add provider (Anthropic 兼容, fill API key) → verify auto-fetches models, auto-selects first, auto-activates
3. Model dropdown → change model → verify persisted on reload
4. Add second provider (OpenAI 兼容) → verify not auto-activated
5. Click 启用 on second → verify first deactivated
6. Edit provider → change name → save → verify persisted
7. Delete provider → verify removed
8. Go to /chat → verify model name in header
9. Type `/model` → verify shows models + can switch
