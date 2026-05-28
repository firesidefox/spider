# 知识库接口「试一试」实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在知识库 entry card 展开后内联一个「试一试」面板，让用户选择已配置的 Prometheus 数据源、填写参数、发送真实 HTTP 请求并查看响应。

**Architecture:** 后端新增 `POST /api/v1/knowledge-entries/{id}/try` 代理端点，接收 source_id + params，用 PrometheusSource 的 base URL 和 auth 凭据转发请求，返回原始响应体 + 状态码 + 耗时。前端仅修改 KnowledgeView.vue，在 inline-detail 区域追加 try panel，局部状态管理。

**Tech Stack:** Go (net/http), Vue 3 (Composition API), TypeScript

---

## 文件变更清单

| 操作 | 文件 |
|------|------|
| 新建 | `internal/api/knowledge_try.go` |
| 新建 | `internal/api/knowledge_try_test.go` |
| 修改 | `internal/api/handler.go` — 注册新路由 |
| 修改 | `web/src/api/knowledge.ts` — 新增 tryEntry 函数 |
| 修改 | `web/src/views/KnowledgeView.vue` — try panel UI + 状态 |

---

## Task 1: 后端 handler — tryKnowledgeEntry

**Files:**
- Create: `internal/api/knowledge_try.go`
- Create: `internal/api/knowledge_try_test.go`

### 接口定义

```
POST /api/v1/knowledge-entries/{id}/try
Authorization: cookie (现有 auth 中间件)

Request body:
{
  "source_id": "abc123",
  "params": { "query": "up", "time": "1716000000" }
}

Response 200:
{
  "status": 200,
  "body": "{\"status\":\"success\",...}",
  "latency_ms": 38
}

Response 400: { "error": "invalid entry id" }
Response 404: { "error": "entry not found" }
Response 404: { "error": "source not found" }
Response 502: { "error": "upstream: connection refused" }
```

- [ ] **Step 1: 写失败测试**

新建 `internal/api/knowledge_try_test.go`：

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/models"
)

// mockPrometheusSourceStore implements the prometheusSourceStore interface for tests.
type mockPrometheusSourceStore struct {
	sources map[string]*models.PrometheusSource
}

