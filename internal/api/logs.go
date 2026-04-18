package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	authmw "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
)

func listLogs(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hostID := q.Get("host_id")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	if hostID != "" {
		h, err := app.HostStore.GetByIDOrName(hostID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		hostID = h.ID
	}

	triggeredBy := q.Get("triggered_by")
	if triggeredBy == "me" {
		uc := authmw.GetUser(r.Context())
		if uc == nil || uc.UserID == "anonymous" {
			triggeredBy = ""
		} else {
			user, err := app.UserStore.GetByID(uc.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			triggeredBy = user.Username
		}
	}

	logs, err := app.LogStore.List(hostID, triggeredBy, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = []*models.ExecutionLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func getLog(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var log struct {
		ID          string `json:"id"`
		HostID      string `json:"host_id"`
		HostName    string `json:"host_name"`
		Command     string `json:"command"`
		Stdout      string `json:"stdout"`
		Stderr      string `json:"stderr"`
		ExitCode    int    `json:"exit_code"`
		DurationMs  int64  `json:"duration_ms"`
		TriggeredBy string `json:"triggered_by"`
		CreatedAt   string `json:"created_at"`
	}
	row := app.DB.QueryRow(
		`SELECT l.id, l.host_id, COALESCE(h.name,''), l.command, l.stdout, l.stderr,
		 l.exit_code, l.duration_ms, l.triggered_by, l.created_at
		 FROM execution_logs l LEFT JOIN hosts h ON h.id = l.host_id
		 WHERE l.id = ?`, id,
	)
	err := row.Scan(&log.ID, &log.HostID, &log.HostName, &log.Command,
		&log.Stdout, &log.Stderr, &log.ExitCode, &log.DurationMs,
		&log.TriggeredBy, &log.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, fmt.Sprintf("日志不存在: %s", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, log)
}
