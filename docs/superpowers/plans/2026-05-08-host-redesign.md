# Host Model Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the Host model from a flat SSH-only struct into four concepts: 基本信息, 操作面 (AccessFace), 指纹 (Fingerprint), 记忆 (Memory).

**Architecture:** New tables `access_faces`, `host_fingerprints`, `host_memories`, `host_knowledge_sources` are added via incremental migration. Existing SSH data is migrated to `access_faces`. The SSH client and agent tools are updated to read from `AccessFace` instead of `Host`.

**Tech Stack:** Go 1.21, SQLite (database/sql), Vue 3 + TypeScript

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/models/host.go` | Rewrite | New types: AccessFace, Fingerprint, Memory, KnowledgeSourceRef |
| `internal/db/schema.go` | Modify | Add 4 new tables + ALTER hosts for new columns |
| `internal/store/host_store.go` | Modify | Remove SSH fields, add KnowledgeSources loading |
| `internal/store/access_face_store.go` | Create | CRUD for AccessFace |
| `internal/store/fingerprint_store.go` | Create | Upsert/Get for Fingerprint |
| `internal/store/memory_store.go` | Create | CRUD for Memory |
| `internal/api/hosts.go` | Modify | New endpoints for access faces, fingerprint, memory |
| `internal/ssh/client.go` | Modify | Accept AccessFace instead of Host |
| `internal/agent/tools_cli.go` | Modify | Resolve SSH AccessFace from host_id |
| `internal/agent/tools_api.go` | Modify | Resolve REST AccessFace from host_id |
| `web/src/api/hosts.ts` | Rewrite | New TypeScript types + API calls |
| `web/src/views/HostsView.vue` | Modify | Updated form + detail view |

---

## Task 1: Go Models

**Files:**
- Rewrite: `internal/models/host.go`

- [ ] **Step 1: Rewrite models/host.go**

```go
package models

import "time"

type AccessFaceType string

const (
	FaceSSH     AccessFaceType = "ssh"
	FaceRESTAPI AccessFaceType = "restapi"
)

type SSHAuthType string

const (
	SSHAuthPassword    SSHAuthType = "password"
	SSHAuthKey         SSHAuthType = "key"
	SSHAuthKeyPassword SSHAuthType = "key_password"
)

type RESTAuthType string

const (
	RESTAuthBearer RESTAuthType = "bearer"
	RESTAuthBasic  RESTAuthType = "basic"
	RESTAuthAPIKey RESTAuthType = "apikey"
	RESTAuthNone   RESTAuthType = "none"
)

type KnowledgeSourceRef struct {
	Type  string `json:"type"` // "group" | "doc"
	ID    int    `json:"id"`
}

type AccessFace struct {
	ID               string               `json:"id"`
	HostID           string               `json:"host_id"`
	Type             AccessFaceType       `json:"type"`
	IP               string               `json:"ip"`
	Port             int                  `json:"port"`
	Username         string               `json:"username,omitempty"`
	SSHAuthType      SSHAuthType          `json:"ssh_auth_type,omitempty"`
	EncryptedCred    string               `json:"-"`
	EncryptedPass    string               `json:"-"`
	SSHKeyID         string               `json:"ssh_key_id,omitempty"`
	SSHLegacy        bool                 `json:"ssh_legacy,omitempty"`
	BaseURL          string               `json:"base_url,omitempty"`
	RESTAuthType     RESTAuthType         `json:"rest_auth_type,omitempty"`
	RESTUsername     string               `json:"rest_username,omitempty"`
	HeaderName       string               `json:"header_name,omitempty"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type FingerprintStatus string

const (
	FingerprintOK         FingerprintStatus = "ok"
	FingerprintChanged    FingerprintStatus = "changed"
	FingerprintUnverified FingerprintStatus = "unverified"
)

type Fingerprint struct {
	HostID        string            `json:"host_id"`
	SSHHostKey    string            `json:"ssh_host_key,omitempty"`
	SystemVersion string            `json:"system_version,omitempty"`
	HardwareID    string            `json:"hardware_id,omitempty"`
	APISignature  string            `json:"api_signature,omitempty"`
	Status        FingerprintStatus `json:"status"`
	SnapshotAt    *time.Time        `json:"snapshot_at,omitempty"`
}

type Memory struct {
	ID        int       `json:"id"`
	HostID    string    `json:"host_id"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"` // "user" | "agent"
	CreatedAt time.Time `json:"created_at"`
}

