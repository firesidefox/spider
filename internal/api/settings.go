package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/spiderai/spider/internal/logger"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/permission"
	"gopkg.in/yaml.v3"
)

type settingsResponse struct {
	SSEAddr                string `json:"sse_addr"`
	SSEBaseURL             string `json:"sse_base_url"`
	SSHTimeout             int    `json:"ssh_default_timeout_seconds"`
	SSHPoolTTL             int    `json:"ssh_pool_ttl_seconds"`
	SSHMaxPool             int    `json:"ssh_max_pool_size"`
	SSHNoProxy             string `json:"ssh_no_proxy"`
	PermissionMode         string `json:"permission_mode"`
	ApprovalTimeout        int    `json:"approval_timeout"`
	MaxTurns               int    `json:"max_turns"`
	CompactionThreshold    int    `json:"compaction_threshold_tokens"`
	CompactionRecentTurns  int    `json:"compaction_recent_turns"`
	CompactionMaxSummary   int    `json:"compaction_max_summary_tokens"`
}

func saveConfig(app *mcppkg.App) error {
	data, err := yaml.Marshal(app.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(app.ConfigPath, data, 0600)
}

func buildSettingsResponse(app *mcppkg.App) settingsResponse {
	return settingsResponse{
		SSEAddr:               app.Config.SSE.Addr,
		SSEBaseURL:            app.Config.SSE.BaseURL,
		SSHTimeout:            app.Config.SSH.DefaultTimeout,
		SSHPoolTTL:            app.Config.SSH.PoolTTL,
		SSHMaxPool:            app.Config.SSH.MaxPoolSize,
		SSHNoProxy:            app.Config.SSH.NoProxy,
		PermissionMode:        app.Config.Agent.PermissionMode,
		ApprovalTimeout:       app.Config.Agent.ApprovalTimeout,
		MaxTurns:              app.Config.Agent.MaxTurns,
		CompactionThreshold:   app.Config.Agent.Compaction.ThresholdTokens,
		CompactionRecentTurns: app.Config.Agent.Compaction.RecentTurns,
		CompactionMaxSummary:  app.Config.Agent.Compaction.MaxSummaryTokens,
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
	if req.MaxTurns > 0 {
		app.Config.Agent.MaxTurns = req.MaxTurns
	}
	if req.CompactionThreshold > 0 {
		app.Config.Agent.Compaction.ThresholdTokens = req.CompactionThreshold
	}
	if req.CompactionRecentTurns > 0 {
		app.Config.Agent.Compaction.RecentTurns = req.CompactionRecentTurns
	}
	if req.CompactionMaxSummary > 0 {
		app.Config.Agent.Compaction.MaxSummaryTokens = req.CompactionMaxSummary
	}
	if app.AgentFactory != nil {
		app.AgentFactory.MaxTurns = app.Config.Agent.MaxTurns
		app.AgentFactory.CompactionCfg = app.Config.Agent.Compaction
	}

	if err := saveConfig(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildSettingsResponse(app))
}

func getLogLevel(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"level":   logger.CurrentLevel(),
		"modules": logger.ModuleLevels(),
	})
}

func setLogLevel(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level  string `json:"level"`
		Module string `json:"module"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Module != "" {
		if req.Level == "inherit" {
			logger.ClearModuleLevel(req.Module)
			logger.FromContext(r.Context()).Info().
				Str("module", req.Module).Str("level", "cleared").Msg("module log level changed")
		} else {
			if err := logger.SetModuleLevel(req.Module, req.Level); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			logger.FromContext(r.Context()).Info().
				Str("module", req.Module).Str("level", req.Level).Msg("module log level changed")
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"level":   logger.CurrentLevel(),
			"modules": logger.ModuleLevels(),
		})
		return
	}

	if !logger.IsValidLevel(req.Level) {
		writeError(w, http.StatusBadRequest, "level must be debug, info, warn, or error")
		return
	}
	logger.SetLevel(req.Level)
	app.Config.Log.Level = req.Level
	if err := saveConfig(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logger.FromContext(r.Context()).Info().Str("level", req.Level).Msg("log level changed")
	writeJSON(w, http.StatusOK, map[string]any{
		"level":   req.Level,
		"modules": logger.ModuleLevels(),
	})
}
