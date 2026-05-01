package api

import (
	"encoding/json"
	"net/http"
	"strings"

	authmw "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
)

func listSSHKeys(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	uc := authmw.GetUser(r.Context())
	keys, err := app.SSHKeyStore.ListByUser(uc.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	safe := make([]*models.SafeSSHKey, 0, len(keys))
	for _, k := range keys {
		safe = append(safe, k.Safe())
	}
	writeJSON(w, http.StatusOK, safe)
}

func addSSHKey(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	uc := authmw.GetUser(r.Context())
	var req models.AddSSHKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	key, err := app.SSHKeyStore.Add(uc.UserID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, key.Safe())
}

func getSSHKey(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	uc := authmw.GetUser(r.Context())
	key, err := app.SSHKeyStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if key.UserID != uc.UserID {
		writeError(w, http.StatusNotFound, "ssh key not found")
		return
	}
	writeJSON(w, http.StatusOK, key.Safe())
}

func deleteSSHKey(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	uc := authmw.GetUser(r.Context())
	err := app.SSHKeyStore.Delete(id, uc.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "CONFLICT") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}
