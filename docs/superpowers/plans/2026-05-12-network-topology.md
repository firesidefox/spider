# Network Topology Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add network topology feature — DB schema, REST API, Agent tool, and Vue UI with Cytoscape.js visualization.

**Architecture:** 4 new DB tables (topologies, topology_groups, topology_nodes, topology_edges) with CASCADE deletes. Store layer follows existing `*Store` pattern. API follows existing `mux.HandleFunc` pattern in `handler.go`. Frontend adds `/topology` route with a 3-panel layout (list / canvas / detail) using Cytoscape.js + dagre layout.

**Tech Stack:** Go (zerolog, gopkg.in/yaml.v3, github.com/google/uuid), Cytoscape.js + cytoscape-dagre (npm), Vue 3 Composition API.

---

## File Map

**Create:**
- `internal/models/topology.go` — Topology, TopologyGroup, TopologyNode, TopologyEdge structs + request types
- `internal/store/topology_store.go` — TopologyStore CRUD
- `internal/api/topology.go` — HTTP handlers for all topology routes
- `internal/agent/tools_topology.go` — GetTopologyTool
- `web/src/api/topology.ts` — API client types + fetch functions
- `web/src/views/TopologyView.vue` — 3-panel topology UI

**Modify:**
- `internal/db/schema.go` — add 4 topology tables in `migrate()`
- `internal/mcp/server.go` — add `TopologyStore *store.TopologyStore` to App struct
- `internal/api/handler.go` — register topology routes + wire TopologyStore
- `cmd/spider/main.go` — init TopologyStore and pass to App
- `internal/agent/factory.go` — pass TopologyStore to GetTopologyTool
- `web/src/main.ts` — add `/topology` route
- `web/src/App.vue` — add Topology nav link (if nav exists)

---

## Task 1: DB schema — 4 topology tables


**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add tables to migrate()**

In `internal/db/schema.go`, inside `migrate()` before the final `return nil`, add:

```go
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topologies (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    notes      TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
)`); err != nil {
    return err
}
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topology_groups (
    id          TEXT PRIMARY KEY,
    topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '#3b82f6',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL
)`); err != nil {
    return err
}
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topology_nodes (
    id          TEXT PRIMARY KEY,
    topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
    group_id    TEXT NOT NULL REFERENCES topology_groups(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT '',
    host_id     TEXT REFERENCES hosts(id) ON DELETE SET NULL,
    notes       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
)`); err != nil {
    return err
}
if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS topology_edges (
    id          TEXT PRIMARY KEY,
    topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
    from_node   TEXT NOT NULL REFERENCES topology_nodes(id) ON DELETE CASCADE,
    to_node     TEXT NOT NULL REFERENCES topology_nodes(id) ON DELETE CASCADE,
    created_at  DATETIME NOT NULL
)`); err != nil {
    return err
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add topology tables"
```

---

## Task 2: Models

**Files:**
- Create: `internal/models/topology.go`

- [ ] **Step 1: Create models file**

```go
package models

import "time"

type Topology struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Notes     string    `json:"notes"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type TopologyGroup struct {
    ID         string    `json:"id"`
    TopologyID string    `json:"topology_id"`
    Name       string    `json:"name"`
    Color      string    `json:"color"`
    SortOrder  int       `json:"sort_order"`
    CreatedAt  time.Time `json:"created_at"`
}

