package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 是 Spider 的全局配置。
type Config struct {
	DataDir  string    `yaml:"data_dir"` // SQLite 文件、master.key 等存放目录
	LogLevel string    `yaml:"log_level"`
	SSH      SSHConfig `yaml:"ssh"`
	SSE      SSEConfig `yaml:"sse"`
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
	home, _ := os.UserHomeDir()
	return &Config{
		DataDir:  filepath.Join(home, ".spider"),
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

// Load 从文件加载配置，文件不存在时返回默认配置。
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// 优先级 2：config.yaml
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".spider", "config.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		fmt.Fprintf(os.Stderr, "config: %s not found, using defaults\n", path)
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 优先级 1：环境变量（最高）
	if v := os.Getenv("SPIDER_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	return cfg, nil
}

// EnsureDataDir 确保数据目录存在。
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0700)
}
