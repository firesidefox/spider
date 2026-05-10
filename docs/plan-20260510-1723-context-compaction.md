# Plan: 会话上下文压缩（Context Compaction）

**日期：** 2026-05-10  
**Spec：** `docs/spec-20260510-context-compaction.md`

---

## 当前状态（Phase 0 发现）

| 文件 | 关键事实 |
|------|---------|
| `internal/llm/client.go:45` | `Client` 接口只有 `ChatStream`，需加 `Chat` + `CountTokens` |
| `internal/agent/agent.go:52` | `MessageStorer` 接口只有 `Save` + `ListByConversation`，需加 `ListAfterMessage` |
| `internal/agent/agent.go:16` | `epaSystemPromptPrefix` 当前为中文 |
| `internal/agent/agent.go:66` | `agent.AgentConfig` 需加 `Compactor *Compactor` |
| `internal/config/config.go:36` | `config.AgentConfig` 需加 `Compaction CompactionConfig` |
| `internal/config/config.go:62` | `DefaultConfig()` 需加 compaction 默认值 |
| `internal/store/message.go` | `MessageStore` 需实现 `ListAfterMessage` |
| `internal/db/schema.go` | 需加 `conversation_summaries` 表 + `idx_messages_conv_created` 索引 |
| `internal/agent/agent_test.go:13` | `mockLLMClient` 只有 `ChatStream`，需补 `Chat` + `CountTokens` |
| `internal/agent/agent_test.go:28` | `mockMsgStore` 只有 `Save` + `ListByConversation`，需补 `ListAfterMessage` |
| `go.mod` | Go 1.23，无 tiktoken 依赖（字符估算，无需新依赖） |

---

## Phase 1：LLM 层扩展

**目标：** `Client` 接口加 `Chat()` + `CountTokens()`，两个实现都补全。

### 任务

**1.1 `internal/llm/models.go`（新建）**

```go
package llm

var knownContextWindows = map[string]int{
    "claude-sonnet-4-6": 1_000_000,
    "claude-opus-4-7":   1_000_000,
    "claude-haiku-4-5":  200_000,
    "gpt-4o":            128_000,
    "gpt-4o-mini":       128_000,
}

func DefaultThreshold(model string) int {
    if w, ok := knownContextWindows[model]; ok {
        return w / 2
    }
    return 120_000
}
```

**1.2 `internal/llm/client.go`**

`Client` 接口改为：

```go
type Client interface {
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
    Chat(ctx context.Context, req *ChatRequest) (string, error)
    CountTokens(ctx context.Context, msgs []Message) (int, error)
}
```

新增 `estimateTokens` 工具函数（供 OpenAI 实现和 compactor 用）：

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

func EstimateTokens(s string) int { return estimateTokens(s) }
```

**1.3 `internal/llm/claude.go`**

实现 `Chat()`：构造与 `ChatStream` 相同的请求体，发 POST，读完整响应，返回 `content[0].text`。

实现 `CountTokens()`：POST `/v1/messages/count_tokens`，返回 `input_tokens`。

**1.4 `internal/llm/openai.go`**

实现 `Chat()`：发非流式请求（`stream: false`），返回 `choices[0].message.content`。

实现 `CountTokens()`：对所有消息内容调 `estimateTokens`，累加，返回 `nil` error。

### 验证

```bash
go build ./internal/llm/...
```

所有实现编译通过，接口满足。

---

## Phase 2：Store 层扩展

**目标：** `MessageStore` 加 `ListAfterMessage`，新建 `SummaryStore`，更新 DB schema。

### 任务

**2.1 `internal/db/schema.go`**

追加到 `migrate()` 末尾（`CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS`，幂等）：

```go
// Context compaction
db.Exec(`CREATE TABLE IF NOT EXISTS conversation_summaries (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id  TEXT NOT NULL,
    up_to_message_id TEXT NOT NULL,
    chunks           TEXT NOT NULL,
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id)
)`)
db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at)`)
```

**2.2 `internal/store/message.go`**

新增方法：

