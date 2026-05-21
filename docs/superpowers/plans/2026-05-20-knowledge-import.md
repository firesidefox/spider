# Knowledge Base Import Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement document import pipeline: OpenAPI parser, Markdown LLM parser, section clustering, embedding generation, and ImportDocument orchestration.

**Architecture:** Parser layer extracts entries from OpenAPI/Markdown → LLM clusters entries into sections → Embedder generates vectors for summaries → Store writes everything atomically. Each parser is independent, ImportDocument orchestrates the pipeline.

**Tech Stack:** Go, YAML parser (gopkg.in/yaml.v3), LLM client for Markdown parsing + clustering, Embedder for vectors

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/knowledge/parser.go` | Parser interface + OpenAPI parser |
| Create | `internal/knowledge/parser_test.go` | Parser tests |
| Create | `internal/knowledge/markdown_parser.go` | LLM-driven Markdown parser |
| Create | `internal/knowledge/markdown_parser_test.go` | Markdown parser tests |
| Create | `internal/knowledge/clustering.go` | LLM-driven section clustering |
| Create | `internal/knowledge/clustering_test.go` | Clustering tests |
| Modify | `internal/knowledge/store.go` | Add ImportDocument method |
| Modify | `internal/knowledge/store_test.go` | Add ImportDocument tests |

---

### Task 1: OpenAPI Parser

**Files:**
- Create: `internal/knowledge/parser.go`
- Create: `internal/knowledge/parser_test.go`

- [ ] **Step 1: Define Parser interface and types**

Create `internal/knowledge/parser.go`:
```go
package knowledge

import (
	"context"
	"fmt"
)

// Parser extracts entries from document content.
type Parser interface {
	Parse(ctx context.Context, content []byte, filename string) ([]ParsedEntry, error)
}

// ParsedEntry represents a single extracted entry (before DB write).
type ParsedEntry struct {
	Title   string // "GET /api/v1/query"
	Summary string // One-line description
	Content string // Full content (method + path + params + response)
}

// DetectDocType returns "openapi" or "markdown" based on content/filename.
func DetectDocType(content []byte, filename string) string {
	// Check YAML structure for OpenAPI
	if len(content) > 0 && (content[0] == '{' || hasYAMLMarker(content)) {
		return "openapi"
	}
	return "markdown"
}

