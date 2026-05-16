# Tool Result Budgeting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cap individual and aggregate tool result sizes before they enter LLM history, persisting oversized content to disk and replacing it with a preview.

**Architecture:** Two layers — Layer 1 truncates single results >50K chars immediately after `tool.Execute()` in `agent.go`; Layer 2 truncates aggregate results >200K chars per turn inside `toLLMMessages()` in `compactor.go`. A `ContentReplacementState` struct freezes decisions so the same `tool_use_id` always maps to the same preview across history rebuilds.

**Tech Stack:** Go stdlib (`os`, `sync`, `path/filepath`), existing `config.AgentConfig`, existing `llm.ContentBlock`

---

### Task 1: Add budget fields to `config.AgentConfig`

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add two fields to `AgentConfig`**

In `internal/config/config.go`, add to the `AgentConfig` struct after the `Compaction` field:

```go
// AgentConfig 是 Agent 执行权限相关配置。
type AgentConfig struct {
	PermissionMode  string           `yaml:"permission_mode"`
	ApprovalTimeout int              `yaml:"approval_timeout"`
	MaxTurns        int              `yaml:"max_turns"`
	Rules           []RuleConfig     `yaml:"rules,omitempty" json:"rules,omitempty"`
	Compaction      CompactionConfig `yaml:"compaction"`
	PerToolResultMaxChars        int `yaml:"per_tool_result_max_chars"`        // 0 = disabled
	PerMessageToolResultMaxChars int `yaml:"per_message_tool_result_max_chars"` // 0 = disabled
}
```

- [ ] **Step 2: Set defaults in `DefaultConfig()`**

In `DefaultConfig()`, update the `Agent` block:

```go
Agent: AgentConfig{
    PermissionMode:  "ask",
    ApprovalTimeout: 300,
    MaxTurns:        10000,
    Compaction: CompactionConfig{
        ThresholdTokens:  0,
        RecentTurns:      20,
        MaxSummaryTokens: 4000,
    },
    PerToolResultMaxChars:        50_000,
    PerMessageToolResultMaxChars: 200_000,
},
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/config/... -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add PerToolResultMaxChars and PerMessageToolResultMaxChars"
```

---

### Task 2: Create `tool_result_budget.go` with core types and helpers

**Files:**
- Create: `internal/agent/tool_result_budget.go`
- Create: `internal/agent/tool_result_budget_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/agent/tool_result_budget_test.go`:

```go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePreview_ShortContent(t *testing.T) {
	content := "hello world"
	preview := generatePreview(content, "/tmp/fake.txt")
	if strings.Contains(preview, "too large") {
		t.Errorf("short content should not get truncation header, got: %s", preview)
	}
}

func TestGeneratePreview_LongContent(t *testing.T) {
	content := strings.Repeat("x", 3000)
	preview := generatePreview(content, "/tmp/fake.txt")
	if !strings.Contains(preview, "too large") {
		t.Errorf("expected truncation header, got: %s", preview[:100])
	}
	if !strings.Contains(preview, "/tmp/fake.txt") {
		t.Errorf("expected file path in preview")
	}
	// preview body should be capped at 2000 chars
	lines := strings.SplitN(preview, "Preview (first 2000 chars):\n", 2)
	if len(lines) != 2 {
		t.Fatalf("expected preview section, got: %s", preview[:200])
	}
	if len(lines[1]) > 2100 {
		t.Errorf("preview body too long: %d", len(lines[1]))
	}
}

func TestPersistToolResult_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path, err := persistToolResult(dir, "conv1", "tool-abc", "hello content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(data) != "hello content" {
		t.Errorf("wrong content: %s", data)
	}
	if !strings.HasSuffix(path, filepath.Join("conv1", "tool-abc.txt")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestContentReplacementState_FreezeDecision(t *testing.T) {
	s := newContentReplacementState()
	s.setReplacement("id1", "preview1")
	// second set must not overwrite
	s.setReplacement("id1", "preview2")
	if got := s.getReplacement("id1"); got != "preview1" {
		t.Errorf("expected frozen preview1, got %s", got)
	}
}

func TestContentReplacementState_SeenNotReplaced(t *testing.T) {
	s := newContentReplacementState()
	s.markSeen("id2")
	if s.getReplacement("id2") != "" {
		t.Errorf("seen-only id should have no replacement")
	}
	if !s.isSeen("id2") {
		t.Errorf("id2 should be seen")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/agent/ -run "TestGeneratePreview|TestPersistToolResult|TestContentReplacementState" -v
```

