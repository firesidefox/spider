package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

var ErrNoPrometheusBinding = fmt.Errorf("该主机未配置 Prometheus 数据源")

// --- PrometheusSourceStore ---

type PrometheusSourceStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

func NewPrometheusSourceStore(db *sql.DB, cm *crypto.Manager) *PrometheusSourceStore {
	return &PrometheusSourceStore{db: db, crypto: cm}
}

const promSourceCols = `id,name,base_url,timeout_seconds,auth_type,username,` +
	`encrypted_password,encrypted_token,skip_tls_verify,created_at,updated_at`

func (s *PrometheusSourceStore) Add(req *models.AddPrometheusSourceRequest) (*models.PrometheusSource, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	encPwd, err := s.crypto.Encrypt(req.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}
	encTok, err := s.crypto.Encrypt(req.Token)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	timeout := req.TimeoutSeconds
	if timeout == 0 {
		timeout = 30
	}
	skipTLS := 0
	if req.SkipTLSVerify {
		skipTLS = 1
	}
	_, err = s.db.Exec(`INSERT INTO prometheus_sources
		(id,name,base_url,timeout_seconds,auth_type,username,
		 encrypted_password,encrypted_token,skip_tls_verify,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, req.Name, req.BaseURL, timeout, string(req.AuthType), req.Username,
		encPwd, encTok, skipTLS, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *PrometheusSourceStore) List() ([]*models.PrometheusSource, error) {
	rows, err := s.db.Query(`SELECT ` + promSourceCols + ` FROM prometheus_sources ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.PrometheusSource
	for rows.Next() {
		src, err := scanPrometheusSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *PrometheusSourceStore) GetByID(id string) (*models.PrometheusSource, error) {
	row := s.db.QueryRow(`SELECT `+promSourceCols+` FROM prometheus_sources WHERE id=?`, id)
	return scanPrometheusSource(row)
}

func (s *PrometheusSourceStore) Update(id string, req *models.UpdatePrometheusSourceRequest) (*models.PrometheusSource, error) {
	now := time.Now().UTC()
	cur, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		cur.Name = *req.Name
	}
	if req.BaseURL != nil {
		cur.BaseURL = *req.BaseURL
	}
	if req.TimeoutSeconds != nil {
		cur.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.AuthType != nil {
		cur.AuthType = *req.AuthType
	}
	if req.Username != nil {
		cur.Username = *req.Username
	}
	if req.SkipTLSVerify != nil {
		cur.SkipTLSVerify = *req.SkipTLSVerify
	}
	encPwd := cur.EncryptedPassword
	encTok := cur.EncryptedToken
	if req.Password != nil {
		encPwd, err = s.crypto.Encrypt(*req.Password)
		if err != nil {
			return nil, err
		}
	}
	if req.Token != nil {
		encTok, err = s.crypto.Encrypt(*req.Token)
		if err != nil {
			return nil, err
		}
	}
	skipTLS := 0
	if cur.SkipTLSVerify {
		skipTLS = 1
	}
	_, err = s.db.Exec(`UPDATE prometheus_sources SET
		name=?,base_url=?,timeout_seconds=?,auth_type=?,username=?,
		encrypted_password=?,encrypted_token=?,skip_tls_verify=?,updated_at=?
		WHERE id=?`,
		cur.Name, cur.BaseURL, cur.TimeoutSeconds, string(cur.AuthType), cur.Username,
		encPwd, encTok, skipTLS, now, id)
	if err != nil {
		return nil, err
	}
	cur.EncryptedPassword = encPwd
	cur.EncryptedToken = encTok
	cur.UpdatedAt = now
	return cur, nil
}

func (s *PrometheusSourceStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM prometheus_sources WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PrometheusSourceStore) DecryptCredentials(src *models.PrometheusSource) (password, token string, err error) {
	password, err = s.crypto.Decrypt(src.EncryptedPassword)
	if err != nil {
		return
	}
	token, err = s.crypto.Decrypt(src.EncryptedToken)
	return
}

type promSourceScanner interface {
	Scan(dest ...any) error
}

func scanPrometheusSource(sc promSourceScanner) (*models.PrometheusSource, error) {
	var src models.PrometheusSource
	var skipTLS int
	err := sc.Scan(
		&src.ID, &src.Name, &src.BaseURL, &src.TimeoutSeconds,
		&src.AuthType, &src.Username,
		&src.EncryptedPassword, &src.EncryptedToken, &skipTLS,
		&src.CreatedAt, &src.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	src.SkipTLSVerify = skipTLS != 0
	return &src, nil
}

// --- PrometheusBindingStore ---

type PrometheusBindingStore struct {
	db *sql.DB
}

func NewPrometheusBindingStore(db *sql.DB) *PrometheusBindingStore {
	return &PrometheusBindingStore{db: db}
}

const promBindingCols = `id,source_id,scope_type,` +
	`COALESCE(topology_id,''),COALESCE(layer,''),COALESCE(host_id,''),created_at`

func (s *PrometheusBindingStore) Add(req *models.AddPrometheusBindingRequest) (*models.PrometheusBinding, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	var topologyID, layer, hostID *string
	if req.TopologyID != "" {
		topologyID = &req.TopologyID
	}
	if req.Layer != "" {
		layer = &req.Layer
	}
	if req.HostID != "" {
		hostID = &req.HostID
	}
	_, err := s.db.Exec(`INSERT INTO prometheus_bindings
		(id,source_id,scope_type,topology_id,layer,host_id,created_at)
		VALUES (?,?,?,?,?,?,?)`,
		id, req.SourceID, string(req.ScopeType), topologyID, layer, hostID, now)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *PrometheusBindingStore) ListBySource(sourceID string) ([]*models.PrometheusBinding, error) {
	rows, err := s.db.Query(`SELECT `+promBindingCols+` FROM prometheus_bindings WHERE source_id=? ORDER BY created_at`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.PrometheusBinding
	for rows.Next() {
		b, err := scanPrometheusBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *PrometheusBindingStore) GetByID(id string) (*models.PrometheusBinding, error) {
	row := s.db.QueryRow(`SELECT `+promBindingCols+` FROM prometheus_bindings WHERE id=?`, id)
	return scanPrometheusBinding(row)
}

func (s *PrometheusBindingStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM prometheus_bindings WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// FindSourceIDForHost implements the lookup priority: host-level prometheus face first, then topology_layer binding.
func (s *PrometheusBindingStore) FindSourceIDForHost(hostID string) (string, error) {
	// 1. host-level prometheus access face
	var sourceID string
	err := s.db.QueryRow(`SELECT prometheus_source_id FROM access_faces
		WHERE host_id=? AND type='prometheus' AND prometheus_source_id!='' LIMIT 1`, hostID).Scan(&sourceID)
	if err == nil {
		return sourceID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	// 2. topology_layer binding — find the node's (topology_id, layer)
	err = s.db.QueryRow(`
		SELECT pb.source_id FROM prometheus_bindings pb
		JOIN topology_nodes tn ON tn.topology_id = pb.topology_id AND tn.layer = pb.layer
		WHERE pb.scope_type='topology_layer' AND tn.host_id=?
		LIMIT 1`, hostID).Scan(&sourceID)
	if err == nil {
		return sourceID, nil
	}
	if err == sql.ErrNoRows {
		return "", ErrNoPrometheusBinding
	}
	return "", err
}

type promBindingScanner interface {
	Scan(dest ...any) error
}

func scanPrometheusBinding(sc promBindingScanner) (*models.PrometheusBinding, error) {
	var b models.PrometheusBinding
	err := sc.Scan(&b.ID, &b.SourceID, &b.ScopeType, &b.TopologyID, &b.Layer, &b.HostID, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}
