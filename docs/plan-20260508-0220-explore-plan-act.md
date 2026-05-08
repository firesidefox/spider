# Explore-Plan-Act Agent 行为约束 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过系统提示前缀和工具描述语义标注，让 agent 自然遵循 Explore → Plan → Act 顺序。

**Architecture:** 纯 prompt 层改动。`NewAgent` 在用户传入的 `SystemPrompt` 前拼接固定 EPA 指令块；各工具的 `Description()` 方法加入副作用说明，让 LLM 自己判断阶段归属。`Run()` 结构不变，权限钩子不变。

**Tech Stack:** Go, 无新依赖

---

### Task 1: NewAgent 拼接 EPA 系统提示前缀

**Files:**
- Modify: `internal/agent/agent.go:65-78`
- Test: `internal/agent/agent_test.go`

- [ ] **Step 1: 写失败测试**

在 `agent_test.go` 末尾添加：

```go
func TestNewAgentPrependsEPAPrefix(t *testing.T) {
	cfg := AgentConfig{
		LLMClient:    nil,
		Registry:     NewToolRegistry(),
		Hooks:        NewHookChain(),
		MsgStore:     nil,
		SystemPrompt: "你是运维助手。",
	}
	a := NewAgent(cfg)
	if !strings.HasPrefix(a.systemPrompt, "## 行为约束") {
		t.Errorf("systemPrompt should start with EPA prefix, got: %q", a.systemPrompt[:min(50, len(a.systemPrompt))])
	}
	if !strings.Contains(a.systemPrompt, "你是运维助手。") {
		t.Error("systemPrompt should contain original prompt")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/ -run TestNewAgentPrependsEPAPrefix -v
```

期望：FAIL — `systemPrompt should start with EPA prefix`

- [ ] **Step 3: 在 agent.go 添加 EPA 常量并修改 NewAgent**

在 `agent.go` 的 `import` 块之后、`EventType` 定义之前插入常量：

```go
const epaSystemPromptPrefix = `## 行为约束

按以下顺序处理任务：

Explore：先用只读工具收集信息，在没有充分了解现状之前不执行有副作用的操作。
Plan：基于探索结果，在内部推理出完整执行步骤，明确每步目的和预期结果。
Act：按计划执行，每步完成后验证结果再继续；遇到意外重新进入 Explore，不盲目继续。

`
```

修改 `NewAgent`（L65-78），将 `systemPrompt` 字段赋值改为：

```go
func NewAgent(cfg AgentConfig) *Agent {
	maxTurns := cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = 10
	}
	return &Agent{
		llmClient:    cfg.LLMClient,
		registry:     cfg.Registry,
		hooks:        cfg.Hooks,
		msgStore:     cfg.MsgStore,
		systemPrompt: epaSystemPromptPrefix + cfg.SystemPrompt,
		maxTurns:     maxTurns,
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/agent/ -run TestNewAgentPrependsEPAPrefix -v
```

期望：PASS

- [ ] **Step 5: 运行全量测试确认无回归**

```bash
go test ./internal/agent/ -v
```