type TopologyNode struct {
    ID         string    `json:"id"`
    TopologyID string    `json:"topology_id"`
    GroupID    string    `json:"group_id"`
    Name       string    `json:"name"`
    Role       string    `json:"role"`
    HostID     string    `json:"host_id,omitempty"`
    Notes      string    `json:"notes"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
    // Joined fields (populated by GetFull)
    HostName string `json:"host_name,omitempty"`
    IP       string `json:"ip,omitempty"`
}

type TopologyEdge struct {
    ID         string    `json:"id"`
    TopologyID string    `json:"topology_id"`
    FromNode   string    `json:"from_node"`
    ToNode     string    `json:"to_node"`
    CreatedAt  time.Time `json:"created_at"`
}

type TopologyFull struct {
    Topology
    Groups []*TopologyGroup `json:"groups"`
    Nodes  []*TopologyNode  `json:"nodes"`
    Edges  []*TopologyEdge  `json:"edges"`
}
```

- [ ] **Step 2: Add request types (append to same file)**

```go
type CreateTopologyRequest struct {
    Name  string `json:"name"`
    Notes string `json:"notes"`
}

type UpdateTopologyRequest struct {
    Name  string `json:"name"`
    Notes string `json:"notes"`
}

type CreateGroupRequest struct {
    Name      string `json:"name"`
    Color     string `json:"color"`
    SortOrder int    `json:"sort_order"`
}

type UpdateGroupRequest struct {
    Name      string `json:"name"`
    Color     string `json:"color"`
    SortOrder int    `json:"sort_order"`
}

type CreateNodeRequest struct {
    GroupID string `json:"group_id"`
    Name    string `json:"name"`
    Role    string `json:"role"`
    HostID  string `json:"host_id"`
    Notes   string `json:"notes"`
}

type UpdateNodeRequest struct {
    GroupID string `json:"group_id"`
    Name    string `json:"name"`
    Role    string `json:"role"`
    HostID  string `json:"host_id"`
    Notes   string `json:"notes"`
}

type CreateEdgeRequest struct {
    FromNode string `json:"from_node"`
    ToNode   string `json:"to_node"`
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/models/topology.go
git commit -m "feat(models): add topology types"
```

---

## Task 3: TopologyStore

**Files:**
- Create: `internal/store/topology_store.go`

- [ ] **Step 1: Write failing test**

Create `internal/store/topology_store_test.go`:

```go
package store_test

import (
    "testing"

    "github.com/spiderai/spider/internal/db"
    "github.com/spiderai/spider/internal/models"
    "github.com/spiderai/spider/internal/store"
)

func newTestTopologyStore(t *testing.T) *store.TopologyStore {
    t.Helper()
    database, err := db.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { database.Close() })
    return store.NewTopologyStore(database)
}

func TestTopologyStoreCRUD(t *testing.T) {
    s := newTestTopologyStore(t)

    topo, err := s.Create(&models.CreateTopologyRequest{Name: "prod", Notes: "production"})
    if err != nil {
        t.Fatal(err)
    }
    if topo.Name != "prod" {
        t.Errorf("want name=prod, got %s", topo.Name)
    }

    list, err := s.List()
    if err != nil {
        t.Fatal(err)
    }
    if len(list) != 1 {
        t.Errorf("want 1 topology, got %d", len(list))
    }

    got, err := s.GetByID(topo.ID)
    if err != nil {
        t.Fatal(err)
    }
    if got.ID != topo.ID {
        t.Errorf("GetByID mismatch")
    }

    if err := s.Delete(topo.ID); err != nil {
        t.Fatal(err)
    }
    _, err = s.GetByID(topo.ID)
    if err == nil {
        t.Error("expected not found after delete")
    }
}

func TestTopologyStoreGetFull(t *testing.T) {
    s := newTestTopologyStore(t)

    topo, _ := s.Create(&models.CreateTopologyRequest{Name: "test"})
    grp, _ := s.CreateGroup(topo.ID, &models.CreateGroupRequest{Name: "fw", Color: "#ff0000"})
    node, _ := s.CreateNode(topo.ID, &models.CreateNodeRequest{GroupID: grp.ID, Name: "fw-01"})
    node2, _ := s.CreateNode(topo.ID, &models.CreateNodeRequest{GroupID: grp.ID, Name: "fw-02"})
    _, _ = s.CreateEdge(topo.ID, &models.CreateEdgeRequest{FromNode: node.ID, ToNode: node2.ID})

    full, err := s.GetFull(topo.ID)
    if err != nil {
        t.Fatal(err)
    }
    if len(full.Groups) != 1 {
        t.Errorf("want 1 group, got %d", len(full.Groups))
    }
    if len(full.Nodes) != 2 {
        t.Errorf("want 2 nodes, got %d", len(full.Nodes))
    }
    if len(full.Edges) != 1 {
        t.Errorf("want 1 edge, got %d", len(full.Edges))
    }
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/store/... -run TestTopologyStore -v
```

Expected: `FAIL — NewTopologyStore undefined`

- [ ] **Step 3: Create topology_store.go (part 1 — struct + List/Create/GetByID/Delete)**

Create `internal/store/topology_store.go`:

```go
package store

import (
    "database/sql"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/spiderai/spider/internal/models"
)

type TopologyStore struct {
    db *sql.DB
}

func NewTopologyStore(db *sql.DB) *TopologyStore {
    return &TopologyStore{db: db}
}

func (s *TopologyStore) List() ([]*models.Topology, error) {
    rows, err := s.db.Query(`SELECT id, name, notes, created_at, updated_at FROM topologies ORDER BY name`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var list []*models.Topology
    for rows.Next() {
        var t models.Topology
        if err := rows.Scan(&t.ID, &t.Name, &t.Notes, &t.CreatedAt, &t.UpdatedAt); err != nil {
            return nil, err
        }
        list = append(list, &t)
    }
    return list, nil
}

func (s *TopologyStore) Create(req *models.CreateTopologyRequest) (*models.Topology, error) {
    if req.Name == "" {
        return nil, fmt.Errorf("name is required")
    }
    now := time.Now().UTC()
    id := uuid.New().String()
    _, err := s.db.Exec(
        `INSERT INTO topologies (id, name, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
        id, req.Name, req.Notes, now, now,
    )
    if err != nil {
        return nil, fmt.Errorf("create topology: %w", err)
    }
    return s.GetByID(id)
}

