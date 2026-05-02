package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/llm"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"gopkg.in/yaml.v3"
)

var validProviderTypes = map[string]bool{"claude": true, "openai": true}

func saveConfig(app *mcppkg.App) error {
	cfgPath := filepath.Join(app.Config.DataDir, "config.yaml")
	data, err := yaml.Marshal(app.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0600)
}

func maskedProvider(p config.ProviderConfig) config.ProviderConfig {
	p.APIKey = maskKey(p.APIKey)
	return p
}

func listProviders(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	app.ConfigMu.RLock()
	defer app.ConfigMu.RUnlock()
	type response struct {
		Providers      []config.ProviderConfig `json:"providers"`
		ActiveProvider string                  `json:"active_provider"`
		ActiveModel    string                  `json:"active_model"`
	}
	masked := make([]config.ProviderConfig, 0, len(app.Config.Model.Providers))
	for _, p := range app.Config.Model.Providers {
		masked = append(masked, maskedProvider(p))
	}
	writeJSON(w, 200, response{
		Providers:      masked,
		ActiveProvider: app.Config.Model.ActiveProvider,
		ActiveModel:    app.Config.Model.ActiveModel,
	})
}

func createProvider(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if !validProviderTypes[req.Type] {
		writeError(w, 400, "unsupported provider type: "+req.Type)
		return
	}
	app.ConfigMu.Lock()
	defer app.ConfigMu.Unlock()
	p := config.ProviderConfig{
		ID: uuid.New().String(), Name: req.Name,
		Type: req.Type, APIKey: req.APIKey, BaseURL: req.BaseURL,
	}
	app.Config.Model.Providers = append(app.Config.Model.Providers, p)
	if err := saveConfig(app); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, maskedProvider(p))
}

func updateProvider(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Name    *string `json:"name"`
		Type    *string `json:"type"`
		APIKey  *string `json:"api_key"`
		BaseURL *string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if req.Type != nil && !validProviderTypes[*req.Type] {
		writeError(w, 400, "unsupported provider type: "+*req.Type)
		return
	}
	app.ConfigMu.Lock()
	defer app.ConfigMu.Unlock()
	p := app.Config.Model.GetProvider(id)
	if p == nil {
		writeError(w, 404, "provider not found")
		return
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Type != nil {
		p.Type = *req.Type
	}
	if req.APIKey != nil && !strings.HasPrefix(*req.APIKey, maskedPrefix) {
		p.APIKey = *req.APIKey
	}
	if req.BaseURL != nil {
		p.BaseURL = *req.BaseURL
	}
	if err := saveConfig(app); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, maskedProvider(*p))
}

func deleteProvider(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, id string) {
	app.ConfigMu.Lock()
	defer app.ConfigMu.Unlock()
	providers := app.Config.Model.Providers
	found := false
	for i, p := range providers {
		if p.ID == id {
			app.Config.Model.Providers = append(providers[:i], providers[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		writeError(w, 404, "provider not found")
		return
	}
	if app.Config.Model.ActiveProvider == id {
		app.Config.Model.ActiveProvider = ""
		app.Config.Model.ActiveModel = ""
	}
	if err := saveConfig(app); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func setActiveModel(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if req.ProviderID == "" {
		writeError(w, 400, "provider_id is required")
		return
	}
	app.ConfigMu.Lock()
	defer app.ConfigMu.Unlock()
	if app.Config.Model.GetProvider(req.ProviderID) == nil {
		writeError(w, 404, "provider not found")
		return
	}
	app.Config.Model.ActiveProvider = req.ProviderID
	app.Config.Model.ActiveModel = req.Model
	if err := saveConfig(app); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{
		"active_provider": req.ProviderID,
		"active_model":    req.Model,
	})
}

func listProviderModels(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, providerID string) {
	app.ConfigMu.RLock()
	provider := app.Config.Model.GetProvider(providerID)
	if provider == nil {
		app.ConfigMu.RUnlock()
		writeError(w, 404, "provider not found")
		return
	}
	pType, apiKey, baseURL := provider.Type, provider.ResolveAPIKey(), provider.BaseURL
	app.ConfigMu.RUnlock()
	models, err := llm.ListModels(pType, apiKey, baseURL)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, models)
}
