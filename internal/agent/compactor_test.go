package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// mockChatClient extends mockLLMClient and counts Chat() calls.
type mockChatClient struct {
	mockLLMClient
	chatCalls int
	chatReply string
}

func (m *mockChatClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
	m.chatCalls++
	if m.chatReply != "" {
		return m.chatReply, nil
	}
	return "summary text", nil
}

// mockSummaryStore implements summaryStorer for tests.
type mockSummaryStore struct {
	data        *store.ConversationSummary
	upsertCalls int
	lastChunks  []string
	lastBoundID string
}

func (m *mockSummaryStore) Get(_ string) (*store.ConversationSummary, error) {
	return m.data, nil
}

func (m *mockSummaryStore) Upsert(_ string, upToMessageID string, chunks []string) error {
	m.upsertCalls++
	m.lastBoundID = upToMessageID
	m.lastChunks = chunks
	return nil
}

// makeAltMessages builds alternating user/assistant messages with the given content.
func makeAltMessages(pairs int, content string) []*models.Message {
	msgs := make([]*models.Message, 0, pairs*2)
	for i := range pairs {
		msgs = append(msgs,
			&models.Message{ID: fmt.Sprintf("u%d", i+1), Role: "user", Content: content},
			&models.Message{ID: fmt.Sprintf("a%d", i+1), Role: "assistant", Content: content},
		)
	}
	return msgs
}

// fixedMsgStore returns a fixed slice from ListAfterMessage (ignores IDs).
type fixedMsgStore struct {
	msgs []*models.Message
}

func (f *fixedMsgStore) Save(_, _, _, _ string) error { return nil }
func (f *fixedMsgStore) ListByConversation(_ string) ([]*models.Message, error) {
	return f.msgs, nil
}
func (f *fixedMsgStore) ListAfterMessage(_, _ string) ([]*models.Message, error) {
	return f.msgs, nil
}

func newCompactor(llmC llm.Client, ss *mockSummaryStore, msgs []*models.Message, cfg config.CompactionConfig) *Compactor {
	return NewCompactor(llmC, ss, &fixedMsgStore{msgs: msgs}, "", cfg, "", 0, nil)
}

func TestBuildHistory_UnderThreshold(t *testing.T) {
	msgs := makeAltMessages(3, "hello world") // ~3 tokens each, total well under 100000
	ss := &mockSummaryStore{}
	llmC := &mockChatClient{}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens: 100000,
		RecentTurns:     20,
	})

	history, err := c.BuildHistory(context.Background(), "conv1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != len(msgs) {
		t.Errorf("history len = %d, want %d", len(history), len(msgs))
	}
	if llmC.chatCalls != 0 {
		t.Errorf("Chat() called %d times, want 0", llmC.chatCalls)
	}
	if ss.upsertCalls != 0 {
		t.Errorf("Upsert() called %d times, want 0", ss.upsertCalls)
	}
}

func TestBuildHistory_OverThreshold_NoCache(t *testing.T) {
	// 50 pairs, each message ~25 chars → ~6 tokens each → 600 tokens total, threshold=10
	content := strings.Repeat("x", 100) // ~25 tokens per message
	msgs := makeAltMessages(50, content)
	ss := &mockSummaryStore{}
	llmC := &mockChatClient{chatReply: "new summary"}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens: 10,
		RecentTurns:     5,
	})

	history, err := c.BuildHistory(context.Background(), "conv1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llmC.chatCalls == 0 {
		t.Error("Chat() was not called")
	}
	if ss.upsertCalls == 0 {
		t.Error("Upsert() was not called")
	}
	// history[0] should be the summary user prompt, history[1] the assistant summary
	if len(history) < 2 {
		t.Fatalf("history too short: %d", len(history))
	}
	if history[0].Role != "user" || history[1].Role != "assistant" {
		t.Errorf("expected summary pair at head, got roles %q %q", history[0].Role, history[1].Role)
	}
}

func TestBuildHistory_OverThreshold_WithCache(t *testing.T) {
	content := strings.Repeat("x", 100)
	msgs := makeAltMessages(50, content)
	ss := &mockSummaryStore{
		data: &store.ConversationSummary{
			UpToMessageID: "prev-boundary",
			Chunks:        []string{"old chunk"},
		},
	}
	llmC := &mockChatClient{chatReply: "new delta"}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens:  10,
		RecentTurns:      5,
		MaxSummaryTokens: 100000,
	})

	_, err := c.BuildHistory(context.Background(), "conv1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss.upsertCalls == 0 {
		t.Fatal("Upsert() was not called")
	}
	if ss.lastChunks[0] != "old chunk" {
		t.Errorf("chunks[0] = %q, want %q", ss.lastChunks[0], "old chunk")
	}
	if len(ss.lastChunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(ss.lastChunks))
	}
}

