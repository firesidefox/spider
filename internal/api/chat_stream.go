package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

// chatStreamGet handles GET /api/v1/chat/conversations/:id/stream?last_event_id=X
// Replays messages from DB as SSE events, then subscribes to live updates.
func chatStreamGet(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	_, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}

	// Parse last_event_id: query param takes precedence, then browser Last-Event-ID header
	lastEventID := 0
	rawID := r.URL.Query().Get("last_event_id")
	if rawID == "" {
		rawID = r.Header.Get("Last-Event-ID")
	}
	if rawID != "" {
		if n, err := strconv.Atoi(rawID); err == nil {
			lastEventID = n
		}
	}

	// Fetch all messages
	msgs, err := app.MsgStore.ListByConversation(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	// Setup SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	// Replay messages after last_event_id
	for i, msg := range msgs {
		if i <= lastEventID {
			continue
		}
		event := map[string]any{
			"type": "message",
			"content": map[string]any{
				"id":              msg.ID,
				"conversation_id": msg.ConversationID,
				"role":            msg.Role,
				"content":         msg.Content,
				"tool_calls":      msg.ToolCalls,
				"created_at":      msg.CreatedAt,
			},
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "id: %d\ndata: %s\n\n", i, data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	// Subscribe to live updates
	ch := make(chan []byte, 10)
	app.RegisterSSEClient(id, ch)
	defer app.UnregisterSSEClient(id, ch)

	// Keep-alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}