```go
func (s *MessageStore) ListAfterMessage(conversationID, messageID string) ([]*models.Message, error)
```

- `messageID == ""`：等价于 `ListByConversation`（取全量）
- 否则：`WHERE conversation_id = ? AND created_at > (SELECT created_at FROM messages WHERE id = ?)`

**2.3 `internal/store/summary.go`（新建）**

```go
type ConversationSummary struct {
    ID             int64
    ConversationID string
    UpToMessageID  string
    Chunks         []string  // JSON 反序列化后
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type SummaryStore struct { db *sql.DB }

func NewSummaryStore(db *sql.DB) *SummaryStore

// 取摘要缓存，不存在返回 nil, nil
func (s *SummaryStore) Get(conversationID string) (*ConversationSummary, error)

// upsert：INSERT ... ON CONFLICT DO UPDATE SET ...
func (s *SummaryStore) Upsert(conversationID, upToMessageID string, chunks []string) error
```

### 验证

```bash
go build ./internal/store/... ./internal/db/...
```

---

## Phase 3：Compactor 核心逻辑

**目标：** 新建 `internal/agent/compactor.go`，实现压缩主流程。

### 任务

**3.1 `internal/agent/compactor.go`**

```go
type Compactor struct {
    llmClient    llm.Client
    summaryStore *store.SummaryStore
    msgStore     MessageStorer
    model        string
    cfg          config.CompactionConfig
}

func NewCompactor(
    llmClient llm.Client,
    summaryStore *store.SummaryStore,
    msgStore MessageStorer,
    model string,
    cfg config.CompactionConfig,
) *Compactor

// BuildHistory 返回注入摘要后的 history，供 Agent.Run() 使用。
// 若触发压缩，同步完成后返回。
func (c *Compactor) BuildHistory(ctx context.Context, conversationID string) ([]llm.Message, error)
```

**BuildHistory 流程（对应 spec 步骤 1-5g）：**

```
1. summaryStore.Get(conversationID) → summary (可能为 nil)
2. msgStore.ListAfterMessage(conversationID, summary.UpToMessageID) → msgs
3. totalTokens = llmClient.CountTokens(ctx, toLLMMessages(msgs))
4. threshold = resolveThreshold(c.cfg, c.model)
5. if totalTokens < threshold:
       return injectSummary(summary, msgs)
6. // 超限：计算新边界
   newBoundaryIdx = findBoundaryByTurns(msgs, c.cfg.RecentTurns)
   if newBoundaryIdx <= 0:
       return nil, ErrCannotAdvanceBoundary
   toCompress = msgs[:newBoundaryIdx]
   recent     = msgs[newBoundaryIdx:]
7. newDelta = c.summarize(ctx, toCompress)
8. chunks = append(summary.Chunks, newDelta)
9. if estimateChunksTokens(chunks) > c.cfg.MaxSummaryTokens:
       chunks = []string{c.consolidate(ctx, chunks)}
10. summaryStore.Upsert(conversationID, toCompress[last].ID, chunks)
11. return injectSummary(chunks, recent)
```

**辅助函数：**

- `findBoundaryByTurns(msgs, n int) int`：从尾部数 n 个 `role=user` 消息，返回第 n 个 user 消息的索引（即保留原文的起点）
- `injectSummary(chunks []string, recent []*models.Message) []llm.Message`：拼接 user+assistant 摘要对 + 近期原文
- `estimateChunksTokens(chunks []string) int`：对每个 chunk 调 `llm.EstimateTokens`，累加
- `summarize(ctx, msgs)`：调 `llmClient.Chat()` 用片段压缩 prompt
- `consolidate(ctx, chunks)`：调 `llmClient.Chat()` 用整体压缩 prompt

**ErrCannotAdvanceBoundary**：边界无法推进时返回，供 `Run()` 在强制压缩路径中直接返回错误。

### 验证

```bash
go build ./internal/agent/...
```

---

## Phase 4：Agent 集成

**目标：** EPA 英文化，`AgentConfig` 加 `Compactor`，`Run()` 调用 `BuildHistory`，config 加 `CompactionConfig`。

