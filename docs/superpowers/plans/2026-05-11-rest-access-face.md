# REST API 操作面优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Agent 能通过 face 绑定的知识库查询 API 文档，并修复 REST API face 表单字段动态显示问题。

**Architecture:** 后端在 `DocumentStore` 加 `GetByID`，在 `rag.Store` 加 `SearchByGroups`，扩展 `SearchDocsTool` 加 `group_ids`/`doc_ids` 数组参数并注册进 `buildRegistry`；前端修复 REST face 表单字段按 auth type 动态显示，并扩展知识库绑定 UI 支持三种模式（全局/文档组/具体文档）。

**Tech Stack:** Go 1.21, Vue 3 (Composition API), SQLite

---

## 现状说明

- `SearchDocsTool` 存在于 `internal/agent/tools_docs.go`，但**未注册进 `buildRegistry`**，Agent 无法使用
- `ragStore` 通过 `app.GetOrBuildRagStore()` 按需构造，不是 `Factory` 字段
- `Factory` 有 `DocStore` 字段（`*store.DocumentStore`）但未使用
- `NewAgentFactory()` 在 `internal/mcp/server.go:77`，目前不传 ragStore 给 Factory
- 前端 `faceForm` 已有 `header_name` 字段，但模板未按 auth type 动态显示
- 前端知识库绑定只支持文档组（group），不支持具体文档（doc）或全局模式

---

## 文件变更清单

| 文件 | 操作 |
|------|------|
| `internal/store/document.go` | 新增 `GetByID` 方法 |
| `internal/rag/store.go` | 新增 `SearchByGroups` 方法（多 group_id 合并搜索） |
| `internal/agent/tools_docs.go` | 加 `docStore` 依赖、`group_ids`/`doc_ids` 数组参数、`toInt`/`toIntSlice` helper |
| `internal/agent/tools_docs_test.go` | 更新测试覆盖新参数和新构造函数签名 |
| `internal/agent/factory.go` | 加 `RagStore *rag.Store` 字段，`buildRegistry` 注册 `SearchDocsTool` |
| `internal/mcp/server.go` | `NewAgentFactory` 传入 ragStore 和 docStore |
| `web/src/views/HostsView.vue` | REST face 表单动态字段 + 知识库绑定三种模式 |

---

## Task 1: DocumentStore.GetByID + rag.Store.SearchByGroups

**Files:**
- Modify: `internal/store/document.go`
- Modify: `internal/rag/store.go`

- [ ] **Step 1: 在 DocumentStore 加 GetByID**

在 `internal/store/document.go` 的 `Delete` 方法（第 85-88 行）后插入：

```go
func (s *DocumentStore) GetByID(id int) (*models.Document, error) {
	row := s.db.QueryRow(
		"SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents WHERE id = ?",
		id,
	)
	var d models.Document
	var tagsJSON string
	err := row.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document by id: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
		d.Tags = []string{}
	}
	return &d, nil
}
```

- [ ] **Step 2: 在 rag.Store 加 SearchByGroups**

在 `internal/rag/store.go` 的 `SearchByGroup` 方法（第 75-96 行）后插入：

```go
// SearchByGroups 在多个 group 中搜索，合并结果去重后按相似度排序。
func (s *Store) SearchByGroups(ctx context.Context, query string, groupIDs []int, topK int) ([]*models.Document, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}
	if len(groupIDs) == 0 {
		return s.SearchByGroup(ctx, query, nil, topK)
	}
	if len(groupIDs) == 1 {
		return s.SearchByGroup(ctx, query, &groupIDs[0], topK)
	}

	qvec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	placeholders := make([]string, len(groupIDs))
	args := make([]any, len(groupIDs))
	for i, gid := range groupIDs {
		placeholders[i] = "?"
		args[i] = gid
	}
	q := "SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE embedding IS NOT NULL AND group_id IN (" + 
		strings.Join(placeholders, ",") + ")"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()
	return s.rankFromRows(rows, qvec, topK)
}
```

注意：需要在 import 里加 `"strings"`。

- [ ] **Step 3: 编译验证**

```bash
GOCACHE=/Users/cw/fty.ai/spider.ai/.gocache go build ./internal/store/ ./internal/rag/
```

期望：无错误。

---

## Task 2: SearchDocsTool 扩展

