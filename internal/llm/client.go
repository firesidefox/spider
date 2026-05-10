package llm

import (
	"context"
	"fmt"
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
	Chat(ctx context.Context, req *ChatRequest) (string, error)
	CountTokens(ctx context.Context, msgs []Message) (int, error)
}

// EstimateTokens 字符分段估算 token 数。
// r > 0x2E80 覆盖 CJK 及 Hangul/Kana 等东亚字符，约 1 token/字；
// 其他 Unicode（阿拉伯、西里尔等）归入 ascii 路径，误差较大但可接受。
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	var cjk, ascii int
	for _, r := range s {
		if r > 0x2E80 {
			cjk++
		} else {
			ascii++
		}
	}
	t := cjk + ascii/4
	if t == 0 {
		t = 1
	}
	return t
}

func estimateMessagesTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokens(m.Content)
	}
	return total
}

func NewClient(providerType, apiKey, model, baseURL string) (Client, error) {
	switch providerType {
	case "claude", "anthropic":
		return NewClaudeClient(apiKey, model, baseURL), nil
	case "openai":
		return NewOpenAIClient(apiKey, model, baseURL), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", providerType)
	}
}
