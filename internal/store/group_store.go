package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type GroupStore struct {
	db *sql.DB
}

func NewGroupStore(db *sql.DB) *GroupStore {
	return &GroupStore{db: db}
}

func (s *GroupStore) List() ([]*models.DocumentGroup, error) {
	rows, err := s.db.Query("SELECT id, name, created_at FROM document_groups ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.DocumentGroup
	for rows.Next() {
		var g models.DocumentGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &g)
	}
	return list, nil
}

func (s *GroupStore) Create(name string) (*models.DocumentGroup, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec("INSERT INTO document_groups (name, created_at) VALUES (?, ?)", name, now)
	if err != nil {
		return nil, fmt.Errorf("insert group: %w", err)
	}
	id, _ := res.LastInsertId()
	return &models.DocumentGroup{ID: int(id), Name: name, CreatedAt: now}, nil
}

func (s *GroupStore) Rename(id int, name string) error {
	_, err := s.db.Exec("UPDATE document_groups SET name = ? WHERE id = ?", name, id)
	return err
}

func (s *GroupStore) Delete(id int) error {
	_, err := s.db.Exec("DELETE FROM document_groups WHERE id = ?", id)
	return err
}

func (s *GroupStore) MoveDocument(docID int, groupID *int) error {
	if groupID == nil {
		_, err := s.db.Exec("UPDATE documents SET group_id = NULL WHERE id = ?", docID)
		return err
	}
	_, err := s.db.Exec("UPDATE documents SET group_id = ? WHERE id = ?", *groupID, docID)
	return err
}
