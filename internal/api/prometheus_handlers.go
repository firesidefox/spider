package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	promclient "github.com/spiderai/spider/internal/prometheus"
	"github.com/spiderai/spider/internal/store"
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
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func deletePrometheusSource(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.PrometheusSourceStore.Delete(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
	if strings.TrimSpace(req.SourceID) == "" {
		writeError(w, http.StatusBadRequest, "source_id required")
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

func registerPrometheusRoutes(mux *http.ServeMux, d routeDeps) {
	app := d.app
	// Prometheus Data Sources
	mux.HandleFunc("/api/v1/prometheus/sources", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listPrometheusSources(app, w, r)
		case http.MethodPost:
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addPrometheusSource(app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/prometheus/sources/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/prometheus/sources/"):]
		id := rest
		sub := ""
		if idx := indexOf(rest, '/'); idx >= 0 {
			id = rest[:idx]
			sub = rest[idx+1:]
		}
		switch sub {
		case "":
			switch r.Method {
			case http.MethodGet:
				getPrometheusSource(app, w, r, id)
			case http.MethodPut:
				d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					updatePrometheusSource(app, w, r, id)
				})).ServeHTTP(w, r)
			case http.MethodDelete:
				d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					deletePrometheusSource(app, w, r, id)
				})).ServeHTTP(w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		case "test":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			testPrometheusConnection(app, w, r, id)
		case "bindings":
			switch r.Method {
			case http.MethodGet:
				listPrometheusBindings(app, w, r, id)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	mux.HandleFunc("/api/v1/prometheus/bindings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addPrometheusBinding(app, w, r)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/prometheus/bindings/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/prometheus/bindings/"):]
		if r.Method == http.MethodDelete {
			d.operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deletePrometheusBinding(app, w, r, id)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
}
