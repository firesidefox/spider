package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	sshpkg "github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

func listHosts(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	hosts, err := app.HostStore.List(tag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if hosts == nil {
		hosts = []*models.Host{}
	}
	writeJSON(w, http.StatusOK, hosts)
}

func addHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.AddHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h, err := app.HostStore.Add(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func getHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	h, err := app.HostStore.GetByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "host not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func updateHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdateHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h, err := app.HostStore.Update(id, &req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "host not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func deleteHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.HostStore.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "host not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

func pingHost(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	sshFace, err := app.AccessFaceStore.GetSSHFaceForHost(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": "无 SSH 操作面"})
		return
	}
	latency, err := sshpkg.CheckConnectivity(sshFace, app.AccessFaceStore, app.SSHKeyStore)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": true, "latency_ms": latency.Milliseconds()})
}

// Access face handlers

func listAccessFaces(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	faces, err := app.AccessFaceStore.ListByHost(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if faces == nil {
		faces = []*models.AccessFace{}
	}
	writeJSON(w, http.StatusOK, faces)
}

func addAccessFace(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	var req models.AddAccessFaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	f, err := app.AccessFaceStore.Add(hostID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func updateAccessFace(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID, faceID string) {
	existing, err := app.AccessFaceStore.GetByID(faceID)
	if err != nil || existing == nil || existing.HostID != hostID {
		writeError(w, http.StatusNotFound, "access face not found")
		return
	}
	var req models.UpdateAccessFaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	f, err := app.AccessFaceStore.Update(faceID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func deleteAccessFace(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID, faceID string) {
	existing, err := app.AccessFaceStore.GetByID(faceID)
	if err != nil || existing == nil || existing.HostID != hostID {
		writeError(w, http.StatusNotFound, "access face not found")
		return
	}
	if err := app.AccessFaceStore.Delete(faceID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "access face not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// Fingerprint handler

func getFingerprint(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	fp, err := app.FingerprintStore.Get(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fp == nil {
		writeError(w, http.StatusNotFound, "fingerprint not found")
		return
	}
	writeJSON(w, http.StatusOK, fp)
}

// Memory handlers

func listMemories(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	mems, err := app.MemoryStore.ListByHost(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if mems == nil {
		mems = []*models.Memory{}
	}
	writeJSON(w, http.StatusOK, mems)
}

func addMemory(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string) {
	var req models.AddMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// set created_by from auth context if not provided
	if req.CreatedBy == "" {
		req.CreatedBy = "user"
	}
	m, err := app.MemoryStore.Add(hostID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func deleteMemory(app *mcppkg.App, w http.ResponseWriter, r *http.Request, hostID string, memID int) {
	if err := app.MemoryStore.Delete(hostID, memID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "memory not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// parseMemID parses a string memory ID to int, returns -1 on failure.
func parseMemID(s string) int {
	id, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return id
}
