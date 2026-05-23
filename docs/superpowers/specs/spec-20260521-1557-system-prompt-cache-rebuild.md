# System Prompt Restructure for Prompt Caching

**Date:** 2026-05-21  
**Status:** Implemented — static/dynamic two-segment split in factory.go, CacheControl on static segment

## Problem

Current system prompt construction has three issues blocking effective prompt caching:

1. **Duplicate EPA flow** — `epaSystemPromptPrefix` (agent.go:21) and `orchestrationPrompt` (factory.go:174) both describe Explore/Plan/Act, causing redundancy.
2. **No "tool calls are invisible" mental model** — Model doesn't understand why it needs to narrate steps, leading to inconsistent explanations.
3. **Dynamic content mixed with static** — Host inventory appears at the start of the prompt (factory.go:212), causing cache misses when host count changes.

## Goals

1. **Enable prompt caching** — Split system prompt into static (cacheable) and dynamic (session-specific) segments.
2. **Consistent step-by-step narration** — Add explicit timing rules (first-call / load-bearing / direction-change) borrowed from Claude Code.
3. **Single source of truth** — Remove EPA duplication, keep only the detailed version.
4. **Support both Anthropic and OpenAI** — Anthropic uses `cache_control` markers, OpenAI uses automatic prefix caching.

## Success Criteria

- Static prefix byte-identical across sessions with same factory config (different host counts).
- Model explains intent before first tool call in >90% of turns.
- Anthropic provider achieves >80% cache hit rate on static segment after warmup.
- OpenAI provider benefits from automatic prefix caching without code changes.

## Architecture

### Two-Segment Structure

```
[STATIC SEGMENT — cacheable across sessions]
  identityPrompt          "You are Spider..."
  communicatingPrompt     Tool invisibility + 3 timing rules
  toneAndStylePrompt      Chinese response + colon ban + intent exemption
  tool sections           Registry order (GetHosts → CLI → Batch → Verify → API → SearchDocs)
  orchestrationPrompt     EPA → Plan → Confirm → Act → Verify (single source)
  intentFieldPrompt       Intent field writing rules

[DYNAMIC SEGMENT — session/environment specific]
  environmentSection      Host count + vendor breakdown
```

No explicit `---` boundary marker. Segments separated by `## Environment` heading.

### Key Invariants

1. **Static segment = literal constants only** — No runtime branches, map iteration, or string interpolation.
2. **Tool registration order fixed** — Registry.All() returns deterministic slice order.
3. **All tools always registered** — SearchDocs, Todo, Topology, Task, Skill tools registered unconditionally. Actual blocking happens at hook/Execute level when store is nil or feature disabled.
4. **Tools array stable** — Anthropic prompt caching covers tools + system + messages prefix. Variable tool definitions fragment cache. All tools must be registered to stabilize the tools array sent to LLM.

## Design Details

### 1. Delete EPA Prefix Duplication

**File:** `internal/agent/agent.go`

