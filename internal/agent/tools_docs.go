package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/store"
)

type SearchDocsTool struct {
	ragStore *rag.Store
	docStore *store.DocumentStore
}

func NewSearchDocsTool(ragStore *rag.Store, docStore *store.DocumentStore) *SearchDocsTool {
	return &SearchDocsTool{ragStore: ragStore, docStore: docStore}
}

func (t *SearchDocsTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *SearchDocsTool) Name() string                { return "SearchDocs" }

func (t *SearchDocsTool) Description() string {
	return "Search documentation for CLI commands, API references, and troubleshooting guides. Read-only. No side effects. Use freely in Explore phase."
}

func (t *SearchDocsTool) SystemPromptSection() string {
	return `## SearchDocs — Knowledge Base

**When to use:**
- Before running vendor-specific CLI commands — syntax varies by vendor/version
- Before calling any API endpoint — need correct path, params, auth
- When troubleshooting an unfamiliar error or behavior

**When NOT to use:**
- Universal commands (df -h, ps aux, ls, grep) — no need to look these up
- Purely informational tasks (listing hosts, checking task status)

**Rules:**
- Query with operation intent, not just keywords (e.g., "huawei 查看内存占用" not "memory")
- For API calls: get group_id from face.knowledge_sources, then SearchDocs to find the endpoint

**For full-text documents (no embedding):**
1. Call SearchDocs with catalog=true and group_id to list available documents (ID + title).
2. Pick relevant documents by ID.
3. Call SearchDocs with doc_ids=[...] to fetch full content.`
}

func (t *SearchDocsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":     map[string]any{"type": "string", "description": "Search query"},
			"vendor":    map[string]any{"type": "string", "description": "Device vendor (e.g. huawei, cisco)"},
			"group_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Search within these document groups. Get from face.knowledge_sources where type=group."},
			"doc_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Fetch full content of specific documents by IDs. Get from face.knowledge_sources where type=doc."},
			"catalog":   map[string]any{"type": "boolean", "description": "List document titles in a group without fetching full content. Use with group_id to browse available documents before deciding which to read."},
			"group_id":  map[string]any{"type": "integer", "description": "Group ID to list when catalog=true."},
		},
		"required": []string{},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	if catalog, _ := input["catalog"].(bool); catalog {
		if t.docStore == nil {
			return &ToolResult{Content: "doc store unavailable", IsError: true, RiskLevel: RiskL1}, nil
		}
		groupID := toInt(input["group_id"])
		if groupID == 0 {
			return &ToolResult{Content: "group_id is required when catalog=true", IsError: true, RiskLevel: RiskL1}, nil
		}
		docs, err := t.docStore.ListByGroup(groupID)
		if err != nil {
			return &ToolResult{Content: fmt.Sprintf("list group: %v", err), IsError: true, RiskLevel: RiskL1}, nil
		}
		type entry struct {
			ID         int    `json:"id"`
			Title      string `json:"title"`
			SourceFile string `json:"source_file"`
		}
		entries := make([]entry, len(docs))
		for i, d := range docs {
			entries[i] = entry{ID: d.ID, Title: d.Title, SourceFile: d.SourceFile}
		}
		b, _ := json.Marshal(entries)
		return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d results", len(entries))}, nil
	}

	query, _ := input["query"].(string)
	if query == "" {
		return &ToolResult{Content: "query is required", IsError: true, RiskLevel: RiskL1}, nil
	}

	// doc_ids: fetch multiple documents, skip vector search
	if docIDsRaw, ok := input["doc_ids"]; ok && docIDsRaw != nil {
		docIDs := toIntSlice(docIDsRaw)
		if len(docIDs) > 0 && t.docStore != nil {
			type result struct {
				Title      string   `json:"title"`
				Content    string   `json:"content"`
				Tags       []string `json:"tags"`
				SourceFile string   `json:"source_file"`
			}
			results := make([]result, 0, len(docIDs))
			for _, id := range docIDs {
				doc, err := t.docStore.GetByID(id)
				if err != nil {
					return &ToolResult{Content: fmt.Sprintf("get document %d: %v", id, err), IsError: true, RiskLevel: RiskL1}, nil
				}
				if doc == nil {
					continue
				}
				results = append(results, result{
					Title:      doc.Title,
					Content:    doc.Content,
					Tags:       doc.Tags,
					SourceFile: doc.SourceFile,
				})
			}
			b, _ := json.Marshal(results)
			return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d results", len(results))}, nil
		}
	}

	if t.ragStore == nil {
		return &ToolResult{Content: "search unavailable: embedding not configured", IsError: true, RiskLevel: RiskL1}, nil
	}

	// group_ids: search within multiple groups
	var groupIDs []int
	if gidsRaw, ok := input["group_ids"]; ok && gidsRaw != nil {
		groupIDs = toIntSlice(gidsRaw)
	}

	docs, err := t.ragStore.SearchByGroups(ctx, query, groupIDs, 5)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("search error: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	type result struct {
		Title      string   `json:"title"`
		Content    string   `json:"content"`
		Tags       []string `json:"tags"`
		SourceFile string   `json:"source_file"`
	}
	results := make([]result, 0, len(docs))
	for _, d := range docs {
		results = append(results, result{
			Title:      d.Title,
			Content:    d.Content,
			Tags:       d.Tags,
			SourceFile: d.SourceFile,
		})
	}
	b, err := json.Marshal(results)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("marshal error: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}
	return &ToolResult{Content: string(b), RiskLevel: RiskL1, Summary: fmt.Sprintf("found %d results", len(results))}, nil
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
