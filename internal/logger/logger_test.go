package logger_test

import (
	"bytes"
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
