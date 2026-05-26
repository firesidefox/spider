# Prometheus Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Prometheus data source management and two agent tools (`ListMetrics`, `QueryMetrics`) to spider.ai.

**Architecture:** Standalone `prometheus_sources` table stores Prometheus instances; `prometheus_bindings` table scopes them to topology layers or individual hosts (host binding overrides layer binding). An internal HTTP client wraps Prometheus HTTP API. Two new agent tools discover metrics and run PromQL queries. A new Settings → Data Sources → Prometheus page (Vue 3, right-side drawer) manages sources.

**Tech Stack:** Go 1.21, SQLite (database/sql), Vue 3 (Composition API, TypeScript), existing `crypto.Manager` for secret encryption.

**Spec:** `docs/superpowers/specs/2026-05-24-prometheus-integration-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/db/schema.go` | Modify | Add table DDL + ALTER TABLE migrations |
| `internal/models/prometheus.go` | Create | `PrometheusSource`, `PrometheusBinding`, request/response types |
| `internal/store/prometheus_store.go` | Create | CRUD for sources + bindings, source lookup by host_id |
| `internal/store/prometheus_store_test.go` | Create | Unit tests for store |
| `internal/prometheus/client.go` | Create | HTTP client wrapping Prometheus API |
| `internal/prometheus/client_test.go` | Create | Unit tests using httptest server |
| `internal/api/prometheus_handlers.go` | Create | REST handlers for sources + bindings CRUD + test-connection |
| `internal/api/handler.go` | Modify | Register new routes |
| `internal/mcp/server.go` | Modify | Add `PrometheusSourceStore`, `PrometheusBindingStore` to `App` struct |
| `cmd/spider/main.go` | Modify | Wire up new stores |
| `internal/agent/tools_prometheus.go` | Create | `ListMetricsTool` + `QueryMetricsTool` |
| `internal/agent/factory.go` | Modify | Add store fields, register tools |
| `web/src/api/prometheus.ts` | Create | TypeScript API client |
| `web/src/views/SettingsView.vue` | Create | Settings shell with sidebar (hosts Data Sources section) |
| `web/src/components/PrometheusDataSourcesPanel.vue` | Create | List + drawer for source CRUD |
| `web/src/main.ts` | Modify | Add `/settings` and `/settings/prometheus` routes |

---

## Task 1: DB Schema — prometheus_sources + prometheus_bindings

**Files:**
- Modify: `internal/db/schema.go`
- Test: `internal/db/schema_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/db/schema_test.go — add to existing test file
func TestPrometheusTables(t *testing.T) {
    db := openTestDB(t)
    // sources table
    _, err := db.Exec(`INSERT INTO prometheus_sources
        (id,name,base_url,timeout_seconds,auth_type,username,
         encrypted_password,encrypted_token,skip_tls_verify,created_at,updated_at)
        VALUES ('s1','test','http://localhost:9090',30,'none','','','',0,
                datetime('now'),datetime('now'))`)
    if err != nil {
        t.Fatalf("insert prometheus_sources: %v", err)
    }
    // bindings table — topology_layer scope
    _, err = db.Exec(`INSERT INTO prometheus_bindings
        (id,source_id,scope_type,topology_id,layer,host_id,created_at)
        VALUES ('b1','s1','topology_layer','topo1','server',NULL,datetime('now'))`)
    if err != nil {
        t.Fatalf("insert prometheus_bindings (topology_layer): %v", err)
    }
    // bindings table — host scope
    _, err = db.Exec(`INSERT INTO prometheus_bindings
        (id,source_id,scope_type,topology_id,layer,host_id,created_at)
        VALUES ('b2','s1','host',NULL,NULL,'host1',datetime('now'))`)
    if err != nil {
        t.Fatalf("insert prometheus_bindings (host): %v", err)
    }
    // unique constraint on (topology_id, layer)
    _, err = db.Exec(`INSERT INTO prometheus_bindings
        (id,source_id,scope_type,topology_id,layer,host_id,created_at)
        VALUES ('b3','s1','topology_layer','topo1','server',NULL,datetime('now'))`)
    if err == nil {
        t.Fatal("expected unique constraint violation for duplicate (topology_id, layer)")
    }
}
```

- [ ] **Step 2: Run test, expect failure**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/db/ -run TestPrometheusTables -v
```
Expected: FAIL — table doesn't exist.

- [ ] **Step 3: Add DDL to schema.go**

In `internal/db/schema.go`, inside the `migrate()` function, after the last existing `db.Exec(...)` call (currently around line 300), add:

```go
// Prometheus integration
db.Exec(`CREATE TABLE IF NOT EXISTS prometheus_sources (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    base_url           TEXT NOT NULL,
    timeout_seconds    INTEGER NOT NULL DEFAULT 30,
    auth_type          TEXT NOT NULL DEFAULT 'none',
    username           TEXT NOT NULL DEFAULT '',
    encrypted_password TEXT NOT NULL DEFAULT '',
    encrypted_token    TEXT NOT NULL DEFAULT '',
    skip_tls_verify    INTEGER NOT NULL DEFAULT 0,
    created_at         DATETIME NOT NULL,
    updated_at         DATETIME NOT NULL
)`)
db.Exec(`CREATE TABLE IF NOT EXISTS prometheus_bindings (
    id          TEXT PRIMARY KEY,
    source_id   TEXT NOT NULL REFERENCES prometheus_sources(id) ON DELETE CASCADE,
    scope_type  TEXT NOT NULL CHECK(scope_type IN ('topology_layer','host')),
    topology_id TEXT REFERENCES topologies(id) ON DELETE CASCADE,
    layer       TEXT,
    host_id     TEXT REFERENCES hosts(id) ON DELETE CASCADE,
    created_at  DATETIME NOT NULL
)`)
db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_pb_topology_layer
    ON prometheus_bindings(topology_id, layer)
    WHERE scope_type = 'topology_layer'`)
db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_pb_host
    ON prometheus_bindings(host_id)
    WHERE scope_type = 'host'`)
```

- [ ] **Step 4: Run test, expect pass**

```bash
go test ./internal/db/ -run TestPrometheusTables -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/schema.go internal/db/schema_test.go
git commit -m "feat(db): add prometheus_sources and prometheus_bindings tables"
```

---

## Task 2: Go Models

**Files:**
- Create: `internal/models/prometheus.go`

- [ ] **Step 1: Write the file**

```go
package models

import "time"

type PrometheusAuthType string

const (
    PrometheusAuthNone   PrometheusAuthType = "none"
    PrometheusAuthBasic  PrometheusAuthType = "basic"
    PrometheusAuthBearer PrometheusAuthType = "bearer"
)

type PrometheusSource struct {
    ID                string             `json:"id"`
    Name              string             `json:"name"`
    BaseURL           string             `json:"base_url"`
    TimeoutSeconds    int                `json:"timeout_seconds"`
    AuthType          PrometheusAuthType `json:"auth_type"`
    Username          string             `json:"username,omitempty"`
    EncryptedPassword string             `json:"-"`
    EncryptedToken    string             `json:"-"`
    SkipTLSVerify     bool               `json:"skip_tls_verify"`
    CreatedAt         time.Time          `json:"created_at"`
    UpdatedAt         time.Time          `json:"updated_at"`
}

type PrometheusScopeType string

const (
    ScopeTopologyLayer PrometheusScopeType = "topology_layer"
    ScopeHost          PrometheusScopeType = "host"
)

type PrometheusBinding struct {
    ID         string              `json:"id"`
    SourceID   string              `json:"source_id"`
    ScopeType  PrometheusScopeType `json:"scope_type"`
    TopologyID string              `json:"topology_id,omitempty"`
    Layer      string              `json:"layer,omitempty"`
    HostID     string              `json:"host_id,omitempty"`
    CreatedAt  time.Time           `json:"created_at"`
}

// Request / response types

