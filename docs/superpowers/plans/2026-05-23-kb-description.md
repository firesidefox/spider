# KB Description — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `description` field to KB documents and groups, with manual LLM generation and user editing via API and UI.

**Architecture:** DB migration adds `description` column to both tables. Store layer gets two update methods and updated SELECT queries. Four API handlers handle regeneration (POST) and editing (PUT). `KnowledgeView.vue` gains a doc description block in the entries panel and a group detail panel replacing the empty state.

**Tech Stack:** Go (SQLite, net/http), Vue 3 (Composition API), existing `llm.Client` interface (`internal/llm`), `internal/mcp.App.AgentFactory.LLMClient`.

---

## File Map

| File | Change |
|------|--------|
| `internal/db/schema.go` | Add 2 `ALTER TABLE` migration statements |
| `internal/knowledge/plugin.go` | Add `Description string` to `Group` and `Document` structs |
| `internal/knowledge/store.go` | Add `UpdateDocumentDescription`, `UpdateGroupDescription`; update 4 SELECT queries to include `description` |
| `internal/api/knowledge.go` | Add `descStore` interface; add `regenerateDocDescription`, `regenerateGroupDescription`, `updateDocDescription`, `updateGroupDescription` handlers; add `normalizeDescription` helper |
| `internal/api/handler.go` | Register 4 new routes |
| `internal/api/knowledge_test.go` | Add tests for all 4 handlers |
| `internal/knowledge/store_test.go` | Add tests for `UpdateDocumentDescription`, `UpdateGroupDescription` |
| `web/src/views/KnowledgeView.vue` | Add doc description block in entries panel; add group detail panel |

---

### Task 1: DB Migration + Struct Fields

**Files:**
- Modify: `internal/db/schema.go`
- Modify: `internal/knowledge/plugin.go`

- [ ] **Step 1: Add migration statements to `internal/db/schema.go`**

Find the end of the `migrate` function (before the final `return nil`). Add:

```go
db.Exec("ALTER TABLE knowledge_documents ADD COLUMN description TEXT NOT NULL DEFAULT ''")
db.Exec("ALTER TABLE knowledge_groups ADD COLUMN description TEXT NOT NULL DEFAULT ''")
```

- [ ] **Step 2: Add `Description` field to `Group` struct in `internal/knowledge/plugin.go`**

