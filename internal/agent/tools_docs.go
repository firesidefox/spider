package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/rag"
)

type SearchDocsTool struct {
	knowledgeStore *knowledge.Store
	embedder       rag.Embedder
}

func NewSearchDocsTool(knowledgeStore *knowledge.Store, embedder rag.Embedder) *SearchDocsTool {
	return &SearchDocsTool{
		knowledgeStore: knowledgeStore,
		embedder:       embedder,
	}
}

func (t *SearchDocsTool) DefaultRiskLevel() RiskLevel             { return RiskL1 }
func (t *SearchDocsTool) IsConcurrencySafe(_ map[string]any) bool { return true }
func (t *SearchDocsTool) Name() string                            { return "SearchDocs" }

func (t *SearchDocsTool) Description() string {
	return "Search knowledge base for API endpoints, CLI commands, and documentation. Read-only. No side effects. Use freely in Explore phase."
}

func (t *SearchDocsTool) SystemPromptSection() string {
	return `## SearchDocs — Hierarchical Knowledge Retrieval

**When to use:**
- Before calling any API endpoint — need correct path, params, auth
- Before running vendor-specific CLI commands — syntax varies by vendor/version
- When troubleshooting unfamiliar errors or behaviors

**When NOT to use:**
- Universal commands (df, ps, ls, grep) — no need to look these up
- Purely informational tasks (listing hosts, checking task status)

**Four modes:**

1. **sections** — List chapters in a knowledge scope (kb/group/document)
   - Returns: [{section_id, name, summary, entry_count}]
   - Use: Get overview of available topics

2. **entries** — List entries in a section
   - Returns: [{entry_id, title, summary}]
   - Use: Browse specific chapter contents

3. **fetch** — Get full content of specific entries
   - Returns: [{title, content}]
   - Use: Read the actual documentation

4. **search** — Vector search when catalog navigation fails
   - Returns: [{title, content}] (top-K matches)
   - Use: When entry_count ≥ 500 or catalog doesn't help

**Face KB Bindings**

Each access face has kb_mode and knowledge_sources. When kb_mode='specific',
the face exposes one or more KB scopes. Each entry includes name for groups
or title + group_name for documents.

When solving tasks for selected hosts, prefer SearchDocs scoped to bound sources.
Multiple sources may bind to different scopes; call SearchDocs separately per scope as needed.

- type=group -> scope_type=group, scope_id=source.id
- type=doc -> scope_type=document, scope_id=source.id
- kb_mode=none -> no binding signal

**Typical workflow:**
1. Get scope from face where kb_mode='specific'
2. SearchDocs mode=sections with scope_type/scope_id from the bound source
3. Pick relevant section_id from results
4. SearchDocs mode=entries, section_id=N
5. Pick relevant entry_ids
6. SearchDocs mode=fetch, entry_ids=[...]

**Fallback to search:**
- If sections returns total_entries ≥ 500, use mode=search instead
- If catalog navigation doesn't find what you need, use mode=search`
}

func (t *SearchDocsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"sections", "entries", "fetch", "search"},
				"description": "Operation mode",
			},
			"scope_type": map[string]any{
				"type":        "string",
				"enum":        []string{"group", "document"},
				"description": "Scope type for sections/search mode",
			},
			"scope_id": map[string]any{
				"type":        "integer",
				"description": "Scope ID for sections/search mode",
			},
			"section_id": map[string]any{
				"type":        "integer",
				"description": "Section ID for entries mode",
			},
			"entry_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer"},
				"description": "Entry IDs for fetch mode",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query for search mode",
			},
		},
		"required": []string{"mode"},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	if t.knowledgeStore == nil {
		return &ToolResult{Content: "knowledge base not configured", IsError: true, RiskLevel: RiskL1}, nil
	}
	mode, _ := input["mode"].(string)
	if mode == "" {
		return &ToolResult{Content: "mode is required", IsError: true, RiskLevel: RiskL1}, nil
	}

	switch mode {
	case "sections":
		return t.executeSections(ctx, input)
	case "entries":
		return t.executeEntries(ctx, input)
	case "fetch":
		return t.executeFetch(ctx, input)
	case "search":
		return t.executeSearch(ctx, input)
	default:
		return &ToolResult{Content: fmt.Sprintf("invalid mode: %s", mode), IsError: true, RiskLevel: RiskL1}, nil
	}
}

