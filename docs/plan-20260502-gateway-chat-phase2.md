# Gateway Chat Phase 2: Agent Engine + RAG Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Agent Engine (while-loop + tool dispatch + hook chain) and RAG document system, so Phase 3 (API + Frontend) can wire up the chat endpoint.

**Architecture:** Agent loop receives user message, calls LLM with tools, dispatches tool calls through hook chain (risk classification), executes tools, feeds results back. RAG uses sqlite-vec (via `modernc.org/sqlite/vec` blank import) for vector search. Embedding client abstracts OpenAI/Voyage providers.

**Tech Stack:** Go 1.23, modernc.org/sqlite + sqlite/vec, existing SSH pool, Phase 1 LLM client + stores

**Spec Reference:** `docs/spec-20260502-gateway-chat.md` sections 3-6

**Phase 1 Dependencies:**
- `internal/llm` — Client interface, ChatRequest, StreamEvent, ToolCall, ToolDef
- `internal/config` — LLMConfig, EmbeddingConfig with ActiveModel()/ResolveAPIKey()
- `internal/store` — ConversationStore, MessageStore, DocumentStore
- `internal/models` — Conversation, Message, Document, PendingConfirmation
- `internal/ssh` — Client.Execute(ctx, command) → ExecResult{Stdout, Stderr, ExitCode, Duration}

---

### Task 1: Embedding Client — 接口 + OpenAI Provider

**Files:**
- Create: `internal/rag/embedder.go`
- Create: `internal/rag/embedder_test.go`

- [ ] **Step 1: Create embedding client interface**

```go
package rag

import (
	"context"
	"fmt"

	"github.com/spiderai/spider/internal/config"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

func NewEmbedder(cfg *config.EmbeddingModelConfig) (Embedder, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAIEmbedder(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Provider)
	}
}
```

- [ ] **Step 2: Implement OpenAI embedding provider**

```go
type OpenAIEmbedder struct {
	apiKey     string
	model      string
	dimensions int
	http       *http.Client
}

func NewOpenAIEmbedder(cfg *config.EmbeddingModelConfig) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey:     cfg.ResolveAPIKey(),
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}
```

POST to `https://api.openai.com/v1/embeddings` with `{"model": model, "input": texts}`. Parse response `data[].embedding` into `[]float32`.

- [ ] **Step 3: Write test for construction + Dimensions()**

```go
func TestNewOpenAIEmbedder(t *testing.T) {
	cfg := &config.EmbeddingModelConfig{
		ID: "test", Provider: "openai", APIKey: "sk-test",
		Model: "text-embedding-3-small", Dimensions: 1536,
	}
	e := NewOpenAIEmbedder(cfg)
	if e.Dimensions() != 1536 {
		t.Errorf("Dimensions = %d, want 1536", e.Dimensions())
	}
}
```

- [ ] **Step 4: Run tests, commit**

Run: `go test ./internal/rag/ -v`
Commit: `git commit -m "feat(rag): add embedding client interface and OpenAI provider"`

---

### Task 2: RAG Vector Store — sqlite-vec 集成

**Files:**
- Create: `internal/rag/store.go`
- Create: `internal/rag/store_test.go`
- Modify: `go.mod` (add `modernc.org/sqlite/vec`)
- Modify: `internal/db/db.go` (add blank import for vec)

- [ ] **Step 1: Add sqlite-vec dependency**

Run: `go get modernc.org/sqlite/vec`

Add blank import in `internal/db/db.go`:
```go
import _ "modernc.org/sqlite/vec"
```

- [ ] **Step 2: Create RAG store with vector search**

```go
package rag

import (
	"database/sql"
	"fmt"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type Store struct {
	docs     *store.DocumentStore
	db       *sql.DB
	embedder Embedder
}

func NewStore(db *sql.DB, docs *store.DocumentStore, embedder Embedder) *Store {
	return &Store{db: db, docs: docs, embedder: embedder}
}

func (s *Store) Search(ctx context.Context, query string, vendor string, cliType string, topK int) ([]*models.Document, error) {
	// 1. Embed query
	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// 2. Vector search with metadata filter
	// Use sqlite-vec vec_distance_cosine for similarity
	rows, err := s.db.QueryContext(ctx,
		`SELECT d.id, d.vendor, d.cli_type, d.doc_type, d.title, d.content,
		        d.source_file, d.chunk_index, d.created_at
		 FROM documents d
		 WHERE d.vendor = ? AND d.cli_type = ?
		   AND d.embedding IS NOT NULL
		 ORDER BY vec_distance_cosine(d.embedding, ?)
		 LIMIT ?`,
		vendor, cliType, serializeVec(vec), topK,
	)
	// ... scan rows into []*models.Document
}
```

