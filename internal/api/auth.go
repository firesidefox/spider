package api

import (
	"encoding/json"
	"net/http"
	"time"

	authmw "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func loginHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		user, err := app.UserStore.Authenticate(req.Username, req.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if !user.Enabled {
			writeError(w, http.StatusForbidden, "account disabled")
			return
		}
		token, err := app.JWTManager.Sign(user.ID, string(user.Role))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to sign token")
			return
		}
		_ = app.UserStore.UpdateLastLogin(user.ID)
		writeJSON(w, http.StatusOK, map[string]any{
			"token":      token,
			"expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			"user":       user.ToInfo(),
		})
	}
}

func logoutHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 简化实现：直接返回 ok，黑名单由 JWT 过期自然失效
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func meHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uc := authmw.GetUser(r.Context())
		if uc == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if uc.UserID == "anonymous" {
			writeJSON(w, http.StatusOK, map[string]any{
				"id": "anonymous", "username": "admin",
				"role": "admin", "enabled": true,
			})
			return
		}
		user, err := app.UserStore.GetByID(uc.UserID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, user.ToInfo())
	}
}
