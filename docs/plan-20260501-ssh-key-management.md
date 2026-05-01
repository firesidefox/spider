# SSH Key Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SSH key management to personal settings, with host management referencing keys via dropdown.

**Architecture:** New `ssh_keys` table + `SSHKeyStore` for CRUD. Hosts gain optional `ssh_key_id` column. SSH client resolves credentials from either `ssh_key_id` or inline `encrypted_credential`. Frontend adds SSH Keys tab in ProfileView and key dropdown in HostsView modal.

**Tech Stack:** Go 1.23, SQLite, Vue 3 + TypeScript, AES-256-GCM encryption

---

## File Structure

**Create:**
- `internal/models/ssh_key.go` — SSHKey, SafeSSHKey, AddSSHKeyRequest models
- `internal/store/ssh_key_store.go` — SSHKeyStore CRUD + reference check
- `internal/api/ssh_keys.go` — HTTP handlers for `/api/v1/me/ssh-keys`
- `web/src/api/ssh-keys.ts` — Frontend API client

**Modify:**
- `internal/db/schema.go` — Add ssh_keys table + hosts.ssh_key_id column
- `internal/models/host.go` — Add SSHKeyID to Host, SafeHost, request structs
- `internal/store/host_store.go` — Persist/read ssh_key_id, update scan functions
- `internal/mcp/server.go` — Add SSHKeyStore to App struct
- `internal/api/handler.go` — Register ssh-keys routes
- `internal/api/hosts.go` — Validate ssh_key_id on add/update
- `internal/ssh/client.go` — Resolve credential from ssh_key_id
- `internal/mcp/tools.go` — Add ssh key MCP tools, update add/update_host
- `web/src/api/hosts.ts` — Add ssh_key_id to types
- `web/src/views/ProfileView.vue` — Add SSH Keys tab
- `web/src/views/HostsView.vue` — Add key dropdown in modal

---

### Task 1: Database Schema — ssh_keys table + hosts.ssh_key_id

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add ssh_keys table and hosts.ssh_key_id migration**

In `internal/db/schema.go`, append to `schemaSQL`:

```sql
CREATE TABLE IF NOT EXISTS ssh_keys (
    id                    TEXT PRIMARY KEY,
    user_id               TEXT NOT NULL,
    name                  TEXT NOT NULL,
    encrypted_private_key TEXT NOT NULL,
    encrypted_passphrase  TEXT NOT NULL DEFAULT '',
    fingerprint           TEXT NOT NULL DEFAULT '',
    created_at            DATETIME NOT NULL,
    updated_at            DATETIME NOT NULL,
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_ssh_keys_user_id ON ssh_keys(user_id);
```

In `migrate()`, add after the existing `ALTER TABLE execution_logs` block:

```go
_, err = db.Exec(`ALTER TABLE hosts ADD COLUMN ssh_key_id TEXT NOT NULL DEFAULT ''`)
if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
    return err
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat: add ssh_keys table and hosts.ssh_key_id column"
```

### Task 2: SSHKey Model

**Files:**
- Create: `internal/models/ssh_key.go`

- [ ] **Step 1: Create SSHKey model file**

```go
package models

import "time"

type SSHKey struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	EncryptedPrivateKey string    `json:"-"`
	EncryptedPassphrase string    `json:"-"`
	Fingerprint         string    `json:"fingerprint"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type SafeSSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (k *SSHKey) Safe() *SafeSSHKey {
	return &SafeSSHKey{
		ID:          k.ID,
		Name:        k.Name,
		Fingerprint: k.Fingerprint,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
	}
}

type AddSSHKeyRequest struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase"`
}
```

- [ ] **Step 2: Add SSHKeyID to Host model**

In `internal/models/host.go`, add field to `Host` struct after `EncryptedPassphrase`:

```go
SSHKeyID            string    `json:"-"`
```

Add field to `SafeHost` after `AuthType`:

```go
SSHKeyID   string `json:"ssh_key_id,omitempty"`
SSHKeyName string `json:"ssh_key_name,omitempty"`
```

Update `Safe()` method to include `SSHKeyID`:

```go
func (h *Host) Safe() *SafeHost {
	return &SafeHost{
		ID:        h.ID,
		Name:      h.Name,
		IP:        h.IP,
		Port:      h.Port,
		Username:  h.Username,
		AuthType:  h.AuthType,
		SSHKeyID:  h.SSHKeyID,
		Tags:      h.Tags,
		CreatedAt: h.CreatedAt,
		UpdatedAt: h.UpdatedAt,
	}
}
```

Add `SSHKeyID` to `AddHostRequest`:

```go
SSHKeyID   string   `json:"ssh_key_id"`
```

Add `SSHKeyID` to `UpdateHostRequest`:

```go
SSHKeyID   *string  `json:"ssh_key_id"`
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/models/ssh_key.go internal/models/host.go
git commit -m "feat: add SSHKey model and SSHKeyID to Host"
```

### Task 3: SSHKeyStore — CRUD operations

**Files:**
- Create: `internal/store/ssh_key_store.go`

- [ ] **Step 1: Create SSHKeyStore**

```go
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
)

