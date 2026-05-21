# System Prompt Cache Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure system prompt into static (cacheable) and dynamic segments, enable Anthropic/OpenAI prompt caching, add step-by-step narration rules.

**Architecture:** Split BuildSystemPrompt into two SystemBlock segments. Static segment (identity + communicating + tone + tool sections + orchestration + intent) gets cache_control marker. Dynamic segment (host inventory + optional task context) changes per session. All tools always registered to stabilize tools array. Anthropic uses cache_control, OpenAI uses automatic prefix caching.

**Tech Stack:** Go 1.21+, Anthropic API, OpenAI API

---

## File Structure

**Modified files:**
- `internal/llm/client.go` — Add SystemBlock type, change ChatRequest.System to []SystemBlock
- `internal/llm/claude.go` — Serialize []SystemBlock to Anthropic system array with cache_control
- `internal/llm/openai.go` — Concatenate []SystemBlock to single system string
- `internal/agent/factory.go` — Add 3 prompt constants, rewrite BuildSystemPrompt(extraDynamic ...string), unconditional tool registration
- `internal/agent/agent.go` — Delete epaSystemPromptPrefix, change systemPrompt field type to []SystemBlock
- `internal/agent/tools_docs.go` — Add nil knowledgeStore guard
- `internal/agent/tools_todo_task.go` — Add nil todoStore guard
- `internal/agent/tools_topology_context.go` — Add nil topologyStore guard
- `internal/agent/tools_task.go` — Add nil taskStore guard
- `internal/agent/tools_skill.go` — Add empty dataDir guard
- `internal/knowledge/markdown_parser.go` — Fix ChatRequest.System type
- `internal/knowledge/clustering.go` — Fix ChatRequest.System type
- `internal/agent/compactor.go` — Fix ChatRequest.System type
- `internal/scheduler/executor.go` — Fix ChatRequest.System type, add task context to BuildSystemPrompt

**New files:**
- `internal/agent/factory_test.go` — Static prefix stability invariant test

**Modified test files:**
- `internal/agent/agent_test.go` — Fix systemPrompt assertions for []SystemBlock
- `internal/agent/tools_docs_test.go` — Add nil store test
- `internal/agent/tools_todo_task_test.go` — Add nil store test (if exists)
- `internal/agent/tools_topology_context_test.go` — Add nil store test (if exists)
- `internal/agent/tools_task_test.go` — Add nil store test (if exists)
- `internal/agent/tools_skill_test.go` — Add nil store test (if exists)

---

## Task 1: Add SystemBlock Type to LLM Client

**Files:**
- Modify: `internal/llm/client.go:60-98`

- [ ] **Step 1: Add SystemBlock type after ToolCall**

```go
type SystemBlock struct {
    Text         string  `json:"text"`
    CacheControl *string `json:"cache_control,omitempty"` // "ephemeral" for Anthropic, nil otherwise
}
```

Insert after line 73 (after ToolCall definition).

- [ ] **Step 2: Change ChatRequest.System field type**

Find ChatRequest struct (line 87-92). Change:

```go
// Before
type ChatRequest struct {
    System    string    `json:"system"`
    Messages  []Message `json:"messages"`
    Tools     []ToolDef `json:"tools,omitempty"`
    MaxTokens int       `json:"max_tokens"`
}

// After
type ChatRequest struct {
    System    []SystemBlock `json:"-"`             // Serialized by each provider
    Messages  []Message     `json:"messages"`
    Tools     []ToolDef     `json:"tools,omitempty"`
    MaxTokens int           `json:"max_tokens"`
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/llm/`
Expected: Compilation errors in claude.go, openai.go, and callers (expected, will fix in next tasks)

- [ ] **Step 4: Commit**

```bash
git add internal/llm/client.go
git commit -m "feat(llm): add SystemBlock type, change ChatRequest.System to []SystemBlock

Breaking change: ChatRequest.System changed from string to []SystemBlock.
Providers must serialize blocks according to their API format."
```

