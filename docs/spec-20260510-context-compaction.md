# Spec: 会话上下文压缩（Context Compaction）

**日期：** 2026-05-10  
**状态：** 草稿

---

## 背景

当前 `Agent.Run()` 每次从 DB 取全量消息历史，直接发给 LLM。对话越长，token 消耗越多，最终会撞上模型 context window 上限导致请求失败。

目前无任何截断、摘要或压缩机制。

---

## 目标

- 对话超过阈值时，自动将旧消息压缩为摘要，保留近期原文
- 摘要缓存到 DB，避免重复生成
- 正常对话（未超限）零开销
- compaction 始终启用，超限必压缩（不压缩则 API 报错）

---

## 附加改动

### EPA 行为约束英文化

`internal/agent/agent.go` 中 `epaSystemPromptPrefix` 改为英文：

```go
const epaSystemPromptPrefix = `## Behavioral Constraints

Process tasks in the following order:

Explore: Use read-only tools to gather information first. Do not perform any side-effecting operations until you have a clear understanding of the current state.
Plan: Based on exploration results, reason through a complete execution plan internally. Clarify the purpose and expected outcome of each step.
Act: Execute the plan step by step, verifying results after each step before continuing. If anything unexpected occurs, re-enter Explore — do not proceed blindly.

`
```

---

## 非目标

- 不做流式摘要（摘要在 Run() 开始前同步完成）
- 不支持多级摘要（摘要的摘要）
- 不引入外部 tokenizer，用字符数近似估算

---

## 方案

### 压缩触发流程

边界固定原则：摘要边界只往前推进，不浮动。每次以上次摘要的 `up_to_message_id` 为起点取后续消息，避免重叠或断裂。

**追加式摘要**：每次只对新增的原始消息生成摘要片段，旧摘要片段原封不动保留，不重压。

```
Agent.Run() 开始时：

1. 查 DB 是否有该会话的摘要缓存 → 得到 (chunks[], boundary_id)
2. 取 boundary_id 之后的所有消息（无缓存则取全量）
3. 估算 messages token 数（只算 messages 层，不算 system）
4. if token < threshold → history = [摘要对] + [boundary 后消息]，结束
5. if token >= threshold：
   a. 新边界 = boundary 后消息中，倒数第 recent_turns 轮的起点 message_id
   b. 待压缩消息 = boundary_id 到新边界之间的消息
   c. 调 LLM 生成新摘要片段 new_delta = summarize(待压缩消息)
   d. chunks.append(new_delta)
   e. if sum(tokens(chunks)) > max_summary_tokens：
      把所有 chunks 整体压缩为一个新片段（见"整体压缩 prompt"）
      chunks = [compressed]
      注：chunks 是纯字符串，token 计数统一用 `estimateTokens(string)` 字符分段估算，不走 LLM API。
   f. 存 DB（新边界 + chunks）
   g. history = [摘要对] + [新边界之后的近 recent_turns 轮]

LLM 请求发出后，如果 API 返回 context length 超限错误：
   a. 强制触发一次压缩（忽略 threshold，直接执行步骤 5）
      - 若 boundary 后消息不足 recent_turns 轮（无法推进边界）→ 直接返回错误，不重试
   b. 用压缩后的 history 重试请求
   c. 重试仍失败 → 返回错误，不再重试
```

**边界推进示意：**

```
初始：消息 1-100，threshold 触发
  → 新边界 = 消息 80
  → S1 = summarize(消息1-80)
  → chunks = [S1]，boundary = msg_80
  → history = [S1 注入] + [消息 81-100]

继续对话：消息 101-110，threshold 再次触发
  → 从缓存取 chunks=[S1]，boundary=msg_80
  → boundary 后消息 = 81-110，超限
  → 新边界 = 消息 90
  → S2 = summarize(消息81-90)
  → chunks = [S1, S2]，boundary = msg_90
  → history = [S1+S2 注入] + [消息 91-110]

chunks 超过 max_summary_tokens：
  → compressed = consolidate(S1, S2, S3, ...)
  → chunks = [compressed]
```

### 轮次定义

一轮 = 一条 `role=user` 消息 + 若干 `role=tool` 消息 + 一条 `role=assistant` 消息。

**轮次计数算法**：从消息列表尾部往前扫，遇到 `role=user` 则轮次计数 +1，直到计到 `recent_turns` 轮。该 user 消息的 message_id 即为新边界。

```
msgs = [u1, a1, u2, a2, u3, a3, u4, a4]   // 简化，忽略 tool 消息
recent_turns = 2
→ 从尾部数 2 个 user：u4、u3
→ 新边界 = u3 的前一条消息（a2）的 message_id
→ 待压缩 = u1..a2，保留原文 = u3..a4
```

注入的摘要 user+assistant 对**不计入轮次**。按轮次截断保证 LLM 不会看到残缺的 tool 调用序列。

### Token 估算

按 provider 选策略，**只估算 messages 层**，不计算 system prompt（system 层内容固定，token 数恒定）。

