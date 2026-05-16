package logger_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spiderai/spider/internal/logger"
)

func TestInitWritesLog(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(nil)
	logger.Init(logger.Config{Level: "info", Format: "json"})
	l := logger.Global()
	l.Info().Str("k", "v").Msg("hello")
	if !bytes.Contains(buf.Bytes(), []byte(`"hello"`)) {
		t.Errorf("expected log output, got: %s", buf.String())
	}
}

func TestSetLevel(t *testing.T) {
	logger.Init(logger.Config{Level: "info", Format: "json"})
	logger.SetLevel("debug")
	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Errorf("expected debug level after SetLevel")
	}
	logger.SetLevel("info") // reset
}

func TestFromContext(t *testing.T) {
	logger.Init(logger.Config{Level: "info", Format: "json"})
	ctx := context.Background()
	got := logger.FromContext(ctx)
	if got == nil {
		t.Fatal("expected non-nil logger from empty context")
	}

	enriched := logger.Global().With().Str("req_id", "abc").Logger()
	ctx2 := logger.WithContext(ctx, &enriched)
	got2 := logger.FromContext(ctx2)
	if got2 == nil {
		t.Error("expected enriched logger from context, got nil")
	}
}

func TestMiddleware(t *testing.T) {
	logger.Init(logger.Config{Level: "info", Format: "json"})
	handler := logger.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
