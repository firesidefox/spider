package api

import (
	"net/http"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

// NOTE: This file will be rewritten in Task 5 to use the DB-backed ProviderStore.
// For now all handlers return 501 so the package compiles.

func listProviders(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	writeError(w, 501, "not implemented")
}

func createProvider(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	writeError(w, 501, "not implemented")
}

func updateProvider(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, _ string) {
	writeError(w, 501, "not implemented")
}

func deleteProvider(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, _ string) {
	writeError(w, 501, "not implemented")
}

func setActiveModel(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	writeError(w, 501, "not implemented")
}

func listProviderModels(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, _ string) {
	writeError(w, 501, "not implemented")
}