func (m *mockPrometheusSourceStore) GetByID(id string) (*models.PrometheusSource, error) {
	s, ok := m.sources[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockPrometheusSourceStore) DecryptCredentials(src *models.PrometheusSource) (password, token string, err error) {
	return "", "", nil
}

func TestTryKnowledgeEntry_Success(t *testing.T) {
	// upstream fake server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer upstream.Close()

	docStore := &mockDocStore{entries: []knowledge.Entry{
		{ID: 7, Title: "GET /api/v1/query", Content: ""},
	}}
	srcStore := &mockPrometheusSourceStore{sources: map[string]*models.PrometheusSource{
		"src1": {ID: "src1", BaseURL: upstream.URL, AuthType: "none"},
	}}

	body := bytes.NewBufferString(`{"source_id":"src1","params":{"query":"up"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-entries/7/try", body)
	w := httptest.NewRecorder()

	tryKnowledgeEntry(docStore, srcStore, w, req, "7")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result tryResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != 200 {
		t.Errorf("expected status 200, got %d", result.Status)
	}
	if result.LatencyMs < 0 {
		t.Errorf("negative latency")
	}
}

func TestTryKnowledgeEntry_EntryNotFound(t *testing.T) {
	docStore := &mockDocStore{entries: []knowledge.Entry{}}
	srcStore := &mockPrometheusSourceStore{sources: map[string]*models.PrometheusSource{}}

	body := bytes.NewBufferString(`{"source_id":"src1","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-entries/99/try", body)
	w := httptest.NewRecorder()

	tryKnowledgeEntry(docStore, srcStore, w, req, "99")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTryKnowledgeEntry_SourceNotFound(t *testing.T) {
	docStore := &mockDocStore{entries: []knowledge.Entry{
		{ID: 7, Title: "GET /api/v1/query", Content: ""},
	}}
	srcStore := &mockPrometheusSourceStore{sources: map[string]*models.PrometheusSource{}}

	body := bytes.NewBufferString(`{"source_id":"missing","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-entries/7/try", body)
	w := httptest.NewRecorder()

	tryKnowledgeEntry(docStore, srcStore, w, req, "7")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/api/ -run TestTryKnowledge -v
```

期望：编译错误 `undefined: tryKnowledgeEntry`

- [ ] **Step 3: 实现 knowledge_try.go**

新建 `internal/api/knowledge_try.go`：

```go
package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/models"
)

type prometheusSourceStore interface {
	GetByID(id string) (*models.PrometheusSource, error)
	DecryptCredentials(src *models.PrometheusSource) (password, token string, err error)
}

type tryRequest struct {
	SourceID string            `json:"source_id"`
	Params   map[string]string `json:"params"`
}

type tryResult struct {
	Status    int    `json:"status"`
	Body      string `json:"body"`
	LatencyMs int64  `json:"latency_ms"`
}

func tryKnowledgeEntry(ds docStore, ss prometheusSourceStore, w http.ResponseWriter, r *http.Request, entryIDStr string) {
	entryID, err := strconv.Atoi(entryIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entry id")
		return
	}

	var req tryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceID == "" {
		writeError(w, http.StatusBadRequest, "source_id required")
		return
	}

	entries, err := ds.FetchEntries(r.Context(), []int{entryID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(entries) == 0 {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	entry := entries[0]
	_, path := splitMethodPath(entry.Title)

	src, err := ss.GetByID(req.SourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	pwd, tok, err := ss.DecryptCredentials(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt credentials")
		return
	}

	result, err := doProxyRequest(r.Context(), src, pwd, tok, path, req.Params)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func doProxyRequest(ctx context.Context, src *models.PrometheusSource, pwd, tok, path string, params map[string]string) (*tryResult, error) {
	baseURL := strings.TrimRight(src.BaseURL, "/")

	// substitute path params like {label_name}
	resolvedPath := path
	queryParams := url.Values{}
	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(resolvedPath, placeholder) {
			resolvedPath = strings.ReplaceAll(resolvedPath, placeholder, url.PathEscape(v))
		} else {
			queryParams.Set(k, v)
		}
	}

	fullURL := baseURL + resolvedPath
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	transport := &http.Transport{}
	if src.SkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	timeout := time.Duration(src.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout, Transport: transport}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	switch src.AuthType {
	case models.PrometheusAuthBasic:
		httpReq.SetBasicAuth(src.Username, pwd)
	case models.PrometheusAuthBearer:
		httpReq.Header.Set("Authorization", "Bearer "+tok)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &tryResult{
		Status:    resp.StatusCode,
		Body:      string(body),
		LatencyMs: latency,
	}, nil
}
```

- [ ] **Step 4: 在 knowledge_test.go 补充 mockDocStore**

`knowledge_test.go` 已有 `mockKBStore`，但 `tryKnowledgeEntry` 需要 `docStore`。在 `knowledge_try_test.go` 中添加：

```go
// mockDocStore implements docStore for try tests.
type mockDocStore struct {
	entries []knowledge.Entry
}

func (m *mockDocStore) GetDocument(_ context.Context, _ int) (*knowledge.Document, error) {
	return nil, nil
}
func (m *mockDocStore) GetDocumentSections(_ context.Context, _ int) ([]knowledge.Section, error) {
	return nil, nil
}
func (m *mockDocStore) GetSectionEntries(_ context.Context, _ int) ([]knowledge.EntrySummary, error) {
	return nil, nil
}
func (m *mockDocStore) FetchEntries(_ context.Context, ids []int) ([]knowledge.Entry, error) {
	var out []knowledge.Entry
	for _, e := range m.entries {
		for _, id := range ids {
			if e.ID == id {
				out = append(out, e)
			}
		}
	}
	return out, nil
}
func (m *mockDocStore) ListDocuments(_ context.Context, _ int) ([]knowledge.Document, error) {
	return nil, nil
}
func (m *mockDocStore) DeleteDocuments(_ context.Context, _ []int) error { return nil }
func (m *mockDocStore) MoveDocuments(_ context.Context, _ []int, _ int) error { return nil }
func (m *mockDocStore) CatalogSections(_ context.Context, _ knowledge.Scope) ([]knowledge.Section, error) {
	return nil, nil
}
func (m *mockDocStore) CatalogEntries(_ context.Context, _ int) ([]knowledge.EntrySummary, error) {
	return nil, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/api/ -run TestTryKnowledge -v
```

期望：3 个测试全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/knowledge_try.go internal/api/knowledge_try_test.go
git commit -m "feat(api): add knowledge entry try-it proxy endpoint"
```

---

## Task 2: 注册路由

**Files:**
- Modify: `internal/api/handler.go`

- [ ] **Step 1: 在 handler.go 注册路由**

找到 `/api/v1/knowledge-entries/` 的 handler block（约第 580 行），在 `getKnowledgeEntry` 分支后追加 try 路由：

```go
mux.HandleFunc("/api/v1/knowledge-entries/", func(w http.ResponseWriter, r *http.Request) {
    requireAuth(app, w, r, func(w http.ResponseWriter, r *http.Request) {
        rest := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge-entries/")
        // existing: GET /{id}
        if r.Method == http.MethodGet && !strings.Contains(rest, "/") {
            getKnowledgeEntry(app.KnowledgeStore, w, r, rest)
            return
        }
        // new: POST /{id}/try
        if r.Method == http.MethodPost && strings.HasSuffix(rest, "/try") {
            id := strings.TrimSuffix(rest, "/try")
            tryKnowledgeEntry(app.KnowledgeStore, app.PrometheusSourceStore, w, r, id)
            return
        }
        http.NotFound(w, r)
    })
})
```

- [ ] **Step 2: 编译确认无错误**

```bash
go build ./...
```

期望：无错误输出

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler.go
git commit -m "feat(api): register POST /knowledge-entries/{id}/try route"
```

---

## Task 3: 前端 API 函数

**Files:**
- Modify: `web/src/api/knowledge.ts`

- [ ] **Step 1: 追加类型和函数**

在 `knowledge.ts` 末尾追加：

```typescript
export interface TryEntryRequest {
  source_id: string
  params: Record<string, string>
}

export interface TryEntryResult {
  status: number
  body: string
  latency_ms: number
}

export async function tryEntry(entryID: number, req: TryEntryRequest): Promise<TryEntryResult> {
  const r = await fetch(`${BASE}/knowledge-entries/${entryID}/try`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  return handleResponse<TryEntryResult>(r)
}
```

- [ ] **Step 2: 编译前端确认无 TS 错误**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

期望：`built in` 成功，无 TypeScript 错误

- [ ] **Step 3: Commit**

```bash
git add web/src/api/knowledge.ts
git commit -m "feat(api-client): add tryEntry function for knowledge try-it"
```

---

## Task 4: 前端 Try Panel UI

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

### 4a: 新增状态变量

- [ ] **Step 1: 在 script setup 中追加状态和导入**

在 `KnowledgeView.vue` 的 `<script setup>` 中，找到现有 import 行，追加：

```typescript
import {
  listPrometheusSources,
  type PrometheusSource,
} from '../api/prometheus'
import {
  tryEntry,
  type TryEntryRequest,
  type TryEntryResult,
} from '../api/knowledge'
```

在现有 `const loadingEntries` 之后追加：

```typescript
const prometheusSources = ref<PrometheusSource[]>([])
const tryOpen = ref(new Set<number>())
const trySourceId = ref<Record<number, string>>({})
const tryParams = ref<Record<number, Record<string, string>>>({})
const tryResult = ref<Record<number, TryEntryResult | null>>({})
const tryLoading = ref(new Set<number>())
const tryError = ref<Record<number, string>>({})
```

- [ ] **Step 2: 追加 try panel 操作函数**

在 `toggleEntry` 函数之后追加：

```typescript
function toggleTry(entryId: number) {
  const next = new Set(tryOpen.value)
  next.has(entryId) ? next.delete(entryId) : next.add(entryId)
  tryOpen.value = next
}

function setTryParam(entryId: number, key: string, value: string) {
  tryParams.value = {
    ...tryParams.value,
    [entryId]: { ...(tryParams.value[entryId] ?? {}), [key]: value },
  }
}

async function sendTry(entry: KnowledgeEntry) {
  const id = entry.id
  const sourceId = trySourceId.value[id]
  if (!sourceId) return
  const loading = new Set(tryLoading.value)
  loading.add(id)
  tryLoading.value = loading
  tryError.value = { ...tryError.value, [id]: '' }
  tryResult.value = { ...tryResult.value, [id]: null }
  try {
    const req: TryEntryRequest = {
      source_id: sourceId,
      params: tryParams.value[id] ?? {},
    }
    const result = await tryEntry(id, req)
    tryResult.value = { ...tryResult.value, [id]: result }
  } catch (e: any) {
    tryError.value = { ...tryError.value, [id]: e.message ?? '请求失败' }
  } finally {
    const l = new Set(tryLoading.value)
    l.delete(id)
    tryLoading.value = l
  }
}
```

- [ ] **Step 3: 在 init() 中加载数据源**

找到 `async function init()` 函数，在 `loadPersistence()` 之后追加：

```typescript
listPrometheusSources().then(list => { prometheusSources.value = list }).catch(() => {})
```

### 4b: Try Panel 模板

- [ ] **Step 4: 在 inline-detail 中追加 try panel**

找到 `inline-detail` 区域中 `<div class="collapse-btn"` 之前，追加 try panel：

```html
<!-- Try panel -->
<div class="try-panel">
  <div class="try-header" @click.stop="toggleTry(entry.id)">
    <span class="try-label">试一试</span>
    <span class="try-chevron">{{ tryOpen.has(entry.id) ? '▲' : '▼' }}</span>
  </div>

  <div v-if="tryOpen.has(entry.id)" class="try-body" @click.stop>
    <!-- no sources -->
    <div v-if="!prometheusSources.length" class="try-empty">
      暂无数据源，请先在系统设置中添加 Prometheus 数据源
    </div>

    <template v-else>
      <!-- source selector -->
      <div class="try-row">
        <label class="try-lbl">数据源</label>
        <select class="try-select"
          :value="trySourceId[entry.id] ?? ''"
          @change="trySourceId = { ...trySourceId, [entry.id]: ($event.target as HTMLSelectElement).value }">
          <option value="" disabled>选择数据源…</option>
          <option v-for="s in prometheusSources" :key="s.id" :value="s.id">
            {{ s.name }}
          </option>
        </select>
      </div>

      <!-- param inputs -->
      <div v-if="entryDetails[entry.id]?.parameters?.length"
           class="try-params">
        <div v-for="p in entryDetails[entry.id].parameters" :key="p.name"
             class="try-row">
          <label class="try-lbl try-lbl-mono">{{ p.name }}<span v-if="p.required" class="required-mark">*</span></label>
          <input class="try-input"
            :placeholder="p.description || p.type || ''"
            :value="tryParams[entry.id]?.[p.name] ?? ''"
            @input="setTryParam(entry.id, p.name, ($event.target as HTMLInputElement).value)" />
        </div>
      </div>

      <!-- send button -->
      <div class="try-actions">
        <button class="btn btn-primary btn-sm"
          :disabled="!trySourceId[entry.id] || tryLoading.has(entry.id)"
          @click.stop="sendTry(entry)">
          {{ tryLoading.has(entry.id) ? '发送中…' : '发送' }}
        </button>
      </div>

      <!-- error -->
      <div v-if="tryError[entry.id]" class="try-error">{{ tryError[entry.id] }}</div>

      <!-- result -->
      <div v-if="tryResult[entry.id]" class="try-result">
        <div class="try-result-meta">
          <span :class="tryResult[entry.id]!.status < 400 ? 'try-status-ok' : 'try-status-err'">
            {{ tryResult[entry.id]!.status }}
          </span>
          <span class="try-latency">{{ tryResult[entry.id]!.latency_ms }}ms</span>
        </div>
        <pre class="resp-body"><code>{{ formatTryBody(tryResult[entry.id]!.body) }}</code></pre>
      </div>
    </template>
  </div>
</div>
```

- [ ] **Step 5: 追加 formatTryBody 辅助函数**

在 `formatDate` 函数之后追加：

```typescript
function formatTryBody(body: string): string {
  try { return JSON.stringify(JSON.parse(body), null, 2) } catch { return body }
}
```

### 4c: Try Panel 样式

- [ ] **Step 6: 追加 CSS**

在最后一个 `<style scoped>` 块末尾追加：

```css
/* Try panel */
.try-panel {
  margin-top: 12px;
  border-top: 1px solid var(--border);
  padding-top: 10px;
}
.try-header {
  display: flex; align-items: center; justify-content: space-between;
  cursor: pointer; padding: 2px 0;
}
.try-label {
  font-size: 11px; font-weight: 700; color: var(--primary);
  letter-spacing: .5px; text-transform: uppercase;
}
.try-chevron { font-size: 10px; color: var(--muted); }
.try-body { margin-top: 10px; display: flex; flex-direction: column; gap: 8px; }
.try-empty { font-size: 12px; color: var(--muted); padding: 4px 0; }
.try-row { display: flex; align-items: center; gap: 8px; }
.try-lbl {
  font-size: 11px; color: var(--text-sub); white-space: nowrap;
  min-width: 60px; flex-shrink: 0;
}
.try-lbl-mono { font-family: ui-monospace, monospace; }
.try-select, .try-input {
  flex: 1; background: var(--surface); border: 1px solid var(--border);
  border-radius: 4px; padding: 5px 8px; font-size: 12px; color: var(--text);
}
.try-input { font-family: ui-monospace, monospace; }
.try-params { display: flex; flex-direction: column; gap: 6px; }
.try-actions { display: flex; justify-content: flex-end; }
.try-error { font-size: 12px; color: #dc2626; }
.try-result-meta {
  display: flex; align-items: center; gap: 8px;
  padding: 5px 10px; background: var(--surface);
  border: 1px solid var(--border); border-radius: 4px 4px 0 0;
  border-bottom: none; font-size: 11px; font-weight: 700;
}
.try-status-ok { color: #10b981; }
.try-status-err { color: #dc2626; }
.try-latency { color: var(--muted); font-weight: 400; }
.try-result .resp-body { border-radius: 0 0 4px 4px; margin-top: 0; }
```

- [ ] **Step 7: 前端构建确认无错误**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

期望：`built in` 成功，无 TypeScript 错误

- [ ] **Step 8: Commit**

```bash
git add web/src/views/KnowledgeView.vue web/src/api/knowledge.ts
git commit -m "feat(ui): add try-it panel to knowledge entry cards"
```

---

## Task 5: 端到端验证

- [ ] **Step 1: 运行全部后端测试**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/api/ -v 2>&1 | tail -30
```

期望：所有测试 PASS，包括 `TestTryKnowledge*`

- [ ] **Step 2: 启动服务**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: 手动验证**

1. 打开 http://localhost:8002，登录（admin / 12345qwer）
2. 进入「知识库」，选择含 API 文档的文档
3. 展开任意 entry card
4. 点击「试一试 ▼」展开 try panel
5. 选择数据源
6. 填写参数（如 query=up）
7. 点击「发送」
8. 确认响应区显示状态码 + 耗时 + JSON 响应体

- [ ] **Step 4: 验证无数据源时的提示**

若系统无 Prometheus 数据源，try panel 应显示「暂无数据源，请先在系统设置中添加 Prometheus 数据源」。

- [ ] **Step 5: 最终 commit（如有遗漏文件）**

```bash
git status
# 确认无遗漏，若有则 git add + commit
```
