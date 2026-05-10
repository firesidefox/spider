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

	"github.com/spiderai/spider/internal/logger"
)

const defaultClaudeBaseURL = "https://api.anthropic.com"

type ClaudeClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewClaudeClient(apiKey, model, baseURL string) *ClaudeClient {
	if baseURL == "" {
		baseURL = defaultClaudeBaseURL
	}
	return &ClaudeClient{
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

func (c *ClaudeClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	log := logger.FromContext(ctx).With().Str("module", "llm").Logger()
	log.Debug().Str("model", c.model).Int("msgs", len(req.Messages)).Msg("llm stream start")
	start := time.Now()

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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
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
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(errBody))
	}

	log.Debug().Str("model", c.model).Int64("ttfb_ms", time.Since(start).Milliseconds()).Msg("llm stream connected")
	ch := make(chan StreamEvent, 32)
	go c.readSSE(resp.Body, ch)
	return ch, nil
}

func (c *ClaudeClient) Chat(ctx context.Context, req *ChatRequest) (string, error) {
	log := logger.FromContext(ctx).With().Str("module", "llm").Logger()
	log.Debug().Str("model", c.model).Int("msgs", len(req.Messages)).Msg("llm chat start")
	start := time.Now()
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	body := map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"system":     req.System,
		"messages":   req.Messages,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
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
		return "", fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response content")
	}
	log.Debug().Str("model", c.model).Int64("duration_ms", time.Since(start).Milliseconds()).Msg("llm chat done")
	return result.Content[0].Text, nil
}

func (c *ClaudeClient) CountTokens(ctx context.Context, msgs []Message) (int, error) {
	body := map[string]any{
		"model":    c.model,
		"messages": msgs,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages/count_tokens", bytes.NewReader(jsonBody))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return estimateMessagesTokens(msgs), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return estimateMessagesTokens(msgs), nil
	}

	var result struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return estimateMessagesTokens(msgs), nil
	}
	return result.InputTokens, nil
}

func (c *ClaudeClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
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
		case "message_delta":
			usage, _ := raw["usage"].(map[string]any)
			if usage != nil {
				out, _ := usage["output_tokens"].(float64)
				ch <- StreamEvent{Type: "usage", Usage: &Usage{OutputTokens: int(out)}}
			}
		case "message_start":
			msg, _ := raw["message"].(map[string]any)
			if msg != nil {
				u, _ := msg["usage"].(map[string]any)
				if u != nil {
					in, _ := u["input_tokens"].(float64)
					ch <- StreamEvent{Type: "usage", Usage: &Usage{InputTokens: int(in)}}
				}
			}
		case "message_stop":
			ch <- StreamEvent{Type: "message_stop"}
			return
		}
	}
}