type Host struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	IP             string     `json:"ip"`
	Notes          string     `json:"notes,omitempty"`
	Tags           []string   `json:"tags"`
	Vendor         string     `json:"vendor,omitempty"`
	ProductName    string     `json:"product_name,omitempty"`
	ProductVersion string     `json:"product_version,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources,omitempty"`
	AccessFaces    []AccessFace  `json:"access_faces,omitempty"`
	Fingerprint    *Fingerprint  `json:"fingerprint,omitempty"`
	Memories       []Memory      `json:"memories,omitempty"`
}

type AddHostRequest struct {
	Name           string   `json:"name"`
	IP             string   `json:"ip"`
	Notes          string   `json:"notes"`
	Tags           []string `json:"tags"`
	Vendor         string   `json:"vendor"`
	ProductName    string   `json:"product_name"`
	ProductVersion string   `json:"product_version"`
}

type UpdateHostRequest struct {
	Name           *string  `json:"name"`
	IP             *string  `json:"ip"`
	Notes          *string  `json:"notes"`
	Tags           []string `json:"tags"`
	Vendor         *string  `json:"vendor"`
	ProductName    *string  `json:"product_name"`
	ProductVersion *string  `json:"product_version"`
}

type AddAccessFaceRequest struct {
	Type         AccessFaceType `json:"type"`
	IP           string         `json:"ip"`
	Port         int            `json:"port"`
	Username     string         `json:"username"`
	SSHAuthType  SSHAuthType    `json:"ssh_auth_type"`
	Credential   string         `json:"credential"`
	Passphrase   string         `json:"passphrase"`
	SSHKeyID     string         `json:"ssh_key_id"`
	SSHLegacy    bool           `json:"ssh_legacy"`
	BaseURL      string         `json:"base_url"`
	RESTAuthType RESTAuthType   `json:"rest_auth_type"`
	RESTUsername string         `json:"rest_username"`
	HeaderName   string         `json:"header_name"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"`
}

type UpdateAccessFaceRequest struct {
	IP           *string      `json:"ip"`
	Port         *int         `json:"port"`
	Username     *string      `json:"username"`
	SSHAuthType  *SSHAuthType `json:"ssh_auth_type"`
	Credential   *string      `json:"credential"`
	Passphrase   *string      `json:"passphrase"`
	SSHKeyID     *string      `json:"ssh_key_id"`
	SSHLegacy    *bool        `json:"ssh_legacy"`
	BaseURL      *string      `json:"base_url"`
	RESTAuthType *RESTAuthType `json:"rest_auth_type"`
	RESTUsername *string      `json:"rest_username"`
	HeaderName   *string      `json:"header_name"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"`
}

type AddMemoryRequest struct {
	Content   string `json:"content"`
	CreatedBy string `json:"created_by"`
}

type UpdateFingerprintRequest struct {
	SSHHostKey    string `json:"ssh_host_key"`
	SystemVersion string `json:"system_version"`
	HardwareID    string `json:"hardware_id"`
	APISignature  string `json:"api_signature"`
}
```

- [ ] **Step 2: Commit**
  ```
  git add internal/models/host.go
  git commit -m "feat(models): new Host/AccessFace/Fingerprint/Memory types"
  ```

---

## Task 2: DB Migration

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add new tables and columns inside `migrate()` after line 223**

Append the following block after `db.Exec("ALTER TABLE conversations ADD COLUMN status TEXT NOT NULL DEFAULT 'idle'")`:

```go
	// Host redesign: new tables
	db.Exec(`CREATE TABLE IF NOT EXISTS access_faces (
		id TEXT PRIMARY KEY,
		host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		type TEXT NOT NULL CHECK(type IN ('ssh','restapi')),
		ip TEXT NOT NULL,
		port INTEGER NOT NULL,
		username TEXT NOT NULL DEFAULT '',
		auth_type TEXT NOT NULL DEFAULT '',
		encrypted_credential TEXT NOT NULL DEFAULT '',
		encrypted_passphrase TEXT NOT NULL DEFAULT '',
		ssh_key_id TEXT NOT NULL DEFAULT '',
		ssh_legacy INTEGER NOT NULL DEFAULT 0,
		base_url TEXT NOT NULL DEFAULT '',
		rest_username TEXT NOT NULL DEFAULT '',
		header_name TEXT NOT NULL DEFAULT '',
		knowledge_sources TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS host_fingerprints (
		host_id TEXT PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
		ssh_host_key TEXT NOT NULL DEFAULT '',
		system_version TEXT NOT NULL DEFAULT '',
		hardware_id TEXT NOT NULL DEFAULT '',
		api_signature TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'unverified',
		snapshot_at DATETIME
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS host_memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		created_by TEXT NOT NULL CHECK(created_by IN ('user','agent')),
		created_at DATETIME NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS host_knowledge_sources (
		host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
		type TEXT NOT NULL CHECK(type IN ('group','doc')),
		ref_id INTEGER NOT NULL,
		PRIMARY KEY (host_id, type, ref_id)
	)`)
	// New hosts columns
	db.Exec("ALTER TABLE hosts ADD COLUMN notes TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE hosts ADD COLUMN product_name TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE hosts ADD COLUMN product_version TEXT NOT NULL DEFAULT ''")
