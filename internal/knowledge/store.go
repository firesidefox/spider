package knowledge

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/rag"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateGroup(ctx context.Context, name string) (*Group, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_groups (name, created_at) VALUES (?, ?)`, name, now)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &Group{ID: int(id), Name: name, CreatedAt: now}, nil
}

func (s *Store) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM knowledge_groups ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		g := Group{}
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
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

func (s *Store) ListDocuments(ctx context.Context, groupID int) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at
		FROM knowledge_documents
		WHERE group_id = ?
		ORDER BY id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.RawContent, &d.Filename,
			&d.Status, &d.ErrorMsg, &d.EntryCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDocument(ctx context.Context, docID int) (*Document, error) {
	var d Document
	err := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at
		FROM knowledge_documents
		WHERE id = ?`, docID).Scan(&d.ID, &d.GroupID, &d.Name, &d.DocType, &d.RawContent, &d.Filename,
		&d.Status, &d.ErrorMsg, &d.EntryCount, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %d", docID)
		}
		return nil, err
	}
	return &d, nil
}

func (s *Store) DeleteDocuments(ctx context.Context, docIDs []int) error {
	return s.deleteByIDs(ctx, "knowledge_documents", docIDs)
}

func (s *Store) DeleteGroups(ctx context.Context, groupIDs []int) error {
	return s.deleteByIDs(ctx, "knowledge_groups", groupIDs)
}

func (s *Store) deleteByIDs(ctx context.Context, table string, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE id IN (%s)`, table, placeholders), args...)
	return err
}

func (s *Store) MoveDocuments(ctx context.Context, docIDs []int, targetGroupID int) error {
	if len(docIDs) == 0 {
		return nil
	}
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM knowledge_groups WHERE id = ?`, targetGroupID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("target group not found: %d", targetGroupID)
		}
		return err
	}
	placeholders := strings.Repeat("?,", len(docIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]interface{}, 0, len(docIDs)+2)
	args = append(args, targetGroupID, time.Now().UTC())
	for _, id := range docIDs {
		args = append(args, id)
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE knowledge_documents SET group_id = ?, updated_at = ? WHERE id IN (%s)`, placeholders),
		args...)
	return err
}

func (s *Store) CreateSection(ctx context.Context, documentID int, name, summary string, position int) (*Section, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_sections (document_id, name, summary, position) VALUES (?, ?, ?, ?)`,
		documentID, name, summary, position)
	if err != nil {
		return nil, fmt.Errorf("create section: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &Section{
		ID:         int(id),
		DocumentID: documentID,
		Name:       name,
		Summary:    summary,
		Position:   position,
	}, nil
}

func (s *Store) ListSections(ctx context.Context, documentID int) ([]Section, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.document_id, s.name, s.summary, s.position, COUNT(e.id) as entry_count
		FROM knowledge_sections s
		LEFT JOIN knowledge_entries e ON e.section_id = s.id
		WHERE s.document_id = ?
		GROUP BY s.id, s.document_id, s.name, s.summary, s.position
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

func (s *Store) CreateEntry(ctx context.Context, documentID int, sectionID *int, title, summary, content string, embedding []byte, position int) (*Entry, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_entries (document_id, section_id, title, summary, content, embedding, position)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		documentID, sectionID, title, summary, content, embedding, position)
	if err != nil {
		return nil, fmt.Errorf("create entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &Entry{
		ID:         int(id),
		DocumentID: documentID,
		SectionID:  sectionID,
		Title:      title,
		Summary:    summary,
		Content:    content,
		Embedding:  embedding,
		Position:   position,
	}, nil
}

func (s *Store) ListEntries(ctx context.Context, documentID int) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, document_id, section_id, title, summary, content, embedding, position
		FROM knowledge_entries
		WHERE document_id = ?
		ORDER BY position`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.DocumentID, &e.SectionID, &e.Title, &e.Summary, &e.Content, &e.Embedding, &e.Position); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CatalogSections returns all sections within a given scope (group or document)
