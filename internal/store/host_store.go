package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// HostStore 提供主机的 CRUD 操作。
type HostStore struct {
	db *sql.DB
}

// NewHostStore 创建一个新的 HostStore。
func NewHostStore(db *sql.DB) *HostStore {
	return &HostStore{db: db}
}

// Add 添加新主机。
func (s *HostStore) Add(req *models.AddHostRequest) (*models.Host, error) {
	if req.Name == "" || req.IP == "" {
		return nil, fmt.Errorf("name、ip 不能为空")
	}
	tagsJSON, _ := json.Marshal(req.Tags)
	now := time.Now().UTC()
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO hosts (id, name, ip, notes, vendor, product_name, product_version, tags, username, auth_type, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
		id, req.Name, req.IP, req.Notes, req.Vendor, req.ProductName, req.ProductVersion,
		string(tagsJSON), now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, fmt.Errorf("主机名 %q 已存在", req.Name)
		}
		return nil, fmt.Errorf("插入主机失败: %w", err)
	}
	h, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// GetByID 按 ID 查询主机。
func (s *HostStore) GetByID(id string) (*models.Host, error) {
	row := s.db.QueryRow(
		`SELECT id, name, ip, notes, vendor, product_name, product_version, tags, created_at, updated_at
		 FROM hosts WHERE id = ?`, id,
	)
	h, err := scanHost(row)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// GetByName 按名称查询主机。
func (s *HostStore) GetByName(name string) (*models.Host, error) {
	row := s.db.QueryRow(
		`SELECT id, name, ip, notes, vendor, product_name, product_version, tags, created_at, updated_at
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
	const q = `SELECT id, name, ip, notes, vendor, product_name, product_version, tags, created_at, updated_at FROM hosts`
	var (
		rows *sql.Rows
		err  error
	)
	if tag == "" {
		rows, err = s.db.Query(q + ` ORDER BY name`)
	} else {
		rows, err = s.db.Query(
			`SELECT h.id, h.name, h.ip, h.notes, h.vendor, h.product_name, h.product_version,
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
	if req.Notes != nil {
		h.Notes = *req.Notes
	}
	if req.Tags != nil {
		h.Tags = req.Tags
	}
	if req.Vendor != nil {
		h.Vendor = *req.Vendor
	}
	if req.ProductName != nil {
		h.ProductName = *req.ProductName
	}
	if req.ProductVersion != nil {
		h.ProductVersion = *req.ProductVersion
	}
	tagsJSON, _ := json.Marshal(h.Tags)
	h.UpdatedAt = time.Now().UTC()
	_, err = s.db.Exec(
		`UPDATE hosts SET name=?, ip=?, notes=?, vendor=?, product_name=?, product_version=?,
		 tags=?, updated_at=? WHERE id=?`,
		h.Name, h.IP, h.Notes, h.Vendor, h.ProductName, h.ProductVersion,
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
		return ErrNotFound
	}
	return nil
}

func scanHost(row *sql.Row) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	err := row.Scan(
		&h.ID, &h.Name, &h.IP, &h.Notes,
		&h.Vendor, &h.ProductName, &h.ProductVersion,
		&tagsJSON, &h.CreatedAt, &h.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("扫描主机数据失败: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &h.Tags); err != nil {
		return nil, fmt.Errorf("解析主机标签失败: %w", err)
	}
	if h.Tags == nil {
		h.Tags = []string{}
	}
	return &h, nil
}

func scanHostRows(rows *sql.Rows) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	err := rows.Scan(
		&h.ID, &h.Name, &h.IP, &h.Notes,
		&h.Vendor, &h.ProductName, &h.ProductVersion,
		&tagsJSON, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("扫描主机数据失败: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &h.Tags); err != nil {
		return nil, fmt.Errorf("解析主机标签失败: %w", err)
	}
	if h.Tags == nil {
		h.Tags = []string{}
	}
	return &h, nil
}

// ResolveNames resolves host IDs from a tool input map to host names.
// Reads host_ids ([]string or []any) and host_id (string) from input.
// Falls back to the raw ID if a host is not found.
func (s *HostStore) ResolveNames(input map[string]any) []string {
	if input == nil {
		return nil
	}
	var ids []string
	switch v := input["host_ids"].(type) {
	case []any:
		for _, x := range v {
			if id, ok := x.(string); ok {
				ids = append(ids, id)
			}
		}
	case []string:
		ids = v
	}
	if id, ok := input["host_id"].(string); ok && id != "" {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if h, err := s.GetByID(id); err == nil {
			names = append(names, h.Name)
		} else {
			names = append(names, id)
		}
	}
	return names
}