**Claude / Anthropic：** 调用 `/v1/messages/count_tokens` API，精确计数。

**OpenAI 及其他：** 字符分段估算，零依赖，无需词表文件：

```go
func estimateTokens(s string) int {
    var cjk, ascii int
    for _, r := range s {
        // r > 0x2E80 覆盖 CJK 及 Hangul/Kana 等东亚字符，约 1 token/字
        // 其他 Unicode（阿拉伯、西里尔等）归入 ascii 路径，误差较大但可接受
        if r > 0x2E80 {
            cjk++
        } else {
            ascii++
        }
    }
    return cjk + ascii/4
}
```

CJK 字符约 1 token/字，ASCII 约 4 字符/token。误差：中文 ±5%，英文 ±15%，混合 ±10%。

阈值比例 `× 0.5` 给误差留足 buffer，内网部署无联网依赖。

`llm.Client` 接口新增 `CountTokens`：

```go
type Client interface {
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
    Chat(ctx context.Context, req *ChatRequest) (string, error)
    CountTokens(ctx context.Context, msgs []Message) (int, error)
}
```

- `ClaudeClient.CountTokens` → 调 Anthropic count tokens API
- `OpenAIClient.CountTokens` → 字符分段估算，返回 `nil` error

### 阈值计算

```
if config.ThresholdTokens > 0:
    threshold = config.ThresholdTokens          // 用户手动配置
else:
    threshold = knownContextWindows[model] * 0.5 // 内置模型表
    fallback  = 120_000                          // 未知模型
```

内置模型表（`internal/llm/models.go`）：

| 模型 | Context Window |
|------|---------------|
| claude-sonnet-4-6 | 1,000,000 |
| claude-opus-4-7 | 1,000,000 |
| claude-haiku-4-5 | 200,000 |
| gpt-4o | 128,000 |
| gpt-4o-mini | 128,000 |

---

## 配置

`CompactionConfig` 加入 `internal/config/config.go` 的 `AgentConfig`（YAML 配置结构体，非 `internal/agent/agent.go` 的同名运行时结构体）：

```go
type CompactionConfig struct {
    ThresholdTokens  int `yaml:"threshold_tokens"`   // 0 = 自动用模型表
    RecentTurns      int `yaml:"recent_turns"`        // 默认 20
    MaxSummaryTokens int `yaml:"max_summary_tokens"`  // 摘要片段总上限，默认 4000
}
```

`config.yaml` 示例：

```yaml
agent:
  compaction:
    threshold_tokens: 0
    recent_turns: 20
    max_summary_tokens: 4000
```

默认值：`threshold_tokens: 0`，`recent_turns: 20`，`max_summary_tokens: 4000`。compaction 始终启用，无开关。

---

## 数据库

新增表 `conversation_summaries`：

```sql
CREATE TABLE IF NOT EXISTS conversation_summaries (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id  TEXT NOT NULL,
    up_to_message_id TEXT NOT NULL,
    chunks           TEXT NOT NULL,  -- JSON 数组，每个元素是一个摘要片段
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id)
);
```

每个会话只保留一条记录（`UNIQUE(conversation_id)`）。  
`chunks` 存储所有摘要片段的 JSON 数组，按时间顺序排列。  
边界推进时用 SQLite upsert 更新，`updated_at` 记录最后一次边界推进时间，`created_at` 记录首次生成时间：

```sql
INSERT INTO conversation_summaries (conversation_id, up_to_message_id, chunks)
VALUES (?, ?, ?)
ON CONFLICT(conversation_id) DO UPDATE SET
    up_to_message_id = excluded.up_to_message_id,
    chunks           = excluded.chunks,
    updated_at       = CURRENT_TIMESTAMP;
```

---

## 摘要 Prompt

### 片段压缩 prompt

每次对新增原始消息生成摘要片段：

```
The following is a segment of a network device management conversation.
Generate a concise summary. You MUST preserve:
- The user's goal and intent
- Commands executed and their key results
- Device states, issues, and anomalies discovered
- Incomplete tasks

Ignore: small talk, repeated confirmations, intermediate reasoning.

Conversation segment:
{messages}
```

### 整体压缩 prompt

chunks 超过 `max_summary_tokens` 时，把所有片段整体压缩为一个新片段：

```
The following are multiple historical summary segments.
First, identify the user's current core objective. Then compress all content into a single new summary.

You MUST preserve:
- Current objective (synthesized from all segments, in one sentence)
- Any long-term constraints or rules the user has explicitly stated
  (e.g. "always use IPv6", "do not reboot devices", "only operate on VLAN 100")
- Critical device states and anomalies
- Incomplete tasks

You MAY omit:
- Completed operation details whose results are already reflected in device states

Historical summaries:
{chunks}
```

整体压缩后 chunks 重置为一个片段，不会无限膨胀。整体压缩是低频操作，只在超限时触发。

摘要以 user+assistant 对注入，作为 history 的第一轮：

```
[user:      "The following is a summary of the previous conversation, for reference."]
[assistant: "{summary content}"]
[user:      first recent original message]
...
```

