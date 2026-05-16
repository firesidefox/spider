package agent

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	dbpkg "github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/store"

	_ "modernc.org/sqlite"
)

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := dbpkg.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

type mockIntegrationLLMClient struct {
	chatCalls int
	response  string
}

func (m *mockIntegrationLLMClient) ChatStream(_ context.Context, _ *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockIntegrationLLMClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
	m.chatCalls++
	if m.response == "" {
		return "summary of conversation", nil
	}
	return m.response, nil
}

func (m *mockIntegrationLLMClient) CountTokens(_ context.Context, msgs []llm.Message) (int, error) {
	total := 0
	for _, msg := range msgs {
		if s, ok := msg.Content.(string); ok {
			total += llm.EstimateTokens(s)
		}
	}
	return total, nil
}

// insertTestMessages inserts n user/assistant pairs directly into DB with fixed IDs.
// startIdx offsets the ID numbering so successive calls don't collide.
// Returns all inserted message IDs in order.
func insertTestMessages(t *testing.T, db *sql.DB, convID string, pairs int, content string, startIdx ...int) []string {
	t.Helper()
	base := 0
	if len(startIdx) > 0 {
		base = startIdx[0]
	}
	// Count existing messages to offset created_at correctly
	var existingCount int
	db.QueryRow("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", convID).Scan(&existingCount)
	var ids []string
	for i := range pairs {
		n := base + i + 1
		uid := fmt.Sprintf("%s-u%d", convID, n)
		aid := fmt.Sprintf("%s-a%d", convID, n)
		for _, row := range []struct{ id, role string }{{uid, "user"}, {aid, "assistant"}} {
			off := existingCount + len(ids)
			_, err := db.Exec(
				`INSERT OR IGNORE INTO messages (id, conversation_id, role, content, tool_calls, created_at)
				 VALUES (?, ?, ?, ?, '', datetime('now', ?))`,
				row.id, convID, row.role, content, fmt.Sprintf("+%d seconds", off),
			)
			if err != nil {
				t.Fatalf("insert message %s: %v", row.id, err)
			}
			ids = append(ids, row.id)
		}
	}
	return ids
}

// cleanupConv removes test data for a conversation.
func cleanupConv(t *testing.T, db *sql.DB, convID string) {
	t.Helper()
	db.Exec("DELETE FROM messages WHERE conversation_id = ?", convID)
	db.Exec("DELETE FROM conversation_summaries WHERE conversation_id = ?", convID)
}

// uniqueConvID generates a unique conversation ID for each test.
func uniqueConvID(prefix string) string {
	return fmt.Sprintf("inttest-%s-%d", prefix, time.Now().UnixNano())
}

// TestIntegration_ShortConversation: 3 pairs, high threshold → no compaction.
func TestIntegration_ShortConversation(t *testing.T) {
	db := openIntegrationDB(t)
	convID := uniqueConvID("short")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	insertTestMessages(t, db, convID, 3, "hello world")

	llmC := &mockIntegrationLLMClient{}
	msgStore := store.NewMessageStore(db)
	sumStore := store.NewSummaryStore(db)
	cfg := config.CompactionConfig{ThresholdTokens: 100000, RecentTurns: 20, MaxSummaryTokens: 4000}
	c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)

	history, err := c.BuildHistory(context.Background(), convID, false)
	if err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	if len(history) != 6 {
		t.Errorf("want 6 messages, got %d", len(history))
	}
	sum, err := sumStore.Get(convID)
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}
	if sum != nil {
		t.Errorf("want nil summary, got %+v", sum)
	}
	if llmC.chatCalls != 0 {
		t.Errorf("want 0 Chat calls, got %d", llmC.chatCalls)
	}
}

// TestIntegration_CacheReuse: pre-existing summary, few new messages → no Chat call.
func TestIntegration_CacheReuse(t *testing.T) {
	db := openIntegrationDB(t)
	convID := uniqueConvID("cache")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	ids := insertTestMessages(t, db, convID, 5, "hello")
	boundaryID := ids[5] // after 3rd pair (index 5 = a3)

	sumStore := store.NewSummaryStore(db)
	if err := sumStore.Upsert(convID, boundaryID, []string{"cached summary"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	llmC := &mockIntegrationLLMClient{}
	msgStore := store.NewMessageStore(db)
	cfg := config.CompactionConfig{ThresholdTokens: 100000, RecentTurns: 20, MaxSummaryTokens: 4000}
	c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)

	history, err := c.BuildHistory(context.Background(), convID, false)
	if err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	if llmC.chatCalls != 0 {
		t.Errorf("want 0 Chat calls (cache hit), got %d", llmC.chatCalls)
	}
	// First message pair should be the summary injection
	if len(history) < 2 {
		t.Fatalf("want at least 2 history entries, got %d", len(history))
	}
	// history[0] is the preamble user message; history[1] is the assistant message with chunk content
	if !strings.Contains(history[1].Content.(string), "cached summary") {
		t.Errorf("want chunk in assistant summary message, got %q", history[1].Content)
	}
}

// TestIntegration_FirstCompaction: 10 pairs, low threshold → Chat called once.
func TestIntegration_FirstCompaction(t *testing.T) {
	db := openIntegrationDB(t)
	convID := uniqueConvID("first")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	insertTestMessages(t, db, convID, 10, strings.Repeat("x", 200))

	llmC := &mockIntegrationLLMClient{}
	msgStore := store.NewMessageStore(db)
	sumStore := store.NewSummaryStore(db)
	cfg := config.CompactionConfig{ThresholdTokens: 100, RecentTurns: 5, MaxSummaryTokens: 4000}
	c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)

	history, err := c.BuildHistory(context.Background(), convID, false)
	if err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	if llmC.chatCalls != 1 {
		t.Errorf("want 1 Chat call, got %d", llmC.chatCalls)
	}
	sum, err := sumStore.Get(convID)
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}
	if sum == nil {
		t.Fatal("want non-nil summary after compaction")
	}
	if len(history) < 2 || !strings.Contains(history[0].Content.(string), "summary") {
		t.Errorf("want summary injected at head of history")
	}
}

