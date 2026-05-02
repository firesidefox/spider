package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 是 Spider 的全局配置。
type Config struct {
	DataDir   string          `yaml:"data_dir"` // SQLite 文件、master.key 等存放目录
	LogLevel  string          `yaml:"log_level"`
	SSH       SSHConfig       `yaml:"ssh"`
	SSE       SSEConfig       `yaml:"sse"`
	Auth      AuthConfig      `yaml:"auth"`
	LLM       LLMConfig       `yaml:"llm"`
	Embedding EmbeddingConfig `yaml:"embedding"`
}

// AuthConfig 是认证相关配置。
type AuthConfig struct {
	Enabled bool `yaml:"enabled"` // 默认 false
}

// LLMModelConfig 是单个 LLM 模型的配置。
type LLMModelConfig struct {
	ID        string `yaml:"id"`
	Provider  string `yaml:"provider"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

// LLMConfig 是 LLM 多模型配置。
type LLMConfig struct {
	Active string           `yaml:"active"`
	Models []LLMModelConfig `yaml:"models"`
}

// EmbeddingModelConfig 是单个 Embedding 模型的配置。
type EmbeddingModelConfig struct {
	ID         string `yaml:"id"`
	Provider   string `yaml:"provider"`
	APIKey     string `yaml:"api_key"`
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions"`
}

// EmbeddingConfig 是 Embedding 多模型配置。
type EmbeddingConfig struct {
	Active string                 `yaml:"active"`
	Models []EmbeddingModelConfig `yaml:"models"`
}

// SSEConfig 是 MCP SSE server 相关配置。
type SSEConfig struct {
	Addr    string `yaml:"addr"`     // 监听地址，默认 :8000
	BaseURL string `yaml:"base_url"` // 对外暴露的 URL，例如 http://localhost:8000
}

// SSHConfig 是 SSH 相关配置。
type SSHConfig struct {
	DefaultTimeout int `yaml:"default_timeout_seconds"` // 默认命令超时（秒）
	PoolTTL        int `yaml:"pool_ttl_seconds"`        // 连接池 TTL
	MaxPoolSize    int `yaml:"max_pool_size"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		DataDir:  "/var/lib/spider",
		LogLevel: "info",
		SSH: SSHConfig{
			DefaultTimeout: 30,
			PoolTTL:        300,
			MaxPoolSize:    50,
		},
		SSE: SSEConfig{
			Addr:    ":8000",
			BaseURL: "http://localhost:8000",
		},
	}
}

// Load 从文件加载配置，文件不存在时静默使用默认配置。
// path 为空时自动推导为 DataDir/config.yaml；推导路径读取失败一律静默忽略。
// path 显式指定时，文件不存在则静默忽略，其他错误则返回。
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	derived := path == ""
	if derived {
		path = filepath.Join(cfg.DataDir, "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if derived || os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, nil
}

// EnsureDataDir 确保数据目录存在。
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0700)
}

// ActiveModel 返回当前激活的 LLM 模型配置，未找到时返回 nil。
func (c *LLMConfig) ActiveModel() *LLMModelConfig {
	for i := range c.Models {
		if c.Models[i].ID == c.Active {
			return &c.Models[i]
		}
	}
	return nil
}

// ActiveModel 返回当前激活的 Embedding 模型配置，未找到时返回 nil。
func (c *EmbeddingConfig) ActiveModel() *EmbeddingModelConfig {
	for i := range c.Models {
		if c.Models[i].ID == c.Active {
			return &c.Models[i]
		}
	}
	return nil
}

// ResolveAPIKey 优先从环境变量 SPIDER_LLM_APIKEY_<ID> 读取 API Key。
func (m *LLMModelConfig) ResolveAPIKey() string {
	envKey := os.Getenv("SPIDER_LLM_APIKEY_" + m.ID)
	if envKey != "" {
		return envKey
	}
	return m.APIKey
}

// ResolveAPIKey 优先从环境变量 SPIDER_EMBEDDING_APIKEY_<ID> 读取 API Key。
func (m *EmbeddingModelConfig) ResolveAPIKey() string {
	envKey := os.Getenv("SPIDER_EMBEDDING_APIKEY_" + m.ID)
	if envKey != "" {
		return envKey
	}
	return m.APIKey
}
