# Knowledge Base HTTP API + Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add REST API endpoints for KB/Group/Document CRUD and a new import pipeline, then rewrite KnowledgeView.vue to use the KB→Group→Document hierarchy with a new import UI.

**Architecture:** New `internal/api/knowledge.go` handles all KB/Group/Document endpoints. Import endpoint calls `knowledge.Store.ImportDocument` with an LLM client obtained from the provider store. Frontend `KnowledgeView.vue` is rewritten to navigate KB→Group→Document with a drag-and-drop multi-file import modal.

**Tech Stack:** Go net/http, SQLite via `knowledge.Store`, Vue 3 Composition API, TypeScript

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/api/knowledge.go` | Create | All KB/Group/Document/Import handlers |
| `internal/api/handler.go` | Modify | Register new routes |
| `web/src/api/knowledge.ts` | Create | Frontend API client for new endpoints |
| `web/src/views/KnowledgeView.vue` | Rewrite | KB→Group→Document hierarchy UI + import modal |

---

### Task 1: Backend — KB and Group CRUD handlers

**Files:**
- Create: `internal/api/knowledge.go`

- [ ] **Step 1: Write failing test**

Create `internal/api/knowledge_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
)

type mockKBStore struct {
	kbs    []knowledge.KnowledgeBase
	groups []knowledge.Group
}

func (m *mockKBStore) CreateKB(ctx context.Context, name string) (*knowledge.KnowledgeBase, error) {
	kb := knowledge.KnowledgeBase{ID: len(m.kbs) + 1, Name: name}
	m.kbs = append(m.kbs, kb)
	return &kb, nil
}
func (m *mockKBStore) ListKBs(ctx context.Context) ([]knowledge.KnowledgeBase, error) {
	return m.kbs, nil
}
func (m *mockKBStore) DeleteKB(ctx context.Context, kbID int) error {
	for i, kb := range m.kbs {
		if kb.ID == kbID {
			m.kbs = append(m.kbs[:i], m.kbs[i+1:]...)
			return nil
		}
	}
	return nil
}
func (m *mockKBStore) CreateGroup(ctx context.Context, kbID int, name string) (*knowledge.Group, error) {
	g := knowledge.Group{ID: len(m.groups) + 1, KBID: kbID, Name: name}
	m.groups = append(m.groups, g)
	return &g, nil
}
func (m *mockKBStore) ListGroups(ctx context.Context, kbID int) ([]knowledge.Group, error) {
	var out []knowledge.Group
	for _, g := range m.groups {
		if g.KBID == kbID {
			out = append(out, g)
		}
	}
	return out, nil
}
func (m *mockKBStore) DeleteGroup(ctx context.Context, groupID int) error { return nil }

