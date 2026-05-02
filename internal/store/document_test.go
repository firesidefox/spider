package store

import "testing"

func TestDocumentStore_SaveAndSearch(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	err := ds.Save("huawei", "vrp", "cli_ref", "display interface", "display interface [type] [number]", nil, "cli-ref.md", 0)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	err = ds.Save("cisco", "ios", "cli_ref", "show interface", "show interface [type] [number]", nil, "cli-ref.md", 1)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	docs, err := ds.ListByVendor("huawei", "vrp")
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

	ds.Save("huawei", "vrp", "cli_ref", "cmd1", "content1", nil, "file-a.md", 0)
	ds.Save("huawei", "vrp", "cli_ref", "cmd2", "content2", nil, "file-a.md", 1)
	ds.Save("cisco", "ios", "cli_ref", "cmd3", "content3", nil, "file-b.md", 0)

	err := ds.DeleteBySource("file-a.md")
	if err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}
	all, _ := ds.List()
	if len(all) != 1 {
		t.Errorf("len = %d, want 1", len(all))
	}
}
