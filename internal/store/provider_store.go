package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
)

// ProviderStore 提供 LLM provider 的 CRUD 操作。
type ProviderStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

// NewProviderStore 创建一个新的 ProviderStore。
func NewProviderStore(db *sql.DB, cm *crypto.Manager) *ProviderStore {
	return &ProviderStore{db: db, crypto: cm}
}

// Create 创建新 provider，API key 加密后存储。
func (s *ProviderStore) Create(name, providerType, apiKey, baseURL string) (*models.Provider, error) {
	encKey, err := s.crypto.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("加密 API key 失败: %w", err)
	}
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO providers (id, name, type, encrypted_api_key, base_url, selected_model, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, '', 0, ?, ?)`,
		id, name, providerType, encKey, baseURL, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("插入 provider 失败: %w", err)
	}
	return s.GetByID(id)
}

// GetByID 按 ID 查询 provider。
func (s *ProviderStore) GetByID(id string) (*models.Provider, error) {
	row := s.db.QueryRow(
		`SELECT id, name, type, encrypted_api_key, base_url, selected_model, is_active, created_at, updated_at
		 FROM providers WHERE id = ?`, id,
	)
	return scanProvider(row)
}

// List 列出所有 provider，按创建时间排序。
func (s *ProviderStore) List() ([]*models.Provider, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, encrypted_api_key, base_url, selected_model, is_active, created_at, updated_at
		 FROM providers ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 provider 列表失败: %w", err)
	}
	defer rows.Close()
	var providers []*models.Provider
	for rows.Next() {
		p, err := scanProviderRows(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// Update 更新 provider 信息，仅更新非 nil 字段。
func (s *ProviderStore) Update(id string, name, providerType, apiKey, baseURL *string) (*models.Provider, error) {
	p, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		p.Name = *name
	}
	if providerType != nil {
		p.Type = *providerType
	}
	if apiKey != nil {
		p.EncryptedAPIKey, err = s.crypto.Encrypt(*apiKey)
		if err != nil {
			return nil, fmt.Errorf("加密 API key 失败: %w", err)
		}
	}
	if baseURL != nil {
		p.BaseURL = *baseURL
	}
	p.UpdatedAt = time.Now().UTC()
	_, err = s.db.Exec(
		`UPDATE providers SET name=?, type=?, encrypted_api_key=?, base_url=?, updated_at=? WHERE id=?`,
		p.Name, p.Type, p.EncryptedAPIKey, p.BaseURL, p.UpdatedAt, id,
	)
	if err != nil {
		return nil, fmt.Errorf("更新 provider 失败: %w", err)
	}
	return p, nil
}

// Delete 删除 provider（级联删除 provider_models）。
func (s *ProviderStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除 provider 失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("provider 不存在: %s", id)
	}
	return nil
}

// Activate 激活指定 provider，同时停用其他所有 provider。
func (s *ProviderStore) Activate(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE providers SET is_active = 0 WHERE is_active = 1`); err != nil {
		return fmt.Errorf("停用 provider 失败: %w", err)
	}
	if _, err := tx.Exec(`UPDATE providers SET is_active = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("激活 provider 失败: %w", err)
	}
	return tx.Commit()
}

// SetSelectedModel 设置 provider 的选中模型。
func (s *ProviderStore) SetSelectedModel(id, model string) error {
	_, err := s.db.Exec(
		`UPDATE providers SET selected_model = ?, updated_at = ? WHERE id = ?`,
		model, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("设置选中模型失败: %w", err)
	}
	return nil
}

// GetActive 返回当前激活的 provider，若无则返回 nil, nil。
func (s *ProviderStore) GetActive() (*models.Provider, error) {
	row := s.db.QueryRow(
		`SELECT id, name, type, encrypted_api_key, base_url, selected_model, is_active, created_at, updated_at
		 FROM providers WHERE is_active = 1 LIMIT 1`,
	)
	p, err := scanProvider(row)
	if err != nil && err.Error() == "provider 不存在" {
		return nil, nil
	}
	return p, err
}

// DecryptAPIKey 解密 provider 的 API key。
func (s *ProviderStore) DecryptAPIKey(p *models.Provider) (string, error) {
	key, err := s.crypto.Decrypt(p.EncryptedAPIKey)
	if err != nil {
		return "", fmt.Errorf("解密 API key 失败: %w", err)
	}
	return key, nil
}

// CountAll 返回 provider 总数。
func (s *ProviderStore) CountAll() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM providers`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计 provider 数量失败: %w", err)
	}
	return count, nil
}

// SaveModels 替换 provider 的模型列表。
func (s *ProviderStore) SaveModels(providerID string, modelList []llm.ModelInfo) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM provider_models WHERE provider_id = ?`, providerID); err != nil {
		return fmt.Errorf("删除旧模型失败: %w", err)
	}
	now := time.Now().UTC()
	for _, m := range modelList {
		if _, err := tx.Exec(
			`INSERT INTO provider_models (provider_id, model_id, display_name, created_at) VALUES (?, ?, ?, ?)`,
			providerID, m.ID, m.DisplayName, now,
		); err != nil {
			return fmt.Errorf("插入模型 %s 失败: %w", m.ID, err)
		}
	}
	return tx.Commit()
}

// ListModels 列出 provider 的所有模型，按 model_id 排序。
func (s *ProviderStore) ListModels(providerID string) ([]*models.ProviderModel, error) {
	rows, err := s.db.Query(
		`SELECT id, provider_id, model_id, display_name, created_at
		 FROM provider_models WHERE provider_id = ? ORDER BY model_id`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}
	defer rows.Close()
	var list []*models.ProviderModel
	for rows.Next() {
		var m models.ProviderModel
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描模型数据失败: %w", err)
		}
		list = append(list, &m)
	}
	return list, rows.Err()
}

// scanProvider 从 *sql.Row 扫描一个 Provider。
func scanProvider(row *sql.Row) (*models.Provider, error) {
	var p models.Provider
	var isActive int
	err := row.Scan(
		&p.ID, &p.Name, &p.Type, &p.EncryptedAPIKey,
		&p.BaseURL, &p.SelectedModel, &isActive,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("provider 不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("扫描 provider 数据失败: %w", err)
	}
	p.IsActive = isActive == 1
	return &p, nil
}

// scanProviderRows 从 *sql.Rows 扫描一个 Provider。
func scanProviderRows(rows *sql.Rows) (*models.Provider, error) {
	var p models.Provider
	var isActive int
	err := rows.Scan(
		&p.ID, &p.Name, &p.Type, &p.EncryptedAPIKey,
		&p.BaseURL, &p.SelectedModel, &isActive,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("扫描 provider 数据失败: %w", err)
	}
	p.IsActive = isActive == 1
	return &p, nil
}
