package api

import (
	"net/http"

	"github.com/spiderai/spider/internal/llm"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

// listProviderModels handles GET /api/v1/providers/{id}/models.
func listProviderModels(app *mcppkg.App, w http.ResponseWriter, r *http.Request, providerID string) {
	provider := app.Config.Model.GetProvider(providerID)
	if provider == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	models, err := llm.ListModels(provider.Type, provider.ResolveAPIKey(), provider.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models)
}
