package store

import (
	"database/sql"
	"fmt"

	"github.com/spiderai/spider/internal/crypto"
)

// RagConfig holds the embedding configuration for the RAG knowledge base.
type RagConfig struct {
	Type    string `json:"type"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	// APIKey is never returned in JSON responses
	APIKey string `json:"-"`
}

type RagConfigStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

func NewRagConfigStore(db *sql.DB, cm *crypto.Manager) *RagConfigStore {
	return &RagConfigStore{db: db, crypto: cm}
}

// Get returns the current RAG config, or nil if not configured.
func (s *RagConfigStore) Get() (*RagConfig, error) {
	row := s.db.QueryRow(
		`SELECT type, base_url, model, encrypted_api_key FROM rag_config LIMIT 1`,
	)
	var c RagConfig
	var encKey string
	err := row.Scan(&c.Type, &c.BaseURL, &c.Model, &encKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan rag_config: %w", err)
	}
	if encKey != "" {
		c.APIKey, err = s.crypto.Decrypt(encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt rag api key: %w", err)
		}
	}
	return &c, nil
}

// Save upserts the RAG config (single-row table). Empty APIKey preserves the existing key.
func (s *RagConfigStore) Save(cfg *RagConfig) error {
	if cfg.APIKey == "" {
		existing, err := s.Get()
		if err != nil {
			return err
		}
		if existing != nil {
			cfg.APIKey = existing.APIKey
		}
	}
	encKey := ""
	if cfg.APIKey != "" {
		var err error
		encKey, err = s.crypto.Encrypt(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt rag api key: %w", err)
		}
	}
	_, err := s.db.Exec(`DELETE FROM rag_config`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO rag_config (type, base_url, model, encrypted_api_key) VALUES (?, ?, ?, ?)`,
		cfg.Type, cfg.BaseURL, cfg.Model, encKey,
	)
	return err
}