type AddPrometheusSourceRequest struct {
    Name           string             `json:"name"`
    BaseURL        string             `json:"base_url"`
    TimeoutSeconds int                `json:"timeout_seconds"`
    AuthType       PrometheusAuthType `json:"auth_type"`
    Username       string             `json:"username"`
    Password       string             `json:"password"`
    Token          string             `json:"token"`
    SkipTLSVerify  bool               `json:"skip_tls_verify"`
}

type UpdatePrometheusSourceRequest struct {
    Name           *string             `json:"name"`
    BaseURL        *string             `json:"base_url"`
    TimeoutSeconds *int                `json:"timeout_seconds"`
    AuthType       *PrometheusAuthType `json:"auth_type"`
    Username       *string             `json:"username"`
    Password       *string             `json:"password"`
    Token          *string             `json:"token"`
    SkipTLSVerify  *bool               `json:"skip_tls_verify"`
}

type AddPrometheusBindingRequest struct {
    SourceID   string              `json:"source_id"`
    ScopeType  PrometheusScopeType `json:"scope_type"`
    TopologyID string              `json:"topology_id"`
    Layer      string              `json:"layer"`
    HostID     string              `json:"host_id"`
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/models/
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/prometheus.go
git commit -m "feat(models): add PrometheusSource, PrometheusBinding types"
```

---

## Task 3: Prometheus Store

**Files:**
- Create: `internal/store/prometheus_store.go`
- Create: `internal/store/prometheus_store_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/store/prometheus_store_test.go
package store_test

import (
    "database/sql"
    "testing"

    _ "github.com/mattn/go-sqlite3"
    "github.com/spiderai/spider/internal/crypto"
    "github.com/spiderai/spider/internal/db"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

func newTestDB(t *testing.T) *sql.DB {
    t.Helper()
    database, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
    if err != nil {
        t.Fatal(err)
    }
    // need topologies and hosts tables for FK
    if err := db.Migrate(database); err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { database.Close() })
    return database
}

func TestPrometheusSourceCRUD(t *testing.T) {
    database := newTestDB(t)
    cm, _ := crypto.NewManager([]byte("0123456789abcdef0123456789abcdef"))
    s := store.NewPrometheusSourceStore(database, cm)

    // Add
    src, err := s.Add(&models.AddPrometheusSourceRequest{
        Name:    "test",
        BaseURL: "http://localhost:9090",
        AuthType: models.PrometheusAuthBasic,
        Username: "admin",
        Password: "secret",
    })
    if err != nil {
        t.Fatalf("Add: %v", err)
    }
    if src.ID == "" {
        t.Fatal("expected non-empty ID")
    }
    if src.Username != "admin" {
        t.Fatalf("expected username admin, got %s", src.Username)
    }
    if src.EncryptedPassword == "" {
        t.Fatal("expected encrypted password")
    }

    // List
    list, err := s.List()
    if err != nil {
        t.Fatalf("List: %v", err)
    }
    if len(list) != 1 {
        t.Fatalf("expected 1 source, got %d", len(list))
    }

    // GetByID
    got, err := s.GetByID(src.ID)
    if err != nil {
        t.Fatalf("GetByID: %v", err)
    }
    if got.Name != "test" {
        t.Fatalf("expected name test, got %s", got.Name)
    }

    // Update
    newName := "updated"
    updated, err := s.Update(src.ID, &models.UpdatePrometheusSourceRequest{Name: &newName})
    if err != nil {
        t.Fatalf("Update: %v", err)
    }
    if updated.Name != "updated" {
        t.Fatalf("expected updated name, got %s", updated.Name)
    }

    // Delete
    if err := s.Delete(src.ID); err != nil {
        t.Fatalf("Delete: %v", err)
    }
    _, err = s.GetByID(src.ID)
    if err != store.ErrNotFound {
        t.Fatalf("expected ErrNotFound after delete, got %v", err)
    }
}

func TestPrometheusBindingStore(t *testing.T) {
    database := newTestDB(t)
    cm, _ := crypto.NewManager([]byte("0123456789abcdef0123456789abcdef"))
    ss := store.NewPrometheusSourceStore(database, cm)
    bs := store.NewPrometheusBindingStore(database)

    src, _ := ss.Add(&models.AddPrometheusSourceRequest{
        Name: "prom", BaseURL: "http://p:9090", AuthType: models.PrometheusAuthNone,
    })

    // Add topology_layer binding
    b, err := bs.Add(&models.AddPrometheusBindingRequest{
        SourceID: src.ID, ScopeType: models.ScopeTopologyLayer,
        TopologyID: "t1", Layer: "server",
    })
    if err != nil {
        t.Fatalf("Add topology_layer binding: %v", err)
    }
    if b.ID == "" {
        t.Fatal("expected ID")
    }

    // Duplicate (topology_id, layer) must fail
    _, err = bs.Add(&models.AddPrometheusBindingRequest{
        SourceID: src.ID, ScopeType: models.ScopeTopologyLayer,
        TopologyID: "t1", Layer: "server",
    })
    if err == nil {
        t.Fatal("expected error for duplicate (topology_id, layer)")
    }

    // Delete binding
    if err := bs.Delete(b.ID); err != nil {
        t.Fatalf("Delete binding: %v", err)
    }
}
```

- [ ] **Step 2: Run tests, expect failure**

```bash
go test ./internal/store/ -run "TestPrometheusSource|TestPrometheusBinding" -v
```
Expected: FAIL — `store.NewPrometheusSourceStore` not defined.

- [ ] **Step 3: Implement prometheus_store.go**

```go
// internal/store/prometheus_store.go
package store

import (
    "database/sql"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/spiderai/spider/internal/crypto"
    "github.com/spiderai/spider/internal/models"
)

// --- PrometheusSourceStore ---

type PrometheusSourceStore struct {
    db     *sql.DB
    crypto *crypto.Manager
}

func NewPrometheusSourceStore(db *sql.DB, cm *crypto.Manager) *PrometheusSourceStore {
    return &PrometheusSourceStore{db: db, crypto: cm}
}

const promSourceCols = `id,name,base_url,timeout_seconds,auth_type,username,
    encrypted_password,encrypted_token,skip_tls_verify,created_at,updated_at`

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

const promBindingCols = `id,source_id,scope_type,
    COALESCE(topology_id,''),COALESCE(layer,''),COALESCE(host_id,''),created_at`

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

// FindSourceForHost implements the lookup priority: host binding first, then topology_layer binding.
func (s *PrometheusBindingStore) FindSourceIDForHost(hostID string) (string, error) {
    // 1. host-level binding
    var sourceID string
    err := s.db.QueryRow(`SELECT source_id FROM prometheus_bindings
        WHERE scope_type='host' AND host_id=? LIMIT 1`, hostID).Scan(&sourceID)
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
        return "", fmt.Errorf("该主机未配置 Prometheus 数据源")
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
    return &b, err
}
```

- [ ] **Step 4: Run tests, expect pass**

```bash
go test ./internal/store/ -run "TestPrometheusSource|TestPrometheusBinding" -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/prometheus_store.go internal/store/prometheus_store_test.go
git commit -m "feat(store): add PrometheusSourceStore and PrometheusBindingStore"
```

---

## Task 4: Internal Prometheus HTTP Client

**Files:**
- Create: `internal/prometheus/client.go`
- Create: `internal/prometheus/client_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/prometheus/client_test.go
package prometheus_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/spiderai/spider/internal/prometheus"
)

func makeServer(t *testing.T, handler http.Handler) *httptest.Server {
    t.Helper()
    srv := httptest.NewServer(handler)
    t.Cleanup(srv.Close)
    return srv
}

func TestQueryInstant(t *testing.T) {
    srv := makeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/api/v1/query" {
            http.Error(w, "not found", 404)
            return
        }
        json.NewEncoder(w).Encode(map[string]any{
            "status": "success",
            "data": map[string]any{
                "resultType": "vector",
                "result": []map[string]any{
                    {"metric": map[string]string{"__name__": "up"}, "value": []any{1716000000, "1"}},
                },
            },
        })
    }))

    c := prometheus.NewClient(srv.URL, "none", "", "", "", 30, false)
    result, err := c.QueryInstant(context.Background(), `up{job="node"}`, time.Now())
    if err != nil {
        t.Fatalf("QueryInstant: %v", err)
    }
    if result.ResultType != "vector" {
        t.Fatalf("expected vector, got %s", result.ResultType)
    }
    if len(result.Series) != 1 {
        t.Fatalf("expected 1 series, got %d", len(result.Series))
    }
}

