package api

import (
	"encoding/json"
	"net/http"
	"time"

	authmw "github.com/spiderai/spider/internal/auth"
	authpkg "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func listTokensHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uc := authmw.GetUser(r.Context())
		if uc == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		tokens, err := app.TokenStore.ListByUser(uc.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		infos := make([]any, len(tokens))
		for i, t := range tokens {
			infos[i] = t.ToInfo()
		}
		writeJSON(w, http.StatusOK, infos)
	}
}

func createTokenHandler(app *mcppkg.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uc := authmw.GetUser(r.Context())
		if uc == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req struct {
			Name      string  `json:"name"`
			ExpiresAt *string `json:"expires_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name required")
			return
		}
		plain, err := authpkg.Generate()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		hash := authpkg.Hash(plain)
		var expiresAt *time.Time
		if req.ExpiresAt != nil {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid expires_at format")
				return
			}
			expiresAt = &t
		}
		tok, err := app.TokenStore.Create(uc.UserID, req.Name, hash, expiresAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp := map[string]any{
			"id":         tok.ID,
			"name":       tok.Name,
			"token":      plain,
			"expires_at": tok.ExpiresAt,
			"created_at": tok.CreatedAt,
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func deleteTokenHandler(app *mcppkg.App, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uc := authmw.GetUser(r.Context())
		if uc == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		tokens, err := app.TokenStore.ListByUser(uc.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		owned := false
		for _, t := range tokens {
			if t.ID == id {
				owned = true
				break
			}
		}
		if !owned {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if err := app.TokenStore.Delete(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
