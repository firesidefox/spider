package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/logger"
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
	encCred, err := s.crypto.Encrypt(req.Credential)
	if err != nil {
		return nil, fmt.Errorf("encrypt credential: %w", err)
	}
	encPass, err := s.crypto.Encrypt(req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("encrypt passphrase: %w", err)
	}
	ksJSON, _ := json.Marshal(req.KnowledgeSources)
	_, err = s.db.Exec(`INSERT INTO access_faces
		(id,host_id,type,ip,port,username,auth_type,
		 encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,
		 base_url,rest_auth_type,rest_username,header_name,knowledge_sources,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, hostID, req.Type, req.IP, req.Port, req.Username, req.SSHAuthType,
		encCred, encPass, req.SSHKeyID, req.SSHLegacy,
		req.BaseURL, req.RESTAuthType, req.RESTUsername, req.HeaderName, string(ksJSON), now, now)
	if err != nil {
		return nil, err
	}
	f, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	logger.Global().Debug().Str("table", "access_faces").Str("op", "insert").Str("host_id", hostID).Str("face_id", f.ID).Msg("store")
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
	logger.Global().Debug().Str("table", "access_faces").Str("op", "select").Str("host_id", hostID).Int("count", len(out)).Msg("store")
	return out, nil
}

func (s *AccessFaceStore) GetSSHFaceForHost(hostID string) (*models.AccessFace, error) {
	row := s.db.QueryRow(`SELECT `+accessFaceCols+` FROM access_faces WHERE host_id=? AND type='ssh' ORDER BY created_at LIMIT 1`, hostID)
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
	if req.BaseURL != nil {
		cur.BaseURL = *req.BaseURL
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
	if req.KnowledgeSources != nil {
		cur.KnowledgeSources = req.KnowledgeSources
	}
	encCred := cur.EncryptedCred
	encPass := cur.EncryptedPass
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
	ksJSON, _ := json.Marshal(cur.KnowledgeSources)
	_, err = s.db.Exec(`UPDATE access_faces SET
		ip=?,port=?,username=?,auth_type=?,
		encrypted_credential=?,encrypted_passphrase=?,
		ssh_key_id=?,ssh_legacy=?,base_url=?,rest_auth_type=?,rest_username=?,
		header_name=?,knowledge_sources=?,updated_at=?
		WHERE id=?`,
		cur.IP, cur.Port, cur.Username, cur.SSHAuthType,
		encCred, encPass, cur.SSHKeyID, cur.SSHLegacy,
		cur.BaseURL, cur.RESTAuthType, cur.RESTUsername, cur.HeaderName, string(ksJSON), now, id)
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
	logger.Global().Debug().Str("table", "access_faces").Str("op", "delete").Str("face_id", id).Msg("store")
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
	`base_url,rest_auth_type,rest_username,header_name,knowledge_sources,created_at,updated_at`

type accessFaceScanner interface {
	Scan(dest ...any) error
}

func scanAccessFace(s accessFaceScanner) (*models.AccessFace, error) {
	var f models.AccessFace
	var ksJSON string
	var sshLegacy int
	err := s.Scan(
		&f.ID, &f.HostID, &f.Type, &f.IP, &f.Port,
		&f.Username, &f.SSHAuthType,
		&f.EncryptedCred, &f.EncryptedPass,
		&f.SSHKeyID, &sshLegacy,
		&f.BaseURL, &f.RESTAuthType, &f.RESTUsername, &f.HeaderName,
		&ksJSON, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.SSHLegacy = sshLegacy != 0
	_ = json.Unmarshal([]byte(ksJSON), &f.KnowledgeSources)
	if f.KnowledgeSources == nil {
		f.KnowledgeSources = []models.KnowledgeSourceRef{}
	}
	return &f, nil
}