```

- [ ] **Step 2: Add data migration — seed access_faces from existing hosts**

Append after the ALTER TABLE hosts lines above:

```go
	// Data migration: create one SSH access_face per existing host (idempotent via INSERT OR IGNORE)
	db.Exec(`INSERT OR IGNORE INTO access_faces
		(id, host_id, type, ip, port, username, auth_type,
		 encrypted_credential, encrypted_passphrase, ssh_key_id, ssh_legacy,
		 base_url, rest_username, header_name, knowledge_sources,
		 created_at, updated_at)
		SELECT
			lower(hex(randomblob(16))),
			id, 'ssh', ip, port, username, auth_type,
			encrypted_credential, encrypted_passphrase,
			COALESCE(ssh_key_id,''), COALESCE(ssh_legacy,0),
			'', '', '[]',
			created_at, updated_at
		FROM hosts
		WHERE id NOT IN (SELECT host_id FROM access_faces WHERE type='ssh')`)
```

- [ ] **Step 3: Build**
  ```
  go build ./internal/db/...
  ```

- [ ] **Step 4: Commit**
  ```
  git add internal/db/schema.go
  git commit -m "feat(db): add access_faces, host_fingerprints, host_memories tables + migrate SSH data"
  ```

---

## Task 3: Store Layer

**Files:**
- Create: `internal/store/access_face_store.go`
- Create: `internal/store/fingerprint_store.go`
- Create: `internal/store/memory_store.go`
- Modify: `internal/store/host_store.go`

- [ ] **Step 1: Create `internal/store/access_face_store.go`**

```go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"spider.ai/internal/models"
)

type AccessFaceStore struct {
	db  *sql.DB
	enc Encryptor
}

func NewAccessFaceStore(db *sql.DB, enc Encryptor) *AccessFaceStore {
	return &AccessFaceStore{db: db, enc: enc}
}

func (s *AccessFaceStore) Add(hostID string, req *models.AddAccessFaceRequest) (*models.AccessFace, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	encCred, err := s.enc.Encrypt(req.Credential)
	if err != nil {
		return nil, fmt.Errorf("encrypt credential: %w", err)
	}
	encPass, err := s.enc.Encrypt(req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("encrypt passphrase: %w", err)
	}
	ksJSON, _ := json.Marshal(req.KnowledgeSources)
	_, err = s.db.Exec(`INSERT INTO access_faces
		(id,host_id,type,ip,port,username,auth_type,
		 encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,
		 base_url,rest_username,header_name,knowledge_sources,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, hostID, req.Type, req.IP, req.Port, req.Username, req.SSHAuthType,
		encCred, encPass, req.SSHKeyID, req.SSHLegacy,
		req.BaseURL, req.RESTUsername, req.HeaderName, string(ksJSON), now, now)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
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
	return out, rows.Err()
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
		encCred, err = s.enc.Encrypt(*req.Credential)
		if err != nil {
			return nil, err
		}
	}
	if req.Passphrase != nil {
		encPass, err = s.enc.Encrypt(*req.Passphrase)
		if err != nil {
			return nil, err
		}
	}
	ksJSON, _ := json.Marshal(cur.KnowledgeSources)
	_, err = s.db.Exec(`UPDATE access_faces SET
		ip=?,port=?,username=?,auth_type=?,
		encrypted_credential=?,encrypted_passphrase=?,
		ssh_key_id=?,ssh_legacy=?,base_url=?,rest_username=?,
		header_name=?,knowledge_sources=?,updated_at=?
		WHERE id=?`,
		cur.IP, cur.Port, cur.Username, cur.SSHAuthType,
		encCred, encPass, cur.SSHKeyID, cur.SSHLegacy,
		cur.BaseURL, cur.RESTUsername, cur.HeaderName, string(ksJSON), now, id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *AccessFaceStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM access_faces WHERE id=?`, id)
	return err
}

