package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/permission"
)

// listApprovals 返回待审批列表。
// GET /api/v1/approvals
func listApprovals(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	if app.ApprovalManager == nil {
		jsonOK(w, []any{})
		return
	}
	pending := app.ApprovalManager.Pending()
	jsonOK(w, pending)
}

// approveApproval 批准审批请求。
// POST /api/v1/approvals/:id/approve
func approveApproval(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if app.ApprovalManager == nil {
		http.Error(w, "approval manager not available", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		ApprovedBy string `json:"approved_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.ApprovedBy = "operator"
	}
	app.ApprovalManager.Respond(id, true, body.ApprovedBy)
	jsonOK(w, map[string]string{"status": "approved", "id": id})
}

// rejectApproval 拒绝审批请求。
// POST /api/v1/approvals/:id/reject
func rejectApproval(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if app.ApprovalManager == nil {
		http.Error(w, "approval manager not available", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		RejectedBy string `json:"rejected_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.RejectedBy = "operator"
	}
	app.ApprovalManager.Respond(id, false, body.RejectedBy)
	jsonOK(w, map[string]string{"status": "rejected", "id": id})
}

// streamApprovals 通过 SSE 推送审批请求。
// GET /api/v1/approvals/stream
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

	// 先推送当前待审批列表
	pending := app.ApprovalManager.Pending()
	for _, req := range pending {
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

// approvalRouter 处理 /api/v1/approvals/ 路径下的子路由。
func approvalRouter(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	rest := strings.TrimPrefix(path, "/api/v1/approvals")

	// /api/v1/approvals/stream
	if rest == "/stream" {
		if r.Method == http.MethodGet {
			streamApprovals(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// /api/v1/approvals/:id/approve  or  /api/v1/approvals/:id/reject
	rest = strings.TrimPrefix(rest, "/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 2 {
		id := parts[0]
		action := parts[1]
		switch {
		case action == "approve" && r.Method == http.MethodPost:
			approveApproval(app, w, r, id)
		case action == "reject" && r.Method == http.MethodPost:
			rejectApproval(app, w, r, id)
		default:
			http.NotFound(w, r)
		}
		return
	}

	http.NotFound(w, r)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
