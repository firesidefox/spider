package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".spider", "data")
	}
	return "/var/lib/spider"
}

// Config 是 Spider 的全局配置。
type Config struct {
	DataDir  string      `yaml:"data_dir" json:"-"`
	LogLevel string      `yaml:"log_level" json:"-"`
	SSH      SSHConfig   `yaml:"ssh"`
	SSE      SSEConfig   `yaml:"sse"`
	Auth     AuthConfig  `yaml:"auth"`
	Agent    AgentConfig `yaml:"agent"`
	Log      LogConfig   `yaml:"log"`
}

// RuleConfig 是单条权限规则配置。
type RuleConfig struct {
	Pattern     string `yaml:"pattern" json:"pattern"`
	Level       string `yaml:"level" json:"level"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// CompactionConfig 是会话上下文压缩配置。
type CompactionConfig struct {
	ThresholdTokens  int `yaml:"threshold_tokens"`   // 0 = 自动用模型表
	RecentTurns      int `yaml:"recent_turns"`        // 默认 20
	MaxSummaryTokens int `yaml:"max_summary_tokens"`  // 默认 4000
}

// AgentConfig 是 Agent 执行权限相关配置。
type AgentConfig struct {
	PermissionMode  string           `yaml:"permission_mode"`            // ask | auto | plan | readonly，默认 ask
	ApprovalTimeout int              `yaml:"approval_timeout"`           // 审批超时秒数，默认 300
	MaxTurns        int              `yaml:"max_turns"`                  // 单次 agent run 最大 LLM 轮次，默认 10000
	Rules           []RuleConfig     `yaml:"rules,omitempty" json:"rules,omitempty"` // 自定义权限规则
	Compaction      CompactionConfig `yaml:"compaction"`
}

// AuthConfig 是认证相关配置。
type AuthConfig struct {
	Enabled bool `yaml:"enabled"` // 默认 false
}

// LogConfig 是日志相关配置。
type LogConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	Stderr     bool   `yaml:"stderr"`
}

// SSEConfig 是 MCP SSE server 相关配置。
type SSEConfig struct {
	Addr    string `yaml:"addr"`     // 监听地址，默认 :8000
	BaseURL string `yaml:"base_url"` // 对外暴露的 URL，例如 http://localhost:8000
}

// SSHConfig 是 SSH 相关配置。
type SSHConfig struct {
	DefaultTimeout int    `yaml:"default_timeout_seconds"` // 默认命令超时（秒）
	PoolTTL        int    `yaml:"pool_ttl_seconds"`        // 连接池 TTL
	MaxPoolSize    int    `yaml:"max_pool_size"`
	NoProxy        string `yaml:"no_proxy,omitempty"` // 逗号分隔的直连地址/CIDR，绕过系统代理
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		DataDir:  defaultDataDir(),
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
		Agent: AgentConfig{
			PermissionMode:  "ask",
			ApprovalTimeout: 300,
			MaxTurns:        10000,
			Compaction: CompactionConfig{
				ThresholdTokens:  0,
				RecentTurns:      20,
				MaxSummaryTokens: 4000,
			},
		},
		Log: LogConfig{
			Level:      "info",
			Format:     "json",
			MaxSizeMB:  100,
			MaxBackups: 7,
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
