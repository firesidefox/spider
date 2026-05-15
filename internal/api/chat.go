package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spiderai/spider/internal/agent"
	authmw "github.com/spiderai/spider/internal/auth"
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
	if err := app.ConvStore.Delete(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
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
		Content  string   `json:"content"`
		HostIDs  []string `json:"host_ids"`
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

	systemPrompt := factory.BuildSystemPrompt()
	a := factory.NewAgent(systemPrompt, id, req.HostIDs)
	waiter := agent.NewConfirmationWaiter()
	app.StoreChatWaiter(id, waiter)
	defer app.RemoveChatWaiter(id)

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

	app.ConvStore.SetStatus(id, "processing") //nolint:errcheck
	parent := app.ShutdownCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	app.StoreConvCancel(id, cancel)
	defer func() {
		cancel()
		app.RemoveConvCancel(id)
	}()
	events, err := a.Run(ctx, id, content, waiter)
	if err != nil {
		app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
		writeError(w, 500, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

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
		fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
		app.BroadcastSSE(id, data)
	}
	app.ConvStore.SetStatus(id, "idle") //nolint:errcheck
}

// allFacesDisableKB returns true when every access face in the system
// has knowledge_sources set to the "none" sentinel [{type:"none",id:0}].
// Used to skip registering SearchDocsTool entirely.
func allFacesDisableKB(app *mcppkg.App) bool {
	hosts, err := app.HostStore.List("")
	if err != nil || len(hosts) == 0 {
		return false
	}
	for _, h := range hosts {
		faces, err := app.AccessFaceStore.ListByHost(h.ID)
		if err != nil {
			return false
		}
		for _, f := range faces {
			if len(f.KnowledgeSources) == 0 || f.KnowledgeSources[0].Type != "none" {
				return false
			}
		}
	}
	return true
}

func injectHostNames(app *mcppkg.App, content map[string]any) {
	input, _ := content["input"].(map[string]any)
	if input == nil {
		return
	}
	var ids []string
	switch v := input["host_ids"].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				ids = append(ids, s)
			}
		}
	case []string:
		ids = v
	}
	if s, ok := input["host_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if len(ids) == 0 {
		return
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if h, err := app.HostStore.GetByID(id); err == nil {
			names = append(names, h.Name)
		} else {
			names = append(names, id)
		}
	}
	content["host_names"] = names
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
