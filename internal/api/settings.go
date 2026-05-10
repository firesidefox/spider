package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spiderai/spider/internal/logger"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/permission"
	"gopkg.in/yaml.v3"
)

type settingsResponse struct {
	SSEAddr         string `json:"sse_addr"`
	SSEBaseURL      string `json:"sse_base_url"`
	SSHTimeout      int    `json:"ssh_default_timeout_seconds"`
	SSHPoolTTL      int    `json:"ssh_pool_ttl_seconds"`
	SSHMaxPool      int    `json:"ssh_max_pool_size"`
	SSHNoProxy      string `json:"ssh_no_proxy"`
	PermissionMode  string `json:"permission_mode"`
	ApprovalTimeout int    `json:"approval_timeout"`
}

const maskedPrefix = "****"

func maskKey(key string) string {
	if len(key) <= 4 {
		return key
	}
	return maskedPrefix + key[len(key)-4:]
}

func saveConfig(app *mcppkg.App) error {
	cfgPath := filepath.Join(app.Config.DataDir, "config.yaml")
	data, err := yaml.Marshal(app.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0600)
}

func buildSettingsResponse(app *mcppkg.App) settingsResponse {
	return settingsResponse{
		SSEAddr:         app.Config.SSE.Addr,
		SSEBaseURL:      app.Config.SSE.BaseURL,
		SSHTimeout:      app.Config.SSH.DefaultTimeout,
		SSHPoolTTL:      app.Config.SSH.PoolTTL,
		SSHMaxPool:      app.Config.SSH.MaxPoolSize,
		SSHNoProxy:      app.Config.SSH.NoProxy,
		PermissionMode:  app.Config.Agent.PermissionMode,
		ApprovalTimeout: app.Config.Agent.ApprovalTimeout,
	}
}

func getSettings(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, buildSettingsResponse(app))
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
	// no_proxy 允许设为空字符串（清除），所以不用 != "" 判断，直接覆盖
	app.Config.SSH.NoProxy = req.SSHNoProxy
	if req.PermissionMode != "" {
		app.Config.Agent.PermissionMode = req.PermissionMode
		app.PermissionMode = permission.Mode(req.PermissionMode)
	}
	if req.ApprovalTimeout > 0 {
		app.Config.Agent.ApprovalTimeout = req.ApprovalTimeout
	}

	if err := saveConfig(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildSettingsResponse(app))
}

func getLogLevel(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"level": logger.CurrentLevel()})
}

func setLogLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !logger.IsValidLevel(req.Level) {
		writeError(w, http.StatusBadRequest, "level must be debug, info, or error")
		return
	}
	logger.SetLevel(req.Level)
	logger.FromContext(r.Context()).Info().Str("level", req.Level).Msg("log level changed")
	writeJSON(w, http.StatusOK, map[string]string{"level": req.Level})
}
