package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"gopkg.in/yaml.v3"
)

type settingsResponse struct {
	SSEAddr    string `json:"sse_addr"`
	SSEBaseURL string `json:"sse_base_url"`
	SSHTimeout int    `json:"ssh_default_timeout_seconds"`
	SSHPoolTTL int    `json:"ssh_pool_ttl_seconds"`
	SSHMaxPool int    `json:"ssh_max_pool_size"`
}

func getSettings(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsResponse{
		SSEAddr:    app.Config.SSE.Addr,
		SSEBaseURL: app.Config.SSE.BaseURL,
		SSHTimeout: app.Config.SSH.DefaultTimeout,
		SSHPoolTTL: app.Config.SSH.PoolTTL,
		SSHMaxPool: app.Config.SSH.MaxPoolSize,
	})
}

func updateSettings(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req settingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}

	if req.SSEAddr != "" {
		app.Config.SSE.Addr = req.SSEAddr
	}
	if req.SSEBaseURL != "" {
		app.Config.SSE.BaseURL = req.SSEBaseURL
	}
	if req.SSHTimeout > 0 {
		app.Config.SSH.DefaultTimeout = req.SSHTimeout
	}
	if req.SSHPoolTTL > 0 {
		app.Config.SSH.PoolTTL = req.SSHPoolTTL
	}
	if req.SSHMaxPool > 0 {
		app.Config.SSH.MaxPoolSize = req.SSHMaxPool
	}

	cfgPath := filepath.Join(app.Config.DataDir, "config.yaml")
	data, err := yaml.Marshal(app.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "序列化配置失败: "+err.Error())
		return
	}
	if err := os.WriteFile(cfgPath, data, 0600); err != nil {
		writeError(w, http.StatusInternalServerError, "写入配置失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, settingsResponse{
		SSEAddr:    app.Config.SSE.Addr,
		SSEBaseURL: app.Config.SSE.BaseURL,
		SSHTimeout: app.Config.SSH.DefaultTimeout,
		SSHPoolTTL: app.Config.SSH.PoolTTL,
		SSHMaxPool: app.Config.SSH.MaxPoolSize,
	})
}