// with entry counts computed via LEFT JOIN.
func (s *Store) CatalogSections(ctx context.Context, scope Scope) ([]Section, error) {
	var query string
	var args []interface{}

	switch scope.Type {
	case "group":
		// JOIN through documents to filter by group_id
		query = `
			SELECT s.id, s.document_id, s.name, s.summary, s.position, COUNT(e.id) as entry_count
			FROM knowledge_sections s
			INNER JOIN knowledge_documents d ON s.document_id = d.id
			LEFT JOIN knowledge_entries e ON e.section_id = s.id
			WHERE d.group_id = ?
			GROUP BY s.id, s.document_id, s.name, s.summary, s.position
			ORDER BY s.position`
		args = []interface{}{scope.ID}

	case "document":
		// Direct filter on document_id
		query = `
			SELECT s.id, s.document_id, s.name, s.summary, s.position, COUNT(e.id) as entry_count
			FROM knowledge_sections s
			LEFT JOIN knowledge_entries e ON e.section_id = s.id
			WHERE s.document_id = ?
			GROUP BY s.id, s.document_id, s.name, s.summary, s.position
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

// CatalogEntries returns lightweight entry summaries for a given section.
func (s *Store) CatalogEntries(ctx context.Context, sectionID int) ([]EntrySummary, error) {
	query := `
		SELECT id, title, summary
		FROM knowledge_entries
		WHERE section_id = ?
		ORDER BY position`

	rows, err := s.db.QueryContext(ctx, query, sectionID)
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

// FetchEntries returns full entry content for specific entry IDs.
func (s *Store) FetchEntries(ctx context.Context, entryIDs []int) ([]Entry, error) {
	// Handle empty input
	if len(entryIDs) == 0 {
		return []Entry{}, nil
	}

	// Build IN clause dynamically
	placeholders := make([]string, len(entryIDs))
	args := make([]interface{}, len(entryIDs))
	for i, id := range entryIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, title, content
		FROM knowledge_entries
		WHERE id IN (%s)
		ORDER BY position`, strings.Join(placeholders, ","))

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

// Search performs vector similarity search within a given scope.
// Returns top-K entries sorted by cosine similarity to the query embedding.
func (s *Store) Search(ctx context.Context, queryEmb []byte, scope Scope, topK int) ([]Entry, error) {
	// Validate query embedding
	if len(queryEmb) == 0 {
		return nil, fmt.Errorf("query embedding cannot be empty")
	}

	// Build query based on scope type
	var query string
	var args []interface{}

	switch scope.Type {
	case "group":
		query = `
			SELECT e.id, e.document_id, e.section_id, e.title, e.summary, e.content, e.embedding, e.position
			FROM knowledge_entries e
			JOIN knowledge_documents d ON e.document_id = d.id
			WHERE d.group_id = ? AND e.embedding IS NOT NULL`
		args = []interface{}{scope.ID}

	case "document":
		query = `
			SELECT e.id, e.document_id, e.section_id, e.title, e.summary, e.content, e.embedding, e.position
			FROM knowledge_entries e
			WHERE e.document_id = ? AND e.embedding IS NOT NULL`
		args = []interface{}{scope.ID}

	default:
		return nil, fmt.Errorf("invalid scope type: %s", scope.Type)
	}

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect entries with similarity scores
	type entryWithScore struct {
		entry Entry
		score float64
	}
	var candidates []entryWithScore

	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.DocumentID, &e.SectionID, &e.Title, &e.Summary, &e.Content, &e.Embedding, &e.Position); err != nil {
			return nil, err
		}

		// Compute cosine similarity
		score := cosineSimilarity(queryEmb, e.Embedding)
		candidates = append(candidates, entryWithScore{entry: e, score: score})
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

// SearchByQuery embeds the query string and performs vector similarity search.
// This is the high-level interface preferred by callers; Search is the lower-level
// path for callers that already hold a precomputed embedding.
func (s *Store) SearchByQuery(ctx context.Context, query string, scope Scope, topK int, embedder rag.Embedder) ([]Entry, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	emb, err := embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	buf := make([]byte, len(emb)*4)
	for i, f := range emb {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return s.Search(ctx, buf, scope, topK)
}

// cosineSimilarity computes the cosine similarity between two embedding vectors.
// Embeddings are stored as little-endian float32 byte arrays.
func cosineSimilarity(a, b []byte) float64 {
	if len(a) != len(b) || len(a)%4 != 0 {
		return 0.0
	}

	n := len(a) / 4
	var dotProduct, normA, normB float64

	for i := 0; i < n; i++ {
		valA := float64(bytesToFloat32(a[i*4 : (i+1)*4]))
		valB := float64(bytesToFloat32(b[i*4 : (i+1)*4]))

		dotProduct += valA * valB
		normA += valA * valA
		normB += valB * valB
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// bytesToFloat32 converts 4 bytes (little-endian) to float32.
func bytesToFloat32(b []byte) float32 {
	bits := binary.LittleEndian.Uint32(b)
	return math.Float32frombits(bits)
}

// float32SliceToBytes encodes a float32 slice as little-endian bytes.
func float32SliceToBytes(floats []float32) []byte {
	out := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		out[i*4] = byte(bits)
		out[i*4+1] = byte(bits >> 8)
		out[i*4+2] = byte(bits >> 16)
		out[i*4+3] = byte(bits >> 24)
	}
	return out
}

// makeRange returns a slice of ints [start, end).
func makeRange(start, end int) []int {
	result := make([]int, end-start)
	for i := range result {
		result[i] = start + i
	}
	return result
}

func (s *Store) updateDocumentEntryCount(ctx context.Context, docID, count int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET entry_count = ?, updated_at = ? WHERE id = ?`,
		count, time.Now().UTC(), docID)
	return err
}

func (s *Store) setDocumentStatus(ctx context.Context, docID int, status, errorMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET status = ?, error_msg = ?, updated_at = ? WHERE id = ?`,
		status, errorMsg, time.Now().UTC(), docID)
	return err
}

// createDocument inserts a new document record and returns it.
func (s *Store) createDocument(ctx context.Context, groupID int, name, docType, rawContent, filename, status string) (*Document, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge_documents (group_id, name, doc_type, raw_content, filename, status, error_msg, entry_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '', 0, ?, ?)`,
		groupID, name, docType, rawContent, filename, status, now, now)
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &Document{
		ID: int(id), GroupID: groupID, Name: name, DocType: docType,
		RawContent: rawContent, Filename: filename, Status: status,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// ImportDocument orchestrates the parse → cluster → embed → write pipeline.
func (s *Store) ImportDocument(ctx context.Context, req ImportRequest) (*ImportResult, error) {
	// Determine doc type
	docType := req.DocType
	if docType == "" {
		docType = string(DetectDocType(req.Content, req.Filename))
	}

	// Create document record with status "indexing"
	doc, err := s.createDocument(ctx, req.GroupID, req.Name, docType, string(req.Content), req.Filename, "indexing")
	if err != nil {
		return nil, fmt.Errorf("import document: %w", err)
	}

	result, err := s.runImportPipeline(ctx, doc, req)
	if err != nil {
		_ = s.setDocumentStatus(ctx, doc.ID, "error", err.Error())
		return nil, err
	}
	return result, nil
}

// ReindexDocument re-runs the import pipeline for an existing document using
// its stored raw_content. Existing sections/entries are removed first.
// req.LLMClient and req.Embedder are honored; other fields are overridden
// from the persisted document.
func (s *Store) ReindexDocument(ctx context.Context, docID int, req ImportRequest) (*ImportResult, error) {
	doc, err := s.GetDocument(ctx, docID)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM knowledge_entries WHERE document_id = ?`, docID); err != nil {
		return nil, fmt.Errorf("clear entries: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM knowledge_sections WHERE document_id = ?`, docID); err != nil {
		return nil, fmt.Errorf("clear sections: %w", err)
	}
	if err := s.setDocumentStatus(ctx, docID, "indexing", ""); err != nil {
		return nil, err
	}
	if err := s.updateDocumentEntryCount(ctx, docID, 0); err != nil {
		return nil, err
	}
	req.GroupID = doc.GroupID
	req.Name = doc.Name
	req.Content = []byte(doc.RawContent)
	req.Filename = doc.Filename
	req.DocType = doc.DocType
	result, err := s.runImportPipeline(ctx, doc, req)
	if err != nil {
		_ = s.setDocumentStatus(ctx, docID, "error", err.Error())
		return nil, err
	}
	return result, nil
}

// runImportPipeline executes parse → cluster → embed → write for an already-created document.
func (s *Store) runImportPipeline(ctx context.Context, doc *Document, req ImportRequest) (*ImportResult, error) {
	// Parse entries
	entries, err := s.parseEntries(ctx, doc.DocType, req)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	// Cluster into sections
	sections, err := s.clusterToSections(ctx, entries, req)
	if err != nil {
		return nil, fmt.Errorf("cluster: %w", err)
	}

	// Write sections and entries to DB
	result, err := s.writeSectionsAndEntries(ctx, doc.ID, entries, sections, req)
	if err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Update document status
	if err := s.updateDocumentEntryCount(ctx, doc.ID, result.EntryCount); err != nil {
		return nil, err
	}
	if err := s.setDocumentStatus(ctx, doc.ID, "ready", ""); err != nil {
		return nil, err
	}

	result.DocumentID = doc.ID
	return result, nil
}

// parseEntries selects the right parser and returns parsed entries.
func (s *Store) parseEntries(ctx context.Context, docType string, req ImportRequest) ([]ParsedEntry, error) {
	var parser Parser
	switch docType {
	case string(DocTypeOpenAPI):
		parser = &OpenAPIParser{}
	case string(DocTypeMarkdown):
		if req.LLMClient == nil {
			return nil, fmt.Errorf("LLMClient required for markdown parsing")
		}
		parser = NewMarkdownParser(req.LLMClient)
	default:
		return nil, fmt.Errorf("unsupported doc type: %s", docType)
	}
	return parser.Parse(ctx, req.Content, req.Filename)
}

// clusterToSections groups entries into sections using LLM, or returns a single catch-all section.
func (s *Store) clusterToSections(ctx context.Context, entries []ParsedEntry, req ImportRequest) (*ClusterResult, error) {
	if req.LLMClient == nil {
		// No LLM: single section containing all entries
		return singleSectionCluster(entries), nil
	}
	cluster, err := ClusterEntries(ctx, req.LLMClient, entries)
	if err != nil {
		slog.WarnContext(ctx, "knowledge clustering failed; falling back to single section", "error", err)
		return singleSectionCluster(entries), nil
	}
	return cluster, nil
}

func singleSectionCluster(entries []ParsedEntry) *ClusterResult {
	return &ClusterResult{
		Sections: []ClusteredSection{{
			Name:     "All Entries",
			Summary:  "",
			EntryIDs: makeRange(0, len(entries)),
		}},
	}
}

// writeSectionsAndEntries persists sections and entries to the DB, generating embeddings if available.
func (s *Store) writeSectionsAndEntries(ctx context.Context, docID int, entries []ParsedEntry, cluster *ClusterResult, req ImportRequest) (*ImportResult, error) {
	result := &ImportResult{}

	for pos, cs := range cluster.Sections {
		sec, err := s.CreateSection(ctx, docID, cs.Name, cs.Summary, pos)
		if err != nil {
			return nil, err
		}
		result.Sections = append(result.Sections, *sec)

		for entryPos, entryIdx := range cs.EntryIDs {
			if entryIdx < 0 || entryIdx >= len(entries) {
				continue
			}
			e := entries[entryIdx]

			var embedding []byte
			if req.Embedder != nil && e.Summary != "" {
				vec, err := req.Embedder.Embed(ctx, e.Summary)
				if err != nil {
					return nil, fmt.Errorf("embed entry %d: %w", entryIdx, err)
				}
				embedding = float32SliceToBytes(vec)
			}

			secID := sec.ID
			if _, err := s.CreateEntry(ctx, docID, &secID, e.Title, e.Summary, e.Content, embedding, entryPos); err != nil {
				return nil, err
			}
			result.EntryCount++
		}
	}

	result.SectionCount = len(cluster.Sections)
	return result, nil
}

// Ensure Store implements KnowledgePlugin (compile-time check).
var _ KnowledgePlugin = (*Store)(nil)