func (s *AccessFaceStore) DecryptCredential(f *models.AccessFace) (cred, pass string, err error) {
	cred, err = s.enc.Decrypt(f.EncryptedCred)
	if err != nil {
		return
	}
	pass, err = s.enc.Decrypt(f.EncryptedPass)
	return
}

const accessFaceCols = `id,host_id,type,ip,port,username,auth_type,
	encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,
	base_url,rest_username,header_name,knowledge_sources,created_at,updated_at`

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
```

- [ ] **Step 2: Create `internal/store/fingerprint_store.go`**

```go
package store

import (
	"database/sql"
	"time"

	"spider.ai/internal/models"
)

type FingerprintStore struct {
	db *sql.DB
}

func NewFingerprintStore(db *sql.DB) *FingerprintStore {
	return &FingerprintStore{db: db}
}

func (s *FingerprintStore) Upsert(fp *models.Fingerprint) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO host_fingerprints
		(host_id,ssh_host_key,system_version,hardware_id,api_signature,status,snapshot_at)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(host_id) DO UPDATE SET
			ssh_host_key=excluded.ssh_host_key,
			system_version=excluded.system_version,
			hardware_id=excluded.hardware_id,
			api_signature=excluded.api_signature,
			status=excluded.status,
			snapshot_at=excluded.snapshot_at`,
		fp.HostID, fp.SSHHostKey, fp.SystemVersion, fp.HardwareID,
		fp.APISignature, fp.Status, now)
	return err
}

func (s *FingerprintStore) Get(hostID string) (*models.Fingerprint, error) {
	var fp models.Fingerprint
	err := s.db.QueryRow(`SELECT host_id,ssh_host_key,system_version,hardware_id,api_signature,status,snapshot_at
		FROM host_fingerprints WHERE host_id=?`, hostID).
		Scan(&fp.HostID, &fp.SSHHostKey, &fp.SystemVersion, &fp.HardwareID,
			&fp.APISignature, &fp.Status, &fp.SnapshotAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &fp, err
}

func (s *FingerprintStore) MarkChanged(hostID string) error {
	_, err := s.db.Exec(`UPDATE host_fingerprints SET status='changed' WHERE host_id=?`, hostID)
	return err
}
```

- [ ] **Step 3: Create `internal/store/memory_store.go`**

```go
package store

import (
	"database/sql"
	"time"

	"spider.ai/internal/models"
)

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

func (s *MemoryStore) Add(req *models.AddMemoryRequest, hostID string) (*models.Memory, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(`INSERT INTO host_memories (host_id,content,created_by,created_at)
		VALUES (?,?,?,?)`, hostID, req.Content, req.CreatedBy, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &models.Memory{
		ID:        int(id),
		HostID:    hostID,
		Content:   req.Content,
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}, nil
}

func (s *MemoryStore) ListByHost(hostID string) ([]*models.Memory, error) {
	rows, err := s.db.Query(`SELECT id,host_id,content,created_by,created_at
		FROM host_memories WHERE host_id=? ORDER BY created_at`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Memory
	for rows.Next() {
		var m models.Memory
		if err := rows.Scan(&m.ID, &m.HostID, &m.Content, &m.CreatedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *MemoryStore) Delete(id int) error {
	_, err := s.db.Exec(`DELETE FROM host_memories WHERE id=?`, id)
	return err
}
```

- [ ] **Step 4: Update `internal/store/host_store.go` — scanHost**