---

## Task 2: Adapt Anthropic Provider for SystemBlock

**Files:**
- Modify: `internal/llm/claude.go:ChatStream` and `Chat` methods

- [ ] **Step 1: Find ChatStream body construction**

Locate where `body := map[string]any{...}` is built (around line 160-180).

- [ ] **Step 2: Replace system field logic**

Find the line that sets `body["system"] = req.System` and replace with:

```go
// System prompt as array of blocks with cache_control
if len(req.System) > 0 {
    systemArray := make([]map[string]any, len(req.System))
    for i, block := range req.System {
        systemArray[i] = map[string]any{
            "type": "text",
            "text": block.Text,
        }
        if block.CacheControl != nil {
            systemArray[i]["cache_control"] = map[string]any{
                "type": *block.CacheControl,
            }
        }
    }
    body["system"] = systemArray
}
```

- [ ] **Step 3: Apply same change to Chat method**

Find the non-streaming Chat method and apply identical system array logic.

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/llm/`
Expected: Still errors in openai.go and callers (expected)

- [ ] **Step 5: Commit**

```bash
git add internal/llm/claude.go
git commit -m "feat(llm): adapt Anthropic provider for SystemBlock array

Serialize []SystemBlock to Anthropic system array format with cache_control markers."
```

---

## Task 3: Adapt OpenAI Provider for SystemBlock

**Files:**
- Modify: `internal/llm/openai.go:ChatStream` and `Chat` methods

- [ ] **Step 1: Find ChatStream messages construction**

Locate where messages array is built (around line 180-200).

- [ ] **Step 2: Add system block concatenation before messages**

Before the messages loop, add:

```go
var systemText string
if len(req.System) > 0 {
    var parts []string
    for _, block := range req.System {
        parts = append(parts, block.Text)
    }
    systemText = strings.Join(parts, "\n\n")
}

messages := []map[string]any{}
if systemText != "" {
    messages = append(messages, map[string]any{
        "role":    "system",
        "content": systemText,
    })
}
```

- [ ] **Step 3: Apply same change to Chat method**

Find the non-streaming Chat method and apply identical concatenation logic.

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/llm/`
Expected: Still errors in callers (agent, knowledge, scheduler)

- [ ] **Step 5: Commit**

```bash
git add internal/llm/openai.go
git commit -m "feat(llm): adapt OpenAI provider for SystemBlock array

Concatenate []SystemBlock into single system message. OpenAI automatic prefix caching works on byte-identical prefixes."
```

---

## Task 4: Add Prompt Constants to Factory

**Files:**
- Modify: `internal/agent/factory.go` (add constants before BuildSystemPrompt)

- [ ] **Step 1: Add identityPrompt constant**

Insert after package imports, before Factory struct:

```go
const identityPrompt = `You are Spider, an intelligent network operations assistant. Use the available tools to execute CLI commands, verify configurations, query REST APIs, and answer questions about network infrastructure.`
```

- [ ] **Step 2: Add communicatingPrompt constant**

```go
const communicatingPrompt = `## Communicating with the user

Tool calls and tool results are mostly invisible to the user — they see only your text output and the final result. Treat your text as the only reliable channel for explaining what is happening.

**Before your first tool call** in a turn, state in one short sentence what you are about to do.

**During work**, send a short update only at these moments:
- You found something load-bearing (a config error, a root cause, a host that matches the filter).
- You are changing direction based on what you saw.
- A risky / write-class command (RunCommand L2/L3, RunCommandBatch) is about to run — restate the intent and target hosts.

**Do not narrate** routine reads (GetHosts, ListAccessFaces, SearchDocs). Do not echo command output that the UI already renders. Do not write "I will now ...", "Next, I will ..." between every tool call.