func (t *SearchDocsTool) executeSections(ctx context.Context, input map[string]any) (*ToolResult, error) {
	scopeType, _ := input["scope_type"].(string)
	scopeID := toInt(input["scope_id"])
	if scopeType == "" || scopeID == 0 {
		return &ToolResult{Content: "scope_type and scope_id are required for sections mode", IsError: true, RiskLevel: RiskL1}, nil
	}

	sections, err := t.knowledgeStore.CatalogSections(ctx, knowledge.Scope{Type: scopeType, ID: scopeID})
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("catalog sections: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		SectionID  int    `json:"section_id"`
		Name       string `json:"name"`
		Summary    string `json:"summary"`
		EntryCount int    `json:"entry_count"`
	}
	results := make([]result, len(sections))
	totalEntries := 0
	for i, s := range sections {
		results[i] = result{
			SectionID:  s.ID,
			Name:       s.Name,
			Summary:    s.Summary,
			EntryCount: s.EntryCount,
		}
		totalEntries += s.EntryCount
	}

	b, _ := json.Marshal(map[string]any{
		"sections":      results,
		"total_entries": totalEntries,
	})

	summary := fmt.Sprintf("found %d sections, %d total entries", len(sections), totalEntries)
	if totalEntries >= 500 {
		summary += " (consider using mode=search for large result sets)"
	}

	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: summary}, nil
}

func (t *SearchDocsTool) executeEntries(ctx context.Context, input map[string]any) (*ToolResult, error) {
	sectionID := toInt(input["section_id"])
	if sectionID == 0 {
		return &ToolResult{Content: "section_id is required for entries mode", IsError: true, RiskLevel: RiskL1}, nil
	}

	entries, err := t.knowledgeStore.CatalogEntries(ctx, sectionID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("catalog entries: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		EntryID int    `json:"entry_id"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	results := make([]result, len(entries))
	for i, e := range entries {
		results[i] = result{EntryID: e.ID, Title: e.Title, Summary: e.Summary}
	}

	b, _ := json.Marshal(results)
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d entries", len(entries))}, nil
}

func (t *SearchDocsTool) executeFetch(ctx context.Context, input map[string]any) (*ToolResult, error) {
	entryIDsRaw, ok := input["entry_ids"]
	if !ok {
		return &ToolResult{Content: "entry_ids is required for fetch mode", IsError: true, RiskLevel: RiskL1}, nil
	}
	entryIDs := toIntSlice(entryIDsRaw)
	if len(entryIDs) == 0 {
		return &ToolResult{Content: "entry_ids cannot be empty", IsError: true, RiskLevel: RiskL1}, nil
	}

	entries, err := t.knowledgeStore.FetchEntries(ctx, entryIDs)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("fetch entries: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	results := make([]result, len(entries))
	for i, e := range entries {
		results[i] = result{Title: e.Title, Content: e.Content}
	}

	b, _ := json.Marshal(results)
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("fetched %d entries", len(entries))}, nil
}

func (t *SearchDocsTool) executeSearch(ctx context.Context, input map[string]any) (*ToolResult, error) {
	query, _ := input["query"].(string)
	scopeType, _ := input["scope_type"].(string)
	scopeID := toInt(input["scope_id"])

	if query == "" || scopeType == "" || scopeID == 0 {
		return &ToolResult{Content: "query, scope_type, and scope_id are required for search mode", IsError: true, RiskLevel: RiskL1}, nil
	}

	if t.embedder == nil {
		return &ToolResult{Content: "search unavailable: embedder not configured", IsError: true, RiskLevel: RiskL1}, nil
	}

	entries, err := t.knowledgeStore.Search(ctx, query, knowledge.Scope{Type: scopeType, ID: scopeID}, 5, t.embedder)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("search: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	results := make([]result, len(entries))
	for i, e := range entries {
		results[i] = result{Title: e.Title, Content: e.Content}
	}

	b, _ := json.Marshal(results)
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d results", len(entries))}, nil
}

// toInt converts float64 (JSON number) or int to int.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// toIntSlice converts []any (JSON array) to []int.
func toIntSlice(v any) []int {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]int, 0, len(arr))
	for _, item := range arr {
		if n := toInt(item); n > 0 {
			result = append(result, n)
		}
	}
	return result
}