Remove SSH-specific fields from the scan and SQL. The updated `scanHost` function:

```go
func scanHost(s hostScanner) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	err := s.Scan(
		&h.ID, &h.Name, &h.IP, &h.Notes,
		&h.Vendor, &h.ProductName, &h.ProductVersion,
		&tagsJSON, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &h.Tags)
	if h.Tags == nil {
		h.Tags = []string{}
	}
	return &h, nil
}
```

The SELECT in `List`, `GetByID`, `Add`, `Update` must use:
```sql
SELECT id, name, ip, notes, vendor, product_name, product_version, tags, created_at, updated_at
FROM hosts
```

Remove `DecryptCredential` from host_store.go entirely (moved to AccessFaceStore).

- [ ] **Step 5: Build**
  ```
  go build ./internal/store/...
  ```

- [ ] **Step 6: Commit**
  ```
  git add internal/store/access_face_store.go internal/store/fingerprint_store.go \
          internal/store/memory_store.go internal/store/host_store.go
  git commit -m "feat(store): AccessFaceStore, FingerprintStore, MemoryStore; strip SSH from HostStore"
  ```

---

## Task 4: API Layer

**Files:**
- Modify: `internal/api/hosts.go`
- Modify: `cmd/spider/main.go` (route registration only)

- [ ] **Step 1: Add handler functions to `internal/api/hosts.go`**

New dependencies to inject into the handler struct (or pass via closure):
```go
type HostsHandler struct {
	hosts        *store.HostStore
	faces        *store.AccessFaceStore
	fingerprints *store.FingerprintStore
	memories     *store.MemoryStore
}
```

Handler signatures:
```go
func (h *HostsHandler) ListAccessFaces(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) AddAccessFace(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) UpdateAccessFace(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) DeleteAccessFace(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) GetFingerprint(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) ListMemories(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) AddMemory(w http.ResponseWriter, r *http.Request)
func (h *HostsHandler) DeleteMemory(w http.ResponseWriter, r *http.Request)
```

Complete example — `AddAccessFace`:
```go
func (h *HostsHandler) AddAccessFace(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	var req models.AddAccessFaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	face, err := h.faces.Add(hostID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(face)
}
```

- [ ] **Step 2: Register routes in `cmd/spider/main.go`**

Add inside the existing `/api/hosts` route group:
```go
r.Get("/{id}/faces", hh.ListAccessFaces)
r.Post("/{id}/faces", hh.AddAccessFace)
r.Put("/{id}/faces/{faceID}", hh.UpdateAccessFace)
r.Delete("/{id}/faces/{faceID}", hh.DeleteAccessFace)
r.Get("/{id}/fingerprint", hh.GetFingerprint)
r.Get("/{id}/memories", hh.ListMemories)
r.Post("/{id}/memories", hh.AddMemory)
r.Delete("/{id}/memories/{memID}", hh.DeleteMemory)
```

- [ ] **Step 3: Build**
  ```
  go build ./...
  ```

- [ ] **Step 4: Commit**
  ```
  git add internal/api/hosts.go cmd/spider/main.go
  git commit -m "feat(api): access faces, fingerprint, memory endpoints"
  ```

---

## Task 5: SSH Client Refactor

**Files:**
- Modify: `internal/ssh/client.go`
- Modify: `internal/agent/tools_cli.go`

- [ ] **Step 1: Update `newSSHConfig` in `internal/ssh/client.go`**

Change signature from `*models.Host` to `*models.AccessFace`:
```go
func newSSHConfig(face *models.AccessFace, authMethods []gossh.AuthMethod) *gossh.ClientConfig {
	cfg := &gossh.ClientConfig{
		User:            face.Username,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	if face.SSHLegacy {
		cfg.KeyExchanges = append(cfg.KeyExchanges, legacyKexAlgos...)
		cfg.Ciphers = append(cfg.Ciphers, legacyCiphers...)
	}
	return cfg
}
```

Update `Pool.Get()` to accept `*models.AccessFace` and use `face.IP`, `face.Port` for the dial address.

- [ ] **Step 2: Update `internal/agent/tools_cli.go`**

