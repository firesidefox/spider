package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spiderai/spider/internal/config"
)

// NewMCPCmd 返回 mcp 子命令组。
func NewMCPCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "管理 Claude Code MCP 注册",
	}
	cmd.AddCommand(
		newMCPRegisterCmd(cfg),
		newMCPUnregisterCmd(),
		newMCPStatusCmd(),
	)
	return cmd
}

// claudeSettings 是 ~/.claude/settings.json 的部分结构。
type claudeSettings struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func loadSettings(path string) (*claudeSettings, error) {
	s := &claudeSettings{MCPServers: map[string]mcpServerEntry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	// 先解析为 map[string]any 以保留未知字段，再单独处理 mcpServers
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 settings.json 失败: %w", err)
	}
	if v, ok := raw["mcpServers"]; ok {
		if err := json.Unmarshal(v, &s.MCPServers); err != nil {
			return nil, fmt.Errorf("解析 mcpServers 失败: %w", err)
		}
	}
	return s, nil
}

func saveSettings(path string, s *claudeSettings) error {
	// 读取原始文件，合并 mcpServers 字段，保留其他字段
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("解析 settings.json 失败: %w", err)
		}
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	mcpJSON, err := json.Marshal(s.MCPServers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = mcpJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0600)
}

func newMCPRegisterCmd(cfg *config.Config) *cobra.Command {
	var name string
	var url string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "将 Spider MCP server 注册到 Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				url = cfg.SSE.BaseURL + "/sse"
			}
			path, err := settingsPath()
			if err != nil {
				return err
			}
			s, err := loadSettings(path)
			if err != nil {
				return err
			}
			s.MCPServers[name] = mcpServerEntry{Type: "sse", URL: url}
			if err := saveSettings(path, s); err != nil {
				return fmt.Errorf("写入 settings.json 失败: %w", err)
			}
			fmt.Printf("已注册: %s -> %s\n", name, url)
			fmt.Printf("配置文件: %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "spider", "MCP server 名称")
	cmd.Flags().StringVar(&url, "url", "", "SSE URL（默认使用配置中的 base_url）")
	return cmd
}

func newMCPUnregisterCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "unregister",
		Short: "从 Claude Code 移除 Spider MCP 注册",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := settingsPath()
			if err != nil {
				return err
			}
			s, err := loadSettings(path)
			if err != nil {
				return err
			}
			if _, ok := s.MCPServers[name]; !ok {
				return fmt.Errorf("未找到注册项: %s", name)
			}
			delete(s.MCPServers, name)
			if err := saveSettings(path, s); err != nil {
				return fmt.Errorf("写入 settings.json 失败: %w", err)
			}
			fmt.Printf("已移除: %s\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "spider", "MCP server 名称")
	return cmd
}

func newMCPStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "查看当前 Claude Code MCP 注册状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := settingsPath()
			if err != nil {
				return err
			}
			s, err := loadSettings(path)
			if err != nil {
				return err
			}
			if len(s.MCPServers) == 0 {
				fmt.Println("未注册任何 MCP server")
				return nil
			}
			data, _ := json.MarshalIndent(s.MCPServers, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
