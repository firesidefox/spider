package api

import (
	"net/http"
	"strconv"
	"strings"
)

func registerAdminRoutes(mux *http.ServeMux, d routeDeps) {
	app := d.app
	adminOnly := d.adminOnly
	operatorOrAbove := d.operatorOrAbove

	mux.HandleFunc("GET /api/v1/me", meHandler(app))
	mux.HandleFunc("PUT /api/v1/me/password", changePasswordHandler(app))
	mux.HandleFunc("GET /api/v1/me/prefs", getUIPrefsHandler(app))
	mux.HandleFunc("PUT /api/v1/me/prefs", setUIPrefsHandler(app))

	mux.HandleFunc("/api/v1/me/ssh-keys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listSSHKeys(app, w, r)
		case http.MethodPost:
			addSSHKey(app, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/me/ssh-keys/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/me/ssh-keys/"):]
		switch r.Method {
		case http.MethodGet:
			getSSHKey(app, w, r, id)
		case http.MethodDelete:
			deleteSSHKey(app, w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Phase 2: 用户管理（admin only）
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

	mux.HandleFunc("/api/v1/approvals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				listApprovals(app, w, r)
			})).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/approvals/", func(w http.ResponseWriter, r *http.Request) {
		operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			approvalRouter(app, w, r)
		})).ServeHTTP(w, r)
	})

	// Permission rules API (admin only)
	mux.HandleFunc("/api/v1/permission/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listRules(app, w, r)
		case http.MethodPost:
			adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addRule(app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/permission/rules/", func(w http.ResponseWriter, r *http.Request) {
		idxStr := r.URL.Path[len("/api/v1/permission/rules/"):]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				updateRule(app, w, r, idx)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteRule(app, w, r, idx)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/permission/builtin-rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			listBuiltinRules(app, w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Notify channels API
	mux.HandleFunc("/api/v1/notify-channels", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listNotifyChannels(app, w, r)
		case http.MethodPost:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createNotifyChannel(app, w, r)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/notify-channels/", func(w http.ResponseWriter, r *http.Request) {
		rest := r.URL.Path[len("/api/v1/notify-channels/"):]
		// Handle /api/v1/notify-channels/{id}/enabled
		if strings.HasSuffix(rest, "/enabled") {
			idStr := strings.TrimSuffix(rest, "/enabled")
			id := parseChannelID(idStr)
			if id < 0 {
				http.NotFound(w, r)
				return
			}
			if r.Method == http.MethodPatch {
				operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					toggleNotifyChannelEnabled(app, w, r, id)
				})).ServeHTTP(w, r)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		id := parseChannelID(rest)
		if id < 0 {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				updateNotifyChannel(app, w, r, id)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleteNotifyChannel(app, w, r, id)
			})).ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle("/api/v1/log-level", adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getLogLevel(w, r)
		case http.MethodPut:
			setLogLevel(app, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
}
