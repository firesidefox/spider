# Per-Module Dynamic Log Level — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-module log level override to spider.ai's logger package, with HTTP API support.

**Architecture:** Lower zerolog's global level to `TraceLevel` permanently; store a `defaultLevel` and a `sync.Map` of per-module overrides in the logger package. `ForModule(name)` returns a logger filtered at the module's override level (or `defaultLevel` if no override). The HTTP API gains a `module` field to set/clear overrides at runtime; overrides are not persisted across restarts.

**Tech Stack:** Go, zerolog (github.com/rs/zerolog), sync.Map, net/http

---

## File Map

| File | Change |
|------|--------|
| `internal/logger/logger.go` | Add `defaultLevel`, `moduleLevels sync.Map`; update `Init`, `SetLevel`, `CurrentLevel`; add `ForModule`, `SetModuleLevel`, `ClearModuleLevel`, `ModuleLevels` |
| `internal/logger/logger_test.go` | Fix `TestSetLevel`; add `TestForModule`, `TestSetModuleLevel`, `TestModuleLevels` |
| `internal/api/settings.go` | Update `getLogLevel` response shape; update `setLogLevel` to handle `module` + `"inherit"` |

---

## Task 1: Update logger.go — data model, Init, SetLevel

**Files:**
- Modify: `internal/logger/logger.go`

- [ ] **Step 1: Replace var block**

Replace lines 23-26 in `internal/logger/logger.go`:

```go
var (
	global       zerolog.Logger
	defaultLevel zerolog.Level
	moduleLevels sync.Map // map[string]zerolog.Level — runtime overrides only
	extraOut     io.Writer
)
```

Add `"sync"` to the import block.

- [ ] **Step 2: Update Init**

Replace the `Init` function body so it sets `defaultLevel` and fixes `zerolog.GlobalLevel` to `TraceLevel`:

```go
func Init(cfg Config) error {
	defaultLevel = parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(zerolog.TraceLevel) // filtering done per-logger

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

	global = zerolog.New(w).With().Timestamp().Logger().Level(defaultLevel)
	return nil
}
```

- [ ] **Step 3: Update SetLevel and CurrentLevel**

Replace `SetLevel` and `CurrentLevel`:

```go
func SetLevel(level string) {
	l := parseLevel(level)
	if l == defaultLevel {
		return
	}
	defaultLevel = l
	global = global.Level(defaultLevel)
}

func CurrentLevel() string {
	return defaultLevel.String()
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/logger/logger.go
git commit -m "refactor(logger): use defaultLevel var, fix GlobalLevel to TraceLevel"
```

---

## Task 2: Add ForModule, SetModuleLevel, ClearModuleLevel, ModuleLevels

**Files:**
- Modify: `internal/logger/logger.go`

- [ ] **Step 1: Add new functions after CurrentLevel**

Append after `CurrentLevel()`:

```go
// ForModule returns a logger filtered at the module's override level.
// Falls back to defaultLevel if no override is set.
func ForModule(name string) zerolog.Logger {
	if v, ok := moduleLevels.Load(name); ok {
		return global.Level(v.(zerolog.Level))
	}
	return global.Level(defaultLevel)
}

func SetModuleLevel(module, level string) error {
	l := parseLevel(level)
	if !IsValidLevel(level) {
		return fmt.Errorf("invalid level %q", level)
	}
	moduleLevels.Store(module, l)
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
```

Add `"fmt"` to the import block.

- [ ] **Step 2: Commit**

```bash
git add internal/logger/logger.go
git commit -m "feat(logger): add ForModule, SetModuleLevel, ClearModuleLevel, ModuleLevels"
```

---

## Task 3: Fix and extend logger tests

**Files:**
- Modify: `internal/logger/logger_test.go`

- [ ] **Step 1: Fix TestSetLevel — no longer checks zerolog.GlobalLevel**

Replace `TestSetLevel`:

