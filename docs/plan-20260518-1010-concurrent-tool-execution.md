# Plan: 并发工具执行

**Spec：** `spec-20260518-1000-concurrent-tool-execution.md`  
**日期：** 2026-05-18

---

## 任务列表

### Task 1 — Tool 接口加 `IsConcurrencySafe`
**文件：** `internal/agent/tools.go`

在 `Tool` interface 加一个方法：
```go
IsConcurrencySafe(input map[string]any) bool
```

**验证：** `go build ./internal/agent/...` 报编译错误（所有工具未实现新方法）——符合预期，继续下一步。

---

### Task 2 — 各工具实现 `IsConcurrencySafe`
**文件：** 6 个工具文件

| 文件 | 返回值 |
|------|--------|
| `tools_list_hosts.go` | `true` |
| `tools_topology.go` | `true` |
| `tools_docs.go` | `true` |
| `tools_skill.go` | `true` |
| `tools_cli.go` | `false` |
| `tools_api.go` | `false` |
| `tools_batch.go` | `false` |
| `tools_task.go` | `false` |
| `tools_todo_task.go` | `false` |
| `tools_verify.go` | `false` |

每个工具加一行方法，无逻辑。

**验证：** `go build ./internal/agent/...` 通过。

---

### Task 3 — 提取 `executeOne` 函数
**文件：** `internal/agent/agent.go`

把当前 `for _, tc := range toolCalls` 循环体（单个工具的完整执行逻辑，含 hook、confirm、deny、plan、Execute、result size limit、event emit）提取为：

```go
type toolExecResult struct {
    tc          llm.ToolCall
    result      *ToolResult
    hidden      bool
    hookAction  HookAction  // Deny/Plan/RequireConfirm 的结果
    durationMs  int64
}

func (a *Agent) executeOne(
    ctx context.Context,
    tc llm.ToolCall,
    conversationID string,
    waiter ConfirmWaiter,
    events chan<- Event,
) toolExecResult
```

主循环改为调用 `executeOne`，行为与现在完全一致（串行）。

**验证：** 现有测试通过，`go test ./internal/agent/...`。

---

### Task 4 — 实现 `partitionToolCalls`
**文件：** `internal/agent/agent.go`（或新文件 `internal/agent/tool_dispatch.go`）

```go
type toolBatch struct {
    concurrent bool
    calls      []llm.ToolCall
}

func partitionToolCalls(calls []llm.ToolCall, registry *ToolRegistry) []toolBatch
```

逻辑：连续 safe 工具合并为一个并发批次，unsafe 工具各自单独批次。

**验证：** 单元测试覆盖以下场景：
- 全串行
- 全并发
- 混合（safe/unsafe/safe）
- 未知工具名（fallback false）

---

### Task 5 — 实现 `executeConcurrent`
**文件：** 同 Task 4

```go
func (a *Agent) executeConcurrent(
    ctx context.Context,
    calls []llm.ToolCall,
    conversationID string,
    waiter ConfirmWaiter,
    events chan<- Event,
) []toolExecResult
```

用 `sync.WaitGroup` + semaphore channel（容量 = `SPIDER_MAX_TOOL_CONCURRENCY`，默认 10）并发调用 `executeOne`，结果按输入顺序写入固定大小 slice。

**验证：** `go test ./internal/agent/...` 通过。

---

### Task 6 — 主循环改为按批次执行
**文件：** `internal/agent/agent.go`

把 `for _, tc := range toolCalls { executeOne(...) }` 替换为：

```go
for _, batch := range partitionToolCalls(toolCalls, a.registry) {
    var results []toolExecResult
    if batch.concurrent {
        results = a.executeConcurrent(ctx, batch.calls, ...)
    } else {
        for _, tc := range batch.calls {
            results = append(results, a.executeOne(ctx, tc, ...))
        }
    }
    // 按顺序 apply results → history, pendingToolResults, tcRecords
}
```

**验证：** `go test ./internal/agent/...` 全部通过。

---

### Task 7 — 端到端验证
启动测试服务器，发一条会触发多个 `ListHosts` 调用的消息，确认：
1. 日志中两个工具调用的 `start` 时间戳重叠（并发）
2. 结果顺序正确
3. 串行工具（`RunCommand`）不受影响

---

## 执行顺序

Task 1 → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7

每步完成后运行 `go build` / `go test` 验证，不批量推进。
