package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

var ErrCannotAdvanceBoundary = errors.New("cannot advance compaction boundary: not enough turns")

type summaryStorer interface {
	Get(conversationID string) (*store.ConversationSummary, error)
	Upsert(conversationID, upToMessageID string, chunks []string) error
}

type Compactor struct {
	llmClient    llm.Client
	summaryStore summaryStorer
	msgStore     MessageStorer
	model        string
	cfg          config.CompactionConfig
}

func NewCompactor(
	llmClient llm.Client,
	summaryStore summaryStorer,
	msgStore MessageStorer,
	model string,
	cfg config.CompactionConfig,
) *Compactor {
	if cfg.RecentTurns == 0 {
		cfg.RecentTurns = 20
	}
	if cfg.MaxSummaryTokens == 0 {
		cfg.MaxSummaryTokens = 4000
	}
	return &Compactor{
		llmClient:    llmClient,
		summaryStore: summaryStore,
		msgStore:     msgStore,
		model:        model,
		cfg:          cfg,
	}
}

// BuildHistory 返回注入摘要后的 history，供 Agent.Run() 使用。
// 若触发压缩，同步完成后返回。
func (c *Compactor) BuildHistory(ctx context.Context, conversationID string) ([]llm.Message, error) {
	// 1. 取摘要缓存
	summary, err := c.summaryStore.Get(conversationID)
	if err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}

	var boundaryID string
	var existingChunks []string
	if summary != nil {
		boundaryID = summary.UpToMessageID
		existingChunks = summary.Chunks
	}

	// 2. 取 boundary 之后的所有消息
	msgs, err := c.msgStore.ListAfterMessage(conversationID, boundaryID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	// 3. 估算 token 数
	threshold := c.resolveThreshold()
	totalTokens, err := c.llmClient.CountTokens(ctx, toLLMMessages(msgs))
	if err != nil {
		return nil, fmt.Errorf("count tokens: %w", err)
	}

	// 4. 未超限：直接返回
	if totalTokens < threshold {
		return injectSummary(existingChunks, msgs), nil
	}

	// 5. 超限：计算新边界
	boundaryIdx := findBoundaryByTurns(msgs, c.cfg.RecentTurns)
	if boundaryIdx <= 0 {
		return nil, ErrCannotAdvanceBoundary
	}

	toCompress := msgs[:boundaryIdx]
	recent := msgs[boundaryIdx:]

	// 6. 生成新摘要片段
	newDelta, err := c.summarize(ctx, toCompress)
	if err != nil {
		return nil, fmt.Errorf("summarize: %w", err)
	}

	chunks := append(existingChunks[:len(existingChunks):len(existingChunks)], newDelta)

	// 7. chunks 超限则整体压缩
	if estimateChunksTokens(chunks) > c.cfg.MaxSummaryTokens {
		consolidated, err := c.consolidate(ctx, chunks)
		if err != nil {
			return nil, fmt.Errorf("consolidate: %w", err)
		}
		chunks = []string{consolidated}
	}

	// 8. 存 DB
	newBoundaryID := toCompress[len(toCompress)-1].ID
	if err := c.summaryStore.Upsert(conversationID, newBoundaryID, chunks); err != nil {
		return nil, fmt.Errorf("upsert summary: %w", err)
	}

	return injectSummary(chunks, recent), nil
}

func (c *Compactor) resolveThreshold() int {
	if c.cfg.ThresholdTokens > 0 {
		return c.cfg.ThresholdTokens
	}
	return llm.DefaultThreshold(c.model)
}

// findBoundaryByTurns 从尾部数 n 个 role=user 消息，返回第 n 个 user 消息的索引。
// 若消息不足 n 轮，返回 0（调用方应返回 ErrCannotAdvanceBoundary）。
func findBoundaryByTurns(msgs []*models.Message, n int) int {
	count := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			count++
			if count == n {
				return i
			}
		}
	}
	return 0
}

func injectSummary(chunks []string, recent []*models.Message) []llm.Message {
	var history []llm.Message
	if len(chunks) > 0 {
		history = append(history,
			llm.Message{Role: "user", Content: "The following is a summary of the previous conversation, for reference."},
			llm.Message{Role: "assistant", Content: strings.Join(chunks, "\n\n")},
		)
	}
	history = append(history, toLLMMessages(recent)...)
	return history
}

func estimateChunksTokens(chunks []string) int {
	total := 0
	for _, c := range chunks {
		total += llm.EstimateTokens(c)
	}
	return total
}

func toLLMMessages(msgs []*models.Message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, llm.Message{Role: llm.Role(m.Role), Content: m.Content})
	}
	return out
}

const segmentSummaryPrompt = `The following is a segment of a network device management conversation.
Generate a concise summary. You MUST preserve:
- The user's goal and intent
- Commands executed and their key results
- Device states, issues, and anomalies discovered
- Incomplete tasks

Ignore: small talk, repeated confirmations, intermediate reasoning.

Conversation segment:
%s`

const consolidateSummaryPrompt = `The following are multiple historical summary segments.
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
%s`

func (c *Compactor) summarize(ctx context.Context, msgs []*models.Message) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&sb, "[%s]: %s\n", m.Role, m.Content)
	}
	return c.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: fmt.Sprintf(segmentSummaryPrompt, sb.String())},
		},
		MaxTokens: 1024,
	})
}

func (c *Compactor) consolidate(ctx context.Context, chunks []string) (string, error) {
	return c.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: fmt.Sprintf(consolidateSummaryPrompt, strings.Join(chunks, "\n\n---\n\n"))},
		},
		MaxTokens: 1024,
	})
}