```go
type Group struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 3: Add `Description` field to `Document` struct in `internal/knowledge/plugin.go`**

```go
type Document struct {
	ID          int       `json:"id"`
	GroupID     int       `json:"group_id"`
	Name        string    `json:"name"`
	DocType     string    `json:"doc_type"`
	RawContent  string    `json:"raw_content"`
	Filename    string    `json:"filename"`
	Status      string    `json:"status"`
	ErrorMsg    string    `json:"error_msg"`
	EntryCount  int       `json:"entry_count"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/db/schema.go internal/knowledge/plugin.go
git commit -m "feat(knowledge): add description field to Group and Document"
```

---

### Task 2: Update Store SELECT Queries

**Files:**
- Modify: `internal/knowledge/store.go`

- [ ] **Step 1: Update `ListGroups` SELECT (line ~42)**

Change:
```go
`SELECT id, name, created_at FROM knowledge_groups ORDER BY id`
```
To:
```go
`SELECT id, name, description, created_at FROM knowledge_groups ORDER BY id`
```

Update the `rows.Scan` call to include `&g.Description`:
```go
if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt); err != nil {
```

- [ ] **Step 2: Update `GetGroupsByIDs` SELECT (line ~69)**

Change:
```go
fmt.Sprintf(`SELECT id, name, created_at FROM knowledge_groups WHERE id IN (%s) ORDER BY id`, placeholders)
```
To:
```go
fmt.Sprintf(`SELECT id, name, description, created_at FROM knowledge_groups WHERE id IN (%s) ORDER BY id`, placeholders)
```

Update `rows.Scan`:
```go
if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt); err != nil {
```

- [ ] **Step 3: Update `ListDocuments` SELECT (line ~91)**

Change:
```go
`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at
		FROM knowledge_documents
		WHERE group_id = ?
		ORDER BY id`
```
To:
```go
`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, description, created_at, updated_at
		FROM knowledge_documents
		WHERE group_id = ?
		ORDER BY id`
```

Update `rows.Scan`:
```go
if err := rows.Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.RawContent, &d.Filename,
    &d.Status, &d.ErrorMsg, &d.EntryCount, &d.Description, &d.CreatedAt, &d.UpdatedAt); err != nil {
```

- [ ] **Step 4: Update `GetDocument` SELECT (line ~113)**

Change:
```go
`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at
		FROM knowledge_documents
		WHERE id = ?`
```
To:
```go
`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, description, created_at, updated_at
		FROM knowledge_documents
		WHERE id = ?`
```

Update `.Scan`:
```go
).Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.RawContent, &d.Filename,
    &d.Status, &d.ErrorMsg, &d.EntryCount, &d.Description, &d.CreatedAt, &d.UpdatedAt)
```

- [ ] **Step 5: Update `GetDocumentsByIDs` SELECT (line ~140)**

Change:
```go
`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at
		FROM knowledge_documents
		WHERE id IN (%s)
		ORDER BY id`
```
To:
```go
`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, description, created_at, updated_at
		FROM knowledge_documents
		WHERE id IN (%s)
		ORDER BY id`
```

Update `rows.Scan`:
```go
if err := rows.Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.RawContent, &d.Filename,
    &d.Status, &d.ErrorMsg, &d.EntryCount, &d.Description, &d.CreatedAt, &d.UpdatedAt); err != nil {
```

- [ ] **Step 6: Add `UpdateDocumentDescription` and `UpdateGroupDescription` to `internal/knowledge/store.go`**

Add after `setDocumentStatus`:

```go
func (s *Store) UpdateDocumentDescription(ctx context.Context, docID int, desc string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET description = ? WHERE id = ?`, desc, docID)
	return err
}

