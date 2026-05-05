package api

import (
	"encoding/json"
	"net/http"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/store"
)

type ragConfigResponse struct {
	Type      string `json:"type"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeySet bool   `json:"api_key_set"`
}

func getRagConfig(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	cfg, err := app.RagConfigStore.Get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusOK, ragConfigResponse{Type: "openai"})
		return
	}
	writeJSON(w, http.StatusOK, ragConfigResponse{
		Type:      cfg.Type,
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		APIKeySet: cfg.APIKey != "",
	})
}

func putRagConfig(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Type == "" {
		req.Type = "openai"
	}
	cfg := &store.RagConfig{
		Type:    req.Type,
		BaseURL: req.BaseURL,
		Model:   req.Model,
		APIKey:  req.APIKey,
	}
	if err := app.RagConfigStore.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	getRagConfig(app, w, r)
}

func validateRagConfig(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	if req.Type == "" {
		req.Type = "openai"
	}
	embedder, err := rag.NewEmbedder(req.Type, req.APIKey, req.Model, req.BaseURL, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := embedder.Embed(r.Context(), "test"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "embedding request failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
