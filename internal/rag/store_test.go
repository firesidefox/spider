package rag

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/spiderai/spider/internal/store"
)

type mockEmbedder struct {
	vecs map[string][]float32
	dim  int
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if v, ok := m.vecs[text]; ok {
		return v, nil
	}
	v := make([]float32, m.dim)
	return v, nil
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

const createDocumentsSQL = `
CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vendor TEXT NOT NULL DEFAULT '',
    cli_type TEXT NOT NULL DEFAULT '',
    doc_type TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    embedding BLOB,
    source_file TEXT NOT NULL DEFAULT '',
    chunk_index INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);`

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(createDocumentsSQL); err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSerializeDeserializeVec(t *testing.T) {
	in := []float32{0.1, 0.2, 0.3, -0.5, 1.0}
	b := serializeVec(in)
	out := deserializeVec(b)
	if len(out) != len(in) {
		t.Fatalf("length mismatch: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("index %d: got %v want %v", i, out[i], in[i])
		}
	}
}

func TestIngestAndSearch(t *testing.T) {
	db := setupTestDB(t)
	docs := store.NewDocumentStore(db)
	emb := &mockEmbedder{
		dim: 3,
		vecs: map[string][]float32{
			"doc about routing":  {1, 0, 0},
			"doc about firewall": {0, 1, 0},
			"query routing":      {0.9, 0.1, 0},
		},
	}
	s := NewStore(db, docs, emb)
	ctx := context.Background()

	if err := s.Ingest(ctx, "cisco", "ios", "routing", "Routing Doc", "doc about routing", "routing.md", 0); err != nil {
		t.Fatalf("ingest routing: %v", err)
	}
	if err := s.Ingest(ctx, "cisco", "ios", "firewall", "Firewall Doc", "doc about firewall", "fw.md", 0); err != nil {
		t.Fatalf("ingest firewall: %v", err)
	}

	results, err := s.Search(ctx, "query routing", "cisco", "ios", 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "doc about routing" {
		t.Errorf("expected routing doc, got %q", results[0].Content)
	}
}

func TestSearchNoResults(t *testing.T) {
	db := setupTestDB(t)
	docs := store.NewDocumentStore(db)
	emb := &mockEmbedder{dim: 3, vecs: map[string][]float32{}}
	s := NewStore(db, docs, emb)
	ctx := context.Background()

	results, err := s.Search(ctx, "anything", "cisco", "ios", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
