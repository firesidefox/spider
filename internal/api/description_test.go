package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spiderai/spider/internal/agent"
	dbpkg "github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/llm"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/store"
)

// mockLLMClient implements llm.Client for testing.
type mockLLMClient struct {
	resp string
	err  error
}

func (m *mockLLMClient) Chat(_ context.Context, _ *llm.ChatRequest) (string, error) {
	return m.resp, m.err
}

func (m *mockLLMClient) ChatStream(_ context.Context, _ *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockLLMClient) CountTokens(_ context.Context, _ []llm.Message) (int, error) {
	return 0, nil
}

func newTestApp(t *testing.T, llmClient llm.Client) *mcppkg.App {
	t.Helper()
	database, err := dbpkg.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	app := &mcppkg.App{
		DocStore:   store.NewDocumentStore(database),
		GroupStore: store.NewGroupStore(database),
	}
	if llmClient != nil {
		app.AgentFactory = &agent.Factory{LLMClient: llmClient}
	}
	return app
}

func TestRegenerateDocDescription(t *testing.T) {
	llmC := &mockLLMClient{resp: "Nginx ops manual."}
	app := newTestApp(t, llmC)

	if err := app.DocStore.Save("v", []string{}, "ops.md", "nginx content", nil, "ops.md", 0, nil); err != nil {
		t.Fatal(err)
	}
	docs, err := app.DocStore.List()
	if err != nil || len(docs) == 0 {
		t.Fatal("failed to list docs")
	}
	docID := docs[0].ID

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	regenerateDocDescription(app, w, req, fmt.Sprint(docID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["description"] != "Nginx ops manual." {
		t.Fatalf("unexpected description: %q", resp["description"])
	}
	// Verify persisted
	doc, err := app.DocStore.GetByID(docID)
	if err != nil || doc == nil {
		t.Fatal("doc not found after update")
	}
	if doc.Description != "Nginx ops manual." {
		t.Fatalf("description not persisted, got %q", doc.Description)
	}
}

func TestRegenerateDocDescription_NotFound(t *testing.T) {
	app := newTestApp(t, &mockLLMClient{resp: "x"})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	regenerateDocDescription(app, w, req, "99")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRegenerateDocDescription_NoLLM(t *testing.T) {
	app := newTestApp(t, nil) // no LLM
	if err := app.DocStore.Save("v", []string{}, "t", "c", nil, "f.md", 0, nil); err != nil {
		t.Fatal(err)
	}
	docs, _ := app.DocStore.List()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	regenerateDocDescription(app, w, req, fmt.Sprint(docs[0].ID))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestRegenerateGroupDescription(t *testing.T) {
	llmC := &mockLLMClient{resp: "Ops knowledge base."}
	app := newTestApp(t, llmC)

	g, err := app.GroupStore.Create("ops")
	if err != nil {
		t.Fatal(err)
	}
	gid := g.ID
	if err := app.DocStore.Save("v", []string{}, "nginx", "nginx content", nil, "nginx.md", 0, &gid); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	regenerateGroupDescription(app, w, req, fmt.Sprint(gid))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["description"] != "Ops knowledge base." {
		t.Fatalf("unexpected description: %q", resp["description"])
	}
}

func TestRegenerateGroupDescription_NotFound(t *testing.T) {
	app := newTestApp(t, &mockLLMClient{resp: "x"})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	regenerateGroupDescription(app, w, req, "99")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRegenerateGroupDescription_NoDocuments(t *testing.T) {
	app := newTestApp(t, &mockLLMClient{resp: "x"})
	g, err := app.GroupStore.Create("empty")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	regenerateGroupDescription(app, w, req, fmt.Sprint(g.ID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateDocDescription(t *testing.T) {
	app := newTestApp(t, nil)
	if err := app.DocStore.Save("v", []string{}, "t", "c", nil, "f.md", 0, nil); err != nil {
		t.Fatal(err)
	}
	docs, _ := app.DocStore.List()
	docID := docs[0].ID

	body := `{"description":"A nice description."}`
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	updateDocDescription(app, w, req, fmt.Sprint(docID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	doc, err := app.DocStore.GetByID(docID)
	if err != nil || doc == nil {
		t.Fatal("doc not found")
	}
	if doc.Description != "A nice description." {
		t.Fatalf("description not updated, got %q", doc.Description)
	}
}

func TestUpdateDocDescription_NotFound(t *testing.T) {
	app := newTestApp(t, nil)
	body := `{"description":"x"}`
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	updateDocDescription(app, w, req, "99")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateGroupDescription(t *testing.T) {
	app := newTestApp(t, nil)
	g, err := app.GroupStore.Create("mygroup")
	if err != nil {
		t.Fatal(err)
	}

	body := `{"description":"Group about ops."}`
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	updateGroupDescription(app, w, req, fmt.Sprint(g.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	groups, err := app.GroupStore.List()
	if err != nil || len(groups) == 0 {
		t.Fatal("groups not found")
	}
	if groups[0].Description != "Group about ops." {
		t.Fatalf("description not updated, got %q", groups[0].Description)
	}
}

func TestUpdateGroupDescription_NotFound(t *testing.T) {
	app := newTestApp(t, nil)
	body := `{"description":"x"}`
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	updateGroupDescription(app, w, req, "99")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNormalizeDescription(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  hello  ", "hello"},
		{"line1\nline2", "line1 line2"},
		{"a\t\tb", "a b"},
		{strings.Repeat("x", 250), strings.Repeat("x", 200)},
	}
	for _, c := range cases {
		got := normalizeDescription(c.in)
		if got != c.want {
			t.Errorf("normalizeDescription(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
