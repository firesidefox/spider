package api

import (
	"encoding/json"
	"net/http"

	authmw "github.com/spiderai/spider/internal/auth"
	"github.com/spiderai/spider/internal/models"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

// NewRouter 注册所有 /api/v1 路由，返回 http.Handler。
func NewRouter(app *mcppkg.App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listHosts(app, w, r)
		case http.MethodPost:
			addHost(app, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/hosts/", func(w http.ResponseWriter, r *http.Request) {
		// /api/v1/hosts/:id  or  /api/v1/hosts/:id/ping
		path := r.URL.Path
		// strip prefix "/api/v1/hosts/"
		rest := path[len("/api/v1/hosts/"):]
		if idx := indexOf(rest, '/'); idx >= 0 {
			id := rest[:idx]
			action := rest[idx+1:]
			if action == "ping" && r.Method == http.MethodPost {
				pingHost(app, w, r, id)
				return
			}
			http.NotFound(w, r)
			return
		}
		id := rest
		switch r.Method {
		case http.MethodGet:
			getHost(app, w, r, id)
		case http.MethodPut:
			updateHost(app, w, r, id)
		case http.MethodDelete:
			deleteHost(app, w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			execCommand(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/exec/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			streamCommand(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/exec/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			execBatch(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			listLogs(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/logs/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/logs/"):]
		if r.Method == http.MethodGet {
			getLog(app, w, r, id)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getSettings(app, w, r)
		case http.MethodPut:
			updateSettings(app, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("GET /api/v1/install/skills.tar.gz", SkillsTarGzHandler(app.Config.DataDir))
	mux.HandleFunc("GET /api/v1/skills", listSkillsHandler(app.Config.DataDir))
	mux.HandleFunc("PUT /api/v1/skills/{name}", uploadSkillHandler(app.Config.DataDir))
	mux.HandleFunc("DELETE /api/v1/skills/{name}", deleteSkillHandler(app.Config.DataDir))
	mux.HandleFunc("GET /api/v1/me", meHandler(app))

	// Phase 2: 用户管理（admin only）
	adminOnly := authmw.RequireRole(models.RoleAdmin)

	mux.Handle("/api/v1/users", adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listUsersHandler(app)(w, r)
		case http.MethodPost:
			createUserHandler(app)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	mux.HandleFunc("/api/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/users/"):]
		switch r.Method {
		case http.MethodPut:
			adminOnly(updateUserHandler(app, id)).ServeHTTP(w, r)
		case http.MethodDelete:
			adminOnly(deleteUserHandler(app, id)).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Phase 2: API Token 管理
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTokensHandler(app)(w, r)
		case http.MethodPost:
			createTokenHandler(app)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/tokens/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/tokens/"):]
		if r.Method == http.MethodDelete {
			deleteTokenHandler(app, id)(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Auth middleware wraps the inner mux; login/logout are exposed without auth.
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
	return outer
}

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
