package store_test

import (
	"testing"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func newTestTopologyStore(t *testing.T) *store.TopologyStore {
	t.Helper()
	tmpDir := t.TempDir()
	database, err := db.Open(tmpDir)
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
	node, _ := s.CreateNode(topo.ID, &models.CreateNodeRequest{Layer: "核心层", Name: "fw-01"})
	node2, _ := s.CreateNode(topo.ID, &models.CreateNodeRequest{Layer: "接入层", Name: "fw-02"})
	_, _ = s.CreateEdge(topo.ID, &models.CreateEdgeRequest{FromNode: node.ID, ToNode: node2.ID})

	full, err := s.GetFull(topo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Nodes) != 2 {
		t.Errorf("want 2 nodes, got %d", len(full.Nodes))
	}
	if len(full.Edges) != 1 {
		t.Errorf("want 1 edge, got %d", len(full.Edges))
	}
}
