package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/rag"
)

type SearchDocsTool struct {
	ragStore *rag.Store
}

func NewSearchDocsTool(ragStore *rag.Store) *SearchDocsTool {
	return &SearchDocsTool{ragStore: ragStore}
}

func (t *SearchDocsTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *SearchDocsTool) Name() string                  { return "SearchDocs" }

func (t *SearchDocsTool) Description() string {
	return "Search documentation for CLI commands, API references, and troubleshooting guides. Read-only. No side effects. Use freely in Explore phase."
}

func (t *SearchDocsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":    map[string]any{"type": "string", "description": "Search query"},
			"vendor":   map[string]any{"type": "string", "description": "Device vendor (e.g. huawei, cisco)"},
			"cli_type": map[string]any{"type": "string", "description": "CLI type (e.g. vrp, ios, junos)"},
		},
		"required": []string{"query"},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return &ToolResult{Content: "query is required", IsError: true, RiskLevel: RiskL1}, nil
	}
	vendor, _ := input["vendor"].(string)
	cliType, _ := input["cli_type"].(string)

	docs, err := t.ragStore.SearchWithCLIType(ctx, query, vendor, cliType, 5)
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
	return &ToolResult{Content: string(b), RiskLevel: RiskL1}, nil
}
