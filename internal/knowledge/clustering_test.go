package knowledge

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/llm"
)

// clusterMockClient implements llm.Client for clustering tests.
type clusterMockClient struct {
	response string
	err      error
	called   bool
}

func (m *clusterMockClient) Chat(ctx context.Context, req *llm.ChatRequest) (string, error) {
	m.called = true
	return m.response, m.err
}

func (m *clusterMockClient) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *clusterMockClient) CountTokens(ctx context.Context, msgs []llm.Message) (int, error) {
	return 0, nil
}

func TestClusterEntries(t *testing.T) {
	entries := []ParsedEntry{
		{Title: "POST /login", Summary: "User login"},
		{Title: "POST /logout", Summary: "User logout"},
		{Title: "GET /users", Summary: "List users"},
		{Title: "GET /hosts", Summary: "List hosts"},
	}

	mockResp := `{"sections": [{"name": "认证接口", "summary": "登录和登出相关接口", "entry_ids": [0, 1]}, {"name": "查询接口", "summary": "资源列表查询接口", "entry_ids": [2, 3]}]}`
	client := &clusterMockClient{response: mockResp}

	result, err := ClusterEntries(context.Background(), client, entries)
	if err != nil {
		t.Fatalf("ClusterEntries() error = %v", err)
	}
	if len(result.Sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(result.Sections))
	}

	s0 := result.Sections[0]
	if s0.Name != "认证接口" {
		t.Errorf("sections[0].Name = %q, want %q", s0.Name, "认证接口")
	}
	if len(s0.EntryIDs) != 2 || s0.EntryIDs[0] != 0 || s0.EntryIDs[1] != 1 {
		t.Errorf("sections[0].EntryIDs = %v, want [0 1]", s0.EntryIDs)
	}

	s1 := result.Sections[1]
	if s1.Name != "查询接口" {
		t.Errorf("sections[1].Name = %q, want %q", s1.Name, "查询接口")
	}
	if len(s1.EntryIDs) != 2 || s1.EntryIDs[0] != 2 || s1.EntryIDs[1] != 3 {
		t.Errorf("sections[1].EntryIDs = %v, want [2 3]", s1.EntryIDs)
	}
}

func TestClusterEntriesEmpty(t *testing.T) {
	client := &clusterMockClient{}

	result, err := ClusterEntries(context.Background(), client, []ParsedEntry{})
	if err != nil {
		t.Fatalf("ClusterEntries() error = %v", err)
	}
	if len(result.Sections) != 0 {
		t.Errorf("got %d sections, want 0", len(result.Sections))
	}
	if client.called {
		t.Error("LLM should not be called for empty input")
	}
}
