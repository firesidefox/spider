package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 是 Spider 的全局配置。
type Config struct {
	DataDir string `yaml:"data_dir"` // SQLite 文件、master.key 等存放目录
	LogLevel string `yaml:"log_level"`
	SSH     SSHConfig `yaml:"ssh"`
}

// SSHConfig 是 SSH 相关配置。
type SSHConfig struct {
	DefaultTimeout  int `yaml:"default_timeout_seconds"` // 默认命令超时（秒）
	PoolTTL         int `yaml:"pool_ttl_seconds"`        // 连接池 TTL
	MaxPoolSize     int `yaml:"max_pool_size"`
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
	}
}

// Load 从文件加载配置，文件不存在时返回默认配置。
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".spider", "config.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	// 环境变量覆盖
	if v := os.Getenv("SPIDER_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	return cfg, nil
}

// EnsureDataDir 确保数据目录存在。
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0700)
}