**Files:**
- Modify: `internal/agent/tools_docs.go`
- Modify: `internal/agent/tools_docs_test.go`

- [ ] **Step 1: 替换 tools_docs.go 全文**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/store"
)

type SearchDocsTool struct {
	ragStore *rag.Store
	docStore *store.DocumentStore
}

func NewSearchDocsTool(ragStore *rag.Store, docStore *store.DocumentStore) *SearchDocsTool {
	return &SearchDocsTool{ragStore: ragStore, docStore: docStore}
}

func (t *SearchDocsTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *SearchDocsTool) Name() string                { return "SearchDocs" }

func (t *SearchDocsTool) Description() string {
	return "Search documentation for CLI commands, API references, and troubleshooting guides. Read-only. No side effects. Use freely in Explore phase."
}

func (t *SearchDocsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":     map[string]any{"type": "string", "description": "Search query"},
			"vendor":    map[string]any{"type": "string", "description": "Device vendor (e.g. huawei, cisco)"},
			"group_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Search within these document groups. Get from face.knowledge_sources where type=group."},
			"doc_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Fetch full content of specific documents by IDs. Get from face.knowledge_sources where type=doc."},
		},
		"required": []string{"query"},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return &ToolResult{Content: "query is required", IsError: true, RiskLevel: RiskL1}, nil
	}

	// doc_ids: fetch multiple documents, skip vector search
	if docIDsRaw, ok := input["doc_ids"]; ok && docIDsRaw != nil {
		docIDs := toIntSlice(docIDsRaw)
		if len(docIDs) > 0 && t.docStore != nil {
			type result struct {
				Title      string   `json:"title"`
				Content    string   `json:"content"`
				Tags       []string `json:"tags"`
				SourceFile string   `json:"source_file"`
			}
			results := make([]result, 0, len(docIDs))
			for _, id := range docIDs {
				doc, err := t.docStore.GetByID(id)
				if err != nil {
					return &ToolResult{Content: fmt.Sprintf("get document %d: %v", id, err), IsError: true, RiskLevel: RiskL1}, nil
				}
				if doc == nil {
					continue
				}
				results = append(results, result{
					Title:      doc.Title,
					Content:    doc.Content,
					Tags:       doc.Tags,
					SourceFile: doc.SourceFile,
				})
			}
			b, _ := json.Marshal(results)
			return &ToolResult{Content: string(b), RiskLevel: RiskL1}, nil
		}
	}

	if t.ragStore == nil {
		return &ToolResult{Content: "search unavailable: embedding not configured", IsError: true, RiskLevel: RiskL1}, nil
	}

	// group_ids: search within multiple groups
	var groupIDs []int
	if gidsRaw, ok := input["group_ids"]; ok && gidsRaw != nil {
		groupIDs = toIntSlice(gidsRaw)
	}

	docs, err := t.ragStore.SearchByGroups(ctx, query, groupIDs, 5)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("search error: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		Title      string   `json:"title"`
		Content    string   `json:"content"`
		Tags       []string `json:"tags"`
		SourceFile string   `json:"source_file"`
	}
	results := make([]result, 0, len(docs))
	for _, d := range docs {
		results = append(results, result{
			Title:      d.Title,
			Content:    d.Content,
			Tags:       d.Tags,
			SourceFile: d.SourceFile,
		})
	}
	b, err := json.Marshal(results)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("marshal error: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}
	return &ToolResult{Content: string(b), RiskLevel: RiskL1}, nil
}

// toInt converts float64 (JSON number) or int to int.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// toIntSlice converts []any (JSON array) to []int.
func toIntSlice(v any) []int {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]int, 0, len(arr))
	for _, item := range arr {
		if n := toInt(item); n > 0 {
			result = append(result, n)
		}
	}
	return result
}
```

- [ ] **Step 2: 替换 tools_docs_test.go 全文**

```go
package agent

import (
	"testing"
)

func TestSearchDocsTool_Metadata(t *testing.T) {
	tool := NewSearchDocsTool(nil, nil)

	if tool.Name() != "SearchDocs" {
		t.Errorf("got name %q, want %q", tool.Name(), "SearchDocs")
	}
	if tool.Description() == "" {
		t.Error("description must not be empty")
	}

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"query", "vendor", "group_ids", "doc_ids"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
	req, _ := schema["required"].([]string)
	if len(req) != 1 || req[0] != "query" {
		t.Errorf("required = %v, want [query]", req)
	}
}

