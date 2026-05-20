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
	id, _ := res.LastInsertId()
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
	id, _ := res.LastInsertId()
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
