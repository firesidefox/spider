package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const previewMaxChars = 2000

// persistToolResult writes content to {dataDir}/tool-results/{convID}/{toolUseID}.txt.
func persistToolResult(dataDir, convID, toolUseID, content string) (string, error) {
	dir := filepath.Join(dataDir, "tool-results", convID)
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
	if idx := lastNewline(cut); idx > 0 {
		cut = cut[:idx]
	}
	return fmt.Sprintf(
		"[Output too large: %d chars. Full output saved to: %s]\n\nPreview (first 2000 chars):\n%s\n...",
		len(content), filePath, cut,
	)
}

func lastNewline(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
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
