package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/logger"
)

type OpenAIClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}
}

func (c *OpenAIClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	log := logger.FromContext(ctx).With().Str("module", "llm").Logger()
	msgsJSON, _ := json.Marshal(req.Messages)
	log.Debug().Str("model", c.model).Int("msgs", len(req.Messages)).Int("system_blocks", len(req.System)).RawJSON("messages", msgsJSON).Msg("llm stream start")
	start := time.Now()

	msgs := c.buildMessages(req)

	body := map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"stream":     true,
		"max_tokens": req.MaxTokens,
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.InputSchema,
				},
			}
		}
		body["tools"] = tools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		log.Error().Err(err).Str("model", c.model).Msg("llm stream error")
		return nil, fmt.Errorf("http request: %w", err)
	}
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		log.Error().Str("model", c.model).Int("status", resp.StatusCode).Msg("llm stream api error")
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(errBody))
	}

	log.Debug().Str("model", c.model).Int64("ttfb_ms", time.Since(start).Milliseconds()).Msg("llm stream connected")
	ch := make(chan StreamEvent, 32)
	go c.readSSE(resp.Body, ch)
	return ch, nil
}

func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (string, error) {
	log := logger.FromContext(ctx).With().Str("module", "llm").Logger()
	msgsJSON, _ := json.Marshal(req.Messages)
	log.Debug().Str("model", c.model).Int("msgs", len(req.Messages)).Int("system_blocks", len(req.System)).RawJSON("messages", msgsJSON).Msg("llm chat start")
	start := time.Now()
	msgs := c.buildMessages(req)

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	body := map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"stream":     false,
		"max_tokens": maxTokens,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices in response")
	}
	log.Debug().Str("model", c.model).Int64("duration_ms", time.Since(start).Milliseconds()).Str("response", result.Choices[0].Message.Content).Msg("llm chat done")
	return result.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) CountTokens(_ context.Context, msgs []Message) (int, error) {
	total := 0
	for _, m := range msgs {
		if s, ok := m.Content.(string); ok {
			total += EstimateTokens(s)
		}
	}
	return total, nil
}

// openaiDelta is the partial structure of an OpenAI SSE chunk.
type openaiDelta struct {
	Content   string `json:"content"`
	ToolCalls []struct {
		Index    int    `json:"index"`
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

type openaiChunk struct {
	Choices []struct {
		Delta        openaiDelta `json:"delta"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *OpenAIClient) buildMessages(req *ChatRequest) []map[string]any {
	msgs := make([]map[string]any, 0, len(req.Messages)+1)
	if len(req.System) > 0 {
		var parts []string
		for _, block := range req.System {
			parts = append(parts, block.Text)
		}
		systemText := strings.Join(parts, "\n\n")
		msgs = append(msgs, map[string]any{"role": "system", "content": systemText})
	}
	for _, m := range req.Messages {
		blocks, ok := m.Content.([]ContentBlock)
		if !ok {
			msgs = append(msgs, map[string]any{"role": string(m.Role), "content": m.Content})
			continue
		}
		if m.Role == RoleAssistant {
			var toolCalls []map[string]any
			var text string
			for _, b := range blocks {
				if b.Type == "tool_use" {
					argsJSON, _ := json.Marshal(b.Input)
					toolCalls = append(toolCalls, map[string]any{
						"id":   b.ID,
						"type": "function",
						"function": map[string]any{
							"name":      b.Name,
							"arguments": string(argsJSON),
						},
					})
				} else if b.Type == "text" {
					text = b.Content
				}
			}
			msg := map[string]any{"role": "assistant", "content": text}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			msgs = append(msgs, msg)
		} else {
			for _, b := range blocks {
				if b.Type == "tool_result" {
					msgs = append(msgs, map[string]any{
						"role":         "tool",
						"content":      b.Content,
						"tool_call_id": b.ToolUseID,
					})
				}
			}
		}
	}
	return msgs
}

func (c *OpenAIClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}

func (c *OpenAIClient) readSSE(body io.ReadCloser, ch chan<- StreamEvent) {
	defer close(ch)
	defer body.Close()

	// toolIDs tracks the call ID for each tool_calls index (sent only on first delta).
	toolIDs := map[int]string{}

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			ch <- StreamEvent{Type: "message_stop"}
			return
		}

		var chunk openaiChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			ch <- StreamEvent{Type: "usage", Usage: &Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			}}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			ch <- StreamEvent{Type: "text_delta", Text: delta.Content}
		}

		for _, tc := range delta.ToolCalls {
			if tc.ID != "" {
				toolIDs[tc.Index] = tc.ID
				ch <- StreamEvent{
					Type:     "tool_start",
					ToolCall: &ToolCall{ID: tc.ID, Name: tc.Function.Name},
				}
			}
			if len(tc.Function.Arguments) > 0 {
				var argStr string
				if json.Unmarshal(tc.Function.Arguments, &argStr) == nil {
					if argStr != "" {
						ch <- StreamEvent{Type: "tool_input_delta", Text: argStr}
					}
				} else {
					ch <- StreamEvent{Type: "tool_input_delta", Text: string(tc.Function.Arguments)}
				}
			}
		}

		// Some providers omit [DONE] and signal completion via finish_reason.
		if chunk.Choices[0].FinishReason != "" {
			ch <- StreamEvent{Type: "message_stop"}
			return
		}
	}
	// Connection closed without [DONE] or finish_reason.
	ch <- StreamEvent{Type: "message_stop"}
}