理由：用 user role 单独注入摘要内容，LLM 可能误解为当前用户指令（如"设备配置已更新"会被当成新指令）。用 assistant role 承载摘要内容，语义上是"我之前做过这些事"，不会混淆。user+assistant 对也满足两个 provider 的消息交替格式要求。

---

## 接口变更

### `internal/llm/client.go`

新增 `Chat()` 和 `CountTokens()`：

```go
type Client interface {
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
    Chat(ctx context.Context, req *ChatRequest) (string, error)
    CountTokens(ctx context.Context, msgs []Message) (int, error)
}
```

### `internal/agent/agent.go`（MessageStorer 接口扩展）

`MessageStorer` 接口定义在 `internal/agent/agent.go`，新增 `ListAfterMessage`：

```go
type MessageStorer interface {
    Save(conversationID, role, content, toolCalls string) error
    ListByConversation(conversationID string) ([]*models.Message, error)
    // 新增
    ListAfterMessage(conversationID, messageID string) ([]*models.Message, error)
}
```

实现在 `internal/store/message.go` 的 `MessageStore` struct 上。

`ListAfterMessage`：取指定 message_id 之后的所有消息（messageID 为空则取全量）。  
不再需要 `ListRecentTurns`，轮次截断在 compactor 内存中完成。

`messages` 表新增复合索引，覆盖所有按会话查消息的场景：

```sql
CREATE INDEX IF NOT EXISTS idx_messages_conv_created
ON messages(conversation_id, created_at);
```

对现有 `ListByConversation` 同样有效，无需改查询语句。

---

## 文件改动清单

| 文件 | 类型 | 说明 |
|------|------|------|
| `internal/config/config.go` | 修改 | `AgentConfig` 加 `Compaction CompactionConfig`，`DefaultConfig()` 加默认值 |
| `internal/llm/models.go` | 修改 | 新增 `knownContextWindows` 表和 `DefaultThreshold(model)` |
| `internal/llm/client.go` | 修改 | `Client` 接口新增 `Chat()`、`CountTokens()` |
| `internal/llm/claude.go` | 修改 | 实现 `Chat()`、`CountTokens()`（调 Anthropic count tokens API）|
| `internal/llm/openai.go` | 修改 | 实现 `Chat()`、`CountTokens()`（字符分段估算）|
| `internal/store/message.go` | 修改 | 实现 `ListAfterMessage` |
| `internal/agent/agent.go` | 修改 | `epaSystemPromptPrefix` 英文化，`MessageStorer` 加 `ListAfterMessage`，`AgentConfig` 加 `Compactor`，`Run()` 调用 |
| `internal/agent/agent_test.go` | 修改 | `mockLLMClient` 补 `Chat()`/`CountTokens()`，`mockMsgStore` 补 `ListAfterMessage` |
| `internal/db/schema.go` | 修改 | 新增 `idx_messages_conv_created` 复合索引，新增 `conversation_summaries` 表 |
| `internal/store/summary.go` | 新建 | `SummaryStore` CRUD |
| `internal/agent/compactor.go` | 新建 | 压缩主逻辑 `Compactor` |
| `internal/agent/factory.go` | 修改 | 构造 `Compactor`，传入 `Agent` |

---

## 关键设计决策

1. **不做全量替换**：保留近期 `recent_turns` 轮原文，只压缩旧消息。网络设备操作场景近期上下文关键，全量压缩风险高。

2. **摘要缓存**：同一边界只生成一次摘要，后续复用。边界推进才重新生成。

3. **分步取消息**：先取近期轮次估算，未超限不取旧消息。正常对话 DB 查询量大幅减少。

4. **始终启用**：compaction 是保护机制，不提供开关。超限不压缩则 API 直接报错，没有关闭的理由。

5. **同一 LLM 做摘要**：不引入额外 provider 配置，用 `Chat()` 非流式调用。

---

## 验收标准

- [ ] 短对话（未超限）：不触发摘要，history 为 boundary 后全量消息
- [ ] 有缓存且 boundary 后消息未超限：直接复用摘要，不调用 LLM，边界不变
- [ ] 长对话（超限，无缓存）：生成摘要，存 DB，后续复用
- [ ] 边界推进：新摘要包含旧摘要内容 + 新增旧消息，缓存覆盖更新
- [ ] 摘要以 user+assistant 对注入 history 首位
- [ ] `threshold_tokens > 0` 时忽略模型表，使用配置值
- [ ] 未知模型 fallback 到 120,000

## 测试项

### 边界推进验证

构造 50 轮对话，设低阈值触发压缩：

- DB 中 `up_to_message_id` 正确更新为新边界
- `chunks` 为合法 JSON 数组
- `chunks` 追加了新片段，旧片段原封不动
- history 中近期原文消息从新边界之后开始，无重叠或断裂

### 整体压缩触发验证

设 `max_summary_tokens: 500`，多次追加片段直到超限：

- 触发整体压缩后，DB 中 `chunks` 长度变为 1
- `chunks[0]` 非空
- `chunks[0]` token 数 < `max_summary_tokens`
