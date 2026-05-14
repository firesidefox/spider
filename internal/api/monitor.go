package api

import (
	"fmt"
	"net/http"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func hostStatuses(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	if app.Monitor == nil {
		writeJSON(w, 200, []any{})
		return
	}
	statuses := app.Monitor.Statuses()
	type item struct {
		HostID    string    `json:"host_id"`
		Online    bool      `json:"online"`
		CheckedAt time.Time `json:"checked_at"`
	}
	out := make([]item, 0, len(statuses))
	now := time.Now()
	for id, online := range statuses {
		out = append(out, item{HostID: id, Online: online, CheckedAt: now})
	}
	writeJSON(w, 200, out)
}

func globalStream(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 32)
	app.AddGlobalSSEClient(ch)
	defer app.RemoveGlobalSSEClient(ch)

	fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