func hasYAMLMarker(content []byte) bool {
	// Simple heuristic: starts with "openapi:" or "swagger:"
	s := string(content[:min(200, len(content))])
	return contains(s, "openapi:") || contains(s, "swagger:")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 2: Write failing test for DetectDocType**

Create `internal/knowledge/parser_test.go`:
```go
package knowledge_test

import (
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
)

func TestDetectDocType(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		filename string
		want     string
	}{
		{
			name:     "OpenAPI YAML",
			content:  "openapi: 3.0.0\ninfo:\n  title: API",
			filename: "api.yaml",
			want:     "openapi",
		},
		{
			name:     "Swagger YAML",
			content:  "swagger: '2.0'\ninfo:\n  title: API",
			filename: "api.yml",
			want:     "openapi",
		},
		{
			name:     "OpenAPI JSON",
			content:  `{"openapi":"3.0.0","info":{"title":"API"}}`,
			filename: "api.json",
			want:     "openapi",
		},
		{
			name:     "Markdown",
			content:  "# CLI Commands\n\n## show version",
			filename: "cli.md",
			want:     "markdown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := knowledge.DetectDocType([]byte(tt.content), tt.filename)
			if got != tt.want {
				t.Errorf("DetectDocType() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./internal/knowledge -run TestDetectDocType -v`
Expected: PASS

- [ ] **Step 4: Implement OpenAPI parser**

Add to `internal/knowledge/parser.go`:
```go
import (
	"gopkg.in/yaml.v3"
)

// OpenAPIParser parses OpenAPI 2.0/3.0 specs.
type OpenAPIParser struct{}

func NewOpenAPIParser() *OpenAPIParser {
	return &OpenAPIParser{}
}

func (p *OpenAPIParser) Parse(ctx context.Context, content []byte, filename string) ([]ParsedEntry, error) {
	var spec map[string]interface{}
	if err := yaml.Unmarshal(content, &spec); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no paths found in OpenAPI spec")
	}

	var entries []ParsedEntry
	for path, pathItem := range paths {
		pathMap, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}

		for method, operation := range pathMap {
			if !isHTTPMethod(method) {
				continue
			}
			opMap, ok := operation.(map[string]interface{})
			if !ok {
				continue
			}

			entry := parseOperation(method, path, opMap)
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func isHTTPMethod(s string) bool {
	methods := []string{"get", "post", "put", "delete", "patch", "options", "head"}
	for _, m := range methods {
		if s == m {
			return true
		}
	}
	return false
}

func parseOperation(method, path string, op map[string]interface) ParsedEntry {
	title := fmt.Sprintf("%s %s", toUpper(method), path)
	
	// Extract summary
	summary := ""
	if s, ok := op["summary"].(string); ok && s != "" {
		summary = s
	} else if oid, ok := op["operationId"].(string); ok && oid != "" {
		summary = oid
	} else if desc, ok := op["description"].(string); ok && desc != "" {
		summary = truncate(desc, 100)
	}

	// Build full content (JSON representation)
	contentMap := map[string]interface{}{
		"method": toUpper(method),
		"path":   path,
	}
	for k, v := range op {
		contentMap[k] = v
	}
	
	contentBytes, _ := yaml.Marshal(contentMap)
	content := string(contentBytes)

	return ParsedEntry{
		Title:   title,
		Summary: summary,
		Content: content,
	}
}

func toUpper(s string) string {
	if len(s) == 0 {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

- [ ] **Step 5: Write test for OpenAPI parser**

Add to `internal/knowledge/parser_test.go`:
```go
func TestOpenAPIParser(t *testing.T) {
	content := `
openapi: 3.0.0
info:
  title: Test API
paths:
  /api/v1/query:
    get:
      summary: Query data
      operationId: queryData
      description: Retrieve query results
      parameters:
        - name: q
          in: query
          required: true
      responses:
        '200':
          description: Success
  /api/v1/login:
    post:
      summary: User login
      requestBody:
        required: true
      responses:
        '200':
          description: Login successful
`

	parser := knowledge.NewOpenAPIParser()
	entries, err := parser.Parse(context.Background(), []byte(content), "api.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify first entry
	if entries[0].Title != "GET /api/v1/query" {
		t.Errorf("expected title 'GET /api/v1/query', got %q", entries[0].Title)
	}
	if entries[0].Summary != "Query data" {
		t.Errorf("expected summary 'Query data', got %q", entries[0].Summary)
	}
	if !contains(entries[0].Content, "parameters") {
		t.Error("expected content to contain 'parameters'")
	}

	// Verify second entry
	if entries[1].Title != "POST /api/v1/login" {
		t.Errorf("expected title 'POST /api/v1/login', got %q", entries[1].Title)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 6: Run test**

Run: `go test ./internal/knowledge -run TestOpenAPIParser -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/knowledge/parser.go internal/knowledge/parser_test.go
git commit -m "feat(knowledge): implement OpenAPI parser"
```

### Task 2: Markdown LLM Parser

**Files:**
- Create: `internal/knowledge/markdown_parser.go`
- Create: `internal/knowledge/markdown_parser_test.go`

- [ ] **Step 1: Define Markdown parser with LLM client**

Create `internal/knowledge/markdown_parser.go`:
```go
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spiderai/spider/internal/llm"
)

// MarkdownParser uses LLM to parse Markdown into semantic entries.
type MarkdownParser struct {
	llmClient llm.Client
}

func NewMarkdownParser(llmClient llm.Client) *MarkdownParser {
	return &MarkdownParser{llmClient: llmClient}
}

func (p *MarkdownParser) Parse(ctx context.Context, content []byte, filename string) ([]ParsedEntry, error) {
	text := string(content)
	
	// Split into chunks if too long (> 8000 tokens ≈ 32000 chars)
	chunks := splitMarkdown(text, 32000)
	
	var allEntries []ParsedEntry
	for _, chunk := range chunks {
		entries, err := p.parseChunk(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("parse chunk: %w", err)
		}
		allEntries = append(allEntries, entries...)
	}
	
	return allEntries, nil
}

func (p *MarkdownParser) parseChunk(ctx context.Context, text string) ([]ParsedEntry, error) {
	prompt := buildMarkdownParsePrompt(text)
	
	resp, err := p.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.0,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}
	
	// Parse JSON response
	var result struct {
		Entries []struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Content string `json:"content"`
		} `json:"entries"`
	}
	
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	
	entries := make([]ParsedEntry, len(result.Entries))
	for i, e := range result.Entries {
		entries[i] = ParsedEntry{
			Title:   e.Title,
			Summary: e.Summary,
			Content: e.Content,
		}
	}
	
	return entries, nil
}

func buildMarkdownParsePrompt(text string) string {
	return fmt.Sprintf(`Parse this Markdown documentation into semantic entries. Each entry should be a distinct command, API endpoint, or topic.

For each entry, extract:
- title: The command name or topic (e.g., "show version", "配置接口")
- summary: One-line description (max 100 chars)
- content: Full original text for this entry (including code blocks, options, examples)

Identify semantic boundaries automatically - don't rely on heading levels. A single ## section might contain multiple entries if it covers multiple commands.

Return JSON:
{
  "entries": [
    {"title": "...", "summary": "...", "content": "..."},
    ...
  ]
}

Markdown content:
%s`, text)
}

func splitMarkdown(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}
	
	// Split by ## headings
	parts := strings.Split(text, "\n##")
	if len(parts) == 1 {
		// No ## headings, split by blank lines
		parts = strings.Split(text, "\n\n")
	}
	
	var chunks []string
	var current strings.Builder
	
	for i, part := range parts {
		// Re-add ## prefix (except first part)
		if i > 0 && !strings.HasPrefix(part, "#") {
			part = "##" + part
		}
		
		if current.Len()+len(part) > maxChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(part)
	}
	
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	
	return chunks
}
```

- [ ] **Step 2: Write test with mock LLM client**

Create `internal/knowledge/markdown_parser_test.go`:
```go
package knowledge_test

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/llm"
)

type mockLLMClient struct {
	response string
}

func (m *mockLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: m.response}, nil
}

func (m *mockLLMClient) Stream(ctx context.Context, req llm.ChatRequest, handler func(llm.StreamChunk) error) error {
	return nil
}

func (m *mockLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}

func TestMarkdownParser(t *testing.T) {
	content := `# CLI Commands

## Network Commands

### show version
Display system version information.

### show interfaces
Display interface status and statistics.
`

	mockLLM := &mockLLMClient{
		response: `{
  "entries": [
    {
      "title": "show version",
      "summary": "Display system version information",
      "content": "### show version\nDisplay system version information."
    },
    {
      "title": "show interfaces",
      "summary": "Display interface status and statistics",
      "content": "### show interfaces\nDisplay interface status and statistics."
    }
  ]
}`,
	}

	parser := knowledge.NewMarkdownParser(mockLLM)
	entries, err := parser.Parse(context.Background(), []byte(content), "cli.md")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Title != "show version" {
		t.Errorf("expected title 'show version', got %q", entries[0].Title)
	}
	if entries[0].Summary != "Display system version information" {
		t.Errorf("unexpected summary: %q", entries[0].Summary)
	}
}

func TestSplitMarkdown(t *testing.T) {
	// Test that long content gets split
	longText := strings.Repeat("## Section\nContent here.\n", 2000)
	chunks := knowledge.SplitMarkdownForTest(longText, 10000)
	
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long text, got %d", len(chunks))
	}
}
```

Also add to `markdown_parser.go` for testing:
```go
// SplitMarkdownForTest exposes splitMarkdown for testing.
func SplitMarkdownForTest(text string, maxChars int) []string {
	return splitMarkdown(text, maxChars)
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/knowledge -run TestMarkdownParser -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/knowledge/markdown_parser.go internal/knowledge/markdown_parser_test.go
git commit -m "feat(knowledge): implement LLM-driven Markdown parser"
```

### Task 3: Section Clustering

**Files:**
- Create: `internal/knowledge/clustering.go`
- Create: `internal/knowledge/clustering_test.go`

- [ ] **Step 1: Define clustering function**

Create `internal/knowledge/clustering.go`:
```go
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spiderai/spider/internal/llm"
)

// ClusterResult represents clustered sections.
type ClusterResult struct {
	Sections []ClusteredSection
}

// ClusteredSection represents a semantic section with assigned entries.
type ClusteredSection struct {
	Name     string // Chinese section name
	Summary  string // One-line description
	EntryIDs []int  // Entry indices (0-based)
}

// ClusterEntries groups entries into semantic sections using LLM.
func ClusterEntries(ctx context.Context, llmClient llm.Client, entries []ParsedEntry) (*ClusterResult, error) {
	if len(entries) == 0 {
		return &ClusterResult{}, nil
	}

	prompt := buildClusteringPrompt(entries)
	
	resp, err := llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.0,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// Parse JSON response
	var result struct {
		Sections []struct {
			Name     string `json:"name"`
			Summary  string `json:"summary"`
			EntryIDs []int  `json:"entry_ids"`
		} `json:"sections"`
	}

	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	sections := make([]ClusteredSection, len(result.Sections))
	for i, s := range result.Sections {
		sections[i] = ClusteredSection{
			Name:     s.Name,
			Summary:  s.Summary,
			EntryIDs: s.EntryIDs,
		}
	}

	return &ClusterResult{Sections: sections}, nil
}

func buildClusteringPrompt(entries []ParsedEntry) string {
	var b strings.Builder
	b.WriteString("Cluster these documentation entries into semantic sections (3-15 sections).\n\n")
	b.WriteString("Entries:\n")
	for i, e := range entries {
		fmt.Fprintf(&b, "%d. %s - %s\n", i, e.Title, e.Summary)
	}
	b.WriteString("\nFor each section, provide:\n")
	b.WriteString("- name: Chinese section name (e.g., \"认证接口\", \"查询接口\")\n")
	b.WriteString("- summary: One-line description\n")
	b.WriteString("- entry_ids: Array of entry indices (0-based)\n\n")
	b.WriteString("Return JSON:\n")
	b.WriteString(`{
  "sections": [
    {"name": "...", "summary": "...", "entry_ids": [0, 1, 2]},
    ...
  ]
}`)
	return b.String()
}
```

- [ ] **Step 2: Write test with mock LLM**

Create `internal/knowledge/clustering_test.go`:
```go
package knowledge_test

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
)

func TestClusterEntries(t *testing.T) {
	entries := []knowledge.ParsedEntry{
		{Title: "POST /login", Summary: "User login"},
		{Title: "POST /logout", Summary: "User logout"},
		{Title: "GET /query", Summary: "Query data"},
		{Title: "GET /metrics", Summary: "Get metrics"},
	}

	mockLLM := &mockLLMClient{
		response: `{
  "sections": [
    {
      "name": "认证接口",
      "summary": "用户认证相关接口",
      "entry_ids": [0, 1]
    },
    {
      "name": "查询接口",
      "summary": "数据查询和监控接口",
      "entry_ids": [2, 3]
    }
  ]
}`,
	}

	result, err := knowledge.ClusterEntries(context.Background(), mockLLM, entries)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result.Sections))
	}

	// Verify first section
	if result.Sections[0].Name != "认证接口" {
		t.Errorf("expected name '认证接口', got %q", result.Sections[0].Name)
	}
	if len(result.Sections[0].EntryIDs) != 2 {
		t.Errorf("expected 2 entry IDs, got %d", len(result.Sections[0].EntryIDs))
	}
	if result.Sections[0].EntryIDs[0] != 0 || result.Sections[0].EntryIDs[1] != 1 {
		t.Errorf("unexpected entry IDs: %v", result.Sections[0].EntryIDs)
	}

	// Verify second section
	if result.Sections[1].Name != "查询接口" {
		t.Errorf("expected name '查询接口', got %q", result.Sections[1].Name)
	}
}

func TestClusterEntriesEmpty(t *testing.T) {
	mockLLM := &mockLLMClient{}
	result, err := knowledge.ClusterEntries(context.Background(), mockLLM, []knowledge.ParsedEntry{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sections) != 0 {
		t.Errorf("expected 0 sections for empty input, got %d", len(result.Sections))
	}
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/knowledge -run TestCluster -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/knowledge/clustering.go internal/knowledge/clustering_test.go
git commit -m "feat(knowledge): implement LLM-driven section clustering"
```

### Task 4: ImportDocument Implementation

**Files:**
- Modify: `internal/knowledge/store.go` (add ImportDocument method)
- Modify: `internal/knowledge/store_test.go` (add ImportDocument test)

- [ ] **Step 1: Add helper to update document entry count**

Add to `internal/knowledge/store.go` after `DeleteDocuments`:
```go
func (s *Store) updateDocumentEntryCount(ctx context.Context, docID, count int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET entry_count = ?, updated_at = datetime('now') WHERE id = ?`,
		count, docID)
	return err
}

func (s *Store) setDocumentStatus(ctx context.Context, docID int, status, errorMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE knowledge_documents SET status = ?, error_msg = ?, updated_at = datetime('now') WHERE id = ?`,
		status, errorMsg, docID)
	return err
}
```

- [ ] **Step 2: Implement ImportDocument**

Add to `internal/knowledge/store.go` after helper methods:
```go
func (s *Store) ImportDocument(ctx context.Context, req ImportRequest) (*ImportResult, error) {
	// Create document record
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge_documents 
		(group_id, name, doc_type, raw_content, filename, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.GroupID, req.Name, req.DocType, string(req.Content), req.Filename, "indexing", now, now)
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	docID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get document id: %w", err)
	}

	// Parse entries
	var parser Parser
	switch req.DocType {
	case "openapi":
		parser = NewOpenAPIParser()
	case "markdown":
		if req.LLMClient == nil {
			return nil, fmt.Errorf("LLM client required for markdown parsing")
		}
		parser = NewMarkdownParser(req.LLMClient)
	default:
		return nil, fmt.Errorf("unsupported doc type: %s", req.DocType)
	}

	entries, err := parser.Parse(ctx, req.Content, req.Filename)
	if err != nil {
		s.setDocumentStatus(ctx, int(docID), "error", err.Error())
		return nil, fmt.Errorf("parse document: %w", err)
	}

	if len(entries) == 0 {
		s.setDocumentStatus(ctx, int(docID), "ready", "")
		return &ImportResult{DocumentID: int(docID)}, nil
	}

	// Cluster into sections
	var clusterResult *ClusterResult
	if req.LLMClient != nil {
		clusterResult, err = ClusterEntries(ctx, req.LLMClient, entries)
		if err != nil {
			s.setDocumentStatus(ctx, int(docID), "error", err.Error())
			return nil, fmt.Errorf("cluster entries: %w", err)
		}
	} else {
		// No clustering - create single section
		clusterResult = &ClusterResult{
			Sections: []ClusteredSection{
				{
					Name:     "All Entries",
					Summary:  "All documentation entries",
					EntryIDs: makeRange(0, len(entries)),
				},
			},
		}
	}

	// Create sections and entries
	var sections []Section
	for i, cs := range clusterResult.Sections {
		sec, err := s.CreateSection(ctx, int(docID), cs.Name, cs.Summary, i)
		if err != nil {
			s.setDocumentStatus(ctx, int(docID), "error", err.Error())
			return nil, fmt.Errorf("create section: %w", err)
		}
		sections = append(sections, *sec)

		// Create entries in this section
		for pos, entryIdx := range cs.EntryIDs {
			if entryIdx >= len(entries) {
				continue
			}
			e := entries[entryIdx]

			// Generate embedding if embedder provided
			var embedding []byte
			if req.Embedder != nil {
				emb, err := req.Embedder.Embed(ctx, e.Summary)
				if err != nil {
					// Log but don't fail
					embedding = nil
				} else {
					embedding = float32SliceToBytes(emb)
				}
			}

			_, err := s.CreateEntry(ctx, int(docID), &sec.ID, e.Title, e.Summary, e.Content, embedding, pos)
			if err != nil {
				s.setDocumentStatus(ctx, int(docID), "error", err.Error())
				return nil, fmt.Errorf("create entry: %w", err)
			}
		}
	}

	// Update document status and entry count
	if err := s.updateDocumentEntryCount(ctx, int(docID), len(entries)); err != nil {
		return nil, fmt.Errorf("update entry count: %w", err)
	}
	if err := s.setDocumentStatus(ctx, int(docID), "ready", ""); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}

	return &ImportResult{
		DocumentID:   int(docID),
		EntryCount:   len(entries),
		SectionCount: len(sections),
		Sections:     sections,
	}, nil
}

func makeRange(start, end int) []int {
	result := make([]int, end-start)
	for i := range result {
		result[i] = start + i
	}
	return result
}

func float32SliceToBytes(floats []float32) []byte {
	bytes := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		offset := i * 4
		bytes[offset] = byte(bits)
		bytes[offset+1] = byte(bits >> 8)
		bytes[offset+2] = byte(bits >> 16)
		bytes[offset+3] = byte(bits >> 24)
	}
	return bytes
}
```

Update imports:
```go
import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/rag"
)
```

- [ ] **Step 3: Update ImportRequest type in plugin.go**

Edit `internal/knowledge/plugin.go` - update ImportRequest:
```go
// ImportRequest specifies parameters for importing a document.
type ImportRequest struct {
	GroupID    int
	Name       string
	Content    []byte
	Filename   string
	DocType    string // "openapi" | "markdown"
	LLMClient  llm.Client
	Embedder   rag.Embedder
}
```

- [ ] **Step 4: Write integration test**

Add to `internal/knowledge/store_test.go`:
```go
func TestImportDocument(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	// OpenAPI content
	content := []byte(`
openapi: 3.0.0
info:
  title: Test API
paths:
  /api/v1/login:
    post:
      summary: User login
      responses:
        '200':
          description: Success
  /api/v1/query:
    get:
      summary: Query data
      responses:
        '200':
          description: Success
`)

	// Mock LLM for clustering
	mockLLM := &mockLLMClient{
		response: `{
  "sections": [
    {"name": "认证接口", "summary": "用户认证", "entry_ids": [0]},
    {"name": "查询接口", "summary": "数据查询", "entry_ids": [1]}
  ]
}`,
	}

	// Mock embedder
	mockEmb := &mockEmbedder{embedding: []float32{1.0, 0.0, 0.0, 0.0}}

	result, err := s.ImportDocument(ctx, knowledge.ImportRequest{
		GroupID:   g.ID,
		Name:      "Test API",
		Content:   content,
		Filename:  "api.yaml",
		DocType:   "openapi",
		LLMClient: mockLLM,
		Embedder:  mockEmb,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify result
	if result.EntryCount != 2 {
		t.Errorf("expected 2 entries, got %d", result.EntryCount)
	}
	if result.SectionCount != 2 {
		t.Errorf("expected 2 sections, got %d", result.SectionCount)
	}

	// Verify document status
	doc, err := s.GetDocument(ctx, result.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", doc.Status)
	}
	if doc.EntryCount != 2 {
		t.Errorf("expected entry_count 2, got %d", doc.EntryCount)
	}

	// Verify sections created
	sections, err := s.ListSections(ctx, result.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].Name != "认证接口" {
		t.Errorf("expected section name '认证接口', got %q", sections[0].Name)
	}

	// Verify entries created with embeddings
	entries, err := s.ListEntries(ctx, result.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "POST /api/v1/login" {
		t.Errorf("unexpected entry title: %q", entries[0].Title)
	}
	if len(entries[0].Embedding) == 0 {
		t.Error("expected embedding to be set")
	}
}

func TestImportDocumentMarkdown(t *testing.T) {
	db := newTestDB(t)
	s := knowledge.NewStore(db)
	ctx := context.Background()

	kb, _ := s.CreateKB(ctx, "AISG")
	g, _ := s.CreateGroup(ctx, kb.ID, "v706")

	content := []byte(`# CLI Commands

## show version
Display system version.

## show interfaces
Display interface status.
`)

	mockLLM := &mockLLMClient{
		response: `{
  "entries": [
    {"title": "show version", "summary": "Display system version", "content": "## show version\nDisplay system version."},
    {"title": "show interfaces", "summary": "Display interface status", "content": "## show interfaces\nDisplay interface status."}
  ]
}`,
	}

	result, err := s.ImportDocument(ctx, knowledge.ImportRequest{
		GroupID:   g.ID,
		Name:      "CLI Guide",
		Content:   content,
		Filename:  "cli.md",
		DocType:   "markdown",
		LLMClient: mockLLM,
		Embedder:  nil, // No embedder
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.EntryCount != 2 {
		t.Errorf("expected 2 entries, got %d", result.EntryCount)
	}

	// Verify entries have no embeddings
	entries, err := s.ListEntries(ctx, result.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Embedding != nil {
		t.Error("expected no embedding when embedder not provided")
	}
}
```

- [ ] **Step 5: Run test**

Run: `go test ./internal/knowledge -run TestImportDocument -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/knowledge/store.go internal/knowledge/store_test.go internal/knowledge/plugin.go
git commit -m "feat(knowledge): implement ImportDocument with parsing and clustering"
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ OpenAPI parser (path+method extraction)
- ✅ Markdown LLM parser (semantic boundary detection)
- ✅ Section clustering (LLM-driven)
- ✅ Embedding generation (via Embedder interface)
- ✅ ImportDocument orchestration
- ✅ Document status tracking (pending/indexing/ready/error)
- ✅ Entry count updates

**Placeholder scan:**
- ✅ No TBD/TODO
- ✅ All code blocks complete
- ✅ All test assertions specific
- ✅ All commands have expected output

**Type consistency:**
- ✅ `ParsedEntry` used consistently across parsers
- ✅ `ClusteredSection` matches clustering output
- ✅ `ImportRequest` has all required fields
- ✅ `ImportResult` matches return structure

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-20-knowledge-import.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?

