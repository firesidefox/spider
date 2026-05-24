package prometheus_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/prometheus"
)

func makeServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestQueryInstant(t *testing.T) {
	srv := makeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.Error(w, "not found", 404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []map[string]any{
					{"metric": map[string]string{"__name__": "up"}, "value": []any{1716000000, "1"}},
				},
			},
		})
	}))

	c := prometheus.NewClient(srv.URL, "none", "", "", "", 30, false)
	result, err := c.QueryInstant(context.Background(), `up{job="node"}`, time.Now())
	if err != nil {
		t.Fatalf("QueryInstant: %v", err)
	}
	if result.ResultType != "vector" {
		t.Fatalf("expected vector, got %s", result.ResultType)
	}
	if len(result.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(result.Series))
	}
	if result.Series[0].Latest != "1" {
		t.Fatalf("expected latest=1, got %s", result.Series[0].Latest)
	}
}

func TestQueryRange(t *testing.T) {
	srv := makeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result": []map[string]any{
					{
						"metric": map[string]string{"__name__": "up"},
						"values": []any{
							[]any{1716000000.0, "1"},
							[]any{1716000060.0, "1"},
						},
					},
				},
			},
		})
	}))

	c := prometheus.NewClient(srv.URL, "none", "", "", "", 30, false)
	result, err := c.QueryRange(context.Background(), "up",
		"2024-05-18T00:00:00Z", "2024-05-18T01:00:00Z", "1m")
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if result.ResultType != "matrix" {
		t.Fatalf("expected matrix, got %s", result.ResultType)
	}
	if len(result.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(result.Series))
	}
	if result.Series[0].Latest != "1" {
		t.Fatalf("expected latest=1, got %s", result.Series[0].Latest)
	}
}

func TestListMetricNames(t *testing.T) {
	srv := makeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   []string{"node_cpu_seconds_total", "node_memory_MemTotal_bytes"},
		})
	}))

	c := prometheus.NewClient(srv.URL, "none", "", "", "", 30, false)
	names, err := c.ListMetricNames(context.Background(), `{instance="10.0.0.1:9100"}`)
	if err != nil {
		t.Fatalf("ListMetricNames: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2, got %d", len(names))
	}
}

func TestQueryRange_TooManyPoints(t *testing.T) {
	c := prometheus.NewClient("http://localhost:9090", "none", "", "", "", 30, false)
	// 7-day window with 1s step = 604800 points > 10000
	_, err := c.QueryRange(context.Background(), "up", "2026-01-01T00:00:00Z", "2026-01-08T00:00:00Z", "1s")
	if err == nil {
		t.Fatal("expected error for too many data points")
	}
}

func TestQueryRange_WindowTooLarge(t *testing.T) {
	c := prometheus.NewClient("http://localhost:9090", "none", "", "", "", 30, false)
	// 8-day window > 7-day limit
	_, err := c.QueryRange(context.Background(), "up", "2026-01-01T00:00:00Z", "2026-01-09T00:00:00Z", "1m")
	if err == nil {
		t.Fatal("expected error for window > 7 days")
	}
}

func TestHTTPError(t *testing.T) {
	srv := makeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))

	c := prometheus.NewClient(srv.URL, "none", "", "", "", 30, false)
	_, err := c.QueryInstant(context.Background(), "up", time.Now())
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