### 任务

**4.1 `internal/config/config.go`**

```go
type CompactionConfig struct {
    ThresholdTokens  int `yaml:"threshold_tokens"`
    RecentTurns      int `yaml:"recent_turns"`
    MaxSummaryTokens int `yaml:"max_summary_tokens"`
}

type AgentConfig struct {
    PermissionMode  string            `yaml:"permission_mode"`
    ApprovalTimeout int               `yaml:"approval_timeout"`
    Rules           []RuleConfig      `yaml:"rules,omitempty" json:"rules,omitempty"`
    Compaction      CompactionConfig  `yaml:"compaction"`
}
```

`DefaultConfig()` 加：

```go
Agent: AgentConfig{
    PermissionMode:  "ask",
    ApprovalTimeout: 300,
    Compaction: CompactionConfig{
        ThresholdTokens:  0,
        RecentTurns:      20,
        MaxSummaryTokens: 4000,
    },
},
```

**4.2 `internal/agent/agent.go`**

- `epaSystemPromptPrefix` 改为英文（见 spec 附加改动节）
- `MessageStorer` 接口加 `ListAfterMessage`
- `AgentConfig` 加 `Compactor *Compactor`（可为 nil，nil 时跳过压缩）
- `Agent` struct 加 `compactor *Compactor`
- `NewAgent` 赋值 `compactor`
- `Run()` 在取 history 前调用：

```go
var history []llm.Message
if a.compactor != nil {
    history, err = a.compactor.BuildHistory(ctx, conversationID)
    if err != nil {
        // 发 EventError，关闭 channel，返回
    }
} else {
    // 原有逻辑：ListByConversation → toLLMMessages
}
```

强制压缩路径（API 返回 context length 错误时）：

```go
// 在 ChatStream 错误处理中
if isContextLengthError(err) && a.compactor != nil && !alreadyRetried {
    history, err = a.compactor.BuildHistory(ctx, conversationID) // force=true 语义：threshold 设为 0
    if errors.Is(err, ErrCannotAdvanceBoundary) {
        return fmt.Errorf("context too long and cannot compact further: %w", err)
    }
    alreadyRetried = true
    // 重试 ChatStream
}
```

**4.3 `internal/agent/factory.go`**

`Factory` 加 `SummaryStore *store.SummaryStore` 和 `CompactionCfg config.CompactionConfig`。

`NewAgent` 方法中构造 `Compactor`：

```go
compactor := NewCompactor(f.LLMClient, f.SummaryStore, f.MsgStore, f.LLMModel, f.CompactionCfg)
return NewAgent(AgentConfig{
    ...,
    Compactor: compactor,
})
```

`Factory` 需知道当前 model 名（从 provider 配置取），加 `LLMModel string` 字段。

### 验证

```bash
go build ./...
```

---

## Phase 5：单元测试

**目标：** compactor 核心逻辑单元测试，更新 agent_test.go mocks。

### 任务

**5.1 `internal/agent/agent_test.go`**

更新 `mockLLMClient`：

```go
func (m *mockLLMClient) Chat(_ context.Context, req *llm.ChatRequest) (string, error) {
    // 返回固定字符串 "summary"
    return "summary", nil
}

func (m *mockLLMClient) CountTokens(_ context.Context, msgs []llm.Message) (int, error) {
    total := 0
    for _, msg := range msgs {
        total += llm.EstimateTokens(msg.Content)
    }
    return total, nil
}
```

更新 `mockMsgStore`：

```go
func (m *mockMsgStore) ListAfterMessage(convID, messageID string) ([]*models.Message, error) {
    if messageID == "" {
        return m.ListByConversation(convID)
    }
    var out []*models.Message
    found := false
    for _, msg := range m.messages {
        if found && msg.convID == convID {
            out = append(out, &models.Message{...})
        }
        if msg.id == messageID {
            found = true
        }
    }
    return out, nil
}
```

**5.2 `internal/agent/compactor_test.go`（新建）**

测试用例：

