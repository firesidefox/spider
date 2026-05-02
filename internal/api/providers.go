package api

import (
	"encoding/json"
	"net/http"

	"github.com/spiderai/spider/internal/llm"
	"github.com/spiderai/spider/internal/models"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

type providerResponse struct {
	models.Provider
	Models []*models.ProviderModel `json:"models"`
}

func validProviderType(t string) bool {
	return t == "claude" || t == "openai"
}

func buildProviderResponse(app *mcppkg.App, p *models.Provider) (*providerResponse, error) {
	ms, err := app.ProviderStore.ListModels(p.ID)
	if err != nil {
		return nil, err
	}
	if ms == nil {
		ms = []*models.ProviderModel{}
	}
	return &providerResponse{Provider: *p, Models: ms}, nil
}

func createProvider(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if !validProviderType(req.Type) {
		writeError(w, http.StatusBadRequest, "type 必须为 claude 或 openai")
		return
	}
	p, err := app.ProviderStore.Create(req.Name, req.Type, req.APIKey, req.BaseURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Auto-fetch models
	apiKey, _ := app.ProviderStore.DecryptAPIKey(p)
	fetchedModels, err := llm.ListModels(p.Type, apiKey, p.BaseURL)
	if err == nil && len(fetchedModels) > 0 {
		_ = app.ProviderStore.SaveModels(p.ID, fetchedModels)
		_ = app.ProviderStore.SetSelectedModel(p.ID, fetchedModels[0].ID)
	}
	// Auto-activate if first provider
	count, _ := app.ProviderStore.CountAll()
	if count == 1 {
		_ = app.ProviderStore.Activate(p.ID)
	}
	p, err = app.ProviderStore.GetByID(p.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pr, err := buildProviderResponse(app, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, pr)
}

func listProviders(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	providers, err := app.ProviderStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]*providerResponse, 0, len(providers))
	for _, p := range providers {
		pr, err := buildProviderResponse(app, p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, pr)
	}
	writeJSON(w, http.StatusOK, result)
}

func updateProvider(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Name    *string `json:"name"`
		Type    *string `json:"type"`
		APIKey  *string `json:"api_key"`
		BaseURL *string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Type != nil && !validProviderType(*req.Type) {
		writeError(w, http.StatusBadRequest, "type 必须为 claude 或 openai")
		return
	}
	p, err := app.ProviderStore.Update(id, req.Name, req.Type, req.APIKey, req.BaseURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pr, err := buildProviderResponse(app, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

func deleteProvider(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, id string) {
	if err := app.ProviderStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func refreshModels(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, id string) {
	p, err := app.ProviderStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	apiKey, err := app.ProviderStore.DecryptAPIKey(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	fetchedModels, err := llm.ListModels(p.Type, apiKey, p.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := app.ProviderStore.SaveModels(id, fetchedModels); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p.SelectedModel == "" && len(fetchedModels) > 0 {
		_ = app.ProviderStore.SetSelectedModel(id, fetchedModels[0].ID)
	}
	ms, err := app.ProviderStore.ListModels(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ms == nil {
		ms = []*models.ProviderModel{}
	}
	writeJSON(w, http.StatusOK, ms)
}

func activateProvider(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, id string) {
	if err := app.ProviderStore.Activate(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p, err := app.ProviderStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p.SelectedModel == "" {
		ms, _ := app.ProviderStore.ListModels(id)
		if len(ms) > 0 {
			_ = app.ProviderStore.SetSelectedModel(id, ms[0].ModelID)
			p, _ = app.ProviderStore.GetByID(id)
		}
	}
	pr, err := buildProviderResponse(app, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

func setProviderModel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if err := app.ProviderStore.SetSelectedModel(id, req.Model); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func listProviderModels(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, id string) {
	ms, err := app.ProviderStore.ListModels(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ms == nil {
		ms = []*models.ProviderModel{}
	}
	writeJSON(w, http.StatusOK, ms)
}

// setActiveModel handles PUT /api/v1/providers/active — activates a provider and sets its model.
func setActiveModel(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.ProviderID == "" {
		writeError(w, http.StatusBadRequest, "provider_id 不能为空")
		return
	}
	if err := app.ProviderStore.Activate(req.ProviderID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Model != "" {
		if err := app.ProviderStore.SetSelectedModel(req.ProviderID, req.Model); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	p, err := app.ProviderStore.GetByID(req.ProviderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pr, err := buildProviderResponse(app, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pr)
}