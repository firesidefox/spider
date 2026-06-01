package api

import (
	"net/http"
)

func registerSettingsRoutes(mux *http.ServeMux, d routeDeps) {
	app := d.app
	adminOnly := d.adminOnly
	operatorOrAbove := d.operatorOrAbove

	mux.HandleFunc("/api/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getSettings(app, w, r)
		case http.MethodPut:
			adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				updateSettings(app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("GET /api/v1/install/skills.tar.gz", SkillsTarGzHandler(app.Config.DataDir))
	mux.HandleFunc("GET /api/v1/skills", listSkillsHandler(app.Config.DataDir))
	mux.HandleFunc("GET /api/v1/skills/{source}/{name...}", getSkillBySourceHandler(app.Config.DataDir))
	mux.HandleFunc("PUT /api/v1/skills/custom/{name...}", uploadCustomSkillHandler(app.Config.DataDir))
	mux.HandleFunc("DELETE /api/v1/skills/custom/{name...}", deleteCustomSkillHandler(app.Config.DataDir))

	mux.HandleFunc("/api/v1/providers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				listProviders(app, w, r)
			})).ServeHTTP(w, r)
		case http.MethodPost:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createProvider(app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/providers/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/providers/"):]
		if idx := indexOf(rest, '/'); idx >= 0 {
			id := rest[:idx]
			action := rest[idx+1:]
			switch {
			case action == "refresh" && r.Method == http.MethodPost:
				operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					refreshModels(app, w, r, id)
				})).ServeHTTP(w, r)
			case action == "activate" && r.Method == http.MethodPut:
				operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					activateProvider(app, w, r, id)
				})).ServeHTTP(w, r)
			case action == "model" && r.Method == http.MethodPut:
				operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					setProviderModel(app, w, r, id)
				})).ServeHTTP(w, r)
			case action == "models" && r.Method == http.MethodGet:
				listProviderModels(app, w, r, id)
			default:
				http.NotFound(w, r)
			}
			return
		}
		id := rest
		switch r.Method {
		case http.MethodPut:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				updateProvider(app, w, r, id)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteProvider(app, w, r, id)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
