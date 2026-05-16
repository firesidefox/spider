package logger_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	if logger.CurrentLevel() != "debug" {
		t.Errorf("expected debug, got %s", logger.CurrentLevel())
	}
	logger.SetLevel("info") // reset
}

func TestFromContext(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(nil)
	logger.Init(logger.Config{Level: "info", Format: "json"})

	ctx := context.Background()
	got := logger.FromContext(ctx)
	if got == nil {
		t.Fatal("expected non-nil logger from empty context")
	}

	enriched := logger.Global().With().Str("req_id", "test-abc").Logger()
	ctx2 := logger.WithContext(ctx, &enriched)
	got2 := logger.FromContext(ctx2)
	got2.Info().Msg("probe")
	if !bytes.Contains(buf.Bytes(), []byte("test-abc")) {
		t.Error("expected enriched logger with req_id field from context")
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

func TestForModule(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(nil)
	logger.Init(logger.Config{Level: "info", Format: "json"})

	// default: module inherits global level (info), debug suppressed
	buf.Reset()
	logger.ForModule("agent").Debug().Msg("should-be-suppressed")
	if bytes.Contains(buf.Bytes(), []byte("should-be-suppressed")) {
		t.Error("debug message should be suppressed at info level")
	}

	// override to debug: message should appear
	if err := logger.SetModuleLevel("agent", "debug"); err != nil {
		t.Fatal(err)
	}
	defer logger.ClearModuleLevel("agent")
	buf.Reset()
	logger.ForModule("agent").Debug().Msg("should-appear")
	if !bytes.Contains(buf.Bytes(), []byte("should-appear")) {
		t.Errorf("expected debug message, got: %s", buf.String())
	}

	// clear override: back to info, debug suppressed again
	logger.ClearModuleLevel("agent")
	buf.Reset()
	logger.ForModule("agent").Debug().Msg("suppressed-again")
	if bytes.Contains(buf.Bytes(), []byte("suppressed-again")) {
		t.Error("debug message should be suppressed after clearing override")
	}
}

func TestModuleLevels(t *testing.T) {
	logger.Init(logger.Config{Level: "info", Format: "json"})
	logger.ClearModuleLevel("ssh")
	logger.ClearModuleLevel("llm")

	if err := logger.SetModuleLevel("ssh", "warn"); err != nil {
		t.Fatal(err)
	}
	if err := logger.SetModuleLevel("llm", "debug"); err != nil {
		t.Fatal(err)
	}
	defer logger.ClearModuleLevel("ssh")
	defer logger.ClearModuleLevel("llm")

	levels := logger.ModuleLevels()
	if levels["ssh"] != "warn" {
		t.Errorf("expected ssh=warn, got %s", levels["ssh"])
	}
	if levels["llm"] != "debug" {
		t.Errorf("expected llm=debug, got %s", levels["llm"])
	}
}

func TestSetModuleLevelInvalidLevel(t *testing.T) {
	err := logger.SetModuleLevel("agent", "verbose")
	if err == nil {
		t.Error("expected error for invalid level")
	}
}
