# Agent 工具提示词三层规范补全

## 背景

TodoTaskTool 已实现完整三层架构（L1/L2/L3）。其余 7 个工具只有 L1，缺少 L2 和 L3。

## 现状

| 工具 | L1 | L2 | L3 |
|------|----|----|-----|
| ListDevicesTool | ✅ | ❌ | ❌ |
| GetDeviceInfoTool | ✅ | ❌ | ❌ |
| ExecuteCLITool | ✅ | ❌ | ❌ |
| BatchExecuteTool | ✅ | ❌ | ❌ |
| VerifyTool | ✅ | ❌ | ❌ |
| CallRESTAPITool | ✅ | ❌ | ❌ |
| SearchDocsTool | ✅ | ❌ | ❌ |
| TodoTaskTool | ✅ | ✅ | ✅ |

## L2 设计（system prompt 补全）

在 `factory.go` 的 `todoTaskPrompt` 常量后追加各工具的行为规范。

### L2 原则

- 只写 read-only 工具的"何时用"（它们没有副作用，不需要"何时不用"）
- 写操作工具（ExecuteCLI、BatchExecute、CallRESTAPI）需要写"何时用 / 何时不用 / 确认流程"
- 每个工具 2-4 个 reasoning 示例（正反对称）

### 各工具 L2 内容

**ListDevicesTool / GetDeviceInfoTool / SearchDocsTool**（纯 read-only，Explore 阶段自由使用）
- When to use：开始任何任务前先调用，了解可用主机和设备信息
- 不需要 When NOT to use（无副作用）
- 1-2 个 reasoning 示例即可

**VerifyTool**（read-only，但有 retry 语义）
- When to use：部署后验证服务就绪、配置变更后轮询确认
- When NOT to use：不要用来替代 ExecuteCLI 做一次性检查（VerifyTool 有 retry 开销）
- 2 个 reasoning 示例

**ExecuteCLITool / BatchExecuteTool**（有副作用，最重要）
- When to use：Explore 阶段用 read-only 命令；Act 阶段用 state-changing 命令
- When NOT to use：不要在未确认意图前执行 state-changing 命令
- 风险分级规则（已在 L1 里，L2 补充决策流程）
- 3-4 个 reasoning 示例（含反例）

**CallRESTAPITool**
- When to use：GET 在 Explore 阶段自由使用；POST/PUT/DELETE 仅在 Act 阶段
- When NOT to use：不要用 REST API 替代 ExecuteCLI 做只读查询（除非设备只支持 API）
- 2 个 reasoning 示例

## L3 设计（运行时反馈）

### 原则

- **Read-only 工具不加 L3**：ListDevices、GetDeviceInfo、SearchDocs、VerifyTool 返回数据，不需要行为引导
- **写操作工具加 L3**：ExecuteCLI、BatchExecute、CallRESTAPI 执行后追加 nudge

### ExecuteCLITool / BatchExecuteTool nudge

```
Command executed. Update your todo list if this completes a task, then verify
the result before proceeding to the next step.
```

conditional nudge（命令失败时，IsError=true 不追加，已有错误信息）：无需额外 conditional，失败路径已有明确错误。

### CallRESTAPITool nudge

```
API call completed. Check status_code in the response. Update your todo list
if this completes a task.
```

### 与 TodoTaskTool 的差异

ExecuteCLI/BatchExecute/CallRESTAPI 的 nudge 主动引导"更新 todo list"，形成工具间协同闭环：执行工具完成后提醒维护任务状态。

## 实现位置

- `internal/agent/factory.go` — 追加各工具的 L2 常量，注入 `BuildSystemPrompt()`
- `internal/agent/tools_cli.go` — `Execute()` 成功路径追加 nudge
- `internal/agent/tools_batch.go` — 同上
- `internal/agent/tools_api.go` — 同上

## 不需要修改

- `tools_list_devices.go` — read-only，无 L3
- `tools_device.go` — read-only，无 L3
- `tools_docs.go` — read-only，无 L3
- `tools_verify.go` — read-only，无 L3