func TestSearchDocsTool_ImplementsTool(t *testing.T) {
	var _ Tool = NewSearchDocsTool(nil, nil)
}

func TestCallRESTAPITool_Metadata(t *testing.T) {
	tool := NewCallRESTAPITool(nil)

	if tool.Name() != "CallAPI" {
		t.Errorf("got name %q, want %q", tool.Name(), "CallAPI")
	}
	if tool.Description() == "" {
		t.Error("description must not be empty")
	}

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"url", "method", "headers", "body"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
}

func TestCallRESTAPITool_ImplementsTool(t *testing.T) {
	var _ Tool = NewCallRESTAPITool(nil)
}
```

- [ ] **Step 3: 运行测试**

```bash
GOCACHE=/Users/cw/fty.ai/spider.ai/.gocache go test ./internal/agent/ -run "TestSearchDocsTool|TestCallRESTAPITool" -v
```

期望：全部 PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/store/document.go internal/rag/store.go internal/agent/tools_docs.go internal/agent/tools_docs_test.go
git commit -m "feat(agent): SearchDocs supports group_ids and doc_ids arrays; DocumentStore.GetByID and rag.Store.SearchByGroups added"
```

---

## Task 3: 注册 SearchDocsTool 进 Factory

**Files:**
- Modify: `internal/agent/factory.go`
- Modify: `internal/mcp/server.go`

**背景：**
- `Factory` 已有 `DocStore *store.DocumentStore` 字段（未使用）
- `ragStore` 通过 `app.GetOrBuildRagStore()` 按需构造，需要加到 `Factory`
- `buildRegistry` 目前不注册 `SearchDocsTool`

- [ ] **Step 1: Factory 加 RagStore 字段**

在 `internal/agent/factory.go` 的 `Factory` 结构体（第 17-33 行）中，`DocStore` 字段后加：

```go
RagStore  *rag.Store
```

同时确认 import 里有 `"github.com/spiderai/spider/internal/rag"`，如没有则加上。

- [ ] **Step 2: buildRegistry 注册 SearchDocsTool**

在 `buildRegistry`（第 191-206 行）的 `NewCallRESTAPITool` 注册行后加：

```go
registry.Register(NewSearchDocsTool(f.RagStore, f.DocStore))
```

- [ ] **Step 3: NewAgentFactory 传入 RagStore 和 DocStore**

在 `internal/mcp/server.go` 的 `NewAgentFactory`（第 77-91 行）中，`f.SSEBroadcaster = a` 后加：

```go
f.DocStore = a.DocStore
if rs, err := a.GetOrBuildRagStore(); err == nil {
    f.RagStore = rs
}
```

- [ ] **Step 4: 编译验证**

```bash
GOCACHE=/Users/cw/fty.ai/spider.ai/.gocache go build ./...
```

期望：无错误。

- [ ] **Step 5: 运行全量测试**

```bash
GOCACHE=/Users/cw/fty.ai/spider.ai/.gocache go test ./internal/... -v 2>&1 | tail -20
```

