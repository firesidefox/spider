package llm

import "testing"

func TestNewClaudeClient(t *testing.T) {
	client := NewClaudeClient("sk-test-key", "claude-sonnet-4-6")
	if client.model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", client.model)
	}
	if client.apiKey != "sk-test-key" {
		t.Errorf("apiKey = %q, want sk-test-key", client.apiKey)
	}
}
