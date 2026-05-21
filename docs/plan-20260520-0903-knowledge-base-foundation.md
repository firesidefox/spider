# Knowledge Base Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the 5 new knowledge base tables to SQLite, define the `KnowledgePlugin` Go interface and all shared types, and implement the store layer (CRUD for KB/Group/Document/Section/Entry).

**Architecture:** New package `internal/knowledge/` holds the interface + types. Store implementation lives in `internal/knowledge/store.go`. DB migration appended to existing `migrate()` in `internal/db/schema.go`. No HTTP handlers in this plan.

**Tech Stack:** Go, `database/sql`, SQLite (existing driver), standard library only.

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/db/schema.go` | Append 5 new table migrations |
| Create | `internal/knowledge/plugin.go` | `KnowledgePlugin` interface + all shared types |
| Create | `internal/knowledge/store.go` | SQLite store implementing `KnowledgePlugin` (CRUD only, no search/import) |
| Create | `internal/knowledge/store_test.go` | Integration tests against in-memory SQLite |

---

### Task 1: DB Migration — 5 new tables

**Files:**
- Modify: `internal/db/schema.go` (append to end of `migrate()`, before `return nil`)

- [ ] **Step 1: Read current end of migrate()**

Confirm last line before `return nil` is line 446:
```
db.Exec(`CREATE INDEX IF NOT EXISTS idx_todo_tasks_conv ON todo_tasks(conversation_id)`)
```

- [ ] **Step 2: Add knowledge base tables migration**

Edit `internal/db/schema.go` — replace the final two lines:
```go
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_todo_tasks_conv ON todo_tasks(conversation_id)`)
	return nil
```
with:
```go
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_todo_tasks_conv ON todo_tasks(conversation_id)`)

	// Knowledge base tables
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_groups (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		kb_id      INTEGER NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
		name       TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_documents (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id    INTEGER NOT NULL REFERENCES knowledge_groups(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		doc_type    TEXT NOT NULL CHECK(doc_type IN ('openapi','markdown')),
		raw_content TEXT NOT NULL DEFAULT '',
		filename    TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','indexing','ready','error')),
		error_msg   TEXT NOT NULL DEFAULT '',
		entry_count INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_sections (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		summary     TEXT NOT NULL DEFAULT '',
		position    INTEGER NOT NULL DEFAULT 0
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_entries (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id INTEGER NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
		section_id  INTEGER REFERENCES knowledge_sections(id) ON DELETE SET NULL,
		title       TEXT NOT NULL,
		summary     TEXT NOT NULL DEFAULT '',
		content     TEXT NOT NULL,
		embedding   BLOB,
		position    INTEGER NOT NULL DEFAULT 0
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_groups_kb_id ON knowledge_groups(kb_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_docs_group_id ON knowledge_documents(group_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_sections_doc_id ON knowledge_sections(document_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_entries_doc_id ON knowledge_entries(document_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_entries_section_id ON knowledge_entries(section_id)`)
	return nil
```

- [ ] **Step 3: Build to verify no syntax errors**

```bash
go build ./...
```
Expected: no output (success).

- [ ] **Step 4: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(knowledge): add 5 knowledge base tables to DB migration"
```

---

### Task 2: KnowledgePlugin Interface + Types

**Files:**
- Create: `internal/knowledge/plugin.go`

- [ ] **Step 1: Create plugin.go with interface and types**

```go
package knowledge

import "context"

type Scope struct {
	Type string // "kb" | "group" | "document"
	ID   int
}

type KnowledgeBase struct {
	ID        int
	Name      string
	CreatedAt string
}

type Group struct {
	ID        int
	KBID      int
	Name      string
	CreatedAt string
}

type Document struct {
	ID         int
	GroupID    int
	Name       string
	DocType    string // "openapi" | "markdown"
	RawContent string
	Filename   string
	Status     string // "pending" | "indexing" | "ready" | "error"
	ErrorMsg   string
	EntryCount int
	CreatedAt  string
	UpdatedAt  string
}

type Section struct {
	ID         int
	DocumentID int
	Name       string
	Summary    string
	Position   int
	EntryCount int // populated by store queries, not stored in DB
}

type EntrySummary struct {
	ID      int
	Title   string
	Summary string
}

type Entry struct {
	ID        int
	Title     string
	Summary   string
	Content   string
	Embedding []byte
}

type ImportRequest struct {
	GroupID  int
	Name     string
	Content  []byte
	Filename string
}

type ImportResult struct {
	DocumentID   int
	EntryCount   int
	SectionCount int
}

// KnowledgePlugin is the in-process interface for all knowledge base operations.
type KnowledgePlugin interface {
	// CRUD
	CreateKB(ctx context.Context, name string) (*KnowledgeBase, error)
	ListKBs(ctx context.Context) ([]*KnowledgeBase, error)
	DeleteKB(ctx context.Context, kbID int) error

	CreateGroup(ctx context.Context, kbID int, name string) (*Group, error)
	ListGroups(ctx context.Context, kbID int) ([]*Group, error)
	DeleteGroup(ctx context.Context, groupID int) error

	ListDocuments(ctx context.Context, groupID int) ([]*Document, error)
	GetDocument(ctx context.Context, docID int) (*Document, error)
	DeleteDocuments(ctx context.Context, docIDs []int) error

	// Retrieval (implemented in Plan 3)
	CatalogSections(ctx context.Context, scope Scope) ([]Section, error)
	CatalogEntries(ctx context.Context, sectionID int) ([]EntrySummary, error)
	FetchEntries(ctx context.Context, entryIDs []int) ([]Entry, error)
	Search(ctx context.Context, query string, scope Scope, topK int) ([]Entry, error)

	// Import (implemented in Plan 2)
	ImportDocument(ctx context.Context, req ImportRequest) (*ImportResult, error)
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/knowledge/...
```
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/knowledge/plugin.go
git commit -m "feat(knowledge): define KnowledgePlugin interface and shared types"
```

---

### Task 3: Store Layer — KB and Group CRUD

**Files:**
- Create: `internal/knowledge/store.go`
- Create: `internal/knowledge/store_test.go`

- [ ] **Step 1: Write failing tests for KB and Group CRUD**

Create `internal/knowledge/store_test.go`:

```go
package knowledge_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(sqldb); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })
	return sqldb
}

func TestKBCRUD(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, err := s.CreateKB(ctx, "AISG")
	if err != nil {
		t.Fatal(err)
	}
	if kb.Name != "AISG" || kb.ID == 0 {
		t.Fatalf("unexpected kb: %+v", kb)
	}

	kbs, err := s.ListKBs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(kbs) != 1 || kbs[0].ID != kb.ID {
		t.Fatalf("expected 1 kb, got %d", len(kbs))
	}

	if err := s.DeleteKB(ctx, kb.ID); err != nil {
		t.Fatal(err)
	}
	kbs, _ = s.ListKBs(ctx)
	if len(kbs) != 0 {
		t.Fatal("expected 0 kbs after delete")
	}
}

func TestGroupCRUD(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, err := s.CreateGroup(ctx, kb.ID, "v706")
	if err != nil {
		t.Fatal(err)
	}
	if g.KBID != kb.ID || g.Name != "v706" {
		t.Fatalf("unexpected group: %+v", g)
	}

	groups, err := s.ListGroups(ctx, kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if err := s.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	groups, _ = s.ListGroups(ctx, kb.ID)
	if len(groups) != 0 {
		t.Fatal("expected 0 groups after delete")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/knowledge/... 2>&1 | head -20
```
Expected: compile error — `knowledge.NewStore` undefined.

- [ ] **Step 3: Create store.go with KB and Group CRUD**

Create `internal/knowledge/store.go`:

```go
package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateKB(ctx context.Context, name string) (*KnowledgeBase, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_bases (name, created_at) VALUES (?, ?)`, name, now)
	if err != nil {
		return nil, fmt.Errorf("create kb: %w", err)
	}
	id, _ := res.LastInsertId()
	return &KnowledgeBase{ID: int(id), Name: name, CreatedAt: now.Format(time.RFC3339)}, nil
}

func (s *Store) ListKBs(ctx context.Context) ([]*KnowledgeBase, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at FROM knowledge_bases ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*KnowledgeBase
	for rows.Next() {
		kb := &KnowledgeBase{}
		if err := rows.Scan(&kb.ID, &kb.Name, &kb.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, kb)
	}
	return out, rows.Err()
}

func (s *Store) DeleteKB(ctx context.Context, kbID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_bases WHERE id = ?`, kbID)
	return err
}

func (s *Store) CreateGroup(ctx context.Context, kbID int, name string) (*Group, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_groups (kb_id, name, created_at) VALUES (?, ?, ?)`, kbID, name, now)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	id, _ := res.LastInsertId()
	return &Group{ID: int(id), KBID: kbID, Name: name, CreatedAt: now.Format(time.RFC3339)}, nil
}

func (s *Store) ListGroups(ctx context.Context, kbID int) ([]*Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kb_id, name, created_at FROM knowledge_groups WHERE kb_id = ? ORDER BY id`, kbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		g := &Group{}
		if err := rows.Scan(&g.ID, &g.KBID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) DeleteGroup(ctx context.Context, groupID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_groups WHERE id = ?`, groupID)
	return err
}
```

- [ ] **Step 4: Run KB/Group tests**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/knowledge/... -run "TestKBCRUD|TestGroupCRUD" -v
```
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/knowledge/store.go internal/knowledge/store_test.go
git commit -m "feat(knowledge): store layer — KB and Group CRUD"
```

---

### Task 4: Store Layer — Document CRUD

**Files:**
- Modify: `internal/knowledge/store_test.go` (add test)
- Modify: `internal/knowledge/store.go` (add methods)

- [ ] **Step 1: Add failing test for Document CRUD**

Append to `internal/knowledge/store_test.go`:

```go
func TestDocumentCRUD(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	req := knowledge.ImportRequest{
		GroupID:  g.ID,
		Name:     "监控 API",
		Content:  []byte("openapi: 3.0.3"),
		Filename: "monitor.yaml",
	}
	docID, err := s.CreateDocument(ctx, req, "openapi")
	if err != nil {
		t.Fatal(err)
	}
	if docID == 0 {
		t.Fatal("expected non-zero docID")
	}

	doc, err := s.GetDocument(ctx, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "监控 API" || doc.DocType != "openapi" || doc.Status != "pending" {
		t.Fatalf("unexpected doc: %+v", doc)
	}
	if doc.RawContent != "openapi: 3.0.3" {
		t.Fatalf("unexpected raw content: %q", doc.RawContent)
	}

	docs, err := s.ListDocuments(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}

	if err := s.DeleteDocuments(ctx, []int{docID}); err != nil {
		t.Fatal(err)
	}
	docs, _ = s.ListDocuments(ctx, g.ID)
	if len(docs) != 0 {
		t.Fatal("expected 0 docs after delete")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/knowledge/... -run TestDocumentCRUD 2>&1 | head -10
```
Expected: compile error — `CreateDocument` undefined.

- [ ] **Step 3: Add Document methods to store.go**

Append to `internal/knowledge/store.go`:

```go
func (s *Store) CreateDocument(ctx context.Context, req ImportRequest, docType string) (int, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_documents
		 (group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'pending', '', 0, ?, ?)`,
		req.GroupID, req.Name, docType, string(req.Content), req.Filename, now, now)
	if err != nil {
		return 0, fmt.Errorf("create document: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) GetDocument(ctx context.Context, docID int) (*Document, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at
		 FROM knowledge_documents WHERE id = ?`, docID)
	d := &Document{}
	err := row.Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.RawContent, &d.Filename,
		&d.Status, &d.ErrorMsg, &d.EntryCount, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document %d not found", docID)
	}
	return d, err
}

func (s *Store) ListDocuments(ctx context.Context, groupID int) ([]*Document, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, name, doc_type, filename, status, error_msg, entry_count, created_at, updated_at
		 FROM knowledge_documents WHERE group_id = ? ORDER BY id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Document
	for rows.Next() {
		d := &Document{}
		if err := rows.Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.Filename,
			&d.Status, &d.ErrorMsg, &d.EntryCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDocuments(ctx context.Context, docIDs []int) error {
	if len(docIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(docIDs))
	args := make([]any, len(docIDs))
	for i, id := range docIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "DELETE FROM knowledge_documents WHERE id IN (" +
		joinStrings(placeholders, ",") + ")"
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
```

- [ ] **Step 4: Run Document tests**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/knowledge/... -v
```
Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/knowledge/store.go internal/knowledge/store_test.go
git commit -m "feat(knowledge): store layer — Document CRUD"
```

---

### Task 5: Store Layer — Section and Entry write methods

These are used by the import pipeline (Plan 2). We add write-only methods here; read methods (CatalogSections, CatalogEntries, FetchEntries) are in Plan 3.

**Files:**
- Modify: `internal/knowledge/store_test.go`
- Modify: `internal/knowledge/store.go`

- [ ] **Step 1: Add failing test**

Append to `internal/knowledge/store_test.go`:

```go
func TestSectionAndEntryWrite(t *testing.T) {
	s := knowledge.NewStore(newTestDB(t))
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")
	docID, _ := s.CreateDocument(ctx, knowledge.ImportRequest{
		GroupID: g.ID, Name: "API", Content: []byte("x"), Filename: "api.yaml",
	}, "openapi")

	secID, err := s.CreateSection(ctx, docID, "指标查询", "查询 Prometheus 指标", 0)
	if err != nil {
		t.Fatal(err)
	}
	if secID == 0 {
		t.Fatal("expected non-zero secID")
	}

	entID, err := s.CreateEntry(ctx, knowledge.EntryInput{
		DocumentID: docID,
		SectionID:  &secID,
		Title:      "GET /api/v1/query",
		Summary:    "即时查询",
		Content:    `{"method":"GET","path":"/api/v1/query"}`,
		Position:   0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entID == 0 {
		t.Fatal("expected non-zero entID")
	}

	if err := s.SetDocumentReady(ctx, docID, 1); err != nil {
		t.Fatal(err)
	}
	doc, _ := s.GetDocument(ctx, docID)
	if doc.Status != "ready" || doc.EntryCount != 1 {
		t.Fatalf("unexpected doc after ready: %+v", doc)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/knowledge/... -run TestSectionAndEntryWrite 2>&1 | head -10
```
Expected: compile error — `CreateSection`, `CreateEntry`, `EntryInput`, `SetDocumentReady` undefined.

- [ ] **Step 3: Add EntryInput type to plugin.go**

Append to `internal/knowledge/plugin.go`:

```go
type EntryInput struct {
	DocumentID int
	SectionID  *int // nil = no section
	Title      string
	Summary    string
	Content    string
	Embedding  []byte
	Position   int
}
```

- [ ] **Step 4: Add Section/Entry write methods to store.go**

Append to `internal/knowledge/store.go`:

```go
func (s *Store) CreateSection(ctx context.Context, docID int, name, summary string, position int) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_sections (document_id, name, summary, position) VALUES (?, ?, ?, ?)`,
		docID, name, summary, position)
	if err != nil {
		return 0, fmt.Errorf("create section: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) CreateEntry(ctx context.Context, e EntryInput) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_entries (document_id, section_id, title, summary, content, embedding, position)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.DocumentID, e.SectionID, e.Title, e.Summary, e.Content, e.Embedding, e.Position)
	if err != nil {
		return 0, fmt.Errorf("create entry: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) SetDocumentStatus(ctx context.Context, docID int, status, errMsg string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET status = ?, error_msg = ?, updated_at = ? WHERE id = ?`,
		status, errMsg, now, docID)
	return err
}

func (s *Store) SetDocumentReady(ctx context.Context, docID, entryCount int) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET status = 'ready', entry_count = ?, updated_at = ? WHERE id = ?`,
		entryCount, now, docID)
	return err
}

func (s *Store) UpdateEntryEmbedding(ctx context.Context, entryID int, embedding []byte) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_entries SET embedding = ? WHERE id = ?`, embedding, entryID)
	return err
}
```

- [ ] **Step 5: Run all tests**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/knowledge/... -v
```
Expected: all 4 tests PASS.

- [ ] **Step 6: Full build check**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/knowledge/plugin.go internal/knowledge/store.go internal/knowledge/store_test.go
git commit -m "feat(knowledge): store layer — Section/Entry write methods + document status helpers"
```

---

## Self-Review

**Spec coverage:**
- §6 DB Schema: all 5 tables present ✓
- §5 KnowledgePlugin interface: all methods declared ✓ (retrieval/import stubs for Plans 2/3)
- `raw_content` + `filename` added to `knowledge_documents` (needed for "原文档" tab in UI) ✓
- `access_faces.knowledge_sources` format unchanged — still `[{"type":"kb","id":1}]` ✓

**Placeholder scan:** None found.

**Type consistency:**
- `EntryInput.SectionID *int` — nullable, matches `knowledge_entries.section_id` nullable FK ✓
- `Store` does not implement full `KnowledgePlugin` interface yet (retrieval/import methods missing) — intentional, Plans 2/3 add them ✓
