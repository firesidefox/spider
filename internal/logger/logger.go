package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
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
	global   zerolog.Logger
	levelVal atomic.Int32
	extraOut io.Writer
)

func Init(cfg Config) error {
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)
	levelVal.Store(int32(level))

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

	global = zerolog.New(w).With().Timestamp().Logger()
	return nil
}

func Global() zerolog.Logger { return global }

func SetLevel(level string) {
	l := parseLevel(level)
	zerolog.SetGlobalLevel(l)
	levelVal.Store(int32(l))
}

func CurrentLevel() string {
	return zerolog.Level(levelVal.Load()).String()
}

// SetOutput redirects all log output — for tests only.
func SetOutput(w io.Writer) { extraOut = w }

func parseLevel(s string) zerolog.Level {
	switch s {
	case "debug":
		return zerolog.DebugLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
