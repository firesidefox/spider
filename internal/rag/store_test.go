package rag

import (
	"context"
	"database/sql"
	"testing"

	"github.com/spiderai/spider/internal/db"
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

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
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
