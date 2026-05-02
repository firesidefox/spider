package llm

import (
	"testing"

	"github.com/spiderai/spider/internal/config"
)

func TestNewClaudeClient(t *testing.T) {
	cfg := &config.LLMModelConfig{
		ID:       "test",
		Provider: "claude",
		APIKey:   "sk-test-key",
		Model:    "claude-sonnet-4-6",
	}
	client := NewClaudeClient(cfg)
	if client.model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", client.model)
	}
	if client.apiKey != "sk-test-key" {
		t.Errorf("apiKey = %q, want sk-test-key", client.apiKey)
	}
}
