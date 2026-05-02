package llm

import "testing"

func TestNewClaudeClient(t *testing.T) {
	client := NewClaudeClient("sk-test-key", "claude-sonnet-4-6", "")
	if client.model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", client.model)
	}
	if client.apiKey != "sk-test-key" {
		t.Errorf("apiKey = %q, want sk-test-key", client.apiKey)
	}
	if client.baseURL != defaultClaudeBaseURL {
		t.Errorf("baseURL = %q, want default", client.baseURL)
	}
}

func TestNewClaudeClient_CustomBaseURL(t *testing.T) {
	client := NewClaudeClient("sk-test", "model", "https://proxy.example.com")
	if client.baseURL != "https://proxy.example.com" {
		t.Errorf("baseURL = %q, want custom", client.baseURL)
	}
}
