# Spec: 并发工具执行

**日期：** 2026-05-18  
**状态：** 已实现 — IsConcurrencySafe()、tool_dispatch.go 并发分区、WaitGroup + semaphore

---

## 背景

当前 `agent.go` 中工具调用完全串行：`for _, tc := range toolCalls { tool.Execute(...) }`。LLM 经常在一个 turn 里返回多个只读工具调用（如同时查询多台主机、同时搜索文档），串行执行浪费时间。

参考：Claude Code 的 `toolOrchestration.ts` 用 `isConcurrencySafe` + `partitionToolCalls` 实现了相同模式。

---

## 目标

- 同一 turn 内，多个并发安全的工具调用并行执行
- 不改变工具接口的语义（`Execute` 签名不变）
- 不影响需要确认（`HookRequireConfirm`）的工具流程

---

## 设计

### 1. Tool 接口新增方法

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    DefaultRiskLevel() RiskLevel
    Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
    IsConcurrencySafe(input map[string]any) bool  // 新增
}
```

**默认实现**（嵌入到现有工具的 base struct，或在 `ToolRegistry` 层 fallback）：返回 `false`。

### 2. 各工具的 IsConcurrencySafe 声明

| 工具 | 返回值 | 理由 |
|------|--------|------|
| `ListHosts` | `true` | 纯读，无副作用 |
| `GetTopology` | `true` | 纯读，无副作用 |
| `SearchDocs` | `true` | 纯读，无副作用 |
| `InvokeSkill` | `true` | 只读取 skill 文件 |
| `TodoTool` (read actions) | `false` | write action 存在，保守处理 |
| `PollUntil` | `false` | 有等待循环，并发无意义 |
| `RunCommand` | `false` | SSH 执行，有副作用 |
| `BatchExecute` | `false` | 批量 SSH，有副作用 |
| `CallRESTAPI` | `false` | HTTP 写操作可能存在 |
| `CreateTask` | `false` | 写数据库 |
| `TodoTool` | `false` | 写操作 |

### 3. 分区逻辑

```go
type toolBatch struct {
    concurrent bool
    calls      []llm.ToolCall
}

func partitionToolCalls(calls []llm.ToolCall, registry *ToolRegistry) []toolBatch {
    var batches []toolBatch
    for _, tc := range calls {
        tool, ok := registry.Get(tc.Name)
        safe := ok && tool.IsConcurrencySafe(tc.Input)
        last := len(batches) - 1
        if safe && last >= 0 && batches[last].concurrent {
            batches[last].calls = append(batches[last].calls, tc)
        } else {
            batches = append(batches, toolBatch{concurrent: safe, calls: []llm.ToolCall{tc}})
        }
    }
    return batches
}
```

示例：`[ListHosts, ListHosts, RunCommand, SearchDocs, SearchDocs]`  
→ `[{concurrent:[ListHosts,ListHosts]}, {serial:[RunCommand]}, {concurrent:[SearchDocs,SearchDocs]}]`

### 4. 并发执行

用 `sync.WaitGroup` + 固定大小结果数组，保证结果顺序与输入一致：

```go
func executeConcurrent(ctx context.Context, calls []llm.ToolCall, ...) []toolExecResult {
    results := make([]toolExecResult, len(calls))
    var wg sync.WaitGroup
    sem := make(chan struct{}, maxConcurrency) // 默认 10
    for i, tc := range calls {
        wg.Add(1)
        go func(i int, tc llm.ToolCall) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            results[i] = executeOne(ctx, tc, ...)
        }(i, tc)
    }
    wg.Wait()
    return results
}
```

### 5. 需要确认的工具

`IsConcurrencySafe` 返回 `true` 的工具均为 L1（只读），不会触发 `HookRequireConfirm`。因此并发批次内不需要处理确认流程。

若未来出现需要确认的并发安全工具，该工具应将 `IsConcurrencySafe` 返回 `false`，走串行路径。

### 6. 并发上限

环境变量 `SPIDER_MAX_TOOL_CONCURRENCY`，默认 `10`。

---

## 实现范围

1. `tools.go`：`Tool` 接口加 `IsConcurrencySafe`
2. `tools_list_hosts.go`、`tools_topology.go`、`tools_docs.go`、`tools_skill.go`：实现返回 `true`
3. 其余工具：实现返回 `false`
4. `agent.go`：提取 `executeOne`，新增 `partitionToolCalls` + `executeConcurrent`，主循环改为按批次执行

---

## 不在范围内

- 工具内部的并发（如 `BatchExecute` 已有自己的并发逻辑）
- 跨 turn 的并发
- 动态并发声明（LLM 在调用时指定）

---

## 验证

- 单元测试：`partitionToolCalls` 分区逻辑
- 集成测试：两个 `ListHosts` 调用并发执行，总耗时 < 串行耗时之和
- 回归：现有工具测试全部通过
