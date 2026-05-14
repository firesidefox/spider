package store

import "testing"

func TestGroupStore_DeleteBatch_WithDocs(t *testing.T) {
	database := setupTestDB(t)
	gs := NewGroupStore(database)
	ds := NewDocumentStore(database)

	g1, _ := gs.Create("group1")
	g2, _ := gs.Create("group2")
	ds.Save("h3c", nil, "doc1", "c1", nil, "f.md", 0, &g1.ID)
	ds.Save("h3c", nil, "doc2", "c2", nil, "f.md", 1, &g1.ID)

	err := gs.DeleteBatch([]int{g1.ID}, true)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}

	groups, _ := gs.List()
	if len(groups) != 1 || groups[0].ID != g2.ID {
		t.Errorf("expected 1 group (g2), got %d", len(groups))
	}
	docs, _ := ds.List()
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

func TestGroupStore_DeleteBatch_MoveDocs(t *testing.T) {
	database := setupTestDB(t)
	gs := NewGroupStore(database)
	ds := NewDocumentStore(database)

	g1, _ := gs.Create("group1")
	ds.Save("h3c", nil, "doc1", "c1", nil, "f.md", 0, &g1.ID)
	ds.Save("h3c", nil, "doc2", "c2", nil, "f.md", 1, &g1.ID)

	err := gs.DeleteBatch([]int{g1.ID}, false)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}

	groups, _ := gs.List()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	docs, _ := ds.List()
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
	for _, d := range docs {
		if d.GroupID != nil {
			t.Errorf("doc %d still has group_id %v, want nil", d.ID, *d.GroupID)
		}
	}
}

func TestGroupStore_DeleteBatch_Empty(t *testing.T) {
	database := setupTestDB(t)
	gs := NewGroupStore(database)
	ds := NewDocumentStore(database)

	g1, _ := gs.Create("group1")
	ds.Save("h3c", nil, "doc1", "c1", nil, "f.md", 0, &g1.ID)

	if err := gs.DeleteBatch([]int{}, true); err != nil {
		t.Fatalf("DeleteBatch empty: %v", err)
	}
	groups, _ := gs.List()
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

func TestGroupStore_DeleteBatch_MultipleGroups(t *testing.T) {
	database := setupTestDB(t)
	gs := NewGroupStore(database)
	ds := NewDocumentStore(database)

	g1, _ := gs.Create("group1")
	g2, _ := gs.Create("group2")
	g3, _ := gs.Create("group3")
	ds.Save("h3c", nil, "doc1", "c1", nil, "f.md", 0, &g1.ID)
	ds.Save("h3c", nil, "doc2", "c2", nil, "f.md", 0, &g2.ID)

	err := gs.DeleteBatch([]int{g1.ID, g2.ID}, true)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}

	groups, _ := gs.List()
	if len(groups) != 1 || groups[0].ID != g3.ID {
		t.Errorf("expected 1 group (g3), got %d", len(groups))
	}
	docs, _ := ds.List()
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}
