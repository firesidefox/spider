package rag

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type Store struct {
	docs     *store.DocumentStore
	db       *sql.DB
	embedder Embedder
}

func NewStore(db *sql.DB, docs *store.DocumentStore, embedder Embedder) *Store {
	return &Store{docs: docs, db: db, embedder: embedder}
}

func (s *Store) Ingest(ctx context.Context, vendor, cliType, docType, title, content, sourceFile string, chunkIndex int) error {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	return s.docs.Save(vendor, cliType, docType, title, content, serializeVec(vec), sourceFile, chunkIndex)
}

func (s *Store) Search(ctx context.Context, query, vendor, cliType string, topK int) ([]*models.Document, error) {
	qvec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, vendor, cli_type, doc_type, title, content, embedding, source_file, chunk_index, created_at FROM documents WHERE vendor = ? AND cli_type = ? AND embedding IS NOT NULL",
		vendor, cliType,
	)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	type scored struct {
		doc   *models.Document
		score float32
	}
	var candidates []scored

	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.Vendor, &d.CLIType, &d.DocType, &d.Title, &d.Content, &d.Embedding, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		dvec := deserializeVec(d.Embedding)
		if len(dvec) == 0 {
			continue
		}
		candidates = append(candidates, scored{doc: &d, score: cosineSimilarity(qvec, dvec)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if topK > 0 && len(candidates) > topK {
		candidates = candidates[:topK]
	}

	out := make([]*models.Document, len(candidates))
	for i, c := range candidates {
		out[i] = c.doc
	}
	return out, nil
}

func serializeVec(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func deserializeVec(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}
