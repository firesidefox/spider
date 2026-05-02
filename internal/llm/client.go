package llm

import (
	"context"
	"fmt"

	"github.com/spiderai/spider/internal/config"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type StreamEvent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

type ChatRequest struct {
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens"`
}

type Client interface {
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}

func NewClient(cfg *config.LLMModelConfig) (Client, error) {
	switch cfg.Provider {
	case "claude":
		return NewClaudeClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