func TestListKBs(t *testing.T) {
	store := &mockKBStore{kbs: []knowledge.KnowledgeBase{{ID: 1, Name: "AISG"}}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	listKBs(store, w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var result []knowledge.KnowledgeBase
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 || result[0].Name != "AISG" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestCreateKB(t *testing.T) {
	store := &mockKBStore{}
	body, _ := json.Marshal(map[string]string{"name": "F5"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases", bytes.NewReader(body))
	createKB(store, w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /Users/cw/fty.ai/spider.ai
go test ./internal/api/... -run TestListKBs -v 2>&1 | head -20
```
Expected: compile error — `listKBs` undefined

- [ ] **Step 3: Create `internal/api/knowledge.go` — KB handlers**

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/spiderai/spider/internal/knowledge"
)

// kbStore is the subset of knowledge.KnowledgePlugin used by KB/Group handlers.
type kbStore interface {
	CreateKB(ctx context.Context, name string) (*knowledge.KnowledgeBase, error)
	ListKBs(ctx context.Context) ([]knowledge.KnowledgeBase, error)
	DeleteKB(ctx context.Context, kbID int) error
	CreateGroup(ctx context.Context, kbID int, name string) (*knowledge.Group, error)
	ListGroups(ctx context.Context, kbID int) ([]knowledge.Group, error)
	DeleteGroup(ctx context.Context, groupID int) error
}

func listKBs(s kbStore, w http.ResponseWriter, r *http.Request) {
	kbs, err := s.ListKBs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kbs == nil {
		kbs = []knowledge.KnowledgeBase{}
	}
	writeJSON(w, http.StatusOK, kbs)
}

func createKB(s kbStore, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	kb, err := s.CreateKB(r.Context(), strings.TrimSpace(req.Name))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, kb)
}

func deleteKB(s kbStore, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.DeleteKB(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listKBGroups(s kbStore, w http.ResponseWriter, r *http.Request, kbIDStr string) {
	kbID, err := strconv.Atoi(kbIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kb_id")
		return
	}
	groups, err := s.ListGroups(r.Context(), kbID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []knowledge.Group{}
	}
	writeJSON(w, http.StatusOK, groups)
}

func createKBGroup(s kbStore, w http.ResponseWriter, r *http.Request, kbIDStr string) {
	kbID, err := strconv.Atoi(kbIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kb_id")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	g, err := s.CreateGroup(r.Context(), kbID, strings.TrimSpace(req.Name))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func deleteKBGroup(s kbStore, w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.DeleteGroup(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/api/... -run "TestListKBs|TestCreateKB" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/knowledge.go internal/api/knowledge_test.go
git commit -m "feat(knowledge): add KB and Group CRUD API handlers"
```

---

### Task 2: Backend — Document list/delete handlers + route registration

**Files:**
- Modify: `internal/api/knowledge.go`
- Modify: `internal/api/knowledge_test.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Write failing tests for document handlers**

Append to `internal/api/knowledge_test.go`:

```go
func (m *mockKBStore) ListDocuments(ctx context.Context, groupID int) ([]knowledge.Document, error) {
	return []knowledge.Document{{ID: 1, GroupID: groupID, Name: "test.yaml", DocType: "openapi", Status: "ready"}}, nil
}
func (m *mockKBStore) GetDocument(ctx context.Context, docID int) (*knowledge.Document, error) {
	return &knowledge.Document{ID: docID, Name: "test.yaml"}, nil
}
func (m *mockKBStore) DeleteDocuments(ctx context.Context, docIDs []int) error { return nil }

func TestListGroupDocuments(t *testing.T) {
	store := &mockKBStore{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-groups/1/documents", nil)
	listGroupDocuments(store, w, r, "1")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var docs []knowledge.Document
	json.NewDecoder(w.Body).Decode(&docs)
	if len(docs) != 1 {
		t.Fatalf("want 1 doc, got %d", len(docs))
	}
}

func TestDeleteDocuments(t *testing.T) {
	store := &mockKBStore{}
	body, _ := json.Marshal(map[string][]int{"ids": {1, 2}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/knowledge-documents", bytes.NewReader(body))
	deleteDocuments(store, w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/api/... -run "TestListGroupDocuments|TestDeleteDocuments" -v 2>&1 | head -20
```
Expected: compile error — `listGroupDocuments` undefined

- [ ] **Step 3: Add document handlers to `internal/api/knowledge.go`**

Append to `knowledge.go` (after `deleteKBGroup`):

```go
// docStore is the subset of knowledge.KnowledgePlugin used by document handlers.
type docStore interface {
	ListDocuments(ctx context.Context, groupID int) ([]knowledge.Document, error)
	GetDocument(ctx context.Context, docID int) (*knowledge.Document, error)
	DeleteDocuments(ctx context.Context, docIDs []int) error
}

func listGroupDocuments(s docStore, w http.ResponseWriter, r *http.Request, groupIDStr string) {
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group_id")
		return
	}
	docs, err := s.ListDocuments(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []knowledge.Document{}
	}
	writeJSON(w, http.StatusOK, docs)
}

func deleteDocuments(s docStore, w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if err := s.DeleteDocuments(r.Context(), req.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Register routes in `internal/api/handler.go`**

Add after the existing document routes (search for `mux.HandleFunc("/api/v1/documents"` block, add after it):

```go
// Knowledge bases
mux.HandleFunc("/api/v1/knowledge-bases", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        listKBs(app.KnowledgeStore, w, r)
    case http.MethodPost:
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            createKB(app.KnowledgeStore, w, r)
        })).ServeHTTP(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
mux.HandleFunc("/api/v1/knowledge-bases/", func(w http.ResponseWriter, r *http.Request) {
    rest := r.URL.Path[len("/api/v1/knowledge-bases/"):]
    id := rest
    sub := ""
    if idx := indexOf(rest, '/'); idx >= 0 {
        id = rest[:idx]
        sub = rest[idx+1:]
    }
    if sub == "" {
        if r.Method == http.MethodDelete {
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                deleteKB(app.KnowledgeStore, w, r, id)
            })).ServeHTTP(w, r)
            return
        }
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if sub == "groups" {
        switch r.Method {
        case http.MethodGet:
            listKBGroups(app.KnowledgeStore, w, r, id)
        case http.MethodPost:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                createKBGroup(app.KnowledgeStore, w, r, id)
            })).ServeHTTP(w, r)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    http.NotFound(w, r)
})
// Knowledge groups
mux.HandleFunc("/api/v1/knowledge-groups/", func(w http.ResponseWriter, r *http.Request) {
    rest := r.URL.Path[len("/api/v1/knowledge-groups/"):]
    id := rest
    sub := ""
    if idx := indexOf(rest, '/'); idx >= 0 {
        id = rest[:idx]
        sub = rest[idx+1:]
    }
    if sub == "" {
        if r.Method == http.MethodDelete {
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                deleteKBGroup(app.KnowledgeStore, w, r, id)
            })).ServeHTTP(w, r)
            return
        }
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if sub == "documents" {
        if r.Method == http.MethodGet {
            listGroupDocuments(app.KnowledgeStore, w, r, id)
            return
        }
    }
    http.NotFound(w, r)
})
// Knowledge documents
mux.HandleFunc("/api/v1/knowledge-documents", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodDelete {
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            deleteDocuments(app.KnowledgeStore, w, r)
        })).ServeHTTP(w, r)
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
```

- [ ] **Step 5: Run tests**

```
go test ./internal/api/... -run "TestListGroupDocuments|TestDeleteDocuments|TestListKBs|TestCreateKB" -v
```
Expected: all PASS

- [ ] **Step 6: Build check**

```
go build ./...
```
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/api/knowledge.go internal/api/knowledge_test.go internal/api/handler.go
git commit -m "feat(knowledge): add document handlers and register KB/Group/Document routes"
```

---

### Task 3: Backend — Import endpoint

**Files:**
- Modify: `internal/api/knowledge.go`
- Modify: `internal/api/handler.go`

The import endpoint receives a multipart form upload (file + group_id), detects doc type, calls `knowledge.Store.ImportDocument` with an LLM client from the provider store.

- [ ] **Step 1: Add `importDocument` handler to `internal/api/knowledge.go`**

First add the import-related interface and handler. Append after `deleteDocuments`:

```go
// importStore is the subset needed for the import endpoint.
type importStore interface {
	ImportDocument(ctx context.Context, req knowledge.ImportRequest) (*knowledge.ImportResult, error)
}

func importKnowledgeDocument(s importStore, app interface {
	NewAgentFactory() (interface{ LLMClient() interface{} }, error)
	GetOrBuildRagStore() (interface{ Embedder() interface{} }, error)
}, w http.ResponseWriter, r *http.Request) {
}
```

Wait — the handler needs the real `*mcppkg.App` to get LLM client and embedder. Use `*mcppkg.App` directly (same pattern as other handlers). Replace the above with:

```go
func importKnowledgeDocument(ks *knowledge.Store, app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	groupIDStr := r.FormValue("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil || groupID <= 0 {
		writeError(w, http.StatusBadRequest, "group_id is required")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}
	f, err := app.NewAgentFactory()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "LLM not configured: "+err.Error())
		return
	}
	rs, err := app.GetOrBuildRagStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "embedding not configured: "+err.Error())
		return
	}
	req := knowledge.ImportRequest{
		GroupID:   groupID,
		Name:      header.Filename,
		Content:   content,
		Filename:  header.Filename,
		LLMClient: f.LLMClient(),
		Embedder:  rs.Embedder(),
	}
	result, err := ks.ImportDocument(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
```

Add `"io"` to the imports in `knowledge.go`.

- [ ] **Step 2: Check what `Factory` and `rag.Store` expose**

`Factory` in `internal/agent/factory.go` — check if it has a `LLMClient()` method. If not, we need a different approach.

Run:
```
grep -n "func.*Factory.*LLMClient\|func.*LLMClient" /Users/cw/fty.ai/spider.ai/internal/agent/factory.go
grep -n "func.*Store.*Embedder\|func.*Embedder" /Users/cw/fty.ai/spider.ai/internal/rag/store.go
```

If `LLMClient()` doesn't exist on Factory, use `app.ProviderStore` to build an `llm.Client` directly (same way `NewAgentFactory` does internally). Check:
```
grep -n "llm.NewClient\|NewLLMClient\|llm.Client" /Users/cw/fty.ai/spider.ai/internal/agent/factory.go | head -10
```

- [ ] **Step 3: Wire import route in `internal/api/handler.go`**

Add after the `knowledge-documents` route block:

```go
mux.HandleFunc("/api/v1/knowledge-documents/import", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            importKnowledgeDocument(app.KnowledgeStore, app, w, r)
        })).ServeHTTP(w, r)
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
```

- [ ] **Step 4: Build check**

```
go build ./...
```

Fix any compile errors (missing imports, wrong method names). The key unknowns are `f.LLMClient()` and `rs.Embedder()` — adjust based on what Step 2 reveals.

- [ ] **Step 5: Commit**

```bash
git add internal/api/knowledge.go internal/api/handler.go
git commit -m "feat(knowledge): add import endpoint wired to ImportDocument pipeline"
```

---

### Task 4: Frontend API client

**Files:**
- Create: `web/src/api/knowledge.ts`

- [ ] **Step 1: Create `web/src/api/knowledge.ts`**

```typescript
import { authHeaders } from './auth'

export interface KnowledgeBase {
  id: number
  name: string
  created_at: string
}

export interface KnowledgeGroup {
  id: number
  kb_id: number
  name: string
  created_at: string
}

export interface KnowledgeDocument {
  id: number
  group_id: number
  name: string
  doc_type: string
  status: string
  error_msg: string
  entry_count: number
  created_at: string
  updated_at: string
}

export interface ImportResult {
  document_id: number
  entry_count: number
  section_count: number
}

const BASE = '/api/v1'

export async function listKBs(): Promise<KnowledgeBase[]> {
  const r = await fetch(`${BASE}/knowledge-bases`, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function createKB(name: string): Promise<KnowledgeBase> {
  const r = await fetch(`${BASE}/knowledge-bases`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteKB(id: number): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-bases/${id}`, {
    method: 'DELETE', headers: authHeaders(),
  })
  if (!r.ok) throw new Error(await r.text())
}

export async function listGroups(kbID: number): Promise<KnowledgeGroup[]> {
  const r = await fetch(`${BASE}/knowledge-bases/${kbID}/groups`, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function createGroup(kbID: number, name: string): Promise<KnowledgeGroup> {
  const r = await fetch(`${BASE}/knowledge-bases/${kbID}/groups`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteGroup(id: number): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-groups/${id}`, {
    method: 'DELETE', headers: authHeaders(),
  })
  if (!r.ok) throw new Error(await r.text())
}

export async function listDocuments(groupID: number): Promise<KnowledgeDocument[]> {
  const r = await fetch(`${BASE}/knowledge-groups/${groupID}/documents`, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteDocuments(ids: number[]): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-documents`, {
    method: 'DELETE',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
  if (!r.ok) throw new Error(await r.text())
}

export async function importDocument(groupID: number, file: File): Promise<ImportResult> {
  const form = new FormData()
  form.append('group_id', String(groupID))
  form.append('file', file)
  const r = await fetch(`${BASE}/knowledge-documents/import`, {
    method: 'POST',
    headers: authHeaders(),
    body: form,
  })
  if (!r.ok) {
    const text = await r.text()
    let msg = text
    try { msg = JSON.parse(text).error ?? text } catch {}
    throw new Error(msg)
  }
  return r.json()
}
```

- [ ] **Step 2: TypeScript check**

```
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors in `knowledge.ts`

- [ ] **Step 3: Commit**

```bash
git add web/src/api/knowledge.ts
git commit -m "feat(knowledge): add frontend API client for KB/Group/Document/Import"
```

---

### Task 5: Rewrite KnowledgeView.vue — sidebar (KB→Group→Document tree)

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

Replace the entire file. The new view has three levels in the sidebar: KB list → expand to groups → expand to documents. Right panel shows document detail (name, status, entry_count). Import modal is multi-file with per-file status.

- [ ] **Step 1: Replace `<template>` section (lines 1–217)**

Replace the entire `<template>` block with:

```vue
<template>
  <div class="fullscreen-page kb-page">
    <aside class="kb-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">知识库</span>
        <button class="btn btn-primary btn-sm" @click="showNewKB = true">+ 知识库</button>
      </div>
      <div class="sidebar-list">
        <div v-for="kb in kbs" :key="kb.id" class="kb-section">
          <div class="kb-header" @click="toggleKB(kb.id)">
            <span class="chevron">{{ collapsedKBs.has(kb.id) ? '▶' : '▼' }}</span>
            <span class="kb-name">{{ kb.name }}</span>
            <button class="del-btn" @click.stop="doDeleteKB(kb)" title="删除知识库">×</button>
          </div>
          <template v-if="!collapsedKBs.has(kb.id)">
            <div v-for="group in groupsByKB[kb.id] ?? []" :key="group.id" class="group-section">
              <div class="group-header" @click="toggleGroup(group.id)">
                <span class="chevron">{{ collapsedGroups.has(group.id) ? '▶' : '▼' }}</span>
                <span class="group-name">{{ group.name }}</span>
                <button class="del-btn" @click.stop="doDeleteGroup(group)" title="删除分组">×</button>
                <button class="add-btn" @click.stop="openImport(group.id)" title="导入文档">+</button>
              </div>
              <template v-if="!collapsedGroups.has(group.id)">
                <div v-for="doc in docsByGroup[group.id] ?? []" :key="doc.id"
                  class="doc-row" :class="{ selected: activeDoc?.id === doc.id }"
                  @click="activeDoc = doc">
                  <span class="doc-name">{{ doc.name }}</span>
                  <span class="doc-status" :class="doc.status">{{ doc.status }}</span>
                </div>
                <div v-if="!(docsByGroup[group.id]?.length)" class="group-empty">暂无文档</div>
              </template>
            </div>
            <button class="add-group-btn" @click.stop="openNewGroup(kb.id)">+ 分组</button>
          </template>
        </div>
        <div v-if="kbs.length === 0" class="sidebar-empty">暂无知识库</div>
      </div>
    </aside>

    <div class="kb-detail">
      <div v-if="activeDoc" style="flex:1;display:flex;flex-direction:column;overflow:hidden;min-height:0">
        <div class="detail-topbar">
          <span class="detail-title">{{ activeDoc.name }}</span>
          <span class="doc-status" :class="activeDoc.status">{{ activeDoc.status }}</span>
          <span v-if="activeDoc.entry_count" class="entry-count">{{ activeDoc.entry_count }} 条目</span>
          <button class="btn btn-sm btn-danger" style="margin-left:auto"
            @click="doDeleteDoc(activeDoc)">删除</button>
        </div>
        <div v-if="activeDoc.error_msg" class="detail-error">{{ activeDoc.error_msg }}</div>
        <div class="detail-empty" v-else>
          <div class="detail-empty-icon">📄</div>
          <div>{{ activeDoc.doc_type }} · {{ activeDoc.entry_count }} 条目已索引</div>
        </div>
      </div>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">📚</div>
        <div>选择左侧文档查看详情</div>
      </div>
    </div>

    <!-- 新建知识库 -->
    <div v-if="showNewKB" class="modal-overlay" @click.self="showNewKB = false">
      <div class="modal" style="max-width:360px">
        <h3>新建知识库</h3>
        <div class="form-row">
          <label>名称</label>
          <input v-model="newKBName" class="input" @keyup.enter="doCreateKB" />
        </div>
        <div class="modal-footer">
          <button class="btn" @click="showNewKB = false">取消</button>
          <button class="btn btn-primary" :disabled="!newKBName.trim()" @click="doCreateKB">创建</button>
        </div>
      </div>
    </div>

    <!-- 新建分组 -->
    <div v-if="showNewGroup" class="modal-overlay" @click.self="showNewGroup = false">
      <div class="modal" style="max-width:360px">
        <h3>新建分组</h3>
        <div class="form-row">
          <label>名称</label>
          <input v-model="newGroupName" class="input" @keyup.enter="doCreateGroup" />
        </div>
        <div class="modal-footer">
          <button class="btn" @click="showNewGroup = false">取消</button>
          <button class="btn btn-primary" :disabled="!newGroupName.trim()" @click="doCreateGroup">创建</button>
        </div>
      </div>
    </div>

    <!-- 导入弹窗 -->
    <div v-if="showImport" class="modal-overlay" @click.self="showImport = false">
      <div class="modal">
        <h3>导入文档</h3>
        <div class="file-drop-zone"
          @dragover.prevent @drop.prevent="onDrop"
          @click="fileInputRef?.click()">
          <div v-if="importFiles.length === 0" class="drop-hint">拖拽文件到此处，或点击选择（.yaml/.yml/.json/.md）</div>
          <div v-else class="file-list">
            <div v-for="(f, i) in importFiles" :key="i" class="file-item">
              <span class="file-item-name">{{ f.file.name }}</span>
              <span class="file-item-status" :class="f.status">{{ f.statusText }}</span>
            </div>
          </div>
          <input ref="fileInputRef" type="file" multiple accept=".yaml,.yml,.json,.md"
            style="display:none" @change="onFileSelected" />
        </div>
        <div v-if="importErr" class="err" style="margin-top:8px">{{ importErr }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showImport = false; importFiles = []">取消</button>
          <button class="btn btn-primary" :disabled="importing || importFiles.length === 0" @click="doImport">
            {{ importing ? '导入中…' : '导入' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Replace `<script setup>` section (lines 219–487)**

Replace the entire script block with:

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  listKBs, createKB, deleteKB,
  listGroups, createGroup, deleteGroup,
  listDocuments, deleteDocuments, importDocument,
  type KnowledgeBase, type KnowledgeGroup, type KnowledgeDocument,
} from '../api/knowledge'

const fileInputRef = ref<HTMLInputElement | null>(null)

const kbs = ref<KnowledgeBase[]>([])
const groupsByKB = ref<Record<number, KnowledgeGroup[]>>({})
const docsByGroup = ref<Record<number, KnowledgeDocument[]>>({})
const activeDoc = ref<KnowledgeDocument | null>(null)
const collapsedKBs = ref(new Set<number>())
const collapsedGroups = ref(new Set<number>())

const showNewKB = ref(false)
const newKBName = ref('')
const showNewGroup = ref(false)
const newGroupName = ref('')
const newGroupKBID = ref(0)

const showImport = ref(false)
const importGroupID = ref(0)
const importing = ref(false)
const importErr = ref('')

interface ImportFile { file: File; status: 'pending' | 'ok' | 'error'; statusText: string }
const importFiles = ref<ImportFile[]>([])

function toggleKB(id: number) {
  const s = new Set(collapsedKBs.value)
  s.has(id) ? s.delete(id) : s.add(id)
  collapsedKBs.value = s
  if (!s.has(id)) loadGroups(id)
}

function toggleGroup(id: number) {
  const s = new Set(collapsedGroups.value)
  s.has(id) ? s.delete(id) : s.add(id)
  collapsedGroups.value = s
  if (!s.has(id)) loadDocs(id)
}

async function loadKBs() {
  kbs.value = await listKBs()
}

async function loadGroups(kbID: number) {
  groupsByKB.value = { ...groupsByKB.value, [kbID]: await listGroups(kbID) }
}

async function loadDocs(groupID: number) {
  docsByGroup.value = { ...docsByGroup.value, [groupID]: await listDocuments(groupID) }
}

async function doCreateKB() {
  if (!newKBName.value.trim()) return
  const kb = await createKB(newKBName.value.trim())
  kbs.value.push(kb)
  newKBName.value = ''
  showNewKB.value = false
}

function openNewGroup(kbID: number) {
  newGroupKBID.value = kbID
  newGroupName.value = ''
  showNewGroup.value = true
}

async function doCreateGroup() {
  if (!newGroupName.value.trim()) return
  const g = await createGroup(newGroupKBID.value, newGroupName.value.trim())
  groupsByKB.value = {
    ...groupsByKB.value,
    [newGroupKBID.value]: [...(groupsByKB.value[newGroupKBID.value] ?? []), g],
  }
  newGroupName.value = ''
  showNewGroup.value = false
}

async function doDeleteKB(kb: KnowledgeBase) {
  if (!confirm(`删除知识库「${kb.name}」及其所有内容？`)) return
  await deleteKB(kb.id)
  kbs.value = kbs.value.filter(k => k.id !== kb.id)
  const g = { ...groupsByKB.value }; delete g[kb.id]; groupsByKB.value = g
}

async function doDeleteGroup(group: KnowledgeGroup) {
  if (!confirm(`删除分组「${group.name}」及其所有文档？`)) return
  await deleteGroup(group.id)
  groupsByKB.value = {
    ...groupsByKB.value,
    [group.kb_id]: (groupsByKB.value[group.kb_id] ?? []).filter(g => g.id !== group.id),
  }
  const d = { ...docsByGroup.value }; delete d[group.id]; docsByGroup.value = d
}

async function doDeleteDoc(doc: KnowledgeDocument) {
  if (!confirm(`删除文档「${doc.name}」？`)) return
  await deleteDocuments([doc.id])
  docsByGroup.value = {
    ...docsByGroup.value,
    [doc.group_id]: (docsByGroup.value[doc.group_id] ?? []).filter(d => d.id !== doc.id),
  }
  if (activeDoc.value?.id === doc.id) activeDoc.value = null
}

function openImport(groupID: number) {
  importGroupID.value = groupID
  importFiles.value = []
  importErr.value = ''
  showImport.value = true
}

function onFileSelected(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files) return
  importFiles.value = Array.from(files).map(f => ({ file: f, status: 'pending', statusText: '待导入' }))
}

function onDrop(e: DragEvent) {
  const files = e.dataTransfer?.files
  if (!files) return
  importFiles.value = Array.from(files).map(f => ({ file: f, status: 'pending', statusText: '待导入' }))
}

async function doImport() {
  importing.value = true
  importErr.value = ''
  let anyError = false
  for (const item of importFiles.value) {
    item.status = 'pending'
    item.statusText = '导入中…'
    try {
      await importDocument(importGroupID.value, item.file)
      item.status = 'ok'
      item.statusText = '成功'
    } catch (e: any) {
      item.status = 'error'
      item.statusText = e.message ?? '失败'
      anyError = true
    }
  }
  importing.value = false
  await loadDocs(importGroupID.value)
  if (!anyError) {
    showImport.value = false
    importFiles.value = []
  }
}

onMounted(loadKBs)
</script>
```

- [ ] **Step 3: Replace `<style>` section (lines 489–603)**

Replace the entire style block with:

```vue
<style scoped>
.kb-page { display: flex; height: 100%; overflow: hidden; }

.kb-sidebar {
  width: 280px; min-width: 220px; max-width: 340px;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; flex-shrink: 0; overflow: hidden;
}
.sidebar-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}
.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); }
.sidebar-list { flex: 1; overflow-y: auto; }
.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }

.kb-section { border-bottom: 1px solid var(--border); }
.kb-header {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 12px; cursor: pointer; background: var(--surface);
  font-size: 12px; font-weight: 700; color: var(--text);
}
.kb-header:hover { background: var(--row-hover); }
.kb-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.group-section { padding-left: 12px; }
.group-header {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 8px; cursor: pointer;
  font-size: 12px; font-weight: 600; color: var(--text-sub);
}
.group-header:hover { background: var(--row-hover); }
.group-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.group-empty { font-size: 12px; color: var(--muted); padding: 4px 16px; font-style: italic; }

.chevron { font-size: 10px; color: var(--muted); width: 12px; flex-shrink: 0; }
.del-btn { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 14px; padding: 0 2px; }
.del-btn:hover { color: #dc2626; }
.add-btn { background: none; border: none; color: var(--primary); cursor: pointer; font-size: 14px; padding: 0 2px; }
.add-group-btn {
  display: block; width: calc(100% - 24px); margin: 4px 12px;
  background: none; border: 1px dashed var(--border); border-radius: 4px;
  color: var(--text-sub); font-size: 11px; padding: 4px; cursor: pointer;
}
.add-group-btn:hover { border-color: var(--primary); color: var(--primary); }

.doc-row {
  padding: 5px 8px 5px 20px; cursor: pointer; display: flex; align-items: center; gap: 8px;
  border-left: 3px solid transparent;
}
.doc-row:hover { background: var(--row-hover); }
.doc-row.selected { border-left-color: var(--primary); background: rgba(99,102,241,0.08); }
.doc-name { flex: 1; font-size: 12px; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.doc-status { font-size: 10px; border-radius: 4px; padding: 1px 5px; flex-shrink: 0; }
.doc-status.ready { background: rgba(16,185,129,0.12); color: #10b981; }
.doc-status.indexing { background: rgba(245,158,11,0.12); color: #f59e0b; }
.doc-status.error { background: rgba(220,38,38,0.12); color: #dc2626; }
.doc-status.pending { background: rgba(107,114,128,0.12); color: #6b7280; }

.kb-detail { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
.detail-topbar {
  display: flex; align-items: center; gap: 8px;
  padding: 14px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; background: var(--surface);
}
.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }
.entry-count { font-size: 12px; color: var(--text-sub); }
.detail-error { padding: 16px 20px; color: #dc2626; font-size: 13px; }
.detail-empty {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; color: var(--label); font-size: 14px;
}
.detail-empty-icon { font-size: 36px; opacity: 0.5; }

.file-drop-zone {
  border: 2px dashed var(--border); border-radius: 8px; padding: 20px;
  cursor: pointer; min-height: 100px; display: flex; flex-direction: column;
  align-items: center; justify-content: center;
}
.file-drop-zone:hover { border-color: var(--primary); }
.drop-hint { color: var(--text-sub); font-size: 13px; text-align: center; }
.file-list { width: 100%; display: flex; flex-direction: column; gap: 6px; }
.file-item { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.file-item-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); }
.file-item-status { flex-shrink: 0; }
.file-item-status.ok { color: #10b981; }
.file-item-status.error { color: #dc2626; }
</style>
```

- [ ] **Step 4: Build frontend**

```
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```
Expected: build succeeds, no TypeScript errors

- [ ] **Step 5: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat(knowledge): rewrite KnowledgeView with KB→Group→Document hierarchy and import UI"
```

---

### Task 6: End-to-end verification

**Files:** none (verification only)

- [ ] **Step 1: Build binary**

```
cd /Users/cw/fty.ai/spider.ai && go build -a -o /tmp/spider-test ./cmd/spider
```

- [ ] **Step 2: Start server**

```
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: Playwright smoke test**

```javascript
// Navigate to knowledge base page
await page.goto('http://localhost:8002')
// Login if needed, then navigate to /knowledge
// Verify KB list loads (empty state shows "暂无知识库")
// Create a KB, verify it appears
// Create a group inside the KB
// Import a .yaml file into the group
// Verify document appears with status
```

Use `mcp__playwright__browser_navigate` and `mcp__playwright__browser_snapshot` to verify the UI renders correctly.

- [ ] **Step 4: Commit if any fixes needed**

```bash
git add -p
git commit -m "fix(knowledge): UI fixes from e2e verification"
```

---

## Self-Review

**Spec coverage check:**
- ✅ KB/Group/Document CRUD — Tasks 1, 2, 4, 5
- ✅ Import endpoint calling `ImportDocument` pipeline — Task 3
- ✅ Multi-file import UI with per-file status — Task 5
- ✅ KB→Group→Document hierarchy in sidebar — Task 5
- ✅ Frontend API client — Task 4
- ✅ Route registration — Task 2

**Gaps noted:**
- Task 3 Step 2 requires runtime investigation of `Factory.LLMClient()` and `rag.Store.Embedder()` — implementer must check and adapt. If these methods don't exist, use `app.ProviderStore` directly to build `llm.Client` (same pattern as `NewAgentFactory`).
- The `mockKBStore` in tests only implements `kbStore` and `docStore` interfaces — it does NOT implement the full `KnowledgePlugin`. This is intentional (interface segregation).

