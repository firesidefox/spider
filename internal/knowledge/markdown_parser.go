package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spiderai/spider/internal/llm"
)

const markdownMaxChars = 32000

// MarkdownParser uses an LLM to extract semantic entries from Markdown documents.
type MarkdownParser struct {
	client llm.Client
}

// NewMarkdownParser creates a MarkdownParser backed by the given LLM client.
func NewMarkdownParser(llmClient llm.Client) *MarkdownParser {
	return &MarkdownParser{client: llmClient}
}

// Parse implements Parser. Splits long documents into chunks, parses each, and merges results.
func (p *MarkdownParser) Parse(ctx context.Context, content []byte, filename string) ([]ParsedEntry, error) {
	chunks := p.splitMarkdown(string(content), markdownMaxChars)
	var all []ParsedEntry
	for _, chunk := range chunks {
		entries, err := p.parseChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
	}
	return all, nil
}

// llmEntriesResponse is the expected JSON shape from the LLM.
type llmEntriesResponse struct {
	Entries []struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Content string `json:"content"`
	} `json:"entries"`
}

// parseMarkdownLLMResponse extracts entries from raw LLM JSON output.
func parseMarkdownLLMResponse(raw string) ([]ParsedEntry, error) {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fences
	if strings.HasPrefix(raw, "```") {
		start := strings.Index(raw, "\n")
		end := strings.LastIndex(raw, "```")
		if start != -1 && end > start {
			raw = strings.TrimSpace(raw[start+1 : end])
		}
	}

	// Extract JSON object from mixed text
	jsonStart := strings.Index(raw, "{")
	jsonEnd := strings.LastIndex(raw, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		// Fallback: treat entire response as single entry
		return []ParsedEntry{{
			Title:   "Document Content",
			Summary: "Full document content (LLM returned non-JSON response)",
			Content: raw,
		}}, nil
	}
	raw = raw[jsonStart : jsonEnd+1]

	var resp llmEntriesResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		// Fallback: treat as single entry
		return []ParsedEntry{{
			Title:   "Document Content",
			Summary: "Full document content (JSON parse failed)",
			Content: raw,
		}}, nil
	}

	if len(resp.Entries) == 0 {
		return nil, fmt.Errorf("LLM returned empty entries array")
	}

	entries := make([]ParsedEntry, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		if e.Title == "" || e.Content == "" {
			continue // skip incomplete entries
		}
		entries = append(entries, ParsedEntry{Title: e.Title, Summary: e.Summary, Content: e.Content})
	}
	return entries, nil
}

// parseChunk sends one chunk to the LLM and parses the JSON response.
func (p *MarkdownParser) parseChunk(ctx context.Context, text string) ([]ParsedEntry, error) {
	prompt := buildMarkdownParsePrompt(text)
	req := &llm.ChatRequest{
		System: []llm.SystemBlock{
			{Text: "You are a technical documentation parser. You MUST respond with ONLY valid JSON. Never include explanations, markdown fences, or any text outside the JSON object."},
		},
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens: 4096,
	}
	resp, err := p.client.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	// Log response for debugging
	slog.Debug("markdown parser LLM response", "response", truncate(resp, 500))

	return parseMarkdownLLMResponse(resp)
}

// splitMarkdown splits text into chunks no larger than maxChars.
// Splits on "\n##" section boundaries first, then "\n\n" paragraphs.
func (p *MarkdownParser) splitMarkdown(text string, maxChars int) []string {
	return splitMarkdownText(text, maxChars)
}

// SplitMarkdownForTest is an exported wrapper for use in store_test.go.
func SplitMarkdownForTest(text string, maxChars int) []string {
	return splitMarkdownText(text, maxChars)
}

func splitMarkdownText(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}
	// Try splitting by section headings first, fall back to paragraphs.
	parts := strings.Split(text, "\n##")
	if len(parts) <= 1 {
		parts = strings.Split(text, "\n\n")
	} else {
		// Re-attach the heading marker that was split away (except first part).
		for i := 1; i < len(parts); i++ {
			parts[i] = "##" + parts[i]
		}
	}
	return accumulateChunks(parts, maxChars)
}

func accumulateChunks(parts []string, maxChars int) []string {
	var chunks []string
	var buf strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		// If adding this part would exceed the limit, flush the buffer.
		sep := ""
		if buf.Len() > 0 {
			sep = "\n\n"
		}
		if buf.Len()+len(sep)+len(part) > maxChars && buf.Len() > 0 {
			chunks = append(chunks, buf.String())
			buf.Reset()
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		// If a single part exceeds maxChars, hard-split it.
		if len(part) > maxChars {
			for len(part) > 0 {
				end := maxChars
				if end > len(part) {
					end = len(part)
				}
				chunks = append(chunks, part[:end])
				part = part[end:]
			}
			buf.Reset()
			continue
		}
		buf.WriteString(part)
	}
	if buf.Len() > 0 {
		chunks = append(chunks, buf.String())
	}
	return chunks
}

// buildMarkdownParsePrompt constructs the LLM prompt for a document chunk.
func buildMarkdownParsePrompt(text string) string {
	return `Parse the following Markdown documentation and extract all meaningful entries.

Each entry should be a self-contained concept: a command, flag, option, section, or topic.

CRITICAL: You MUST respond with ONLY a JSON object. No explanations, no markdown fences, no extra text.

Required JSON format:
{"entries": [{"title": "...", "summary": "...", "content": "..."}]}

Fields:
- title: short identifier (command name, section heading, etc.)
- summary: one sentence describing what it does
- content: the full relevant text for this entry

Start your response with { and end with }

Document:
` + text
}