func TestListMetricNames(t *testing.T) {
    srv := makeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "status": "success",
            "data":   []string{"node_cpu_seconds_total", "node_memory_MemTotal_bytes"},
        })
    }))

    c := prometheus.NewClient(srv.URL, "none", "", "", "", 30, false)
    names, err := c.ListMetricNames(context.Background(), `{instance="10.0.0.1:9100"}`)
    if err != nil {
        t.Fatalf("ListMetricNames: %v", err)
    }
    if len(names) != 2 {
        t.Fatalf("expected 2, got %d", len(names))
    }
}

func TestQueryRange_ValidationError(t *testing.T) {
    c := prometheus.NewClient("http://localhost:9090", "none", "", "", "", 30, false)
    // step causes >10000 points: 7d / 1s = 604800 points
    _, err := c.QueryRange(context.Background(), "up", "2026-01-01T00:00:00Z", "2026-01-08T00:00:00Z", "1s")
    if err == nil {
        t.Fatal("expected error for too many data points")
    }
}
```

- [ ] **Step 2: Run tests, expect failure**

```bash
go test ./internal/prometheus/ -v
```
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Create the package directory and client.go**

```bash
mkdir -p /Users/cw/fty.ai/spider.ai/internal/prometheus
```

```go
// internal/prometheus/client.go
package prometheus

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io"
    "math"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"
)

const (
    maxWindowDays  = 7
    maxDataPoints  = 10_000
    defaultTimeout = 30 * time.Second
)

type Sample struct {
    Timestamp float64 `json:"timestamp"`
    Value     string  `json:"value"`
}

type Series struct {
    Metric  map[string]string `json:"metric"`
    Samples []Sample          `json:"samples"`
    Latest  string            `json:"latest,omitempty"`
    Min     string            `json:"min,omitempty"`
    Max     string            `json:"max,omitempty"`
    Avg     string            `json:"avg,omitempty"`
}

type QueryResult struct {
    ResultType  string   `json:"result_type"`
    SeriesCount int      `json:"series_count"`
    Series      []Series `json:"series"`
}

type Client struct {
    baseURL  string
    authType string
    username string
    password string
    token    string
    http     *http.Client
}

func NewClient(baseURL, authType, username, password, token string, timeoutSeconds int, skipTLSVerify bool) *Client {
    timeout := time.Duration(timeoutSeconds) * time.Second
    if timeout == 0 {
        timeout = defaultTimeout
    }
    transport := &http.Transport{}
    if skipTLSVerify {
        transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
    }
    return &Client{
        baseURL:  strings.TrimRight(baseURL, "/"),
        authType: authType,
        username: username,
        password: password,
        token:    token,
        http:     &http.Client{Timeout: timeout, Transport: transport},
    }
}

func (c *Client) addAuth(req *http.Request) {
    switch c.authType {
    case "basic":
        req.SetBasicAuth(c.username, c.password)
    case "bearer":
        req.Header.Set("Authorization", "Bearer "+c.token)
    }
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
    u := c.baseURL + path
    if len(params) > 0 {
        u += "?" + params.Encode()
    }
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return nil, err
    }
    c.addAuth(req)
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("prometheus HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
    }
    return body, nil
}

// QueryInstant runs an instant query at ts.
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
    params := url.Values{
        "query": {query},
        "time":  {strconv.FormatFloat(float64(ts.Unix()), 'f', -1, 64)},
    }
    body, err := c.get(ctx, "/api/v1/query", params)
    if err != nil {
        return nil, err
    }
    return parseQueryResponse(body)
}

// QueryRange runs a range query. start/end are RFC3339 or Unix timestamp strings.
func (c *Client) QueryRange(ctx context.Context, query, start, end, step string) (*QueryResult, error) {
    startT, err := parseTime(start)
    if err != nil {
        return nil, fmt.Errorf("invalid start: %w", err)
    }
    endT, err := parseTime(end)
    if err != nil {
        return nil, fmt.Errorf("invalid end: %w", err)
    }
    if endT.Sub(startT) > maxWindowDays*24*time.Hour {
        return nil, fmt.Errorf("时间窗口超过 %d 天限制", maxWindowDays)
    }
    stepDur, err := parseDuration(step)
    if err != nil {
        // default: (end-start)/100, min 1s
        stepDur = time.Duration(math.Max(float64(endT.Sub(startT)/100), float64(time.Second)))
    }
    if stepDur < time.Second {
        stepDur = time.Second
    }
    points := int(endT.Sub(startT) / stepDur)
    if points > maxDataPoints {
        return nil, fmt.Errorf("预计数据点 %d 超过上限 %d，请增大 step", points, maxDataPoints)
    }
    params := url.Values{
        "query": {query},
        "start": {strconv.FormatFloat(float64(startT.Unix()), 'f', -1, 64)},
        "end":   {strconv.FormatFloat(float64(endT.Unix()), 'f', -1, 64)},
        "step":  {stepDur.String()},
    }
    body, err := c.get(ctx, "/api/v1/query_range", params)
    if err != nil {
        return nil, err
    }
    return parseQueryResponse(body)
}

