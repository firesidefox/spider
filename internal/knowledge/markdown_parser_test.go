package knowledge

import (
	"context"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/llm"
)

// mockLLMClient implements llm.Client for testing.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Chat(ctx context.Context, req *llm.ChatRequest) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockLLMClient) CountTokens(ctx context.Context, msgs []llm.Message) (int, error) {
	return 0, nil
}

func TestSplitMarkdown(t *testing.T) {
	// Short text: single chunk
	short := "# Hello\nThis is short."
	chunks := SplitMarkdownForTest(short, 1000)
	if len(chunks) != 1 {
		t.Errorf("short text: got %d chunks, want 1", len(chunks))
	}

	// Long text with ## headings: should split into multiple chunks
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		sb.WriteString("\n## Section\n")
		sb.WriteString(strings.Repeat("x", 500))
	}
	long := sb.String()
	chunks = SplitMarkdownForTest(long, 1000)
	if len(chunks) < 2 {
		t.Errorf("long text: got %d chunks, want >= 2", len(chunks))
	}
	for i, c := range chunks {
		if c == "" {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestMarkdownParser(t *testing.T) {
	mockResp := `{"entries": [{"title": "ls command", "summary": "List directory contents", "content": "## ls\nList files in a directory."}, {"title": "cd command", "summary": "Change directory", "content": "## cd\nChange the current working directory."}]}`

	client := &mockLLMClient{response: mockResp}
	parser := NewMarkdownParser(client)

	content := []byte("## ls\nList files in a directory.\n\n## cd\nChange the current working directory.")
	entries, err := parser.Parse(context.Background(), content, "manual.md")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Parse() returned %d entries, want 2", len(entries))
	}
	if entries[0].Title != "ls command" {
		t.Errorf("entries[0].Title = %q, want %q", entries[0].Title, "ls command")
	}
	if entries[0].Summary != "List directory contents" {
		t.Errorf("entries[0].Summary = %q, want %q", entries[0].Summary, "List directory contents")
	}
	if entries[1].Title != "cd command" {
		t.Errorf("entries[1].Title = %q, want %q", entries[1].Title, "cd command")
	}
}
