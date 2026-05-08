package store

import (
	"database/sql"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

func (s *MemoryStore) Add(hostID string, req *models.AddMemoryRequest) (*models.Memory, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(`INSERT INTO host_memories (host_id,content,created_by,created_at)
		VALUES (?,?,?,?)`, hostID, req.Content, req.CreatedBy, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &models.Memory{
		ID:        int(id),
		HostID:    hostID,
		Content:   req.Content,
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}, nil
}

func (s *MemoryStore) ListByHost(hostID string) ([]*models.Memory, error) {
	rows, err := s.db.Query(`SELECT id,host_id,content,created_by,created_at
		FROM host_memories WHERE host_id=? ORDER BY created_at`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Memory
	for rows.Next() {
		var m models.Memory
		if err := rows.Scan(&m.ID, &m.HostID, &m.Content, &m.CreatedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *MemoryStore) Delete(hostID string, id int) error {
	res, err := s.db.Exec(`DELETE FROM host_memories WHERE id=? AND host_id=?`, id, hostID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
