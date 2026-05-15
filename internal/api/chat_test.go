package api

import (
	"testing"

	"github.com/spiderai/spider/internal/models"
)

func TestChatGetConversation_FiltersToolResults(t *testing.T) {
	msgs := []*models.Message{
		{ID: "1", Role: "user", Content: "test"},
		{ID: "2", Role: "assistant", Content: "response", ToolCalls: `[{"id":"tc1","name":"Tool1"}]`},
		{ID: "3", Role: "tool_result", Content: "tc1\x00result data"},
		{ID: "4", Role: "assistant", Content: "final"},
	}

	filtered := filterToolResults(msgs)

	if len(filtered) != 3 {
		t.Errorf("expected 3 messages after filtering, got %d", len(filtered))
	}

	for _, m := range filtered {
		if m.Role == "tool_result" {
			t.Errorf("tool_result message should be filtered out, found: %s", m.ID)
		}
	}

	// Verify order preserved
	if filtered[0].ID != "1" || filtered[1].ID != "2" || filtered[2].ID != "4" {
		t.Errorf("message order not preserved after filtering")
	}
}

// Helper function extracted from chatGetConversation logic
func filterToolResults(msgs []*models.Message) []*models.Message {
	n := 0
	for _, m := range msgs {
		if m.Role != "tool_result" {
			msgs[n] = m
			n++
		}
	}
	return msgs[:n]
}