type SSHKeyStore struct {
	db     *sql.DB
	crypto *crypto.Manager
}

func NewSSHKeyStore(db *sql.DB, cm *crypto.Manager) *SSHKeyStore {
	return &SSHKeyStore{db: db, crypto: cm}
}
```

- [ ] **Step 2: Add `Add` method with fingerprint parsing**

```go
func (s *SSHKeyStore) Add(userID string, req *models.AddSSHKeyRequest) (*models.SSHKey, error) {
	if req.Name == "" || req.PrivateKey == "" {
		return nil, fmt.Errorf("name 和 private_key 不能为空")
	}

	fingerprint, err := parseFingerprint(req.PrivateKey, req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
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
		`INSERT INTO ssh_keys (id, user_id, name, encrypted_private_key, encrypted_passphrase, fingerprint, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, req.Name, encKey, encPass, fingerprint, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, fmt.Errorf("密钥名称 %q 已存在", req.Name)
		}
		return nil, fmt.Errorf("创建密钥失败: %w", err)
	}
	return s.GetByID(id)
}

func parseFingerprint(privateKey, passphrase string) (string, error) {
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
```

- [ ] **Step 3: Add query methods**

```go
func (s *SSHKeyStore) GetByID(id string) (*models.SSHKey, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, encrypted_private_key, encrypted_passphrase, fingerprint, created_at, updated_at
		 FROM ssh_keys WHERE id = ?`, id,
	)
	return scanSSHKey(row)
}

func (s *SSHKeyStore) ListByUser(userID string) ([]*models.SSHKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, encrypted_private_key, encrypted_passphrase, fingerprint, created_at, updated_at
		 FROM ssh_keys WHERE user_id = ? ORDER BY name`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询密钥列表失败: %w", err)
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
```

- [ ] **Step 4: Add Delete with reference check + scan helpers**

```go
func (s *SSHKeyStore) Delete(id, userID string) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE ssh_key_id = ?`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查引用失败: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("CONFLICT:密钥被 %d 台主机引用，无法删除", count)
	}
	res, err := s.db.Exec(`DELETE FROM ssh_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("删除密钥失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("密钥不存在")
	}
	return nil
}

func (s *SSHKeyStore) GetRefCount(id string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE ssh_key_id = ?`, id).Scan(&count)
	return count, err
}

func scanSSHKey(row *sql.Row) (*models.SSHKey, error) {
	var k models.SSHKey
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.EncryptedPrivateKey, &k.EncryptedPassphrase, &k.Fingerprint, &k.CreatedAt, &k.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("密钥不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("扫描密钥数据失败: %w", err)
	}
	return &k, nil
}

func scanSSHKeyRows(rows *sql.Rows) (*models.SSHKey, error) {
	var k models.SSHKey
	err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.EncryptedPrivateKey, &k.EncryptedPassphrase, &k.Fingerprint, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("扫描密钥数据失败: %w", err)
	}
	return &k, nil
}
```

- [ ] **Step 5: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/store/ssh_key_store.go
git commit -m "feat: add SSHKeyStore with CRUD and reference check"
```

### Task 4: Update HostStore — persist and read ssh_key_id

**Files:**
- Modify: `internal/store/host_store.go`

- [ ] **Step 1: Update Add method**

In `HostStore.Add()`, add `req.SSHKeyID` to the INSERT statement. Change the SQL to:

```sql
INSERT INTO hosts (id, name, ip, port, username, auth_type, encrypted_credential,
 encrypted_passphrase, tags, ssh_key_id, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```

Add `req.SSHKeyID` as the 10th parameter (before `now, now`).

- [ ] **Step 2: Update all SELECT queries to include ssh_key_id**

