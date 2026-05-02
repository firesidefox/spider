package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ClaudeClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewClaudeClient(apiKey, model string) *ClaudeClient {
	return &ClaudeClient{
		apiKey: apiKey,
		model:  model,
		http: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}
}

func (c *ClaudeClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": req.MaxTokens,
		"system":     req.System,
		"messages":   req.Messages,
		"stream":     true,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamEvent, 32)
	go c.readSSE(resp.Body, ch)
	return ch, nil
}

func (c *ClaudeClient) readSSE(body io.ReadCloser, ch chan<- StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 || line[:6] != "data: " {
			continue
		}
		data := line[6:]

		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}

		eventType, _ := raw["type"].(string)
		switch eventType {
		case "content_block_delta":
			delta, _ := raw["delta"].(map[string]any)
			deltaType, _ := delta["type"].(string)
			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				ch <- StreamEvent{Type: "text_delta", Text: text}
			} else if deltaType == "input_json_delta" {
				text, _ := delta["partial_json"].(string)
				ch <- StreamEvent{Type: "tool_input_delta", Text: text}
			}
		case "content_block_start":
			cb, _ := raw["content_block"].(map[string]any)
			cbType, _ := cb["type"].(string)
			if cbType == "tool_use" {
				name, _ := cb["name"].(string)
				id, _ := cb["id"].(string)
				ch <- StreamEvent{
					Type:     "tool_start",
					ToolCall: &ToolCall{ID: id, Name: name},
				}
			}
		case "message_stop":
			ch <- StreamEvent{Type: "message_stop"}
			return
		}
	}
}