| 测试名 | 场景 | 验证 |
|--------|------|------|
| `TestBuildHistory_UnderThreshold` | token < threshold | 返回全量消息，不调 Chat |
| `TestBuildHistory_OverThreshold_NoCache` | 超限，无缓存 | 调 Chat 生成摘要，Upsert 被调用，history 首位是摘要对 |
| `TestBuildHistory_OverThreshold_WithCache` | 超限，有缓存 | 追加新 chunk，旧 chunk 不变 |
| `TestBuildHistory_ChunksOverflow` | chunks 超 MaxSummaryTokens | 触发整体压缩，chunks 变为 1 个 |
| `TestBuildHistory_CannotAdvanceBoundary` | 消息不足 recent_turns 轮 | 返回 ErrCannotAdvanceBoundary |
| `TestFindBoundaryByTurns` | 各种消息序列 | 边界索引正确 |
| `TestEstimateChunksTokens` | 纯 ASCII / 纯 CJK / 混合 | token 数在预期范围内 |

**5.3 `internal/store/summary_test.go`（新建）**

使用 in-memory SQLite（`modernc.org/sqlite`）：

| 测试名 | 验证 |
|--------|------|
| `TestSummaryStore_GetNotFound` | 不存在返回 nil, nil |
| `TestSummaryStore_UpsertAndGet` | 写入后读回，chunks 正确 |
| `TestSummaryStore_UpsertOverwrite` | 二次 upsert 覆盖，updated_at 更新，created_at 不变 |

**5.4 `internal/store/message_test.go`（新建或追加）**

| 测试名 | 验证 |
|--------|------|
| `TestListAfterMessage_EmptyID` | 等价全量 |
| `TestListAfterMessage_WithID` | 只返回指定 ID 之后的消息 |

### 验证

```bash
go test ./internal/agent/... ./internal/store/...
```

所有测试通过，无 race condition（`-race` flag）。

---

## Phase 6：集成测试

**目标：** 端到端自动化测试，连接测试环境 DB `/Users/cw/.spider/spider.db`，覆盖 spec 验收标准全部条目。

### 任务

**6.1 `internal/agent/compaction_integration_test.go`（新建）**

```go
const testDBPath = "/Users/cw/.spider/spider.db"

func openTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite", testDBPath)
    require.NoError(t, err)
    require.NoError(t, dbpkg.ApplySchema(db)) // 幂等，只建新表/索引
    return db
}
```

mock 只替换 `llmClient`（避免真实 API 调用），其余全用真实实现。

| 测试名 | 对应验收标准 | 验证 |
|--------|-------------|------|
| `TestIntegration_ShortConversation` | 短对话未超限 | `conversation_summaries` 表无记录，history 为全量消息 |
| `TestIntegration_CacheReuse` | 有缓存且 boundary 后未超限 | `Chat()` 不被调用，边界不变 |
| `TestIntegration_FirstCompaction` | 超限无缓存 | `Chat()` 被调用一次，DB 有摘要记录，history 首位是摘要对 |
| `TestIntegration_BoundaryAdvance` | 边界推进 | 二次超限后 `up_to_message_id` 更新，chunks 追加新片段，旧片段不变 |
| `TestIntegration_ChunksConsolidation` | chunks 超 MaxSummaryTokens | 整体压缩触发，DB 中 chunks 长度变为 1 |
| `TestIntegration_ThresholdConfig` | `threshold_tokens > 0` 时忽略模型表 | 手动设 threshold=100，短消息也触发压缩 |
| `TestIntegration_UnknownModelFallback` | 未知模型 fallback 120,000 | model="unknown-model"，threshold 为 120,000 |

```go
// mockChatClient 记录 Chat() 调用次数，返回固定摘要文本
type mockChatClient struct {
    chatCalls int
    response  string
}
```

### 验证

```bash
go test ./internal/agent/... -run Integration -v
```

所有集成测试通过。

---

## 全量验证

```bash
go build -a ./...
go test ./internal/agent/... ./internal/store/... ./internal/llm/...
```

所有测试通过后，Phase 完成。
