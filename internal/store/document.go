package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type DocumentStore struct {
	db *sql.DB
}

func NewDocumentStore(db *sql.DB) *DocumentStore {
	return &DocumentStore{db: db}
}

func (s *DocumentStore) Save(vendor string, tags []string, title, content string, embedding []byte, sourceFile string, chunkIndex int, groupID *int) error {
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, _ := json.Marshal(tags)
	_, err := s.db.Exec(
		"INSERT INTO documents (vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		vendor, string(tagsJSON), title, content, embedding, sourceFile, chunkIndex, time.Now().UTC(), groupID,
	)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (s *DocumentStore) List() ([]*models.Document, error) {
	rows, err := s.db.Query("SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) ListByVendor(vendor string) ([]*models.Document, error) {
	rows, err := s.db.Query(
		"SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents WHERE vendor = ? ORDER BY id",
		vendor,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) ListByTag(tag string) ([]*models.Document, error) {
	rows, err := s.db.Query(
		"SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents WHERE EXISTS (SELECT 1 FROM json_each(tags) WHERE value = ?) ORDER BY id",
		tag,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) ListByGroup(groupID int) ([]*models.Document, error) {
	rows, err := s.db.Query(
		"SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents WHERE group_id = ? ORDER BY id",
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) DeleteBySource(sourceFile string) error {
	_, err := s.db.Exec("DELETE FROM documents WHERE source_file = ?", sourceFile)
	return err
}

func (s *DocumentStore) Delete(id int) error {
	_, err := s.db.Exec("DELETE FROM documents WHERE id = ?", id)
	return err
}

func (s *DocumentStore) GetByID(id int) (*models.Document, error) {
	row := s.db.QueryRow(
		"SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents WHERE id = ?",
		id,
	)
	var d models.Document
	var tagsJSON string
	err := row.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document by id: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
		d.Tags = []string{}
	}
	return &d, nil
}

// FindByTitle returns the first document matching groupID and title, or nil if not found.
func (s *DocumentStore) FindByTitle(groupID int, title string) (*models.Document, error) {
	row := s.db.QueryRow(
		"SELECT id, vendor, tags, title, content, source_file, chunk_index, created_at, group_id FROM documents WHERE group_id = ? AND title = ? LIMIT 1",
		groupID, title,
	)
	var d models.Document
	var tagsJSON string
	err := row.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find document by title: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
		d.Tags = []string{}
	}
	return &d, nil
}

func scanDocumentRows(rows *sql.Rows) ([]*models.Document, error) {
	list := make([]*models.Document, 0)
	for rows.Next() {
		var d models.Document
		var tagsJSON string
		if err := rows.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
			d.Tags = []string{}
		}
		list = append(list, &d)
	}
	return list, nil
}