func TestBuildHistory_ChunksOverflow(t *testing.T) {
	// MaxSummaryTokens=50 is tiny; two chunks of 200 chars each → ~100 tokens total, exceeds 50.
	content := strings.Repeat("x", 100)
	msgs := makeAltMessages(50, content)
	ss := &mockSummaryStore{
		data: &store.ConversationSummary{
			UpToMessageID: "prev",
			Chunks:        []string{strings.Repeat("a", 200), strings.Repeat("b", 200)},
		},
	}
	llmC := &mockChatClient{chatReply: "consolidated"}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens:  10,
		RecentTurns:      5,
		MaxSummaryTokens: 50,
	})

	_, err := c.BuildHistory(context.Background(), "conv1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss.upsertCalls == 0 {
		t.Fatal("Upsert() was not called")
	}
	if len(ss.lastChunks) != 1 {
		t.Errorf("chunks len = %d, want 1 (consolidated)", len(ss.lastChunks))
	}
}

func TestBuildHistory_CannotAdvanceBoundary(t *testing.T) {
	// Only 1 pair (2 messages), RecentTurns=5 → boundary cannot advance.
	content := strings.Repeat("x", 100)
	msgs := makeAltMessages(1, content)
	ss := &mockSummaryStore{}
	llmC := &mockChatClient{}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens: 10,
		RecentTurns:     5,
	})

	_, err := c.BuildHistory(context.Background(), "conv1", false)
	if !errors.Is(err, ErrCannotAdvanceBoundary) {
		t.Errorf("err = %v, want ErrCannotAdvanceBoundary", err)
	}
}

func TestFindBoundaryByTurns(t *testing.T) {
	// [u1,a1,u2,a2,u3,a3,u4,a4] — indices 0..7
	msgs := []*models.Message{
		{Role: "user"},      // 0
		{Role: "assistant"}, // 1
		{Role: "user"},      // 2
		{Role: "assistant"}, // 3
		{Role: "user"},      // 4
		{Role: "assistant"}, // 5
		{Role: "user"},      // 6
		{Role: "assistant"}, // 7
	}

	if got := findBoundaryByTurns(msgs, 2); got != 4 {
		t.Errorf("n=2: got %d, want 4", got)
	}
	if got := findBoundaryByTurns(msgs, 3); got != 2 {
		t.Errorf("n=3: got %d, want 2", got)
	}
	// n=4 means we need 4 user messages from the tail; the 4th is at index 0,
	// which findBoundaryByTurns returns as -1 (not-enough sentinel).
	if got := findBoundaryByTurns(msgs, 4); got != -1 {
		t.Errorf("n=4: got %d, want -1 (not enough)", got)
	}
}

func TestEstimateChunksTokens(t *testing.T) {
	// Pure ASCII: 40 chars → ~10 tokens (40/4)
	ascii := strings.Repeat("a", 40)
	if got := estimateChunksTokens([]string{ascii}); got != 10 {
		t.Errorf("ascii: got %d, want 10", got)
	}

	// Pure CJK: 20 chars → ~20 tokens (1 per char)
	cjk := strings.Repeat("中", 20)
	if got := estimateChunksTokens([]string{cjk}); got != 20 {
		t.Errorf("cjk: got %d, want 20", got)
	}
}

// errCountTokensClient returns an error from CountTokens.
type errCountTokensClient struct {
	mockChatClient
}

func (e *errCountTokensClient) CountTokens(_ context.Context, _ []llm.Message) (int, error) {
	return 0, errors.New("count tokens unavailable")
}

func TestBuildHistory_CountTokensError(t *testing.T) {
	msgs := makeAltMessages(3, "hello world")
	ss := &mockSummaryStore{}
	llmC := &errCountTokensClient{}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens: 100000,
		RecentTurns:     20,
	})

	_, err := c.BuildHistory(context.Background(), "conv1", false)
	if err == nil {
		t.Error("expected error from CountTokens, got nil")
	}
}

// errChatClient returns an error from Chat (even after retry).
type errChatClient struct {
	mockLLMClient
}

func (e *errChatClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
	return "", errors.New("LLM unavailable")
}

func TestBuildHistory_SummarizeError(t *testing.T) {
	content := strings.Repeat("x", 100)
	msgs := makeAltMessages(50, content)
	ss := &mockSummaryStore{}
	llmC := &errChatClient{}
	c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
		ThresholdTokens: 10,
		RecentTurns:     5,
	})

	_, err := c.BuildHistory(context.Background(), "conv1", false)
	if err == nil {
		t.Error("expected error from summarize, got nil")
	}
}

func TestBuildHistory_ConcurrentSafe(t *testing.T) {
	content := strings.Repeat("x", 100)
	msgs := makeAltMessages(50, content)

	const goroutines = 10
	errs := make(chan error, goroutines)
	for range goroutines {
		go func() {
			// Each goroutine gets its own LLM client and summary store to avoid shared state.
			llmC := &mockChatClient{chatReply: "summary"}
			ss := &mockSummaryStore{}
			c := newCompactor(llmC, ss, msgs, config.CompactionConfig{
				ThresholdTokens: 10,
				RecentTurns:     5,
			})
			_, err := c.BuildHistory(context.Background(), "conv1", false)
			errs <- err
		}()
	}
	for range goroutines {
		if err := <-errs; err != nil {
			t.Errorf("concurrent BuildHistory error: %v", err)
		}
	}
}
