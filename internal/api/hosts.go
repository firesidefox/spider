package api

import (
	"encoding/json"
	"net/http"

	authmw "github.com/spiderai/spider/internal/auth"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	sshpkg "github.com/spiderai/spider/internal/ssh"
)

func enrichSafeHost(app *mcppkg.App, h *models.Host) *models.SafeHost {
	safe := h.Safe()
	if safe.SSHKeyID != "" && app.SSHKeyStore != nil {
		if key, err := app.SSHKeyStore.GetByID(safe.SSHKeyID); err == nil {
			safe.SSHKeyName = key.Name
		}
	}
	return safe
}

func listHosts(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	hosts, err := app.HostStore.List(tag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	safe := make([]*models.SafeHost, 0, len(hosts))
	for _, h := range hosts {
		safe = append(safe, enrichSafeHost(app, h))
	}
	writeJSON(w, http.StatusOK, safe)
}

func addHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.AddHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.SSHKeyID != "" && req.Credential != "" {
		writeError(w, http.StatusBadRequest, "ssh_key_id 和 credential 不能同时提供")
		return
	}
	if req.SSHKeyID != "" {
		uc := authmw.GetUser(r.Context())
		key, err := app.SSHKeyStore.GetByID(req.SSHKeyID)
		if err != nil || key.UserID != uc.UserID {
			writeError(w, http.StatusBadRequest, "ssh_key_id 无效或不属于当前用户")
			return
		}
	}
	h, err := app.HostStore.Add(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, enrichSafeHost(app, h))
}

func getHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	h, err := app.HostStore.GetByIDOrName(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, enrichSafeHost(app, h))
}

func updateHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	h, err := app.HostStore.GetByIDOrName(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req models.UpdateHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.SSHKeyID != nil && *req.SSHKeyID != "" {
		uc := authmw.GetUser(r.Context())
		key, err := app.SSHKeyStore.GetByID(*req.SSHKeyID)
		if err != nil || key.UserID != uc.UserID {
			writeError(w, http.StatusBadRequest, "ssh_key_id 无效或不属于当前用户")
			return
		}
	}
	updated, err := app.HostStore.Update(h.ID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, enrichSafeHost(app, updated))
}

func deleteHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	h, err := app.HostStore.GetByIDOrName(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := app.HostStore.Delete(h.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

func pingHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	h, err := app.HostStore.GetByIDOrName(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	latency, err := sshpkg.CheckConnectivity(h, app.HostStore, app.SSHKeyStore)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": true, "latency_ms": latency.Milliseconds()})
}
