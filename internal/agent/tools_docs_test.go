package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/store"
)

func TestSearchDocsTool_Metadata(t *testing.T) {
	tool := NewSearchDocsTool(nil, nil)

	if tool.Name() != "SearchDocs" {
		t.Errorf("got name %q, want %q", tool.Name(), "SearchDocs")
	}
	if tool.Description() == "" {
		t.Error("description must not be empty")
	}

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"query", "vendor", "group_ids", "doc_ids"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
	req, _ := schema["required"].([]string)
	if len(req) != 1 || req[0] != "query" {
		t.Errorf("required = %v, want [query]", req)
	}
}

func TestSearchDocsTool_ImplementsTool(t *testing.T) {
	var _ Tool = NewSearchDocsTool(nil, nil)
}

func TestSearchDocsTool_Schema_HasCatalogAndGroupID(t *testing.T) {
	tool := NewSearchDocsTool(nil, nil)
	props, ok := tool.InputSchema()["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"catalog", "group_id"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
}

func TestCallRESTAPITool_Metadata(t *testing.T) {
	tool := NewCallRESTAPITool(nil)

	if tool.Name() != "CallAPI" {
		t.Errorf("got name %q, want %q", tool.Name(), "CallAPI")
	}
	if tool.Description() == "" {
		t.Error("description must not be empty")
	}

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	for _, key := range []string{"url", "method", "headers", "body"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema missing property %q", key)
		}
	}
}

func TestCallRESTAPITool_ImplementsTool(t *testing.T) {
	var _ Tool = NewCallRESTAPITool(nil)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestSearchDocsTool_Catalog(t *testing.T) {
	database := openTestDB(t)
	ds := store.NewDocumentStore(database)
	gid := 1
	ds.Save("v", nil, "doc-alpha", "full content alpha", nil, "a.md", 0, &gid)
	ds.Save("v", nil, "doc-beta", "full content beta", nil, "b.md", 0, &gid)

	tool := NewSearchDocsTool(nil, ds)
	result, err := tool.Execute(context.Background(), map[string]any{
		"query":    "",
		"catalog":  true,
		"group_id": float64(gid),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(result.Content), &items); err != nil {
		t.Fatalf("unmarshal: %v — content: %s", err, result.Content)
	}
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}
	for _, item := range items {
		if _, ok := item["id"]; !ok {
			t.Error("item missing id")
		}
		if _, ok := item["title"]; !ok {
			t.Error("item missing title")
		}
		if _, ok := item["content"]; ok {
			t.Error("catalog must not include content")
		}
	}
}

func TestSearchDocsTool_Catalog_MissingGroupID(t *testing.T) {
	database := openTestDB(t)
	ds := store.NewDocumentStore(database)
	tool := NewSearchDocsTool(nil, ds)
	result, err := tool.Execute(context.Background(), map[string]any{
		"catalog": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error when group_id missing")
	}
	if result.Content != "group_id is required when catalog=true" {
		t.Errorf("unexpected message: %s", result.Content)
	}
}
