package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/models"
)

// mockSourceStore implements agent.sourceStorer for tests.
type mockSourceStore struct {
	src *models.PrometheusSource
}

func (m *mockSourceStore) GetByID(_ string) (*models.PrometheusSource, error) {
	return m.src, nil
}

func (m *mockSourceStore) DecryptCredentials(_ *models.PrometheusSource) (password, token string, err error) {
	return "", "", nil
}

// mockBindingStore implements agent.bindingStorer for tests.
type mockBindingStore struct {
	sourceID string
}

func (m *mockBindingStore) FindSourceIDForHost(_ string) (string, error) {
	return m.sourceID, nil
}

func TestListMetricsTool_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"status": "success",
			"data":   []string{"node_cpu_seconds_total", "node_load1"},
		})
	}))
	defer srv.Close()

	src := &models.PrometheusSource{
		ID:      "src1",
		BaseURL: srv.URL,
	}
	ss := &mockSourceStore{src: src}
	bs := &mockBindingStore{sourceID: "src1"}

	tool := agent.NewListMetricsTool(ss, bs, nil)

	result, err := tool.Execute(context.Background(), map[string]any{"host_id": "h1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "node_cpu_seconds_total") {
		t.Errorf("expected node_cpu_seconds_total in content, got: %s", result.Content)
	}
}

func TestListMetricsTool_FilterPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"status": "success",
			"data":   []string{"node_cpu_seconds_total", "node_load1", "go_gc_duration_seconds"},
		})
	}))
	defer srv.Close()

	src := &models.PrometheusSource{ID: "src1", BaseURL: srv.URL}
	tool := agent.NewListMetricsTool(&mockSourceStore{src: src}, &mockBindingStore{sourceID: "src1"}, nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"host_id": "h1",
		"filter":  "node_",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Content, "go_gc_duration_seconds") {
		t.Errorf("filtered out go_ metric should not appear, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "node_cpu_seconds_total") {
		t.Errorf("node_ metric should appear, got: %s", result.Content)
	}
}

func TestQueryMetricsTool_InstantQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{
						"metric": map[string]any{"__name__": "up"},
						"value":  []any{1716000000.0, "1"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	src := &models.PrometheusSource{ID: "src1", BaseURL: srv.URL}
	tool := agent.NewQueryMetricsTool(&mockSourceStore{src: src}, &mockBindingStore{sourceID: "src1"})

	result, err := tool.Execute(context.Background(), map[string]any{
		"host_id": "h1",
		"query":   "up",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "result_type: vector") {
		t.Errorf("expected 'result_type: vector' in content, got: %s", result.Content)
	}
}

func TestQueryMetricsTool_MissingStartOrEnd(t *testing.T) {
	src := &models.PrometheusSource{ID: "src1", BaseURL: "http://localhost:9090"}
	tool := agent.NewQueryMetricsTool(&mockSourceStore{src: src}, &mockBindingStore{sourceID: "src1"})

	_, err := tool.Execute(context.Background(), map[string]any{
		"host_id": "h1",
		"query":   "up",
		"start":   "2024-01-01T00:00:00Z",
		// end is missing
	})
	if err == nil {
		t.Fatal("expected error when start is set but end is missing")
	}
	if !strings.Contains(err.Error(), "必须同时") {
		t.Errorf("error should contain '必须同时', got: %v", err)
	}
}

func TestQueryMetricsTool_RawOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{
						"metric": map[string]any{"__name__": "up"},
						"value":  []any{1716000000.0, "1"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	src := &models.PrometheusSource{ID: "src1", BaseURL: srv.URL}
	tool := agent.NewQueryMetricsTool(&mockSourceStore{src: src}, &mockBindingStore{sourceID: "src1"})

	result, err := tool.Execute(context.Background(), map[string]any{
		"host_id": "h1",
		"query":   "up",
		"raw":     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// raw output should be JSON
	var v any
	if jsonErr := json.Unmarshal([]byte(result.Content), &v); jsonErr != nil {
		t.Errorf("raw output should be valid JSON, got: %s", result.Content)
	}
}
