package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/spiderai/spider/internal/models"
)

var ErrTokenNotFound = errors.New("token not found")

// TokenStore 提供 API Token 的 CRUD 操作。
type TokenStore struct {
	db *sql.DB
}

// NewTokenStore 创建一个新的 TokenStore。
func NewTokenStore(db *sql.DB) *TokenStore {
	return &TokenStore{db: db}
}

// Create 创建 Token（tokenHash 为 SHA-256 hex）。
func (s *TokenStore) Create(userID, name, tokenHash string, expiresAt *time.Time) (*models.ApiToken, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO api_tokens (id, user_id, name, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, name, tokenHash, expiresAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建 token 失败: %w", err)
	}
	return s.getByID(id)
}

// GetByHash 按 token hash 查询（用于认证）。
func (s *TokenStore) GetByHash(hash string) (*models.ApiToken, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, token_hash, expires_at, created_at, last_used
		 FROM api_tokens WHERE token_hash = ?`, hash,
	)
	return scanToken(row)
}

// ListByUser 列出用户的所有 token。
func (s *TokenStore) ListByUser(userID string) ([]*models.ApiToken, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, token_hash, expires_at, created_at, last_used
		 FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 token 列表失败: %w", err)
	}
	defer rows.Close()
	var tokens []*models.ApiToken
	for rows.Next() {
		t, err := scanTokenRows(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// Delete 删除 token。
func (s *TokenStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除 token 失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// UpdateLastUsed 异步更新最后使用时间（fire-and-forget）。
func (s *TokenStore) UpdateLastUsed(id string) {
	go func() {
		now := time.Now().UTC()
		_, _ = s.db.Exec(`UPDATE api_tokens SET last_used=? WHERE id=?`, now, id)
	}()
}

// getByID 按 ID 查询 token（内部使用）。
func (s *TokenStore) getByID(id string) (*models.ApiToken, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, token_hash, expires_at, created_at, last_used
		 FROM api_tokens WHERE id = ?`, id,
	)
	return scanToken(row)
}

// scanToken 从 *sql.Row 扫描一个 ApiToken。
func scanToken(row *sql.Row) (*models.ApiToken, error) {
	var t models.ApiToken
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt, &t.LastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("扫描 token 数据失败: %w", err)
	}
	return &t, nil
}

// scanTokenRows 从 *sql.Rows 扫描一个 ApiToken。
func scanTokenRows(rows *sql.Rows) (*models.ApiToken, error) {
	var t models.ApiToken
	err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt, &t.LastUsed)
	if err != nil {
		return nil, fmt.Errorf("扫描 token 数据失败: %w", err)
	}
	return &t, nil
}
