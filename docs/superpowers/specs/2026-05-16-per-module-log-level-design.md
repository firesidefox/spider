# Per-Module Dynamic Log Level — Design Spec

## Goal

Support per-module log level override at runtime, without restarting the server. Modules not explicitly overridden inherit the global default level.

## Background

Current state: single global level via `zerolog.SetGlobalLevel()`. All modules share one level. No way to turn on `debug` for `agent` only without flooding every other package.

zerolog's `GlobalLevel` is a hard filter — even if a logger's own `.Level()` is `debug`, zerolog discards the event if `GlobalLevel` is `info`. To support per-module levels, `GlobalLevel` must be lowered to `TraceLevel` and filtering delegated to each logger's own level.

---

## Data Model

```go
// in internal/logger/logger.go
var (
    global       zerolog.Logger
    defaultLevel zerolog.Level  // replaces zerolog.GlobalLevel() as source of truth
    moduleLevels sync.Map       // map[string]zerolog.Level — runtime overrides only
    extraOut     io.Writer
)
```

On `Init`: set `zerolog.SetGlobalLevel(zerolog.TraceLevel)` once and never change it again. All filtering goes through `defaultLevel` or per-module level.

---

## API

### logger package

```go
// existing — behavior change: updates defaultLevel, rebuilds global
func SetLevel(level string)

// existing — behavior change: reads defaultLevel, not zerolog.GlobalLevel()
func CurrentLevel() string

// new — returns logger filtered at module's override level, or defaultLevel
func ForModule(name string) zerolog.Logger

// new — set a module-level override (runtime only, resets on restart)
func SetModuleLevel(module, level string) error

// new — remove module override, module falls back to defaultLevel
func ClearModuleLevel(module string)

// new — returns map of module name → level string, only overridden modules
func ModuleLevels() map[string]string
```

`ForModule` returns a new logger each call (no caching), so `SetModuleLevel` takes effect immediately for all subsequent calls.

### HTTP API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/log-level` | Return global level + all active module overrides |
| PUT | `/api/v1/log-level` | Set global or module level |

**GET response:**
```json
{
  "level": "info",
  "modules": {
    "agent": "debug",
    "ssh": "warn"
  }
}
```
`modules` contains only overridden modules (not all registered modules).

**PUT body — set global level:**
```json
{ "level": "debug" }
```

**PUT body — set module override:**
```json
{ "module": "agent", "level": "debug" }
```

**PUT body — clear module override (back to global):**
```json
{ "module": "agent", "level": "inherit" }
```

No DELETE endpoint. Clearing is done via PUT with `"level": "inherit"`.

---

## Persistence

| What | Persisted? |
|------|-----------|
| Global level | Yes — written to config file (existing behavior) |
| Module overrides | No — runtime only, reset on restart |

Frontend settings UI must display: "模块级别设置重启后失效，如需永久生效请修改配置文件。"

---

## Frontend (Settings UI)

- Show current global level with edit control
- Show table of active module overrides (name, current level, clear button)
- Input row: module name + level dropdown → PUT API
- "inherit" option in dropdown clears the override
- Page note: "重启后恢复配置文件设置"

---

## Module Migration (Gradual)

Existing code continues to work unchanged:
```go
// still valid
logger.Global().With().Str("module", "agent").Logger()
```

New pattern, enables per-module level control:
```go
logger.ForModule("agent")
```

No forced migration. Teams adopt `ForModule` when they want module-level control.

---

## Constraints

- No module registration required — `ForModule("agent")` works with any string
- `SetModuleLevel` on an unknown module name is valid (no error, just stored)
- Existing tests must continue to pass
- `SetOutput` (test helper) behavior unchanged
- `ForModule` must be called per log-site (not cached in a struct field). Caching a returned logger means `SetModuleLevel` won't affect that instance — the level is baked in at call time. This is intentional: the tradeoff is immediate effect vs. one extra sync.Map lookup per call. For spider.ai's call frequency this is not a performance concern.
