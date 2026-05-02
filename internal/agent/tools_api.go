package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBody = 64 * 1024

type CallRESTAPITool struct {
	http *http.Client
}

func NewCallRESTAPITool() *CallRESTAPITool {
	return &CallRESTAPITool{http: &http.Client{Timeout: 30 * time.Second}}
}

func (t *CallRESTAPITool) Name() string { return "call_rest_api" }

func (t *CallRESTAPITool) Description() string {
	return "Call a REST API endpoint on a gateway device"
}

func (t *CallRESTAPITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":     map[string]any{"type": "string", "description": "Full URL to call"},
			"method":  map[string]any{"type": "string", "description": "HTTP method", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
			"headers": map[string]any{"type": "object", "description": "HTTP headers"},
			"body":    map[string]any{"type": "string", "description": "Request body"},
		},
		"required": []string{"url", "method"},
	}
}

func (t *CallRESTAPITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	url, _ := input["url"].(string)
	method, _ := input["method"].(string)
	if url == "" || method == "" {
		return &ToolResult{Content: "url and method are required", IsError: true, RiskLevel: RiskModerate}, nil
	}

	bodyStr, _ := input["body"].(string)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(bodyStr))
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("build request: %v", err), IsError: true, RiskLevel: RiskModerate}, nil
	}

	if hdrs, ok := input["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("request error: %v", err), IsError: true, RiskLevel: RiskModerate}, nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("read body: %v", err), IsError: true, RiskLevel: RiskModerate}, nil
	}

	respHeaders := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	out, _ := json.Marshal(map[string]any{
		"status_code": resp.StatusCode,
		"headers":     respHeaders,
		"body":        string(raw),
	})
	return &ToolResult{Content: string(out), RiskLevel: RiskModerate}, nil
}
