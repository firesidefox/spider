# System Prompt Layered Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor system prompt construction into a three-layer architecture where each tool owns its behavior guidance via a `SystemPromptSection` interface.

**Architecture:** Add `SystemPromptSection` interface to `tools.go`; refactor `ToolRegistry` to preserve insertion order; move per-tool prompt sections into each tool file; keep cross-tool orchestration constants in `factory.go`.

**Tech Stack:** Go

---

### Task 1: Refactor `ToolRegistry` to ordered and add `SystemPromptSection` interface

**Files:**
- Modify: `internal/agent/tools.go:42-69`

- [ ] **Step 1: Write failing tests**

Add to `internal/agent/tools_test.go`:

```go
func TestToolRegistry_PreservesInsertionOrder(t *testing.T) {
	r := NewToolRegistry()
	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		m := &registryMockTool{}
		m.name = n
		r.Register(m)
	}
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(all))
	}
	for i, tool := range all {
		if tool.Name() != names[i] {
			t.Errorf("position %d: expected %q, got %q", i, names[i], tool.Name())
		}
	}
}

func TestToolRegistry_GetStillWorks(t *testing.T) {
	r := NewToolRegistry()
	m := &registryMockTool{}
	m.name = "mytool"
	r.Register(m)
	got, ok := r.Get("mytool")
	if !ok || got.Name() != "mytool" {
		t.Errorf("Get failed after ordered refactor")
	}
}

func TestSystemPromptSection_CollectedInOrder(t *testing.T) {
	r := NewToolRegistry()

	type promptTool struct {
		registryMockTool
		section string
	}
	pt1 := &promptTool{section: "section-A"}
	pt1.name = "tool-a"
	pt2 := &promptTool{section: "section-B"}
	pt2.name = "tool-b"

	// Only pt1 and pt2 implement SystemPromptSection
	r.Register(pt1)
	r.Register(&registryMockTool{name: "no-prompt"})
	r.Register(pt2)

	var sections []string
	for _, tool := range r.All() {
		if sp, ok := tool.(SystemPromptSection); ok {
			s := sp.SystemPromptSection()
			if strings.TrimSpace(s) != "" {
				sections = append(sections, s)
			}
		}
	}
	if len(sections) != 2 || sections[0] != "section-A" || sections[1] != "section-B" {
		t.Errorf("unexpected sections: %v", sections)
	}
}
```

Note: `registryMockTool` already exists in `tools_test.go` with a `name` field. The `promptTool` inline struct adds `SystemPromptSection()`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/... -run "TestToolRegistry_PreservesInsertionOrder|TestToolRegistry_GetStillWorks|TestSystemPromptSection_CollectedInOrder" -v
```

Expected: FAIL — `All()` undefined, `SystemPromptSection` undefined

- [ ] **Step 3: Implement**

Replace `tools.go` lines 42–69 with:

```go
// SystemPromptSection is implemented by tools that contribute a section to the system prompt.
type SystemPromptSection interface {
	SystemPromptSection() string
}

type ToolRegistry struct {
	tools []Tool
	index map[string]int
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{index: make(map[string]int)}
}

func (r *ToolRegistry) Register(t Tool) {
	if _, exists := r.index[t.Name()]; exists {
		r.tools[r.index[t.Name()]] = t
		return
	}
	r.index[t.Name()] = len(r.tools)
	r.tools = append(r.tools, t)
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	i, ok := r.index[name]
	if !ok {
		return nil, false
	}
	return r.tools[i], true
}

func (r *ToolRegistry) All() []Tool {
	return r.tools
}