func (s *Store) UpdateGroupDescription(ctx context.Context, groupID int, desc string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_groups SET description = ? WHERE id = ?`, desc, groupID)
	return err
}
```

- [ ] **Step 7: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/knowledge/store.go
git commit -m "feat(knowledge): update store queries and add description update methods"
```

---

### Task 3: Store Tests

**Files:**
- Modify: `internal/knowledge/store_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/knowledge/store_test.go`:

```go
func TestUpdateDocumentDescription(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "g1")
	req := knowledge.ImportRequest{
		GroupID:  g.ID,
		Name:     "doc1",
		Content:  []byte("# Title\n\nContent here."),
		Filename: "doc1.md",
	}
	// Create doc directly via createDocument path — use ImportDocument with no LLM (openapi stub)
	// Instead, insert directly for test isolation:
	sqldb := newTestDB(t)
	s2 := knowledge.NewStore(sqldb)
	g2, _ := s2.CreateGroup(ctx, "g2")
	_ = g2

	// Use a helper that inserts a ready doc
	docID := insertReadyDoc(t, sqldb, g2.ID, "test.md")

	if err := s2.UpdateDocumentDescription(ctx, docID, "A test description."); err != nil {
		t.Fatal(err)
	}
	doc, err := s2.GetDocument(ctx, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Description != "A test description." {
		t.Fatalf("expected description %q, got %q", "A test description.", doc.Description)
	}
	_ = s // suppress unused
}

func TestUpdateGroupDescription(t *testing.T) {
	sqldb := newTestDB(t)
	s := knowledge.NewStore(sqldb)
	ctx := context.Background()

	g, _ := s.CreateGroup(ctx, "mygroup")
	if err := s.UpdateGroupDescription(ctx, g.ID, "Group about ops."); err != nil {
		t.Fatal(err)
	}
	groups, _ := s.ListGroups(ctx)
	if len(groups) != 1 || groups[0].Description != "Group about ops." {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}

// insertReadyDoc inserts a knowledge_documents row with status=ready directly via SQL.
func insertReadyDoc(t *testing.T, db *sql.DB, groupID int, filename string) int {
	t.Helper()
	res, err := db.Exec(`INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, description, created_at, updated_at)
		VALUES (?, ?, 'markdown', '', ?, 'ready', '', 0, '', datetime('now'), datetime('now'))`,
		groupID, filename, filename)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/knowledge/... -run "TestUpdateDocumentDescription|TestUpdateGroupDescription" -v
```

Expected: FAIL — `insertReadyDoc` or method not found.

- [ ] **Step 3: Run tests after Task 2 implementation**

```bash
go test ./internal/knowledge/... -run "TestUpdateDocumentDescription|TestUpdateGroupDescription" -v
```

Expected: PASS.

- [ ] **Step 4: Run full knowledge package tests**

```bash
go test ./internal/knowledge/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/knowledge/store_test.go
git commit -m "test(knowledge): add UpdateDocumentDescription and UpdateGroupDescription tests"
```


---

### Task 4: API Handlers — `normalizeDescription` + `descStore` interface

**Files:**
- Modify: `internal/api/knowledge.go`

- [ ] **Step 1: Add `normalizeDescription` helper and `descStore` interface**

Add near the top of `internal/api/knowledge.go` (after existing interfaces):

```go
// descStore is the subset of knowledge.Store used by description handlers.
type descStore interface {
	GetDocument(ctx context.Context, docID int) (*knowledge.Document, error)
	GetGroupsByIDs(ctx context.Context, ids []int) ([]knowledge.Group, error)
	ListDocuments(ctx context.Context, groupID int) ([]knowledge.Document, error)
	ListEntries(ctx context.Context, documentID int) ([]knowledge.Entry, error)
	UpdateDocumentDescription(ctx context.Context, docID int, desc string) error
	UpdateGroupDescription(ctx context.Context, groupID int, desc string) error
}

// normalizeDescription trims, collapses whitespace, and truncates to 200 runes.
func normalizeDescription(s string) string {
	s = strings.TrimSpace(s)
	// collapse newlines/tabs to single space
	s = strings.NewReplacer("\n", " ", "\t", " ", "\r", " ").Replace(s)
	// compress consecutive spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	runes := []rune(s)
	if len(runes) > 200 {
		runes = runes[:200]
	}
	return string(runes)
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no errors.

---

### Task 5: API Handlers — regenerate-description (doc + group)

**Files:**
- Modify: `internal/api/knowledge.go`

- [ ] **Step 1: Add `regenerateDocDescription` handler**

```go
func regenerateDocDescription(s descStore, llmClient llm.Client, w http.ResponseWriter, r *http.Request, docIDStr string) {
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	doc, err := s.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	entries, err := s.ListEntries(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build entry list for prompt
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Title, e.Summary))
	}
	prompt := fmt.Sprintf(
		"为知识库文档生成一句话描述。文档标题: %s。条目列表:\n%s\n输出一句话（≤50字）概括本文档主题。纯文本，不含换行与Markdown。",
		doc.Name, sb.String(),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := llmClient.Chat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 256,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
		return
	}

	desc := normalizeDescription(resp)
	if err := s.UpdateDocumentDescription(r.Context(), docID, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}
```

- [ ] **Step 2: Add `regenerateGroupDescription` handler**

```go
func regenerateGroupDescription(s descStore, llmClient llm.Client, w http.ResponseWriter, r *http.Request, groupIDStr string) {
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	groups, err := s.GetGroupsByIDs(r.Context(), []int{groupID})
	if err != nil || len(groups) == 0 {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	group := groups[0]

	docs, err := s.ListDocuments(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(docs) == 0 {
		writeError(w, http.StatusBadRequest, "group has no documents")
		return
	}

	var sb strings.Builder
	for _, d := range docs {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", d.Name, d.Description))
	}
	prompt := fmt.Sprintf(
		"为知识库分组生成一句话描述。分组名: %s。包含文档:\n%s\n输出一句话（≤50字）概括本组涵盖的知识范围。纯文本，不含换行与Markdown。",
		group.Name, sb.String(),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := llmClient.Chat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 256,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
		return
	}

	desc := normalizeDescription(resp)
	if err := s.UpdateGroupDescription(r.Context(), groupID, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors. (Routes not wired yet — that's Task 7.)

---

### Task 6: API Handlers — PUT description (doc + group)

**Files:**
- Modify: `internal/api/knowledge.go`

- [ ] **Step 1: Add `updateDocDescription` handler**

```go
func updateDocDescription(s descStore, w http.ResponseWriter, r *http.Request, docIDStr string) {
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}
	doc, err := s.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	desc := normalizeDescription(body.Description)
	if len([]rune(desc)) > 200 {
		writeError(w, http.StatusBadRequest, "description too long")
		return
	}
	if err := s.UpdateDocumentDescription(r.Context(), docID, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}
```

- [ ] **Step 2: Add `updateGroupDescription` handler**

```go
func updateGroupDescription(s descStore, w http.ResponseWriter, r *http.Request, groupIDStr string) {
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	groups, err := s.GetGroupsByIDs(r.Context(), []int{groupID})
	if err != nil || len(groups) == 0 {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	desc := normalizeDescription(body.Description)
	if len([]rune(desc)) > 200 {
		writeError(w, http.StatusBadRequest, "description too long")
		return
	}
	if err := s.UpdateGroupDescription(r.Context(), groupID, desc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": desc})
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit Tasks 4–6**

```bash
git add internal/api/knowledge.go
git commit -m "feat(api): add KB description regenerate and update handlers"
```


---

### Task 7: Wire Routes in handler.go

**Files:**
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Extend `/api/v1/knowledge-documents/` handler to support new routes**

Find the existing `mux.HandleFunc("/api/v1/knowledge-documents/", ...)` block (around line 538). Replace it with:

```go
mux.HandleFunc("/api/v1/knowledge-documents/", func(w http.ResponseWriter, r *http.Request) {
    rest := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge-documents/")
    if rest == "" {
        http.NotFound(w, r)
        return
    }
    // POST /:id/regenerate-description
    if r.Method == http.MethodPost && strings.HasSuffix(rest, "/regenerate-description") {
        docIDStr := strings.TrimSuffix(rest, "/regenerate-description")
        if app.AgentFactory == nil {
            writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
            return
        }
        regenerateDocDescription(app.KnowledgeStore, app.AgentFactory.LLMClient, w, r, docIDStr)
        return
    }
    // PUT /:id (description edit)
    if r.Method == http.MethodPut {
        updateDocDescription(app.KnowledgeStore, w, r, rest)
        return
    }
    if r.Method == http.MethodGet {
        // Check for /sections suffix
        if strings.HasSuffix(rest, "/sections") {
            docID := strings.TrimSuffix(rest, "/sections")
            getKnowledgeDocumentSections(app.KnowledgeStore, w, r, docID)
            return
        }
        getKnowledgeDocument(app.KnowledgeStore, w, r, rest)
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
```

- [ ] **Step 2: Extend `/api/v1/knowledge-groups/` handler to support new routes**

Find the existing `mux.HandleFunc("/api/v1/knowledge-groups/", ...)` block (around line 511). Replace it with:

```go
mux.HandleFunc("/api/v1/knowledge-groups/", func(w http.ResponseWriter, r *http.Request) {
    rest := r.URL.Path[len("/api/v1/knowledge-groups/"):]
    // POST /:id/regenerate-description
    if r.Method == http.MethodPost && strings.HasSuffix(rest, "/regenerate-description") {
        groupIDStr := strings.TrimSuffix(rest, "/regenerate-description")
        if app.AgentFactory == nil {
            writeError(w, http.StatusServiceUnavailable, "llm provider unavailable")
            return
        }
        regenerateGroupDescription(app.KnowledgeStore, app.AgentFactory.LLMClient, w, r, groupIDStr)
        return
    }
    // PUT /:id (description edit)
    if r.Method == http.MethodPut {
        updateGroupDescription(app.KnowledgeStore, w, r, rest)
        return
    }
    // existing DELETE handler
    if r.Method == http.MethodDelete {
        deleteKnowledgeGroup(app.KnowledgeStore, w, r, rest)
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler.go
git commit -m "feat(api): wire KB description routes"
```

---

### Task 8: API Tests

**Files:**
- Modify: `internal/api/knowledge_test.go`

- [ ] **Step 1: Add mock methods to `mockKBStore`**

Add to the `mockKBStore` struct and its methods:

```go
// Add entries field to mockKBStore
type mockKBStore struct {
    groups  []knowledge.Group
    docs    []knowledge.Document
    entries []knowledge.Entry
    nextGr  int
    nextDoc int
}

func (m *mockKBStore) ListEntries(_ context.Context, documentID int) ([]knowledge.Entry, error) {
    var out []knowledge.Entry
    for _, e := range m.entries {
        if e.DocumentID == documentID {
            out = append(out, e)
        }
    }
    return out, nil
}

func (m *mockKBStore) UpdateDocumentDescription(_ context.Context, docID int, desc string) error {
    for i, d := range m.docs {
        if d.ID == docID {
            m.docs[i].Description = desc
            return nil
        }
    }
    return nil
}

func (m *mockKBStore) UpdateGroupDescription(_ context.Context, groupID int, desc string) error {
    for i, g := range m.groups {
        if g.ID == groupID {
            m.groups[i].Description = desc
            return nil
        }
    }
    return nil
}
```

- [ ] **Step 2: Add mock LLM client**

```go
type mockLLMClient struct {
    resp string
    err  error
}

func (m *mockLLMClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
    return m.resp, m.err
}
func (m *mockLLMClient) ChatStream(_ context.Context, _ *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
    ch := make(chan llm.StreamEvent)
    close(ch)
    return ch, nil
}
func (m *mockLLMClient) CountTokens(_ context.Context, _ []llm.Message) (int, error) {
    return 0, nil
}
```

- [ ] **Step 3: Write failing tests**

```go
func TestRegenerateDocDescription(t *testing.T) {
    s := newMockKBStore()
    s.docs = []knowledge.Document{{ID: 1, GroupID: 1, Name: "ops.md", Status: "ready"}}
    s.entries = []knowledge.Entry{
        {ID: 1, DocumentID: 1, Title: "nginx status", Summary: "check nginx"},
    }
    llmClient := &mockLLMClient{resp: "Nginx ops manual."}

    req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-documents/1/regenerate-description", nil)
    w := httptest.NewRecorder()
    regenerateDocDescription(s, llmClient, w, req, "1")

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
    }
    var resp map[string]string
    json.NewDecoder(w.Body).Decode(&resp)
    if resp["description"] != "Nginx ops manual." {
        t.Fatalf("unexpected description: %q", resp["description"])
    }
    if s.docs[0].Description != "Nginx ops manual." {
        t.Fatal("description not persisted in store")
    }
}

func TestRegenerateDocDescription_NotFound(t *testing.T) {
    s := newMockKBStore()
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    w := httptest.NewRecorder()
    regenerateDocDescription(s, &mockLLMClient{resp: "x"}, w, req, "99")
    if w.Code != http.StatusNotFound {
        t.Fatalf("expected 404, got %d", w.Code)
    }
}

func TestRegenerateGroupDescription_NoDocuments(t *testing.T) {
    s := newMockKBStore()
    s.groups = []knowledge.Group{{ID: 1, Name: "empty"}}
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    w := httptest.NewRecorder()
    regenerateGroupDescription(s, &mockLLMClient{resp: "x"}, w, req, "1")
    if w.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
    }
}

func TestUpdateDocDescription(t *testing.T) {
    s := newMockKBStore()
    s.docs = []knowledge.Document{{ID: 1, GroupID: 1, Name: "ops.md"}}
    body := `{"description":"A nice description."}`
    req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
    w := httptest.NewRecorder()
    updateDocDescription(s, w, req, "1")
    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
    }
    if s.docs[0].Description != "A nice description." {
        t.Fatal("description not updated")
    }
}

func TestUpdateDocDescription_TooLong(t *testing.T) {
    s := newMockKBStore()
    s.docs = []knowledge.Document{{ID: 1, GroupID: 1, Name: "ops.md"}}
    long := strings.Repeat("x", 201)
    body := fmt.Sprintf(`{"description":%q}`, long)
    req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
    w := httptest.NewRecorder()
    updateDocDescription(s, w, req, "1")
    // normalizeDescription truncates to 200, so this should pass (not 400)
    // The 400 only fires if after normalization still > 200 — which can't happen.
    // So expect 200 with truncated description.
    if w.Code != http.StatusOK {
        t.Fatalf("expected 200 (truncated), got %d", w.Code)
    }
}

func TestNormalizeDescription(t *testing.T) {
    cases := []struct{ in, want string }{
        {"  hello  ", "hello"},
        {"line1\nline2", "line1 line2"},
        {"a\t\tb", "a b"},
        {strings.Repeat("x", 250), strings.Repeat("x", 200)},
    }
    for _, c := range cases {
        got := normalizeDescription(c.in)
        if got != c.want {
            t.Errorf("normalizeDescription(%q) = %q, want %q", c.in, got, c.want)
        }
    }
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
go test ./internal/api/... -run "TestRegenerateDocDescription|TestRegenerateGroupDescription|TestUpdateDocDescription|TestNormalizeDescription" -v
```

Expected: FAIL — functions not defined yet (or compile error).

- [ ] **Step 5: Run tests after Tasks 4–6 implementation**

```bash
go test ./internal/api/... -run "TestRegenerateDocDescription|TestRegenerateGroupDescription|TestUpdateDocDescription|TestNormalizeDescription" -v
```

Expected: all PASS.

- [ ] **Step 6: Run full API tests**

```bash
go test ./internal/api/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/knowledge_test.go
git commit -m "test(api): add KB description handler tests"
```


---

### Task 9: Frontend — Doc Description Block

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: Add reactive state for doc description**

In the `<script setup>` section, find where `activeDoc` is declared (around line 269). Add:

```ts
const docDescDraft = ref('')
const docDescGenerating = ref(false)

// Keep docDescDraft in sync with activeDoc
watch(activeDoc, (d) => {
  docDescDraft.value = d?.description ?? ''
})
```

- [ ] **Step 2: Add `saveDocDescription` and `generateDocDescription` functions**

```ts
async function saveDocDescription() {
  if (!activeDoc.value) return
  await fetch(`/api/v1/knowledge-documents/${activeDoc.value.id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ description: docDescDraft.value }),
  })
  activeDoc.value = { ...activeDoc.value, description: docDescDraft.value }
}

async function generateDocDescription() {
  if (!activeDoc.value) return
  docDescGenerating.value = true
  try {
    const res = await fetch(`/api/v1/knowledge-documents/${activeDoc.value.id}/regenerate-description`, {
      method: 'POST',
    })
    if (res.ok) {
      const data = await res.json()
      docDescDraft.value = data.description
      activeDoc.value = { ...activeDoc.value, description: data.description }
    }
  } finally {
    docDescGenerating.value = false
  }
}
```

- [ ] **Step 3: Add `description` field to `KnowledgeDocument` TypeScript type**

Find the `KnowledgeDocument` interface/type definition in the `<script setup>` section. Add `description: string`:

```ts
interface KnowledgeDocument {
  id: number
  group_id: number
  name: string
  doc_type: string
  status: string
  error_msg: string
  entry_count: number
  description: string
  created_at: string
  updated_at: string
}
```

Also add `description: string` to `KnowledgeGroup` interface.

- [ ] **Step 4: Add doc description block to entries panel template**

Find the entries panel section (around line 76, `<section v-if="activeDoc" class="kb-entries">`). After the `entries-header` div and before the `entries-scroll` div, insert:

```html
<!-- Doc description block -->
<div class="doc-desc-block">
  <div class="doc-desc-label">文档描述</div>
  <textarea
    v-model="docDescDraft"
    class="doc-desc-textarea"
    placeholder="暂无描述，点击生成或手动输入..."
    rows="3"
  />
  <div class="doc-desc-actions">
    <button class="btn-desc-gen" :disabled="docDescGenerating" @click="generateDocDescription">
      {{ docDescGenerating ? '生成中...' : '✦ 生成' }}
    </button>
    <button class="btn-desc-save" @click="saveDocDescription">保存</button>
    <span class="desc-char-hint">≤200字</span>
  </div>
</div>
```

- [ ] **Step 5: Add CSS for doc description block**

In the `<style scoped>` section, add:

```css
.doc-desc-block {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
}
.doc-desc-label {
  font-size: 11px;
  color: var(--text-sub);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 5px;
}
.doc-desc-textarea {
  width: 100%;
  background: var(--input-bg, #252525);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  padding: 5px 8px;
  font-size: 12px;
  resize: vertical;
  font-family: inherit;
  line-height: 1.5;
}
.doc-desc-actions {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 6px;
}
.btn-desc-gen {
  background: var(--btn-secondary-bg, #1e2d3d);
  border: 1px solid var(--btn-secondary-border, #2d4a6a);
  color: var(--primary, #7eb8f7);
  padding: 3px 10px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 11px;
}
.btn-desc-gen:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn-desc-save {
  background: var(--btn-success-bg, #1a2e22);
  border: 1px solid var(--btn-success-border, #2a5a3a);
  color: var(--success, #6bcf8a);
  padding: 3px 10px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 11px;
}
.desc-char-hint {
  font-size: 11px;
  color: var(--text-muted, #555);
  margin-left: auto;
}
```

- [ ] **Step 6: Build frontend**

```bash
cd web && npm run build
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat(ui): add doc description block in KB entries panel"
```

---

### Task 10: Frontend — Group Detail Panel

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

- [ ] **Step 1: Add reactive state for group description**

In `<script setup>`, add:

```ts
const groupDescDraft = ref('')
const groupDescGenerating = ref(false)

watch(
  () => ({ groupId: activeGroupId.value, hasDoc: !!activeDoc.value }),
  ({ groupId, hasDoc }) => {
    if (!hasDoc && groupId != null) {
      const g = groups.value.find(g => g.id === groupId)
      groupDescDraft.value = g?.description ?? ''
    }
  }
)
```

- [ ] **Step 2: Add `saveGroupDescription` and `generateGroupDescription` functions**

```ts
async function saveGroupDescription() {
  if (activeGroupId.value == null) return
  await fetch(`/api/v1/knowledge-groups/${activeGroupId.value}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ description: groupDescDraft.value }),
  })
  const idx = groups.value.findIndex(g => g.id === activeGroupId.value)
  if (idx !== -1) {
    groups.value[idx] = { ...groups.value[idx], description: groupDescDraft.value }
  }
}

async function generateGroupDescription() {
  if (activeGroupId.value == null) return
  groupDescGenerating.value = true
  try {
    const res = await fetch(`/api/v1/knowledge-groups/${activeGroupId.value}/regenerate-description`, {
      method: 'POST',
    })
    if (res.ok) {
      const data = await res.json()
      groupDescDraft.value = data.description
      const idx = groups.value.findIndex(g => g.id === activeGroupId.value)
      if (idx !== -1) {
        groups.value[idx] = { ...groups.value[idx], description: data.description }
      }
    }
  } finally {
    groupDescGenerating.value = false
  }
}
```

- [ ] **Step 3: Add group detail panel to template**

Find the `<section class="kb-detail">` block. The current empty state shows when `!activeDoc` and `!loadingDetail`. Replace the `v-else-if="activeDoc"` empty state and the final `v-else` empty state with:

```html
<!-- Group detail panel (group selected, no doc) -->
<div v-else-if="activeGroupId && !activeDoc" class="group-detail-panel">
  <div class="group-detail-inner">
    <div class="group-detail-title">
      <span class="group-detail-icon">📁</span>
      <span class="group-detail-name">{{ groups.find(g => g.id === activeGroupId)?.name }}</span>
      <span class="group-detail-count">{{ docsByGroup[activeGroupId]?.length ?? 0 }} 篇文档</span>
    </div>

    <div class="doc-desc-label">分组描述</div>
    <textarea
      v-model="groupDescDraft"
      class="doc-desc-textarea"
      placeholder="暂无描述，点击生成或手动输入..."
      rows="3"
    />
    <div class="doc-desc-actions">
      <button class="btn-desc-gen" :disabled="groupDescGenerating" @click="generateGroupDescription">
        {{ groupDescGenerating ? '生成中...' : '✦ 生成' }}
      </button>
      <button class="btn-desc-save" @click="saveGroupDescription">保存</button>
      <span class="desc-char-hint">≤200字</span>
    </div>

    <hr class="group-detail-divider" />

    <div class="doc-desc-label" style="margin-bottom:10px">文档列表</div>
    <div
      v-for="doc in docsByGroup[activeGroupId] ?? []"
      :key="doc.id"
      class="group-doc-item"
    >
      <span class="group-doc-icon">📄</span>
      <div class="group-doc-info">
        <div class="group-doc-name">{{ doc.name }}</div>
        <div class="group-doc-desc" :class="{ empty: !doc.description }">
          {{ doc.description || '暂无描述' }}
        </div>
      </div>
      <span class="doc-status" :class="doc.status">{{ doc.status }}</span>
    </div>
  </div>
</div>

<!-- Fallback empty state -->
<div v-else class="detail-empty">
  <div class="detail-empty-icon">📚</div>
  <div>选择文档浏览条目</div>
</div>
```

- [ ] **Step 4: Add CSS for group detail panel**

```css
.group-detail-panel {
  flex: 1;
  overflow-y: auto;
}
.group-detail-inner {
  padding: 20px 24px;
  max-width: 560px;
}
.group-detail-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
}
.group-detail-icon { font-size: 20px; }
.group-detail-name { font-size: 15px; font-weight: 600; color: var(--text); }
.group-detail-count { font-size: 12px; color: var(--text-muted, #555); }
.group-detail-divider {
  border: none;
  border-top: 1px solid var(--border);
  margin: 18px 0;
}
.group-doc-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 7px 0;
  border-bottom: 1px solid var(--border-subtle, #222);
}
.group-doc-icon { font-size: 14px; margin-top: 1px; }
.group-doc-info { flex: 1; min-width: 0; }
.group-doc-name { font-size: 13px; color: var(--text-sub); margin-bottom: 2px; }
.group-doc-desc {
  font-size: 11px;
  color: var(--text-muted, #555);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.group-doc-desc.empty { font-style: italic; }
```

- [ ] **Step 5: Build frontend**

```bash
cd web && npm run build
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/views/KnowledgeView.vue
git commit -m "feat(ui): add group detail panel with description in KB view"
```

---

### Task 11: End-to-End Verification

- [ ] **Step 1: Build full binary**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
```

- [ ] **Step 2: Start test server**

```bash
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: Verify doc description block appears**

Open `http://localhost:8002` in browser. Navigate to Knowledge view. Click a document. Confirm description textarea and buttons appear above entry list.

- [ ] **Step 4: Verify group detail panel appears**

Click a group name (not a document). Confirm right panel shows group name, description textarea, and document list with description previews.

- [ ] **Step 5: Test generate flow**

Click "✦ 生成" on a doc with entries. Confirm button shows "生成中..." during request, then textarea fills with generated description.

- [ ] **Step 6: Test save flow**

Edit textarea manually, click "保存". Reload page, confirm description persists.

- [ ] **Step 7: Run all Go tests**

```bash
go test ./internal/knowledge/... ./internal/api/... -v
```

Expected: all PASS.

- [ ] **Step 8: Final commit if any fixes needed**

```bash
git add -p
git commit -m "fix(kb-description): <describe fix>"
```

