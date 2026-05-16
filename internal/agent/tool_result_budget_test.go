package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/llm"
)

func TestGeneratePreview_ShortContent(t *testing.T) {
	content := "hello world"
	preview := generatePreview(content, "/tmp/fake.txt")
	if strings.Contains(preview, "too large") {
		t.Errorf("short content should not get truncation header, got: %s", preview)
	}
}

func TestGeneratePreview_LongContent(t *testing.T) {
	content := strings.Repeat("x", 3000)
	preview := generatePreview(content, "/tmp/fake.txt")
	if !strings.Contains(preview, "too large") {
		t.Errorf("expected truncation header, got: %s", preview[:100])
	}
	if !strings.Contains(preview, "/tmp/fake.txt") {
		t.Errorf("expected file path in preview")
	}
	lines := strings.SplitN(preview, "Preview (first 2000 chars):\n", 2)
	if len(lines) != 2 {
		t.Fatalf("expected preview section, got: %s", preview[:200])
	}
	if len(lines[1]) > 2100 {
		t.Errorf("preview body too long: %d", len(lines[1]))
	}
}

func TestPersistToolResult_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path, err := persistToolResult(dir, "conv1", "tool-abc", "hello content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(data) != "hello content" {
		t.Errorf("wrong content: %s", data)
	}
	if !strings.HasSuffix(path, filepath.Join("conv1", "tool-abc.txt")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestContentReplacementState_FreezeDecision(t *testing.T) {
	s := newContentReplacementState()
	s.setReplacement("id1", "preview1")
	s.setReplacement("id1", "preview2")
	if got := s.getReplacement("id1"); got != "preview1" {
		t.Errorf("expected frozen preview1, got %s", got)
	}
}

func TestContentReplacementState_SeenNotReplaced(t *testing.T) {
	s := newContentReplacementState()
	s.markSeen("id2")
	if s.getReplacement("id2") != "" {
		t.Errorf("seen-only id should have no replacement")
	}
	if !s.isSeen("id2") {
		t.Errorf("id2 should be seen")
	}
}

func TestEnforcePerMessageBudget_UnderLimit(t *testing.T) {
	state := newContentReplacementState()
	blocks := []llm.ContentBlock{
		{Type: "tool_result", ToolUseID: "id1", Content: strings.Repeat("a", 1000)},
		{Type: "tool_result", ToolUseID: "id2", Content: strings.Repeat("b", 1000)},
	}
	result := enforcePerMessageBudget(blocks, 10_000, t.TempDir(), "conv1", state)
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	if result[0].Content != blocks[0].Content {
		t.Errorf("content should be unchanged under limit")
	}
}

func TestEnforcePerMessageBudget_OverLimit_ReplacesLargest(t *testing.T) {
	state := newContentReplacementState()
	dataDir := t.TempDir()
	small := strings.Repeat("s", 100)
	large := strings.Repeat("L", 9000)
	blocks := []llm.ContentBlock{
		{Type: "tool_result", ToolUseID: "small1", Content: small},
		{Type: "tool_result", ToolUseID: "large1", Content: large},
	}
	result := enforcePerMessageBudget(blocks, 5_000, dataDir, "conv1", state)
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	var largeBlock llm.ContentBlock
	for _, b := range result {
		if b.ToolUseID == "large1" {
			largeBlock = b
		}
	}
	if !strings.Contains(largeBlock.Content, "too large") {
		t.Errorf("large block should be replaced with preview, got: %s", largeBlock.Content[:80])
	}
	for _, b := range result {
		if b.ToolUseID == "small1" && b.Content != small {
			t.Errorf("small block should be unchanged")
		}
	}
}

func TestEnforcePerMessageBudget_StableAcrossRebuild(t *testing.T) {
	state := newContentReplacementState()
	dataDir := t.TempDir()
	large := strings.Repeat("L", 9000)
	blocks := []llm.ContentBlock{
		{Type: "tool_result", ToolUseID: "id1", Content: large},
	}
	result1 := enforcePerMessageBudget(blocks, 5_000, dataDir, "conv1", state)
	result2 := enforcePerMessageBudget(blocks, 5_000, dataDir, "conv1", state)
	if result1[0].Content != result2[0].Content {
		t.Errorf("preview must be identical across rebuilds")
	}
}
