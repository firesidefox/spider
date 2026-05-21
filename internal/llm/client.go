package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentBlock represents a structured content block for tool_use or tool_result.
type ContentBlock struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

func (b ContentBlock) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case "tool_use":
		input := b.Input
		if input == nil {
			input = map[string]any{}
		}
		return json.Marshal(struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		}{b.Type, b.ID, b.Name, input})
	case "tool_result":
		return json.Marshal(struct {
			Type      string `json:"type"`
			ToolUseID string `json:"tool_use_id"`
			Content   string `json:"content"`
			IsError   bool   `json:"is_error,omitempty"`
		}{b.Type, b.ToolUseID, b.Content, b.IsError})
	case "text":
		return json.Marshal(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{b.Type, b.Content})
	default:
		type plain ContentBlock
		return json.Marshal(plain(b))
	}
}

type Message struct {
	Role    Role `json:"role"`
	Content any  `json:"content"` // string or []ContentBlock
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

type SystemBlock struct {
	Text         string  `json:"text"`
	CacheControl *string `json:"cache_control,omitempty"` // "ephemeral" for Anthropic, nil otherwise
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type StreamEvent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Usage    *Usage    `json:"usage,omitempty"`
}

type ChatRequest struct {
	System    []SystemBlock `json:"-"` // Serialized by each provider
	Messages  []Message     `json:"messages"`
	Tools     []ToolDef     `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens"`
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
		switch v := m.Content.(type) {
		case string:
			total += EstimateTokens(v)
		case []ContentBlock:
			for _, b := range v {
				total += EstimateTokens(b.Content) + EstimateTokens(b.Name)
			}
		}
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
