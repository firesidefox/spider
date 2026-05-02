package rag

import (
	"testing"
)

func TestNewOpenAIEmbedder(t *testing.T) {
	e := NewOpenAIEmbedder("sk-test", "text-embedding-3-small", 1536)
	if e.Dimensions() != 1536 {
		t.Errorf("Dimensions = %d, want 1536", e.Dimensions())
	}
}

func TestNewEmbedder_OpenAI(t *testing.T) {
	e, err := NewEmbedder("openai", "sk-test", "text-embedding-3-small", 1536)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Dimensions() != 1536 {
		t.Errorf("Dimensions = %d, want 1536", e.Dimensions())
	}
}

func TestNewEmbedder_UnsupportedProvider(t *testing.T) {
	_, err := NewEmbedder("cohere", "key", "embed-english-v3.0", 0)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
}
