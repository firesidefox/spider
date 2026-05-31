# Server Architecture Optimization Design

## Background

The server is still a Go monolith, which is the right deployment shape for the current product. The architecture issue is not service splitting. The issue is that several internal boundaries have grown too broad:

- `cmd/spider/main.go` owns CLI setup, config loading, logger setup, database setup, store construction, runtime wiring, scheduler lifecycle, HTTP routing, static assets, and shutdown.
- `mcp.App` has become a service locator for stores, agent factory, permission services, runtime maps, SSE clients, and cached RAG state.
- `internal/api/handler.go` centralizes most route registration and permission wrapping in one large file.
- `internal/db/schema.go` uses append-only idempotent migration code. It already contains repeated Prometheus table creation blocks.
- Chat, agent execution, mid-turn injection, cancellation, and SSE streaming are the highest-risk runtime path and have had recent race and lifecycle fixes.

The optimization should keep the monolith and external API stable while improving internal ownership boundaries.

## Goals

- Reduce race-prone shared state in `mcp.App`.
- Make chat/agent/SSE lifecycle easier to reason about and test.
- Thin the server entrypoint without changing deployment.
- Split API route registration by domain while preserving URLs and response shapes.
- Make schema migration safer for long-term evolution.

## Non-Goals

- No microservice split.
- No HTTP framework replacement.
- No endpoint redesign.
- No database replacement.
- No behavior changes in chat, SSE, scheduler, or knowledge features unless required to preserve existing behavior under the new boundaries.

## Recommended Order

Use four phases:

1. Chat Runtime boundary.
2. Server composition root.
3. API route modularization.
4. Migration registry.

This order stabilizes the most complex runtime path before cleaning up broader maintainability issues.

## Phase 1: Chat Runtime Boundary

### Intent

Extract chat, injection, cancellation, queued input, and SSE runtime state out of `mcp.App` into a dedicated package. This is the first phase because this path is the most concurrency-sensitive part of the server.

### Proposed Package

Create `internal/chatruntime`.

The package owns:

- conversation waiters
- conversation cancel functions
- per-conversation injection channels
- queued mid-turn user messages
- per-conversation SSE clients
- per-conversation in-flight SSE buffers
- global SSE clients

`mcp.App` should keep only:

```go
ChatRuntime *chatruntime.Runtime
```

### API Shape

The runtime should expose focused methods matching existing behavior:

```go
type Runtime struct {
    // private mutexes and maps
}

func New() *Runtime

func (r *Runtime) StoreChatWaiter(convID string, waiter *agent.ConfirmationWaiter)
func (r *Runtime) GetChatWaiter(convID string) *agent.ConfirmationWaiter
func (r *Runtime) RemoveChatWaiter(convID string)

func (r *Runtime) StoreConvCancel(convID string, cancel context.CancelFunc)
func (r *Runtime) CancelConv(convID string) bool
func (r *Runtime) RemoveConvCancel(convID string)

func (r *Runtime) TryClaimConv(convID string) (chan string, bool)
func (r *Runtime) TryInject(convID string, msg string) (queued bool, full bool)
func (r *Runtime) ReleaseConv(convID string)
func (r *Runtime) GetQueuedMsgs(convID string) []string
func (r *Runtime) ClearQueuedMsgs(convID string)

func (r *Runtime) RegisterSSEClientAndDrain(convID string, ch chan []byte) [][]byte
func (r *Runtime) UnregisterSSEClient(convID string, ch chan []byte)
func (r *Runtime) BroadcastSSE(convID string, data []byte)
func (r *Runtime) ClearSSEBuffer(convID string)

func (r *Runtime) RegisterGlobalSSEClient(ch chan []byte)
func (r *Runtime) UnregisterGlobalSSEClient(ch chan []byte)
func (r *Runtime) BroadcastGlobal(data []byte)
```

If importing `agent.ConfirmationWaiter` into `internal/chatruntime` creates an undesirable dependency, use a small interface or keep waiter management in a separate `ChatSessions` type. The simplest first pass can use the direct type because this is still an internal package.

### Migration Approach

Keep method behavior identical and move implementation from `mcp.App` to `chatruntime.Runtime`.

Handler changes should be mechanical:

- `app.TryClaimConv(id)` becomes `app.ChatRuntime.TryClaimConv(id)`.
- `app.TryInject(id, content)` becomes `app.ChatRuntime.TryInject(id, content)`.
- `app.RegisterSSEClientAndDrain(id, ch)` becomes `app.ChatRuntime.RegisterSSEClientAndDrain(id, ch)`.
- SSE broadcaster methods on `mcp.App` can either delegate to `ChatRuntime` or be moved if the agent broadcaster interface allows it cleanly.

### Tests

Add focused tests for:

- one caller can claim a conversation; concurrent claim fails
- inject succeeds only when a conversation is running
- full inject channel returns `(false, true)`
- release removes inject channel and queued messages
- cancel calls the stored cancel function and removes it
- `RegisterSSEClientAndDrain` atomically registers and returns buffered events
- broadcast stores in-flight buffer and sends to current clients
- unregister removes only the target client
- global broadcast reaches registered global clients

Existing `internal/api/chat_send_test.go` and SSE-related tests must continue to pass.

### Success Criteria

