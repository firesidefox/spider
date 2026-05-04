package rag

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
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

func (s *Store) Ingest(ctx context.Context, vendor string, tags []string, title, content, sourceFile string, chunkIndex int, groupID *int) error {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	return s.docs.Save(vendor, tags, title, content, serializeVec(vec), sourceFile, chunkIndex, groupID)
}

func (s *Store) Search(ctx context.Context, query, vendor string, topK int) ([]*models.Document, error) {
	qvec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	var rows *sql.Rows
	switch {
	case vendor != "":
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE vendor = ? AND embedding IS NOT NULL",
			vendor,
		)
	default:
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE embedding IS NOT NULL",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()
	return s.rankFromRows(rows, qvec, topK)
}

// SearchByGroup 按向量相似度检索文档。groupID 非 nil 时只检索该分组，nil 时全局检索。
// 注意：不按 vendor 过滤，如需 vendor 过滤请使用 Search。
func (s *Store) SearchByGroup(ctx context.Context, query string, groupID *int, topK int) ([]*models.Document, error) {
	qvec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	var rows *sql.Rows
	const baseQ = "SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE embedding IS NOT NULL"
	if groupID != nil {
		rows, err = s.db.QueryContext(ctx, baseQ+" AND group_id = ?", *groupID)
	} else {
		rows, err = s.db.QueryContext(ctx, baseQ)
	}
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()
	return s.rankFromRows(rows, qvec, topK)
}

// rankFromRows 从 sql.Rows 中读取文档，计算与 qvec 的余弦相似度，排序后返回 topK 条。
func (s *Store) rankFromRows(rows *sql.Rows, qvec []float32, topK int) ([]*models.Document, error) {
	type scored struct {
		doc   *models.Document
		score float32
	}
	var candidates []scored

	for rows.Next() {
		var d models.Document
		var tagsJSON string
		if err := rows.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.Embedding, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
			d.Tags = []string{}
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
