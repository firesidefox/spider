package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/spiderai/spider/internal/llm"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/rag"
	"github.com/spiderai/spider/internal/store"
)

type ragConfigResponse struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	BaseURL      string   `json:"base_url"`
	Model        string   `json:"model"`
	APIKeySet    bool     `json:"api_key_set"`
	CachedModels []string `json:"cached_models"`
	ValidatedAt  string   `json:"validated_at"`
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
		Name:         cfg.Name,
		Type:         cfg.Type,
		BaseURL:      cfg.BaseURL,
		Model:        cfg.Model,
		APIKeySet:    cfg.APIKey != "",
		CachedModels: cfg.CachedModels,
		ValidatedAt:  cfg.ValidatedAt,
	})
}

func putRagConfig(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		BaseURL      string   `json:"base_url"`
		Model        string   `json:"model"`
		APIKey       string   `json:"api_key"`
		CachedModels []string `json:"cached_models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Type == "" {
		req.Type = "openai"
	}
	// 连接参数变化时清除模型列表和验证状态
	cachedModels := req.CachedModels
	clearValidation := false
	if existing, err := app.RagConfigStore.Get(); err == nil && existing != nil {
		existingKey := existing.APIKey
		newKey := req.APIKey
		if newKey == "" {
			newKey = existingKey // 空 key 表示保留原值
		}
		if existing.Type != req.Type || existing.BaseURL != req.BaseURL || existingKey != newKey {
			cachedModels = nil
			clearValidation = true
		}
	}
	cfg := &store.RagConfig{
		Name:             req.Name,
		Type:             req.Type,
		BaseURL:          req.BaseURL,
		Model:            req.Model,
		APIKey:           req.APIKey,
		CachedModels:     cachedModels,
		ClearValidatedAt: clearValidation,
	}
	if err := app.RagConfigStore.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	app.InvalidateRagStore()
	writeJSON(w, http.StatusOK, ragConfigResponse{
		Name:         cfg.Name,
		Type:         cfg.Type,
		BaseURL:      cfg.BaseURL,
		Model:        cfg.Model,
		APIKeySet:    cfg.APIKey != "",
		CachedModels: cfg.CachedModels,
		ValidatedAt:  cfg.ValidatedAt,
	})
}

func resolveRagAPIKey(app *mcppkg.App, provided string) string {
	if provided != "" {
		return provided
	}
	if cfg, err := app.RagConfigStore.Get(); err == nil && cfg != nil {
		return cfg.APIKey
	}
	return ""
}

func listRagModels(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Type == "" {
		req.Type = "openai"
	}
	models, err := llm.ListModels(req.Type, resolveRagAPIKey(app, req.APIKey), req.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models)
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
	apiKey := resolveRagAPIKey(app, req.APIKey)
	embedder, err := rag.NewEmbedder(req.Type, apiKey, req.Model, req.BaseURL, 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := embedder.Embed(r.Context(), "test"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "embedding request failed: "+err.Error())
		return
	}
	// 验证通过，持久化验证时间
	_ = app.RagConfigStore.SetValidatedAt(time.Now().UTC().Format(time.RFC3339))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
