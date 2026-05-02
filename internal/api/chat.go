package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spiderai/spider/internal/agent"
	authmw "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func chatCreateConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
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
	conv, err := app.ConvStore.GetByID(id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	msgs, err := app.MsgStore.ListByConversation(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"conversation": conv, "messages": msgs})
}

func chatDeleteConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
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
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if err := app.ConvStore.UpdateTitle(id, req.Title); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func chatSendMessage(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if app.AgentFactory == nil {
		writeError(w, 503, "LLM not configured")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}

	systemPrompt := agent.BuildSystemPrompt(app.HostStore)
	a := app.AgentFactory.NewAgent(systemPrompt)
	waiter := agent.NewConfirmationWaiter()
	app.StoreChatWaiter(id, waiter)
	defer app.RemoveChatWaiter(id)

	events, err := a.Run(r.Context(), id, req.Content, waiter)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	for ev := range events {
		data, _ := json.Marshal(ev)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
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
