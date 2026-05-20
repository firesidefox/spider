package knowledge

import (
	"context"
	"testing"
)

func TestDetectDocType(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		filename string
		want     DocType
	}{
		{
			name: "OpenAPI YAML",
			content: `openapi: 3.0.0
info:
  title: Test API
paths:
  /test:
    get:
      summary: Test endpoint`,
			filename: "api.yaml",
			want:     DocTypeOpenAPI,
		},
		{
			name: "Swagger YAML",
			content: `swagger: "2.0"
info:
  title: Test API
paths:
  /test:
    get:
      summary: Test endpoint`,
			filename: "api.yml",
			want:     DocTypeOpenAPI,
		},
		{
			name: "OpenAPI JSON",
			content: `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API"
  },
  "paths": {}
}`,
			filename: "api.json",
			want:     DocTypeOpenAPI,
		},
		{
			name: "Markdown",
			content: `# Test Document

This is a markdown file.`,
			filename: "README.md",
			want:     DocTypeMarkdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDocType([]byte(tt.content), tt.filename)
			if got != tt.want {
				t.Errorf("DetectDocType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenAPIParser(t *testing.T) {
	parser := &OpenAPIParser{}
	ctx := context.Background()

	content := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /api/v1/query:
    get:
      operationId: queryData
      summary: Query data from the system
      description: Retrieves data based on query parameters
      responses:
        '200':
          description: Success
    post:
      summary: Create a new query
      responses:
        '201':
          description: Created
`

	entries, err := parser.Parse(ctx, []byte(content), "api.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Parse() returned %d entries, want 2", len(entries))
	}

	// Check first entry (GET)
	if entries[0].Title != "GET /api/v1/query" {
		t.Errorf("entries[0].Title = %q, want %q", entries[0].Title, "GET /api/v1/query")
	}
	if entries[0].Summary != "queryData" {
		t.Errorf("entries[0].Summary = %q, want %q", entries[0].Summary, "queryData")
	}
	if entries[0].Content == "" {
		t.Error("entries[0].Content is empty")
	}

	// Check second entry (POST)
	if entries[1].Title != "POST /api/v1/query" {
		t.Errorf("entries[1].Title = %q, want %q", entries[1].Title, "POST /api/v1/query")
	}
	if entries[1].Summary != "Create a new query" {
		t.Errorf("entries[1].Summary = %q, want %q", entries[1].Summary, "Create a new query")
	}
	if entries[1].Content == "" {
		t.Error("entries[1].Content is empty")
	}
}
