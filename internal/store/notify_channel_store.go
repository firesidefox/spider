package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

// NotifyChannelStore provides CRUD for notify_channels.
// No mutex — matches HostStore pattern (sql.DB is concurrency-safe).
type NotifyChannelStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

// NewNotifyChannelStore creates a new NotifyChannelStore.
func NewNotifyChannelStore(db *sql.DB, cm *crypto.Manager) *NotifyChannelStore {
	return &NotifyChannelStore{db: db, crypto: cm}
}

// Create inserts a new notify channel. cfg is a JSON string encrypted before storage.
func (s *NotifyChannelStore) Create(name string, typ models.NotifyChannelType, cfg string) (*models.NotifyChannel, error) {
	enc, err := s.crypto.Encrypt(cfg)
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO notify_channels (name, type, encrypted_config, created_at, updated_at) VALUES (?,?,?,?,?)`,
		name, string(typ), enc, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &models.NotifyChannel{
		ID: id, Name: name, Type: typ, Config: cfg,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// List returns all notify channels with decrypted config.
func (s *NotifyChannelStore) List() ([]*models.NotifyChannel, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, encrypted_config, created_at, updated_at FROM notify_channels ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.NotifyChannel
	for rows.Next() {
		ch, err := scanNotifyChannel(rows, s.crypto)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// Update replaces name, type, and config for the given channel. cfg is a JSON string.
func (s *NotifyChannelStore) Update(id int64, name string, typ models.NotifyChannelType, cfg string) (*models.NotifyChannel, error) {
	enc, err := s.crypto.Encrypt(cfg)
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(
		`UPDATE notify_channels SET name=?, type=?, encrypted_config=?, updated_at=? WHERE id=?`,
		name, string(typ), enc, now, id,
	)
	if err != nil {
		return nil, err
	}
	return &models.NotifyChannel{
		ID: id, Name: name, Type: typ, Config: cfg,
		UpdatedAt: now,
	}, nil
}

// Delete removes a notify channel by ID.
func (s *NotifyChannelStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notify_channels WHERE id=?`, id)
	return err
}

// GetByID returns a single channel with decrypted config.
func (s *NotifyChannelStore) GetByID(id int64) (*models.NotifyChannel, error) {
	row := s.db.QueryRow(
		`SELECT id, name, type, encrypted_config, created_at, updated_at FROM notify_channels WHERE id=?`, id,
	)
	return scanNotifyChannel(row, s.crypto)
}

type notifyChannelScanner interface {
	Scan(dest ...any) error
}

func scanNotifyChannel(sc notifyChannelScanner, cm *crypto.Manager) (*models.NotifyChannel, error) {
	var ch models.NotifyChannel
	var encCfg string
	var typ string
	if err := sc.Scan(&ch.ID, &ch.Name, &typ, &encCfg, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		return nil, err
	}
	ch.Type = models.NotifyChannelType(typ)
	plain, err := cm.Decrypt(encCfg)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}
	ch.Config = plain
	return &ch, nil
}
