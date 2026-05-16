package logger

import (
	"fmt"
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

// ForModule returns a logger filtered at the module's override level.
// Falls back to defaultLevel if no override is set.
// Must be called per log-site, not cached — level is resolved at call time.
func ForModule(name string) *zerolog.Logger {
	mu.RLock()
	base := global
	dl := defaultLevel
	mu.RUnlock()
	var level zerolog.Level
	if v, ok := moduleLevels.Load(name); ok {
		if l, ok := v.(zerolog.Level); ok {
			level = l
		} else {
			level = dl
		}
	} else {
		level = dl
	}
	l := base.Level(level)
	return &l
}

func SetModuleLevel(module, level string) error {
	if !IsValidLevel(level) {
		return fmt.Errorf("invalid level %q", level)
	}
	moduleLevels.Store(module, parseLevel(level))
	return nil
}

func ClearModuleLevel(module string) {
	moduleLevels.Delete(module)
}

func ModuleLevels() map[string]string {
	result := map[string]string{}
	moduleLevels.Range(func(k, v any) bool {
		result[k.(string)] = v.(zerolog.Level).String()
		return true
	})
	return result
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