// TestIntegration_BoundaryAdvance: two BuildHistory calls → up_to_message_id advances.
func TestIntegration_BoundaryAdvance(t *testing.T) {
	db := openIntegrationDB(t)
	convID := uniqueConvID("advance")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	insertTestMessages(t, db, convID, 10, strings.Repeat("x", 200))

	llmC := &mockIntegrationLLMClient{}
	msgStore := store.NewMessageStore(db)
	sumStore := store.NewSummaryStore(db)
	cfg := config.CompactionConfig{ThresholdTokens: 100, RecentTurns: 5, MaxSummaryTokens: 4000}
	c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)

	if _, err := c.BuildHistory(context.Background(), convID, false); err != nil {
		t.Fatalf("first BuildHistory: %v", err)
	}
	sum1, _ := sumStore.Get(convID)
	if sum1 == nil {
		t.Fatal("want summary after first compaction")
	}
	firstBoundary := sum1.UpToMessageID

	// Insert 5 more pairs with offset to avoid ID collision
	insertTestMessages(t, db, convID, 5, strings.Repeat("y", 200), 10)

	if _, err := c.BuildHistory(context.Background(), convID, false); err != nil {
		t.Fatalf("second BuildHistory: %v", err)
	}
	sum2, _ := sumStore.Get(convID)
	if sum2 == nil {
		t.Fatal("want summary after second compaction")
	}
	if sum2.UpToMessageID == firstBoundary {
		t.Errorf("want boundary to advance, still at %s", firstBoundary)
	}
	if len(sum2.Chunks) < 2 {
		t.Errorf("want ≥2 chunks after second compaction, got %d", len(sum2.Chunks))
	}
}

// TestIntegration_ChunksConsolidation: tiny MaxSummaryTokens → chunks consolidated to 1.
func TestIntegration_ChunksConsolidation(t *testing.T) {
	db := openIntegrationDB(t)
	convID := uniqueConvID("consol")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	// Pre-seed a summary with a long chunk (>50 tokens)
	longChunk := strings.Repeat("word ", 60) // ~60 tokens
	ids := insertTestMessages(t, db, convID, 3, "seed")
	sumStore := store.NewSummaryStore(db)
	if err := sumStore.Upsert(convID, ids[1], []string{longChunk}); err != nil {
		t.Fatalf("Upsert seed: %v", err)
	}

	// Insert more messages to push over threshold
	insertTestMessages(t, db, convID, 10, strings.Repeat("x", 200))

	llmC := &mockIntegrationLLMClient{response: "consolidated summary"}
	msgStore := store.NewMessageStore(db)
	cfg := config.CompactionConfig{ThresholdTokens: 100, RecentTurns: 5, MaxSummaryTokens: 50}
	c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)

	if _, err := c.BuildHistory(context.Background(), convID, false); err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	sum, _ := sumStore.Get(convID)
	if sum == nil {
		t.Fatal("want summary")
	}
	if len(sum.Chunks) != 1 {
		t.Errorf("want 1 consolidated chunk, got %d: %v", len(sum.Chunks), sum.Chunks)
	}
}

// TestIntegration_ThresholdConfig: explicit threshold_tokens triggers compaction.
func TestIntegration_ThresholdConfig(t *testing.T) {
	db := openIntegrationDB(t)
	convID := uniqueConvID("thresh")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	// 10 pairs × 200 chars ≈ 500 tokens total, well above threshold=50
	insertTestMessages(t, db, convID, 10, strings.Repeat("a", 200))

	llmC := &mockIntegrationLLMClient{}
	msgStore := store.NewMessageStore(db)
	sumStore := store.NewSummaryStore(db)
	cfg := config.CompactionConfig{ThresholdTokens: 50, RecentTurns: 5, MaxSummaryTokens: 4000}
	c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)

	if _, err := c.BuildHistory(context.Background(), convID, false); err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	if llmC.chatCalls == 0 {
		t.Error("want compaction triggered by explicit threshold_tokens=50")
	}
}

// TestIntegration_UnknownModelFallback: unknown model uses DefaultThreshold=120000.
func TestIntegration_UnknownModelFallback(t *testing.T) {
	model := "unknown-model-xyz"
	got := llm.DefaultThreshold(model)
	if got != 120000 {
		t.Errorf("DefaultThreshold(%q) = %d, want 120000", model, got)
	}

	// Also verify it doesn't trigger compaction with short messages
	db := openIntegrationDB(t)
	convID := uniqueConvID("fallback")
	cleanupConv(t, db, convID)
	t.Cleanup(func() { cleanupConv(t, db, convID) })

	insertTestMessages(t, db, convID, 3, "hello")

	llmC := &mockIntegrationLLMClient{}
	msgStore := store.NewMessageStore(db)
	sumStore := store.NewSummaryStore(db)
	// ThresholdTokens=0 → use model table → 120000 for unknown model
	cfg := config.CompactionConfig{ThresholdTokens: 0, RecentTurns: 20, MaxSummaryTokens: 4000}
	c := NewCompactor(llmC, sumStore, msgStore, model, cfg, t.TempDir(), 0, nil)

	if _, err := c.BuildHistory(context.Background(), convID, false); err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	if llmC.chatCalls != 0 {
		t.Errorf("want no compaction for unknown model with short messages, got %d Chat calls", llmC.chatCalls)
	}
}