**End-of-turn**: one sentence on the result. No bullet recap of every step. If the result is a table or list, the table IS the answer — don't prepend "Here is the result:".`
```

- [ ] **Step 3: Add toneAndStylePrompt constant**

```go
const toneAndStylePrompt = `## Tone and Style

- Always respond in Simplified Chinese. Use English only for technical terms, command output, and code.
- Be direct. Lead with the result. No pleasantries ("好的", "当然", "我来帮您", "没问题").
- **Intent statements are NOT preamble.** The one-sentence statement before your first tool call ("先列出 cisco 设备查端口状态") is required by the Communicating with the user section. Do not omit it. The distinction: pleasantries are social niceties; intent statements explain the next action.
- Do not use a colon before tool calls. Writing "让我看看：" followed by a tool call becomes a broken sentence when the tool call is not rendered. Rewrite as "让我看看。" + tool call, or state the intent directly.
- For multi-host results, use tables or lists — not prose.
- Reference code with file_path:line_number format (e.g., internal/agent/factory.go:193).
- Reference hosts by hostname, not host_id.
- Do not use emojis unless the user explicitly requests them.`
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/agent/`
Expected: Compiles (constants don't break anything yet)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/factory.go
git commit -m "feat(agent): add identityPrompt, communicatingPrompt, toneAndStylePrompt constants

Three new prompt constants for static segment:
- identityPrompt: Spider identity without host count
- communicatingPrompt: Tool invisibility + 3 timing rules (first-call / load-bearing / direction-change)
- toneAndStylePrompt: Chinese response + colon ban + intent statement exemption"
```

---

## Task 5: Rewrite BuildSystemPrompt to Return SystemBlock Array

**Files:**
- Modify: `internal/agent/factory.go:BuildSystemPrompt` function (line ~193-243)

- [ ] **Step 1: Change function signature**

Find `func (f *Factory) BuildSystemPrompt() string` and change to:

```go
func (f *Factory) BuildSystemPrompt(extraDynamic ...string) []llm.SystemBlock
```

- [ ] **Step 2: Replace function body (part 1: static segment)**

Replace the entire function body with:

```go
    var static strings.Builder
    static.WriteString(identityPrompt)
    static.WriteString("\n\n")
    static.WriteString(communicatingPrompt)
    static.WriteString("\n\n")
    static.WriteString(toneAndStylePrompt)
    static.WriteString("\n\n")

    reg := f.buildRegistry("")
    for _, tool := range reg.All() {
        if sp, ok := tool.(SystemPromptSection); ok {
            section := sp.SystemPromptSection()
            if strings.TrimSpace(section) != "" {
                static.WriteString(section)
                static.WriteString("\n\n")
            }
        }
    }

    static.WriteString(orchestrationPrompt)
    static.WriteString("\n\n")
    static.WriteString(intentFieldPrompt)
```

- [ ] **Step 3: Replace function body (part 2: dynamic segment)**

Continue in same function:

```go
    dynamic := f.buildEnvironmentSection()
    for _, extra := range extraDynamic {
        dynamic += "\n\n" + extra
    }

    cacheMark := "ephemeral"
    return []llm.SystemBlock{
        {Text: static.String(), CacheControl: &cacheMark},
        {Text: dynamic},
    }
}
```

- [ ] **Step 4: Add buildEnvironmentSection helper**

After BuildSystemPrompt, add new function:

```go
func (f *Factory) buildEnvironmentSection() string {
    allHosts, err := f.Hosts.List("")
    if err != nil || len(allHosts) == 0 {
        return "## Environment\n\nNo hosts are currently registered."
    }
    vendorCount := make(map[string]int)
    for _, h := range allHosts {
        v := h.Vendor
        if v == "" {
            v = "unknown"
        }
        vendorCount[v]++
    }
    // Sort by vendor name to avoid map iteration non-determinism
    vendors := make([]string, 0, len(vendorCount))
    for v := range vendorCount {
        vendors = append(vendors, v)
    }
    sort.Strings(vendors)
    var parts []string
    for _, v := range vendors {
        parts = append(parts, fmt.Sprintf("%s(%d)", v, vendorCount[v]))
    }
    return fmt.Sprintf(
        "## Environment\n\nManaged devices: %d total — %s.",
        len(allHosts), strings.Join(parts, ", "),
    )
}
```

