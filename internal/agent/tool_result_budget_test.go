package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
