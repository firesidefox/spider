# KB No-Embedding Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow importing documents without Embedding, and let Agent browse a group catalog then fetch full content via `doc_ids`.

**Architecture:** Three changes: (1) `ingestDocument` API accepts `use_embedding=false` to skip embedder and store raw content; (2) `SearchDocsTool` gains a `catalog` branch that returns `{id, title, source_file}` list for a group; (3) frontend import form adds a checkbox to toggle embedding.

**Tech Stack:** Go (backend), Vue 3 + TypeScript (frontend), SQLite

---

### Task 1: Backend — `ingestDocument` API supports `use_embedding=false`

**Files:**
- Modify: `internal/api/documents.go`

- [ ] **Step 1: Write failing test**

Add to `internal/api/documents.go` test file — but this handler is integration-tested via the store. Instead, verify the existing build passes first:

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 2: Modify `ingestDocument` to accept `use_embedding`**

In `internal/api/documents.go`, replace the `ingestDocument` function body:

```go
func ingestDocument(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Vendor       string   `json:"vendor"`
		Tags         []string `json:"tags"`
		Title        string   `json:"title"`
		Content      string   `json:"content"`
		SourceFile   string   `json:"source_file"`
		ChunkIndex   int      `json:"chunk_index"`
		GroupID      *int     `json:"group_id"`
		UseEmbedding *bool    `json:"use_embedding"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	useEmbed := req.UseEmbedding == nil || *req.UseEmbedding
	if useEmbed {
		rs, err := ragStore(app)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "embedding unavailable: "+err.Error())
			return
		}
		if err := rs.Ingest(r.Context(), req.Vendor, req.Tags, req.Title, req.Content, req.SourceFile, req.ChunkIndex, req.GroupID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		if err := app.DocStore.Save(req.Vendor, req.Tags, req.Title, req.Content, nil, req.SourceFile, req.ChunkIndex, req.GroupID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
}
```

- [ ] **Step 3: Build and verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/api/documents.go
git commit -m "feat(api): ingestDocument supports use_embedding=false"
```

---

### Task 2: Agent — `SearchDocsTool` catalog branch

**Files:**
- Modify: `internal/agent/tools_docs.go`
- Modify: `internal/agent/tools_docs_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/tools_docs_test.go`:

```go
func TestSearchDocsTool_Schema_HasCatalogAndGroupID(t *testing.T) {
	tool := NewSearchDocsTool(nil, nil)
	props, ok := tool.InputSchema()["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"catalog", "group_id"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/agent/ -run TestSearchDocsTool_Schema_HasCatalogAndGroupID -v
```

Expected: FAIL — `schema missing property "catalog"`

- [ ] **Step 3: Add `catalog` and `group_id` to `InputSchema()`**

In `internal/agent/tools_docs.go`, update `InputSchema()`:

```go
func (t *SearchDocsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":     map[string]any{"type": "string", "description": "Search query"},
			"vendor":    map[string]any{"type": "string", "description": "Device vendor (e.g. huawei, cisco)"},
			"group_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Search within these document groups. Get from face.knowledge_sources where type=group."},
			"doc_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Fetch full content of specific documents by IDs. Get from face.knowledge_sources where type=doc."},
			"catalog":   map[string]any{"type": "boolean", "description": "List document titles in a group without fetching full content. Use with group_id to browse available documents before deciding which to read."},
			"group_id":  map[string]any{"type": "integer", "description": "Group ID to list when catalog=true."},
		},
		"required": []string{"query"},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/agent/ -run TestSearchDocsTool_Schema_HasCatalogAndGroupID -v
```

Expected: PASS

- [ ] **Step 5: Write failing test for catalog execution**

Add to `internal/agent/tools_docs_test.go`:

```go
func TestSearchDocsTool_Catalog(t *testing.T) {
	database := openTestDB(t)
	ds := store.NewDocumentStore(database)
	gid := 1
	ds.Save("v", nil, "doc-alpha", "full content alpha", nil, "a.md", 0, &gid)
	ds.Save("v", nil, "doc-beta", "full content beta", nil, "b.md", 0, &gid)

	tool := NewSearchDocsTool(nil, ds)
	result, err := tool.Execute(context.Background(), map[string]any{
		"query":    "",
		"catalog":  true,
		"group_id": float64(gid),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(result.Content), &items); err != nil {
		t.Fatalf("unmarshal: %v — content: %s", err, result.Content)
	}
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}
	for _, item := range items {
		if _, ok := item["id"]; !ok {
			t.Error("item missing id")
		}
		if _, ok := item["title"]; !ok {
			t.Error("item missing title")
		}
		if _, ok := item["content"]; ok {
			t.Error("catalog must not include content")
		}
	}
}
```

Also add helper to `internal/agent/tools_docs_test.go` (import `database/sql`, `testing`, `github.com/spiderai/spider/internal/db`):

```go
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
go test ./internal/agent/ -run TestSearchDocsTool_Catalog -v
```

Expected: FAIL — catalog branch not implemented yet.

- [ ] **Step 7: Implement catalog branch in `Execute()`**

In `internal/agent/tools_docs.go`, add catalog branch at the top of `Execute()`, before the `doc_ids` check:

```go
func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// catalog branch: list titles in a group
	if catalog, _ := input["catalog"].(bool); catalog {
		if t.docStore == nil {
			return &ToolResult{Content: "doc store unavailable", IsError: true, RiskLevel: RiskL1}, nil
		}
		groupID := toInt(input["group_id"])
		docs, err := t.docStore.ListByGroup(groupID)
		if err != nil {
			return &ToolResult{Content: fmt.Sprintf("list group: %v", err), IsError: true, RiskLevel: RiskL1}, nil
		}
		type entry struct {
			ID         int    `json:"id"`
			Title      string `json:"title"`
			SourceFile string `json:"source_file"`
		}
		entries := make([]entry, len(docs))
		for i, d := range docs {
			entries[i] = entry{ID: d.ID, Title: d.Title, SourceFile: d.SourceFile}
		}
		b, _ := json.Marshal(entries)
		return &ToolResult{Content: string(b), RiskLevel: RiskL1}, nil
	}

	query, _ := input["query"].(string)
	// ... rest of existing Execute body unchanged
```

- [ ] **Step 8: Run all agent tests**

```bash
go test ./internal/agent/ -v
```

Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add internal/agent/tools_docs.go internal/agent/tools_docs_test.go
git commit -m "feat(agent): SearchDocsTool catalog branch for no-embedding docs"
```

---

### Task 3: Agent — update `SystemPromptSection` to document catalog flow

**Files:**
- Modify: `internal/agent/tools_docs.go`

- [ ] **Step 1: Update `SystemPromptSection()`**

Replace the `SystemPromptSection()` body in `internal/agent/tools_docs.go`:

```go
func (t *SearchDocsTool) SystemPromptSection() string {
	return `## SearchDocs — Knowledge Base

**When to use:** Before running any command on a host, search the knowledge base first to find the correct CLI syntax, expected output, or known caveats for that host/vendor.

**When NOT to use:** Skip only if the task is purely informational (e.g., listing hosts) and involves no command execution.

**Rules:**
- SearchDocs comes before RunCommand in Explore phase. Do not run a command without first checking if relevant docs exist.
- Query with operation intent, not just keywords (e.g., "huawei 查看内存占用" not "memory").

**For full-text documents (no embedding):**
1. Call SearchDocs with catalog=true and group_id to list available documents (ID + title).
2. Pick relevant documents by ID.
3. Call SearchDocs with doc_ids=[...] to fetch full content.`
}
```

- [ ] **Step 2: Build and test**

```bash
go build ./... && go test ./internal/agent/ -v
```

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add internal/agent/tools_docs.go
git commit -m "docs(agent): SearchDocsTool system prompt documents catalog flow"
```

---

### Task 4: Frontend — `use_embedding` toggle in import form

**Files:**
- Modify: `web/src/api/documents.ts`
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: Add `use_embedding` to `IngestRequest`**

In `web/src/api/documents.ts`, update `IngestRequest`:

```ts
export interface IngestRequest {
  vendor: string
  content: string
  source_file: string
  chunk_index: number
  group_id?: number | null
  use_embedding?: boolean
}
```

- [ ] **Step 2: Update `emptyForm` and add `useEmbedding` ref**

In `web/src/views/KnowledgeView.vue`, update `emptyForm` and add ref:

```ts
const emptyForm = () => ({ vendor: '', useEmbedding: true })
const form = ref(emptyForm())
```

- [ ] **Step 3: Add checkbox to import form template**

In `web/src/views/KnowledgeView.vue`, add after the 分组 row (before the error div):

```html
<div class="form-row">
  <label>
    <input type="checkbox" v-model="form.useEmbedding" />
    使用 Embedding（语义搜索，需配置 Embedding 模型）
  </label>
</div>
```

- [ ] **Step 4: Pass `use_embedding` in `doIngest()`**

In `web/src/views/KnowledgeView.vue`, update both `ingestDocument` calls inside `doIngest()`:

```ts
// PDF pages branch:
.map(({ p, i }) => ingestDocument({
  vendor: form.value.vendor,
  content: p,
  source_file: file.name,
  chunk_index: i,
  group_id: ingestGroupId.value,
  use_embedding: form.value.useEmbedding,
}))

// chunks branch:
chunks.map((c, i) => ingestDocument({
  vendor: form.value.vendor,
  content: c,
  source_file: file.name,
  chunk_index: i,
  group_id: ingestGroupId.value,
  use_embedding: form.value.useEmbedding,
}))
```

- [ ] **Step 5: Build frontend**

```bash
cd web && npm run build
```

Expected: build succeeds, no TypeScript errors.

- [ ] **Step 6: Start dev server and verify UI**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

Open http://localhost:8002, navigate to 知识库, click 导入, verify:
- Checkbox "使用 Embedding" is visible and checked by default
- Unchecking it and importing a file succeeds even without embedding model configured

- [ ] **Step 7: Commit**

```bash
git add web/src/api/documents.ts web/src/views/KnowledgeView.vue
git commit -m "feat(frontend): kb import form adds use_embedding toggle"
```

---

### Task 5: Build verification

- [ ] **Step 1: Full build**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
```

Expected: no errors.

- [ ] **Step 2: Run all tests**

```bash
go test ./...
```

Expected: all PASS

- [ ] **Step 3: Smoke test catalog flow**

Start server:
```bash
/tmp/spider-test serve --addr :8003 --data-dir ~/.spider/data
```

Import a doc without embedding via curl:
```bash
curl -s -X POST http://localhost:8003/api/v1/documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"vendor":"test","title":"test-doc","content":"hello world","source_file":"test.md","chunk_index":0,"group_id":1,"use_embedding":false}'
```

Expected: HTTP 201

Verify catalog via curl:
```bash
curl -s "http://localhost:8003/api/v1/documents?group_id=1" \
  -H "Authorization: Bearer <token>"
```

Expected: doc appears in list with `embedding` absent (null).
