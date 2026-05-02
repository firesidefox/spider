package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spiderai/spider/internal/config"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"gopkg.in/yaml.v3"
)

type settingsResponse struct {
	SSEAddr    string             `json:"sse_addr"`
	SSEBaseURL string             `json:"sse_base_url"`
	SSHTimeout int                `json:"ssh_default_timeout_seconds"`
	SSHPoolTTL int                `json:"ssh_pool_ttl_seconds"`
	SSHMaxPool int                `json:"ssh_max_pool_size"`
	Model      config.ModelConfig `json:"model"`
}

const maskedPrefix = "****"

func maskKey(key string) string {
	if len(key) <= 4 {
		return key
	}
	return maskedPrefix + key[len(key)-4:]
}

func maskedModelConfig(c config.ModelConfig) config.ModelConfig {
	masked := config.ModelConfig{
		ActiveProvider: c.ActiveProvider,
		ActiveModel:    c.ActiveModel,
	}
	for _, p := range c.Providers {
		p.APIKey = maskKey(p.APIKey)
		masked.Providers = append(masked.Providers, p)
	}
	return masked
}

func buildSettingsResponse(app *mcppkg.App) settingsResponse {
	return settingsResponse{
		SSEAddr:    app.Config.SSE.Addr,
		SSEBaseURL: app.Config.SSE.BaseURL,
		SSHTimeout: app.Config.SSH.DefaultTimeout,
		SSHPoolTTL: app.Config.SSH.PoolTTL,
		SSHMaxPool: app.Config.SSH.MaxPoolSize,
		Model:      maskedModelConfig(app.Config.Model),
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
	if len(req.Model.Providers) > 0 || req.Model.ActiveProvider != "" || req.Model.ActiveModel != "" {
		for i := range req.Model.Providers {
			if strings.HasPrefix(req.Model.Providers[i].APIKey, maskedPrefix) {
				if existing := app.Config.Model.GetProvider(req.Model.Providers[i].ID); existing != nil {
					req.Model.Providers[i].APIKey = existing.APIKey
				}
			}
		}
		app.Config.Model = req.Model
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

	writeJSON(w, http.StatusOK, buildSettingsResponse(app))
}