func (r *ToolRegistry) Definitions() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}
```

- [ ] **Step 4: Add `strings` import to test file if needed, run tests**

```bash
go test ./internal/agent/... -run "TestToolRegistry_PreservesInsertionOrder|TestToolRegistry_GetStillWorks|TestSystemPromptSection_CollectedInOrder" -v
```

Expected: PASS

- [ ] **Step 5: Run full test suite**

```bash
go test ./internal/agent/...
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools.go internal/agent/tools_test.go
git commit -m "refactor: ordered ToolRegistry + SystemPromptSection interface"
```

---

### Task 2: Add `SystemPromptSection()` to `ListDevicesTool`

**Files:**
- Modify: `internal/agent/tools_list_devices.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/tools_device_test.go`:

```go
func TestListDevicesTool_SystemPromptSection(t *testing.T) {
	tool := NewListDevicesTool(nil)
	sp, ok := any(tool).(SystemPromptSection)
	if !ok {
		t.Fatal("ListDevicesTool does not implement SystemPromptSection")
	}
	section := sp.SystemPromptSection()
	if !strings.Contains(section, "ListDevices") && !strings.Contains(section, "ListHosts") {
		t.Errorf("section should mention ListDevices/ListHosts, got: %s", section)
	}
	if strings.TrimSpace(section) == "" {
		t.Error("section must not be empty")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/agent/... -run TestListDevicesTool_SystemPromptSection -v
```

Expected: FAIL

- [ ] **Step 3: Implement**

Add to `internal/agent/tools_list_devices.go` after the `Description()` method:

```go
func (t *ListDevicesTool) SystemPromptSection() string {
	return `### ListHosts / GetDeviceInfo / SearchDocs (read-only, no side effects)

**When to use:** Call these freely at the start of any task to understand the environment.

<example>
User: Check disk usage on all web servers.
Assistant: Calls ListHosts to find web servers before running any commands.
</example>`
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/agent/... -run TestListDevicesTool_SystemPromptSection -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools_list_devices.go internal/agent/tools_device_test.go
git commit -m "feat: ListDevicesTool implements SystemPromptSection"
```

---

### Task 3: Add `SystemPromptSection()` to `VerifyTool`

**Files:**
- Modify: `internal/agent/tools_verify.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/tools_device_test.go`:

```go
func TestVerifyTool_SystemPromptSection(t *testing.T) {
	tool := NewVerifyTool(nil, nil, nil, nil)
	sp, ok := any(tool).(SystemPromptSection)
	if !ok {
		t.Fatal("VerifyTool does not implement SystemPromptSection")
	}
	section := sp.SystemPromptSection()
	if !strings.Contains(section, "VerifyTool") && !strings.Contains(section, "Verify") {
		t.Errorf("section should mention Verify, got: %s", section)
	}
	if !strings.Contains(section, "NOT") {
		t.Errorf("section should contain when-NOT-to-use guidance")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/agent/... -run TestVerifyTool_SystemPromptSection -v
```

Expected: FAIL

- [ ] **Step 3: Implement**

Add to `internal/agent/tools_verify.go` after the `Description()` method:

```go
func (t *VerifyTool) SystemPromptSection() string {
	return `### VerifyTool (read-only, has retry semantics)

**When to use:** After a deployment or config change, to poll until a service is ready.
**When NOT to use:** Don't use for a one-shot check — use RunCommand instead. VerifyTool retries on failure, adding latency when you just need a single result.

<example>
User: Restart nginx and confirm it's up.
Assistant: Calls RunCommand to restart, then VerifyTool to poll until nginx responds.
</example>

<example>
User: Is port 80 open on web-01?
Assistant: Calls RunCommand with "ss -tlnp | grep :80". Does NOT use VerifyTool.
</example>`
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/agent/... -run TestVerifyTool_SystemPromptSection -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools_verify.go internal/agent/tools_device_test.go
git commit -m "feat: VerifyTool implements SystemPromptSection"
```

---

### Task 4: Add `SystemPromptSection()` to `ExecuteCLITool` and `BatchExecuteTool`

**Files:**
- Modify: `internal/agent/tools_cli.go`
- Modify: `internal/agent/tools_batch.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/agent/tools_cli_test.go`:

```go
func TestExecuteCLITool_SystemPromptSection(t *testing.T) {
	tool := NewExecuteCLITool(nil, nil, nil, nil, nil)
	sp, ok := any(tool).(SystemPromptSection)
	if !ok {
		t.Fatal("ExecuteCLITool does not implement SystemPromptSection")
	}
	section := sp.SystemPromptSection()
	if !strings.Contains(section, "Explore") {
		t.Errorf("section should mention Explore phase, got: %s", section)
	}
	if !strings.Contains(section, "Act") {
		t.Errorf("section should mention Act phase, got: %s", section)
	}
}
```

Add to `internal/agent/tools_batch_test.go`:

```go
func TestBatchExecuteTool_SystemPromptSection(t *testing.T) {
	tool := NewBatchExecuteTool(nil, nil, nil, nil, nil)
	sp, ok := any(tool).(SystemPromptSection)
	if !ok {
		t.Fatal("BatchExecuteTool does not implement SystemPromptSection")
	}
	// Must return same content as ExecuteCLITool (shared constant)
	cliTool := NewExecuteCLITool(nil, nil, nil, nil, nil)
	cliSP := cliTool.(SystemPromptSection)
	if sp.SystemPromptSection() != cliSP.SystemPromptSection() {
		t.Error("BatchExecuteTool and ExecuteCLITool must return identical SystemPromptSection")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/agent/... -run "TestExecuteCLITool_SystemPromptSection|TestBatchExecuteTool_SystemPromptSection" -v
```

Expected: FAIL

- [ ] **Step 3: Implement**

Add to `internal/agent/tools_cli.go` after the `Description()` method:

```go
const runCommandPromptSection = `### RunCommand / RunCommandBatch (has side effects)

**When to use:**
- Explore phase: read-only commands (ls, cat, grep, ps, df, systemctl status) — use freely
- Act phase: state-changing commands (rm, kill, systemctl restart, apt, chmod) — only after confirming intent

**When NOT to use:** Do not run state-changing commands before the user has confirmed the plan.

<example>
User: Clean up logs older than 30 days on all app servers.
Assistant: First calls RunCommandBatch with "find /var/log -mtime +30" to preview what would be deleted. Confirms with user. Then runs the delete command.
</example>

<example>
User: Restart the database service.
Assistant: Confirms the target host and service name, then calls RunCommand with "systemctl restart postgresql".
</example>`

func (t *ExecuteCLITool) SystemPromptSection() string {
	return runCommandPromptSection
}
```

Add to `internal/agent/tools_batch.go` after the `Description()` method:

```go
func (t *BatchExecuteTool) SystemPromptSection() string {
	return runCommandPromptSection
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/agent/... -run "TestExecuteCLITool_SystemPromptSection|TestBatchExecuteTool_SystemPromptSection" -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools_cli.go internal/agent/tools_batch.go internal/agent/tools_cli_test.go internal/agent/tools_batch_test.go
git commit -m "feat: ExecuteCLITool and BatchExecuteTool implement SystemPromptSection"
```

---

### Task 5: Add `SystemPromptSection()` to `CallRESTAPITool`

**Files:**
- Modify: `internal/agent/tools_api.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/tools_api_test.go`:

```go
func TestCallRESTAPITool_SystemPromptSection(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
	sp, ok := any(tool).(SystemPromptSection)
	if !ok {
		t.Fatal("CallRESTAPITool does not implement SystemPromptSection")
	}
	section := sp.SystemPromptSection()
	if !strings.Contains(section, "GET") {
		t.Errorf("section should mention GET, got: %s", section)
	}
	if !strings.Contains(section, "POST") {
		t.Errorf("section should mention POST, got: %s", section)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/agent/... -run TestCallRESTAPITool_SystemPromptSection -v
```

Expected: FAIL

- [ ] **Step 3: Implement**

Add to `internal/agent/tools_api.go` after the `Description()` method:

```go
func (t *CallRESTAPITool) SystemPromptSection() string {
	return `### CallAPI (GET: read-only; POST/PUT/DELETE: has side effects)

**When to use:**
- GET: use freely in Explore phase
- POST/PUT/DELETE: only in Act phase after confirming intent

<example>
User: Push a new ACL rule via the firewall API.
Assistant: Shows the request body to the user, confirms, then calls CallAPI with POST.
</example>`
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/agent/... -run TestCallRESTAPITool_SystemPromptSection -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools_api.go internal/agent/tools_api_test.go
git commit -m "feat: CallRESTAPITool implements SystemPromptSection"
```

---

### Task 6: Add `SystemPromptSection()` to `TodoTaskTool`

**Files:**
- Modify: `internal/agent/tools_todo_task.go`

- [ ] **Step 1: Write failing test**

Create `internal/agent/tools_todo_task_test.go`:

```go
package agent

import (
	"strings"
	"testing"
)

func TestTodoTaskTool_SystemPromptSection(t *testing.T) {
	tool := NewTodoTaskTool(nil, nil, "")
	sp, ok := any(tool).(SystemPromptSection)
	if !ok {
		t.Fatal("TodoTaskTool does not implement SystemPromptSection")
	}
	section := sp.SystemPromptSection()
	if !strings.Contains(section, "in_progress") {
		t.Errorf("section should mention in_progress status, got: %s", section)
	}
	if !strings.Contains(section, "completed") {
		t.Errorf("section should mention completed status, got: %s", section)
	}
	if strings.TrimSpace(section) == "" {
		t.Error("section must not be empty")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/agent/... -run TestTodoTaskTool_SystemPromptSection -v
```

Expected: FAIL

- [ ] **Step 3: Implement**

Add to `internal/agent/tools_todo_task.go` after the `Description()` method:

```go
func (t *TodoTaskTool) SystemPromptSection() string {
	return `## Task Management (TodoTask tool)

Use the TodoTask tool proactively to track progress on complex tasks.

**When to use:**
- Task requires 3 or more distinct steps
- User provides multiple tasks to complete

**When NOT to use:**
- Single, straightforward task
- Purely conversational or informational response

**Rules:**
- Mark a task in_progress BEFORE beginning work on it
- Only ONE task in_progress at a time
- Mark completed IMMEDIATELY after finishing — do not batch completions
- Only mark completed when fully done; if blocked, keep in_progress and create a new task describing the blocker

<example>
User: Check disk usage on all web servers, clean up logs older than 30 days, and restart nginx if free space is below 20%.
Assistant: Creates tasks: 1) Check disk usage 2) Clean up logs 3) Restart nginx if space < 20%
</example>

<example>
User: What is the IP address of host web-01?
Assistant: Calls GetDeviceInfo directly. No todo list.
</example>`
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/agent/... -run TestTodoTaskTool_SystemPromptSection -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools_todo_task.go internal/agent/tools_todo_task_test.go
git commit -m "feat: TodoTaskTool implements SystemPromptSection"
```

---

### Task 7: Refactor `BuildSystemPrompt` — wire layers, rename constants, remove old prompts

**Files:**
- Modify: `internal/agent/factory.go`

- [ ] **Step 1: Write failing test**

Add to `internal/agent/agent_test.go` (or a new `factory_test.go`):

```go
func TestBuildSystemPrompt_ContainsToolSections(t *testing.T) {
	hosts := store.NewHostStore(/* use in-memory or nil-safe stub */)
	registry := NewToolRegistry()
	registry.Register(NewListDevicesTool(hosts))
	registry.Register(NewVerifyTool(hosts, nil, nil, nil))

	prompt := BuildSystemPrompt(hosts, registry)

	if !strings.Contains(prompt, "ListHosts") {
		t.Error("prompt should contain ListHosts section from ListDevicesTool")
	}
	if !strings.Contains(prompt, "VerifyTool") {
		t.Error("prompt should contain VerifyTool section")
	}
	if !strings.Contains(prompt, "intent") {
		t.Error("prompt should contain intentFieldPrompt (Layer 3)")
	}
	if !strings.Contains(prompt, "Explore → Plan") {
		t.Error("prompt should contain orchestrationPrompt (Layer 3)")
	}
}

func TestBuildSystemPrompt_SkipsEmptySections(t *testing.T) {
	hosts := store.NewHostStore(nil)
	registry := NewToolRegistry()
	// registryMockTool does not implement SystemPromptSection
	registry.Register(&registryMockTool{name: "no-prompt"})

	prompt := BuildSystemPrompt(hosts, registry)
	// Should not have double blank lines from empty sections
	if strings.Contains(prompt, "\n\n\n") {
		t.Error("prompt should not contain triple newlines from empty sections")
	}
}
```

Note: `BuildSystemPrompt` signature changes to accept `*ToolRegistry` as second parameter.

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/agent/... -run "TestBuildSystemPrompt" -v
```

Expected: FAIL — signature mismatch

- [ ] **Step 3: Implement**

Replace `factory.go` constants and `BuildSystemPrompt` function:

```go
const intentFieldPrompt = `### Intent Field (RunCommand / RunCommandBatch / CallAPI)

Always set the intent field. This field is shown to the user in the UI.

**Rules:**
- Write the goal only — do not include device names (the UI adds those automatically)
- Keep it short: 10 Chinese characters or fewer is ideal

<example>
Good: "重启 nginx 使配置生效"
Good: "清理 30 天前的日志"
Bad: "在 local110 和 local201 上重启 nginx" — device names belong in host_ids, not intent
</example>`

const orchestrationPrompt = `## Complex Multi-Step Tasks

**Explore → Plan → Confirm → Act → Verify**

**Dependency chain:** If a step fails, stop. Report what failed before asking how to continue.

**Conditional branching:** Gather facts in Explore phase first. Pick one path based on data — do not execute branches speculatively.

<example>
User: Optimize the web server response time.
Assistant: Collects CPU, memory, and I/O metrics first. Then picks one optimization path based on the bottleneck — does not apply all optimizations at once.
</example>

**Verification:** After each Act step, verify before marking completed. If verification fails, keep in_progress and offer rollback if available.`

// BuildSystemPrompt assembles the three-layer system prompt.
func BuildSystemPrompt(hosts *store.HostStore, registry *ToolRegistry) string {
	var layer1 string
	allHosts, err := hosts.List("")
	if err != nil || len(allHosts) == 0 {
		layer1 = "You are Spider, an intelligent network operations assistant. No hosts are currently registered."
	} else {
		vendorCount := make(map[string]int)
		for _, h := range allHosts {
			v := h.Vendor
			if v == "" {
				v = "unknown"
			}
			vendorCount[v]++
		}
		var parts []string
		for vendor, count := range vendorCount {
			parts = append(parts, fmt.Sprintf("%s(%d)", vendor, count))
		}
		layer1 = fmt.Sprintf(
			"You are Spider, an intelligent network operations assistant. "+
				"You manage %d network devices: %s. "+
				"Use the available tools to execute CLI commands, verify configurations, "+
				"and answer questions about the network infrastructure.",
			len(allHosts),
			strings.Join(parts, ", "),
		)
	}

	var b strings.Builder
	b.WriteString(layer1)
	for _, tool := range registry.All() {
		if sp, ok := tool.(SystemPromptSection); ok {
			section := sp.SystemPromptSection()
			if strings.TrimSpace(section) != "" {
				b.WriteString("\n\n")
				b.WriteString(section)
			}
		}
	}
	b.WriteString("\n\n")
	b.WriteString(intentFieldPrompt)
	b.WriteString("\n\n")
	b.WriteString(orchestrationPrompt)
	return b.String()
}
```

Remove the old constants `toolBehaviorPrompt`, `todoTaskPrompt`, `complexTaskPrompt`.

- [ ] **Step 4: 更新 `internal/api/chat.go` 的调用点**

`BuildSystemPrompt` 唯一的调用点在 `internal/api/chat.go:152`：

```go
// 修改前
systemPrompt := agent.BuildSystemPrompt(app.HostStore)
a := factory.NewAgent(systemPrompt, id)
```

`registry` 在 `factory.NewAgent` 内部构建，需要将 registry 构建提前。修改 `Factory.NewAgent` 签名，将 registry 构建拆分为独立方法 `Factory.NewRegistry() *ToolRegistry`，然后在 `chat.go` 中：

```go
registry := factory.NewRegistry(id)
systemPrompt := agent.BuildSystemPrompt(app.HostStore, registry)
a := factory.NewAgentWithRegistry(systemPrompt, id, registry)
```

同时在 `factory.go` 中：
- 新增 `NewRegistry(conversationID string) *ToolRegistry` — 提取现有 `NewAgent` 中的 registry 构建逻辑
- 新增 `NewAgentWithRegistry(systemPrompt, conversationID string, registry *ToolRegistry) *Agent` — 接受已构建的 registry
- 保留 `NewAgent` 作为便捷方法（内部调用 `NewRegistry` + `BuildSystemPrompt` + `NewAgentWithRegistry`）

- [ ] **Step 5: 构建**

```bash
go build ./...
```

Expected: 无报错

- [ ] **Step 6: 运行全部测试**

```bash
go test ./internal/agent/... && go test ./internal/api/...
```

Expected: 全部通过

- [ ] **Step 7: Commit**

```bash
git add internal/agent/factory.go internal/api/chat.go
git commit -m "refactor: BuildSystemPrompt three-layer assembly, remove old prompt constants"
```