期望：全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/agent/factory.go internal/mcp/server.go
git commit -m "feat(agent): register SearchDocsTool in buildRegistry; wire RagStore and DocStore via factory"
```

---

## Task 4: 后端 — Update 按 auth type 清空无关字段

**Files:**
- Modify: `internal/store/access_face_store.go`

**背景：**
`Update` 是 patch 语义，`RESTAuthType` 切换时不会自动清空 `RESTUsername`、`HeaderName`。用户切换 auth type 保存后，旧字段残留在 DB。

- [ ] **Step 1: 修改 Update 方法**

在 `internal/store/access_face_store.go` 的 `Update` 方法中，找到：

```go
if req.RESTAuthType != nil {
    cur.RESTAuthType = *req.RESTAuthType
}
```

替换为：

```go
if req.RESTAuthType != nil {
    cur.RESTAuthType = *req.RESTAuthType
    switch cur.RESTAuthType {
    case "bearer", "none":
        cur.RESTUsername = ""
        cur.HeaderName = ""
    case "basic":
        cur.HeaderName = ""
    case "apikey":
        cur.RESTUsername = ""
    }
}
```

- [ ] **Step 2: 编译验证**

```bash
GOCACHE=/Users/cw/fty.ai/spider.ai/.gocache go build ./internal/store/
```

期望：无错误。

- [ ] **Step 3: Commit**

```bash
git add internal/store/access_face_store.go
git commit -m "fix(store): clear irrelevant REST auth fields on auth type change"
```

---

## Task 5: 前端 — REST face 表单动态字段

**Files:**
- Modify: `web/src/views/HostsView.vue`（template 第 262-274 行）

**背景：**
- `faceForm` 已有 `header_name` 字段（第 368 行）
- `submitFace` 已提交 `header_name`（第 500 行）
- 问题：模板里 `rest_username` 和 `credential` 对所有 auth type 无条件显示，`header_name` 完全没有显示

- [ ] **Step 1: 替换 REST API 表单字段区块**

找到第 262-274 行的 `<template v-if="faceForm.type === 'restapi'">` 块，替换为：

```vue
<template v-if="faceForm.type === 'restapi'">
  <div class="form-row">
    <label>Base URL</label>
    <input v-model="faceForm.base_url" class="input" placeholder="http://192.168.1.1:8080" />
  </div>
  <div class="form-row">
    <label>认证方式</label>
    <select v-model="faceForm.rest_auth_type" class="input" @change="onRestAuthTypeChange">
      <option value="none">无</option>
      <option value="bearer">Bearer Token</option>
      <option value="basic">Basic</option>
      <option value="apikey">API Key</option>
    </select>
  </div>
  <template v-if="faceForm.rest_auth_type === 'basic'">
    <div class="form-row"><label>用户名</label><input v-model="faceForm.rest_username" class="input" /></div>
    <div class="form-row"><label>密码</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
  </template>
  <template v-if="faceForm.rest_auth_type === 'bearer'">
    <div class="form-row"><label>Token</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
  </template>
  <template v-if="faceForm.rest_auth_type === 'apikey'">
    <div class="form-row"><label>Header Name</label><input v-model="faceForm.header_name" class="input" placeholder="X-API-Key" /></div>
    <div class="form-row"><label>API Key</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
  </template>
</template>
```

- [ ] **Step 2: 在 script 加 onRestAuthTypeChange（空函数，仅供 @change 绑定）**

在 `toggleFormKnowledgeSource` 函数（第 478 行）前加：

```ts
function onRestAuthTypeChange() {
  // 无需前端清空；后端 Update 按 auth type 清空无关字段
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/views/HostsView.vue
git commit -m "feat(web): REST face form — dynamic fields by auth type, add header_name"
```

---

## Task 6: 前端 — 知识库绑定三种模式

**Files:**
- Modify: `web/src/views/HostsView.vue`
- Modify: `web/src/api/documents.ts`（确认 `listDocuments` 已有）

**背景：**
- 当前知识库绑定只支持文档组（group），用 checkbox 列表
- 需要支持三种模式：全局（`[]`）/ 文档组（`{type:'group'}`）/ 具体文档（`{type:'doc'}`）
- `listDocuments()` 已存在于 `web/src/api/documents.ts`
- `listDocumentsByGroup` 也已存在

- [ ] **Step 1: 加 ksMode ref 和 allDocs ref**

在 `HostsView.vue` script 中，`docGroups` ref 声明后加：

```ts
const ksMode = ref<'global' | 'group' | 'doc'>('global')
const allDocs = ref<import('../api/documents').Document[]>([])
```

- [ ] **Step 2: 加载 allDocs**

在 `onMounted` 里已有 `listGroups()` 调用，同行加载 allDocs：

```ts
const [hostsData, keysData, groupsData, docsData] = await Promise.all([
  listHosts(),
  listSSHKeys(),
  listGroups(),
  listDocuments(),
])
hosts.value = hostsData
sshKeys.value = keysData
docGroups.value = groupsData
allDocs.value = docsData
```

注意：需要在 import 里加 `listDocuments` 和 `type Document`（从 `../api/documents`）。

- [ ] **Step 3: 加 ksMode 初始化逻辑**

在 `startEditFace` 函数里（找到设置 `faceForm` 的地方），加初始化 ksMode：

```ts
const ks = f.knowledge_sources ?? []
if (ks.length === 0) {
  ksMode.value = 'global'
} else if (ks[0].type === 'doc') {
  ksMode.value = 'doc'
} else {
  ksMode.value = 'group'
}
```

在 `openAddFace` 函数里加：

```ts
ksMode.value = 'global'
```

- [ ] **Step 4: 替换知识库绑定 UI**

找到第 275-284 行的知识库绑定区块（`<div v-if="docGroups.length > 0" class="form-row">`），替换为：

```vue
<div class="form-row">
  <label>知识来源</label>
  <div class="ks-mode-tabs">
    <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'global' }" @click="setKsMode('global')">全局</button>
    <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'group' }" @click="setKsMode('group')">文档组</button>
    <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'doc' }" @click="setKsMode('doc')">具体文档</button>
  </div>
  <div v-if="ksMode === 'group' && docGroups.length > 0" class="ks-checkboxes">
    <label v-for="g in docGroups" :key="g.id" class="checkbox-label">
      <input type="checkbox"
        :checked="faceForm.knowledge_sources.some(k => k.type === 'group' && k.id === g.id)"
        @change="toggleKs('group', g.id)" />
      {{ g.name }}
    </label>
  </div>
  <div v-if="ksMode === 'doc' && allDocs.length > 0" class="ks-checkboxes">
    <label v-for="d in allDocs" :key="d.id" class="checkbox-label">
      <input type="checkbox"
        :checked="faceForm.knowledge_sources.some(k => k.type === 'doc' && k.id === d.id)"
        @change="toggleKs('doc', d.id)" />
      {{ d.title || d.source_file }}
    </label>
  </div>