func (s *TopologyStore) GetByID(id string) (*models.Topology, error) {
    var t models.Topology
    err := s.db.QueryRow(
        `SELECT id, name, notes, created_at, updated_at FROM topologies WHERE id = ?`, id,
    ).Scan(&t.ID, &t.Name, &t.Notes, &t.CreatedAt, &t.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &t, err
}

func (s *TopologyStore) Update(id string, req *models.UpdateTopologyRequest) (*models.Topology, error) {
    now := time.Now().UTC()
    _, err := s.db.Exec(
        `UPDATE topologies SET name = ?, notes = ?, updated_at = ? WHERE id = ?`,
        req.Name, req.Notes, now, id,
    )
    if err != nil {
        return nil, fmt.Errorf("update topology: %w", err)
    }
    return s.GetByID(id)
}

func (s *TopologyStore) Delete(id string) error {
    _, err := s.db.Exec(`DELETE FROM topologies WHERE id = ?`, id)
    return err
}
```

- [ ] **Step 4: Add group/node/edge methods (append to topology_store.go)**

```go
func (s *TopologyStore) ListGroups(topologyID string) ([]*models.TopologyGroup, error) {
    rows, err := s.db.Query(
        `SELECT id, topology_id, name, color, sort_order, created_at FROM topology_groups WHERE topology_id = ? ORDER BY sort_order, name`,
        topologyID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var list []*models.TopologyGroup
    for rows.Next() {
        var g models.TopologyGroup
        if err := rows.Scan(&g.ID, &g.TopologyID, &g.Name, &g.Color, &g.SortOrder, &g.CreatedAt); err != nil {
            return nil, err
        }
        list = append(list, &g)
    }
    return list, nil
}

func (s *TopologyStore) CreateGroup(topologyID string, req *models.CreateGroupRequest) (*models.TopologyGroup, error) {
    id := uuid.New().String()
    now := time.Now().UTC()
    color := req.Color
    if color == "" {
        color = "#3b82f6"
    }
    _, err := s.db.Exec(
        `INSERT INTO topology_groups (id, topology_id, name, color, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
        id, topologyID, req.Name, color, req.SortOrder, now,
    )
    if err != nil {
        return nil, fmt.Errorf("create group: %w", err)
    }
    var g models.TopologyGroup
    err = s.db.QueryRow(
        `SELECT id, topology_id, name, color, sort_order, created_at FROM topology_groups WHERE id = ?`, id,
    ).Scan(&g.ID, &g.TopologyID, &g.Name, &g.Color, &g.SortOrder, &g.CreatedAt)
    return &g, err
}

func (s *TopologyStore) UpdateGroup(id string, req *models.UpdateGroupRequest) (*models.TopologyGroup, error) {
    _, err := s.db.Exec(
        `UPDATE topology_groups SET name = ?, color = ?, sort_order = ? WHERE id = ?`,
        req.Name, req.Color, req.SortOrder, id,
    )
    if err != nil {
        return nil, fmt.Errorf("update group: %w", err)
    }
    var g models.TopologyGroup
    err = s.db.QueryRow(
        `SELECT id, topology_id, name, color, sort_order, created_at FROM topology_groups WHERE id = ?`, id,
    ).Scan(&g.ID, &g.TopologyID, &g.Name, &g.Color, &g.SortOrder, &g.CreatedAt)
    return &g, err
}

func (s *TopologyStore) DeleteGroup(id string) error {
    _, err := s.db.Exec(`DELETE FROM topology_groups WHERE id = ?`, id)
    return err
}
```

- [ ] **Step 5: Add node/edge methods (append to topology_store.go)**

```go
func (s *TopologyStore) ListNodes(topologyID string) ([]*models.TopologyNode, error) {
    rows, err := s.db.Query(
        `SELECT n.id, n.topology_id, n.group_id, n.name, n.role,
                COALESCE(n.host_id,''), n.notes, n.created_at, n.updated_at,
                COALESCE(h.name,''), COALESCE(h.ip,'')
         FROM topology_nodes n
         LEFT JOIN hosts h ON h.id = n.host_id
         WHERE n.topology_id = ? ORDER BY n.name`,
        topologyID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var list []*models.TopologyNode
    for rows.Next() {
        var n models.TopologyNode
        if err := rows.Scan(&n.ID, &n.TopologyID, &n.GroupID, &n.Name, &n.Role,
            &n.HostID, &n.Notes, &n.CreatedAt, &n.UpdatedAt, &n.HostName, &n.IP); err != nil {
            return nil, err
        }
        list = append(list, &n)
    }
    return list, nil
}

func (s *TopologyStore) CreateNode(topologyID string, req *models.CreateNodeRequest) (*models.TopologyNode, error) {
    id := uuid.New().String()
    now := time.Now().UTC()
    var hostID *string
    if req.HostID != "" {
        hostID = &req.HostID
    }
    _, err := s.db.Exec(
        `INSERT INTO topology_nodes (id, topology_id, group_id, name, role, host_id, notes, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        id, topologyID, req.GroupID, req.Name, req.Role, hostID, req.Notes, now, now,
    )
    if err != nil {
        return nil, fmt.Errorf("create node: %w", err)
    }
    nodes, err := s.ListNodes(topologyID)
    if err != nil {
        return nil, err
    }
    for _, n := range nodes {
        if n.ID == id {
            return n, nil
        }
    }
    return nil, ErrNotFound
}

func (s *TopologyStore) UpdateNode(id string, req *models.UpdateNodeRequest) (*models.TopologyNode, error) {
    now := time.Now().UTC()
    var hostID *string
    if req.HostID != "" {
        hostID = &req.HostID
    }
    _, err := s.db.Exec(
        `UPDATE topology_nodes SET group_id=?, name=?, role=?, host_id=?, notes=?, updated_at=? WHERE id=?`,
        req.GroupID, req.Name, req.Role, hostID, req.Notes, now, id,
    )
    if err != nil {
        return nil, fmt.Errorf("update node: %w", err)
    }
    var n models.TopologyNode
    err = s.db.QueryRow(
        `SELECT n.id, n.topology_id, n.group_id, n.name, n.role,
                COALESCE(n.host_id,''), n.notes, n.created_at, n.updated_at,
                COALESCE(h.name,''), COALESCE(h.ip,'')
         FROM topology_nodes n LEFT JOIN hosts h ON h.id = n.host_id WHERE n.id = ?`, id,
    ).Scan(&n.ID, &n.TopologyID, &n.GroupID, &n.Name, &n.Role,
        &n.HostID, &n.Notes, &n.CreatedAt, &n.UpdatedAt, &n.HostName, &n.IP)
    return &n, err
}

func (s *TopologyStore) DeleteNode(id string) error {
    _, err := s.db.Exec(`DELETE FROM topology_nodes WHERE id = ?`, id)
    return err
}

func (s *TopologyStore) ListEdges(topologyID string) ([]*models.TopologyEdge, error) {
    rows, err := s.db.Query(
        `SELECT id, topology_id, from_node, to_node, created_at FROM topology_edges WHERE topology_id = ?`,
        topologyID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var list []*models.TopologyEdge
    for rows.Next() {
        var e models.TopologyEdge
        if err := rows.Scan(&e.ID, &e.TopologyID, &e.FromNode, &e.ToNode, &e.CreatedAt); err != nil {
            return nil, err
        }
        list = append(list, &e)
    }
    return list, nil
}

func (s *TopologyStore) CreateEdge(topologyID string, req *models.CreateEdgeRequest) (*models.TopologyEdge, error) {
    id := uuid.New().String()
    now := time.Now().UTC()
    _, err := s.db.Exec(
        `INSERT INTO topology_edges (id, topology_id, from_node, to_node, created_at) VALUES (?, ?, ?, ?, ?)`,
        id, topologyID, req.FromNode, req.ToNode, now,
    )
    if err != nil {
        return nil, fmt.Errorf("create edge: %w", err)
    }
    var e models.TopologyEdge
    err = s.db.QueryRow(
        `SELECT id, topology_id, from_node, to_node, created_at FROM topology_edges WHERE id = ?`, id,
    ).Scan(&e.ID, &e.TopologyID, &e.FromNode, &e.ToNode, &e.CreatedAt)
    return &e, err
}

func (s *TopologyStore) DeleteEdge(id string) error {
    _, err := s.db.Exec(`DELETE FROM topology_edges WHERE id = ?`, id)
    return err
}

func (s *TopologyStore) GetFull(topologyID string) (*models.TopologyFull, error) {
    topo, err := s.GetByID(topologyID)
    if err != nil {
        return nil, err
    }
    groups, err := s.ListGroups(topologyID)
    if err != nil {
        return nil, err
    }
    nodes, err := s.ListNodes(topologyID)
    if err != nil {
        return nil, err
    }
    edges, err := s.ListEdges(topologyID)
    if err != nil {
        return nil, err
    }
    if groups == nil {
        groups = []*models.TopologyGroup{}
    }
    if nodes == nil {
        nodes = []*models.TopologyNode{}
    }
    if edges == nil {
        edges = []*models.TopologyEdge{}
    }
    return &models.TopologyFull{
        Topology: *topo,
        Groups:   groups,
        Nodes:    nodes,
        Edges:    edges,
    }, nil
}
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
go test ./internal/store/... -run TestTopologyStore -v
```

Expected: `PASS`

- [ ] **Step 7: Commit**

```bash
git add internal/store/topology_store.go internal/store/topology_store_test.go
git commit -m "feat(store): add TopologyStore with full CRUD"
```

---

## Task 4: Wire TopologyStore into App and main.go

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Add TopologyStore field to App struct**

In `internal/mcp/server.go`, add after `TodoTaskStore *store.TodoTaskStore`:

```go
TopologyStore *store.TopologyStore
```

- [ ] **Step 2: Init TopologyStore in main.go**

In `cmd/spider/main.go`, find where other stores are initialized (e.g. `store.NewHostStore(database)`). Add:

```go
topologyStore := store.NewTopologyStore(database)
```

Then in the `App{}` literal, add:

```go
TopologyStore: topologyStore,
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(app): wire TopologyStore into App"
```

---

## Task 5: API handlers

**Files:**
- Create: `internal/api/topology.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Create internal/api/topology.go (part 1 — topology CRUD)**

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func listTopologies(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	list, err := app.TopologyStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.Topology{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.CreateTopologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := app.TopologyStore.Create(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func getTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	full, err := app.TopologyStore.GetFull(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "topology not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, full)
}

func updateTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdateTopologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := app.TopologyStore.Update(id, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func deleteTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.TopologyStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Append group handlers to topology.go**

```go
func listGroups(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	list, err := app.TopologyStore.ListGroups(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.TopologyGroup{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var req models.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	g, err := app.TopologyStore.CreateGroup(topoID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func updateGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, gid string) {
	var req models.UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	g, err := app.TopologyStore.UpdateGroup(gid, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func deleteGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, gid string) {
	if err := app.TopologyStore.DeleteGroup(gid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Append node and edge handlers to topology.go**

```go
func listNodes(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	list, err := app.TopologyStore.ListNodes(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.TopologyNode{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createNode(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var req models.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	n, err := app.TopologyStore.CreateNode(topoID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func updateNode(app *mcppkg.App, w http.ResponseWriter, r *http.Request, nid string) {
	var req models.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	n, err := app.TopologyStore.UpdateNode(nid, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func deleteNode(app *mcppkg.App, w http.ResponseWriter, r *http.Request, nid string) {
	if err := app.TopologyStore.DeleteNode(nid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listEdges(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	list, err := app.TopologyStore.ListEdges(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.TopologyEdge{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createEdge(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var req models.CreateEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	e, err := app.TopologyStore.CreateEdge(topoID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func deleteEdge(app *mcppkg.App, w http.ResponseWriter, r *http.Request, eid string) {
	if err := app.TopologyStore.DeleteEdge(eid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// idFromPath extracts the last path segment: "/api/v1/topologies/abc123" -> "abc123"
func idFromPath(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	return parts[len(parts)-1]
}
```

- [ ] **Step 4: Register routes in handler.go**

In `internal/api/handler.go`, before the final `return` in `NewRouter()`, add:

```go
// Topology routes
mux.HandleFunc("/api/v1/topologies", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        listTopologies(app, w, r)
    case http.MethodPost:
        createTopology(app, w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
mux.HandleFunc("/api/v1/topologies/", func(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path
    // /api/v1/topologies/{id}/import
    if strings.HasSuffix(path, "/import") {
        topoID := idFromPath(strings.TrimSuffix(path, "/import"))
        if r.Method == http.MethodPost {
            importTopologyYAML(app, w, r, topoID)
        } else {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}/groups/{gid}
    if i := strings.Index(path, "/groups/"); i != -1 {
        gid := path[i+len("/groups/"):]
        switch r.Method {
        case http.MethodPut:
            updateGroup(app, w, r, gid)
        case http.MethodDelete:
            deleteGroup(app, w, r, gid)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}/groups
    if strings.HasSuffix(path, "/groups") {
        topoID := idFromPath(strings.TrimSuffix(path, "/groups"))
        switch r.Method {
        case http.MethodGet:
            listGroups(app, w, r, topoID)
        case http.MethodPost:
            createGroup(app, w, r, topoID)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}/nodes/{nid}
    if i := strings.Index(path, "/nodes/"); i != -1 {
        nid := path[i+len("/nodes/"):]
        switch r.Method {
        case http.MethodPut:
            updateNode(app, w, r, nid)
        case http.MethodDelete:
            deleteNode(app, w, r, nid)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}/nodes
    if strings.HasSuffix(path, "/nodes") {
        topoID := idFromPath(strings.TrimSuffix(path, "/nodes"))
        switch r.Method {
        case http.MethodGet:
            listNodes(app, w, r, topoID)
        case http.MethodPost:
            createNode(app, w, r, topoID)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}/edges/{eid}
    if i := strings.Index(path, "/edges/"); i != -1 {
        eid := path[i+len("/edges/"):]
        if r.Method == http.MethodDelete {
            deleteEdge(app, w, r, eid)
        } else {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}/edges
    if strings.HasSuffix(path, "/edges") {
        topoID := idFromPath(strings.TrimSuffix(path, "/edges"))
        switch r.Method {
        case http.MethodGet:
            listEdges(app, w, r, topoID)
        case http.MethodPost:
            createEdge(app, w, r, topoID)
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
        return
    }
    // /api/v1/topologies/{id}
    id := idFromPath(path)
    switch r.Method {
    case http.MethodGet:
        getTopology(app, w, r, id)
    case http.MethodPut:
        updateTopology(app, w, r, id)
    case http.MethodDelete:
        deleteTopology(app, w, r, id)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
```

- [ ] **Step 5: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/api/topology.go internal/api/handler.go
git commit -m "feat(api): add topology REST endpoints"
```

---

## Task 6: YAML import endpoint

**Files:**
- Modify: `internal/api/topology.go`

- [ ] **Step 1: Append importTopologyYAML to topology.go**

```go
type topoYAML struct {
	Name    string      `yaml:"name"`
	Layers  []layerYAML `yaml:"layers"`
	Devices []deviceYAML `yaml:"devices"`
}

type layerYAML struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color"`
}

type deviceYAML struct {
	Name     string   `yaml:"name"`
	Layer    string   `yaml:"layer"`
	Role     string   `yaml:"role"`
	IP       string   `yaml:"ip"`
	Upstream []string `yaml:"upstream"`
}

func importTopologyYAML(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var payload topoYAML
	if err := yaml.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid YAML: "+err.Error())
		return
	}

	// Ensure topology exists
	topo, err := app.TopologyStore.GetByID(topoID)
	if err != nil {
		writeError(w, http.StatusNotFound, "topology not found")
		return
	}

	// Build group map: name -> *TopologyGroup
	existingGroups, _ := app.TopologyStore.ListGroups(topoID)
	groupByName := map[string]*models.TopologyGroup{}
	for _, g := range existingGroups {
		groupByName[g.Name] = g
	}
	for _, layer := range payload.Layers {
		if _, ok := groupByName[layer.Name]; !ok {
			color := layer.Color
			if color == "" {
				color = "#3b82f6"
			}
			g, err := app.TopologyStore.CreateGroup(topoID, &models.CreateGroupRequest{Name: layer.Name, Color: color})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create group: "+err.Error())
				return
			}
			groupByName[layer.Name] = g
		}
	}

	// Build node map: name -> *TopologyNode
	existingNodes, _ := app.TopologyStore.ListNodes(topoID)
	nodeByName := map[string]*models.TopologyNode{}
	for _, n := range existingNodes {
		nodeByName[n.Name] = n
	}

	// Match hosts by IP
	hosts, _ := app.HostStore.List("")
	hostByIP := map[string]string{}
	for _, h := range hosts {
		hostByIP[h.IP] = h.ID
	}

	for _, dev := range payload.Devices {
		grp, ok := groupByName[dev.Layer]
		if !ok {
			writeError(w, http.StatusBadRequest, "unknown layer: "+dev.Layer)
			return
		}
		hostID := hostByIP[dev.IP]
		if _, exists := nodeByName[dev.Name]; !exists {
			n, err := app.TopologyStore.CreateNode(topoID, &models.CreateNodeRequest{
				GroupID: grp.ID,
				Name:    dev.Name,
				Role:    dev.Role,
				HostID:  hostID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create node: "+err.Error())
				return
			}
			nodeByName[dev.Name] = n
		}
	}

	// Create edges from upstream declarations
	existingEdges, _ := app.TopologyStore.ListEdges(topoID)
	edgeKey := func(from, to string) string { return from + "->" + to }
	edgeExists := map[string]bool{}
	for _, e := range existingEdges {
		edgeExists[edgeKey(e.FromNode, e.ToNode)] = true
	}
	for _, dev := range payload.Devices {
		toNode, ok := nodeByName[dev.Name]
		if !ok {
			continue
		}
		for _, upName := range dev.Upstream {
			fromNode, ok := nodeByName[upName]
			if !ok {
				continue
			}
			key := edgeKey(fromNode.ID, toNode.ID)
			if !edgeExists[key] {
				_, _ = app.TopologyStore.CreateEdge(topoID, &models.CreateEdgeRequest{
					FromNode: fromNode.ID,
					ToNode:   toNode.ID,
				})
				edgeExists[key] = true
			}
		}
	}

	full, err := app.TopologyStore.GetFull(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = topo
	writeJSON(w, http.StatusOK, full)
}
```

Add import `"gopkg.in/yaml.v3"` to topology.go imports.

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/api/topology.go
git commit -m "feat(api): add YAML import endpoint for topology"
```

---

## Task 7: Agent tool — GetTopologyTool

**Files:**
- Create: `internal/agent/tools_topology.go`
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Create tools_topology.go**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/store"
)

type GetTopologyTool struct {
	topos *store.TopologyStore
}

func NewGetTopologyTool(topos *store.TopologyStore) *GetTopologyTool {
	return &GetTopologyTool{topos: topos}
}

func (t *GetTopologyTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *GetTopologyTool) Name() string                { return "GetTopology" }
func (t *GetTopologyTool) Description() string {
	return "Get topology data including groups, nodes, and edges. Read-only. Use in Explore phase."
}

func (t *GetTopologyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topology_id": map[string]any{
				"type":        "string",
				"description": "Topology ID (use if known)",
			},
			"topology_name": map[string]any{
				"type":        "string",
				"description": "Topology name (used if topology_id not provided)",
			},
		},
	}
}

func (t *GetTopologyTool) SystemPromptSection() string { return "" }

func (t *GetTopologyTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	id, _ := input["topology_id"].(string)
	name, _ := input["topology_name"].(string)

	if id == "" && name == "" {
		// Return list of topologies
		list, err := t.topos.List()
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(list)
		return string(b), nil
	}

	if id == "" {
		list, err := t.topos.List()
		if err != nil {
			return "", err
		}
		for _, topo := range list {
			if topo.Name == name {
				id = topo.ID
				break
			}
		}
		if id == "" {
			return "", fmt.Errorf("topology %q not found", name)
		}
	}

	full, err := t.topos.GetFull(id)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(full)
	return string(b), nil
}
```

- [ ] **Step 2: Register in factory.go**

In `internal/agent/factory.go`, in `buildRegistry()`, add:

```go
registry.Register(NewGetTopologyTool(f.TopologyStore))
```

Add `TopologyStore *store.TopologyStore` field to `Factory` struct.

Pass `TopologyStore` when constructing Factory in `cmd/spider/main.go` (or wherever `agent.NewFactory` is called — check `internal/mcp/server.go` `NewAgentFactory()`):

In `internal/mcp/server.go` `NewAgentFactory()`, pass `a.TopologyStore` to the factory. Find the `agent.NewFactory(...)` call and add `a.TopologyStore` as a parameter. Update `agent.NewFactory` signature accordingly.

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/tools_topology.go internal/agent/factory.go internal/mcp/server.go
git commit -m "feat(agent): add GetTopologyTool"
```

---

## Task 8: Frontend — install deps + API client

**Files:**
- Modify: `web/package.json`
- Create: `web/src/api/topology.ts`

- [ ] **Step 1: Install cytoscape and dagre**

```bash
cd web && npm install cytoscape cytoscape-dagre
npm install --save-dev @types/cytoscape
```

Expected: packages added to `package.json`.

- [ ] **Step 2: Create web/src/api/topology.ts**

```typescript
import { authHeaders } from './auth'

export interface Topology {
  id: string
  name: string
  notes: string
  created_at: string
  updated_at: string
}

export interface TopologyGroup {
  id: string
  topology_id: string
  name: string
  color: string
  sort_order: number
  created_at: string
}

export interface TopologyNode {
  id: string
  topology_id: string
  group_id: string
  name: string
  role: string
  host_id?: string
  host_name?: string
  ip?: string
  notes: string
  created_at: string
  updated_at: string
}

export interface TopologyEdge {
  id: string
  topology_id: string
  from_node: string
  to_node: string
  created_at: string
}

export interface TopologyFull extends Topology {
  groups: TopologyGroup[]
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

const BASE = '/api/v1/topologies'

export async function listTopologies(): Promise<Topology[]> {
  const r = await fetch(BASE, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function getTopologyFull(id: string): Promise<TopologyFull> {
  const r = await fetch(`${BASE}/${id}`, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function createTopology(name: string, notes = ''): Promise<Topology> {
  const r = await fetch(BASE, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, notes }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteTopology(id: string): Promise<void> {
  const r = await fetch(`${BASE}/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
}

export async function createGroup(topoID: string, name: string, color: string): Promise<TopologyGroup> {
  const r = await fetch(`${BASE}/${topoID}/groups`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, color }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function createNode(topoID: string, req: { group_id: string; name: string; role?: string; host_id?: string }): Promise<TopologyNode> {
  const r = await fetch(`${BASE}/${topoID}/nodes`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function updateNode(topoID: string, nodeID: string, req: { group_id: string; name: string; role?: string; host_id?: string; notes?: string }): Promise<TopologyNode> {
  const r = await fetch(`${BASE}/${topoID}/nodes/${nodeID}`, {
    method: 'PUT',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteNode(topoID: string, nodeID: string): Promise<void> {
  const r = await fetch(`${BASE}/${topoID}/nodes/${nodeID}`, { method: 'DELETE', headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
}

export async function createEdge(topoID: string, fromNode: string, toNode: string): Promise<TopologyEdge> {
  const r = await fetch(`${BASE}/${topoID}/edges`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ from_node: fromNode, to_node: toNode }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteEdge(topoID: string, edgeID: string): Promise<void> {
  const r = await fetch(`${BASE}/${topoID}/edges/${edgeID}`, { method: 'DELETE', headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
}

export async function importYAML(topoID: string, yamlText: string): Promise<TopologyFull> {
  const r = await fetch(`${BASE}/${topoID}/import`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/x-yaml' },
    body: yamlText,
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}
```

- [ ] **Step 3: Build frontend**

```bash
cd web && npm run build
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/package.json web/package-lock.json web/src/api/topology.ts
git commit -m "feat(web): add topology API client and cytoscape deps"
```

---

## Task 9: Frontend — TopologyView.vue (layout + list panel)

**Files:**
- Create: `web/src/views/TopologyView.vue`

- [ ] **Step 1: Create TopologyView.vue scaffold with 3-panel layout**

Create `web/src/views/TopologyView.vue`:

```vue
<template>
  <div class="topo-page">
    <!-- Left: topology list -->
    <aside class="topo-sidebar">
      <div class="topo-sidebar-header">
        <span class="topo-sidebar-title">拓扑</span>
        <button class="topo-add-btn" @click="showCreate = true">+</button>
      </div>
      <div
        v-for="t in topologies"
        :key="t.id"
        class="topo-item"
        :class="{ active: activeTopo?.id === t.id }"
        @click="selectTopo(t)"
      >
        {{ t.name }}
      </div>
      <div v-if="topologies.length === 0" class="topo-empty">暂无拓扑</div>

      <!-- Create topology dialog -->
      <div v-if="showCreate" class="topo-dialog-overlay" @click.self="showCreate = false">
        <div class="topo-dialog">
          <div class="topo-dialog-title">新建拓扑</div>
          <input v-model="newName" class="topo-input" placeholder="拓扑名称" @keyup.enter="doCreate" />
          <div class="topo-dialog-actions">
            <button class="topo-btn-secondary" @click="showCreate = false">取消</button>
            <button class="topo-btn-primary" @click="doCreate">创建</button>
          </div>
        </div>
      </div>
    </aside>

    <!-- Center: canvas -->
    <div class="topo-canvas-wrap">
      <div v-if="!activeTopo" class="topo-canvas-empty">选择或创建一个拓扑</div>
      <div v-else ref="cyContainer" class="topo-cy"></div>
    </div>

    <!-- Right: node detail -->
    <aside class="topo-detail" :class="{ visible: !!activeNode }">
      <template v-if="activeNode">
        <div class="topo-detail-title">{{ activeNode.host_name || activeNode.name }}</div>
        <div v-if="activeNode.host_name" class="topo-detail-sub">{{ activeNode.name }}</div>
        <div class="topo-detail-row"><span class="topo-detail-label">IP</span><span>{{ activeNode.ip || '—' }}</span></div>
        <div class="topo-detail-row"><span class="topo-detail-label">角色</span><span>{{ activeNode.role || '—' }}</span></div>
        <div class="topo-detail-row"><span class="topo-detail-label">分组</span><span>{{ groupName(activeNode.group_id) }}</span></div>
        <div class="topo-detail-section">上游</div>
        <div v-for="n in upstreamOf(activeNode.id)" :key="n.id" class="topo-detail-neighbor">{{ n.host_name || n.name }}</div>
        <div v-if="upstreamOf(activeNode.id).length === 0" class="topo-detail-empty">无</div>
        <div class="topo-detail-section">下游</div>
        <div v-for="n in downstreamOf(activeNode.id)" :key="n.id" class="topo-detail-neighbor">{{ n.host_name || n.name }}</div>
        <div v-if="downstreamOf(activeNode.id).length === 0" class="topo-detail-empty">无</div>
      </template>
    </aside>
  </div>
</template>
```

- [ ] **Step 2: Add script section to TopologyView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
import cytoscape from 'cytoscape'
import dagre from 'cytoscape-dagre'
import type { TopologyFull, Topology, TopologyNode } from '../api/topology'
import { listTopologies, getTopologyFull, createTopology } from '../api/topology'

cytoscape.use(dagre)

const topologies = ref<Topology[]>([])
const activeTopo = ref<TopologyFull | null>(null)
const activeNode = ref<TopologyNode | null>(null)
const cyContainer = ref<HTMLElement | null>(null)
const showCreate = ref(false)
const newName = ref('')
let cy: cytoscape.Core | null = null

onMounted(async () => {
  topologies.value = await listTopologies()
  if (topologies.value.length > 0) await selectTopo(topologies.value[0])
})

async function selectTopo(t: Topology) {
  activeNode.value = null
  activeTopo.value = await getTopologyFull(t.id)
  await nextTick()
  renderGraph()
}

async function doCreate() {
  if (!newName.value.trim()) return
  const t = await createTopology(newName.value.trim())
  topologies.value.push(t)
  showCreate.value = false
  newName.value = ''
  await selectTopo(t)
}

function groupColor(groupID: string): string {
  const g = activeTopo.value?.groups.find(g => g.id === groupID)
  return g?.color ?? '#3b82f6'
}

function groupName(groupID: string): string {
  return activeTopo.value?.groups.find(g => g.id === groupID)?.name ?? ''
}

function upstreamOf(nodeID: string): TopologyNode[] {
  if (!activeTopo.value) return []
  const fromIDs = activeTopo.value.edges.filter(e => e.to_node === nodeID).map(e => e.from_node)
  return activeTopo.value.nodes.filter(n => fromIDs.includes(n.id))
}

function downstreamOf(nodeID: string): TopologyNode[] {
  if (!activeTopo.value) return []
  const toIDs = activeTopo.value.edges.filter(e => e.from_node === nodeID).map(e => e.to_node)
  return activeTopo.value.nodes.filter(n => toIDs.includes(n.id))
}

function renderGraph() {
  if (!cyContainer.value || !activeTopo.value) return
  if (cy) { cy.destroy(); cy = null }

  const topo = activeTopo.value
  const elements: cytoscape.ElementDefinition[] = []

  for (const node of topo.nodes) {
    const color = groupColor(node.group_id)
    const bound = !!node.host_id
    const label = (node.host_name || node.name).length > 10
      ? (node.host_name || node.name).slice(0, 10) + '…'
      : (node.host_name || node.name)
    elements.push({
      data: { id: node.id, label, bound, color, nodeRef: node },
    })
  }

  for (const edge of topo.edges) {
    const fromNode = topo.nodes.find(n => n.id === edge.from_node)
    const color = fromNode ? groupColor(fromNode.group_id) : '#1f2937'
    const bound = !!fromNode?.host_id
    elements.push({
      data: { id: edge.id, source: edge.from_node, target: edge.to_node, color, bound },
    })
  }

  cy = cytoscape({
    container: cyContainer.value,
    elements,
    style: [
      {
        selector: 'node',
        style: {
          'background-color': (ele: any) => ele.data('bound') ? ele.data('color') : '#1a1a1a',
          'border-color': (ele: any) => ele.data('bound') ? ele.data('color') : '#374151',
          'border-width': 2,
          'label': 'data(label)',
          'color': (ele: any) => ele.data('bound') ? '#fff' : '#374151',
          'font-size': 11,
          'text-valign': 'center',
          'text-halign': 'center',
          'width': 100,
          'height': 36,
          'shape': 'roundrectangle',
        },
      },
      {
        selector: 'edge',
        style: {
          'line-color': 'data(color)',
          'target-arrow-color': 'data(color)',
          'target-arrow-shape': 'triangle',
          'curve-style': 'bezier',
          'line-style': (ele: any) => ele.data('bound') ? 'solid' : 'dashed',
          'opacity': 0.6,
          'width': 1.5,
        },
      },
      {
        selector: 'node:selected',
        style: { 'border-width': 3, 'border-color': '#fff' },
      },
    ],
    layout: { name: 'dagre', rankDir: 'TB', nodeSep: 40, rankSep: 60 } as any,
  })

  cy.on('tap', 'node', (evt) => {
    activeNode.value = evt.target.data('nodeRef') as TopologyNode
  })
  cy.on('tap', (evt) => {
    if (evt.target === cy) activeNode.value = null
  })
}
</script>
```

- [ ] **Step 3: Add style section to TopologyView.vue**

```vue
<style scoped>
.topo-page { display: flex; flex: 1; min-height: 0; overflow: hidden; background: #0d0d0d; }

.topo-sidebar {
  width: 180px; min-width: 140px; border-right: 1px solid #1f2937;
  display: flex; flex-direction: column; overflow-y: auto; padding: 8px 0;
}
.topo-sidebar-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 4px 12px 8px; font-size: 11px; text-transform: uppercase;
  letter-spacing: .06em; color: #6b7280;
}
.topo-add-btn {
  background: none; border: 1px solid #374151; color: #9ca3af;
  border-radius: 4px; width: 20px; height: 20px; cursor: pointer; font-size: 14px;
  display: flex; align-items: center; justify-content: center; padding: 0;
}
.topo-add-btn:hover { border-color: #6b7280; color: #fff; }
.topo-item {
  padding: 6px 12px; font-size: 13px; color: #9ca3af; cursor: pointer;
  border-radius: 4px; margin: 0 4px;
}
.topo-item:hover { background: #1f2937; color: #fff; }
.topo-item.active { background: #1f2937; color: #fff; }
.topo-empty { padding: 12px; font-size: 12px; color: #4b5563; text-align: center; }

.topo-canvas-wrap { flex: 1; min-width: 0; position: relative; }
.topo-canvas-empty { display: flex; align-items: center; justify-content: center; height: 100%; color: #4b5563; font-size: 14px; }
.topo-cy { width: 100%; height: 100%; }

.topo-detail {
  width: 0; overflow: hidden; transition: width .2s; border-left: 1px solid #1f2937;
  display: flex; flex-direction: column; padding: 0;
}
.topo-detail.visible { width: 220px; padding: 16px; overflow-y: auto; }
.topo-detail-title { font-size: 14px; font-weight: 600; color: #f9fafb; margin-bottom: 2px; }
.topo-detail-sub { font-size: 12px; color: #6b7280; margin-bottom: 12px; }
.topo-detail-row { display: flex; gap: 8px; font-size: 12px; margin-bottom: 6px; }
.topo-detail-label { color: #6b7280; min-width: 32px; }
.topo-detail-section { font-size: 11px; text-transform: uppercase; letter-spacing: .06em; color: #4b5563; margin: 12px 0 4px; }
.topo-detail-neighbor { font-size: 12px; color: #9ca3af; padding: 2px 0; }
.topo-detail-empty { font-size: 12px; color: #374151; }

.topo-dialog-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.6);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.topo-dialog {
  background: #1a1a1a; border: 1px solid #374151; border-radius: 8px;
  padding: 20px; min-width: 280px; display: flex; flex-direction: column; gap: 12px;
}
.topo-dialog-title { font-size: 14px; font-weight: 600; color: #f9fafb; }
.topo-input {
  background: #0d0d0d; border: 1px solid #374151; border-radius: 4px;
  padding: 6px 10px; color: #f9fafb; font-size: 13px; outline: none;
}
.topo-input:focus { border-color: #3b82f6; }
.topo-dialog-actions { display: flex; gap: 8px; justify-content: flex-end; }
.topo-btn-primary {
  background: #3b82f6; color: #fff; border: none; border-radius: 4px;
  padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.topo-btn-primary:hover { background: #2563eb; }
.topo-btn-secondary {
  background: none; color: #9ca3af; border: 1px solid #374151; border-radius: 4px;
  padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.topo-btn-secondary:hover { border-color: #6b7280; color: #fff; }
</style>
```

- [ ] **Step 4: Build frontend**

```bash
cd web && npm run build
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/TopologyView.vue
git commit -m "feat(web): add TopologyView with cytoscape dagre canvas"
```

---

## Task 10: Wire route + nav link

**Files:**
- Modify: `web/src/main.ts`
- Modify: `web/src/App.vue`

- [ ] **Step 1: Add /topology route to main.ts**

In `web/src/main.ts`, add to the `routes` array:

```typescript
{ path: '/topology', component: () => import('./views/TopologyView.vue') },
```

- [ ] **Step 2: Add nav link in App.vue**

Find the nav links in `web/src/App.vue` (look for `router-link` elements pointing to `/hosts`, `/exec`, etc.). Add:

```html
<router-link to="/topology">拓扑</router-link>
```

Place it after the Hosts link.

- [ ] **Step 3: Build frontend**

```bash
cd web && npm run build
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/main.ts web/src/App.vue
git commit -m "feat(web): add topology route and nav link"
```

---

## Task 11: Final smoke test

- [ ] **Step 1: Run all Go tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 2: Build binary**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
```

Expected: no errors.

- [ ] **Step 3: Start server and verify topology API**

```bash
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data &
sleep 2

# Create topology
curl -s -X POST http://localhost:8002/api/v1/topologies \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke-test"}' | jq .id

# List topologies
curl -s http://localhost:8002/api/v1/topologies | jq length
```

Expected: topology created, list returns 1+.

- [ ] **Step 4: Kill test server**

```bash
pkill -f spider-test
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: network topology — backend + agent tool + frontend"
```
