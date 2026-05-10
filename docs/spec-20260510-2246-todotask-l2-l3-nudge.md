# TodoTask L2/L3 Prompt Enhancement Spec

## 背景

TodoTask 工具初版只有 L1（工具描述）层。参考 Claude Code 的 TodoWrite 三层设计，补充 L2（system prompt 行为规范）和 L3（运行时反馈）。

## 三层架构

| 层 | 位置 | 内容 | 进入上下文时机 |
|----|------|------|--------------|
| L1 | `TodoTaskTool.Description()` | 一句话用途 + API 摘要 | 每次工具选择时 |
| L2 | `todoTaskPrompt` 常量注入 `BuildSystemPrompt()` | 使用规范 + 正反例 + reasoning | 每次对话开头一次 |
| L3 | `execCreate` / `execUpdate` 返回内容末尾 | base nudge + conditional nudge | 每次写操作调用后 |

## L2 设计

### 结构

```
When to use     — 3 条触发场景
When NOT to use — 3 条边界
Rules           — 状态机约束（in_progress 唯一性、立即标完成等）
Examples        — 2 正例 + 2 反例，每例带 <reasoning>
```

### reasoning 的作用

`<reasoning>` 块教 LLM **决策模式**，而非记规则。
单看规则 LLM 会过拟合字面；reasoning 教"为什么选/不选"的元判断，效果显著优于纯规则列表。

正反例数量：各 2-4 个，必须对称。

## L3 设计

### base nudge

每次 create / update 成功后，在 JSON 返回内容后追加：

```
Todo list updated. Continue using the TodoTask tool to track remaining work —
mark each task in_progress before starting and completed immediately when done.
```

**目的**：对抗模型遗忘。LLM 在长对话中容易忘记继续维护 todo list，base nudge 形成自驱动循环。

### conditional nudge（loop-exit 时刻）

触发条件（全部满足）：
1. 当前操作是 `update`（不是 create）
2. 操作完成后，该对话所有任务均为 `completed` 或 `deleted`
3. 任务总数 ≥ 1

触发时替换 base nudge，内容：

```
All tasks are complete. Before finishing, verify your work by producing a concrete
artifact (test output, build log, or command result) that confirms the changes are
correct. Do not self-assess — let the output speak.
```

**设计原则**：不要求 LLM 自检（自我评分偏高），要求产出可验证 artifact。
这是 Claude Code verifier subagent 的弱版本——spider.ai 暂无 subagent 机制，用"强制产出 artifact"代替"外包判决"。

### 与 Claude Code 的差异

| 机制 | Claude Code | spider.ai |
|------|-------------|-----------|
| 触发条件 | 5 个（含 feature flag、agentId 检查、growthbook） | 2 个（allDone + 任务数） |
| 验证方式 | spawn VERIFICATION_AGENT subagent | 要求产出 artifact |
| 灰度控制 | growthbook flag | 无（全量） |

## 实现

- `internal/agent/factory.go` — `todoTaskPrompt` 常量，注入 `BuildSystemPrompt()`
- `internal/agent/tools_todo_task.go` — `todoNudge(allDone bool)`、`allTasksDone()`，在 `execCreate` / `execUpdate` 返回时追加

## 后续

- L3 conditional nudge 强版本：设计 verifier subagent（需要 spider.ai 支持 subagent 机制后再做）
- L2 可补充更多 reasoning 示例（当前 2+2，建议最终 3+3）
