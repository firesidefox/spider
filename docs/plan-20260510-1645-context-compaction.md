# Context Compaction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent LLM context window overflow by automatically summarizing old messages before each Agent.Run() call, caching summaries in DB, and injecting them as a user+assistant pair at the start of history.

**Architecture:** A new `Compactor` struct in `internal/agent/compactor.go` runs synchronously at the start of `Agent.Run()`. It reads cached summary chunks from `conversation_summaries` DB table, estimates token count of messages after the boundary, and only calls the LLM to generate a new summary chunk when the threshold is exceeded. The `llm.Client` interface gains `Chat()` (non-streaming) and `CountTokens()` methods used by the compactor.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Anthropic count_tokens API for Claude, character-based estimation for OpenAI.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/llm/client.go` | Modify | Add `Chat()` and `CountTokens()` to `Client` interface |
| `internal/llm/claude.go` | Modify | Implement `Chat()` and `CountTokens()` (Anthropic API) |
| `internal/llm/openai.go` | Modify | Implement `Chat()` and `CountTokens()` (char estimation) |
| `internal/llm/models.go` | Modify | Add `knownContextWindows` map and `DefaultThreshold()` |
| `internal/config/config.go` | Modify | Add `CompactionConfig` to `AgentConfig` |
| `internal/db/schema.go` | Modify | Add `conversation_summaries` table + composite index |
| `internal/store/summary.go` | Create | `SummaryStore` CRUD for `conversation_summaries` |
| `internal/store/message.go` | Modify | Add `ListAfterMessage()` method |
| `internal/agent/compactor.go` | Create | `Compactor` — full compaction logic |
| `internal/agent/agent.go` | Modify | English EPA prefix, wire `Compactor` into `Run()` |
| `internal/agent/factory.go` | Modify | Construct `Compactor`, pass to `Agent` |
| `web/src/components/ChatMessage.vue` | Modify | Add compaction status block rendering |
| `web/src/views/ChatView.vue` | Modify | Handle `compacting`/`compacted` SSE events |
| `internal/agent/agent.go` | Modify | Emit compaction events over SSE channel |

---

## Task 1: LLM interface — add Chat() and CountTokens()

**Files:**
- Modify: `internal/llm/client.go`
- Modify: `internal/llm/claude.go`
- Modify: `internal/llm/openai.go`

- [ ] **Step 1: Update Client interface**

In `internal/llm/client.go`, replace the interface:

```go
type Client interface {
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
	Chat(ctx context.Context, req *ChatRequest) (string, error)
	CountTokens(ctx context.Context, msgs []Message) (int, error)
}
```

- [ ] **Step 2: Implement Chat() on ClaudeClient**

Add to `internal/llm/claude.go`:

```go
func (c *ClaudeClient) Chat(ctx context.Context, req *ChatRequest) (string, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": req.MaxTokens,
		"system":     req.System,
		"messages":   req.Messages,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body2, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(body2))
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", nil
}
```

- [ ] **Step 3: Implement CountTokens() on ClaudeClient**

Add to `internal/llm/claude.go`:

```go
func (c *ClaudeClient) CountTokens(ctx context.Context, msgs []Message) (int, error) {
	body := map[string]any{
		"model":    c.model,
		"messages": msgs,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages/count_tokens", bytes.NewReader(jsonBody))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-beta", "token-counting-2024-11-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("claude count_tokens error %d: %s", resp.StatusCode, string(b))
	}
	var result struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return result.InputTokens, nil
}
```

- [ ] **Step 4: Implement Chat() and CountTokens() on OpenAIClient**

Add to `internal/llm/openai.go`:

```go
func estimateTokens(s string) int {
	var cjk, ascii int
	for _, r := range s {
		if r > 0x2E80 {
			cjk++
		} else {
			ascii++
		}
	}
	return cjk + ascii/4
}

func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (string, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": req.MaxTokens,
		"messages":   buildOpenAIMessages(req.System, req.Messages),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(b))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", nil
	}
	return result.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) CountTokens(_ context.Context, msgs []Message) (int, error) {
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m.Content)
	}
	return total, nil
}
```

- [ ] **Step 5: Verify build**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/llm/...
```

Expected: no errors. (mockLLMClient in agent_test.go will fail to compile — fix in Task 6.)

- [ ] **Step 6: Commit**

```bash
git add internal/llm/client.go internal/llm/claude.go internal/llm/openai.go
git commit -m "feat(llm): add Chat() and CountTokens() to Client interface"
```

---

## Task 2: Model context window table + DefaultThreshold()

**Files:**
- Modify: `internal/llm/models.go`

- [ ] **Step 1: Add knownContextWindows and DefaultThreshold**

Add to `internal/llm/models.go` (after the imports block):

```go
var knownContextWindows = map[string]int{
	"claude-sonnet-4-6": 1_000_000,
	"claude-opus-4-7":   1_000_000,
	"claude-haiku-4-5":  200_000,
	"gpt-4o":            128_000,
	"gpt-4o-mini":       128_000,
}

const defaultContextWindow = 120_000

// DefaultThreshold returns the token threshold at which compaction triggers.
// It is 50% of the model's known context window, or 50% of defaultContextWindow
// for unknown models.
func DefaultThreshold(model string) int {
	if w, ok := knownContextWindows[model]; ok {
		return w / 2
	}
	return defaultContextWindow / 2
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/llm/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/llm/models.go
git commit -m "feat(llm): add knownContextWindows table and DefaultThreshold()"
```

---

## Task 3: Config — CompactionConfig

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add CompactionConfig struct and field**

In `internal/config/config.go`, add after the `RuleConfig` struct:

```go
// CompactionConfig controls context compaction behaviour.
type CompactionConfig struct {
	ThresholdTokens  int `yaml:"threshold_tokens"`  // 0 = auto from model table
	RecentTurns      int `yaml:"recent_turns"`       // default 20
	MaxSummaryTokens int `yaml:"max_summary_tokens"` // default 4000
}
```

Add `Compaction CompactionConfig` field to `AgentConfig`:

```go
type AgentConfig struct {
	PermissionMode  string          `yaml:"permission_mode"`
	ApprovalTimeout int             `yaml:"approval_timeout"`
	Rules           []RuleConfig    `yaml:"rules,omitempty" json:"rules,omitempty"`
	Compaction      CompactionConfig `yaml:"compaction"`
}
```

- [ ] **Step 2: Apply defaults in DefaultConfig()**

In `DefaultConfig()`, update the `Agent` field:

```go
Agent: AgentConfig{
	PermissionMode:  "ask",
	ApprovalTimeout: 300,
	Compaction: CompactionConfig{
		RecentTurns:      20,
		MaxSummaryTokens: 4000,
	},
},
```

- [ ] **Step 3: Verify build**

```bash
go build ./internal/config/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add CompactionConfig to AgentConfig"
```

---

## Task 4: DB schema — conversation_summaries table + composite index

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add table to schemaSQL**

In `internal/db/schema.go`, append inside the `schemaSQL` const (before the closing backtick):

```sql
CREATE TABLE IF NOT EXISTS conversation_summaries (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id  TEXT NOT NULL,
    up_to_message_id TEXT NOT NULL,
    chunks           TEXT NOT NULL,
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id)
);

CREATE INDEX IF NOT EXISTS idx_messages_conv_created
ON messages(conversation_id, created_at);
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/db/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add conversation_summaries table and composite index"
```

---
