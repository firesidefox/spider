package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	promclient "github.com/spiderai/spider/internal/prometheus"
)

// --- Sources ---

func listPrometheusSources(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	list, err := app.PrometheusSourceStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func addPrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.AddPrometheusSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		writeError(w, http.StatusBadRequest, "base_url required")
		return
	}
	src, err := app.PrometheusSourceStore.Add(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

func getPrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	src, err := app.PrometheusSourceStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func updatePrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdatePrometheusSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := app.PrometheusSourceStore.Update(id, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func deletePrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.PrometheusSourceStore.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func testPrometheusConnection(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	src, err := app.PrometheusSourceStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	pwd, tok, err := app.PrometheusSourceStore.DecryptCredentials(src)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt error")
		return
	}
	c := promclient.NewClient(src.BaseURL, string(src.AuthType), src.Username, pwd, tok, src.TimeoutSeconds, src.SkipTLSVerify)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	latency, err := c.TestConnection(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "latency_ms": latency})
}

// --- Bindings ---

func listPrometheusBindings(app *mcppkg.App, w http.ResponseWriter, r *http.Request, sourceID string) {
	list, err := app.PrometheusBindingStore.ListBySource(sourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func addPrometheusBinding(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.AddPrometheusBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	b, err := app.PrometheusBindingStore.Add(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func deletePrometheusBinding(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.PrometheusBindingStore.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
