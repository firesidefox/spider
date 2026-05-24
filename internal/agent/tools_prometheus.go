package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/models"
	promclient "github.com/spiderai/spider/internal/prometheus"
	"github.com/spiderai/spider/internal/store"
)

var timeNow = time.Now // allows test override if needed

// sourceStorer is the minimal interface needed by prometheus tools.
type sourceStorer interface {
	GetByID(id string) (*models.PrometheusSource, error)
	DecryptCredentials(src *models.PrometheusSource) (password, token string, err error)
}

// bindingStorer is the minimal interface needed by prometheus tools.
type bindingStorer interface {
	FindSourceIDForHost(hostID string) (string, error)
}

func resolveClient(ss sourceStorer, bs bindingStorer, hostID string) (*promclient.Client, error) {
	sourceID, err := bs.FindSourceIDForHost(hostID)
	if err != nil {
		return nil, err
	}
	src, err := ss.GetByID(sourceID)
	if err != nil {
		return nil, fmt.Errorf("获取 Prometheus 数据源: %w", err)
	}
	pwd, tok, err := ss.DecryptCredentials(src)
	if err != nil {
		return nil, fmt.Errorf("解密凭据: %w", err)
	}
	return promclient.NewClient(src.BaseURL, string(src.AuthType), src.Username, pwd, tok, src.TimeoutSeconds, src.SkipTLSVerify), nil
}

// --- ListMetricsTool ---

type ListMetricsTool struct {
	sources  sourceStorer
	bindings bindingStorer
	hosts    *store.HostStore
}

func NewListMetricsTool(ss sourceStorer, bs bindingStorer, hosts *store.HostStore) *ListMetricsTool {
	return &ListMetricsTool{sources: ss, bindings: bs, hosts: hosts}
}

func (t *ListMetricsTool) Name() string                            { return "ListMetrics" }
func (t *ListMetricsTool) DefaultRiskLevel() RiskLevel             { return RiskL1 }
func (t *ListMetricsTool) IsConcurrencySafe(_ map[string]any) bool { return true }

func (t *ListMetricsTool) Description() string {
	return "List Prometheus metric names available for a host. Read-only. No side effects. Use freely in Explore phase."
}

func (t *ListMetricsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_id": map[string]any{
				"type":        "string",
				"description": "Host ID to query metrics for.",
			},
			"filter": map[string]any{
				"type":        "string",
				"description": "Optional prefix filter for metric names.",
			},
		},
		"required": []string{"host_id"},
	}
}

const listMetricsPromptSection = `## ListMetrics

**When to use:** Before writing PromQL queries — discover available metric names for a host. Use in Explore phase.

**When NOT to use:** When you already know the metric name; do not call repeatedly in a loop.

**Rules:**
- Use filter prefix to narrow results for common namespaces (e.g. "node_", "go_")
- Combine with QueryMetrics to inspect actual values

<example>
User: What CPU metrics are available for host h1?
Assistant: ListMetrics host_id=h1 filter="node_cpu" → QueryMetrics host_id=h1 query="node_cpu_seconds_total{...}"
</example>`

func (t *ListMetricsTool) SystemPromptSection() string {
	return listMetricsPromptSection
}

