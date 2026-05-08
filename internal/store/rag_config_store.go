package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/crypto"
)

// RagConfig holds the embedding configuration for the RAG knowledge base.
type RagConfig struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	BaseURL          string   `json:"base_url"`
	Model            string   `json:"model"`
	CachedModels     []string `json:"cached_models"`
	ValidatedAt      string   `json:"validated_at"`
	ClearValidatedAt bool     `json:"-"` // 强制清空验证状态（连接参数变化时）
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
		`SELECT name, type, base_url, model, encrypted_api_key, cached_models, validated_at FROM rag_config LIMIT 1`,
	)
	var c RagConfig
	var encKey, cachedModelsJSON string
	err := row.Scan(&c.Name, &c.Type, &c.BaseURL, &c.Model, &encKey, &cachedModelsJSON, &c.ValidatedAt)
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
	if cachedModelsJSON != "" {
		if err := json.Unmarshal([]byte(cachedModelsJSON), &c.CachedModels); err != nil {
			c.CachedModels = []string{}
		}
	}
	if c.CachedModels == nil {
		c.CachedModels = []string{}
	}
	return &c, nil
}

// SetValidatedAt updates only the validated_at field without touching other config.
func (s *RagConfigStore) SetValidatedAt(ts string) error {
	_, err := s.db.Exec(`UPDATE rag_config SET validated_at = ?`, ts)
	return err
}
func (s *RagConfigStore) Save(cfg *RagConfig) error {
	existing, err := s.Get()
	if err != nil {
		return err
	}
	if cfg.APIKey == "" && existing != nil {
		cfg.APIKey = existing.APIKey
	}
	encKey := ""
	if cfg.APIKey != "" {
		encKey, err = s.crypto.Encrypt(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt rag api key: %w", err)
		}
	}
	cachedModelsJSON := "[]"
	if len(cfg.CachedModels) > 0 {
		b, err := json.Marshal(cfg.CachedModels)
		if err == nil {
			cachedModelsJSON = string(b)
		}
	}
	// 保留现有 validated_at，除非调用方明确传入新值或 ClearValidatedAt 为 true
	validatedAt := cfg.ValidatedAt
	if !cfg.ClearValidatedAt && validatedAt == "" && existing != nil {
		validatedAt = existing.ValidatedAt
	}
	_, err = s.db.Exec(`DELETE FROM rag_config`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO rag_config (name, type, base_url, model, encrypted_api_key, cached_models, validated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cfg.Name, cfg.Type, cfg.BaseURL, cfg.Model, encKey, cachedModelsJSON, validatedAt,
	)
	if err == nil {
		cfg.ValidatedAt = validatedAt
	}
	return err
}
