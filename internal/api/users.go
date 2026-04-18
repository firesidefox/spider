package api

import (
	"encoding/json"
	"net/http"
	"strings"

	authmw "github.com/spiderai/spider/internal/auth"
	"github.com/spiderai/spider/internal/models"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func listUsersHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := app.UserStore.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		infos := make([]*models.UserInfo, len(users))
		for i, u := range users {
			infos[i] = u.ToInfo()
		}
		writeJSON(w, http.StatusOK, infos)
	}
}

func createUserHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string      `json:"username"`
			Password string      `json:"password"`
			Role     models.Role `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "username and password required")
			return
		}
		if req.Role != models.RoleAdmin && req.Role != models.RoleOperator && req.Role != models.RoleViewer {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
		user, err := app.UserStore.Create(req.Username, req.Password, req.Role)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				writeError(w, http.StatusConflict, "username already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user.ToInfo())
	}
}

func updateUserHandler(app *mcppkg.App, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uc := authmw.GetUser(r.Context())
		var req struct {
			Role     *models.Role `json:"role"`
			Enabled  *bool        `json:"enabled"`
			Password *string      `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if uc != nil && uc.UserID == id && (req.Role != nil || req.Enabled != nil) {
			writeError(w, http.StatusForbidden, "cannot modify own role or enabled status")
			return
		}
		user, err := app.UserStore.Update(id, req.Role, req.Enabled, req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, user.ToInfo())
	}
}

func deleteUserHandler(app *mcppkg.App, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uc := authmw.GetUser(r.Context())
		if uc != nil && uc.UserID == id {
			writeError(w, http.StatusForbidden, "cannot delete yourself")
			return
		}
		if err := app.UserStore.Delete(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
