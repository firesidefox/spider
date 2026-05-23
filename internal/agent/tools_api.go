package agent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

const maxResponseBody = 64 * 1024

const callAPIPromptSection = `### CallAPI (GET: read-only; POST/PUT/DELETE: has side effects)

**When to use:**
- GET: use freely in Explore phase
- POST/PUT/DELETE: only in Act phase after confirming intent

**When NOT to use:** Do not call mutating methods before the user has confirmed the plan.

**URL construction:**
- Always pass face_id (from GetHosts result)
- Pass relative path in url (e.g., /api/cpu/trend?duration=1h)
- Tool auto-constructs full URL: scheme://IP:port + base_url + your path

**Workflow for API calls:**
1. GetHosts → find host with "api" access face
2. If face.kb_mode="specific", SearchDocs with each bound group/doc scope → find correct endpoint, params, auth
3. CallAPI with face_id + relative path

<example>
User: Push a new ACL rule via the firewall API.
Assistant: GetHosts → find gateway (api face, kb_mode=specific) → SearchDocs with bound scope → confirm request body with user → CallAPI POST face_id="xxx" url="/api/acl/rules"
</example>`

type CallRESTAPITool struct {
	http  *http.Client
	faces *store.AccessFaceStore
}

func NewCallRESTAPITool(faces *store.AccessFaceStore) *CallRESTAPITool {
	return &CallRESTAPITool{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			},
		},
		faces: faces,
	}
}

func (t *CallRESTAPITool) DefaultRiskLevel() RiskLevel             { return RiskL2 }
func (t *CallRESTAPITool) IsConcurrencySafe(_ map[string]any) bool { return false }
func (t *CallRESTAPITool) Name() string                            { return "CallAPI" }
func (t *CallRESTAPITool) SystemPromptSection() string             { return callAPIPromptSection }

func (t *CallRESTAPITool) Description() string {
	return "Call a REST API endpoint on a gateway device. Has side effects for POST/PUT/DELETE methods. Use GET freely in Explore phase; use mutating methods only in Act phase after confirming intent. Always set `intent` to a short goal description for mutating calls."
}

func (t *CallRESTAPITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"face_id": map[string]any{"type": "string", "description": "Access face ID from GetHosts. Required for auto URL construction and auth injection."},
			"url":     map[string]any{"type": "string", "description": "Relative path starting with / (e.g., /api/cpu/trend?duration=1h). Tool constructs full URL from face config."},
			"method":  map[string]any{"type": "string", "description": "HTTP method", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
			"headers": map[string]any{"type": "object", "description": "HTTP headers"},
			"body":    map[string]any{"type": "string", "description": "Request body"},
			"intent":  map[string]any{"type": "string", "description": "What you are trying to achieve with this API call (goal only). Required for POST/PUT/DELETE/PATCH."},
		},
		"required": []string{"face_id", "url", "method", "intent"},
	}
}

func (t *CallRESTAPITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	url, _ := input["url"].(string)
	method, _ := input["method"].(string)
	if method == "" {
		return &ToolResult{Content: "method is required", IsError: true, RiskLevel: RiskL2}, nil
	}

	intent, _ := input["intent"].(string)
	if intent == "" && method != "GET" {
		log.Printf("WARNING: CallAPI called without intent field (method=%s)", method)
	}

	faceID, _ := input["face_id"].(string)
	var face *models.AccessFace
	if faceID != "" && t.faces != nil {
		if f, err := t.faces.GetByID(faceID); err == nil {
			face = f
		}
	}
	if face != nil && strings.HasPrefix(url, "/") {
		// If base_url is already a full URL (legacy rows), use it directly.
		if strings.HasPrefix(face.BaseURL, "http://") || strings.HasPrefix(face.BaseURL, "https://") {
			url = strings.TrimRight(face.BaseURL, "/") + url
		} else {
			scheme := face.RESTScheme
			if scheme == "" {
				scheme = "http"
			}
			url = scheme + "://" + face.IP + ":" + strconv.Itoa(face.Port) + face.BaseURL + url
		}
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
			case models.RESTAuthHMACAKSK:
				ts := time.Now().Unix()
				sig := hmacSign(cred, method, req.URL.RequestURI(), ts, face.HMACAlgo)
				req.Header.Set("X-Auth-AccessKey", face.RESTUsername)
				req.Header.Set("X-Auth-Timestamp", strconv.FormatInt(ts, 10))
				algo := face.HMACAlgo
				if algo == "" {
					algo = "HMAC-SHA256"
				}
				req.Header.Set("X-Auth-Algo", algo)
				req.Header.Set("X-Auth-Signature", sig)
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
	return &ToolResult{Content: string(out), Nudge: nudge, RiskLevel: RiskL2}, nil
}

func hmacSign(sk, method, path string, ts int64, algo string) string {
	raw := method + "\n" + strconv.FormatInt(ts, 10) + "\n" + path
	var mac []byte
	switch algo {
	case "HMAC-SM3":
		// SM3 not in stdlib; fall back to SHA256 with a log warning
		log.Printf("WARNING: HMAC-SM3 not supported, falling back to HMAC-SHA256")
		fallthrough
	default:
		h := hmac.New(sha256.New, []byte(sk))
		h.Write([]byte(raw))
		mac = h.Sum(nil)
	}
	return base64.StdEncoding.EncodeToString(mac)
}
