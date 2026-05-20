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
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &KnowledgeBase{ID: int(id), Name: name, CreatedAt: now}, nil
}

func (s *Store) ListKBs(ctx context.Context) ([]KnowledgeBase, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at FROM knowledge_bases ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KnowledgeBase
	for rows.Next() {
		kb := KnowledgeBase{}
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
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &Group{ID: int(id), KBID: kbID, Name: name, CreatedAt: now}, nil
}

func (s *Store) ListGroups(ctx context.Context, kbID int) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kb_id, name, created_at FROM knowledge_groups WHERE kb_id = ? ORDER BY id`, kbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		g := Group{}
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
	if len(docIDs) == 0 {
		return nil
	}

	// Build IN clause with placeholders
	query := `DELETE FROM knowledge_documents WHERE id IN (`
	args := make([]interface{}, len(docIDs))
	for i, id := range docIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	_, err := s.db.ExecContext(ctx, query, args...)
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

// CatalogSections returns all sections within a given scope (kb, group, or document)
// with entry counts computed via LEFT JOIN.
func (s *Store) CatalogSections(ctx context.Context, scope Scope) ([]Section, error) {
	var query string
	var args []interface{}

	switch scope.Type {
	case "kb":
		// JOIN through documents and groups to filter by kb_id
		query = `
			SELECT s.id, s.document_id, s.name, s.summary, s.position, COUNT(e.id) as entry_count
			FROM knowledge_sections s
			INNER JOIN knowledge_documents d ON s.document_id = d.id
			INNER JOIN knowledge_groups g ON d.group_id = g.id
			LEFT JOIN knowledge_entries e ON e.section_id = s.id
			WHERE g.kb_id = ?
			GROUP BY s.id, s.document_id, s.name, s.summary, s.position
			ORDER BY s.position`
		args = []interface{}{scope.ID}

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