Helper `serializeVec([]float32) []byte` converts float32 slice to little-endian bytes for sqlite-vec.

- [ ] **Step 3: Add Ingest method for document embedding**

```go
func (s *Store) Ingest(ctx context.Context, vendor, cliType, docType, title, content, sourceFile string, chunkIndex int) error {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("embed content: %w", err)
	}
	return s.docs.Save(vendor, cliType, docType, title, content, serializeVec(vec), sourceFile, chunkIndex)
}
```

- [ ] **Step 4: Write tests with mock embedder**

Create a `mockEmbedder` that returns fixed vectors. Test Ingest + Search round-trip.

- [ ] **Step 5: Run tests, commit**

Run: `go test ./internal/rag/ -v`
Commit: `git commit -m "feat(rag): add vector store with sqlite-vec search and ingest"`

---

### Task 3: Agent Tool 接口 + Registry

**Files:**
- Create: `internal/agent/tools.go`
- Create: `internal/agent/tools_test.go`

- [ ] **Step 1: Define Tool interface and Registry**

```go
package agent

import "context"

type RiskLevel string

const (
	RiskSafe      RiskLevel = "safe"
	RiskModerate  RiskLevel = "moderate"
	RiskDangerous RiskLevel = "dangerous"
)

type ToolResult struct {
	Content   string    `json:"content"`
	IsError   bool      `json:"is_error"`
	RiskLevel RiskLevel `json:"risk_level"`
}

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) Definitions() []llm.ToolDef {
	// Convert all registered tools to LLM tool definitions
}
```

- [ ] **Step 2: Write test for registry**

Test Register, Get, Definitions with a mock tool.

- [ ] **Step 3: Run tests, commit**

Run: `go test ./internal/agent/ -v`
Commit: `git commit -m "feat(agent): add Tool interface and ToolRegistry"`

---

### Task 4: Hook Chain — 风险分级

**Files:**
- Create: `internal/agent/hooks.go`
- Create: `internal/agent/hooks_test.go`

- [ ] **Step 1: Define Hook types**

```go
package agent

type HookAction string

const (
	HookAllow          HookAction = "allow"
	HookRequireConfirm HookAction = "require_confirm"
	HookDeny           HookAction = "deny"
)

type HookResult struct {
	Action    HookAction
	RiskLevel RiskLevel
	Reason    string
}

type BeforeToolHook func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult
type AfterToolHook func(toolName string, input map[string]any, result *ToolResult)

type HookChain struct {
	before []BeforeToolHook
	after  []AfterToolHook
}

func NewHookChain() *HookChain {
	return &HookChain{}
}

func (h *HookChain) AddBefore(hook BeforeToolHook) { h.before = append(h.before, hook) }
func (h *HookChain) AddAfter(hook AfterToolHook)   { h.after = append(h.after, hook) }

func (h *HookChain) RunBefore(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
	for _, hook := range h.before {
		if result := hook(toolName, input, riskLevel); result.Action != HookAllow {
			return result
		}
	}
	return &HookResult{Action: HookAllow}
}

func (h *HookChain) RunAfter(toolName string, input map[string]any, result *ToolResult) {
	for _, hook := range h.after {
		hook(toolName, input, result)
	}
}
```

- [ ] **Step 2: Implement default risk classification hook**

```go
func DefaultRiskHook() BeforeToolHook {
	return func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
		switch riskLevel {
		case RiskSafe:
			return &HookResult{Action: HookAllow, RiskLevel: RiskSafe}
		case RiskModerate:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: RiskModerate}
		case RiskDangerous:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: RiskDangerous}
		default:
			return &HookResult{Action: HookRequireConfirm, RiskLevel: RiskModerate}
		}
	}
}
```

- [ ] **Step 3: Write tests**

Test HookChain with safe/moderate/dangerous risk levels. Verify allow vs require_confirm.

- [ ] **Step 4: Run tests, commit**

Run: `go test ./internal/agent/ -v`
Commit: `git commit -m "feat(agent): add hook chain with risk classification"`

---

### Task 5: Agent Tools — get_device_info + execute_cli

**Files:**
- Create: `internal/agent/tools_device.go`
- Create: `internal/agent/tools_cli.go`

- [ ] **Step 1: Implement get_device_info tool**

```go
type GetDeviceInfoTool struct {
	hosts *store.HostStore
}

func (t *GetDeviceInfoTool) Name() string { return "get_device_info" }

func (t *GetDeviceInfoTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// Accept host_id or host_name, query HostStore, return JSON with device info
	// RiskLevel: safe
}
```

