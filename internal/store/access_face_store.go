package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

type AccessFaceStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

func NewAccessFaceStore(db *sql.DB, cm *crypto.Manager) *AccessFaceStore {
	return &AccessFaceStore{db: db, crypto: cm}
}

func (s *AccessFaceStore) Add(hostID string, req *models.AddAccessFaceRequest) (*models.AccessFace, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	var encCred, encPass string
	var err error
	if req.Type != models.FacePrometheus {
		encCred, err = s.crypto.Encrypt(req.Credential)
		if err != nil {
			return nil, fmt.Errorf("encrypt credential: %w", err)
		}
		encPass, err = s.crypto.Encrypt(req.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("encrypt passphrase: %w", err)
		}
	}
	mode := normalizeKBMode(req.KBMode)
	sources := normalizeKnowledgeSources(mode, req.KnowledgeSources)
	if err := validateAccessFaceKB(mode, sources); err != nil {
		return nil, err
	}
	ksJSON, _ := json.Marshal(sources)
	_, err = s.db.Exec(`INSERT INTO access_faces
		(id,host_id,type,ip,port,username,auth_type,
		 encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,
		 ssh_login_input,
		 base_url,rest_scheme,rest_auth_type,rest_username,header_name,hmac_algo,kb_mode,knowledge_sources,probe_port,probe_interval,prometheus_source_id,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, hostID, req.Type, req.IP, req.Port, req.Username, req.SSHAuthType,
		encCred, encPass, req.SSHKeyID, req.SSHLegacy,
		req.SSHLoginInput,
		req.BaseURL, req.RESTScheme, req.RESTAuthType, req.RESTUsername, req.HeaderName, req.HMACAlgo, mode, string(ksJSON), req.ProbePort, 0, req.PrometheusSourceID, now, now)
	if err != nil {
		return nil, err
	}
	f, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *AccessFaceStore) GetByID(id string) (*models.AccessFace, error) {
	row := s.db.QueryRow(`SELECT `+accessFaceCols+` FROM access_faces WHERE id=?`, id)
	return scanAccessFace(row)
}

func (s *AccessFaceStore) ListByHost(hostID string) ([]*models.AccessFace, error) {
	rows, err := s.db.Query(`SELECT `+accessFaceCols+` FROM access_faces WHERE host_id=? ORDER BY created_at`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.AccessFace
	for rows.Next() {
		f, err := scanAccessFace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AccessFaceStore) FaceTypesByHostIDs(hostIDs []string) (map[string][]models.AccessFaceType, error) {
	result := make(map[string][]models.AccessFaceType, len(hostIDs))
	if len(hostIDs) == 0 {
		return result, nil
	}
	placeholders := sqlPlaceholders(len(hostIDs))
	args := make([]any, len(hostIDs))
	for i, id := range hostIDs {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT host_id, type FROM access_faces WHERE host_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hostID string
		var faceType models.AccessFaceType
		if err := rows.Scan(&hostID, &faceType); err != nil {
			return nil, err
		}
		result[hostID] = append(result[hostID], faceType)
	}
	return result, rows.Err()
}

func (s *AccessFaceStore) GetSSHFaceForHost(hostID string) (*models.AccessFace, error) {
	row := s.db.QueryRow(`SELECT `+accessFaceCols+` FROM access_faces WHERE host_id=? AND type=? ORDER BY created_at LIMIT 1`, hostID, models.FaceSSH)
	return scanAccessFace(row)
}

func (s *AccessFaceStore) Update(id string, req *models.UpdateAccessFaceRequest) (*models.AccessFace, error) {
	now := time.Now().UTC()
	cur, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if req.IP != nil {
		cur.IP = *req.IP
	}
	if req.Port != nil {
		cur.Port = *req.Port
	}
	if req.Username != nil {
		cur.Username = *req.Username
	}
	if req.SSHAuthType != nil {
		cur.SSHAuthType = *req.SSHAuthType
	}
	if req.SSHKeyID != nil {
		cur.SSHKeyID = *req.SSHKeyID
	}
	if req.SSHLegacy != nil {
		cur.SSHLegacy = *req.SSHLegacy
	}
	if req.SSHLoginInput != nil {
		cur.SSHLoginInput = *req.SSHLoginInput
	}
	if req.BaseURL != nil {
		cur.BaseURL = *req.BaseURL
	}
	if req.RESTScheme != nil {
		cur.RESTScheme = *req.RESTScheme
	}
	if req.RESTAuthType != nil {
		cur.RESTAuthType = *req.RESTAuthType
		switch cur.RESTAuthType {
		case "bearer", "none":
			cur.RESTUsername = ""
			cur.HeaderName = ""
		case "basic":
			cur.HeaderName = ""
		case "apikey":
			cur.RESTUsername = ""
		}
	}
	if req.RESTUsername != nil {
		cur.RESTUsername = *req.RESTUsername
	}
	if req.HeaderName != nil {
		cur.HeaderName = *req.HeaderName
	}
	if req.HMACAlgo != nil {
		cur.HMACAlgo = *req.HMACAlgo
	}
	if req.KBMode != nil {
		cur.KBMode = normalizeKBMode(*req.KBMode)
	}
	if req.KnowledgeSources != nil {
		cur.KnowledgeSources = req.KnowledgeSources
	}
	cur.KnowledgeSources = normalizeKnowledgeSources(cur.KBMode, cur.KnowledgeSources)
	if err := validateAccessFaceKB(cur.KBMode, cur.KnowledgeSources); err != nil {
		return nil, err
	}
	if req.ProbePort != nil {
		cur.ProbePort = *req.ProbePort
	}
	if req.PrometheusSourceID != nil {
		cur.PrometheusSourceID = *req.PrometheusSourceID
	}
	encCred := cur.EncryptedCred
	encPass := cur.EncryptedPass
	if cur.Type != models.FacePrometheus {
		if req.Credential != nil {
			encCred, err = s.crypto.Encrypt(*req.Credential)
			if err != nil {
				return nil, err
			}
		}
		if req.Passphrase != nil {
			encPass, err = s.crypto.Encrypt(*req.Passphrase)
			if err != nil {
				return nil, err
			}
		}
	}
	ksJSON, _ := json.Marshal(cur.KnowledgeSources)
	_, err = s.db.Exec(`UPDATE access_faces SET
		ip=?,port=?,username=?,auth_type=?,
		encrypted_credential=?,encrypted_passphrase=?,
		ssh_key_id=?,ssh_legacy=?,ssh_login_input=?,
		base_url=?,rest_scheme=?,rest_auth_type=?,rest_username=?,
		header_name=?,hmac_algo=?,kb_mode=?,knowledge_sources=?,probe_port=?,probe_interval=?,prometheus_source_id=?,updated_at=?
		WHERE id=?`,
		cur.IP, cur.Port, cur.Username, cur.SSHAuthType,
		encCred, encPass, cur.SSHKeyID, cur.SSHLegacy, cur.SSHLoginInput,
		cur.BaseURL, cur.RESTScheme, cur.RESTAuthType, cur.RESTUsername, cur.HeaderName, cur.HMACAlgo, cur.KBMode, string(ksJSON), cur.ProbePort, 0, cur.PrometheusSourceID, now, id)
	if err != nil {
		return nil, err
	}
	cur.UpdatedAt = now
	cur.EncryptedCred = encCred
	cur.EncryptedPass = encPass
	return cur, nil
}

func (s *AccessFaceStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM access_faces WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *AccessFaceStore) DecryptCredential(f *models.AccessFace) (cred, pass string, err error) {
	cred, err = s.crypto.Decrypt(f.EncryptedCred)
	if err != nil {
		return
	}
	pass, err = s.crypto.Decrypt(f.EncryptedPass)
	return
}

const accessFaceCols = `id,host_id,type,ip,port,username,auth_type,` +
	`encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,` +
	`ssh_login_input,` +
	`base_url,rest_scheme,rest_auth_type,rest_username,header_name,hmac_algo,kb_mode,knowledge_sources,probe_port,probe_interval,` +
	`prometheus_source_id,created_at,updated_at`

type accessFaceScanner interface {
	Scan(dest ...any) error
}

func scanAccessFace(s accessFaceScanner) (*models.AccessFace, error) {
	var f models.AccessFace
	var ksJSON string
	var sshLegacy int
	var probeInterval int // kept in DB for backward compat, not used
	err := s.Scan(
		&f.ID, &f.HostID, &f.Type, &f.IP, &f.Port,
		&f.Username, &f.SSHAuthType,
		&f.EncryptedCred, &f.EncryptedPass,
		&f.SSHKeyID, &sshLegacy,
		&f.SSHLoginInput,
		&f.BaseURL, &f.RESTScheme, &f.RESTAuthType, &f.RESTUsername, &f.HeaderName, &f.HMACAlgo,
		&f.KBMode, &ksJSON, &f.ProbePort, &probeInterval, &f.PrometheusSourceID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.SSHLegacy = sshLegacy != 0
	if err := json.Unmarshal([]byte(ksJSON), &f.KnowledgeSources); err != nil {
		return nil, fmt.Errorf("解析知识源失败: %w", err)
	}
	if f.KnowledgeSources == nil {
		f.KnowledgeSources = []models.KnowledgeSourceRef{}
	}
	f.KBMode = normalizeKBMode(f.KBMode)
	f.KnowledgeSources = normalizeKnowledgeSources(f.KBMode, f.KnowledgeSources)
	return &f, nil
}

func normalizeKBMode(mode string) string {
	if mode == "" {
		return "none"
	}
	return mode
}

func normalizeKnowledgeSources(mode string, sources []models.KnowledgeSourceRef) []models.KnowledgeSourceRef {
	if mode == "none" {
		return []models.KnowledgeSourceRef{}
	}
	out := make([]models.KnowledgeSourceRef, 0, len(sources))
	for _, src := range sources {
		if src.Type == "none" && src.ID == 0 {
			continue
		}
		out = append(out, src)
	}
	if out == nil {
		return []models.KnowledgeSourceRef{}
	}
	return out
}

func validateAccessFaceKB(mode string, sources []models.KnowledgeSourceRef) error {
	switch mode {
	case "none":
		return nil
	case "specific":
		if len(sources) == 0 {
			return fmt.Errorf("kb_mode=specific requires at least one knowledge_source")
		}
		if len(sources) > 10 {
			return fmt.Errorf("knowledge_sources exceeds limit of 10")
		}
		for _, src := range sources {
			if src.Type != "group" && src.Type != "doc" {
				return fmt.Errorf("invalid knowledge_source type")
			}
			if src.ID <= 0 {
				return fmt.Errorf("invalid knowledge_source id")
			}
		}
		return nil
	default:
		return fmt.Errorf("invalid kb_mode")
	}
}