// ListMetricNames calls the label values API to get all metric names matching selector.
func (c *Client) ListMetricNames(ctx context.Context, selector string) ([]string, error) {
    params := url.Values{"match[]": {selector}}
    body, err := c.get(ctx, "/api/v1/label/__name__/values", params)
    if err != nil {
        return nil, err
    }
    var resp struct {
        Status string   `json:"status"`
        Data   []string `json:"data"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, err
    }
    if resp.Status != "success" {
        return nil, fmt.Errorf("prometheus error: status=%s", resp.Status)
    }
    return resp.Data, nil
}

// TestConnection verifies the Prometheus instance is reachable by querying its build info.
func (c *Client) TestConnection(ctx context.Context) (latencyMs int64, err error) {
    start := time.Now()
    _, err = c.get(ctx, "/api/v1/metadata", url.Values{"limit": {"1"}})
    if err != nil {
        return 0, err
    }
    return time.Since(start).Milliseconds(), nil
}

// --- helpers ---

func parseTime(s string) (time.Time, error) {
    if t, err := time.Parse(time.RFC3339, s); err == nil {
        return t, nil
    }
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        return time.Time{}, fmt.Errorf("cannot parse time %q", s)
    }
    return time.Unix(int64(f), 0), nil
}

func parseDuration(s string) (time.Duration, error) {
    if d, err := time.ParseDuration(s); err == nil {
        return d, nil
    }
    // Prometheus duration like "1m30s" is already Go format; try stripping trailing 'w'/'d'
    if strings.HasSuffix(s, "d") {
        n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
        if err == nil {
            return time.Duration(n) * 24 * time.Hour, nil
        }
    }
    return 0, fmt.Errorf("cannot parse duration %q", s)
}

type promResponse struct {
    Status string `json:"status"`
    Data   struct {
        ResultType string            `json:"resultType"`
        Result     []json.RawMessage `json:"result"`
    } `json:"data"`
}

func parseQueryResponse(body []byte) (*QueryResult, error) {
    var resp promResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, err
    }
    if resp.Status != "success" {
        return nil, fmt.Errorf("prometheus error")
    }

    out := &QueryResult{
        ResultType: resp.Data.ResultType,
    }
    const maxSamples = 20

    for _, raw := range resp.Data.Result {
        switch resp.Data.ResultType {
        case "vector":
            var item struct {
                Metric map[string]string `json:"metric"`
                Value  [2]json.RawMessage `json:"value"`
            }
            if err := json.Unmarshal(raw, &item); err != nil {
                continue
            }
            var val string
            json.Unmarshal(item.Value[1], &val)
            out.Series = append(out.Series, Series{
                Metric:  item.Metric,
                Samples: []Sample{{Value: val}},
                Latest:  val,
                Min: val, Max: val, Avg: val,
            })
        case "matrix":
            var item struct {
                Metric map[string]string `json:"metric"`
                Values [][2]json.RawMessage `json:"values"`
            }
            if err := json.Unmarshal(raw, &item); err != nil {
                continue
            }
            series := Series{Metric: item.Metric}
            var sum float64
            min, max := math.MaxFloat64, -math.MaxFloat64
            for i, v := range item.Values {
                var ts float64
                var val string
                json.Unmarshal(v[0], &ts)
                json.Unmarshal(v[1], &val)
                if i < maxSamples {
                    series.Samples = append(series.Samples, Sample{Timestamp: ts, Value: val})
                }
                if f, err := strconv.ParseFloat(val, 64); err == nil {
                    sum += f
                    if f < min { min = f }
                    if f > max { max = f }
                }
            }
            n := len(item.Values)
            if n > 0 {
                // last sample as latest
                var lastVal string
                json.Unmarshal(item.Values[n-1][1], &lastVal)
                series.Latest = lastVal
                series.Min = strconv.FormatFloat(min, 'f', 4, 64)
                series.Max = strconv.FormatFloat(max, 'f', 4, 64)
                series.Avg = strconv.FormatFloat(sum/float64(n), 'f', 4, 64)
            }
            out.Series = append(out.Series, series)
        }
    }
    out.SeriesCount = len(out.Series)
    return out, nil
}
```

- [ ] **Step 4: Run tests, expect pass**

```bash
go test ./internal/prometheus/ -v
```
Expected: PASS.

- [ ] **Step 5: Compile check**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/prometheus/
git commit -m "feat(prometheus): add internal HTTP client (QueryInstant, QueryRange, ListMetricNames)"
```

---

## Task 5: REST API Handlers

**Files:**
- Create: `internal/api/prometheus_handlers.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Create prometheus_handlers.go**

```go
// internal/api/prometheus_handlers.go
package api

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"
    "time"

    "github.com/spiderai/spider/internal/models"
    mcppkg "github.com/spiderai/spider/internal/mcp"
    promclient "github.com/spiderai/spider/internal/prometheus"
)

// --- Sources ---

func listPrometheusSources(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    list, err := app.PrometheusSourceStore.List()
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    writeJSON(w, list)
}

func addPrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    var req models.AddPrometheusSourceRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    if strings.TrimSpace(req.Name) == "" {
        jsonError(w, "name required", http.StatusBadRequest)
        return
    }
    if strings.TrimSpace(req.BaseURL) == "" {
        jsonError(w, "base_url required", http.StatusBadRequest)
        return
    }
    src, err := app.PrometheusSourceStore.Add(&req)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
    writeJSON(w, src)
}

func getPrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    src, err := app.PrometheusSourceStore.GetByID(id)
    if err != nil {
        jsonError(w, "not found", http.StatusNotFound)
        return
    }
    writeJSON(w, src)
}

func updatePrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    var req models.UpdatePrometheusSourceRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    updated, err := app.PrometheusSourceStore.Update(id, &req)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    writeJSON(w, updated)
}

func deletePrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    if err := app.PrometheusSourceStore.Delete(id); err != nil {
        jsonError(w, "not found", http.StatusNotFound)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func testPrometheusConnection(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    src, err := app.PrometheusSourceStore.GetByID(id)
    if err != nil {
        jsonError(w, "not found", http.StatusNotFound)
        return
    }
    pwd, tok, err := app.PrometheusSourceStore.DecryptCredentials(src)
    if err != nil {
        jsonError(w, "decrypt error", http.StatusInternalServerError)
        return
    }
    c := promclient.NewClient(src.BaseURL, string(src.AuthType), src.Username, pwd, tok, src.TimeoutSeconds, src.SkipTLSVerify)
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()
    latency, err := c.TestConnection(ctx)
    if err != nil {
        writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
        return
    }
    writeJSON(w, map[string]any{"ok": true, "latency_ms": latency})
}

// --- Bindings ---

func listPrometheusBindings(app *mcppkg.App, w http.ResponseWriter, r *http.Request, sourceID string) {
    list, err := app.PrometheusBindingStore.ListBySource(sourceID)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    writeJSON(w, list)
}

func addPrometheusBinding(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
    var req models.AddPrometheusBindingRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    b, err := app.PrometheusBindingStore.Add(&req)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
    writeJSON(w, b)
}

func deletePrometheusBinding(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
    if err := app.PrometheusBindingStore.Delete(id); err != nil {
        jsonError(w, "not found", http.StatusNotFound)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Register routes in handler.go**

In `internal/api/handler.go`, after the last existing `mux.HandleFunc(...)` block (before the `return mux` statement), add:

```go
// Prometheus Data Sources
mux.HandleFunc("/api/v1/prometheus/sources", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        listPrometheusSources(app, w, r)
    case http.MethodPost:
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            addPrometheusSource(app, w, r)
        })).ServeHTTP(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
mux.HandleFunc("/api/v1/prometheus/sources/", func(w http.ResponseWriter, r *http.Request) {
    rest := r.URL.Path[len("/api/v1/prometheus/sources/"):]
    id := rest
    sub := ""
    if idx := indexOf(rest, '/'); idx >= 0 {
        id = rest[:idx]
        sub = rest[idx+1:]
    }
    switch sub {
    case "":
        switch r.Method {
        case http.MethodGet:
            getPrometheusSource(app, w, r, id)
        case http.MethodPut:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                updatePrometheusSource(app, w, r, id)
            })).ServeHTTP(w, r)
        case http.MethodDelete:
            operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                deletePrometheusSource(app, w, r, id)
            })).ServeHTTP(w, r)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    case "test":
        testPrometheusConnection(app, w, r, id)
    case "bindings":
        switch r.Method {
        case http.MethodGet:
            listPrometheusBindings(app, w, r, id)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    default:
        http.Error(w, "not found", http.StatusNotFound)
    }
})
mux.HandleFunc("/api/v1/prometheus/bindings", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            addPrometheusBinding(app, w, r)
        })).ServeHTTP(w, r)
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
mux.HandleFunc("/api/v1/prometheus/bindings/", func(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Path[len("/api/v1/prometheus/bindings/"):]
    if r.Method == http.MethodDelete {
        operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            deletePrometheusBinding(app, w, r, id)
        })).ServeHTTP(w, r)
        return
    }
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
```

- [ ] **Step 3: Compile check**

```bash
go build ./internal/api/
```
Expected: no errors (will fail if App struct doesn't have the new fields yet — that's fine, fix in Task 6).

- [ ] **Step 4: Commit**

```bash
git add internal/api/prometheus_handlers.go internal/api/handler.go
git commit -m "feat(api): add Prometheus source/binding REST endpoints"
```

---

## Task 6: Wire Up Stores in App + main

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Add fields to App struct in server.go**

In `internal/mcp/server.go`, in the `App` struct, add after `AccessFaceStore`:

```go
PrometheusSourceStore  *store.PrometheusSourceStore
PrometheusBindingStore *store.PrometheusBindingStore
```

- [ ] **Step 2: Wire stores in main.go**

In `cmd/spider/main.go`, in the `serve` command handler after `app.AccessFaceStore = store.NewAccessFaceStore(...)`, add:

```go
app.PrometheusSourceStore = store.NewPrometheusSourceStore(database, cm)
app.PrometheusBindingStore = store.NewPrometheusBindingStore(database)
```

- [ ] **Step 3: Compile check**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(app): wire PrometheusSourceStore and PrometheusBindingStore"
```

---

## Task 7: Agent Tools — ListMetrics + QueryMetrics