- [ ] **Step 5: Delete old layer1 construction code**

Find and delete the old host inventory code that was at the start of BuildSystemPrompt (lines that built `layer1` string with vendor counts).

- [ ] **Step 6: Verify it compiles**

Run: `go build ./internal/agent/`
Expected: Errors in NewAgent/NewHeadlessAgent callers (expected, will fix next)

- [ ] **Step 7: Commit**

```bash
git add internal/agent/factory.go
git commit -m "feat(agent): rewrite BuildSystemPrompt to return []SystemBlock

- Change signature: BuildSystemPrompt(extraDynamic ...string) []llm.SystemBlock
- Static segment: identity + communicating + tone + tool sections + orchestration + intent
- Dynamic segment: environment (host inventory) + optional extraDynamic
- Static segment gets cache_control=ephemeral marker
- Add buildEnvironmentSection helper with sorted vendor output"
```

---

## Task 6: Delete EPA Prefix from Agent

**Files:**
- Modify: `internal/agent/agent.go:21-29, 129`

- [ ] **Step 1: Delete epaSystemPromptPrefix constant**

Find and delete lines 21-29:

```go
const epaSystemPromptPrefix = `## Behavioral Constraints

Process tasks in the following order:

Explore: Use read-only tools to gather information first. Do not perform any side-effecting operations until you have a clear understanding of the current state.
Plan: Based on exploration results, reason through a complete execution plan internally. Clarify the purpose and expected outcome of each step.
Act: Execute the plan step by step, verifying results after each step before continuing. If anything unexpected occurs, re-enter Explore — do not proceed blindly.

`
```

- [ ] **Step 2: Remove prefix concatenation in NewAgent**

Find line 129 in NewAgent constructor:

```go
// Before
systemPrompt: epaSystemPromptPrefix + cfg.SystemPrompt,

// After
systemPrompt: cfg.SystemPrompt,
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/agent/`
Expected: Still errors (systemPrompt type mismatch)

- [ ] **Step 4: Commit**

```bash
git add internal/agent/agent.go
git commit -m "refactor(agent): delete epaSystemPromptPrefix duplication

EPA flow now lives only in orchestrationPrompt (factory.go).
Remove prefix concatenation in NewAgent constructor."
```

---

## Task 7: Change Agent SystemPrompt Field Type

**Files:**
- Modify: `internal/agent/agent.go:81, 100, 129, 239`

- [ ] **Step 1: Change Agent struct field type**

Find Agent struct (line ~81), change:

```go
// Before
systemPrompt  string

// After
systemPrompt  []llm.SystemBlock
```

- [ ] **Step 2: Change AgentConfig field type**

Find AgentConfig struct (line ~100), change:

```go
// Before
SystemPrompt string

// After
SystemPrompt []llm.SystemBlock
```

- [ ] **Step 3: Update LLM call site**

Find where ChatStream is called (line ~239), change:

```go
// Before
resp, err := a.llmClient.ChatStream(ctx, &llm.ChatRequest{
    System:    a.systemPrompt,  // was string
    Messages:  msgs,
    Tools:     a.registry.Definitions(),
    MaxTokens: 4096,
})

// After (no change needed, type already matches)
resp, err := a.llmClient.ChatStream(ctx, &llm.ChatRequest{
    System:    a.systemPrompt,  // now []llm.SystemBlock
    Messages:  msgs,
    Tools:     a.registry.Definitions(),
    MaxTokens: 4096,
})
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/agent/`
Expected: Errors in factory.go NewAgent/NewHeadlessAgent (expected)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/agent.go
git commit -m "feat(agent): change systemPrompt field type to []llm.SystemBlock

Agent and AgentConfig now use []llm.SystemBlock instead of string.
LLM call site already compatible."
```

// __CONTINUE_HERE__
