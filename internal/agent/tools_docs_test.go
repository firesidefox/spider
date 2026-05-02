package agent

import (
	"testing"
)

func TestSearchDocsTool_Metadata(t *testing.T) {
	tool := NewSearchDocsTool(nil)

	if tool.Name() != "search_docs" {
		t.Errorf("got name %q, want %q", tool.Name(), "search_docs")
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
	for _, key := range []string{"query", "vendor", "cli_type"} {
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
	var _ Tool = NewSearchDocsTool(nil)
}

func TestCallRESTAPITool_Metadata(t *testing.T) {
	tool := NewCallRESTAPITool()

	if tool.Name() != "call_rest_api" {
		t.Errorf("got name %q, want %q", tool.Name(), "call_rest_api")
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
	var _ Tool = NewCallRESTAPITool()
}
