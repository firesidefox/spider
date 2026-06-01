package api

import (
	"errors"
	"net/http"
	"strconv"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/scheduler"
	"github.com/spiderai/spider/internal/store"
)

func listTasks(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	tasks, err := app.TaskStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func getTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	task, err := app.TaskStore.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func triggerTask(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string, executor *scheduler.Executor) {
	_, err := app.TaskStore.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	run, err := executor.Execute(r.Context(), id)
	if err != nil {
		if errors.Is(err, scheduler.ErrAlreadyRunning) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": run.ID, "status": "started"})
}

func listTaskRuns(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	runs, err := app.TaskRunStore.ListByTaskID(id, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func registerTaskRoutes(mux *http.ServeMux, d routeDeps) {
	app := d.app
	// Task automation API — reuse app.Executor (created in main.go)
	mux.HandleFunc("/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTasks(app, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/tasks/"):]
		id := rest
		sub := ""
		if idx := indexOf(rest, '/'); idx >= 0 {
			id = rest[:idx]
			sub = rest[idx+1:]
		}
		switch sub {
		case "":
			if r.Method == http.MethodGet {
				getTask(app, w, r, id)
				return
			}
		case "trigger":
			if r.Method == http.MethodPost {
				d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					triggerTask(app, w, r, id, app.Executor)
				})).ServeHTTP(w, r)
				return
			}
		case "runs":
			if r.Method == http.MethodGet {
				listTaskRuns(app, w, r, id)
				return
			}
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
}