Expected: FAIL — functions not defined

- [ ] **Step 3: Implement `tool_result_budget.go` (part 1 of 2)**

Create `internal/agent/tool_result_budget.go`:

```go
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const previewMaxChars = 2000

// persistToolResult writes content to {dataDir}/tool-results/{convID}/{toolUseID}.txt.
// Returns the absolute file path.
func persistToolResult(dataDir, convID, toolUseID, content string) (string, error) {
	dir := filepath.Join(dataDir, "tool-results", convID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir tool-results: %w", err)
	}
	path := filepath.Join(dir, toolUseID+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write tool result: %w", err)
	}
	return path, nil
}

// generatePreview builds the replacement string shown to the LLM.
// content is the original (full) result; filePath is where it was persisted.
func generatePreview(content, filePath string) string {
	body := content
	if len(body) > previewMaxChars {
		// cut at last newline within limit
		cut := body[:previewMaxChars]
		if idx := lastNewline(cut); idx > 0 {
			cut = cut[:idx]
		}
		body = cut + "\n..."
	}
	return fmt.Sprintf(
		"[Output too large: %d chars. Full output saved to: %s]\n\nPreview (first 2000 chars):\n%s",
		len(content), filePath, body,
	)
}

func lastNewline(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 4: Implement `ContentReplacementState` (part 2 of 2)**

Append to `internal/agent/tool_result_budget.go`:

```go
// ContentReplacementState freezes tool result replacement decisions so that
// the same tool_use_id always maps to the same preview across history rebuilds.
type ContentReplacementState struct {
	mu           sync.Mutex
	replacements map[string]string // toolUseID → preview (frozen once set)
	seen         map[string]bool   // toolUseID → processed but not replaced
}

func newContentReplacementState() *ContentReplacementState {
	return &ContentReplacementState{
		replacements: make(map[string]string),
		seen:         make(map[string]bool),
	}
}

// setReplacement stores a preview for toolUseID. No-op if already set.
func (s *ContentReplacementState) setReplacement(toolUseID, preview string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.replacements[toolUseID]; !exists {
		s.replacements[toolUseID] = preview
	}
}

// getReplacement returns the frozen preview, or "" if none.
func (s *ContentReplacementState) getReplacement(toolUseID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replacements[toolUseID]
}

// markSeen records that toolUseID was processed but not replaced.
func (s *ContentReplacementState) markSeen(toolUseID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[toolUseID] = true
}

