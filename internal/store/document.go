package store

import (
	"database/sql"
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

func (s *DocumentStore) Save(vendor, cliType, docType, title, content string, embedding []byte, sourceFile string, chunkIndex int) error {
	_, err := s.db.Exec(
		"INSERT INTO documents (vendor, cli_type, doc_type, title, content, embedding, source_file, chunk_index, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		vendor, cliType, docType, title, content, embedding, sourceFile, chunkIndex, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (s *DocumentStore) List() ([]*models.Document, error) {
	rows, err := s.db.Query("SELECT id, vendor, cli_type, doc_type, title, content, source_file, chunk_index, created_at FROM documents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func (s *DocumentStore) ListByVendor(vendor, cliType string) ([]*models.Document, error) {
	rows, err := s.db.Query(
		"SELECT id, vendor, cli_type, doc_type, title, content, source_file, chunk_index, created_at FROM documents WHERE vendor = ? AND cli_type = ? ORDER BY id",
		vendor, cliType,
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

func scanDocumentRows(rows *sql.Rows) ([]*models.Document, error) {
	var list []*models.Document
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.Vendor, &d.CLIType, &d.DocType, &d.Title, &d.Content, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		list = append(list, &d)
	}
	return list, nil
}
