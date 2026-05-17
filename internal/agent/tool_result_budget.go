package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/spiderai/spider/internal/llm"
)

const previewMaxChars = 2000

func toolResultDir(dataDir, convID string) string {
	return filepath.Join(dataDir, "tool-results", convID)
}

// persistToolResult writes content to {dataDir}/tool-results/{convID}/{toolUseID}.txt.
func persistToolResult(dataDir, convID, toolUseID, content string) (string, error) {
	dir := toolResultDir(dataDir, convID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir tool-results: %w", err)
	}
	path := filepath.Join(dir, toolUseID+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write tool result: %w", err)
	}
	return path, nil
}

// generatePreview builds the replacement string shown to the LLM.
// For content <= previewMaxChars, returns content unchanged.
func generatePreview(content, filePath string) string {
	if len(content) <= previewMaxChars {
		return content
	}
	cut := content[:previewMaxChars]
	if idx := strings.LastIndexByte(cut, '\n'); idx > 0 {
		cut = cut[:idx]
	}
	return fmt.Sprintf(
		"[Output too large: %d chars. Full output saved to: %s]\n\nPreview (first 2000 chars):\n%s\n...",
		len(content), filePath, cut,
	)
}

// ContentReplacementState freezes tool result replacement decisions so that
// the same tool_use_id always maps to the same preview across history rebuilds.
type ContentReplacementState struct {
	mu           sync.Mutex
	replacements map[string]string
	seen         map[string]bool
}

func newContentReplacementState() *ContentReplacementState {
	return &ContentReplacementState{
		replacements: make(map[string]string),
		seen:         make(map[string]bool),
	}
}

func (s *ContentReplacementState) setReplacement(toolUseID, preview string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.replacements[toolUseID]; !exists {
		s.replacements[toolUseID] = preview
	}
}

func (s *ContentReplacementState) getReplacement(toolUseID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replacements[toolUseID]
}

func (s *ContentReplacementState) markSeen(toolUseID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[toolUseID] = true
}

func (s *ContentReplacementState) isSeen(toolUseID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inReplacements := s.replacements[toolUseID]
	return inReplacements || s.seen[toolUseID]
}

// enforcePerMessageBudget applies the per-message aggregate limit to a slice of
// tool_result ContentBlocks. Blocks already in state are reused; fresh blocks
// that push total over maxChars are persisted and replaced, largest first.
func enforcePerMessageBudget(
	blocks []llm.ContentBlock,
	maxChars int,
	dataDir, convID string,
	state *ContentReplacementState,
) []llm.ContentBlock {
	if maxChars <= 0 {
		return blocks
	}

	out := make([]llm.ContentBlock, len(blocks))
	copy(out, blocks)

	type freshEntry struct {
		idx  int
		size int
	}
	var fresh []freshEntry
	total := 0

	for i, b := range out {
		if b.Type != "tool_result" {
			continue
		}
		if prev := state.getReplacement(b.ToolUseID); prev != "" {
			out[i].Content = prev
			total += len(prev)
			continue
		}
		if state.isSeen(b.ToolUseID) {
			total += len(b.Content)
			continue
		}
		fresh = append(fresh, freshEntry{i, len(b.Content)})
		total += len(b.Content)
	}

	if total <= maxChars {
		for _, f := range fresh {
			state.markSeen(out[f.idx].ToolUseID)
		}
		return out
	}

	sort.Slice(fresh, func(a, b int) bool { return fresh[a].size > fresh[b].size })

	for _, f := range fresh {
		if total <= maxChars {
			break
		}
		b := &out[f.idx]
		filePath, err := persistToolResult(dataDir, convID, b.ToolUseID, b.Content)
		if err != nil {
			state.markSeen(b.ToolUseID)
			continue
		}
		preview := generatePreview(b.Content, filePath)
		state.setReplacement(b.ToolUseID, preview)
		total -= len(b.Content)
		total += len(preview)
		b.Content = preview
	}

	for _, f := range fresh {
		if !state.isSeen(out[f.idx].ToolUseID) {
			state.markSeen(out[f.idx].ToolUseID)
		}
	}

	return out
}
