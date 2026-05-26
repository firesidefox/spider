package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/auth"
	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/crypto"
	dbpkg "github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/knowledge"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/store"
)

func TestChatSendMessageReturnsAcceptedWithoutStreamingResponse(t *testing.T) {
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"ok"}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(llmSrv.Close)

	app := newChatSendTestApp(t, llmSrv.URL)
	if _, err := app.DB.Exec(
		`INSERT INTO users (id, username, password, role, enabled, created_at)
		 VALUES ('anonymous', 'anonymous', 'x', 'admin', 1, datetime('now'))`,
	); err != nil {
		t.Fatal(err)
	}
	conv, err := app.ConvStore.Create("anonymous", "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+conv.ID+"/messages", strings.NewReader(`{"content":"hi"}`))
	w := httptest.NewRecorder()
	handler := auth.AuthMiddleware(false, nil, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatSendMessage(app, w, r, conv.ID)
	}))

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		t.Fatalf("POST /messages must not return event-stream content type, got %q", ct)
	}
}

func newChatSendTestApp(t *testing.T, llmBaseURL string) *mcppkg.App {
	t.Helper()
	dataDir := t.TempDir()
	database, err := dbpkg.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	cm, err := crypto.NewManager(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	providers := store.NewProviderStore(database, cm)
	p, err := providers.Create("test-openai", "openai", "test-key", llmBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := providers.SetSelectedModel(p.ID, "gpt-test"); err != nil {
		t.Fatal(err)
	}
	if err := providers.Activate(p.ID); err != nil {
		t.Fatal(err)
	}
	return &mcppkg.App{
		Config:          &config.Config{DataDir: dataDir},
		DB:              database,
		UserStore:       store.NewUserStore(database),
		ProviderStore:   providers,
		RagConfigStore:  store.NewRagConfigStore(database, cm),
		ConvStore:       store.NewConversationStore(database),
		MsgStore:        store.NewMessageStore(database),
		TodoStore:       store.NewTodoStore(database),
		HostStore:       store.NewHostStore(database),
		AccessFaceStore: store.NewAccessFaceStore(database, cm),
		GroupStore:      store.NewGroupStore(database),
		DocStore:        store.NewDocumentStore(database),
		KnowledgeStore:  knowledge.NewStore(database),
		ShutdownCtx:     context.Background(),
		PermissionMode:  "",
	}
}