**Files:**
- Create: `internal/agent/tools_prometheus.go`
- Create: `internal/agent/tools_prometheus_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/agent/tools_prometheus_test.go
package agent_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/spiderai/spider/internal/agent"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

// mockPromBinding returns sourceID for any hostID lookup.
type mockBindingStore struct{ sourceID string }

func (m *mockBindingStore) FindSourceIDForHost(hostID string) (string, error) {
    return m.sourceID, nil
}

// mockSourceStore returns a fixed source.
type mockSourceStore struct{ src *models.PrometheusSource }

func (m *mockSourceStore) GetByID(id string) (*models.PrometheusSource, error) {
    return m.src, nil
}
func (m *mockSourceStore) DecryptCredentials(src *models.PrometheusSource) (string, string, error) {
    return "", "", nil
}

func TestListMetricsTool_Execute(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "status": "success",
            "data":   []string{"node_cpu_seconds_total", "node_load1"},
        })
    }))
    defer srv.Close()

    src := &models.PrometheusSource{ID: "s1", BaseURL: srv.URL, AuthType: "none", TimeoutSeconds: 30}
    tool := agent.NewListMetricsTool(
        &mockSourceStore{src: src},
        &mockBindingStore{sourceID: "s1"},
        &store.HostStore{}, // nil-safe for test
    )

    result, err := tool.Execute(context.Background(), map[string]any{
        "host_id": "h1",
    })
    if err != nil {
        t.Fatalf("Execute: %v", err)
    }
    s, ok := result.(string)
    if !ok {
        t.Fatalf("expected string result")
    }
    if !contains(s, "node_cpu_seconds_total") {
        t.Fatalf("expected metric name in result, got: %s", s)
    }
}

func contains(s, sub string) bool {
    return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}
func containsHelper(s, sub string) bool {
    for i := 0; i <= len(s)-len(sub); i++ {
        if s[i:i+len(sub)] == sub {
            return true
        }
    }
    return false
}
```

- [ ] **Step 2: Run test, expect failure**

```bash
go test ./internal/agent/ -run TestListMetricsTool -v
```
Expected: FAIL — `agent.NewListMetricsTool` undefined.

- [ ] **Step 3: Create tools_prometheus.go**

```go
// internal/agent/tools_prometheus.go
package agent

import (
    "context"
    "fmt"
    "strings"

    "github.com/spiderai/spider/internal/models"
    promclient "github.com/spiderai/spider/internal/prometheus"
    "github.com/spiderai/spider/internal/store"
)

// sourceStorer is the minimal interface needed by prometheus tools.
type sourceStorer interface {
    GetByID(id string) (*models.PrometheusSource, error)
    DecryptCredentials(src *models.PrometheusSource) (password, token string, err error)
}

type bindingStorer interface {
    FindSourceIDForHost(hostID string) (string, error)
}

func resolveClient(ss sourceStorer, bs bindingStorer, hostID string) (*promclient.Client, error) {
    sourceID, err := bs.FindSourceIDForHost(hostID)
    if err != nil {
        return nil, err
    }
    src, err := ss.GetByID(sourceID)
    if err != nil {
        return nil, fmt.Errorf("获取 Prometheus 数据源: %w", err)
    }
    pwd, tok, err := ss.DecryptCredentials(src)
    if err != nil {
        return nil, fmt.Errorf("解密凭据: %w", err)
    }
    return promclient.NewClient(src.BaseURL, string(src.AuthType), src.Username, pwd, tok, src.TimeoutSeconds, src.SkipTLSVerify), nil
}

// ---- ListMetricsTool ----

type ListMetricsTool struct {
    sources  sourceStorer
    bindings bindingStorer
    hosts    *store.HostStore
}

func NewListMetricsTool(ss sourceStorer, bs bindingStorer, hosts *store.HostStore) *ListMetricsTool {
    return &ListMetricsTool{sources: ss, bindings: bs, hosts: hosts}
}

func (t *ListMetricsTool) Name() string        { return "ListMetrics" }
func (t *ListMetricsTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *ListMetricsTool) IsConcurrencySafe(_ map[string]any) bool { return true }

func (t *ListMetricsTool) Description() string {
    return "List all Prometheus metric names available for a host. Read-only. Use freely in Explore phase before QueryMetrics."
}

func (t *ListMetricsTool) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "host_id": map[string]any{
                "type":        "string",
                "description": "Target host ID",
            },
            "filter": map[string]any{
                "type":        "string",
                "description": "Optional metric name prefix filter, e.g. 'node_cpu'",
            },
        },
        "required": []string{"host_id"},
    }
}

const listMetricsPrompt = `## ListMetrics

**When to use:** Before QueryMetrics when unsure of exact metric names. Call once per host per metric domain.

**When NOT to use:** When you already know the metric name.

**Rules:**
- Use the filter parameter to narrow results (e.g. "node_cpu" instead of no filter)
- Metric names are stable — cache them mentally within a conversation

<example>
User: Show CPU usage for host h1.
Assistant: ListMetrics(host_id="h1", filter="node_cpu") → QueryMetrics(host_id="h1", query="node_cpu_seconds_total{instance=\"IP:9100\",mode=\"idle\"}")
</example>`

func (t *ListMetricsTool) SystemPromptSection() string { return listMetricsPrompt }

func (t *ListMetricsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
    hostID, _ := input["host_id"].(string)
    if hostID == "" {
        return nil, fmt.Errorf("host_id required")
    }
    filter, _ := input["filter"].(string)

    // get host IP for selector
    var instanceIP string
    if t.hosts != nil {
        h, err := t.hosts.GetByID(hostID)
        if err == nil {
            instanceIP = h.IP
        }
    }

    client, err := resolveClient(t.sources, t.bindings, hostID)
    if err != nil {
        return nil, err
    }

    selector := "{}"
    if instanceIP != "" {
        selector = fmt.Sprintf(`{instance=~"%s:.*"}`, instanceIP)
    }

    names, err := client.ListMetricNames(ctx, selector)
    if err != nil {
        return nil, fmt.Errorf("ListMetricNames: %w", err)
    }

    if filter != "" {
        var filtered []string
        for _, n := range names {
            if strings.HasPrefix(n, filter) {
                filtered = append(filtered, n)
            }
        }
        names = filtered
    }

    return fmt.Sprintf("找到 %d 个指标:\n%s", len(names), strings.Join(names, "\n")), nil
}

// ---- QueryMetricsTool ----

type QueryMetricsTool struct {
    sources  sourceStorer
    bindings bindingStorer
}

func NewQueryMetricsTool(ss sourceStorer, bs bindingStorer) *QueryMetricsTool {
    return &QueryMetricsTool{sources: ss, bindings: bs}
}

func (t *QueryMetricsTool) Name() string        { return "QueryMetrics" }
func (t *QueryMetricsTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *QueryMetricsTool) IsConcurrencySafe(_ map[string]any) bool { return true }

func (t *QueryMetricsTool) Description() string {
    return "Execute a PromQL query against the Prometheus instance bound to a host. Supports instant and range queries. Read-only."
}

func (t *QueryMetricsTool) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "host_id": map[string]any{"type": "string", "description": "Target host ID"},
            "query":   map[string]any{"type": "string", "description": "PromQL expression"},
            "start":   map[string]any{"type": "string", "description": "RFC3339 or Unix timestamp (required with end)"},
            "end":     map[string]any{"type": "string", "description": "RFC3339 or Unix timestamp (required with start)"},
            "step":    map[string]any{"type": "string", "description": "Step size, e.g. '1m', '30s'. Default: (end-start)/100"},
            "raw":     map[string]any{"type": "boolean", "description": "Return raw Prometheus JSON. Default false."},
        },
        "required": []string{"host_id", "query"},
    }
}

const queryMetricsPrompt = `## QueryMetrics

**When to use:** To query host metrics from Prometheus. The tool auto-resolves the data source from the host binding.

**When NOT to use:** When you need to discover available metric names first — use ListMetrics before this.

**Rules:**
- start and end must be provided together or both omitted
- Omit both → instant query (current state)
- Provide both → range query (trend analysis)
- Construct PromQL with host IP label: node_cpu_seconds_total{instance="IP:9100"}
- Avoid bare queries without label filters on large clusters