Delete lines 21-29:
```go
const epaSystemPromptPrefix = `## Behavioral Constraints ...`
```

Change line 129:
```go
// Before
systemPrompt: epaSystemPromptPrefix + cfg.SystemPrompt,
// After
systemPrompt: cfg.SystemPrompt,
```

EPA flow now lives only in `orchestrationPrompt` (factory.go:174).

### 2. Add Communicating Prompt

**File:** `internal/agent/factory.go`

Add constant (照搬 Claude Code 三条规则，换成 spider.ai 工具名):

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

### 3. Rewrite Tone and Style

**File:** `internal/agent/factory.go`

Replace line 241 inline string with constant:

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

Key change: **"Intent statements are NOT preamble"** resolves conflict between "No preamble" and "state intent before first tool call".

### 4. Add Identity Prompt

**File:** `internal/agent/factory.go`

Add constant:

```go
const identityPrompt = `You are Spider, an intelligent network operations assistant. Use the available tools to execute CLI commands, verify configurations, query REST APIs, and answer questions about network infrastructure.`
```

Host count moved to dynamic segment.

### 5. Rebuild BuildSystemPrompt

**File:** `internal/agent/factory.go`

Replace `BuildSystemPrompt()` function (line 193-243):

```go
// BuildSystemPrompt builds a two-segment system prompt.
//
// STATIC segment (cacheable across sessions):
//   identityPrompt → communicatingPrompt → toneAndStylePrompt
//   → tool sections (registry order) → orchestrationPrompt → intentFieldPrompt
//
// DYNAMIC segment (session/environment specific):
//   environment section (host inventory, vendor counts)
//   + optional extraDynamic blocks (e.g., task context for headless agents)
//
// The static segment MUST be byte-identical across sessions with the same
// factory configuration. Any runtime branch, map iteration, or interpolation
// inside the static segment will fragment provider-side prefix caching.
func (f *Factory) BuildSystemPrompt(extraDynamic ...string) []llm.SystemBlock {
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
    // Sort by vendor name to avoid map iteration non-determinism leaking into
    // the dynamic segment. (Dynamic segment doesn't need stability for caching,
    // but determinism helps debugging.)
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

Return type changed: `string` → `[]llm.SystemBlock`.

### 6. LLM Client Interface Changes

**File:** `internal/llm/client.go`

Add SystemBlock type:

```go
type SystemBlock struct {
    Text         string  `json:"text"`
    CacheControl *string `json:"cache_control,omitempty"` // "ephemeral" for Anthropic, nil otherwise
}
```

Change ChatRequest.System field:

```go
type ChatRequest struct {
    System    []SystemBlock  `json:"-"`             // Serialized by each provider
    Messages  []Message      `json:"messages"`
    Tools     []ToolDef      `json:"tools,omitempty"`
    MaxTokens int            `json:"max_tokens"`
}
```

`json:"-"` because Anthropic and OpenAI have different system field formats.

### 7. Anthropic Provider Adaptation

**File:** `internal/llm/claude.go`

In `ChatStream` and `Chat`, construct system array:

```go
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

Anthropic API format:
```json
{
  "system": [
    {"type": "text", "text": "...", "cache_control": {"type": "ephemeral"}},
    {"type": "text", "text": "..."}
  ]
}
```

**Note:** Verify current Anthropic API requirements for prompt caching. Basic 5-minute ephemeral caching works with `cache_control` alone. Extended TTL or beta features may require an `anthropic-beta` header. Check the target account/model tier and add to `setHeaders` if needed:
```go
req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")  // Only if required
```

### 8. OpenAI Provider Adaptation

**File:** `internal/llm/openai.go`

In `ChatStream` and `Chat`, concatenate blocks:

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

OpenAI automatic prefix caching works on byte-identical prefixes. No explicit markers needed.

### 9. Agent Config Changes

**File:** `internal/agent/agent.go`

Change AgentConfig.SystemPrompt type (line 100):

```go
type AgentConfig struct {
    // ... existing fields
    SystemPrompt []llm.SystemBlock  // Changed from string
}
```

Change Agent struct field (line 81):

```go
type Agent struct {
    // ... existing fields
    systemPrompt []llm.SystemBlock  // Changed from string
}
```

Change NewAgent constructor (line 129):

```go
systemPrompt: cfg.SystemPrompt,  // Type is now []llm.SystemBlock
```

Change LLM call site (line 239):

```go
resp, err := a.llmClient.ChatStream(ctx, &llm.ChatRequest{
    System:    a.systemPrompt,  // Now []llm.SystemBlock
    Messages:  msgs,
    Tools:     a.registry.Definitions(),
    MaxTokens: 4096,
})
```

### 10. Factory Agent Creation

**File:** `internal/agent/factory.go`

Change NewAgent (line 97) and NewHeadlessAgent (line 134):

```go
// Normal agent
systemPrompt := f.BuildSystemPrompt()
agent := NewAgent(AgentConfig{
    SystemPrompt: systemPrompt,
    // ... other fields
})

// Headless agent with task context
taskPrompt := fmt.Sprintf("## Task\n\nGoal: %s\nTarget hosts: %s", task.Goal, hostnames)
systemPrompt := f.BuildSystemPrompt(taskPrompt)
agent := NewAgent(AgentConfig{
    SystemPrompt: systemPrompt,
    // ... other fields
})
```

Task context goes into dynamic segment, preserving static segment cache.

### 11. All Tools Always-Register Strategy

**Rationale:** Anthropic prompt caching covers the complete prefix: tools array + system + messages. Variable tool definitions fragment cache across sessions. To achieve stable cache hits, the tools array sent to LLM must be byte-identical across sessions.

**File:** `internal/agent/factory.go`

Change buildRegistryWithHosts (line 250-270) to unconditionally register all tools:

```go
func (f *Factory) buildRegistryWithHosts(conversationID string, selectedHostIDs []string) *ToolRegistry {
    registry := NewToolRegistry()
    listTool := NewGetHostsTool(f.Hosts, f.AccessFaces)
    listTool.selectedHostIDs = selectedHostIDs
    registry.Register(listTool)
    registry.Register(NewExecuteCLITool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
    registry.Register(NewBatchExecuteTool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
    registry.Register(NewVerifyTool(f.Hosts, f.AccessFaces, f.SSHPool, f.SSHKeys))
    registry.Register(NewCallRESTAPITool(f.AccessFaces))
    
    // Unconditional registration — no if checks
    registry.Register(NewSearchDocsTool(f.KnowledgeStore, f.Embedder))
    registry.Register(NewTodoTool(f.TodoStore))
    registry.Register(NewGetTopologyContextTool(f.TopologyStore))
    registry.Register(NewTaskTool(f.TaskStore))
    registry.Register(NewInvokeSkillTool(f.DataDir, f.MsgStore, conversationID))
    
    return registry
}
```

Remove all `if f.DisableSearchDocs`, `if f.TodoStore != nil`, `if f.TopologyStore != nil`, `if f.TaskStore != nil`, `if f.DataDir != ""` checks.

Tool schema always sent to LLM. Actual blocking happens at two levels:

**Level 1: Hook interception** (optional, early rejection)

Add to hook chain construction (factory.go:102-106):

```go
hooks := NewHookChain()
if f.Enforcer != nil {
    hooks.AddBefore(PermissionHook(f.Enforcer, f.PermissionMode))
} else {
    hooks.AddBefore(DefaultRiskHook())
}

// Feature disable hooks
if f.DisableSearchDocs {
    hooks.AddBefore(func(toolName string, input map[string]any, riskLevel RiskLevel) *HookResult {
        if toolName == "SearchDocs" {
            return &HookResult{
                Action:    HookDeny,
                RiskLevel: riskLevel,
                Reason:    "SearchDocs is disabled by configuration",
            }
        }
        return &HookResult{Action: HookAllow, RiskLevel: riskLevel}
    })
}
```

**Level 2: Execute nil guard** (defensive, friendly error)

Each tool's Execute method checks for nil store at entry:

**File:** `internal/agent/tools_docs.go`

```go
func (t *SearchDocsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
    if t.knowledgeStore == nil {
        return &ToolResult{
            Content: "SearchDocs is disabled in this deployment. Knowledge base is not available.",
            IsError: true,
        }, nil
    }
    
    // ... existing logic
}
```

Apply same pattern to:
- `tools_todo_task.go` → check `t.todoStore != nil`
- `tools_topology_context.go` → check `t.topologyStore != nil`
- `tools_task.go` → check `t.taskStore != nil`
- `tools_skill.go` → check `t.dataDir != ""`

`Description()` and `SystemPromptSection()` for all tools are already literal constants. No changes needed.

### 12. Invariant Test

**File:** `internal/agent/factory_test.go` (new file)

Add test to ensure static prefix stability:

```go
package agent

import (
    "fmt"
    "testing"
)

func TestSystemPromptStaticPrefixStable(t *testing.T) {
    // Same factory config, different host counts → static block must be byte-identical
    f := newTestFactory(t)
    blocks1 := f.BuildSystemPrompt()
    staticPrefix1 := blocks1[0].Text

    // Add 5 cisco hosts
    for i := 0; i < 5; i++ {
        addTestHost(t, f.Hosts, fmt.Sprintf("cisco-%d", i), "cisco")
    }
    blocks2 := f.BuildSystemPrompt()
    staticPrefix2 := blocks2[0].Text

    // Add 3 huawei hosts
    for i := 0; i < 3; i++ {
        addTestHost(t, f.Hosts, fmt.Sprintf("huawei-%d", i), "huawei")
    }
    blocks3 := f.BuildSystemPrompt()
    staticPrefix3 := blocks3[0].Text

    if staticPrefix1 != staticPrefix2 {
        t.Fatalf("static prefix changed after adding cisco hosts:\n  len1=%d\n  len2=%d\n  first diff at byte %d",
            len(staticPrefix1), len(staticPrefix2), firstDiffOffset(staticPrefix1, staticPrefix2))
    }

    if staticPrefix2 != staticPrefix3 {
        t.Fatalf("static prefix changed after adding huawei hosts:\n  len2=%d\n  len3=%d\n  first diff at byte %d",
            len(staticPrefix2), len(staticPrefix3), firstDiffOffset(staticPrefix2, staticPrefix3))
    }
}

func firstDiffOffset(a, b string) int {
    minLen := len(a)
    if len(b) < minLen {
        minLen = len(b)
    }
    for i := 0; i < minLen; i++ {
        if a[i] != b[i] {
            return i
        }
    }
    return minLen
}

func newTestFactory(t *testing.T) *Factory {
    // Create in-memory DB + empty factory
    db := setupTestDB(t)
    hosts := store.NewHostStore(db)
    faces := store.NewAccessFaceStore(db)
    return &Factory{
        Hosts:       hosts,
        AccessFaces: faces,
        // Other fields zero-valued, test only cares about BuildSystemPrompt
    }
}

func addTestHost(t *testing.T, hosts *store.HostStore, hostname, vendor string) {
    _, err := hosts.Create(hostname, "192.168.1.1", vendor, []string{"test"})
    if err != nil {
        t.Fatalf("addTestHost failed: %v", err)
    }
}
```

### 13. Agent Test Fixes

**File:** `internal/agent/agent_test.go`

Replace lines 239-242:

```go
// Before
if !strings.HasPrefix(a.systemPrompt, "## Behavioral Constraints") {
    t.Errorf("systemPrompt should start with EPA prefix, got: %q", a.systemPrompt[:min(50, len(a.systemPrompt))])
}
if !strings.Contains(a.systemPrompt, "你是运维助手。") {
    t.Errorf("systemPrompt should contain user prompt")
}

// After
if len(a.systemPrompt) == 0 {
    t.Error("systemPrompt (SystemBlocks) should not be empty")
}
if len(a.systemPrompt) < 2 {
    t.Errorf("expected 2 SystemBlocks (static + dynamic), got %d", len(a.systemPrompt))
}
// Check static block contains core sections
staticText := a.systemPrompt[0].Text
if !strings.Contains(staticText, "You are Spider") {
    t.Error("static block should contain identity prompt")
}
if !strings.Contains(staticText, "Communicating with the user") {
    t.Error("static block should contain communicating prompt")
}
// Check dynamic block contains environment info
dynamicText := a.systemPrompt[1].Text
if !strings.Contains(dynamicText, "## Environment") {
    t.Error("dynamic block should contain environment section")
}
// Check cache control markers
if a.systemPrompt[0].CacheControl == nil || *a.systemPrompt[0].CacheControl != "ephemeral" {
    t.Error("static block should have cache_control=ephemeral")
}
if a.systemPrompt[1].CacheControl != nil {
    t.Error("dynamic block should not have cache_control")
}
```

### 14. Tool Nil Store Tests

**File:** `internal/agent/tools_docs_test.go`

Add test:

```go
func TestSearchDocsToolNilStore(t *testing.T) {
    tool := NewSearchDocsTool(nil, nil)
    result, err := tool.Execute(context.Background(), map[string]any{
        "mode":       "sections",
        "scope_type": "kb",
    })
    if err != nil {
        t.Fatalf("Execute returned error: %v", err)
    }
    if !result.IsError {
        t.Error("Expected IsError=true when store is nil")
    }
    if !strings.Contains(result.Content, "disabled") {
        t.Errorf("Expected 'disabled' in content, got: %s", result.Content)
    }
}
```

Add similar tests for Todo, Topology, Task, Skill tools with nil stores.

### 15. Other Code Sites

Search for all `ChatRequest` constructions in production and test code:

```bash
rg "ChatRequest\{" internal cmd
```

Change all occurrences from:

```go
req := &llm.ChatRequest{
    System: "You are...",
    ...
}
```

To:

```go
req := &llm.ChatRequest{
    System: []llm.SystemBlock{{Text: "You are..."}},
    ...
}
```

Known production sites:
- `internal/knowledge/markdown_parser.go:72`
- `internal/knowledge/clustering.go:41`
- `internal/agent/compactor.go:298`
- `internal/scheduler/executor.go:233`

Some may have no system prompt (empty slice is valid). All must compile after the type change.

## Implementation Order

1. Add new constants (identityPrompt, communicatingPrompt, toneAndStylePrompt) to factory.go
2. Add SystemBlock type to llm/client.go
3. Change ChatRequest.System type to []SystemBlock
4. Adapt claude.go and openai.go to handle []SystemBlock
5. Rewrite BuildSystemPrompt(extraDynamic ...string) to return []SystemBlock
6. Delete epaSystemPromptPrefix from agent.go
7. Change AgentConfig.SystemPrompt type to []SystemBlock
8. Update all agent creation call sites (factory.go NewAgent/NewHeadlessAgent with task context)
9. All tools always-register: remove all conditional registration checks in buildRegistryWithHosts
10. Add nil guards to SearchDocs, Todo, Topology, Task, Skill Execute methods
11. Add feature disable hooks (optional, for early rejection)
12. Add invariant test (factory_test.go)
13. Fix agent_test.go assertions
14. Fix all production and test ChatRequest constructions (rg "ChatRequest\\{" internal cmd)
15. Add nil store tests for all optional tools
16. Run full test suite
17. Manual verification: start server, send query, check cache hit metrics

## Verification

### Unit Tests

```bash
go test ./internal/agent/... -v
go test ./internal/llm/... -v
```

All tests must pass.

### Static Prefix Stability

```bash
go test -run TestSystemPromptStaticPrefixStable ./internal/agent/
```

Must pass. If fails, indicates runtime branch or map iteration in static segment.

### Manual Cache Hit Check (Anthropic)

1. Start server with Anthropic provider
2. Send first query → cache miss expected
3. Send second query (different user input, same host config) → cache hit expected
4. Check logs for cache hit metrics (if provider exposes them)

### Manual Cache Hit Check (OpenAI)

OpenAI doesn't expose cache hit metrics. Verify by:
1. Send two queries with same static prefix
2. Check response latency — second query should be faster if cache hit

### Step-by-Step Narration Check

Send 10 diverse queries. Count:
- Queries with intent statement before first tool call
- Queries with unnecessary narration between routine tool calls

Target: >90% have intent statement, <10% have unnecessary narration.

## Risks

### Breaking Changes

- All code constructing `ChatRequest` must change `System` from string to `[]SystemBlock`
- All code calling `BuildSystemPrompt()` must handle `[]SystemBlock` return type
- `agent.systemPrompt` field type changed

**Mitigation:** Comprehensive grep + test coverage. No backward compatibility path — clean break.

### Cache Hit Rate Lower Than Expected

If Anthropic cache hit rate <50% after warmup:
- Check static prefix stability test passes
- Add logging to BuildSystemPrompt to dump static block hash
- Verify no runtime branches in static segment

### Model Behavior Regression

New prompts might change model behavior:
- "Communicating with the user" might cause over-narration initially
- "Intent statements are NOT preamble" distinction might confuse model

**Mitigation:** A/B test on 100 queries before full rollout. Revert if step-by-step narration <80% or unnecessary narration >20%.

### SearchDocs Always-Register Confusion

Users might see SearchDocs in tool list but get "disabled" error when calling.

**Mitigation:** Hook-level rejection returns clear error message. UI can hide disabled tools based on `DisableSearchDocs` flag.

## Future Work

- Add cache hit rate metrics to `/api/metrics` endpoint
- Expose cache control strategy as config option (ephemeral vs persistent)
- Support other providers (Ollama, Azure OpenAI) with provider-specific caching
- Dynamic segment optimization: move rarely-changing content (e.g., topology) to static segment with invalidation logic

## References

- Claude Code prompts.ts: `/Users/cw/fty.ai/claude-code-source-code/src/constants/prompts.ts`
- Anthropic prompt caching docs: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching
- OpenAI prompt caching: https://platform.openai.com/docs/guides/prompt-caching