Add `faces *store.AccessFaceStore` field:
```go
type ExecuteCLITool struct {
	hosts   *store.HostStore
	faces   *store.AccessFaceStore
	sshPool *ssh.Pool
	logs    *store.LogStore
	sshKeys *store.SSHKeyStore
}
```

In `Execute`, resolve the SSH face before dialing:
```go
face, err := t.faces.GetSSHFaceForHost(hostID)
if err != nil || face == nil {
	return nil, fmt.Errorf("no SSH access face for host %s", hostID)
}
cred, pass, err := t.faces.DecryptCredential(face)
if err != nil {
	return nil, err
}
client, err := t.sshPool.Get(face, cred, pass, t.sshKeys)
```

- [ ] **Step 3: Build**
  ```
  go build ./...
  ```

- [ ] **Step 4: Commit**
  ```
  git add internal/ssh/client.go internal/agent/tools_cli.go
  git commit -m "refactor(ssh): accept AccessFace instead of Host; resolve face in CLI tool"
  ```

---

## Task 6: REST API Tool Refactor

**Files:**
- Modify: `internal/agent/tools_api.go`

- [ ] **Step 1: Add optional `host_id` / `face_id` to InputSchema**

```go
func (t *CallRESTAPITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_id":  map[string]any{"type": "string", "description": "Optional. Spider host ID. If provided with face_id, auth headers are injected automatically."},
			"face_id":  map[string]any{"type": "string", "description": "Optional. Access face ID to use for auth injection."},
			"url":     map[string]any{"type": "string", "description": "Full URL to call (required if host_id not provided)."},
			"method":  map[string]any{"type": "string", "enum": []string{"GET","POST","PUT","PATCH","DELETE"}},
			"headers": map[string]any{"type": "object", "description": "Additional headers. Merged with injected auth headers."},
			"body":    map[string]any{"type": "string", "description": "Request body (JSON string)."},
		},
		"required": []string{"method"},
	}
}
```

- [ ] **Step 2: Auth injection logic in `Execute`**

```go
// Auth injection: if face_id provided, load AccessFace and inject auth header
if faceID, ok := input["face_id"].(string); ok && faceID != "" {
	face, err := t.faces.GetByID(faceID)
	if err != nil {
		return nil, fmt.Errorf("load access face: %w", err)
	}
	cred, _, err := t.faces.DecryptCredential(face)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	switch face.RESTAuthType {
	case models.RESTAuthBearer:
		req.Header.Set("Authorization", "Bearer "+cred)
	case models.RESTAuthBasic:
		req.SetBasicAuth(face.RESTUsername, cred)
	case models.RESTAuthAPIKey:
		req.Header.Set(face.HeaderName, cred)
	}
	// base_url prefix: if url input is a path, prepend face.BaseURL
	if urlStr, ok := input["url"].(string); ok && len(urlStr) > 0 && urlStr[0] == '/' {
		input["url"] = face.BaseURL + urlStr
	}
}
// Fall through: if no face_id, use headers from AI input as before
```

Add `faces *store.AccessFaceStore` to `CallRESTAPITool` struct.

- [ ] **Step 3: Build**
  ```
  go build ./...
  ```

- [ ] **Step 4: Commit**
  ```
  git add internal/agent/tools_api.go
  git commit -m "feat(agent): REST tool auto-injects auth from AccessFace when face_id provided"
  ```

---

## Task 7: Frontend

**Files:**
- Rewrite: `web/src/api/hosts.ts`
- Modify: `web/src/views/HostsView.vue`

- [ ] **Step 1: Rewrite `web/src/api/hosts.ts`**

