# CLAUDE.md

## 前端调试方法

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

## Agent 工具提示词规范

工具提示词分两层：

| 层 | 位置 | 内容 |
|----|------|------|
| `Description()` | 工具定义 | 用途、副作用、阶段约束（极简，1-2 句） |
| System prompt | `BuildSystemPrompt()` | 何时用、何时不用、状态机、示例 |

**Description() 写法：**
1. 这个工具做什么（一句话）
2. 副作用声明：Read-only / Has side effects
3. 阶段约束：Use freely in Explore phase / Use only in Act phase

**行为规范放 system prompt：**
- 用 `**When to use:**` / `**When NOT to use:**` 明确边界
- 用 `**Rules:**` 列状态机约束
- 反例比正例更有效