<example>
Instant: QueryMetrics(host_id="h1", query="node_memory_MemAvailable_bytes{instance=\"10.0.0.1:9100\"}")
Range:   QueryMetrics(host_id="h1", query="rate(node_cpu_seconds_total{instance=\"10.0.0.1:9100\",mode=\"idle\"}[5m])", start="2026-05-24T00:00:00Z", end="2026-05-24T01:00:00Z", step="1m")
</example>`

func (t *QueryMetricsTool) SystemPromptSection() string { return queryMetricsPrompt }

func (t *QueryMetricsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
    hostID, _ := input["host_id"].(string)
    query, _ := input["query"].(string)
    if hostID == "" || query == "" {
        return nil, fmt.Errorf("host_id and query required")
    }

    start, _ := input["start"].(string)
    end, _ := input["end"].(string)
    step, _ := input["step"].(string)
    raw, _ := input["raw"].(bool)

    if (start == "") != (end == "") {
        return nil, fmt.Errorf("start 和 end 必须同时提供或同时省略")
    }

    client, err := resolveClient(t.sources, t.bindings, hostID)
    if err != nil {
        return nil, err
    }

    var result *promclient.QueryResult
    if start == "" {
        result, err = client.QueryInstant(ctx, query, timeNow())
    } else {
        result, err = client.QueryRange(ctx, query, start, end, step)
    }
    if err != nil {
        return nil, err
    }

    if raw {
        return result, nil
    }
    return formatQueryResult(result), nil
}

func formatQueryResult(r *promclient.QueryResult) string {
    var sb strings.Builder
    fmt.Fprintf(&sb, "result_type: %s  series_count: %d\n\n", r.ResultType, r.SeriesCount)
    for _, s := range r.Series {
        fmt.Fprintf(&sb, "metric: %v\n", s.Metric)
        if s.Latest != "" {
            fmt.Fprintf(&sb, "  latest=%s  min=%s  max=%s  avg=%s\n", s.Latest, s.Min, s.Max, s.Avg)
        }
        for i, sample := range s.Samples {
            if i >= 20 {
                break
            }
            fmt.Fprintf(&sb, "  [%v] %s\n", sample.Timestamp, sample.Value)
        }
        sb.WriteString("\n")
    }
    return sb.String()
}
```

Add a small helper to `tools_prometheus.go` for testability:

```go
// at top of tools_prometheus.go, inside package agent
import "time"
var timeNow = time.Now  // allows test override
```

- [ ] **Step 4: Run tests, expect pass**

```bash
go test ./internal/agent/ -run TestListMetricsTool -v
```
Expected: PASS.

- [ ] **Step 5: Full compile**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_prometheus.go internal/agent/tools_prometheus_test.go
git commit -m "feat(agent): add ListMetrics and QueryMetrics tools"
```

---

## Task 8: Register Tools in Factory

**Files:**
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Add store fields to Factory struct**

In `internal/agent/factory.go`, in the `Factory` struct after `TaskStore`, add:

```go
PrometheusSourceStore  sourceStorer
PrometheusBindingStore bindingStorer
```

- [ ] **Step 2: Register tools in buildRegistryWithHosts**

In `internal/agent/factory.go`, in `buildRegistryWithHosts`, after `registry.Register(NewCheckConnectivityTool(...))`, add:

```go
if f.PrometheusSourceStore != nil && f.PrometheusBindingStore != nil {
    registry.Register(NewListMetricsTool(f.PrometheusSourceStore, f.PrometheusBindingStore, f.Hosts))
    registry.Register(NewQueryMetricsTool(f.PrometheusSourceStore, f.PrometheusBindingStore))
}
```

- [ ] **Step 3: Wire factory in main.go**

In `cmd/spider/main.go`, where the `Factory` struct is assigned/updated (after wiring stores), add:

```go
agentFactory.PrometheusSourceStore = app.PrometheusSourceStore
agentFactory.PrometheusBindingStore = app.PrometheusBindingStore
```

(Find the exact location with: `grep -n "agentFactory\." cmd/spider/main.go | head -20`)

- [ ] **Step 4: Compile check**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/factory.go cmd/spider/main.go
git commit -m "feat(agent): register ListMetrics and QueryMetrics in factory"
```

---

## Task 9: Frontend — API Client

**Files:**
- Create: `web/src/api/prometheus.ts`

- [ ] **Step 1: Create the API client**

