package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

// HostStore 提供主机的 CRUD 操作。
type HostStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

// NewHostStore 创建一个新的 HostStore。
func NewHostStore(db *sql.DB, cm *crypto.Manager) *HostStore {
	return &HostStore{db: db, crypto: cm}
}

// Add 添加新主机，凭据加密后存储。
func (s *HostStore) Add(req *models.AddHostRequest) (*models.Host, error) {
	if req.Port == 0 {
		req.Port = 22
	}
	if req.Name == "" || req.IP == "" || req.Username == "" {
		return nil, fmt.Errorf("name、ip、username 不能为空")
	}

	encCred, err := s.crypto.Encrypt(req.Credential)
	if err != nil {
		return nil, fmt.Errorf("加密凭据失败: %w", err)
	}
	encPass, err := s.crypto.Encrypt(req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("加密 passphrase 失败: %w", err)
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	now := time.Now().UTC()
	id := uuid.New().String()

	_, err = s.db.Exec(
		`INSERT INTO hosts (id, name, ip, port, username, auth_type, encrypted_credential,
		 encrypted_passphrase, proxy_host_id, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, req.IP, req.Port, req.Username, string(req.AuthType),
		encCred, encPass, req.ProxyHostID, string(tagsJSON), now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, fmt.Errorf("主机名 %q 已存在", req.Name)
		}
		return nil, fmt.Errorf("插入主机失败: %w", err)
	}

	return s.GetByID(id)
}

// GetByID 按 ID 查询主机（含加密凭据）。
func (s *HostStore) GetByID(id string) (*models.Host, error) {
	row := s.db.QueryRow(
		`SELECT id, name, ip, port, username, auth_type, encrypted_credential,
		 encrypted_passphrase, proxy_host_id, tags, created_at, updated_at
		 FROM hosts WHERE id = ?`, id,
	)
	return scanHost(row)
}

// GetByName 按名称查询主机。
func (s *HostStore) GetByName(name string) (*models.Host, error) {
	row := s.db.QueryRow(
		`SELECT id, name, ip, port, username, auth_type, encrypted_credential,
		 encrypted_passphrase, proxy_host_id, tags, created_at, updated_at
		 FROM hosts WHERE name = ?`, name,
	)
	return scanHost(row)
}

// GetByIDOrName 按 ID 或名称查询主机。
func (s *HostStore) GetByIDOrName(idOrName string) (*models.Host, error) {
	h, err := s.GetByID(idOrName)
	if err == nil {
		return h, nil
	}
	return s.GetByName(idOrName)
}

// List 列出所有主机，可按 tag 过滤。
func (s *HostStore) List(tag string) ([]*models.Host, error) {
	var rows *sql.Rows
	var err error
	if tag == "" {
		rows, err = s.db.Query(
			`SELECT id, name, ip, port, username, auth_type, encrypted_credential,
			 encrypted_passphrase, proxy_host_id, tags, created_at, updated_at
			 FROM hosts ORDER BY name`,
		)
	} else {
		// 使用 SQLite json_each 虚拟表过滤 tag
		rows, err = s.db.Query(
			`SELECT h.id, h.name, h.ip, h.port, h.username, h.auth_type,
			 h.encrypted_credential, h.encrypted_passphrase, h.proxy_host_id,
			 h.tags, h.created_at, h.updated_at
			 FROM hosts h, json_each(h.tags)
			 WHERE json_each.value = ?
			 ORDER BY h.name`, tag,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("查询主机列表失败: %w", err)
	}
	defer rows.Close()

	var hosts []*models.Host
	for rows.Next() {
		h, err := scanHostRows(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

// Update 更新主机信息。
func (s *HostStore) Update(id string, req *models.UpdateHostRequest) (*models.Host, error) {
	h, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		h.Name = *req.Name
	}
	if req.IP != nil {
		h.IP = *req.IP
	}
	if req.Port != nil {
		h.Port = *req.Port
	}
	if req.Username != nil {
		h.Username = *req.Username
	}
	if req.AuthType != nil {
		h.AuthType = *req.AuthType
	}
	if req.Credential != nil {
		h.EncryptedCredential, err = s.crypto.Encrypt(*req.Credential)
		if err != nil {
			return nil, fmt.Errorf("加密凭据失败: %w", err)
		}
	}
	if req.Passphrase != nil {
		h.EncryptedPassphrase, err = s.crypto.Encrypt(*req.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("加密 passphrase 失败: %w", err)
		}
	}
	if req.ProxyHostID != nil {
		h.ProxyHostID = *req.ProxyHostID
	}
	if req.Tags != nil {
		h.Tags = req.Tags
	}

	tagsJSON, _ := json.Marshal(h.Tags)
	h.UpdatedAt = time.Now().UTC()

	_, err = s.db.Exec(
		`UPDATE hosts SET name=?, ip=?, port=?, username=?, auth_type=?,
		 encrypted_credential=?, encrypted_passphrase=?, proxy_host_id=?,
		 tags=?, updated_at=? WHERE id=?`,
		h.Name, h.IP, h.Port, h.Username, string(h.AuthType),
		h.EncryptedCredential, h.EncryptedPassphrase, h.ProxyHostID,
		string(tagsJSON), h.UpdatedAt, id,
	)
	if err != nil {
		return nil, fmt.Errorf("更新主机失败: %w", err)
	}
	return h, nil
}

// Delete 删除主机。
func (s *HostStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除主机失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("主机不存在: %s", id)
	}
	return nil
}

// DecryptCredential 解密主机凭据。
func (s *HostStore) DecryptCredential(h *models.Host) (credential, passphrase string, err error) {
	credential, err = s.crypto.Decrypt(h.EncryptedCredential)
	if err != nil {
		return "", "", fmt.Errorf("解密凭据失败: %w", err)
	}
	passphrase, err = s.crypto.Decrypt(h.EncryptedPassphrase)
	if err != nil {
		return "", "", fmt.Errorf("解密 passphrase 失败: %w", err)
	}
	return credential, passphrase, nil
}

// scanHost 从 *sql.Row 扫描一个 Host。
func scanHost(row *sql.Row) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	var authType string
	err := row.Scan(
		&h.ID, &h.Name, &h.IP, &h.Port, &h.Username, &authType,
		&h.EncryptedCredential, &h.EncryptedPassphrase, &h.ProxyHostID,
		&tagsJSON, &h.CreatedAt, &h.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("主机不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("扫描主机数据失败: %w", err)
	}
	h.AuthType = models.AuthType(authType)
	_ = json.Unmarshal([]byte(tagsJSON), &h.Tags)
	if h.Tags == nil {
		h.Tags = []string{}
	}
	return &h, nil
}

// scanHostRows 从 *sql.Rows 扫描一个 Host。
func scanHostRows(rows *sql.Rows) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	var authType string
	err := rows.Scan(
		&h.ID, &h.Name, &h.IP, &h.Port, &h.Username, &authType,
		&h.EncryptedCredential, &h.EncryptedPassphrase, &h.ProxyHostID,
		&tagsJSON, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("扫描主机数据失败: %w", err)
	}
	h.AuthType = models.AuthType(authType)
	_ = json.Unmarshal([]byte(tagsJSON), &h.Tags)
	if h.Tags == nil {
		h.Tags = []string{}
	}
	return &h, nil
}
