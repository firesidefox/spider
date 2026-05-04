package store

import "testing"

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
