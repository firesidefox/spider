package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

const maxResponseBody = 64 * 1024

type CallRESTAPITool struct {
	http  *http.Client
	faces *store.AccessFaceStore
}

func NewCallRESTAPITool(faces *store.AccessFaceStore) *CallRESTAPITool {
	return &CallRESTAPITool{
		http:  &http.Client{Timeout: 30 * time.Second},
		faces: faces,
	}
}

func (t *CallRESTAPITool) DefaultRiskLevel() RiskLevel { return RiskL2 }
func (t *CallRESTAPITool) Name() string                { return "CallAPI" }

func (t *CallRESTAPITool) Description() string {
	return "Call a REST API endpoint on a gateway device. Has side effects for POST/PUT/DELETE methods. Use GET freely in Explore phase; use mutating methods only in Act phase after confirming intent."
}

func (t *CallRESTAPITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":     map[string]any{"type": "string", "description": "Full URL to call"},
			"method":  map[string]any{"type": "string", "description": "HTTP method", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
			"headers": map[string]any{"type": "object", "description": "HTTP headers"},
			"body":    map[string]any{"type": "string", "description": "Request body"},
		"face_id": map[string]any{"type": "string", "description": "Optional. Access face ID. If provided, auth headers are injected automatically from the stored credentials."},
		},
		"required": []string{"method"},
	}
}

func (t *CallRESTAPITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	url, _ := input["url"].(string)
	method, _ := input["method"].(string)
	if method == "" {
		return &ToolResult{Content: "method is required", IsError: true, RiskLevel: RiskL2}, nil
	}

	faceID, _ := input["face_id"].(string)
	var face *models.AccessFace
	if faceID != "" && t.faces != nil {
		if f, err := t.faces.GetByID(faceID); err == nil {
			face = f
		}
	}
	if face != nil && strings.HasPrefix(url, "/") {
		url = face.BaseURL + url
	}

	if url == "" {
		return &ToolResult{Content: "url is required", IsError: true, RiskLevel: RiskL2}, nil
	}

	bodyStr, _ := input["body"].(string)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(bodyStr))
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("build request: %v", err), IsError: true, RiskLevel: RiskL2}, nil
	}

	if hdrs, ok := input["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	if face != nil {
		if cred, _, cerr := t.faces.DecryptCredential(face); cerr == nil {
			switch face.RESTAuthType {
			case models.RESTAuthBearer:
				req.Header.Set("Authorization", "Bearer "+cred)
			case models.RESTAuthBasic:
				req.SetBasicAuth(face.RESTUsername, cred)
			case models.RESTAuthAPIKey:
				req.Header.Set(face.HeaderName, cred)
			}
		}
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("request error: %v", err), IsError: true, RiskLevel: RiskL2}, nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("read body: %v", err), IsError: true, RiskLevel: RiskL2}, nil
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
	nudge := ""
	if method != "GET" {
		nudge = apiMutateNudge
	}
	return &ToolResult{Content: string(out) + nudge, RiskLevel: RiskL2}, nil
}