- [ ] **Step 2: Implement execute_cli tool**

```go
type ExecuteCLITool struct {
	hosts   *store.HostStore
	sshPool *ssh.Pool
	logs    *store.LogStore
}

func (t *ExecuteCLITool) Name() string { return "execute_cli" }

func (t *ExecuteCLITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// Extract host_id, command, risk_level from input
	// Get host from store, get SSH client from pool
	// Execute command, log to execution_logs
	// Return stdout/stderr/exit_code as JSON
}
```

Input schema: `{"host_id": string, "command": string, "risk_level": "safe"|"moderate"|"dangerous"}`

- [ ] **Step 3: Write tests with mock dependencies**

- [ ] **Step 4: Run tests, commit**

Commit: `git commit -m "feat(agent): add get_device_info and execute_cli tools"`

---

### Task 6: Agent Tools — search_docs + call_rest_api

**Files:**
- Create: `internal/agent/tools_docs.go`
- Create: `internal/agent/tools_api.go`

- [ ] **Step 1: Implement search_docs tool**

```go
type SearchDocsTool struct {
	ragStore *rag.Store
}

func (t *SearchDocsTool) Name() string { return "search_docs" }

func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// Extract query, vendor, cli_type from input
	// Call ragStore.Search(ctx, query, vendor, cliType, 5)
	// Return formatted document snippets
	// RiskLevel: safe
}
```

- [ ] **Step 2: Implement call_rest_api tool**

```go
type CallRESTAPITool struct {
	http *http.Client
}

func (t *CallRESTAPITool) Name() string { return "call_rest_api" }

func (t *CallRESTAPITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// Extract url, method, headers, body from input
	// Make HTTP request to gateway REST API
	// Return response status + body
}
```

- [ ] **Step 3: Write tests, commit**

Commit: `git commit -m "feat(agent): add search_docs and call_rest_api tools"`

---

### Task 7: Agent Tools — batch_execute + verify

**Files:**
- Create: `internal/agent/tools_batch.go`
- Create: `internal/agent/tools_verify.go`

- [ ] **Step 1: Implement batch_execute tool**

```go
type BatchExecuteTool struct {
	hosts   *store.HostStore
	sshPool *ssh.Pool
	logs    *store.LogStore
}

func (t *BatchExecuteTool) Name() string { return "batch_execute" }

func (t *BatchExecuteTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// Extract host_ids or tag, command from input
	// Resolve hosts list
	// Execute in parallel with goroutines + WaitGroup
	// Collect results per host: {host_id, stdout, stderr, exit_code, duration}
	// RiskLevel: dangerous (always)
}
```

- [ ] **Step 2: Implement verify tool with retry polling**

```go
type VerifyTool struct {
	hosts   *store.HostStore
	sshPool *ssh.Pool
}

func (t *VerifyTool) Name() string { return "verify" }

func (t *VerifyTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	// Extract checks[], timeout, interval from input
	// Default: timeout=60s, interval=5s, max_retries=12
	// For each check: port_open, file_exists, http_status, cli_output, custom_cmd
	// Poll loop: run checks → all pass? done : sleep interval → retry
	// Timeout → return failure with details
}

type Check struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Expect string `json:"expect"`
	HostID string `json:"host_id"`
}
```

- [ ] **Step 3: Write tests for verify polling logic**

Test with mock checks that fail N times then succeed. Test timeout behavior.

- [ ] **Step 4: Run tests, commit**

Commit: `git commit -m "feat(agent): add batch_execute and verify tools with retry polling"`

---

### Task 8: Agent Loop — 核心 while-loop

**Files:**
- Create: `internal/agent/agent.go`
- Create: `internal/agent/agent_test.go`

- [ ] **Step 1: Define Agent struct and event types**

```go
package agent

type EventType string

const (
	EventTextDelta      EventType = "text_delta"
	EventToolStart      EventType = "tool_start"
	EventToolResult     EventType = "tool_result"
	EventConfirmRequired EventType = "confirm_required"
	EventVerifyProgress EventType = "verify_progress"
	EventBatchProgress  EventType = "batch_progress"
	EventDeviceUpdate   EventType = "device_update"
	EventError          EventType = "error"
	EventDone           EventType = "done"
)

type Event struct {
	Type    EventType      `json:"type"`
	Content map[string]any `json:"content,omitempty"`
}

type Agent struct {
	llmClient    llm.Client
	registry     *ToolRegistry
	hooks        *HookChain
	convStore    *store.ConversationStore
	msgStore     *store.MessageStore
	systemPrompt string
	maxTurns     int
}
```