</div>
```

- [ ] **Step 5: 加 setKsMode 和 toggleKs 函数**

替换现有 `toggleFormKnowledgeSource` 函数，加新函数：

```ts
function setKsMode(mode: 'global' | 'group' | 'doc') {
  ksMode.value = mode
  faceForm.value.knowledge_sources = []
}

function toggleKs(type: 'group' | 'doc', id: number) {
  const ks = faceForm.value.knowledge_sources
  const exists = ks.some(k => k.type === type && k.id === id)
  faceForm.value.knowledge_sources = exists
    ? ks.filter(k => !(k.type === type && k.id === id))
    : [...ks, { type, id }]
}
```

- [ ] **Step 6: 加 ks-mode-tabs 样式**

在 HostsView.vue `<style>` 末尾加：

```css
.ks-mode-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 8px;
}
.ks-mode-tabs .btn.active {
  background: var(--accent);
  color: #fff;
}
```

- [ ] **Step 7: Commit**

```bash
git add web/src/views/HostsView.vue
git commit -m "feat(web): knowledge source binding supports global/group/doc modes"
```

---

## Task 7: 端到端验证

- [ ] **Step 1: 启动开发服务器**

```bash
cd /Users/cw/fty.ai/spider.ai
make dev   # 或查看 Makefile/package.json 确认启动命令
```

- [ ] **Step 2: 验证 REST face 表单**

打开主机管理，选一台主机，添加 REST API 操作面：
- 切换认证方式为 `apikey`，确认出现 Header Name 字段
- 切换为 `bearer`，确认只有 Token 字段，Header Name 消失
- 切换为 `basic`，确认出现用户名 + 密码字段
- 切换认证方式时，确认之前填的凭据被清空

- [ ] **Step 3: 验证知识库绑定**

在同一 face 表单，切换知识库模式：
- 全局：提交后 `knowledge_sources = []`
- 文档组：选一个 group，提交后 face 卡片显示正确
- 具体文档：选一个 doc，提交后 face 卡片显示正确
- 切换模式时，之前的选择被清空

- [ ] **Step 4: 验证 SearchDocs 工具**

通过 Chat 界面，对话中让 Agent 查询某台有 REST face 的设备文档：
- Agent 应调用 `GetDeviceInfo` 拿到 `face.knowledge_sources`
- Agent 应调用 `SearchDocs` 并传入对应 `group_id` 或 `doc_id`
- 返回结果应包含文档内容

- [ ] **Step 5: 最终 commit（如有遗漏）**

```bash
git add -A
git commit -m "chore: REST API access face optimization complete"
```
