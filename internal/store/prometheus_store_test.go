package store_test

import (
	"database/sql"
	"testing"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func newPrometheusTestDB(t *testing.T) (*sql.DB, *crypto.Manager) {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return database, cm
}

func TestPrometheusSourceCRUD(t *testing.T) {
	database, cm := newPrometheusTestDB(t)
	s := store.NewPrometheusSourceStore(database, cm)

	// Add
	src, err := s.Add(&models.AddPrometheusSourceRequest{
		Name:     "test",
		BaseURL:  "http://localhost:9090",
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
	if src.TimeoutSeconds != 30 {
		t.Fatalf("expected default timeout 30, got %d", src.TimeoutSeconds)
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

	// DecryptCredentials
	pwd, _, err := s.DecryptCredentials(got)
	if err != nil {
		t.Fatalf("DecryptCredentials: %v", err)
	}
	if pwd != "secret" {
		t.Fatalf("expected decrypted password 'secret', got %q", pwd)
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

func TestPrometheusBindingCRUD(t *testing.T) {
	database, cm := newPrometheusTestDB(t)
	ss := store.NewPrometheusSourceStore(database, cm)
	bs := store.NewPrometheusBindingStore(database)

	src, _ := ss.Add(&models.AddPrometheusSourceRequest{
		Name: "prom", BaseURL: "http://p:9090", AuthType: models.PrometheusAuthNone,
	})

	// Insert a topology for FK
	_, err := database.Exec(`INSERT INTO topologies (id,name,notes,created_at,updated_at) VALUES ('t1','topo','',datetime('now'),datetime('now'))`)
	if err != nil {
		t.Fatalf("insert topology: %v", err)
	}

	// Add topology_layer binding
	b, err := bs.Add(&models.AddPrometheusBindingRequest{
		SourceID:   src.ID,
		ScopeType:  models.ScopeTopologyLayer,
		TopologyID: "t1",
		Layer:      "server",
	})
	if err != nil {
		t.Fatalf("Add topology_layer binding: %v", err)
	}
	if b.ID == "" {
		t.Fatal("expected ID")
	}
	if b.TopologyID != "t1" {
		t.Fatalf("expected topology_id t1, got %s", b.TopologyID)
	}

	// List bindings by source
	list, err := bs.ListBySource(src.ID)
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(list))
	}

	// Duplicate (topology_id, layer) must fail
	_, err = bs.Add(&models.AddPrometheusBindingRequest{
		SourceID:   src.ID,
		ScopeType:  models.ScopeTopologyLayer,
		TopologyID: "t1",
		Layer:      "server",
	})
	if err == nil {
		t.Fatal("expected error for duplicate (topology_id, layer)")
	}

	// Delete binding
	if err := bs.Delete(b.ID); err != nil {
		t.Fatalf("Delete binding: %v", err)
	}
	_, err = bs.GetByID(b.ID)
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFindSourceIDForHost(t *testing.T) {
	database, cm := newPrometheusTestDB(t)
	ss := store.NewPrometheusSourceStore(database, cm)
	bs := store.NewPrometheusBindingStore(database)

	src, _ := ss.Add(&models.AddPrometheusSourceRequest{
		Name: "prom", BaseURL: "http://p:9090", AuthType: models.PrometheusAuthNone,
	})

	// Insert topology, host, and topology_node
	if _, err := database.Exec(`INSERT INTO topologies (id,name,notes,created_at,updated_at) VALUES ('t1','topo','',datetime('now'),datetime('now'))`); err != nil {
		t.Fatalf("insert topology: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO hosts (id,name,ip,tags,created_at,updated_at) VALUES ('h1','host1','10.0.0.1','[]',datetime('now'),datetime('now'))`); err != nil {
		t.Fatalf("insert host: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO topology_nodes (id,topology_id,layer,name,role,host_id,notes,created_at,updated_at) VALUES ('n1','t1','server','node1','','h1','',datetime('now'),datetime('now'))`); err != nil {
		t.Fatalf("insert topology_node: %v", err)
	}

	// No binding yet — should error
	_, err := bs.FindSourceIDForHost("h1")
	if err == nil {
		t.Fatal("expected error when no binding configured")
	}

	// Add topology_layer binding
	if _, err := bs.Add(&models.AddPrometheusBindingRequest{
		SourceID: src.ID, ScopeType: models.ScopeTopologyLayer,
		TopologyID: "t1", Layer: "server",
	}); err != nil {
		t.Fatalf("Add binding: %v", err)
	}

	// Should find via topology_layer
	foundID, err := bs.FindSourceIDForHost("h1")
	if err != nil {
		t.Fatalf("FindSourceIDForHost via topology_layer: %v", err)
	}
	if foundID != src.ID {
		t.Fatalf("expected source %s, got %s", src.ID, foundID)
	}

	// Add host-level binding with a different source — should override
	src2, _ := ss.Add(&models.AddPrometheusSourceRequest{
		Name: "prom2", BaseURL: "http://p2:9090", AuthType: models.PrometheusAuthNone,
	})
	if _, err := bs.Add(&models.AddPrometheusBindingRequest{
		SourceID: src2.ID, ScopeType: models.ScopeHost, HostID: "h1",
	}); err != nil {
		t.Fatalf("Add binding: %v", err)
	}

	foundID, err = bs.FindSourceIDForHost("h1")
	if err != nil {
		t.Fatalf("FindSourceIDForHost after host override: %v", err)
	}
	if foundID != src2.ID {
		t.Fatalf("expected host-level override source %s, got %s", src2.ID, foundID)
	}
}