```go
func TestSetLevel(t *testing.T) {
	logger.Init(logger.Config{Level: "info", Format: "json"})
	logger.SetLevel("debug")
	if logger.CurrentLevel() != "debug" {
		t.Errorf("expected debug, got %s", logger.CurrentLevel())
	}
	logger.SetLevel("info") // reset
}
```

- [ ] **Step 2: Run existing tests to confirm they pass**

```bash
go test ./internal/logger/... -v -run TestSetLevel
```

Expected: PASS

- [ ] **Step 3: Write failing tests for new functions**

Append to `logger_test.go`:

```go
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
```

- [ ] **Step 4: Run new tests to confirm they fail**

```bash
go test ./internal/logger/... -v -run "TestForModule|TestModuleLevels|TestSetModuleLevelInvalidLevel"
```

Expected: FAIL (functions not yet implemented — but they are after Task 2, so expected: PASS)

- [ ] **Step 5: Run all logger tests**

```bash
go test ./internal/logger/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/logger/logger_test.go
git commit -m "test(logger): fix TestSetLevel, add ForModule/ModuleLevels tests"
```

---

## Task 4: Update HTTP API — getLogLevel and setLogLevel

**Files:**
- Modify: `internal/api/settings.go`

- [ ] **Step 1: Update getLogLevel to include module overrides**

Replace `getLogLevel` (lines 114-116):

```go
func getLogLevel(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"level":   logger.CurrentLevel(),
		"modules": logger.ModuleLevels(),
	})
}
```

- [ ] **Step 2: Update setLogLevel to handle module field and "inherit"**

Replace `setLogLevel` (lines 118-138):

```go
func setLogLevel(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level  string `json:"level"`
		Module string `json:"module"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Module != "" {
		// per-module override
		if req.Level == "inherit" {
			logger.ClearModuleLevel(req.Module)
		} else {
			if !logger.IsValidLevel(req.Level) {
				writeError(w, http.StatusBadRequest, "level must be debug, info, warn, or error")
				return
			}
			if err := logger.SetModuleLevel(req.Module, req.Level); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		logger.FromContext(r.Context()).Info().
			Str("module", req.Module).Str("level", req.Level).Msg("module log level changed")
		writeJSON(w, http.StatusOK, map[string]any{
			"level":   logger.CurrentLevel(),
			"modules": logger.ModuleLevels(),
		})
		return
	}

	// global level
	if !logger.IsValidLevel(req.Level) {
		writeError(w, http.StatusBadRequest, "level must be debug, info, warn, or error")
		return
	}
	logger.SetLevel(req.Level)
	app.Config.Log.Level = req.Level
	if err := saveConfig(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.FromContext(r.Context()).Info().Str("level", req.Level).Msg("log level changed")
	writeJSON(w, http.StatusOK, map[string]any{
		"level":   req.Level,
		"modules": logger.ModuleLevels(),
	})
}
```

- [ ] **Step 3: Build to verify no compile errors**

```bash
go build ./internal/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/api/settings.go
git commit -m "feat(api): extend log-level endpoint with per-module override support"
```

---

## Task 5: Smoke test via curl

- [ ] **Step 1: Start server**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: GET current levels**

```bash
curl -s -b ~/.spider/data/session.cookie http://localhost:8002/api/v1/log-level | jq .
```

Expected:
```json
{ "level": "info", "modules": {} }
```

- [ ] **Step 3: Set module override**

```bash
curl -s -b ~/.spider/data/session.cookie -X PUT http://localhost:8002/api/v1/log-level \
  -H 'Content-Type: application/json' \
  -d '{"module":"agent","level":"debug"}' | jq .
```

Expected:
```json
{ "level": "info", "modules": { "agent": "debug" } }
```

- [ ] **Step 4: Clear module override**

```bash
curl -s -b ~/.spider/data/session.cookie -X PUT http://localhost:8002/api/v1/log-level \
  -H 'Content-Type: application/json' \
  -d '{"module":"agent","level":"inherit"}' | jq .
```

Expected:
```json
{ "level": "info", "modules": {} }
```