- [ ] **Step 2: Implement the agent loop**

```go
func (a *Agent) Run(ctx context.Context, conversationID string, userMessage string) (<-chan Event, error) {
	events := make(chan Event, 64)

	go func() {
		defer close(events)

		// 1. Save user message
		a.msgStore.Save(conversationID, "user", userMessage)

		// 2. Load message history
		history := a.loadHistory(conversationID)

		// 3. Agent loop
		for turn := 0; turn < a.maxTurns; turn++ {
			// Call LLM with streaming
			stream, err := a.llmClient.ChatStream(ctx, &llm.ChatRequest{
				System:    a.systemPrompt,
				Messages:  history,
				Tools:     a.registry.Definitions(),
				MaxTokens: 4096,
			})

			// Process stream events
			var assistantText string
			var toolCalls []llm.ToolCall

			for event := range stream {
				switch event.Type {
				case "text_delta":
					assistantText += event.Text
					events <- Event{Type: EventTextDelta, Content: map[string]any{"text": event.Text}}
				case "tool_start":
					toolCalls = append(toolCalls, *event.ToolCall)
					events <- Event{Type: EventToolStart, Content: map[string]any{...}}
				// ... accumulate tool input deltas
				}
			}

			// No tool calls → done
			if len(toolCalls) == 0 {
				a.msgStore.Save(conversationID, "assistant", assistantText)
				events <- Event{Type: EventDone}
				return
			}

			// Execute tool calls through hook chain
			for _, tc := range toolCalls {
				tool, ok := a.registry.Get(tc.Name)
				if !ok {
					// tool not found error
					continue
				}

				// Run before hooks
				hookResult := a.hooks.RunBefore(tc.Name, tc.Input, extractRiskLevel(tc.Input))

				if hookResult.Action == HookRequireConfirm {
					// Send confirm_required event, wait for confirmation
					events <- Event{Type: EventConfirmRequired, Content: map[string]any{
						"request_id": uuid.New().String(),
						"tool": tc.Name,
						"input": tc.Input,
						"risk_level": hookResult.RiskLevel,
					}}
					// Block on confirmation channel (provided by API layer)
					// ...
				}

				// Execute tool
				result, err := tool.Execute(ctx, tc.Input)

				// Run after hooks
				a.hooks.RunAfter(tc.Name, tc.Input, result)

				// Add tool result to history
				history = append(history, llm.Message{Role: "assistant", Content: toolCallJSON})
				history = append(history, llm.Message{Role: "user", Content: toolResultJSON})
			}
			// Loop continues — next LLM call with updated history
		}
	}()

	return events, nil
}
```

- [ ] **Step 3: Implement confirmation channel mechanism**

```go
type ConfirmationWaiter struct {
	pending map[string]chan bool // request_id → channel
	mu      sync.Mutex
}

func (w *ConfirmationWaiter) Wait(requestID string, timeout time.Duration) (bool, error) {
	ch := make(chan bool, 1)
	w.mu.Lock()
	w.pending[requestID] = ch
	w.mu.Unlock()

	select {
	case approved := <-ch:
		return approved, nil
	case <-time.After(timeout):
		return false, fmt.Errorf("confirmation timeout")
	}
}

func (w *ConfirmationWaiter) Resolve(requestID string, approved bool) {
	w.mu.Lock()
	ch, ok := w.pending[requestID]
	delete(w.pending, requestID)
	w.mu.Unlock()
	if ok {
		ch <- approved
	}
}
```

- [ ] **Step 4: Implement system prompt builder**

```go
func BuildSystemPrompt(hosts *store.HostStore) string {
	// Count hosts by vendor/cli_type
	// Build prompt from spec section 6.2 template
}
```

- [ ] **Step 5: Write test with mock LLM client**

Create mock LLM client that returns predefined responses. Test:
- Simple text response (no tool calls) → EventTextDelta + EventDone
- Tool call response → EventToolStart + EventToolResult + next LLM call

- [ ] **Step 6: Run tests, commit**

Run: `go test ./internal/agent/ -v`
Commit: `git commit -m "feat(agent): implement agent loop with tool dispatch and confirmation flow"`

---

### Task 9: 全量构建验证

- [ ] **Step 1: Run full build**

Run: `go build ./...`

- [ ] **Step 2: Run all tests**

Run: `go test ./... -v`

- [ ] **Step 3: Verify new packages compile cleanly**

Check `internal/agent/` and `internal/rag/` have no import cycles or missing dependencies.

Phase 2 complete. Agent Engine + RAG ready for Phase 3 (API + Frontend).
