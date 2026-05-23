package store

import (
	"context"
	"testing"
)

func TestDocumentStore_SaveAndSearch(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	err := ds.Save("huawei", []string{"cli_ref"}, "display interface", "display interface [type] [number]", nil, "cli-ref.md", 0, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	err = ds.Save("cisco", []string{"cli_ref"}, "show interface", "show interface [type] [number]", nil, "cli-ref.md", 1, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	docs, err := ds.ListByVendor("huawei")
	if err != nil {
		t.Fatalf("ListByVendor: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("len = %d, want 1", len(docs))
	}
	if docs[0].Title != "display interface" {
		t.Errorf("Title = %q, want %q", docs[0].Title, "display interface")
	}
}

func TestDocumentStore_DeleteBySource(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	ds.Save("huawei", []string{"cli_ref"}, "cmd1", "content1", nil, "file-a.md", 0, nil)
	ds.Save("huawei", []string{"cli_ref"}, "cmd2", "content2", nil, "file-a.md", 1, nil)
	ds.Save("cisco", []string{"cli_ref"}, "cmd3", "content3", nil, "file-b.md", 0, nil)

	err := ds.DeleteBySource("file-a.md")
	if err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}
	all, _ := ds.List()
	if len(all) != 1 {
		t.Errorf("len = %d, want 1", len(all))
	}
}

func TestFindByTitle(t *testing.T) {
	db := setupTestDB(t)
	s := NewDocumentStore(db)

	gid := 1
	err := s.Save("v", nil, "nginx配置说明", "worker_processes auto;", nil, "f.txt", 0, &gid)
	if err != nil {
		t.Fatal(err)
	}

	doc, err := s.FindByTitle(gid, "nginx配置说明")
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected doc, got nil")
	}
	if doc.Title != "nginx配置说明" {
		t.Errorf("unexpected title: %s", doc.Title)
	}

	missing, err := s.FindByTitle(gid, "不存在的文档")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Error("expected nil for missing doc")
	}
}

func TestDocumentStore_DeleteBatch(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	ds.Save("h3c", nil, "doc1", "content1", nil, "f.md", 0, nil)
	ds.Save("h3c", nil, "doc2", "content2", nil, "f.md", 1, nil)
	ds.Save("cisco", nil, "doc3", "content3", nil, "g.md", 0, nil)

	all, _ := ds.List()
	ids := []int{all[0].ID, all[1].ID}

	err := ds.DeleteBatch(ids)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}
	remaining, _ := ds.List()
	if len(remaining) != 1 {
		t.Errorf("len = %d, want 1", len(remaining))
	}
	if remaining[0].Title != "doc3" {
		t.Errorf("remaining = %q, want doc3", remaining[0].Title)
	}
}

func TestDocumentStore_DeleteBatch_Empty(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	ds.Save("h3c", nil, "doc1", "content1", nil, "f.md", 0, nil)

	if err := ds.DeleteBatch([]int{}); err != nil {
		t.Fatalf("DeleteBatch empty: %v", err)
	}
	all, _ := ds.List()
	if len(all) != 1 {
		t.Errorf("len = %d, want 1", len(all))
	}
}

func TestDocumentStore_UpdateDescription(t *testing.T) {
	db := setupTestDB(t)
	ds := NewDocumentStore(db)
	ctx := context.Background()

	if err := ds.Save("vendor", []string{}, "title", "content", nil, "f.md", 0, nil); err != nil {
		t.Fatal(err)
	}
	docs, _ := ds.List()
	if len(docs) == 0 {
		t.Fatal("no docs")
	}
	id := docs[0].ID

	if err := ds.UpdateDescription(ctx, id, "A test description."); err != nil {
		t.Fatal(err)
	}
	doc, err := ds.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Description != "A test description." {
		t.Fatalf("got %q, want %q", doc.Description, "A test description.")
	}
}

func TestGroupStore_UpdateDescription(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGroupStore(db)
	ctx := context.Background()

	g, err := gs.Create("mygroup")
	if err != nil {
		t.Fatal(err)
	}
	if err := gs.UpdateDescription(ctx, g.ID, "Group about ops."); err != nil {
		t.Fatal(err)
	}
	groups, err := gs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Description != "Group about ops." {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}