```typescript
export interface AccessFace {
  id: string
  host_id: string
  type: 'ssh' | 'restapi'
  ip: string
  port: number
  username?: string
  ssh_auth_type?: 'password' | 'key' | 'key_password'
  ssh_key_id?: string
  ssh_legacy?: boolean
  base_url?: string
  rest_auth_type?: 'bearer' | 'basic' | 'apikey' | 'none'
  rest_username?: string
  header_name?: string
  knowledge_sources: Array<{ type: 'group' | 'doc'; id: number }>
  created_at: string
  updated_at: string
}

export interface Fingerprint {
  host_id: string
  ssh_host_key?: string
  system_version?: string
  hardware_id?: string
  api_signature?: string
  status: 'ok' | 'changed' | 'unverified'
  snapshot_at?: string
}

export interface Memory {
  id: number
  host_id: string
  content: string
  created_by: 'user' | 'agent'
  created_at: string
}

export interface Host {
  id: string
  name: string
  ip: string
  notes?: string
  tags: string[]
  vendor?: string
  product_name?: string
  product_version?: string
  created_at: string
  updated_at: string
  access_faces?: AccessFace[]
  fingerprint?: Fingerprint
  memories?: Memory[]
}

// Host CRUD
export const listHosts = (): Promise<Host[]> =>
  fetch('/api/hosts').then(r => r.json())

export const getHost = (id: string): Promise<Host> =>
  fetch(`/api/hosts/${id}`).then(r => r.json())

export const createHost = (body: Partial<Host>): Promise<Host> =>
  fetch('/api/hosts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => r.json())

export const updateHost = (id: string, body: Partial<Host>): Promise<Host> =>
  fetch(`/api/hosts/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => r.json())

export const deleteHost = (id: string): Promise<void> =>
  fetch(`/api/hosts/${id}`, { method: 'DELETE' }).then(() => undefined)

// Access Faces
export const listFaces = (hostID: string): Promise<AccessFace[]> =>
  fetch(`/api/hosts/${hostID}/faces`).then(r => r.json())

export const addFace = (hostID: string, body: Partial<AccessFace>): Promise<AccessFace> =>
  fetch(`/api/hosts/${hostID}/faces`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => r.json())

export const updateFace = (hostID: string, faceID: string, body: Partial<AccessFace>): Promise<AccessFace> =>
  fetch(`/api/hosts/${hostID}/faces/${faceID}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => r.json())

export const deleteFace = (hostID: string, faceID: string): Promise<void> =>
  fetch(`/api/hosts/${hostID}/faces/${faceID}`, { method: 'DELETE' }).then(() => undefined)

// Fingerprint
export const getFingerprint = (hostID: string): Promise<Fingerprint | null> =>
  fetch(`/api/hosts/${hostID}/fingerprint`).then(r => r.status === 404 ? null : r.json())

// Memories
export const listMemories = (hostID: string): Promise<Memory[]> =>
  fetch(`/api/hosts/${hostID}/memories`).then(r => r.json())

export const addMemory = (hostID: string, content: string, createdBy: 'user' | 'agent' = 'user'): Promise<Memory> =>
  fetch(`/api/hosts/${hostID}/memories`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ content, created_by: createdBy }) }).then(r => r.json())

export const deleteMemory = (hostID: string, memID: number): Promise<void> =>
  fetch(`/api/hosts/${hostID}/memories/${memID}`, { method: 'DELETE' }).then(() => undefined)
```

- [ ] **Step 2: Update `web/src/views/HostsView.vue`**

Key structural changes (do not rewrite the full component — apply surgically):

1. **Host list card**: Remove SSH credential fields (username, password, port). Show `name`, `ip`, `notes`, `tags`, `vendor`, `product_name`.

2. **Add/Edit host form**: Fields are now `name`, `ip`, `notes`, `tags`, `vendor`, `product_name`, `product_version`. Remove all SSH fields from this form.

3. **Host detail panel** (shown when a host row is selected): Add three collapsible sections below the basic info:
   - **Access Faces** — table of faces with type badge (SSH/REST), IP:port, username/base_url. "Add Face" button opens a sub-form. Each row has Edit/Delete.
   - **Fingerprint** — shows status badge (ok/changed/unverified), system_version, hardware_id, snapshot_at.
   - **Memories** — chronological list of memory entries with created_by badge. Text input + "Add" button at bottom.

4. **Access Face sub-form**: Conditional fields based on `type`:
   - SSH: ip, port, username, ssh_auth_type (select), credential (password input), ssh_key_id (if key/key_password), ssh_legacy (checkbox)
   - REST: base_url, rest_auth_type (select), rest_username (if basic), header_name (if apikey), credential

- [ ] **Step 3: Build frontend**
  ```
  cd web && npm run build
  ```

- [ ] **Step 4: Commit**
  ```
  git add web/src/api/hosts.ts web/src/views/HostsView.vue
  git commit -m "feat(ui): new Host types, AccessFace/Fingerprint/Memory API calls and detail panel"
  ```
