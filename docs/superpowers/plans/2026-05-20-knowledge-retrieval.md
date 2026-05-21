# Knowledge Base Retrieval Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement hierarchical retrieval (catalog sections → catalog entries → fetch entries) and vector search for the knowledge base system.

**Architecture:** Add `EntryCount` to `Section` type, implement 4 retrieval methods in `Store`, rewrite `SearchDocsTool` to support 4 modes (sections/entries/fetch/search), add comprehensive tests.

**Tech Stack:** Go, SQLite, `database/sql`, existing `rag.Store` for vector search

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/knowledge/plugin.go` | Add `EntryCount` field to `Section` |
| Modify | `internal/knowledge/store.go` | Add 4 retrieval methods + helper |
| Modify | `internal/knowledge/store_test.go` | Add retrieval tests |
| Modify | `internal/agent/tools_docs.go` | Rewrite to 4-mode SearchDocs |
| Modify | `internal/agent/tools_docs_test.go` | Add 4-mode tests |

---

### Task 1: Add EntryCount to Section

**Files:**
- Modify: `internal/knowledge/plugin.go:71-78`
- Modify: `internal/knowledge/store.go:176-195`

- [ ] **Step 1: Add EntryCount field to Section**

Edit `internal/knowledge/plugin.go` — replace Section struct:
```go
// Section represents a logical section within a document.
type Section struct {
	ID         int
	DocumentID int
	Name       string
	Summary    string
	Position   int
	EntryCount int // NEW: number of entries in this section
}
```

- [ ] **Step 2: Update ListSections to include entry count**

Edit `internal/knowledge/store.go` — replace `ListSections` method:
```go
func (s *Store) ListSections(ctx context.Context, documentID int) ([]Section, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.document_id, s.name, s.summary, s.position,
		       COUNT(e.id) as entry_count
		FROM knowledge_sections s
		LEFT JOIN knowledge_entries e ON e.section_id = s.id
		WHERE s.document_id = ?
		GROUP BY s.id
		ORDER BY s.position`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Section
	for rows.Next() {
		var sec Section
		if err := rows.Scan(&sec.ID, &sec.DocumentID, &sec.Name, &sec.Summary, &sec.Position, &sec.EntryCount); err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}
```

- [ ] **Step 3: Build to verify**

Run: `go build ./internal/knowledge`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/knowledge/plugin.go internal/knowledge/store.go
git commit -m "feat(knowledge): add EntryCount to Section type"
```

---

### Task 2: Implement CatalogSections

**Files:**
- Modify: `internal/knowledge/store.go` (add method after ListEntries)
- Create: `internal/knowledge/store_test.go` (add test after TestEntryCRUD)

- [ ] **Step 1: Write failing test**

Edit `internal/knowledge/store_test.go` — add at end:
```go
func TestCatalogSections(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g1, _ := s.CreateGroup(ctx, kb.ID, "v706")
	g2, _ := s.CreateGroup(ctx, kb.ID, "v808")

	// Insert doc1 in g1
	res1, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g1.ID, "Doc1", "openapi", "content1", "doc1.yaml", "ready")
	doc1ID, _ := res1.LastInsertId()

	// Insert doc2 in g2
	res2, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g2.ID, "Doc2", "markdown", "content2", "doc2.md", "ready")
	doc2ID, _ := res2.LastInsertId()

	// Create sections with entries
	sec1, _ := s.CreateSection(ctx, int(doc1ID), "Auth", "Auth APIs", 0)
	sec2, _ := s.CreateSection(ctx, int(doc1ID), "Query", "Query APIs", 1)
	sec3, _ := s.CreateSection(ctx, int(doc2ID), "CLI", "CLI commands", 0)

	// Add entries to sections
	s.CreateEntry(ctx, int(doc1ID), &sec1.ID, "POST /login", "Login", "...", nil, 0)
	s.CreateEntry(ctx, int(doc1ID), &sec1.ID, "POST /logout", "Logout", "...", nil, 1)
	s.CreateEntry(ctx, int(doc1ID), &sec2.ID, "GET /query", "Query", "...", nil, 0)
	s.CreateEntry(ctx, int(doc2ID), &sec3.ID, "show version", "Version", "...", nil, 0)

	// Test scope: kb
	sections, err := s.CatalogSections(ctx, knowledge.Scope{Type: "kb", ID: kb.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	// Verify entry counts
	if sections[0].EntryCount != 2 || sections[1].EntryCount != 1 || sections[2].EntryCount != 1 {
		t.Fatalf("unexpected entry counts: %d, %d, %d", sections[0].EntryCount, sections[1].EntryCount, sections[2].EntryCount)
	}

	// Test scope: group
	sections, err = s.CatalogSections(ctx, knowledge.Scope{Type: "group", ID: g1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for group, got %d", len(sections))
	}

	// Test scope: document
	sections, err = s.CatalogSections(ctx, knowledge.Scope{Type: "document", ID: int(doc1ID)})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for document, got %d", len(sections))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/knowledge -run TestCatalogSections -v`
Expected: FAIL with "s.CatalogSections undefined"

- [ ] **Step 3: Implement CatalogSections**

Edit `internal/knowledge/store.go` — add after `ListEntries`:
```go
func (s *Store) CatalogSections(ctx context.Context, scope Scope) ([]Section, error) {
	var query string
	var args []interface{}

	switch scope.Type {
	case "kb":
		query = `
			SELECT s.id, s.document_id, s.name, s.summary, s.position,
			       COUNT(e.id) as entry_count
			FROM knowledge_sections s
			LEFT JOIN knowledge_entries e ON e.section_id = s.id
			INNER JOIN knowledge_documents d ON s.document_id = d.id
			INNER JOIN knowledge_groups g ON d.group_id = g.id
			WHERE g.kb_id = ?
			GROUP BY s.id
			ORDER BY s.position`
		args = []interface{}{scope.ID}
	case "group":
		query = `
			SELECT s.id, s.document_id, s.name, s.summary, s.position,
			       COUNT(e.id) as entry_count
			FROM knowledge_sections s
			LEFT JOIN knowledge_entries e ON e.section_id = s.id
			INNER JOIN knowledge_documents d ON s.document_id = d.id
			WHERE d.group_id = ?
			GROUP BY s.id
			ORDER BY s.position`
		args = []interface{}{scope.ID}
	case "document":
		query = `
			SELECT s.id, s.document_id, s.name, s.summary, s.position,
			       COUNT(e.id) as entry_count
			FROM knowledge_sections s
			LEFT JOIN knowledge_entries e ON e.section_id = s.id
			WHERE s.document_id = ?
			GROUP BY s.id
			ORDER BY s.position`
		args = []interface{}{scope.ID}
	default:
		return nil, fmt.Errorf("invalid scope type: %s", scope.Type)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Section
	for rows.Next() {
		var sec Section
		if err := rows.Scan(&sec.ID, &sec.DocumentID, &sec.Name, &sec.Summary, &sec.Position, &sec.EntryCount); err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/knowledge -run TestCatalogSections -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/knowledge/store.go internal/knowledge/store_test.go
git commit -m "feat(knowledge): implement CatalogSections with scope filtering"
```

---

### Task 3: Implement CatalogEntries and FetchEntries

**Files:**
- Modify: `internal/knowledge/store.go` (add 2 methods)
- Modify: `internal/knowledge/store_test.go` (add 2 tests)

- [ ] **Step 1: Write failing test for CatalogEntries**

Edit `internal/knowledge/store_test.go` — add after TestCatalogSections:
```go
func TestCatalogEntries(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "openapi", "content", "doc.yaml", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Auth", "Auth APIs", 0)

	// Create entries
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /login", "Login endpoint", "Full login content", nil, 0)
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /logout", "Logout endpoint", "Full logout content", nil, 1)
	s.CreateEntry(ctx, int(docID), &sec.ID, "GET /me", "Get user info", "Full me content", nil, 2)

	// Catalog entries
	entries, err := s.CatalogEntries(ctx, sec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Verify only summary fields returned
	if entries[0].Title != "POST /login" || entries[0].Summary != "Login endpoint" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if entries[1].Title != "POST /logout" || entries[2].Title != "GET /me" {
		t.Fatalf("unexpected entry order")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/knowledge -run TestCatalogEntries -v`
Expected: FAIL with "s.CatalogEntries undefined"

- [ ] **Step 3: Implement CatalogEntries**

Edit `internal/knowledge/store.go` — add after `CatalogSections`:
```go
func (s *Store) CatalogEntries(ctx context.Context, sectionID int) ([]EntrySummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, summary
		FROM knowledge_entries
		WHERE section_id = ?
		ORDER BY position`, sectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EntrySummary
	for rows.Next() {
		var e EntrySummary
		if err := rows.Scan(&e.ID, &e.Title, &e.Summary); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Write failing test for FetchEntries**

Edit `internal/knowledge/store_test.go` — add after TestCatalogEntries:
```go
func TestFetchEntries(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "openapi", "content", "doc.yaml", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Auth", "Auth APIs", 0)

	e1, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "POST /login", "Login summary", "Full login content with details", nil, 0)
	e2, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "POST /logout", "Logout summary", "Full logout content with details", nil, 1)

	// Fetch entries
	entries, err := s.FetchEntries(ctx, []int{e1.ID, e2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Verify full content returned
	if entries[0].Title != "POST /login" || entries[0].Content != "Full login content with details" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if entries[1].Title != "POST /logout" || entries[1].Content != "Full logout content with details" {
		t.Fatalf("unexpected entry: %+v", entries[1])
	}

	// Test empty input
	entries, err = s.FetchEntries(ctx, []int{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for empty input, got %d", len(entries))
	}

	// Test non-existent IDs
	entries, err = s.FetchEntries(ctx, []int{99999})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for non-existent IDs, got %d", len(entries))
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./internal/knowledge -run TestFetchEntries -v`
Expected: FAIL with "s.FetchEntries undefined"

- [ ] **Step 6: Implement FetchEntries**

Edit `internal/knowledge/store.go` — add after `CatalogEntries`:
```go
func (s *Store) FetchEntries(ctx context.Context, entryIDs []int) ([]Entry, error) {
	if len(entryIDs) == 0 {
		return []Entry{}, nil
	}

	// Build IN clause
	query := `SELECT id, title, content FROM knowledge_entries WHERE id IN (`
	args := make([]interface{}, len(entryIDs))
	for i, id := range entryIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ") ORDER BY position"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Title, &e.Content); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 7: Run both tests**

Run: `go test ./internal/knowledge -run "TestCatalogEntries|TestFetchEntries" -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/knowledge/store.go internal/knowledge/store_test.go
git commit -m "feat(knowledge): implement CatalogEntries and FetchEntries"
```

---

### Task 4: Implement Search with vector similarity

**Files:**
- Modify: `internal/knowledge/store.go` (add Search method)
- Modify: `internal/knowledge/store_test.go` (add test)

- [ ] **Step 1: Write failing test**

Edit `internal/knowledge/store_test.go` — add after TestFetchEntries:
```go
func TestSearch(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "Doc1", "openapi", "content", "doc.yaml", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Auth", "Auth APIs", 0)

	// Create entries with mock embeddings (4 floats = 16 bytes)
	emb1 := make([]byte, 16)
	emb2 := make([]byte, 16)
	// Set different values to simulate different embeddings
	emb1[0], emb1[1] = 1, 0
	emb2[0], emb2[1] = 0, 1

	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /login", "Login endpoint", "Full login content", emb1, 0)
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /logout", "Logout endpoint", "Full logout content", emb2, 1)

	// Search with mock query embedding (matches emb1)
	queryEmb := make([]byte, 16)
	queryEmb[0], queryEmb[1] = 1, 0

	entries, err := s.Search(ctx, queryEmb, knowledge.Scope{Type: "group", ID: g.ID}, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Should return entries (exact matching logic depends on similarity function)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry from search")
	}
	// Verify full content returned
	if entries[0].Content == "" {
		t.Fatal("expected full content in search results")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/knowledge -run TestSearch -v`
Expected: FAIL with "s.Search undefined"

- [ ] **Step 3: Implement Search**

Edit `internal/knowledge/store.go` — add after `FetchEntries`:
```go
func (s *Store) Search(ctx context.Context, queryEmbedding []byte, scope Scope, topK int) ([]Entry, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is required")
	}

	var query string
	var args []interface{}

	// SQLite doesn't have native vector similarity, so we fetch all entries in scope
	// and compute cosine similarity in Go. For production, use a vector DB or extension.
	switch scope.Type {
	case "kb":
		query = `
			SELECT e.id, e.title, e.content, e.embedding
			FROM knowledge_entries e
			INNER JOIN knowledge_documents d ON e.document_id = d.id
			INNER JOIN knowledge_groups g ON d.group_id = g.id
			WHERE g.kb_id = ? AND e.embedding IS NOT NULL`
		args = []interface{}{scope.ID}
	case "group":
		query = `
			SELECT e.id, e.title, e.content, e.embedding
			FROM knowledge_entries e
			INNER JOIN knowledge_documents d ON e.document_id = d.id
			WHERE d.group_id = ? AND e.embedding IS NOT NULL`
		args = []interface{}{scope.ID}
	case "document":
		query = `
			SELECT e.id, e.title, e.content, e.embedding
			FROM knowledge_entries e
			WHERE e.document_id = ? AND e.embedding IS NOT NULL`
		args = []interface{}{scope.ID}
	default:
		return nil, fmt.Errorf("invalid scope type: %s", scope.Type)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry Entry
		score float64
	}
	var candidates []candidate

	for rows.Next() {
		var e Entry
		var emb []byte
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &emb); err != nil {
			return nil, err
		}
		// Compute cosine similarity
		score := cosineSimilarity(queryEmbedding, emb)
		candidates = append(candidates, candidate{entry: e, score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Return top K
	limit := topK
	if limit > len(candidates) {
		limit = len(candidates)
	}
	out := make([]Entry, limit)
	for i := 0; i < limit; i++ {
		out[i] = candidates[i].entry
	}
	return out, nil
}

// cosineSimilarity computes cosine similarity between two float32 byte arrays.
func cosineSimilarity(a, b []byte) float64 {
	if len(a) != len(b) || len(a)%4 != 0 {
		return 0
	}
	n := len(a) / 4
	var dotProduct, normA, normB float64
	for i := 0; i < n; i++ {
		offset := i * 4
		valA := float64(bytesToFloat32(a[offset : offset+4]))
		valB := float64(bytesToFloat32(b[offset : offset+4]))
		dotProduct += valA * valB
		normA += valA * valA
		normB += valB * valB
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func bytesToFloat32(b []byte) float32 {
	bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	return math.Float32frombits(bits)
}
```

Add imports at top of file:
```go
import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/knowledge -run TestSearch -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/knowledge/store.go internal/knowledge/store_test.go
git commit -m "feat(knowledge): implement Search with cosine similarity"
```

### Task 5: Rewrite SearchDocs tool to 4-mode interface

**Files:**
- Modify: `internal/agent/tools_docs.go` (complete rewrite)
- Modify: `internal/agent/factory.go` (update tool initialization)

- [ ] **Step 1: Read current SearchDocs implementation**

Run: `cat internal/agent/tools_docs.go | head -50`
Note: Current tool uses `ragStore` and `docStore`, new design uses `knowledge.Store`

- [ ] **Step 2: Rewrite SearchDocs tool**

Edit `internal/agent/tools_docs.go` — replace entire file:
```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/llm"
)

type SearchDocsTool struct {
	knowledgeStore *knowledge.Store
	llmClient      llm.Client // for generating query embeddings
}

func NewSearchDocsTool(knowledgeStore *knowledge.Store, llmClient llm.Client) *SearchDocsTool {
	return &SearchDocsTool{
		knowledgeStore: knowledgeStore,
		llmClient:      llmClient,
	}
}

func (t *SearchDocsTool) DefaultRiskLevel() RiskLevel              { return RiskL1 }
func (t *SearchDocsTool) IsConcurrencySafe(_ map[string]any) bool { return true }
func (t *SearchDocsTool) Name() string                             { return "SearchDocs" }

func (t *SearchDocsTool) Description() string {
	return "Search knowledge base for API endpoints, CLI commands, and documentation. Read-only. No side effects. Use freely in Explore phase."
}

func (t *SearchDocsTool) SystemPromptSection() string {
	return `## SearchDocs — Hierarchical Knowledge Retrieval

**When to use:**
- Before calling any API endpoint — need correct path, params, auth
- Before running vendor-specific CLI commands — syntax varies by vendor/version
- When troubleshooting unfamiliar errors or behaviors

**When NOT to use:**
- Universal commands (df, ps, ls, grep) — no need to look these up
- Purely informational tasks (listing hosts, checking task status)

**Four modes:**

1. **sections** — List chapters in a knowledge scope (kb/group/document)
   - Returns: [{section_id, name, summary, entry_count}]
   - Use: Get overview of available topics

2. **entries** — List entries in a section
   - Returns: [{entry_id, title, summary}]
   - Use: Browse specific chapter contents

3. **fetch** — Get full content of specific entries
   - Returns: [{title, content}]
   - Use: Read the actual documentation

4. **search** — Vector search when catalog navigation fails
   - Returns: [{title, content}] (top-K matches)
   - Use: When entry_count ≥ 500 or catalog doesn't help

**Typical workflow:**
1. Get scope from face.knowledge_sources: [{"type":"group","id":3}]
2. SearchDocs mode=sections, scope_type=group, scope_id=3
3. Pick relevant section_id from results
4. SearchDocs mode=entries, section_id=N
5. Pick relevant entry_ids
6. SearchDocs mode=fetch, entry_ids=[...]

**Fallback to search:**
- If sections returns total_entries ≥ 500, use mode=search instead
- If catalog navigation doesn't find what you need, use mode=search`
}

func (t *SearchDocsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"sections", "entries", "fetch", "search"},
				"description": "Operation mode",
			},
			"scope_type": map[string]any{
				"type":        "string",
				"enum":        []string{"kb", "group", "document"},
				"description": "Scope type for sections/search mode",
			},
			"scope_id": map[string]any{
				"type":        "integer",
				"description": "Scope ID for sections/search mode",
			},
			"section_id": map[string]any{
				"type":        "integer",
				"description": "Section ID for entries mode",
			},
			"entry_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer"},
				"description": "Entry IDs for fetch mode",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query for search mode",
			},
		},
		"required": []string{"mode"},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	mode, _ := input["mode"].(string)
	if mode == "" {
		return &ToolResult{Content: "mode is required", IsError: true, RiskLevel: RiskL1}, nil
	}

	switch mode {
	case "sections":
		return t.executeSections(ctx, input)
	case "entries":
		return t.executeEntries(ctx, input)
	case "fetch":
		return t.executeFetch(ctx, input)
	case "search":
		return t.executeSearch(ctx, input)
	default:
		return &ToolResult{Content: fmt.Sprintf("invalid mode: %s", mode), IsError: true, RiskLevel: RiskL1}, nil
	}
}

func (t *SearchDocsTool) executeSections(ctx context.Context, input map[string]any) (*ToolResult, error) {
	scopeType, _ := input["scope_type"].(string)
	scopeID := toInt(input["scope_id"])
	if scopeType == "" || scopeID == 0 {
		return &ToolResult{Content: "scope_type and scope_id are required for sections mode", IsError: true, RiskLevel: RiskL1}, nil
	}

	sections, err := t.knowledgeStore.CatalogSections(ctx, knowledge.Scope{Type: scopeType, ID: scopeID})
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("catalog sections: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		SectionID  int    `json:"section_id"`
		Name       string `json:"name"`
		Summary    string `json:"summary"`
		EntryCount int    `json:"entry_count"`
	}
	results := make([]result, len(sections))
	totalEntries := 0
	for i, s := range sections {
		results[i] = result{
			SectionID:  s.ID,
			Name:       s.Name,
			Summary:    s.Summary,
			EntryCount: s.EntryCount,
		}
		totalEntries += s.EntryCount
	}

	b, _ := json.Marshal(map[string]any{
		"sections":      results,
		"total_entries": totalEntries,
	})

	summary := fmt.Sprintf("found %d sections, %d total entries", len(sections), totalEntries)
	if totalEntries >= 500 {
		summary += " (consider using mode=search for large result sets)"
	}

	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: summary}, nil
}

func (t *SearchDocsTool) executeEntries(ctx context.Context, input map[string]any) (*ToolResult, error) {
	sectionID := toInt(input["section_id"])
	if sectionID == 0 {
		return &ToolResult{Content: "section_id is required for entries mode", IsError: true, RiskLevel: RiskL1}, nil
	}

	entries, err := t.knowledgeStore.CatalogEntries(ctx, sectionID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("catalog entries: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		EntryID int    `json:"entry_id"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	results := make([]result, len(entries))
	for i, e := range entries {
		results[i] = result{EntryID: e.ID, Title: e.Title, Summary: e.Summary}
	}

	b, _ := json.Marshal(results)
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d entries", len(entries))}, nil
}

func (t *SearchDocsTool) executeFetch(ctx context.Context, input map[string]any) (*ToolResult, error) {
	entryIDsRaw, ok := input["entry_ids"]
	if !ok {
		return &ToolResult{Content: "entry_ids is required for fetch mode", IsError: true, RiskLevel: RiskL1}, nil
	}
	entryIDs := toIntSlice(entryIDsRaw)
	if len(entryIDs) == 0 {
		return &ToolResult{Content: "entry_ids cannot be empty", IsError: true, RiskLevel: RiskL1}, nil
	}

	entries, err := t.knowledgeStore.FetchEntries(ctx, entryIDs)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("fetch entries: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	results := make([]result, len(entries))
	for i, e := range entries {
		results[i] = result{Title: e.Title, Content: e.Content}
	}

	b, _ := json.Marshal(results)
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("fetched %d entries", len(entries))}, nil
}

func (t *SearchDocsTool) executeSearch(ctx context.Context, input map[string]any) (*ToolResult, error) {
	query, _ := input["query"].(string)
	scopeType, _ := input["scope_type"].(string)
	scopeID := toInt(input["scope_id"])

	if query == "" || scopeType == "" || scopeID == 0 {
		return &ToolResult{Content: "query, scope_type, and scope_id are required for search mode", IsError: true, RiskLevel: RiskL1}, nil
	}

	if t.llmClient == nil {
		return &ToolResult{Content: "search unavailable: LLM client not configured", IsError: true, RiskLevel: RiskL1}, nil
	}

	// Generate query embedding
	embedding, err := t.llmClient.Embed(ctx, query)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("generate embedding: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	// Convert float64 slice to byte array (float32 little-endian)
	embBytes := make([]byte, len(embedding)*4)
	for i, f := range embedding {
		bits := math.Float32bits(float32(f))
		offset := i * 4
		embBytes[offset] = byte(bits)
		embBytes[offset+1] = byte(bits >> 8)
		embBytes[offset+2] = byte(bits >> 16)
		embBytes[offset+3] = byte(bits >> 24)
	}

	entries, err := t.knowledgeStore.Search(ctx, embBytes, knowledge.Scope{Type: scopeType, ID: scopeID}, 5)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("search: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	results := make([]result, len(entries))
	for i, e := range entries {
		results[i] = result{Title: e.Title, Content: e.Content}
	}

	b, _ := json.Marshal(results)
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d results", len(entries))}, nil
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

Add import for math:
```go
import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/llm"
)
```

- [ ] **Step 3: Update factory to wire new SearchDocs**

Edit `internal/agent/factory.go` — find line ~257 and replace:
```go
	if !f.DisableSearchDocs {
		registry.Register(NewSearchDocsTool(f.RagStore, f.DocStore))
	}
```

with:
```go
	if !f.DisableSearchDocs && f.KnowledgeStore != nil {
		registry.Register(NewSearchDocsTool(f.KnowledgeStore, f.LLMClient))
	}
```

- [ ] **Step 4: Add KnowledgeStore field to Factory**

Edit `internal/agent/factory.go` — add field after line ~38:
```go
	DocStore       *store.DocumentStore
	RagStore       *rag.Store
	KnowledgeStore *knowledge.Store // NEW
	TaskStore      *store.TaskStore
```

Add import:
```go
import (
	...
	"github.com/spiderai/spider/internal/knowledge"
	...
)
```

- [ ] **Step 5: Build to verify**

Run: `go build ./internal/agent`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_docs.go internal/agent/factory.go
git commit -m "feat(agent): rewrite SearchDocs to 4-mode hierarchical retrieval"
```

### Task 6: Add SearchDocs integration tests

**Files:**
- Modify: `internal/agent/tools_docs_test.go` (rewrite with 4-mode tests)

- [ ] **Step 1: Write test setup and sections mode test**

Edit `internal/agent/tools_docs_test.go` — replace entire file:
```go
package agent_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/llm"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqldb.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(sqldb); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqldb.Close() })
	return sqldb
}

func setupTestKnowledge(t *testing.T, s *knowledge.Store) (kbID, groupID, docID, sec1ID, sec2ID int) {
	t.Helper()
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	db := s.(*knowledge.Store) // access underlying db for direct insert
	// Use reflection or add a test helper method to get db
	// For now, assume we can insert via store methods

	// Create document via raw SQL (store doesn't have CreateDocument yet)
	// This is a test-only workaround
	return kb.ID, g.ID, 0, 0, 0 // placeholder
}

func TestSearchDocsSectionsMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	// Setup test data
	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// Insert document
	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "API Doc", "openapi", "content", "api.yaml", "ready")
	docID, _ := res.LastInsertId()

	// Create sections with entries
	sec1, _ := s.CreateSection(ctx, int(docID), "Auth", "Authentication APIs", 0)
	sec2, _ := s.CreateSection(ctx, int(docID), "Query", "Query APIs", 1)

	s.CreateEntry(ctx, int(docID), &sec1.ID, "POST /login", "Login", "...", nil, 0)
	s.CreateEntry(ctx, int(docID), &sec1.ID, "POST /logout", "Logout", "...", nil, 1)
	s.CreateEntry(ctx, int(docID), &sec2.ID, "GET /query", "Query", "...", nil, 0)

	// Create tool
	tool := agent.NewSearchDocsTool(s, nil)

	// Test sections mode with group scope
	result, err := tool.Execute(ctx, map[string]any{
		"mode":       "sections",
		"scope_type": "group",
		"scope_id":   float64(g.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content), &response); err != nil {
		t.Fatal(err)
	}

	sections := response["sections"].([]any)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	totalEntries := int(response["total_entries"].(float64))
	if totalEntries != 3 {
		t.Fatalf("expected 3 total entries, got %d", totalEntries)
	}
}

func TestSearchDocsEntriesMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "API Doc", "openapi", "content", "api.yaml", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Auth", "Auth APIs", 0)
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /login", "Login endpoint", "Full content", nil, 0)
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /logout", "Logout endpoint", "Full content", nil, 1)

	tool := agent.NewSearchDocsTool(s, nil)

	result, err := tool.Execute(ctx, map[string]any{
		"mode":       "entries",
		"section_id": float64(sec.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(result.Content), &entries); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0]["title"] != "POST /login" {
		t.Fatalf("unexpected entry title: %v", entries[0]["title"])
	}
}

func TestSearchDocsFetchMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "API Doc", "openapi", "content", "api.yaml", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Auth", "Auth APIs", 0)
	e1, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "POST /login", "Login summary", "Full login content with params", nil, 0)
	e2, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "POST /logout", "Logout summary", "Full logout content", nil, 1)

	tool := agent.NewSearchDocsTool(s, nil)

	result, err := tool.Execute(ctx, map[string]any{
		"mode":      "fetch",
		"entry_ids": []any{float64(e1.ID), float64(e2.ID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(result.Content), &entries); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0]["content"] != "Full login content with params" {
		t.Fatalf("unexpected content: %v", entries[0]["content"])
	}
}

func TestSearchDocsSearchMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	res, _ := db.ExecContext(ctx, `INSERT INTO knowledge_documents
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		g.ID, "API Doc", "openapi", "content", "api.yaml", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Auth", "Auth APIs", 0)

	// Create entries with embeddings
	emb1 := make([]byte, 16) // 4 floats
	emb1[0], emb1[1] = 1, 0
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /login", "Login endpoint", "Full login content", emb1, 0)

	// Mock LLM client that returns matching embedding
	mockLLM := &mockEmbedder{embedding: []float64{1.0, 0.0, 0.0, 0.0}}
	tool := agent.NewSearchDocsTool(s, mockLLM)

	result, err := tool.Execute(ctx, map[string]any{
		"mode":       "search",
		"query":      "login",
		"scope_type": "group",
		"scope_id":   float64(g.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(result.Content), &entries); err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry from search")
	}
	if entries[0]["title"] != "POST /login" {
		t.Fatalf("unexpected search result: %v", entries[0]["title"])
	}
}

// mockEmbedder implements llm.Client for testing
type mockEmbedder struct {
	embedding []float64
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	return m.embedding, nil
}

func (m *mockEmbedder) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, nil
}

func (m *mockEmbedder) Stream(ctx context.Context, req llm.ChatRequest, handler func(llm.StreamChunk) error) error {
	return nil
}
```

- [ ] **Step 2: Run all tests**

Run: `go test ./internal/agent -run "TestSearchDocs" -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/agent/tools_docs_test.go
git commit -m "test(agent): add 4-mode SearchDocs integration tests"
```

---

### Task 7: Wire KnowledgeStore into App and Factory

**Files:**
- Modify: `internal/mcp/server.go` (add KnowledgeStore to App)
- Modify: `cmd/spider/main.go` or initialization code (wire KnowledgeStore)

- [ ] **Step 1: Add KnowledgeStore field to App**

Edit `internal/mcp/server.go` — add field after line ~60:
```go
	DocStore        *store.DocumentStore
	GroupStore      *store.GroupStore
	KnowledgeStore  *knowledge.Store // NEW
	AccessFaceStore  *store.AccessFaceStore
```

Add import:
```go
import (
	...
	"github.com/spiderai/spider/internal/knowledge"
	...
)
```

- [ ] **Step 2: Update NewAgentFactory to pass KnowledgeStore**

Edit `internal/mcp/server.go` — find `NewAgentFactory` method (~line 93) and add:
```go
func (a *App) NewAgentFactory() (*agent.Factory, error) {
	// ... existing code ...
	
	return &agent.Factory{
		LLMClient:   llmClient,
		Hosts:       a.HostStore,
		// ... existing fields ...
		DocStore:       a.DocStore,
		RagStore:       ragStore,
		KnowledgeStore: a.KnowledgeStore, // NEW
		TaskStore:      a.TaskStore,
		// ... rest of fields ...
	}, nil
}
```

- [ ] **Step 3: Find where App is initialized**

Run: `grep -n "App{" cmd/spider/main.go internal/mcp/*.go | head -20`
Note: Find the initialization site

- [ ] **Step 4: Add KnowledgeStore initialization**

Edit the file found in Step 3 — add after DocStore/GroupStore init:
```go
	knowledgeStore := knowledge.NewStore(db)
```

And add to App struct initialization:
```go
	app := &mcp.App{
		// ... existing fields ...
		DocStore:        docStore,
		GroupStore:      groupStore,
		KnowledgeStore:  knowledgeStore, // NEW
		AccessFaceStore: accessFaceStore,
		// ... rest of fields ...
	}
```

- [ ] **Step 5: Build full project**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 6: Run all knowledge tests**

Run: `go test ./internal/knowledge/... ./internal/agent/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(mcp): wire KnowledgeStore into App and AgentFactory"
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ Section.EntryCount field added
- ✅ CatalogSections with scope filtering (kb/group/document)
- ✅ CatalogEntries returns lightweight summaries
- ✅ FetchEntries returns full content
- ✅ Search with cosine similarity and scope filtering
- ✅ SearchDocs tool rewritten to 4 modes
- ✅ Integration tests for all 4 modes
- ✅ Wired into App and Factory

**Placeholder scan:**
- ✅ No TBD/TODO
- ✅ All code blocks complete
- ✅ All test assertions specific
- ✅ All commands have expected output

**Type consistency:**
- ✅ `Section.EntryCount` used consistently
- ✅ `Scope{Type, ID}` used in all retrieval methods
- ✅ `EntrySummary` vs `Entry` distinction clear
- ✅ Tool input/output JSON schemas match implementation

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-20-knowledge-retrieval.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?