// isSeen returns true if toolUseID was processed (replaced or seen-only).
func (s *ContentReplacementState) isSeen(toolUseID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inReplacements := s.replacements[toolUseID]
	return inReplacements || s.seen[toolUseID]
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/agent/ -run "TestGeneratePreview|TestPersistToolResult|TestContentReplacementState" -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tool_result_budget.go internal/agent/tool_result_budget_test.go
git commit -m "feat(agent): add tool result budget helpers and ContentReplacementState"
```

---

### Task 3: Layer 1 — per-tool limit in `agent.go`

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/agent_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/agent_test.go`:

```go
func TestAgent_PerToolResultLimit_Truncates(t *testing.T) {
	dataDir := t.TempDir()
	bigContent := strings.Repeat("a", 60_000)

	tool := &mockTool{name: "big_tool", result: bigContent}
	reg := NewToolRegistry()
	reg.Register(tool)

	toolCallResp := []llm.StreamEvent{
		{Type: llm.EventToolUse, ToolUseID: "tu1", ToolName: "big_tool", ToolInput: map[string]any{}},
		{Type: llm.EventDone, Usage: &llm.Usage{}},
	}
	doneResp := []llm.StreamEvent{
		{Type: llm.EventText, Text: "done"},
		{Type: llm.EventDone, Usage: &llm.Usage{}},
	}
	client := &mockLLMClient{responses: [][]llm.StreamEvent{toolCallResp, doneResp}}

	a := NewAgent(AgentConfig{
		LLMClient:   client,
		Registry:    reg,
		MsgStore:    &mockMsgStore{},
		DataDir:     dataDir,
		PerToolResultMaxChars: 50_000,
		ReplacementState: newContentReplacementState(),
	})

	events, err := a.Run(context.Background(), "conv1", "go", nil)
	if err != nil {
		t.Fatal(err)
	}
	for e := range events {
		if e.Type == EventToolResult {
			result, _ := e.Content["result"].(string)
			if len(result) >= 60_000 {
				t.Errorf("tool result not truncated: len=%d", len(result))
			}
			if !strings.Contains(result, "too large") {
				t.Errorf("expected truncation notice in result, got: %s", result[:100])
			}
		}
	}
	// verify file was written
	entries, _ := os.ReadDir(filepath.Join(dataDir, "tool-results", "conv1"))
	if len(entries) != 1 {
		t.Errorf("expected 1 persisted file, got %d", len(entries))
	}
}
```

- [ ] **Step 2: Add `mockTool` helper to `agent_test.go`** (if not already present)

```go
type mockTool struct {
	name   string
	result string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return "mock" }
func (m *mockTool) Schema() any         { return map[string]any{} }
func (m *mockTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	return &ToolResult{Content: m.result}, nil
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/agent/ -run TestAgent_PerToolResultLimit_Truncates -v
```

Expected: FAIL — `DataDir`, `PerToolResultMaxChars`, `ReplacementState` not in `AgentConfig`

- [ ] **Step 4: Add fields to `AgentConfig` and `Agent` structs**

In `internal/agent/agent.go`, update `AgentConfig` and `Agent`:

```go
type AgentConfig struct {
	LLMClient    llm.Client
	Registry     *ToolRegistry
	Hooks        *HookChain
	MsgStore     MessageStorer
	TodoStore    *store.TodoStore
	SystemPrompt string
	MaxTurns     int
	Compactor    *Compactor
	SkillManager *SkillManager
	DataDir                      string
	PerToolResultMaxChars        int
	PerMessageToolResultMaxChars int
	ReplacementState             *ContentReplacementState
}

type Agent struct {
	llmClient                    llm.Client
	registry                     *ToolRegistry
	hooks                        *HookChain
	msgStore                     MessageStorer
	todoStore                    *store.TodoStore
	systemPrompt                 string
	maxTurns                     int
	compactor                    *Compactor
	skillManager                 *SkillManager
	lastSkillHash                string
	dataDir                      string
	perToolResultMaxChars        int
	perMessageToolResultMaxChars int
	replacementState             *ContentReplacementState
}
```

- [ ] **Step 5: Update `NewAgent` to wire new fields**

In `NewAgent()`, add after `skillManager: cfg.SkillManager,`:

```go
dataDir:                      cfg.DataDir,
perToolResultMaxChars:        cfg.PerToolResultMaxChars,
perMessageToolResultMaxChars: cfg.PerMessageToolResultMaxChars,
replacementState:             cfg.ReplacementState,
```

- [ ] **Step 6: Apply Layer 1 after `tool.Execute()`**

In `agent.go`, after the block ending with `a.hooks.RunAfter(tc.Name, tc.Input, result)` (around line 384), add:

```go
// Layer 1: per-tool result size limit
if a.perToolResultMaxChars > 0 && len(result.Content) > a.perToolResultMaxChars && a.replacementState != nil {
    filePath, err := persistToolResult(a.dataDir, conversationID, tc.ID, result.Content)
    if err != nil {
        log.Warn().Err(err).Str("tool", tc.Name).Msg("failed to persist large tool result; passing through")
    } else {
        preview := generatePreview(result.Content, filePath)
        a.replacementState.setReplacement(tc.ID, preview)
        result = &ToolResult{Content: preview, IsError: result.IsError, RiskLevel: result.RiskLevel}
    }
}
```

- [ ] **Step 7: Run test**

```bash
go test ./internal/agent/ -run TestAgent_PerToolResultLimit_Truncates -v
```

Expected: PASS

- [ ] **Step 8: Run full agent test suite**

```bash
go test ./internal/agent/ -v -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add internal/agent/agent.go internal/agent/agent_test.go
git commit -m "feat(agent): Layer 1 per-tool result size limit"
```

---

### Task 4: Layer 2 — per-message aggregate limit in `compactor.go`

**Files:**
- Modify: `internal/agent/compactor.go`
- Modify: `internal/agent/tool_result_budget_test.go`

- [ ] **Step 1: Write failing test for `enforcePerMessageBudget`**

Add to `internal/agent/tool_result_budget_test.go`:

```go
func TestEnforcePerMessageBudget_UnderLimit(t *testing.T) {
	state := newContentReplacementState()
	blocks := []llm.ContentBlock{
		{Type: "tool_result", ToolUseID: "id1", Content: strings.Repeat("a", 1000)},
		{Type: "tool_result", ToolUseID: "id2", Content: strings.Repeat("b", 1000)},
	}
	result := enforcePerMessageBudget(blocks, 10_000, t.TempDir(), "conv1", state)
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	if result[0].Content != blocks[0].Content {
		t.Errorf("content should be unchanged under limit")
	}
}

func TestEnforcePerMessageBudget_OverLimit_ReplacesLargest(t *testing.T) {
	state := newContentReplacementState()
	dataDir := t.TempDir()
	small := strings.Repeat("s", 100)
	large := strings.Repeat("L", 9000)
	blocks := []llm.ContentBlock{
		{Type: "tool_result", ToolUseID: "small1", Content: small},
		{Type: "tool_result", ToolUseID: "large1", Content: large},
	}
	result := enforcePerMessageBudget(blocks, 5_000, dataDir, "conv1", state)
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	// large1 should be replaced
	var largeBlock llm.ContentBlock
	for _, b := range result {
		if b.ToolUseID == "large1" {
			largeBlock = b
		}
	}
	if !strings.Contains(largeBlock.Content, "too large") {
		t.Errorf("large block should be replaced with preview, got: %s", largeBlock.Content[:80])
	}
	// small1 should be unchanged
	for _, b := range result {
		if b.ToolUseID == "small1" && b.Content != small {
			t.Errorf("small block should be unchanged")
		}
	}
}

func TestEnforcePerMessageBudget_StableAcrossRebuild(t *testing.T) {
	state := newContentReplacementState()
	dataDir := t.TempDir()
	large := strings.Repeat("L", 9000)
	blocks := []llm.ContentBlock{
		{Type: "tool_result", ToolUseID: "id1", Content: large},
	}
	result1 := enforcePerMessageBudget(blocks, 5_000, dataDir, "conv1", state)
	result2 := enforcePerMessageBudget(blocks, 5_000, dataDir, "conv1", state)
	if result1[0].Content != result2[0].Content {
		t.Errorf("preview must be identical across rebuilds")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/agent/ -run "TestEnforcePerMessageBudget" -v
```

Expected: FAIL — `enforcePerMessageBudget` not defined

- [ ] **Step 3: Implement `enforcePerMessageBudget` in `tool_result_budget.go`**

Append to `internal/agent/tool_result_budget.go`:

```go
// enforcePerMessageBudget applies the per-message aggregate limit to a slice of
// tool_result ContentBlocks. Blocks that were already decided (in state) are
// reused as-is. Fresh blocks that push the total over maxChars are persisted and
// replaced with previews, largest first.
func enforcePerMessageBudget(
	blocks []llm.ContentBlock,
	maxChars int,
	dataDir, convID string,
	state *ContentReplacementState,
) []llm.ContentBlock {
	if maxChars <= 0 {
		return blocks
	}

	// Apply already-frozen replacements and measure total of unfrozen blocks.
	out := make([]llm.ContentBlock, len(blocks))
	copy(out, blocks)

	type freshEntry struct {
		idx  int
		size int
	}
	var fresh []freshEntry
	total := 0

	for i, b := range out {
		if b.Type != "tool_result" {
			continue
		}
		if prev := state.getReplacement(b.ToolUseID); prev != "" {
			out[i].Content = prev
			continue
		}
		if state.isSeen(b.ToolUseID) {
			total += len(b.Content)
			continue
		}
		fresh = append(fresh, freshEntry{i, len(b.Content)})
		total += len(b.Content)
	}

	if total <= maxChars {
		for _, f := range fresh {
			state.markSeen(out[f.idx].ToolUseID)
		}
		return out
	}

	// Sort fresh entries by size descending; replace largest until under budget.
	sort.Slice(fresh, func(a, b int) bool { return fresh[a].size > fresh[b].size })

	for _, f := range fresh {
		if total <= maxChars {
			break
		}
		b := &out[f.idx]
		filePath, err := persistToolResult(dataDir, convID, b.ToolUseID, b.Content)
		if err != nil {
			// persist failed: mark seen, leave content unchanged
			state.markSeen(b.ToolUseID)
			continue
		}
		preview := generatePreview(b.Content, filePath)
		state.setReplacement(b.ToolUseID, preview)
		total -= len(b.Content)
		total += len(preview)
		b.Content = preview
	}

	// Mark remaining fresh entries as seen (not replaced).
	for _, f := range fresh {
		if !state.isSeen(out[f.idx].ToolUseID) && state.getReplacement(out[f.idx].ToolUseID) == "" {
			state.markSeen(out[f.idx].ToolUseID)
		}
	}

	return out
}
```

Also add `"sort"` to the import block in `tool_result_budget.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/agent/ -run "TestEnforcePerMessageBudget" -v
```

Expected: PASS

- [ ] **Step 5: Wire `enforcePerMessageBudget` into `toLLMMessages`**

In `internal/agent/compactor.go`, update the `toLLMMessages` signature and the `tool_result` case:

Change the function signature from:
```go
func toLLMMessages(msgs []*models.Message) []llm.Message {
```
to:
```go
func toLLMMessages(msgs []*models.Message, budget toolResultBudget) []llm.Message {
```

Add a new struct above `toLLMMessages`:
```go
type toolResultBudget struct {
	maxChars  int
	dataDir   string
	convID    string
	state     *ContentReplacementState
}
```

Update the `tool_result` case inside `toLLMMessages`:
```go
case "tool_result":
    var blocks []llm.ContentBlock
    for i < len(msgs) && msgs[i].Role == "tool_result" {
        blocks = append(blocks, parseToolResultBlock(msgs[i].Content))
        i++
    }
    if budget.state != nil && budget.maxChars > 0 {
        blocks = enforcePerMessageBudget(blocks, budget.maxChars, budget.dataDir, budget.convID, budget.state)
    }
    out = append(out, llm.Message{Role: llm.RoleUser, Content: blocks})
```

- [ ] **Step 6: Fix all callers of `toLLMMessages`**

There are two callers:

**In `compactor.go` line ~155** (`BuildHistory`):
```go
history = append(history, toLLMMessages(recent, toolResultBudget{
    maxChars: c.perMessageToolResultMaxChars,
    dataDir:  c.dataDir,
    convID:   conversationID,
    state:    c.replacementState,
})...)
```

**In `agent.go` line ~192** (no-compactor path):
```go
history = toLLMMessages(stored, toolResultBudget{
    maxChars: a.perMessageToolResultMaxChars,
    dataDir:  a.dataDir,
    convID:   conversationID,
    state:    a.replacementState,
})
```

- [ ] **Step 7: Add budget fields to `Compactor`**

In `compactor.go`, update the `Compactor` struct and `NewCompactor`:

```go
type Compactor struct {
	llmClient                    llm.Client
	summaryStore                 summaryStorer
	msgStore                     MessageStorer
	llmModel                     string
	cfg                          config.CompactionConfig
	dataDir                      string
	perMessageToolResultMaxChars int
	replacementState             *ContentReplacementState
}
```

Update `NewCompactor` signature:
```go
func NewCompactor(
	llmClient llm.Client,
	summaryStore summaryStorer,
	msgStore MessageStorer,
	llmModel string,
	cfg config.CompactionConfig,
	dataDir string,
	perMessageToolResultMaxChars int,
	replacementState *ContentReplacementState,
) *Compactor {
	return &Compactor{
		llmClient:                    llmClient,
		summaryStore:                 summaryStore,
		msgStore:                     msgStore,
		llmModel:                     llmModel,
		cfg:                          cfg,
		dataDir:                      dataDir,
		perMessageToolResultMaxChars: perMessageToolResultMaxChars,
		replacementState:             replacementState,
	}
}
```

- [ ] **Step 8: Fix `NewCompactor` callers in tests**

`NewCompactor` signature now has 3 extra params. Update all test call sites:

In `internal/agent/compactor_test.go` line 76:
```go
return NewCompactor(llmC, ss, &fixedMsgStore{msgs: msgs}, "", cfg, t.TempDir(), 0, nil)
```

In `internal/agent/compaction_integration_test.go`, every `NewCompactor(llmC, sumStore, msgStore, ...)` call — append `, t.TempDir(), 0, nil` to each:
```go
c := NewCompactor(llmC, sumStore, msgStore, "test-model", cfg, t.TempDir(), 0, nil)
```

(There are ~6 call sites; apply the same pattern to all of them.)

- [ ] **Step 9: Run full test suite**

```bash
go test ./internal/agent/... -v -count=1 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 10: Commit**

```bash
git add internal/agent/compactor.go internal/agent/tool_result_budget.go internal/agent/tool_result_budget_test.go
git commit -m "feat(agent): Layer 2 per-message aggregate tool result limit"
```

---

### Task 5: Wire budget config through `factory.go`

**Files:**
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Add `BudgetCfg` field to `Factory`**

In `internal/agent/factory.go`, add to the `Factory` struct:

```go
BudgetCfg config.AgentBudgetConfig
```

Where `AgentBudgetConfig` is a new small struct in `config.go`:

```go
// AgentBudgetConfig holds tool result size limits extracted from AgentConfig.
type AgentBudgetConfig struct {
	PerToolResultMaxChars        int
	PerMessageToolResultMaxChars int
}
```

Add a helper method to `AgentConfig`:

```go
func (a AgentConfig) BudgetConfig() AgentBudgetConfig {
	return AgentBudgetConfig{
		PerToolResultMaxChars:        a.PerToolResultMaxChars,
		PerMessageToolResultMaxChars: a.PerMessageToolResultMaxChars,
	}
}
```

- [ ] **Step 2: Update `NewAgent` in `factory.go`**

In `factory.go`, update `NewAgent()` to create a shared `ContentReplacementState` per agent and pass budget fields:

```go
func (f *Factory) NewAgent(systemPrompt string, conversationID string, selectedHostIDs []string) *Agent {
	logger.ForModule("agent").Info().Str("model", f.LLMModel).Str("conv_id", conversationID).Msg("agent factory: creating agent")
	registry := f.buildRegistryWithHosts(conversationID, selectedHostIDs)

	hooks := NewHookChain()
	if f.Enforcer != nil {
		hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
	} else {
		hooks.AddBefore(DefaultRiskHook())
	}

	replacementState := newContentReplacementState()

	var compactor *Compactor
	if f.SummaryStore != nil {
		compactor = NewCompactor(
			f.LLMClient, f.SummaryStore, f.MsgStore, f.LLMModel, f.CompactionCfg,
			f.DataDir, f.BudgetCfg.PerMessageToolResultMaxChars, replacementState,
		)
	}
	return NewAgent(AgentConfig{
		LLMClient:                    f.LLMClient,
		Registry:                     registry,
		Hooks:                        hooks,
		MsgStore:                     f.MsgStore,
		TodoStore:                    f.TodoStore,
		SystemPrompt:                 systemPrompt,
		MaxTurns:                     f.maxTurns(),
		Compactor:                    compactor,
		SkillManager:                 NewSkillManager(f.DataDir),
		DataDir:                      f.DataDir,
		PerToolResultMaxChars:        f.BudgetCfg.PerToolResultMaxChars,
		PerMessageToolResultMaxChars: f.BudgetCfg.PerMessageToolResultMaxChars,
		ReplacementState:             replacementState,
	})
}
```

- [ ] **Step 3: Update `NewHeadlessAgent` similarly**

In `factory.go`, update `NewHeadlessAgent()`:

```go
return NewAgent(AgentConfig{
	LLMClient:                    f.LLMClient,
	Registry:                     registry,
	Hooks:                        hooks,
	MsgStore:                     noopMessageStorer{},
	SystemPrompt:                 systemPrompt,
	MaxTurns:                     f.maxTurns(),
	DataDir:                      f.DataDir,
	PerToolResultMaxChars:        f.BudgetCfg.PerToolResultMaxChars,
	PerMessageToolResultMaxChars: f.BudgetCfg.PerMessageToolResultMaxChars,
	ReplacementState:             newContentReplacementState(),
})
```

- [ ] **Step 4: Wire `BudgetCfg` at the call site**

Find where `Factory` is constructed (likely `internal/api/` or `cmd/`):

```bash
grep -r "agent.Factory{" /Users/cw/fty.ai/spider.ai --include="*.go" -l
```

In each call site, add:

```go
BudgetCfg: cfg.Agent.BudgetConfig(),
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 6: Run full test suite**

```bash
go test ./... 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/agent/factory.go internal/config/config.go
git commit -m "feat(agent): wire tool result budget config through factory"
```
