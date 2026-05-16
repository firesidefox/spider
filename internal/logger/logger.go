package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/lumberjack.v2"
)

// Config mirrors config.LogConfig to avoid import cycle.
type Config struct {
	Level      string
	Format     string
	File       string
	MaxSizeMB  int
	MaxBackups int
	Stderr     bool
}

var (
	mu           sync.RWMutex
	global       zerolog.Logger
	defaultLevel zerolog.Level
	moduleLevels sync.Map // map[string]zerolog.Level — runtime overrides only
	extraOut     io.Writer
)

func Init(cfg Config) error {
	var writers []io.Writer

	if cfg.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0700); err != nil {
			return err
		}
		writers = append(writers, &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			Compress:   true,
		})
	}
	if cfg.Stderr {
		writers = append(writers, os.Stderr)
	}
	if extraOut != nil {
		writers = append(writers, extraOut)
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = zerolog.MultiLevelWriter(writers...)
	}

	if cfg.Format == "text" {
		w = zerolog.ConsoleWriter{Out: w, TimeFormat: time.RFC3339}
	}

	mu.Lock()
	defaultLevel = parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(zerolog.TraceLevel) // filtering done per-logger
	global = zerolog.New(w).With().Timestamp().Logger().Level(defaultLevel)
	mu.Unlock()
	return nil
}

func Global() *zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	l := global // copy under lock; caller gets pointer to the copy, not &global
	return &l
}

func SetLevel(level string) {
	l := parseLevel(level)
	mu.Lock()
	defer mu.Unlock()
	if l == defaultLevel {
		return
	}
	defaultLevel = l
	global = global.Level(defaultLevel)
}

func CurrentLevel() string {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLevel.String()
}

func IsValidLevel(s string) bool {
	switch s {
	case "debug", "info", "warn", "error":
		return true
	}
	return false
}

// SetOutput redirects all log output — for tests only.
func SetOutput(w io.Writer) {
	mu.Lock()
	extraOut = w
	mu.Unlock()
}

func parseLevel(s string) zerolog.Level {
	switch s {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
