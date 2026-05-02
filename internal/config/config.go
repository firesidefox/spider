package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 是 Spider 的全局配置。
type Config struct {
	DataDir  string     `yaml:"data_dir" json:"-"`
	LogLevel string     `yaml:"log_level" json:"-"`
	SSH      SSHConfig  `yaml:"ssh"`
	SSE      SSEConfig  `yaml:"sse"`
	Auth     AuthConfig `yaml:"auth"`
}

// AuthConfig 是认证相关配置。
type AuthConfig struct {
	Enabled bool `yaml:"enabled"` // 默认 false
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
