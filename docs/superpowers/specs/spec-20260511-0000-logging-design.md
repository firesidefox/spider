# Logging System Design

**Date:** 2026-05-11  
**Status:** Implemented — zerolog, HTTP middleware, dynamic log level API, file rotation

## Overview

Add structured logging to spider.ai using zerolog. Supports error/info/debug levels, file output with rotation, optional stderr, runtime level changes via API and UI.

## Goals

- Structured JSON logs (switchable to text)
- File output with size-based rotation, default path `<data-dir>/logs/spider.log`
- Optional stderr output (off by default)
- Dynamic log level: `--debug` flag, config file, and HTTP API + UI hot-change
- Context-propagated logger carrying request/conversation fields
- Coverage: HTTP, agent, SSH, LLM, MCP, auth (P0 first, P1 second)

## Non-Goals

- Log shipping / aggregation (ELK, Loki) — out of scope
- Per-package log level control — global level only
- Slow query logging for store/ — P2, deferred

---

## Library Choice

**zerolog** (`github.com/rs/zerolog`)

- Zero allocations, zero dependencies
- Native JSON output, text mode supported
- Chain API: `log.Info().Str("key", val).Msg("...")`
- Faster than slog; simpler than zap
- Rotation via `gopkg.in/natefinch/lumberjack.v2`

---

## Package Structure

```
internal/logger/
├── logger.go       # Init, Config, global accessor, LevelVar
├── context.go      # WithContext / FromContext
└── middleware.go   # HTTP middleware: inject request_id, log req/resp
```

No other package holds a logger instance. All packages call `logger.FromContext(ctx)`.

---

## Configuration

### Config struct

```go
type Config struct {
    Level      string // "debug" | "info" | "error", default "info"
    Format     string // "json" | "text", default "json"
    File       string // path to log file, default "<data-dir>/logs/spider.log"
    MaxSizeMB  int    // rotate at this size, default 100
    MaxBackups int    // keep N rotated files, default 7
    Stderr     bool   // also write to stderr, default false
}
```

### Config file (`config.yaml`)

```yaml
log:
  level: info
  format: json
  file: ""          # empty = use default data-dir path
  max_size_mb: 100
  max_backups: 7
  stderr: false
```

### CLI flag

`--debug` sets level to `debug`, overrides config file.

### Runtime (hot change)

`PUT /api/log-level` with body `{"level": "debug"}` — atomically updates `zerolog.GlobalLevel`. No restart required. Frontend settings page calls this endpoint.

---

## Initialization

`main.go` calls `logger.Init(cfg)` once after config is loaded, before any other subsystem starts.

```go
func Init(cfg Config) error
```

- Creates log directory if not exists
- Opens lumberjack writer for file output
- Builds zerolog multi-writer (file + optional stderr)
- Sets `zerolog.GlobalLevel`
- Stores a `zerolog.LevelVar` for runtime changes

---

## Context Propagation

```go
// Inject enriched logger into context
func WithContext(ctx context.Context, fields ...Field) context.Context

// Retrieve logger from context (falls back to global logger)
func FromContext(ctx context.Context) zerolog.Logger
```

Usage pattern:

```go
// In HTTP handler (after middleware injects request_id):
log := logger.FromContext(ctx)
log.Info().Str("host", host).Msg("ssh connected")

// Enrich for a conversation:
ctx = logger.WithContext(ctx,
    logger.Str("conv_id", convID),
    logger.Str("user_id", userID),
)
```

---

## HTTP Middleware

`logger.Middleware()` wraps every HTTP request:

- Generates `request_id` (UUID v4)
- Injects enriched logger into `ctx`
- Logs request start: method, path, request_id
- Logs request end: status, duration, request_id

---

## Log Coverage Plan

### P0 — Core path (implement first)

| Package | What to log |
|---------|-------------|
| `api/` | Request start/end via middleware; handler errors |
| `agent/` | Agent start, tool calls, agent done, errors |
| `ssh/` | Connect, disconnect, command start/end, errors |
| Startup | Config loaded, server listening, fatal errors |

### P1 — Important

| Package | What to log |
|---------|-------------|
| `llm/` | Request sent (model, tokens), response received, errors |
| `mcp/` | Session open/close, tool dispatch, errors |
| `auth/` | Login success/fail, token issued |

### P2 — Deferred

| Package | What to log |
|---------|-------------|
| `store/` | Slow queries (>100ms) |
| `config/` | Config reload |

---

## Log Levels

| Level | When |
|-------|------|
| `error` | Unrecoverable errors, unexpected failures |
| `info` | Normal lifecycle events (start, connect, request) |
| `debug` | Detailed trace: params, intermediate state, timing |

---

## Dynamic Level API

```
PUT /api/log-level
Authorization: Bearer <token>
Content-Type: application/json

{"level": "debug"}
```

Response: `200 OK` or `400 Bad Request` (invalid level).

Frontend: settings page shows current level with a selector (debug / info / error). Change triggers PUT immediately.

---

## File Rotation (lumberjack)

```go
&lumberjack.Logger{
    Filename:   cfg.File,
    MaxSize:    cfg.MaxSizeMB,  // MB
    MaxBackups: cfg.MaxBackups,
    Compress:   true,
}
```

Rotated files: `spider.log.1`, `spider.log.2`, ... compressed as `.gz`.

---

## Dependencies

```
github.com/rs/zerolog
gopkg.in/natefinch/lumberjack.v2
```

Both added to `go.mod`.

---

## Testing

- `logger.Init` with `Config{Stderr: true}` in tests (no file I/O)
- HTTP middleware: verify `request_id` present in context
- Dynamic level: call `PUT /api/log-level`, verify `zerolog.GlobalLevel` changed
- Each P0 package: verify log output contains expected fields (use `zerolog.TestWriter`)