期望：全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/agent.go internal/agent/agent_test.go
git commit -m "feat(agent): prepend EPA system prompt prefix in NewAgent"
```

---

### Task 2: 只读工具描述加 Explore 语义

涉及工具：`ListDevicesTool`、`GetDeviceInfoTool`、`SearchDocsTool`、`VerifyTool`

**Files:**
- Modify: `internal/agent/tools_list_devices.go:20`
- Modify: `internal/agent/tools_device.go:22-24`
- Modify: `internal/agent/tools_docs.go:22-24`
- Modify: `internal/agent/tools_verify.go:26`

- [ ] **Step 1: 写失败测试**

在 `agent_test.go` 末尾添加：

```go
func TestReadOnlyToolDescriptionsContainExploreHint(t *testing.T) {
	tools := []Tool{
		NewListDevicesTool(nil),
		NewGetDeviceInfoTool(nil),
		NewSearchDocsTool(nil),
		NewVerifyTool(nil, nil, nil),
	}
	for _, tool := range tools {
		desc := tool.Description()
		if !strings.Contains(desc, "Read-only") {
			t.Errorf("tool %q description should contain 'Read-only', got: %q", tool.Name(), desc)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/agent/ -run TestReadOnlyToolDescriptionsContainExploreHint -v
```

期望：FAIL — 4 个工具均不含 `Read-only`

- [ ] **Step 3: 更新 ListDevicesTool.Description**

`tools_list_devices.go:20`：

```go
func (t *ListDevicesTool) Description() string {
	return "List all managed devices, optionally filtered by tag. Read-only. No side effects. Use freely in Explore phase."
}
```

- [ ] **Step 4: 更新 GetDeviceInfoTool.Description**

`tools_device.go:22-24`：

```go
func (t *GetDeviceInfoTool) Description() string {
	return "Get device information by host ID or name. Read-only. No side effects. Use freely in Explore phase."
}
```

- [ ] **Step 5: 更新 SearchDocsTool.Description**

`tools_docs.go:22-24`：

```go
func (t *SearchDocsTool) Description() string {
	return "Search documentation for CLI commands, API references, and troubleshooting guides. Read-only. No side effects. Use freely in Explore phase."
}
```

- [ ] **Step 6: 更新 VerifyTool.Description**

`tools_verify.go:26`：

```go
func (t *VerifyTool) Description() string {
	return "Verify conditions on remote hosts with retry polling. Read-only. No side effects. Use freely in Explore phase."
}
```

- [ ] **Step 7: 运行测试确认通过**

```bash
go test ./internal/agent/ -run TestReadOnlyToolDescriptionsContainExploreHint -v
```

期望：PASS

- [ ] **Step 8: 运行全量测试确认无回归**

```bash
go test ./internal/agent/ -v
```

期望：全部 PASS

- [ ] **Step 9: Commit**

```bash
git add internal/agent/tools_list_devices.go internal/agent/tools_device.go internal/agent/tools_docs.go internal/agent/tools_verify.go internal/agent/agent_test.go
git commit -m "feat(agent): add Explore-phase hint to read-only tool descriptions"
```

---

### Task 3: 执行类工具描述加 Act 语义

涉及工具：`ExecuteCLITool`、`BatchExecuteTool`、`CallRESTAPITool`

**Files:**
- Modify: `internal/agent/tools_cli.go:28-30`
- Modify: `internal/agent/tools_batch.go:27`
- Modify: `internal/agent/tools_api.go:26-28`

- [ ] **Step 1: 写失败测试**

在 `agent_test.go` 末尾添加：

```go
func TestActToolDescriptionsContainSideEffectHint(t *testing.T) {
	tools := []Tool{
		NewExecuteCLITool(nil, nil, nil, nil),
		NewBatchExecuteTool(nil, nil, nil, nil),
		NewCallRESTAPITool(),
	}
	for _, tool := range tools {
		desc := tool.Description()
		if !strings.Contains(desc, "side effects") {
			t.Errorf("tool %q description should contain 'side effects', got: %q", tool.Name(), desc)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/agent/ -run TestActToolDescriptionsContainSideEffectHint -v
```

期望：FAIL — 3 个工具均不含 `side effects`

- [ ] **Step 3: 更新 ExecuteCLITool.Description**

`tools_cli.go:28-30`：

```go
func (t *ExecuteCLITool) Description() string {
	return `Execute a CLI command on a remote host via SSH. Has side effects. Use only after confirming intent in Plan phase.
Risk depends on the command:
- Read-only commands (ls, cat, grep, ps, df, free, uname, systemctl status): safe, can use in Explore phase
- State-changing commands (rm, kill, systemctl start|stop|restart, apt, yum, chmod, chown): use only in Act phase`
}
```

- [ ] **Step 4: 更新 BatchExecuteTool.Description**

`tools_batch.go:27`：

```go
func (t *BatchExecuteTool) Description() string {
	return `Execute a CLI command on multiple hosts in parallel. Has side effects. Use only after confirming intent in Plan phase.
Risk depends on the command:
- Read-only commands (ls, cat, grep, ps, df, free, uname, systemctl status): safe, can use in Explore phase
- State-changing commands (rm, kill, systemctl start|stop|restart, apt, yum, chmod, chown): use only in Act phase`
}
```

- [ ] **Step 5: 更新 CallRESTAPITool.Description**

`tools_api.go:26-28`：

```go
func (t *CallRESTAPITool) Description() string {
	return "Call a REST API endpoint on a gateway device. Has side effects for POST/PUT/DELETE methods. Use GET freely in Explore phase; use mutating methods only in Act phase after confirming intent."
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./internal/agent/ -run TestActToolDescriptionsContainSideEffectHint -v
```

期望：PASS

- [ ] **Step 7: 运行全量测试确认无回归**

```bash
go test ./internal/agent/ -v
```

期望：全部 PASS

- [ ] **Step 8: Commit**

```bash
git add internal/agent/tools_cli.go internal/agent/tools_batch.go internal/agent/tools_api.go internal/agent/agent_test.go
git commit -m "feat(agent): add Act-phase hint to side-effect tool descriptions"
```