- `mcp.App` no longer directly owns chat/SSE mutexes or maps.
- Existing chat/SSE behavior is preserved.
- Runtime methods have isolated concurrency tests.
- No route, payload, or frontend contract changes.

## Phase 2: Server Composition Root

### Intent

Move server construction out of `cmd/spider/main.go` so the entrypoint is mostly CLI parsing and process startup.

### Proposed Package

Create `internal/server`.

Suggested structure:

```go
type Options struct {
    ConfigFile string
    Addr       string
    DataDir    string
    Debug      bool
    WebFS      fs.FS
    SkillsFS   fs.FS
}

func Run(ctx context.Context, opts Options) error
```

Implementation can be split internally:

- `loadConfig`
- `initLogger`
- `openDatabase`
- `buildStores`
- `buildApp`
- `buildAgentFactory`
- `startScheduler`
- `buildMux`
- `serveHTTP`

### Shutdown Model

Use one root shutdown context for:

- scheduler
- task executor
- chat agent runs
- SSE streams
- HTTP shutdown coordination

The current shutdown behavior should be preserved:

- signal cancels shutdown context
- SSE streams close before HTTP server shutdown completes
- scheduler and executor stop after cancellation
- database closes after server exit

### Success Criteria

- `cmd/spider/main.go` keeps Cobra commands, flag parsing, version output, reset-password command, and a small call into `server.Run`.
- Store and service construction are centralized outside `main.go`.
- Shutdown ordering is explicit and tested where practical.
- No deployment or CLI behavior changes.

## Phase 3: API Route Modularization

### Intent

Split `internal/api/handler.go` into domain route registration functions while staying on `http.ServeMux`.

### Proposed Shape

Keep:

```go
func NewRouter(app *mcp.App) http.Handler
```

Inside it:

```go
func NewRouter(app *mcp.App) http.Handler {
    mux := http.NewServeMux()
    deps := routeDeps{
        app: app,
        adminOnly: authmw.RequireRole(models.RoleAdmin),
        operatorOrAbove: authmw.RequireRole(models.RoleAdmin, models.RoleOperator),
    }

    registerHostRoutes(mux, deps)
    registerChatRoutes(mux, deps)
    registerKnowledgeRoutes(mux, deps)
    registerTaskRoutes(mux, deps)
    registerAdminRoutes(mux, deps)
    registerSettingsRoutes(mux, deps)
    registerStreamRoutes(mux, deps)

    return authmw.AuthMiddleware(app.JWTManager, app.TokenStore)(loggingMiddleware(mux))
}
```

Use small helpers only when they remove real duplication:

- method dispatch
- role wrapping
- path ID extraction

Avoid introducing a custom router abstraction.

### Success Criteria

- `handler.go` becomes a small assembly file.
- Route groups live near the matching handlers.
- URLs, methods, role requirements, and response payloads stay unchanged.
- API tests pass without frontend changes.

## Phase 4: Migration Registry

### Intent

Make schema evolution explicit and testable while preserving the existing SQLite database and data.

### Proposed Shape

Introduce a migration registry:

```go
type Migration struct {
    ID string
    Up func(*sql.DB) error
}
```

Create a table:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    id TEXT PRIMARY KEY,
    applied_at DATETIME NOT NULL
);
```

Migration IDs should be stable and ordered, for example:

- `20260418_0001_initial`
- `20260502_0001_gateway_chat`
- `20260520_0001_knowledge_base`
- `20260524_0001_prometheus`

Because existing installs may already have tables and columns without migration records, early migrations must remain idempotent. The first registry pass should not assume a pristine database.

### Migration Strategy

1. Add `schema_migrations`.
2. Keep existing idempotent creation and alter logic but split it into named steps.
3. Record each step after successful execution.
4. Remove duplicated Prometheus table creation.
5. Add tests for fresh DB, old DB shape, and repeated `Migrate`.

### Success Criteria

- Fresh database initializes correctly.
- Existing database migrates without data loss.
- Re-running migration is safe.
- Duplicate schema blocks are removed.
- Future migrations have an obvious place and naming pattern.

## Risk Management

- Phase 1 is the only phase touching high-risk runtime behavior. It must be backed by concurrency tests before handler rewiring is considered complete.
- Phase 2 should avoid changing construction semantics. It is primarily code movement.
- Phase 3 should not change route semantics. Route tests should be run before and after.
- Phase 4 should be done after the runtime and routing boundaries are stable, because migration mistakes are harder to recover from than package movement mistakes.

## Verification Plan

Run after each phase:

```sh
go test ./internal/api ./internal/agent ./internal/mcp ./internal/scheduler ./internal/store ./internal/db
```

Run before considering a phase complete:

```sh
go test ./...
```

For Phase 1 specifically, also run tests with race detection when feasible:

```sh
go test -race ./internal/chatruntime ./internal/api
```

## Open Decisions

- Package name for Phase 1: `internal/chatruntime` is the clearest option.
- Package name for Phase 2: `internal/server` is preferred because it describes process wiring better than `internal/app`.
- Whether `mcp.App` should continue implementing the agent SSE broadcaster interface by delegating to `ChatRuntime`, or whether `ChatRuntime` should directly implement that interface. Prefer delegation first to keep the diff smaller.

## Approval

This design is approved when the team agrees to:

- start with Phase 1
- keep the monolith
- preserve external API behavior
- avoid framework or database replacement
- defer migration registry work until runtime and route boundaries are cleaner