Every SELECT in `GetByID`, `GetByName`, `List` must add `ssh_key_id` to the column list. The column order becomes:

```
id, name, ip, port, username, auth_type, encrypted_credential,
encrypted_passphrase, tags, ssh_key_id, created_at, updated_at
```

- [ ] **Step 3: Update scanHost and scanHostRows**

Add `&h.SSHKeyID` to the Scan call in both functions, after `&tagsJSON`:

```go
err := row.Scan(
    &h.ID, &h.Name, &h.IP, &h.Port, &h.Username, &authType,
    &h.EncryptedCredential, &h.EncryptedPassphrase,
    &tagsJSON, &h.SSHKeyID, &h.CreatedAt, &h.UpdatedAt,
)
```

Same for `scanHostRows`.

- [ ] **Step 4: Update Update method**

Add handling for `req.SSHKeyID`:

```go
if req.SSHKeyID != nil {
    h.SSHKeyID = *req.SSHKeyID
}
```

Update the UPDATE SQL to include `ssh_key_id=?` and add `h.SSHKeyID` to the parameter list.

```sql
UPDATE hosts SET name=?, ip=?, port=?, username=?, auth_type=?,
 encrypted_credential=?, encrypted_passphrase=?,
 tags=?, ssh_key_id=?, updated_at=? WHERE id=?
```

Parameters: `h.Name, h.IP, h.Port, h.Username, string(h.AuthType), h.EncryptedCredential, h.EncryptedPassphrase, string(tagsJSON), h.SSHKeyID, h.UpdatedAt, id`

