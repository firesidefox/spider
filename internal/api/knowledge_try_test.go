package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spiderai/spider/internal/knowledge"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// mockPrometheusSourceStore implements the prometheusSourceStore interface for tests.
type mockPrometheusSourceStore struct {
	sources map[string]*models.PrometheusSource
}

func (m *mockPrometheusSourceStore) GetByID(id string) (*models.PrometheusSource, error) {
	s, ok := m.sources[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return s, nil
}

func (m *mockPrometheusSourceStore) DecryptCredentials(_ *models.PrometheusSource) (password, token string, err error) {
	return "", "", nil
}

// mockDocStore implements docStore for try tests.
type mockDocStore struct {
	entries []knowledge.Entry
}

func (m *mockDocStore) ListDocuments(_ context.Context, _ int) ([]knowledge.Document, error) {
	return nil, nil
}

func (m *mockDocStore) GetDocument(_ context.Context, _ int) (*knowledge.Document, error) {
	return nil, nil
}

func (m *mockDocStore) DeleteDocuments(_ context.Context, _ []int) error { return nil }

func (m *mockDocStore) MoveDocuments(_ context.Context, _ []int, _ int) error { return nil }

func (m *mockDocStore) CatalogSections(_ context.Context, _ knowledge.Scope) ([]knowledge.Section, error) {
	return nil, nil
}

func (m *mockDocStore) CatalogEntries(_ context.Context, _ int) ([]knowledge.EntrySummary, error) {
	return nil, nil
}

func (m *mockDocStore) FetchEntries(_ context.Context, ids []int) ([]knowledge.Entry, error) {
	var out []knowledge.Entry
	for _, e := range m.entries {
		for _, id := range ids {
			if e.ID == id {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func TestTryKnowledgeEntry_Success(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer upstream.Close()

	docStore := &mockDocStore{entries: []knowledge.Entry{
		{ID: 7, Title: "GET /api/v1/query", Content: ""},
	}}
	srcStore := &mockPrometheusSourceStore{sources: map[string]*models.PrometheusSource{
		"src1": {ID: "src1", BaseURL: upstream.URL, AuthType: "none"},
	}}

	body := bytes.NewBufferString(`{"source_id":"src1","params":{"query":"up"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-entries/7/try", body)
	w := httptest.NewRecorder()

	tryKnowledgeEntry(docStore, srcStore, w, req, "7")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result tryResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != 200 {
		t.Errorf("expected status 200, got %d", result.Status)
	}
	if result.LatencyMs < 0 {
		t.Errorf("negative latency")
	}
}

func TestTryKnowledgeEntry_EntryNotFound(t *testing.T) {
	docStore := &mockDocStore{entries: []knowledge.Entry{}}
	srcStore := &mockPrometheusSourceStore{sources: map[string]*models.PrometheusSource{}}

	body := bytes.NewBufferString(`{"source_id":"src1","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-entries/99/try", body)
	w := httptest.NewRecorder()

	tryKnowledgeEntry(docStore, srcStore, w, req, "99")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTryKnowledgeEntry_SourceNotFound(t *testing.T) {
	docStore := &mockDocStore{entries: []knowledge.Entry{
		{ID: 7, Title: "GET /api/v1/query", Content: ""},
	}}
	srcStore := &mockPrometheusSourceStore{sources: map[string]*models.PrometheusSource{}}

	body := bytes.NewBufferString(`{"source_id":"missing","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-entries/7/try", body)
	w := httptest.NewRecorder()

	tryKnowledgeEntry(docStore, srcStore, w, req, "7")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
