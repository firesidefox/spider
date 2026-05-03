package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/permission"
)

func listApprovals(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	if app.ApprovalManager == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, app.ApprovalManager.Pending())
}

func respondApproval(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string, approve bool) {
	if app.ApprovalManager == nil {
		http.Error(w, "approval manager not available", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		By string `json:"approved_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.By = "operator"
	}
	app.ApprovalManager.Respond(id, approve, body.By)
	status := "approved"
	if !approve {
		status = "rejected"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status, "id": id})
}

func streamApprovals(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if app.ApprovalManager == nil {
		fmt.Fprintf(w, "data: {}\n\n")
		flusher.Flush()
		return
	}

	ch := app.ApprovalManager.Subscribe()
	defer app.ApprovalManager.Unsubscribe(ch)

	for _, req := range app.ApprovalManager.Pending() {
		sendSSEApproval(w, flusher, req)
	}
	for {
		select {
		case req, ok := <-ch:
			if !ok {
				return
			}
			sendSSEApproval(w, flusher, req)
		case <-r.Context().Done():
			return
		}
	}
}

func sendSSEApproval(w http.ResponseWriter, flusher http.Flusher, req *permission.ApprovalRequest) {
	data, err := json.Marshal(req)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func approvalRouter(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/approvals")

	if rest == "/stream" {
		if r.Method == http.MethodGet {
			streamApprovals(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rest = strings.TrimPrefix(rest, "/")
	if idx := indexOf(rest, '/'); idx >= 0 {
		id, action := rest[:idx], rest[idx+1:]
		switch {
		case action == "approve" && r.Method == http.MethodPost:
			respondApproval(app, w, r, id, true)
		case action == "reject" && r.Method == http.MethodPost:
			respondApproval(app, w, r, id, false)
		default:
			http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}