- [ ] **Step 5: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/store/host_store.go
git commit -m "feat: persist and read ssh_key_id in HostStore"
```

### Task 5: Wire SSHKeyStore into App + update SSH client

**Files:**
- Modify: `internal/mcp/server.go` — Add SSHKeyStore field to App
- Modify: `internal/ssh/client.go` — Add NewClientWithKey constructor
- Modify: `internal/ssh/pool.go` — Update Get to accept SSHKeyStore

- [ ] **Step 1: Add SSHKeyStore to App struct**

In `internal/mcp/server.go`, add to the `App` struct:

```go
SSHKeyStore *store.SSHKeyStore
```

- [ ] **Step 2: Add NewClientWithKey to ssh/client.go**

Add a new constructor that takes decrypted key material directly, so the caller (pool/API) can resolve from either HostStore or SSHKeyStore:

```go
func NewClientWithCredential(host *models.Host, credential, passphrase string) (*Client, error) {
	authMethods, err := buildAuthMethods(host.AuthType, credential, passphrase)
	if err != nil {
		return nil, err
	}
	cfg := &gossh.ClientConfig{
		User:            host.Username,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host.IP, host.Port)
	conn, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接 %s 失败: %w", addr, err)
	}
	return &Client{conn: conn, host: host}, nil
}
```

- [ ] **Step 3: Update Pool.Get to accept SSHKeyStore**

Change `Pool.Get` signature to:

```go
func (p *Pool) Get(host *models.Host, hs *store.HostStore, ks *store.SSHKeyStore) (*Client, error)
```

Inside, after the cache check, resolve credentials:

```go
var credential, passphrase string
var err error
if host.SSHKeyID != "" && ks != nil {
    key, kerr := ks.GetByID(host.SSHKeyID)
    if kerr != nil {
        return nil, fmt.Errorf("获取 SSH key 失败: %w", kerr)
    }
    credential, passphrase, err = ks.DecryptKey(key)
} else {
    credential, passphrase, err = hs.DecryptCredential(host)
}
if err != nil {
    return nil, err
}
client, err := NewClientWithCredential(host, credential, passphrase)
```

- [ ] **Step 4: Update CheckConnectivity signature**

In `ssh/client.go`, update `CheckConnectivity`:

```go
func CheckConnectivity(host *models.Host, hs *store.HostStore, ks *store.SSHKeyStore) (latency time.Duration, err error) {
```

Inside, replace `NewClient(host, hs)` with the same credential resolution logic:

```go
var credential, passphrase string
if host.SSHKeyID != "" && ks != nil {
    key, kerr := ks.GetByID(host.SSHKeyID)
    if kerr != nil {
        return tcpLatency, fmt.Errorf("获取 SSH key 失败: %w", kerr)
    }
    credential, passphrase, err = ks.DecryptKey(key)
} else {
    credential, passphrase, err = hs.DecryptCredential(host)
}
if err != nil {
    return tcpLatency, fmt.Errorf("解密凭据失败: %w", err)
}
client, err := NewClientWithCredential(host, credential, passphrase)
```

- [ ] **Step 5: Fix all callers of Pool.Get and CheckConnectivity**

Search for all call sites of `app.Pool.Get(` and `sshpkg.CheckConnectivity(` and add `app.SSHKeyStore` as the third argument. These are in:

- `internal/mcp/tools.go` — `makeExecuteCommand`, `makeExecuteCommandBatch`, `makeCheckConnectivity`, `makeUploadFile`, `makeDownloadFile`
- `internal/api/hosts.go` — `pingHost`

- [ ] **Step 6: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/server.go internal/ssh/client.go internal/ssh/pool.go internal/mcp/tools.go internal/api/hosts.go
git commit -m "feat: resolve SSH credentials from ssh_key_id or inline credential"
```

### Task 6: SSH Keys API handlers

**Files:**
- Create: `internal/api/ssh_keys.go`
- Modify: `internal/api/handler.go` — Register routes

- [ ] **Step 1: Create ssh_keys.go with list and add handlers**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	authmw "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
)

func listSSHKeys(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	uc := authmw.GetUser(r.Context())
	keys, err := app.SSHKeyStore.ListByUser(uc.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	safe := make([]*models.SafeSSHKey, 0, len(keys))
	for _, k := range keys {
		safe = append(safe, k.Safe())
	}
	writeJSON(w, http.StatusOK, safe)
}

func addSSHKey(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	uc := authmw.GetUser(r.Context())
	var req models.AddSSHKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	key, err := app.SSHKeyStore.Add(uc.UserID, &req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "已存在") {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, key.Safe())
}

func getSSHKey(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	uc := authmw.GetUser(r.Context())
	key, err := app.SSHKeyStore.GetByID(id)
	if err != nil || key.UserID != uc.UserID {
		writeError(w, http.StatusNotFound, "密钥不存在")
		return
	}
	writeJSON(w, http.StatusOK, key.Safe())
}

func deleteSSHKey(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	uc := authmw.GetUser(r.Context())
	err := app.SSHKeyStore.Delete(id, uc.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "CONFLICT") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}
```

- [ ] **Step 2: Register routes in handler.go**

In `NewRouter`, before the auth middleware section, add:

```go
mux.HandleFunc("/api/v1/me/ssh-keys", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        listSSHKeys(app, w, r)
    case http.MethodPost:
        addSSHKey(app, w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})

mux.HandleFunc("/api/v1/me/ssh-keys/", func(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Path[len("/api/v1/me/ssh-keys/"):]
    switch r.Method {
    case http.MethodGet:
        getSSHKey(app, w, r, id)
    case http.MethodDelete:
        deleteSSHKey(app, w, r, id)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
```

- [ ] **Step 3: Update addHost/updateHost to validate ssh_key_id**

In `internal/api/hosts.go`, update `addHost`:

After decoding the request, add validation:

```go
if req.SSHKeyID != "" && req.Credential != "" {
    writeError(w, http.StatusBadRequest, "ssh_key_id 和 credential 不能同时提供")
    return
}
if req.SSHKeyID != "" {
    uc := authmw.GetUser(r.Context())
    key, err := app.SSHKeyStore.GetByID(req.SSHKeyID)
    if err != nil || key.UserID != uc.UserID {
        writeError(w, http.StatusBadRequest, "ssh_key_id 无效或不属于当前用户")
        return
    }
}
```

Add the same validation in `updateHost` for `req.SSHKeyID` (when non-nil and non-empty).

Add import for `authmw` if not already present in hosts.go (it's not currently imported there).

- [ ] **Step 4: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/api/ssh_keys.go internal/api/handler.go internal/api/hosts.go
git commit -m "feat: add SSH keys API endpoints and host validation"
```

### Task 7: MCP tools — ssh key management + update host tools

**Files:**
- Modify: `internal/mcp/server.go` — Register new tools
- Modify: `internal/mcp/tools.go` — Add ssh key handlers, update add/update_host

- [ ] **Step 1: Add MCP tool registrations in registerTools**

In `internal/mcp/server.go` `registerTools()`, add after `get_execution_history`:

```go
// list_ssh_keys
s.AddTool(mcpgo.NewTool("list_ssh_keys",
    mcpgo.WithDescription("列出当前用户的 SSH 密钥"),
), makeListSSHKeys(app))

// add_ssh_key
s.AddTool(mcpgo.NewTool("add_ssh_key",
    mcpgo.WithDescription("添加一个 SSH 私钥"),
    mcpgo.WithString("name", mcpgo.Required(), mcpgo.Description("密钥名称")),
    mcpgo.WithString("private_key", mcpgo.Required(), mcpgo.Description("PEM 格式私钥内容")),
    mcpgo.WithString("passphrase", mcpgo.Description("私钥 passphrase（可选）")),
), makeAddSSHKey(app))

// remove_ssh_key
s.AddTool(mcpgo.NewTool("remove_ssh_key",
    mcpgo.WithDescription("删除一个 SSH 密钥（被主机引用时无法删除）"),
    mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("密钥 ID")),
), makeRemoveSSHKey(app))
```

Also update `add_host` tool definition: add `ssh_key_id` parameter, make `credential` no longer Required:

```go
s.AddTool(mcpgo.NewTool("add_host",
    mcpgo.WithDescription("添加一台新的被管理主机"),
    mcpgo.WithString("name", mcpgo.Required(), mcpgo.Description("主机唯一名称")),
    mcpgo.WithString("ip", mcpgo.Required(), mcpgo.Description("主机 IP 地址")),
    mcpgo.WithNumber("port", mcpgo.Description("SSH 端口，默认 22")),
    mcpgo.WithString("username", mcpgo.Required(), mcpgo.Description("SSH 登录用户名")),
    mcpgo.WithString("auth_type", mcpgo.Required(), mcpgo.Description("认证类型: password | key | key_password")),
    mcpgo.WithString("credential", mcpgo.Description("密码明文 或 SSH 私钥内容（与 ssh_key_id 二选一）")),
    mcpgo.WithString("ssh_key_id", mcpgo.Description("SSH 密钥 ID（与 credential 二选一）")),
    mcpgo.WithString("passphrase", mcpgo.Description("私钥 passphrase（auth_type=key_password 时使用）")),
    mcpgo.WithString("tags", mcpgo.Description("逗号分隔的标签，例如 prod,web")),
), makeAddHost(app))
```

Add `ssh_key_id` to `update_host` tool definition too:

```go
mcpgo.WithString("ssh_key_id", mcpgo.Description("SSH 密钥 ID")),
```

- [ ] **Step 2: Add MCP handler functions in tools.go**

```go
func makeListSSHKeys(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		keys, err := app.SSHKeyStore.ListByUser("anonymous")
		if err != nil {
			return toolError(fmt.Sprintf("查询密钥列表失败: %v", err))
		}
		safe := make([]*models.SafeSSHKey, 0, len(keys))
		for _, k := range keys {
			safe = append(safe, k.Safe())
		}
		data, _ := json.MarshalIndent(safe, "", "  ")
		return toolText(string(data))
	}
}

func makeAddSSHKey(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		addReq := &models.AddSSHKeyRequest{
			Name:       getString(args, "name"),
			PrivateKey: getString(args, "private_key"),
			Passphrase: getString(args, "passphrase"),
		}
		key, err := app.SSHKeyStore.Add("anonymous", addReq)
		if err != nil {
			return toolError(fmt.Sprintf("添加密钥失败: %v", err))
		}
		data, _ := json.MarshalIndent(key.Safe(), "", "  ")
		return toolText(fmt.Sprintf("密钥添加成功:\n%s", string(data)))
	}
}

func makeRemoveSSHKey(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		id := getString(req.GetArguments(), "id")
		if id == "" {
			return toolError("id 不能为空")
		}
		if err := app.SSHKeyStore.Delete(id, "anonymous"); err != nil {
			return toolError(fmt.Sprintf("删除密钥失败: %v", err))
		}
		return toolText(fmt.Sprintf("密钥 %s 已删除", id))
	}
}
```

Note: MCP tools use "anonymous" as userID since MCP doesn't have user context. This matches the existing auth-disabled pattern.

- [ ] **Step 3: Update makeAddHost to handle ssh_key_id**

In `makeAddHost`, after building `addReq`, add:

```go
sshKeyID := getString(args, "ssh_key_id")
if sshKeyID != "" && addReq.Credential != "" {
    return toolError("ssh_key_id 和 credential 不能同时提供")
}
addReq.SSHKeyID = sshKeyID
```

In `makeUpdateHost`, add:

```go
if v := getString(args, "ssh_key_id"); v != "" {
    updateReq.SSHKeyID = &v
}
```

- [ ] **Step 4: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools.go
git commit -m "feat: add SSH key MCP tools and update host tools"
```

### Task 8: Wire SSHKeyStore in main startup

**Files:**
- Modify: `cmd/spider/main.go` (or wherever App is constructed)

- [ ] **Step 1: Find where App is constructed**

Search for `mcp.App{` or `mcppkg.App{` in the codebase. Add `SSHKeyStore` initialization:

```go
SSHKeyStore: store.NewSSHKeyStore(db, cryptoMgr),
```

This should be alongside the existing `HostStore`, `UserStore`, `TokenStore` initialization.

- [ ] **Step 2: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add cmd/
git commit -m "feat: wire SSHKeyStore into App startup"
```

### Task 9: Frontend API client for SSH keys

**Files:**
- Create: `web/src/api/ssh-keys.ts`
- Modify: `web/src/api/hosts.ts` — Add ssh_key_id to types

- [ ] **Step 1: Create ssh-keys.ts**

```typescript
import { authHeaders } from './auth'

export interface SafeSSHKey {
  id: string
  name: string
  fingerprint: string
  created_at: string
  updated_at: string
}

export async function listSSHKeys(): Promise<SafeSSHKey[]> {
  const res = await fetch('/api/v1/me/ssh-keys', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function addSSHKey(name: string, privateKey: string, passphrase?: string): Promise<SafeSSHKey> {
  const body: any = { name, private_key: privateKey }
  if (passphrase) body.passphrase = passphrase
  const res = await fetch('/api/v1/me/ssh-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteSSHKey(id: string): Promise<void> {
  const res = await fetch(`/api/v1/me/ssh-keys/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}
```

- [ ] **Step 2: Update hosts.ts types**

Add `ssh_key_id` and `ssh_key_name` to `SafeHost`:

```typescript
export interface SafeHost {
  id: string
  name: string
  ip: string
  port: number
  username: string
  auth_type: string
  ssh_key_id?: string
  ssh_key_name?: string
  tags: string[]
  created_at: string
  updated_at: string
}
```

Add `ssh_key_id` to `AddHostRequest`:

```typescript
export interface AddHostRequest {
  name: string
  ip: string
  port: number
  username: string
  auth_type: string
  credential?: string
  ssh_key_id?: string
  passphrase?: string
  tags: string[]
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api/ssh-keys.ts web/src/api/hosts.ts
git commit -m "feat: add SSH keys frontend API client"
```

### Task 10: ProfileView — SSH Keys tab

**Files:**
- Modify: `web/src/views/ProfileView.vue`

- [ ] **Step 1: Add SSH Keys nav item**

In the sidebar `<nav>`, after the Tokens nav-row and before the Logs nav-row, add:

```html
<div class="nav-row" :class="{ selected: activeTab === 'ssh-keys' }" @click="activeTab = 'ssh-keys'; loadSSHKeys()">
  <span class="nav-icon">🔐</span><span class="nav-label">SSH Keys</span>
</div>
```

Update `activeTab` type to include `'ssh-keys'`:

```typescript
const activeTab = ref<'info' | 'tokens' | 'ssh-keys' | 'logs' | 'users' | 'install' | 'skills' | 'settings'>('info')
```

Add `'ssh-keys': 'SSH Keys'` to `tabTitle` computed.

- [ ] **Step 2: Add SSH Keys tab template**

After the tokens template block and before the logs template block, add:

```html
<template v-if="activeTab === 'ssh-keys'">
  <div class="edit-card">
    <p class="dim" style="margin-bottom:16px;font-size:13px">管理 SSH 私钥，可在添加主机时引用。</p>
    <table class="table">
      <thead><tr><th>名称</th><th>指纹</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="k in sshKeys" :key="k.id">
          <td style="font-weight:500;color:var(--text)">{{ k.name }}</td>
          <td class="dim" style="font-family:'SF Mono',Consolas,monospace;font-size:12px">{{ k.fingerprint.slice(0, 24) }}…</td>
          <td class="dim">{{ new Date(k.created_at).toLocaleString() }}</td>
          <td><button class="btn btn-sm btn-danger" @click="handleDeleteKey(k.id)">删除</button></td>
        </tr>
        <tr v-if="sshKeys.length === 0">
          <td colspan="4" class="dim" style="text-align:center;padding:32px">暂无 SSH Key</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
```

Add topbar button for SSH Keys tab:

```html
<button v-if="activeTab === 'ssh-keys'" class="btn btn-primary btn-sm" @click="showAddKey = true">+ 添加 Key</button>
```

- [ ] **Step 3: Add SSH Key modal**

After the existing modals, add:

```html
<div v-if="showAddKey" class="modal-overlay" @click.self="showAddKey = false">
  <div class="modal">
    <h3>添加 SSH Key</h3>
    <div class="form-row"><label>名称</label><input v-model="keyForm.name" class="input" placeholder="prod-key" /></div>
    <div class="form-row">
      <label>私钥内容</label>
      <textarea v-model="keyForm.privateKey" class="input" rows="5" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
    </div>
    <div class="form-row"><label>Passphrase（可选）</label><input v-model="keyForm.passphrase" type="password" class="input" /></div>
    <div v-if="keyFormError" class="err" style="margin-bottom:12px">{{ keyFormError }}</div>
    <div class="modal-footer">
      <button class="btn" @click="showAddKey = false">取消</button>
      <button class="btn btn-primary" @click="handleAddKey">添加</button>
    </div>
  </div>
</div>
```

- [ ] **Step 4: Add script logic**

Add imports:

```typescript
import { listSSHKeys, addSSHKey, deleteSSHKey } from '../api/ssh-keys'
import type { SafeSSHKey } from '../api/ssh-keys'
```

Add state:

```typescript
const sshKeys = ref<SafeSSHKey[]>([])
const showAddKey = ref(false)
const keyForm = ref({ name: '', privateKey: '', passphrase: '' })
const keyFormError = ref('')
let sshKeysLoaded = false

async function loadSSHKeys() {
  if (sshKeysLoaded) return
  sshKeysLoaded = true
  sshKeys.value = await listSSHKeys()
}

async function handleAddKey() {
  keyFormError.value = ''
  if (!keyForm.value.name.trim()) { keyFormError.value = '请输入名称'; return }
  if (!keyForm.value.privateKey.trim()) { keyFormError.value = '请输入私钥内容'; return }
  try {
    await addSSHKey(keyForm.value.name, keyForm.value.privateKey, keyForm.value.passphrase || undefined)
    showAddKey.value = false
    keyForm.value = { name: '', privateKey: '', passphrase: '' }
    sshKeysLoaded = false
    sshKeys.value = await listSSHKeys()
    sshKeysLoaded = true
  } catch (e: any) { keyFormError.value = e.message }
}

async function handleDeleteKey(id: string) {
  if (!confirm('确认删除此 SSH Key？')) return
  try {
    await deleteSSHKey(id)
    sshKeys.value = await listSSHKeys()
  } catch (e: any) { alert(e.message) }
}
```

- [ ] **Step 5: Verify frontend builds**

Run: `cd /Users/cw/fty.ai/spider.ai/web && npm run build`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat: add SSH Keys tab in ProfileView"
```

### Task 11: HostsView — SSH Key dropdown in add/edit modal

**Files:**
- Modify: `web/src/views/HostsView.vue`

- [ ] **Step 1: Add SSH key state and loading**

Add imports:

```typescript
import { listSSHKeys } from '../api/ssh-keys'
import type { SafeSSHKey } from '../api/ssh-keys'
```

Add state:

```typescript
const sshKeys = ref<SafeSSHKey[]>([])

async function loadSSHKeys() {
  sshKeys.value = await listSSHKeys()
}
```

Call `loadSSHKeys()` in `onMounted` alongside `load()`.

Add `ssh_key_id` to `emptyForm`:

```typescript
const emptyForm = () => ({ name: '', ip: '', port: 22, username: '', auth_type: 'password', credential: '', passphrase: '', ssh_key_id: '', tagsStr: '' })
```

Update `editHost` to populate `ssh_key_id`:

```typescript
function editHost(h: SafeHost) {
  editTarget.value = h
  form.value = { name: h.name, ip: h.ip, port: h.port, username: h.username, auth_type: h.auth_type, credential: '', passphrase: '', ssh_key_id: h.ssh_key_id || '', tagsStr: h.tags.join(',') }
}
```

- [ ] **Step 2: Add dropdown to modal template**

In the modal form, after the `auth_type` select and before the credential textarea, add a conditional SSH key dropdown. Replace the existing credential form-row with:

```html
<template v-if="form.auth_type === 'key' || form.auth_type === 'key_password'">
  <div class="form-row">
    <label>SSH Key</label>
    <select v-model="form.ssh_key_id" class="input" @change="if (form.ssh_key_id) form.credential = ''">
      <option value="">不使用已有 Key</option>
      <option v-for="k in sshKeys" :key="k.id" :value="k.id">
        {{ k.name }} ({{ k.fingerprint.slice(0, 16) }}…)
      </option>
    </select>
  </div>
  <div class="form-row">
    <label>私钥内容</label>
    <textarea v-model="form.credential" class="input" rows="3" placeholder="PEM 格式私钥" :disabled="!!form.ssh_key_id" @input="if (form.credential) form.ssh_key_id = ''" />
  </div>
</template>
<template v-else>
  <div class="form-row">
    <label>密码</label>
    <textarea v-model="form.credential" class="input" rows="3" placeholder="登录密码" />
  </div>
</template>
```

- [ ] **Step 3: Update submitHost to send ssh_key_id**

In `submitHost`, update the `updateHost` and `addHost` calls to include `ssh_key_id`:

```typescript
async function submitHost() {
  const tags = form.value.tagsStr.split(',').map(t => t.trim()).filter(Boolean)
  if (editTarget.value) {
    await updateHost(editTarget.value.id, {
      name: form.value.name || undefined,
      ip: form.value.ip,
      port: form.value.port,
      username: form.value.username,
      auth_type: form.value.auth_type,
      credential: form.value.credential || undefined,
      ssh_key_id: form.value.ssh_key_id || undefined,
      passphrase: form.value.passphrase || undefined,
      tags,
    })
    if (activeHost.value?.id === editTarget.value.id) {
      activeHost.value = { ...activeHost.value, ...form.value, tags }
    }
  } else {
    await addHost({ ...form.value, tags })
  }
  closeModal()
  load()
}
```

- [ ] **Step 4: Show ssh_key_name in host detail**

In the detail-grid, after the "认证方式" field, add:

```html
<div v-if="activeHost.ssh_key_id" class="detail-field">
  <div class="detail-label">SSH Key</div>
  <div class="detail-value">{{ activeHost.ssh_key_name || activeHost.ssh_key_id }}</div>
</div>
```

- [ ] **Step 5: Verify frontend builds**

Run: `cd /Users/cw/fty.ai/spider.ai/web && npm run build`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add web/src/views/HostsView.vue
git commit -m "feat: add SSH key dropdown in host add/edit modal"
```

### Task 12: Populate ssh_key_name in SafeHost response

**Files:**
- Modify: `internal/api/hosts.go`

The `SafeHost.SSHKeyName` field needs to be populated when returning host data. This requires looking up the key name from the SSHKeyStore.

- [ ] **Step 1: Create a helper to enrich SafeHost with key name**

In `internal/api/hosts.go`, add a helper:

```go
func enrichSafeHost(app *mcppkg.App, h *models.Host) *models.SafeHost {
	safe := h.Safe()
	if safe.SSHKeyID != "" && app.SSHKeyStore != nil {
		if key, err := app.SSHKeyStore.GetByID(safe.SSHKeyID); err == nil {
			safe.SSHKeyName = key.Name
		}
	}
	return safe
}
```

- [ ] **Step 2: Use enrichSafeHost in all host API responses**

Replace `h.Safe()` with `enrichSafeHost(app, h)` in:
- `listHosts` — in the loop
- `addHost` — the response
- `getHost` — the response
- `updateHost` — the response

- [ ] **Step 3: Verify build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/api/hosts.go
git commit -m "feat: populate ssh_key_name in SafeHost API responses"
```

### Task 13: End-to-end verification

- [ ] **Step 1: Full build**

Run: `cd /Users/cw/fty.ai/spider.ai && go build ./... && cd web && npm run build`
Expected: both succeed

- [ ] **Step 2: Start server and test in browser**

Run the server, open the web UI, and verify:
1. ProfileView shows SSH Keys tab
2. Can add a new SSH key (paste a test key)
3. Key appears in list with fingerprint
4. HostsView add modal shows key dropdown for key/key_password auth types
5. Selecting a key disables the textarea; typing in textarea clears the dropdown
6. Can create a host referencing an SSH key
7. Host detail shows the SSH key name
8. Cannot delete a key that is referenced by a host (409 error)

- [ ] **Step 3: Final commit if any fixes needed**
