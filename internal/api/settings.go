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
	SSEAddr    string               `json:"sse_addr"`
	SSEBaseURL string               `json:"sse_base_url"`
	SSHTimeout int                  `json:"ssh_default_timeout_seconds"`
	SSHPoolTTL int                  `json:"ssh_pool_ttl_seconds"`
	SSHMaxPool int                  `json:"ssh_max_pool_size"`
	LLM        config.LLMConfig     `json:"llm"`
	Embedding  config.EmbeddingConfig `json:"embedding"`
}

const maskedPrefix = "****"

func maskKey(key string) string {
	if len(key) <= 4 {
		return key
	}
	return maskedPrefix + key[len(key)-4:]
}

func maskedLLMConfig(c config.LLMConfig) config.LLMConfig {
	masked := config.LLMConfig{Active: c.Active}
	for _, m := range c.Models {
		m.APIKey = maskKey(m.APIKey)
		masked.Models = append(masked.Models, m)
	}
	return masked
}

func maskedEmbeddingConfig(c config.EmbeddingConfig) config.EmbeddingConfig {
	masked := config.EmbeddingConfig{Active: c.Active}
	for _, m := range c.Models {
		m.APIKey = maskKey(m.APIKey)
		masked.Models = append(masked.Models, m)
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
		LLM:        maskedLLMConfig(app.Config.LLM),
		Embedding:  maskedEmbeddingConfig(app.Config.Embedding),
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
	if req.LLM.Active != "" {
		// Preserve existing API keys when the incoming value is a masked placeholder.
		for i := range req.LLM.Models {
			if strings.HasPrefix(req.LLM.Models[i].APIKey, maskedPrefix) {
				for _, existing := range app.Config.LLM.Models {
					if existing.ID == req.LLM.Models[i].ID {
						req.LLM.Models[i].APIKey = existing.APIKey
						break
					}
				}
			}
		}
		app.Config.LLM = req.LLM
	}
	if req.Embedding.Active != "" {
		// Preserve existing API keys when the incoming value is a masked placeholder.
		for i := range req.Embedding.Models {
			if strings.HasPrefix(req.Embedding.Models[i].APIKey, maskedPrefix) {
				for _, existing := range app.Config.Embedding.Models {
					if existing.ID == req.Embedding.Models[i].ID {
						req.Embedding.Models[i].APIKey = existing.APIKey
						break
					}
				}
			}
		}
		app.Config.Embedding = req.Embedding
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