func (t *ListMetricsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	hostID, _ := input["host_id"].(string)
	filter, _ := input["filter"].(string)

	selector := "{}"
	if t.hosts != nil {
		if host, err := t.hosts.GetByID(hostID); err == nil && host != nil {
			selector = fmt.Sprintf(`{instance=~"%s:.*"}`, host.IP)
		}
	}

	client, err := resolveClient(t.sources, t.bindings, hostID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("无法获取 Prometheus 客户端: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	names, err := client.ListMetricNames(ctx, selector)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("查询指标列表失败: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	if filter != "" {
		filtered := names[:0]
		for _, n := range names {
			if strings.HasPrefix(n, filter) {
				filtered = append(filtered, n)
			}
		}
		names = filtered
	}

	return &ToolResult{
		Content:   fmt.Sprintf("找到 %d 个指标:\n%s", len(names), strings.Join(names, "\n")),
		RiskLevel: RiskL1,
		Summary:   fmt.Sprintf("找到 %d 个指标", len(names)),
	}, nil
}

// --- QueryMetricsTool ---

type QueryMetricsTool struct {
	sources  sourceStorer
	bindings bindingStorer
}

func NewQueryMetricsTool(ss sourceStorer, bs bindingStorer) *QueryMetricsTool {
	return &QueryMetricsTool{sources: ss, bindings: bs}
}

func (t *QueryMetricsTool) Name() string                            { return "QueryMetrics" }
func (t *QueryMetricsTool) DefaultRiskLevel() RiskLevel             { return RiskL1 }
func (t *QueryMetricsTool) IsConcurrencySafe(_ map[string]any) bool { return true }

func (t *QueryMetricsTool) Description() string {
	return "Execute a PromQL query (instant or range) against a host's Prometheus data source. Read-only. No side effects. Use in Explore phase."
}

func (t *QueryMetricsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_id": map[string]any{
				"type":        "string",
				"description": "Host ID to query.",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "PromQL expression.",
			},
			"start": map[string]any{
				"type":        "string",
				"description": "Range query start time (RFC3339 or Unix timestamp). Must be set with end.",
			},
			"end": map[string]any{
				"type":        "string",
				"description": "Range query end time (RFC3339 or Unix timestamp). Must be set with start.",
			},
			"step": map[string]any{
				"type":        "string",
				"description": "Range query step (e.g. '1m', '5m'). Optional; defaults to auto.",
			},
			"raw": map[string]any{
				"type":        "boolean",
				"description": "Return raw JSON result instead of formatted summary.",
			},
		},
		"required": []string{"host_id", "query"},
	}
}

const queryMetricsPromptSection = `## QueryMetrics

**When to use:** After identifying metric names via ListMetrics, or when you already know the PromQL expression.

**When NOT to use:** Browsing available metrics — use ListMetrics instead.

**Rules:**
- Omit start/end for instant query (current value)
- Provide both start AND end for range query; never provide only one
- Use step to control data density (e.g. "1m" for 1-hour window)
- Use raw=true only when the caller needs the full JSON for further processing

<example>
User: Show CPU usage trend for last hour on h1.
Assistant: QueryMetrics host_id=h1 query="rate(node_cpu_seconds_total{mode='idle'}[5m])" start="now-1h" end="now" step="1m"
</example>`

func (t *QueryMetricsTool) SystemPromptSection() string {
	return queryMetricsPromptSection
}

func (t *QueryMetricsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	hostID, _ := input["host_id"].(string)
	query, _ := input["query"].(string)
	start, _ := input["start"].(string)
	end, _ := input["end"].(string)
	step, _ := input["step"].(string)
	raw, _ := input["raw"].(bool)

	// Validate: start and end must both be set or both be empty.
	if (start == "") != (end == "") {
		return nil, fmt.Errorf("start 和 end 必须同时提供或同时省略")
	}

	client, err := resolveClient(t.sources, t.bindings, hostID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("无法获取 Prometheus 客户端: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	var result *promclient.QueryResult
	if start == "" {
		result, err = client.QueryInstant(ctx, query, timeNow())
	} else {
		result, err = client.QueryRange(ctx, query, start, end, step)
	}
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("查询失败: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	var content string
	if raw {
		b, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			return &ToolResult{Content: fmt.Sprintf("序列化失败: %v", jsonErr), IsError: true, RiskLevel: RiskL1}, nil
		}
		content = string(b)
	} else {
		content = formatQueryResult(result)
	}

	return &ToolResult{
		Content:   content,
		RiskLevel: RiskL1,
		Summary:   fmt.Sprintf("result_type=%s series=%d", result.ResultType, result.SeriesCount),
	}, nil
}

// formatQueryResult produces a human-readable summary of a QueryResult.
func formatQueryResult(r *promclient.QueryResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "result_type: %s  series_count: %d\n", r.ResultType, r.SeriesCount)
	for _, s := range r.Series {
		fmt.Fprintf(&sb, "\nmetric: %v\n", s.Metric)
		fmt.Fprintf(&sb, "  latest=%s  min=%s  max=%s  avg=%s\n", s.Latest, s.Min, s.Max, s.Avg)
		for _, sample := range s.Samples {
			if sample.Timestamp != 0 {
				fmt.Fprintf(&sb, "  [%.3e] %s\n", sample.Timestamp, sample.Value)
			} else {
				fmt.Fprintf(&sb, "  %s\n", sample.Value)
			}
		}
	}
	return sb.String()
}
