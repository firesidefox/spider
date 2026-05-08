# Embedding 配置 UX 改版 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重新设计个人设置知识库 tab 的 Embedding 配置卡片，支持从供应商列表选择请求地址、自动填充 API Key、获取模型列表、以及验证配置有效性。

**Architecture:** 后端新增 `POST /api/v1/rag-config/validate` 接口，前端 ProfileView.vue 的 kb tab 重写为 combobox 交互，供应商数据复用已有 `GET /api/v1/providers` 接口，模型列表复用 `GET /api/v1/providers/:id/models`。

**Tech Stack:** Go (net/http), Vue 3 + TypeScript, fetch API

---

## 文件变更范围

| 文件 | 操作 |
|------|------|
| `internal/api/rag_config.go` | 新增 `validateRagConfig` handler |
| `internal/api/handler.go` | 注册 `POST /api/v1/rag-config/validate` 路由 |
| `internal/api/providers.go` | `providerResponse` 新增 `APIKey string json:"api_key"` 字段 |
| `web/src/views/ProfileView.vue` | 重写 kb tab 的 Embedding 配置卡片（template + script） |

---

### Task 1: 后端 validate 接口

**Files:**
- Modify: `internal/api/rag_config.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: 在 rag_config.go 末尾添加 validateRagConfig handler**

```go
func validateRagConfig(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	if req.Type == "" {
		req.Type = "openai"
	}
	embedder, err := rag.NewEmbedder(req.Type, req.APIKey, req.Model, req.BaseURL, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := embedder.Embed(r.Context(), "test"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "embedding request failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
```

- [ ] **Step 2: 在 handler.go 的 rag-config 路由块后注册新路由**

在 `internal/api/handler.go` 中，找到：
```go
	mux.HandleFunc("/api/v1/rag-config", func(w http.ResponseWriter, r *http.Request) {
```
在该块之后（`})`之后）添加：
```go
	mux.HandleFunc("/api/v1/rag-config/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			validateRagConfig(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
```

- [ ] **Step 3: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add internal/api/rag_config.go internal/api/handler.go
git commit -m "feat(api): add POST /api/v1/rag-config/validate endpoint"
```

---

### Task 2: 后端 — providers 列表响应新增 api_key 字段

**背景：** `models.Provider.EncryptedAPIKey` 标注了 `json:"-"`，前端拿不到明文 key。需要在 `providerResponse` 中新增解密后的 `APIKey` 字段，供前端自动填充。

**Files:**
- Modify: `internal/api/providers.go:12-30`

- [ ] **Step 1: 修改 providerResponse struct**

将 `providers.go` 第 12–15 行：
```go
type providerResponse struct {
	models.Provider
	Models []*models.ProviderModel `json:"models"`
}
```
改为：
```go
type providerResponse struct {
	models.Provider
	APIKey string                  `json:"api_key"`
	Models []*models.ProviderModel `json:"models"`
}
```

- [ ] **Step 2: 修改 buildProviderResponse，填充 APIKey**

将 `providers.go` 第 21–30 行：
```go
func buildProviderResponse(app *mcppkg.App, p *models.Provider) (*providerResponse, error) {
	ms, err := app.ProviderStore.ListModels(p.ID)
	if err != nil {
		return nil, err
	}
	if ms == nil {
		ms = []*models.ProviderModel{}
	}
	return &providerResponse{Provider: *p, Models: ms}, nil
}
```
改为：
```go
func buildProviderResponse(app *mcppkg.App, p *models.Provider) (*providerResponse, error) {
	ms, err := app.ProviderStore.ListModels(p.ID)
	if err != nil {
		return nil, err
	}
	if ms == nil {
		ms = []*models.ProviderModel{}
	}
	apiKey, _ := app.ProviderStore.DecryptAPIKey(p)
	return &providerResponse{Provider: *p, APIKey: apiKey, Models: ms}, nil
}
```

- [ ] **Step 3: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add internal/api/providers.go
git commit -m "feat(api): expose decrypted api_key in provider list response"
```

---

### Task 3: 前端 ProfileView.vue — 重写 kb tab template

**Files:**
- Modify: `web/src/views/ProfileView.vue` (template 部分，约第 386–419 行)

当前 kb tab template（第 387–419 行）：
```html
<template v-if="activeTab === 'kb'">
  <div v-if="ragConfigError" class="err" style="margin-bottom:12px">{{ ragConfigError }}</div>
  <div class="edit-card">
    <div class="edit-card-title">Embedding 配置</div>
    <p class="dim" style="margin-bottom:16px;font-size:13px">
      用于知识库文档向量化和语义检索，需要支持 OpenAI 兼容 embedding 接口的供应商。</p>
    <div class="form-rows">
      <div class="form-row">
        <label>请求地址</label>
        <input v-model="ragConfigForm.base_url" class="input" placeholder="留空使用 https://api.openai.com" />
      </div>
      <div class="form-row">
        <label>模型</label>
        <input v-model="ragConfigForm.model" class="input" placeholder="如 text-embedding-3-small" />
      </div>
      <div class="form-row">
        <label>API Key</label>
        <input
          v-model="ragConfigForm.api_key"
          class="input"
          type="password"
          :placeholder="ragConfig.api_key_set ? '已设置，留空保留原值' : '输入 API Key'"
        />
      </div>
    </div>
    <div v-if="ragConfigSaveError" class="err" style="margin-top:8px;font-size:13px">{{ ragConfigSaveError }}</div>
    <div v-if="ragConfigOk" style="margin-top:8px;font-size:13px;color:var(--green)">已保存 ✓</div>
    <div style="margin-top:16px">
      <button class="btn btn-primary btn-sm" :disabled="ragConfigSaving" @click="saveRagConfig">
        {{ ragConfigSaving ? '保存中…' : '保存' }}
      </button>
    </div>
  </div>
</template>
```

- [ ] **Step 1: 替换 kb tab template**

将上述整块替换为：

```html
        <!-- Tab: 知识库 -->
        <template v-if="activeTab === 'kb'">
          <div v-if="ragConfigError" class="err" style="margin-bottom:12px">{{ ragConfigError }}</div>
          <div class="edit-card">
            <div class="edit-card-title">Embedding 配置</div>
            <p class="dim" style="margin-bottom:16px;font-size:13px">
              用于知识库文档向量化和语义检索，需要支持 OpenAI 兼容 embedding 接口的供应商。</p>
            <div class="form-rows">
              <div class="form-row">
                <label>请求地址</label>
                <div class="combobox-wrap">
                  <input v-model="ragConfigForm.base_url" class="input" placeholder="如 https://api.openai.com/v1"
                    list="provider-urls" @change="onBaseUrlChange" @input="onBaseUrlInput" />
                  <datalist id="provider-urls">
                    <option v-for="p in kbProviders" :key="p.id" :value="p.base_url">{{ p.name }}</option>
                  </datalist>
                </div>
              </div>
              <div class="form-row">
                <label>API Key</label>
                <input v-model="ragConfigForm.api_key" class="input" type="password"
                  :placeholder="ragConfig.api_key_set ? '已设置，留空保留原值' : '输入 API Key'" />
              </div>
              <div class="form-row">
                <label>模型</label>
                <div style="display:flex;gap:8px;flex:1">
                  <input v-model="ragConfigForm.model" class="input" placeholder="如 text-embedding-3-small"
                    list="embedding-models" style="flex:1" />
                  <datalist id="embedding-models">
                    <option v-for="m in kbModelOptions" :key="m" :value="m" />
                  </datalist>
                  <button class="btn btn-sm" :disabled="!kbSelectedProviderId || kbFetchingModels"
                    @click="fetchEmbeddingModels">
                    {{ kbFetchingModels ? '获取中…' : '获取模型' }}
                  </button>
                </div>
              </div>
            </div>
            <div v-if="kbFetchModelsError" class="err" style="margin-top:6px;font-size:13px">{{ kbFetchModelsError }}</div>
            <div style="margin-top:12px">
              <button class="btn btn-sm" :disabled="kbValidating" @click="validateRagConfig">
                {{ kbValidating ? '验证中…' : '验证' }}
              </button>
              <span v-if="kbValidateResult === 'ok'" style="margin-left:10px;font-size:13px;color:var(--green)">✓ 配置有效</span>
              <span v-else-if="kbValidateResult === 'error'" style="margin-left:10px;font-size:13px;color:var(--red)">{{ kbValidateError }}</span>
            </div>
            <div v-if="ragConfigSaveError" class="err" style="margin-top:8px;font-size:13px">{{ ragConfigSaveError }}</div>
            <div v-if="ragConfigOk" style="margin-top:8px;font-size:13px;color:var(--green)">已保存 ✓</div>
            <div style="margin-top:16px">
              <button class="btn btn-primary btn-sm" :disabled="ragConfigSaving" @click="saveRagConfig">
                {{ ragConfigSaving ? '保存中…' : '保存' }}
              </button>
            </div>
          </div>
        </template>
```

- [ ] **Step 2: 检查 template 替换正确**

```bash
grep -n "获取模型\|验证\|kbProviders\|kbSelectedProviderId" /Users/cw/fty.ai/spider.ai/web/src/views/ProfileView.vue | head -20
```
Expected: 能看到上述关键词出现在 template 区域

---

### Task 4: 前端 ProfileView.vue — 更新 Provider 接口 + 新增 kb script 状态和函数

**Files:**
- Modify: `web/src/views/ProfileView.vue` (script 部分)

- [ ] **Step 1: 更新 Provider 接口（约第 711 行），加 api_key 字段**

找到：
```typescript
interface Provider {
  id: string; name: string; type: string; base_url: string
  selected_model: string; is_active: boolean
  models: ProviderModel[]
  created_at: string; updated_at: string
}
```
改为：
```typescript
interface Provider {
  id: string; name: string; type: string; base_url: string
  api_key: string
  selected_model: string; is_active: boolean
  models: ProviderModel[]
  created_at: string; updated_at: string
}
```

- [ ] **Step 2: 替换 kb 相关状态和函数（约第 847–898 行）**

找到当前代码块（第 847–898 行）：
```typescript
// ── 知识库 / Embedding 配置 ──
const ragConfig = ref({ base_url: '', model: '', api_key_set: false })
const ragConfigForm = ref({ base_url: '', model: '', api_key: '' })
const ragConfigSaving = ref(false)
const ragConfigError = ref('')
const ragConfigSaveError = ref('')
const ragConfigOk = ref(false)
let ragConfigLoaded = false

async function loadRagConfig() {
  if (ragConfigLoaded) return
  ragConfigLoaded = true
  ragConfigError.value = ''
  try {
    const res = await fetch('/api/v1/rag-config', { headers: authHeaders() })
    if (!res.ok) return
    const data = await res.json()
    ragConfig.value = data
    ragConfigForm.value = { base_url: data.base_url ?? '', model: data.model ?? '', api_key: '' }
  } catch (e: any) {
    ragConfigError.value = e.message
  }
}

async function saveRagConfig() {
  ragConfigSaveError.value = ''
  ragConfigOk.value = false
  ragConfigSaving.value = true
  try {
    const body: any = { base_url: ragConfigForm.value.base_url, model: ragConfigForm.value.model }
    if (ragConfigForm.value.api_key) body.api_key = ragConfigForm.value.api_key
    const res = await fetch('/api/v1/rag-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      ragConfigSaveError.value = err.error || '保存失败'
      return
    }
    ragConfigForm.value.api_key = ''
    ragConfigLoaded = false
    await loadRagConfig()
    ragConfigOk.value = true
    setTimeout(() => { ragConfigOk.value = false }, 2000)
  } catch (e: any) {
    ragConfigSaveError.value = e.message
  } finally {
    ragConfigSaving.value = false
  }
}
```

替换为（分两次 Edit，每次不超过 50 行）：

**第一次 Edit（状态变量 + loadRagConfig + saveRagConfig）：**
```typescript
// ── 知识库 / Embedding 配置 ──
const ragConfig = ref({ base_url: '', model: '', api_key_set: false })
const ragConfigForm = ref({ base_url: '', model: '', api_key: '' })
const ragConfigSaving = ref(false)
const ragConfigError = ref('')
const ragConfigSaveError = ref('')
const ragConfigOk = ref(false)
let ragConfigLoaded = false

// kb combobox state — Provider 接口已有 api_key 字段（Task 2 后端已暴露）
const kbProviders = ref<Provider[]>([])
const kbSelectedProviderId = ref<string | null>(null)
const kbModelOptions = ref<string[]>([])
const kbFetchingModels = ref(false)
const kbFetchModelsError = ref('')
const kbValidating = ref(false)
const kbValidateResult = ref<'ok' | 'error' | null>(null)
const kbValidateError = ref('')

async function loadRagConfig() {
  if (ragConfigLoaded) return
  ragConfigLoaded = true
  ragConfigError.value = ''
  try {
    const [cfgRes, provRes] = await Promise.all([
      fetch('/api/v1/rag-config', { headers: authHeaders() }),
      fetch('/api/v1/providers', { headers: authHeaders() }),
    ])
    if (cfgRes.ok) {
      const data = await cfgRes.json()
      ragConfig.value = data
      ragConfigForm.value = { base_url: data.base_url ?? '', model: data.model ?? '', api_key: '' }
    }
    if (provRes.ok) {
      kbProviders.value = await provRes.json()
    }
  } catch (e: any) {
    ragConfigError.value = e.message
  }
}

async function saveRagConfig() {
  ragConfigSaveError.value = ''
  ragConfigOk.value = false
  ragConfigSaving.value = true
  try {
    const body: any = { base_url: ragConfigForm.value.base_url, model: ragConfigForm.value.model }
    if (ragConfigForm.value.api_key) body.api_key = ragConfigForm.value.api_key
    const res = await fetch('/api/v1/rag-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      ragConfigSaveError.value = err.error || '保存失败'
      return
    }
    ragConfigForm.value.api_key = ''
    ragConfigLoaded = false
    await loadRagConfig()
    ragConfigOk.value = true
    setTimeout(() => { ragConfigOk.value = false }, 2000)
  } catch (e: any) {
    ragConfigSaveError.value = e.message
  } finally {
    ragConfigSaving.value = false
  }
}
```

**第二次 Edit（追加三个新函数，紧接在 saveRagConfig 之后）：**
```typescript

function onBaseUrlChange() {
  const matched = kbProviders.value.find(p => p.base_url === ragConfigForm.value.base_url)
  if (matched) {
    kbSelectedProviderId.value = matched.id
    if (matched.api_key) ragConfigForm.value.api_key = matched.api_key
  }
}

function onBaseUrlInput() {
  const matched = kbProviders.value.find(p => p.base_url === ragConfigForm.value.base_url)
  if (!matched) {
    kbSelectedProviderId.value = null
    kbModelOptions.value = []
  } else {
    kbSelectedProviderId.value = matched.id
    if (matched.api_key) ragConfigForm.value.api_key = matched.api_key
  }
}

async function fetchEmbeddingModels() {
  if (!kbSelectedProviderId.value) return
  kbFetchingModels.value = true
  kbFetchModelsError.value = ''
  try {
    const res = await fetch(`/api/v1/providers/${kbSelectedProviderId.value}/models`, { headers: authHeaders() })
    if (!res.ok) { kbFetchModelsError.value = '获取失败'; return }
    const data: Array<{ model_id: string; display_name: string }> = await res.json()
    kbModelOptions.value = data.map(m => m.model_id)
  } catch (e: any) {
    kbFetchModelsError.value = e.message
  } finally {
    kbFetchingModels.value = false
  }
}

async function validateRagConfig() {
  kbValidating.value = true
  kbValidateResult.value = null
  kbValidateError.value = ''
  try {
    const body: any = {
      type: 'openai',
      base_url: ragConfigForm.value.base_url,
      model: ragConfigForm.value.model,
    }
    if (ragConfigForm.value.api_key) body.api_key = ragConfigForm.value.api_key
    const res = await fetch('/api/v1/rag-config/validate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(body),
    })
    if (res.ok) {
      kbValidateResult.value = 'ok'
    } else {
      const err = await res.json().catch(() => ({}))
      kbValidateResult.value = 'error'
      kbValidateError.value = err.error || '验证失败'
    }
  } catch (e: any) {
    kbValidateResult.value = 'error'
    kbValidateError.value = e.message
  } finally {
    kbValidating.value = false
  }
}
```

- [ ] **Step 2: 构建前端**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: `✓ built in` 无 TypeScript 错误

- [ ] **Step 3: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ProfileView.vue
git commit -m "feat(frontend): redesign embedding config UX with provider combobox and validate button"
```

---

### Task 5: 端到端验证

- [ ] **Step 1: 后端构建**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: 无错误

- [ ] **Step 2: 前端构建**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -5
```
Expected: `✓ built in`

- [ ] **Step 3: 手动验证路由注册**

```bash
grep -n "rag-config/validate" /Users/cw/fty.ai/spider.ai/internal/api/handler.go
```
Expected: 能看到 `POST /api/v1/rag-config/validate` 路由注册行

- [ ] **Step 4: 验证前端关键词**

```bash
grep -n "获取模型\|kbValidating\|kbSelectedProviderId\|datalist" /Users/cw/fty.ai/spider.ai/web/src/views/ProfileView.vue | head -20
```
Expected: 能看到 template 和 script 中的关键词
