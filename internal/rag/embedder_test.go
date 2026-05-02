package rag

import (
	"testing"

	"github.com/spiderai/spider/internal/config"
)

func TestNewOpenAIEmbedder(t *testing.T) {
	cfg := &config.EmbeddingModelConfig{
		ID:         "test",
		Provider:   "openai",
		APIKey:     "sk-test",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}
	e := NewOpenAIEmbedder(cfg)
	if e.Dimensions() != 1536 {
		t.Errorf("Dimensions = %d, want 1536", e.Dimensions())
	}
}

func TestNewEmbedder_OpenAI(t *testing.T) {
	cfg := &config.EmbeddingModelConfig{
		ID:         "test",
		Provider:   "openai",
		APIKey:     "sk-test",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}
	e, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Dimensions() != 1536 {
		t.Errorf("Dimensions = %d, want 1536", e.Dimensions())
	}
}

func TestNewEmbedder_UnsupportedProvider(t *testing.T) {
	cfg := &config.EmbeddingModelConfig{
		ID:       "test",
		Provider: "cohere",
		APIKey:   "key",
		Model:    "embed-english-v3.0",
	}
	_, err := NewEmbedder(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
}
