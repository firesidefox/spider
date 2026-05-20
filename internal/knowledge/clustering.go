package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spiderai/spider/internal/llm"
)

// ClusteredSection is a named group of entry indices.
type ClusteredSection struct {
	Name     string `json:"name"`
	Summary  string `json:"summary"`
	EntryIDs []int  `json:"entry_ids"`
}

// ClusterResult holds the output of LLM-driven clustering.
type ClusterResult struct {
	Sections []ClusteredSection
}

// clusterResponse is the JSON shape expected from the LLM.
type clusterResponse struct {
	Sections []struct {
		Name     string `json:"name"`
		Summary  string `json:"summary"`
		EntryIDs []int  `json:"entry_ids"`
	} `json:"sections"`
}

// ClusterEntries groups entries into semantic sections using the LLM.
// Returns an empty ClusterResult (no LLM call) when entries is empty.
func ClusterEntries(ctx context.Context, client llm.Client, entries []ParsedEntry) (*ClusterResult, error) {
	if len(entries) == 0 {
		return &ClusterResult{}, nil
	}

	prompt := buildClusteringPrompt(entries)
	req := &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens: 2048,
	}

	raw, err := client.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clustering LLM call failed: %w", err)
	}

	// Strip markdown code fences if present.
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw, "\n"); idx != -1 {
			raw = raw[idx+1:]
		}
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	var resp clusterResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("clustering: invalid LLM JSON response: %w", err)
	}

	result := &ClusterResult{
		Sections: make([]ClusteredSection, 0, len(resp.Sections)),
	}
	for _, s := range resp.Sections {
		result.Sections = append(result.Sections, ClusteredSection{
			Name:     s.Name,
			Summary:  s.Summary,
			EntryIDs: s.EntryIDs,
		})
	}
	return result, nil
}

// buildClusteringPrompt constructs the prompt sent to the LLM.
func buildClusteringPrompt(entries []ParsedEntry) string {
	var sb strings.Builder
	sb.WriteString("You are a technical documentation organizer.\n\n")
	sb.WriteString("Below is a numbered list of document entries (index. Title - Summary):\n\n")
	for i, e := range entries {
		fmt.Fprintf(&sb, "%d. %s - %s\n", i, e.Title, e.Summary)
	}
	sb.WriteString("\nGroup these entries into 3–15 semantic sections with Chinese names.\n")
	sb.WriteString("Return ONLY valid JSON in this exact format, no markdown fences:\n")
	sb.WriteString(`{"sections": [{"name": "认证接口", "summary": "...", "entry_ids": [0, 1, 2]}]}`)
	sb.WriteString("\n\nRules:\n")
	sb.WriteString("- Every entry index must appear in exactly one section.\n")
	sb.WriteString("- Section names must be in Chinese.\n")
	sb.WriteString("- Output only the JSON object, nothing else.\n")
	return sb.String()
}
