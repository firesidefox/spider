package api

import (
	"encoding/json"
	"net/http"

	authmw "github.com/spiderai/spider/internal/auth"
	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/rag"
)

// routeDeps holds the shared dependencies passed to every register*Routes function.
type routeDeps struct {
	app             *mcppkg.App
	adminOnly       func(http.Handler) http.Handler
	operatorOrAbove func(http.Handler) http.Handler
	kbEmbedder      rag.Embedder
}

// NewRouter 注册所有 /api/v1 路由，返回 http.Handler。
func NewRouter(app *mcppkg.App) http.Handler {
	mux := http.NewServeMux()

	var kbEmbedder rag.Embedder
	if cfg, err := app.RagConfigStore.Get(); err == nil && cfg != nil && cfg.Model != "" {
		if emb, err := rag.NewEmbedder(cfg.Type, cfg.APIKey, cfg.Model, cfg.BaseURL, 0); err == nil {
			kbEmbedder = emb
		}
	}

	deps := routeDeps{
		app:             app,
		adminOnly:       authmw.RequireRole(models.RoleAdmin),
		operatorOrAbove: authmw.RequireRole(models.RoleAdmin, models.RoleOperator),
		kbEmbedder:      kbEmbedder,
	}

	registerHostRoutes(mux, deps)
	registerChatRoutes(mux, deps)
	registerKnowledgeRoutes(mux, deps)
	registerTaskRoutes(mux, deps)
	registerAdminRoutes(mux, deps)
	registerSettingsRoutes(mux, deps)
	registerStreamRoutes(mux, deps)
	registerTopologyRoutes(mux, deps)
	registerPrometheusRoutes(mux, deps)

	authMW := authmw.AuthMiddleware(
		app.Config.Auth.Enabled,
		app.JWTManager,
		app.UserStore,
		app.TokenStore,
	)
	outer := http.NewServeMux()
	outer.HandleFunc("POST /api/v1/auth/login", loginHandler(app))
	outer.HandleFunc("POST /api/v1/auth/logout", logoutHandler(app))
	outer.Handle("/", authMW(mux))
	return logger.Middleware()(outer)
}


// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
