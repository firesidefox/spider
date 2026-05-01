package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

var ErrSSHKeyNotFound = errors.New("ssh key not found")

// SSHKeyStore 提供 SSH 密钥的 CRUD 操作。
type SSHKeyStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

// NewSSHKeyStore 创建一个新的 SSHKeyStore。
func NewSSHKeyStore(db *sql.DB, cm *crypto.Manager) *SSHKeyStore {
	return &SSHKeyStore{db: db, crypto: cm}
}

// Add 添加新 SSH 密钥，私钥和 passphrase 加密后存储。
func (s *SSHKeyStore) Add(userID string, req *models.AddSSHKeyRequest) (*models.SSHKey, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name 不能为空")
	}
	if strings.TrimSpace(req.PrivateKey) == "" {
		return nil, fmt.Errorf("private_key 不能为空")
	}

	fingerprint, err := s.parseFingerprint(req.PrivateKey, req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("解析密钥失败: %w", err)
	}

	encKey, err := s.crypto.Encrypt(req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("加密私钥失败: %w", err)
	}
	encPass, err := s.crypto.Encrypt(req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("加密 passphrase 失败: %w", err)
	}

	id := "k_" + uuid.New().String()[:8]
	now := time.Now().UTC()

	_, err = s.db.Exec(
		`INSERT INTO ssh_keys (id, user_id, name, encrypted_private_key,
		 encrypted_passphrase, fingerprint, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, req.Name, encKey, encPass, fingerprint, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, fmt.Errorf("密钥名 %q 已存在", req.Name)
		}
		return nil, fmt.Errorf("插入 SSH 密钥失败: %w", err)
	}
	return s.GetByID(id)
}

// parseFingerprint 解析私钥并返回 SHA256 指纹。
func (s *SSHKeyStore) parseFingerprint(privateKey, passphrase string) (string, error) {
	var signer gossh.Signer
	var err error
	if passphrase != "" {
		signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
	} else {
		signer, err = gossh.ParsePrivateKey([]byte(privateKey))
	}
	if err != nil {
		return "", err
	}
	return gossh.FingerprintSHA256(signer.PublicKey()), nil
}

// GetByID 按 ID 查询 SSH 密钥。
func (s *SSHKeyStore) GetByID(id string) (*models.SSHKey, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, encrypted_private_key, encrypted_passphrase,
		 fingerprint, created_at, updated_at
		 FROM ssh_keys WHERE id = ?`, id,
	)
	return scanSSHKey(row)
}

// ListByUser 列出用户的所有 SSH 密钥。
func (s *SSHKeyStore) ListByUser(userID string) ([]*models.SSHKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, encrypted_private_key, encrypted_passphrase,
		 fingerprint, created_at, updated_at
		 FROM ssh_keys WHERE user_id = ? ORDER BY name`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 SSH 密钥列表失败: %w", err)
	}
	defer rows.Close()
	var keys []*models.SSHKey
	for rows.Next() {
		k, err := scanSSHKeyRows(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// DecryptKey 解密 SSH 密钥的私钥和 passphrase。
func (s *SSHKeyStore) DecryptKey(k *models.SSHKey) (privateKey, passphrase string, err error) {
	privateKey, err = s.crypto.Decrypt(k.EncryptedPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("解密私钥失败: %w", err)
	}
	passphrase, err = s.crypto.Decrypt(k.EncryptedPassphrase)
	if err != nil {
		return "", "", fmt.Errorf("解密 passphrase 失败: %w", err)
	}
	return privateKey, passphrase, nil
}

// Delete 删除 SSH 密钥（若被主机引用则拒绝）。
func (s *SSHKeyStore) Delete(id, userID string) error {
	refCount, err := s.GetRefCount(id)
	if err != nil {
		return err
	}
	if refCount > 0 {
		return fmt.Errorf("CONFLICT: 密钥仍被 %d 台主机引用，无法删除", refCount)
	}
	res, err := s.db.Exec(`DELETE FROM ssh_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("删除 SSH 密钥失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSSHKeyNotFound
	}
	return nil
}

// GetRefCount 返回引用该密钥的主机数量。
func (s *SSHKeyStore) GetRefCount(id string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE ssh_key_id = ?`, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("查询引用计数失败: %w", err)
	}
	return count, nil
}

// scanSSHKey 从 *sql.Row 扫描一个 SSHKey。
func scanSSHKey(row *sql.Row) (*models.SSHKey, error) {
	var k models.SSHKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.Name, &k.EncryptedPrivateKey,
		&k.EncryptedPassphrase, &k.Fingerprint,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSSHKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("扫描 SSH 密钥数据失败: %w", err)
	}
	return &k, nil
}

// scanSSHKeyRows 从 *sql.Rows 扫描一个 SSHKey。
func scanSSHKeyRows(rows *sql.Rows) (*models.SSHKey, error) {
	var k models.SSHKey
	err := rows.Scan(
		&k.ID, &k.UserID, &k.Name, &k.EncryptedPrivateKey,
		&k.EncryptedPassphrase, &k.Fingerprint,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("扫描 SSH 密钥数据失败: %w", err)
	}
	return &k, nil
}
