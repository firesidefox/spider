package agent

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
)

// newTestDB creates an in-memory SQLite database with migrations applied.
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

// mockEmbedder implements the Embedder interface for testing.
// It returns predefined embeddings for known texts, zero vectors otherwise.
type mockEmbedder struct {
	vecs map[string][]float32
	dim  int
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if v, ok := m.vecs[text]; ok {
		return v, nil
	}
	// Return zero vector for unknown texts
	return make([]float32, m.dim), nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (m *mockEmbedder) Dimensions() int { return m.dim }

// float32ToBytes converts a float32 slice to byte array (little-endian).
func float32ToBytes(vec []float32) []byte {
	b := make([]byte, len(vec)*4)
	for i, f := range vec {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(b[i*4:], bits)
	}
	return b
}


// TestSearchDocsSectionsMode tests the sections catalog mode.
func TestSearchDocsSectionsMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	// Create test data
	group, _ := s.CreateGroup(ctx, "TestGroup")

	// Insert document directly
	res, err := db.ExecContext(ctx,
		`INSERT INTO knowledge_documents (group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		group.ID, "TestDoc", "markdown", "# Test", "test.md", "ready")
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := res.LastInsertId()

	// Create sections
	sec1, _ := s.CreateSection(ctx, int(docID), "Introduction", "Getting started", 1)
	sec2, _ := s.CreateSection(ctx, int(docID), "API Reference", "API endpoints", 2)

	// Create entries for each section
	emb := float32ToBytes([]float32{0.1, 0.2, 0.3})
	s.CreateEntry(ctx, int(docID), &sec1.ID, "Intro 1", "Summary 1", "Content 1", emb, 1)
	s.CreateEntry(ctx, int(docID), &sec1.ID, "Intro 2", "Summary 2", "Content 2", emb, 2)
	s.CreateEntry(ctx, int(docID), &sec2.ID, "API 1", "Summary 3", "Content 3", emb, 1)

	// Create tool and execute
	tool := NewSearchDocsTool(s, nil)
	result, err := tool.Execute(ctx, map[string]any{
		"mode":       "sections",
		"scope_type": "group",
		"scope_id":   float64(group.ID),
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Result is error: %s", result.Content)
	}

	// Parse JSON response
	var resp struct {
		Sections []struct {
			SectionID  int    `json:"section_id"`
			Name       string `json:"name"`
			Summary    string `json:"summary"`
			EntryCount int    `json:"entry_count"`
		} `json:"sections"`
		TotalEntries int `json:"total_entries"`
	}
	if err := json.Unmarshal([]byte(result.Content), &resp); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify response
	if len(resp.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(resp.Sections))
	}
	if resp.TotalEntries != 3 {
		t.Errorf("expected 3 total entries, got %d", resp.TotalEntries)
	}
	if resp.Sections[0].Name != "Introduction" {
		t.Errorf("expected section name 'Introduction', got %s", resp.Sections[0].Name)
	}
	if resp.Sections[0].EntryCount != 2 {
		t.Errorf("expected section 1 entry count 2, got %d", resp.Sections[0].EntryCount)
	}
}

// TestSearchDocsEntriesMode tests the entries catalog mode.
func TestSearchDocsEntriesMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	// Create test data
	group, _ := s.CreateGroup(ctx, "TestGroup")
	res, _ := db.ExecContext(ctx,
		`INSERT INTO knowledge_documents (group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		group.ID, "TestDoc", "markdown", "# Test", "test.md", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "API Reference", "API endpoints", 1)

	// Create entries
	emb := float32ToBytes([]float32{0.1, 0.2, 0.3})
	s.CreateEntry(ctx, int(docID), &sec.ID, "GET /users", "List users", "Returns all users", emb, 1)
	s.CreateEntry(ctx, int(docID), &sec.ID, "POST /users", "Create user", "Creates a new user", emb, 2)

	// Create tool and execute
	tool := NewSearchDocsTool(s, nil)
	result, err := tool.Execute(ctx, map[string]any{
		"mode":       "entries",
		"section_id": float64(sec.ID),
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Result is error: %s", result.Content)
	}

	// Parse JSON response
	var entries []struct {
		EntryID int    `json:"entry_id"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(result.Content), &entries); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify response
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "GET /users" {
		t.Errorf("expected title 'GET /users', got %s", entries[0].Title)
	}
	if entries[1].Summary != "Create user" {
		t.Errorf("expected summary 'Create user', got %s", entries[1].Summary)
	}
}

// TestSearchDocsFetchMode tests the fetch mode.
func TestSearchDocsFetchMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	// Create test data
	group, _ := s.CreateGroup(ctx, "TestGroup")
	res, _ := db.ExecContext(ctx,
		`INSERT INTO knowledge_documents (group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		group.ID, "TestDoc", "markdown", "# Test", "test.md", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "Commands", "CLI commands", 1)

	// Create entries
	emb := float32ToBytes([]float32{0.1, 0.2, 0.3})
	e1, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "df command", "Disk usage", "df -h shows disk usage", emb, 1)
	e2, _ := s.CreateEntry(ctx, int(docID), &sec.ID, "ps command", "Process list", "ps aux shows processes", emb, 2)

	// Create tool and execute
	tool := NewSearchDocsTool(s, nil)
	result, err := tool.Execute(ctx, map[string]any{
		"mode":      "fetch",
		"entry_ids": []any{float64(e1.ID), float64(e2.ID)},
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Result is error: %s", result.Content)
	}

	// Parse JSON response
	var entries []struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Content), &entries); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify response
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "df command" {
		t.Errorf("expected title 'df command', got %s", entries[0].Title)
	}
	if entries[0].Content != "df -h shows disk usage" {
		t.Errorf("expected content 'df -h shows disk usage', got %s", entries[0].Content)
	}
	if entries[1].Title != "ps command" {
		t.Errorf("expected title 'ps command', got %s", entries[1].Title)
	}
}

// TestSearchDocsSearchMode tests the vector search mode.
func TestSearchDocsSearchMode(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	// Create test data
	group, _ := s.CreateGroup(ctx, "TestGroup")
	res, _ := db.ExecContext(ctx,
		`INSERT INTO knowledge_documents (group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		group.ID, "TestDoc", "markdown", "# Test", "test.md", "ready")
	docID, _ := res.LastInsertId()

	sec, _ := s.CreateSection(ctx, int(docID), "API", "API docs", 1)

	// Create entries with embeddings
	queryVec := []float32{1.0, 0.0, 0.0}
	similarVec := []float32{0.9, 0.1, 0.0}
	differentVec := []float32{0.0, 0.0, 1.0}

	s.CreateEntry(ctx, int(docID), &sec.ID, "Similar Entry", "Summary 1", "Content 1", float32ToBytes(similarVec), 1)
	s.CreateEntry(ctx, int(docID), &sec.ID, "Different Entry", "Summary 2", "Content 2", float32ToBytes(differentVec), 2)

	// Create mock embedder
	embedder := &mockEmbedder{
		vecs: map[string][]float32{
			"test query": queryVec,
		},
		dim: 3,
	}

	// Create tool and execute
	tool := NewSearchDocsTool(s, embedder)
	result, err := tool.Execute(ctx, map[string]any{
		"mode":       "search",
		"query":      "test query",
		"scope_type": "group",
		"scope_id":   float64(group.ID),
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Result is error: %s", result.Content)
	}

	// Parse JSON response
	var entries []struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Content), &entries); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify response - should return results ordered by similarity
	if len(entries) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// The most similar entry should be first
	if entries[0].Title != "Similar Entry" {
		t.Errorf("expected first result to be 'Similar Entry', got %s", entries[0].Title)
	}
}

// TestSearchDocsErrorCases tests error handling.
func TestSearchDocsErrorCases(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	tool := NewSearchDocsTool(s, nil)

	tests := []struct {
		name      string
		input     map[string]any
		wantError string
	}{
		{
			name:      "missing mode",
			input:     map[string]any{},
			wantError: "mode is required",
		},
		{
			name:      "invalid mode",
			input:     map[string]any{"mode": "invalid"},
			wantError: "invalid mode: invalid",
		},
		{
			name:      "sections missing scope_type",
			input:     map[string]any{"mode": "sections", "scope_id": 1},
			wantError: "scope_type and scope_id are required for sections mode",
		},
		{
			name:      "entries missing section_id",
			input:     map[string]any{"mode": "entries"},
			wantError: "section_id is required for entries mode",
		},
		{
			name:      "fetch missing entry_ids",
			input:     map[string]any{"mode": "fetch"},
			wantError: "entry_ids is required for fetch mode",
		},
		{
			name:      "fetch empty entry_ids",
			input:     map[string]any{"mode": "fetch", "entry_ids": []any{}},
			wantError: "entry_ids cannot be empty",
		},
		{
			name:      "search missing query",
			input:     map[string]any{"mode": "search", "scope_type": "group", "scope_id": 1},
			wantError: "query, scope_type, and scope_id are required for search mode",
		},
		{
			name:      "search without embedder",
			input:     map[string]any{"mode": "search", "query": "test", "scope_type": "group", "scope_id": 1},
			wantError: "search unavailable: embedder not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.input)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected IsError=true")
			}
			if result.Content != tt.wantError {
				t.Errorf("expected error %q, got %q", tt.wantError, result.Content)
			}
		})
	}
}

func TestSearchDocsToolNilStore(t *testing.T) {
	tool := NewSearchDocsTool(nil, nil)
	result, err := tool.Execute(context.Background(), map[string]any{
		"mode":       "sections",
		"scope_type": "group",
		"scope_id":   1,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true when store is nil")
	}
	if !strings.Contains(result.Content, "not configured") {
		t.Errorf("Expected 'not configured' in content, got: %s", result.Content)
	}
}
