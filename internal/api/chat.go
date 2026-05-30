package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spiderai/spider/internal/agent"
	authmw "github.com/spiderai/spider/internal/auth"
	"github.com/spiderai/spider/internal/llm"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/permission"
)

func verifyConvOwner(app *mcppkg.App, r *http.Request, id string) (*models.Conversation, error) {
	conv, err := app.ConvStore.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}
	uc := authmw.GetUser(r.Context())
	if uc != nil && uc.UserID != conv.UserID {
		return nil, fmt.Errorf("forbidden")
	}
	return conv, nil
}

func chatCreateConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	userID := authmw.GetUser(r.Context()).UserID
	conv, err := app.ConvStore.Create(userID, req.Title)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, conv)
}

func chatListConversations(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	userID := authmw.GetUser(r.Context()).UserID
	convs, err := app.ConvStore.ListByUser(userID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, convs)
}

func chatGetConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	conv, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	msgs, err := app.MsgStore.ListByConversation(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	// Filter out tool_result messages - results already in assistant's tool_calls JSON
	n := 0
	for _, m := range msgs {
		if m.Role != "tool_result" {
			msgs[n] = m
			n++
		}
	}
	msgs = msgs[:n]
	tasks, err := app.TodoStore.List(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if tasks == nil {
		tasks = []*models.Todo{}
	}
	writeJSON(w, 200, map[string]any{"conversation": conv, "messages": msgs, "todo_tasks": tasks})
}

func chatDeleteConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if _, err := verifyConvOwner(app, r, id); err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	if err := app.MsgStore.DeleteByConversation(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	app.DB.Exec(`DELETE FROM conversation_summaries WHERE conversation_id = ?`, id)
	if err := app.ConvStore.Delete(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	cleanupToolResultFiles(app.Config.DataDir, id)
	w.WriteHeader(204)
}

func chatUpdateTitle(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if _, err := verifyConvOwner(app, r, id); err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	var req struct {
		Title          *string `json:"title"`
		PermissionMode *string `json:"permission_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if req.Title != nil {
		if err := app.ConvStore.UpdateTitle(id, *req.Title); err != nil {
			writeError(w, 500, err.Error())
			return
		}
	}
	if req.PermissionMode != nil {
		mode := *req.PermissionMode
		if mode != "" {
			m := permission.Mode(mode)
			if !m.IsValid() {
				writeError(w, 400, "无效的权限模式")
				return
			}
		}
		if err := app.ConvStore.UpdatePermissionMode(id, mode); err != nil {
			writeError(w, 500, err.Error())
			return
		}
	}
	w.WriteHeader(204)
}

func chatSendMessage(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	factory, err := app.NewAgentFactory()
	if err != nil {
		writeError(w, 503, "LLM not configured: "+err.Error())
		return
	}
	factory.DataDir = app.Config.DataDir
	var req struct {
		Content string   `json:"content"`
		HostIDs []string `json:"host_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}

	conv, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	if conv.PermissionMode != "" {
		factory.PermissionMode = permission.Mode(conv.PermissionMode)
	}
	factory.DisableSearchDocs = allFacesDisableKB(app)

	content := req.Content
	if rs, rsErr := ragStore(app); rsErr == nil {
		groupLookup := func(name string) *int {
			if name == "" {
				return nil
			}
			groups, _ := app.GroupStore.List()
			for _, g := range groups {
				if g.Name == name {
					gid := g.ID
					return &gid
				}
			}
			return nil
		}
		docLookup := func(groupID int, title string) *models.Document {
			doc, _ := app.DocStore.FindByTitle(groupID, title)
			return doc
		}
		search := func(query string, groupID *int) []*models.Document {
			docs, _ := rs.SearchByGroup(r.Context(), query, groupID, 3)
			return docs
		}
		content = expandKBRefs(content, groupLookup, docLookup, search)
	}

	// Try to inject into a running agent first
	if queued, full := app.TryInject(id, content); queued || full {
		if full {
			writeError(w, 429, "message queue full")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
		return
	}

	// No agent running — try to claim the conv
	injectCh, claimed := app.TryClaimConv(id)
	if !claimed {
		// Lost the race to another concurrent request — try inject again.
		// If the winning agent already finished (ReleaseConv called between our
		// TryClaimConv and this TryInject), fall through to claim a new one.
		if queued, full := app.TryInject(id, content); queued {
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
			return
		} else if full {
			writeError(w, 429, "message queue full")
			return
		}
		// Agent finished between our two checks — try to claim now.
		injectCh, claimed = app.TryClaimConv(id)
		if !claimed {
			writeError(w, 503, "agent start conflict, retry")
			return
		}
	}

	a := factory.NewAgent(id, req.HostIDs)
	waiter := agent.NewConfirmationWaiter()
	app.StoreChatWaiter(id, waiter)
	goroutineLaunched := false
	defer func() {
		if !goroutineLaunched {
			app.RemoveChatWaiter(id)
			app.ReleaseConv(id)
		}
	}()

	app.ConvStore.SetStatus(id, "processing") //nolint:errcheck
	parent := app.ShutdownCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	app.StoreConvCancel(id, cancel)
	events, err := a.Run(ctx, id, content, waiter, injectCh)
	if err != nil {
		cancel()
		app.RemoveConvCancel(id)
		app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
		writeError(w, 500, err.Error())
		return
	}

	goroutineLaunched = true
	go func() {
		defer func() {
			cancel()
			app.RemoveConvCancel(id)
			app.RemoveChatWaiter(id)
			app.ReleaseConv(id)
			app.ClearSSEBuffer(id)
			app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
		}()
		for ev := range events {
			if ev.Type == agent.EventToolStart {
				injectHostNames(app, ev.Content)
			}
			if ev.Type == agent.EventToolStart || ev.Type == agent.EventToolResult {
				name, _ := ev.Content["name"].(string)
				if name == "" {
					name, _ = ev.Content["tool"].(string)
				}
				if name == "Todo" {
					continue
				}
			}
			data, _ := json.Marshal(ev)
			app.BufferSSEEvent(id, data)
			app.BroadcastSSE(id, data)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// allFacesDisableKB is retained for compatibility with the agent factory hook.
// SearchDocs remains available even when faces do not expose KB bindings.
func allFacesDisableKB(app *mcppkg.App) bool {
	return false
}

func injectHostNames(app *mcppkg.App, content map[string]any) {
	input, _ := content["input"].(map[string]any)
	if input == nil {
		return
	}
	names := app.HostStore.ResolveNames(input)
	if len(names) > 0 {
		content["host_names"] = names
	}
}

func chatCancel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if _, err := verifyConvOwner(app, r, id); err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	app.CancelConv(id)
	app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func chatConfirm(app *mcppkg.App, w http.ResponseWriter, r *http.Request, convID, requestID string) {
	var req struct {
		Approved bool `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	waiter := app.GetChatWaiter(convID)
	if waiter == nil {
		writeError(w, 404, "no active conversation waiter")
		return
	}
	waiter.Resolve(requestID, req.Approved)
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func cleanupToolResultFiles(dataDir, conversationID string) {
	if dataDir == "" {
		return
	}
	dir := filepath.Join(dataDir, "tool-results", conversationID)
	_ = os.RemoveAll(dir)
}

const suggestTitlePrompt = `Based on the conversation below, generate a short kebab-case description (English, lowercase, 3-6 words, no date prefix) that captures the main topic.

Rules:
- Output ONLY the kebab-case string, nothing else
- No quotes, no explanation
- Examples: "fix-auth-middleware", "add-sse-reconnect", "refactor-tool-display"

Conversation:
%s`

func chatSuggestTitle(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	conv, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}

	factory, err := app.NewAgentFactory()
	if err != nil {
		writeError(w, 503, "LLM not configured: "+err.Error())
		return
	}

	msgs, err := app.MsgStore.ListByConversation(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if len(msgs) == 0 {
		writeError(w, 400, "no messages in conversation")
		return
	}

	var sb strings.Builder
	for _, m := range msgs {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := m.Content
		if len(content) > 200 {
			content = content[:200]
		}
		fmt.Fprintf(&sb, "[%s]: %s\n", m.Role, content)
		if sb.Len() > 2000 {
			break
		}
	}

	req := &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: fmt.Sprintf(suggestTitlePrompt, sb.String())},
		},
		MaxTokens: 64,
	}
	desc, err := factory.LLMClient.Chat(r.Context(), req)
	if err != nil {
		writeError(w, 500, "LLM error: "+err.Error())
		return
	}

	desc = strings.TrimSpace(desc)
	// LLM generates only the topic slug; server prepends the date prefix.
	title := conv.CreatedAt.Format("2006-01-02-1504") + "-" + desc

	writeJSON(w, 200, map[string]string{"title": title})
}
