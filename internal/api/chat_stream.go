package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

// chatStreamGet handles GET /api/v1/chat/conversations/:id/stream?last_event_id=X
// Replays messages from DB after the given message UUID cursor, then replays any
// in-flight events from the current agent run, then subscribes to live updates.
func chatStreamGet(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	_, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}

	// last_event_id is now a message UUID (or empty for full replay)
	lastMsgID := r.URL.Query().Get("last_event_id")
	if lastMsgID == "" {
		lastMsgID = r.Header.Get("Last-Event-ID")
	}

	// Fetch only messages after cursor
	msgs, err := app.MsgStore.ListAfterMessage(id, lastMsgID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	// Setup SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	// Replay persisted messages
	for _, msg := range msgs {
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
		fmt.Fprintf(w, "id: %s\ndata: %s\n\n", msg.ID, data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	// Replay in-flight events from current agent run (not yet persisted)
	for _, data := range app.DrainSSEBuffer(id) {
		fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	// Subscribe to live updates
	ch := make(chan []byte, 10)
	app.RegisterSSEClient(id, ch)
	defer app.UnregisterSSEClient(id, ch)

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
