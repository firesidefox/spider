# CLAUDE.md

## 1. 文件写入规范

- 需要写或者修改文件时，**单次写入操作不得超过 50 行**。
- 若内容超过 50 行，应分批写入。

## 2. Claude Code 的源码位置：/Users/cw/fty.ai/claude-code-source-codes

## 3. 前端调试方法

### Go embed 增量构建陷阱
`go build` 不一定重新嵌入 `//go:embed` 的静态资源（Go 的增量编译不追踪嵌入文件变化）。
修改前端后必须用 `go build -a` 强制全量重建，否则服务器仍返回旧的 `index.html`。

验证方法：`curl -s http://localhost:PORT/ | grep "index-"` 确认 HTML 里引用的 JS 文件名与 `dist/assets/` 一致。

### 测试新二进制时避免端口冲突
用 `--data-dir` 指定独立数据目录，用不同端口启动：
```
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :PORT --data-dir /tmp/spider-test-data
```
若要复用已有配置（provider、用户），指向同一数据目录（如 `~/.spider`）。

### Playwright 测试多 tab 同步
1. Tab A 发消息，等待响应完成
2. Tab B（同一对话 URL）检查是否收到广播
3. 用 `page.evaluate(() => Array.from(document.querySelectorAll('p')).map(p => p.textContent))` 验证消息列表

### EventSource 调试
- EventSource 不能发自定义 header，需要 cookie auth
- 被动 tab（没有发消息）必须在 `selectConversation` 时打开 EventSource，不能只在 `send()` 里开
- `text_delta` 到达时若最后一条消息不是 streaming 状态，需创建新消息块，否则 delta 会追加到上一条

## 4. Agent 工具提示词规范

### 设计原则

工具提示词分两层，不要混写：

| 层 | 位置 | 内容 | 原则 |
|----|------|------|------|
| `Description()` | 工具定义，每次 tool call 都发送给 LLM | API 契约：用途、副作用、阶段约束 | **极简**，一到两句话 |
| System prompt | `BuildSystemPrompt()` 注入，每次对话开头发送一次 | 行为规范：何时用、何时不用、状态机、示例 | 可以详细 |

### `Description()` 写法

只写三件事：
1. **这个工具做什么**（一句话）
2. **副作用声明**：Read-only / Has side effects
3. **阶段约束**（如适用）：Use freely in Explore phase / Use only in Act phase

```go
// 好
"List all managed devices, optionally filtered by tag. Read-only. No side effects. Use freely in Explore phase."

// 好
"Execute a CLI command on a remote host via SSH. Has side effects. Use only after confirming intent in Plan phase."

// 坏 — 把使用规范塞进 description
"Manage the todo task list.\n\nActions:\n- create: ...\n- update: ...\nStatus values: pending, in_progress..."
```

### 行为规范放 system prompt

需要 LLM 理解"何时调用"、"如何决策"的规范，放进 `BuildSystemPrompt()` 里的常量，不要放 `Description()`。

参考 `todoTaskPrompt` 常量的写法：
- 用 `**When to use:**` / `**When NOT to use:**` 明确边界
- 用 `**Rules:**` 列状态机约束
- 反例比正例更有效（LLM 更容易从反例校准边界）

### 风险分级例外

`risk_level` 相关的说明（哪些命令是 L1/L2/L3）可以留在 `Description()` 里，因为 LLM 在决定调用参数时需要实时参考，不适合放 system prompt。

## 5. Goal-Driven Execution

**4. Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.
