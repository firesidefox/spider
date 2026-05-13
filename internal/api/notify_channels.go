package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func listNotifyChannels(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	channels, err := app.NotifyChannelStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if channels == nil {
		channels = []*models.NotifyChannel{}
	}
	// Clear decrypted config before sending to client — credentials stay server-side.
	for _, ch := range channels {
		ch.Config = ""
	}
	writeJSON(w, http.StatusOK, channels)
}

type createNotifyChannelRequest struct {
	Name    string                    `json:"name"`
	Type    models.NotifyChannelType  `json:"type"`
	Config  string                    `json:"config"`
	Enabled *bool                     `json:"enabled"`
}

func createNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req createNotifyChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ch, err := app.NotifyChannelStore.Create(req.Name, req.Type, req.Config, enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ch.Config = ""
	writeJSON(w, http.StatusCreated, ch)
}

type updateNotifyChannelRequest struct {
	Name    string                   `json:"name"`
	Type    models.NotifyChannelType `json:"type"`
	Config  string                   `json:"config"`
	Enabled *bool                    `json:"enabled"`
}

func updateNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
	var req updateNotifyChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ch, err := app.NotifyChannelStore.Update(id, req.Name, req.Type, req.Config, req.Enabled)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "notify channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ch.Config = ""
	writeJSON(w, http.StatusOK, ch)
}

type toggleEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func toggleNotifyChannelEnabled(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
	var req toggleEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ch, err := app.NotifyChannelStore.ToggleEnabled(id, req.Enabled)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "notify channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ch.Config = ""
	writeJSON(w, http.StatusOK, ch)
}

func deleteNotifyChannel(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id int64) {
	if err := app.NotifyChannelStore.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "notify channel not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// parseChannelID parses a string channel ID to int64, returns -1 on failure.
func parseChannelID(s string) int64 {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return -1
	}
	return id
}