```typescript
// web/src/api/prometheus.ts
import { authHeaders } from './auth'

export type PrometheusAuthType = 'none' | 'basic' | 'bearer'
export type PrometheusScopeType = 'topology_layer' | 'host'

export interface PrometheusSource {
  id: string
  name: string
  base_url: string
  timeout_seconds: number
  auth_type: PrometheusAuthType
  username?: string
  skip_tls_verify: boolean
  created_at: string
  updated_at: string
}

export interface PrometheusBinding {
  id: string
  source_id: string
  scope_type: PrometheusScopeType
  topology_id?: string
  layer?: string
  host_id?: string
  created_at: string
}

export interface AddPrometheusSourceRequest {
  name: string
  base_url: string
  timeout_seconds?: number
  auth_type: PrometheusAuthType
  username?: string
  password?: string
  token?: string
  skip_tls_verify?: boolean
}

export interface UpdatePrometheusSourceRequest {
  name?: string
  base_url?: string
  timeout_seconds?: number
  auth_type?: PrometheusAuthType
  username?: string
  password?: string
  token?: string
  skip_tls_verify?: boolean
}

export async function listPrometheusSources(): Promise<PrometheusSource[]> {
  const res = await fetch('/api/v1/prometheus/sources', { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function addPrometheusSource(req: AddPrometheusSourceRequest): Promise<PrometheusSource> {
  const res = await fetch('/api/v1/prometheus/sources', {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updatePrometheusSource(id: string, req: UpdatePrometheusSourceRequest): Promise<PrometheusSource> {
  const res = await fetch(`/api/v1/prometheus/sources/${id}`, {
    method: 'PUT',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deletePrometheusSource(id: string): Promise<void> {
  const res = await fetch(`/api/v1/prometheus/sources/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function testPrometheusConnection(id: string): Promise<{ ok: boolean; latency_ms?: number; error?: string }> {
  const res = await fetch(`/api/v1/prometheus/sources/${id}/test`, { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function addPrometheusBinding(req: {
  source_id: string
  scope_type: PrometheusScopeType
  topology_id?: string
  layer?: string
  host_id?: string
}): Promise<PrometheusBinding> {
  const res = await fetch('/api/v1/prometheus/bindings', {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deletePrometheusBinding(id: string): Promise<void> {
  const res = await fetch(`/api/v1/prometheus/bindings/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
}
```

- [ ] **Step 2: Type check**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit
```
Expected: no errors in prometheus.ts.

- [ ] **Step 3: Commit**

```bash
git add web/src/api/prometheus.ts
git commit -m "feat(frontend): add Prometheus API client"
```

---

## Task 10: Frontend — Settings View + Prometheus Panel

**Files:**
- Create: `web/src/views/SettingsView.vue`
- Create: `web/src/components/PrometheusDataSourcesPanel.vue`
- Modify: `web/src/main.ts`
- Modify: `web/src/App.vue` (add nav item)

- [ ] **Step 1: Create SettingsView.vue**

```vue
<!-- web/src/views/SettingsView.vue -->
<template>
  <div class="settings-layout">
    <nav class="settings-sidebar">
      <div class="sidebar-section">系统设置</div>
      <router-link class="sidebar-item" to="/settings/prometheus">Data Sources · Prometheus</router-link>
    </nav>
    <div class="settings-content">
      <router-view />
    </div>
  </div>
</template>

<style scoped>
.settings-layout {
  display: flex;
  height: 100%;
  min-height: 0;
}
.settings-sidebar {
  width: 220px;
  background: var(--bg-secondary, #181b1f);
  border-right: 1px solid var(--border, #2c2f36);
  padding: 16px 0;
  flex-shrink: 0;
}
.sidebar-section {
  padding: 6px 16px;
  font-size: 11px;
  font-weight: 600;
  color: #6c7280;
  text-transform: uppercase;
  letter-spacing: .06em;
  margin-top: 12px;
}
.sidebar-item {
  display: block;
  padding: 7px 16px;
  color: #9ca3af;
  font-size: 13px;
  border-left: 2px solid transparent;
  text-decoration: none;
}
.sidebar-item:hover { background: #1f2228; color: #d9d9d9; }
.sidebar-item.router-link-active {
  background: #1a2035;
  color: #5794f2;
  border-left-color: #5794f2;
  font-weight: 500;
}
.settings-content {
  flex: 1;
  overflow-y: auto;
  padding: 32px 40px;
}
</style>
```

- [ ] **Step 2: Create PrometheusDataSourcesPanel.vue**

This is a long component. Create it at `web/src/components/PrometheusDataSourcesPanel.vue`:

```vue
<!-- web/src/components/PrometheusDataSourcesPanel.vue -->
<template>
  <div class="prom-page">
    <div class="page-title">Data Sources — Prometheus</div>
    <div class="page-subtitle">管理 Prometheus 监控数据源，在拓扑页面或主机页面绑定到具体作用域</div>

    <!-- Source list -->
    <div class="ds-list">
      <div class="ds-list-header">
        <span class="ds-list-title">已配置数据源</span>
        <button class="btn-add" @click="openNew">+ 新增数据源</button>
      </div>
      <div v-if="loading" class="ds-empty">加载中...</div>
      <div v-else-if="sources.length === 0" class="ds-empty">暂无数据源</div>
      <div
        v-for="src in sources"
        :key="src.id"
        class="ds-row"
        :class="{ selected: drawerSourceId === src.id }"
        @click="openEdit(src)"
      >
        <div class="ds-icon">P</div>
        <div class="ds-info">
          <div class="ds-name">{{ src.name }}</div>
          <div class="ds-url">{{ src.base_url }} · {{ authLabel(src.auth_type) }}</div>
        </div>
        <div class="ds-chevron">›</div>
      </div>
    </div>

    <!-- Overlay + Drawer -->
    <template v-if="drawerOpen">
      <div class="overlay" @click="closeDrawer" />
      <div class="drawer">
        <div class="drawer-header">
          <div class="drawer-title">
            <div class="ds-icon">P</div>
            {{ isNew ? '新增数据源' : '编辑数据源' }}
          </div>
          <button class="drawer-close" @click="closeDrawer">×</button>
        </div>

        <div class="drawer-body">
          <!-- HTTP section -->
          <div class="form-section">
            <div class="form-section-title">HTTP</div>
            <div class="form-row">
              <label class="form-label">名称 <span class="req">*</span></label>
              <input v-model="form.name" class="form-input" placeholder="生产-业务A-服务层" />
            </div>
            <div class="form-row">
              <label class="form-label">URL <span class="req">*</span></label>
              <div>
                <input v-model="form.base_url" class="form-input" placeholder="http://prometheus:9090" />
                <div class="form-hint">Prometheus 实例地址</div>
              </div>
            </div>
            <div class="form-row">
              <label class="form-label">超时（秒）</label>
              <input v-model.number="form.timeout_seconds" class="form-input sm" type="number" min="1" />
            </div>
          </div>

          <!-- Auth section -->
          <div class="form-section">
            <div class="form-section-title">认证</div>
            <div class="form-row">
              <label class="form-label">认证方式</label>
              <select v-model="form.auth_type" class="form-select">
                <option value="none">无认证</option>
                <option value="basic">Basic Auth</option>
                <option value="bearer">Bearer Token</option>
              </select>
            </div>
            <div class="toggle-row">
              <div class="toggle" :class="{ on: form.skip_tls_verify }" @click="form.skip_tls_verify = !form.skip_tls_verify" />
              <span class="toggle-label">跳过 TLS 验证</span>
              <span class="toggle-hint">内网自签证书时启用</span>
            </div>
            <div v-if="form.auth_type === 'basic'" class="auth-detail">
              <div class="auth-detail-title">Basic Auth 详情</div>
              <div class="form-row">
                <label class="form-label">用户名</label>
                <input v-model="form.username" class="form-input" style="max-width:240px" />
              </div>
              <div class="form-row" style="margin-bottom:0">
                <label class="form-label">密码</label>
                <input v-model="form.password" class="form-input" type="password" placeholder="••••••••" style="max-width:240px" />
              </div>
            </div>
            <div v-if="form.auth_type === 'bearer'" class="auth-detail">
              <div class="auth-detail-title">Bearer Token</div>
              <div class="form-row" style="margin-bottom:0">
                <label class="form-label">Token</label>
                <input v-model="form.token" class="form-input" type="password" placeholder="••••••••" />
              </div>
            </div>
          </div>
        </div>

        <div class="drawer-footer">
          <button class="btn-save" :disabled="saving" @click="save">{{ saving ? '保存中...' : '保存' }}</button>
          <button class="btn-test" :disabled="testing || isNew" @click="testConn">测试连接</button>
          <span v-if="testResult" :class="testResult.ok ? 'test-ok' : 'test-err'">
            {{ testResult.ok ? `连接正常 · ${testResult.latency_ms}ms` : testResult.error }}
          </span>
          <button v-if="!isNew" class="btn-del" @click="confirmDelete">删除</button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import {
  listPrometheusSources, addPrometheusSource, updatePrometheusSource,
  deletePrometheusSource, testPrometheusConnection,
  type PrometheusSource, type PrometheusAuthType,
} from '../api/prometheus'

const sources = ref<PrometheusSource[]>([])
const loading = ref(false)
const drawerOpen = ref(false)
const drawerSourceId = ref<string | null>(null)
const isNew = ref(false)
const saving = ref(false)
const testing = ref(false)
const testResult = ref<{ ok: boolean; latency_ms?: number; error?: string } | null>(null)

const emptyForm = () => ({
  name: '',
  base_url: '',
  timeout_seconds: 30,
  auth_type: 'none' as PrometheusAuthType,
  username: '',
  password: '',
  token: '',
  skip_tls_verify: false,
})

const form = reactive(emptyForm())

async function load() {
  loading.value = true
  try {
    sources.value = await listPrometheusSources()
  } finally {
    loading.value = false
  }
}

onMounted(load)

function authLabel(t: PrometheusAuthType) {
  return t === 'none' ? 'No Auth' : t === 'basic' ? 'Basic Auth' : 'Bearer Token'
}

function openNew() {
  Object.assign(form, emptyForm())
  drawerSourceId.value = null
  isNew.value = true
  testResult.value = null
  drawerOpen.value = true
}

function openEdit(src: PrometheusSource) {
  Object.assign(form, {
    name: src.name,
    base_url: src.base_url,
    timeout_seconds: src.timeout_seconds,
    auth_type: src.auth_type,
    username: src.username ?? '',
    password: '',
    token: '',
    skip_tls_verify: src.skip_tls_verify,
  })
  drawerSourceId.value = src.id
  isNew.value = false
  testResult.value = null
  drawerOpen.value = true
}

function closeDrawer() {
  drawerOpen.value = false
  drawerSourceId.value = null
}

async function save() {
  if (!form.name.trim() || !form.base_url.trim()) return
  saving.value = true
  try {
    if (isNew.value) {
      await addPrometheusSource({
        name: form.name,
        base_url: form.base_url,
        timeout_seconds: form.timeout_seconds,
        auth_type: form.auth_type,
        username: form.username || undefined,
        password: form.password || undefined,
        token: form.token || undefined,
        skip_tls_verify: form.skip_tls_verify,
      })
    } else if (drawerSourceId.value) {
      await updatePrometheusSource(drawerSourceId.value, {
        name: form.name,
        base_url: form.base_url,
        timeout_seconds: form.timeout_seconds,
        auth_type: form.auth_type,
        username: form.username || undefined,
        password: form.password || undefined,
        token: form.token || undefined,
        skip_tls_verify: form.skip_tls_verify,
      })
    }
    await load()
    closeDrawer()
  } catch (e: any) {
    alert(e.message)
  } finally {
    saving.value = false
  }
}

async function testConn() {
  if (!drawerSourceId.value) return
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await testPrometheusConnection(drawerSourceId.value)
  } catch (e: any) {
    testResult.value = { ok: false, error: e.message }
  } finally {
    testing.value = false
  }
}

async function confirmDelete() {
  if (!drawerSourceId.value) return
  if (!confirm('确认删除该数据源？关联绑定将一并删除。')) return
  await deletePrometheusSource(drawerSourceId.value)
  await load()
  closeDrawer()
}
</script>

<style scoped>
.prom-page { max-width: 720px; }
.page-title { font-size: 22px; font-weight: 600; color: #f0f0f0; margin-bottom: 4px; }
.page-subtitle { font-size: 13px; color: #6c7280; margin-bottom: 28px; }

.ds-list { background: #181b1f; border: 1px solid #2c2f36; border-radius: 4px; max-width: 680px; }
.ds-list-header { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px; border-bottom: 1px solid #2c2f36; }
.ds-list-title { font-size: 13px; font-weight: 600; color: #9ca3af; text-transform: uppercase; letter-spacing: .05em; }
.ds-empty { padding: 20px 16px; color: #6c7280; font-size: 13px; }
.btn-add { background: #1f60c4; color: #fff; border: none; border-radius: 3px; padding: 6px 14px; font-size: 13px; cursor: pointer; font-weight: 500; }

.ds-row { display: flex; align-items: center; padding: 12px 16px; border-bottom: 1px solid #1e2128; gap: 12px; cursor: pointer; }
.ds-row:last-child { border-bottom: none; }
.ds-row:hover { background: #1d2029; }
.ds-row.selected { background: #1a2035; border-left: 2px solid #5794f2; padding-left: 14px; }

.ds-icon { width: 28px; height: 28px; background: #e6521a; border-radius: 3px; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 700; color: #fff; flex-shrink: 0; }
.ds-info { flex: 1; }
.ds-name { font-size: 14px; color: #d9d9d9; font-weight: 500; }
.ds-url { font-size: 12px; color: #6c7280; margin-top: 2px; }
.ds-chevron { color: #4b5563; font-size: 16px; }

/* Overlay */
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,.45); z-index: 100; }

/* Drawer */
.drawer {
  position: fixed; top: 48px; right: 0; bottom: 0; width: 480px;
  background: #181b1f; border-left: 1px solid #2c2f36;
  z-index: 101; display: flex; flex-direction: column;
  box-shadow: -8px 0 32px rgba(0,0,0,.4);
}
.drawer-header { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid #2c2f36; flex-shrink: 0; }
.drawer-title { font-size: 16px; font-weight: 600; color: #f0f0f0; display: flex; align-items: center; gap: 10px; }
.drawer-close { background: none; border: none; color: #6c7280; cursor: pointer; font-size: 20px; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; border-radius: 3px; }
.drawer-close:hover { background: #1f2228; color: #d9d9d9; }

.drawer-body { flex: 1; overflow-y: auto; padding: 24px 20px; }

.form-section { margin-bottom: 28px; }
.form-section-title { font-size: 11px; font-weight: 600; color: #6c7280; text-transform: uppercase; letter-spacing: .06em; margin-bottom: 14px; padding-bottom: 8px; border-bottom: 1px solid #1e2128; }
.form-row { margin-bottom: 14px; display: flex; flex-direction: column; gap: 5px; }
.form-label { font-size: 12px; color: #9ca3af; }
.req { color: #e05c5c; }
.form-hint { font-size: 11px; color: #4b5563; margin-top: 4px; }
.form-input { width: 100%; background: #111217; border: 1px solid #2c2f36; border-radius: 3px; padding: 7px 10px; color: #d9d9d9; font-size: 13px; }
.form-input:focus { outline: none; border-color: #5794f2; }
.form-input.sm { max-width: 100px; }
.form-select { background: #111217; border: 1px solid #2c2f36; border-radius: 3px; padding: 7px 10px; color: #d9d9d9; font-size: 13px; max-width: 220px; }

.toggle-row { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.toggle { width: 36px; height: 20px; background: #2c2f36; border-radius: 10px; position: relative; cursor: pointer; flex-shrink: 0; }
.toggle.on { background: #1f60c4; }
.toggle::after { content: ''; position: absolute; width: 14px; height: 14px; background: #fff; border-radius: 50%; top: 3px; left: 3px; transition: left .15s; }
.toggle.on::after { left: 19px; }
.toggle-label { font-size: 13px; color: #9ca3af; }
.toggle-hint { font-size: 11px; color: #4b5563; }

.auth-detail { background: #111217; border: 1px solid #2c2f36; border-radius: 3px; padding: 14px 16px; margin-top: 8px; }
.auth-detail-title { font-size: 11px; font-weight: 600; color: #6c7280; margin-bottom: 12px; text-transform: uppercase; letter-spacing: .05em; }

.drawer-footer { display: flex; gap: 10px; padding: 14px 20px; border-top: 1px solid #2c2f36; align-items: center; flex-shrink: 0; background: #181b1f; }
.btn-save { background: #1f60c4; color: #fff; border: none; border-radius: 3px; padding: 8px 20px; font-size: 14px; font-weight: 500; cursor: pointer; }
.btn-save:disabled { opacity: .6; cursor: not-allowed; }
.btn-test { background: transparent; color: #9ca3af; border: 1px solid #2c2f36; border-radius: 3px; padding: 8px 14px; font-size: 13px; cursor: pointer; }
.btn-test:disabled { opacity: .6; cursor: not-allowed; }
.btn-del { margin-left: auto; background: transparent; color: #e05c5c; border: 1px solid transparent; border-radius: 3px; padding: 8px 12px; font-size: 13px; cursor: pointer; }
.btn-del:hover { border-color: #e05c5c; }
.test-ok { font-size: 12px; color: #6ccf6c; }
.test-err { font-size: 12px; color: #e05c5c; }
</style>
```

- [ ] **Step 3: Add routes to main.ts**

In `web/src/main.ts`, add to the `routes` array:

```typescript
{
  path: '/settings',
  component: () => import('./views/SettingsView.vue'),
  children: [
    { path: '', redirect: '/settings/prometheus' },
    { path: 'prometheus', component: () => import('./components/PrometheusDataSourcesPanel.vue') },
  ],
},
```

- [ ] **Step 4: Add nav item to App.vue**

In `web/src/App.vue`, in the nav section (find the list of nav links like `/hosts`, `/topology`, etc.), add:

```html
<router-link to="/settings" class="nav-item">设置</router-link>
```

(Find the exact location with: `grep -n "router-link\|nav-item" web/src/App.vue | head -20`)

- [ ] **Step 5: Type check + build**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit && npm run build
```
Expected: no TypeScript errors, build succeeds.

- [ ] **Step 6: Start server and verify in browser**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-prom ./cmd/spider
/tmp/spider-prom serve --addr :8003 --data-dir ~/.spider/data &
```

Open browser to `http://localhost:8003/settings/prometheus` and verify:
- Sidebar shows "Data Sources · Prometheus" active
- Empty state "暂无数据源" shown
- "+ 新增数据源" button opens right-side drawer
- Drawer has HTTP and 认证 sections
- Auth type dropdown shows: 无认证 / Basic Auth / Bearer Token
- Selecting Basic Auth shows username + password fields
- Selecting Bearer Token shows token field
- 跳过 TLS 验证 toggle works

- [ ] **Step 7: Kill test server**

```bash
kill $(lsof -ti :8003) 2>/dev/null || true
```

- [ ] **Step 8: Commit**

```bash
git add web/src/views/SettingsView.vue web/src/components/PrometheusDataSourcesPanel.vue web/src/api/prometheus.ts web/src/main.ts web/src/App.vue
git commit -m "feat(frontend): add Settings > Data Sources > Prometheus page with drawer"
```

---

## Final Verification

- [ ] Run all Go tests:
  ```bash
  go test ./...
  ```
  Expected: PASS (zero failures).

- [ ] Build full binary:
  ```bash
  go build -a ./...
  ```
  Expected: no errors.

- [ ] Type check frontend:
  ```bash
  cd web && npx tsc --noEmit
  ```
  Expected: no errors.
