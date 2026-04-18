package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewMCPCmd 返回 mcp 子命令组。url 是 Spider 服务地址，用作注册默认值。
func NewMCPCmd(url *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "管理 MCP 注册（支持 claude / opencode / codex）",
	}
	cmd.AddCommand(
		newMCPRegisterCmd(url),
		newMCPUnregisterCmd(),
		newMCPStatusCmd(),
	)
	return cmd
}

// ── register ──────────────────────────────────────────────────────────────────

func newMCPRegisterCmd(url *string) *cobra.Command {
	var name, mcpURL, target string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "将 Spider MCP server 注册到 AI 工具（claude/opencode/codex）",
		RunE: func(cmd *cobra.Command, args []string) error {
			if mcpURL == "" {
				mcpURL = *url + "/mcp"
			}
			switch target {
			case "claude":
				return registerClaude(name, mcpURL)
			case "opencode":
				return registerOpencode(name, mcpURL)
			case "codex":
				return registerCodex(name, mcpURL)
			default:
				return fmt.Errorf("不支持的工具: %s（可选: claude, opencode, codex）", target)
			}
		},
	}
	cmd.Flags().StringVar(&name, "name", "spider", "MCP server 名称")
	cmd.Flags().StringVar(&mcpURL, "url", "", "MCP URL（默认使用 --url/mcp）")
	cmd.Flags().StringVar(&target, "tool", "claude", "目标工具: claude | opencode | codex")
	return cmd
}

// ── unregister ────────────────────────────────────────────────────────────────

func newMCPUnregisterCmd() *cobra.Command {
	var name, target string
	cmd := &cobra.Command{
		Use:   "unregister",
		Short: "从 AI 工具移除 Spider MCP 注册",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch target {
			case "claude":
				return unregisterClaude(name)
			case "opencode":
				return unregisterOpencode(name)
			case "codex":
				return unregisterCodex(name)
			default:
				return fmt.Errorf("不支持的工具: %s（可选: claude, opencode, codex）", target)
			}
		},
	}
	cmd.Flags().StringVar(&name, "name", "spider", "MCP server 名称")
	cmd.Flags().StringVar(&target, "tool", "claude", "目标工具: claude | opencode | codex")
	return cmd
}

// ── status ────────────────────────────────────────────────────────────────────

func newMCPStatusCmd() *cobra.Command {
	var target string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "查看 MCP 注册状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch target {
			case "claude":
				return statusClaude()
			case "opencode":
				return statusOpencode()
			case "codex":
				return statusCodex()
			case "all":
				fmt.Println("=== Claude Code ===")
				_ = statusClaude()
				fmt.Println("\n=== OpenCode ===")
				_ = statusOpencode()
				fmt.Println("\n=== Codex ===")
				_ = statusCodex()
				return nil
			default:
				return fmt.Errorf("不支持的工具: %s（可选: claude, opencode, codex, all）", target)
			}
		},
	}
	cmd.Flags().StringVar(&target, "tool", "all", "目标工具: claude | opencode | codex | all")
	return cmd
}

// ── Claude Code (~/.claude.json 全局 mcpServers) ─────────────────────────────

type claudeMCPEntry struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func claudeJSONPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude.json"), nil
}

func loadClaudeMCPServers(path string) (map[string]claudeMCPEntry, map[string]json.RawMessage, error) {
	servers := map[string]claudeMCPEntry{}
	raw := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return servers, raw, nil
		}
		return nil, nil, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("解析 .claude.json 失败: %w", err)
	}
	if v, ok := raw["mcpServers"]; ok {
		_ = json.Unmarshal(v, &servers)
	}
	return servers, raw, nil
}

func saveClaudeMCPServers(path string, servers map[string]claudeMCPEntry, raw map[string]json.RawMessage) error {
	b, _ := json.Marshal(servers)
	raw["mcpServers"] = b
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0600)
}

func registerClaude(name, url string) error {
	path, err := claudeJSONPath()
	if err != nil {
		return err
	}
	servers, raw, err := loadClaudeMCPServers(path)
	if err != nil {
		return err
	}
	servers[name] = claudeMCPEntry{Type: "http", URL: url}
	if err := saveClaudeMCPServers(path, servers, raw); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	fmt.Printf("已注册到 Claude Code（全局）: %s -> %s\n配置文件: %s\n", name, url, path)
	return nil
}

func unregisterClaude(name string) error {
	path, err := claudeJSONPath()
	if err != nil {
		return err
	}
	servers, raw, err := loadClaudeMCPServers(path)
	if err != nil {
		return err
	}
	if _, ok := servers[name]; !ok {
		return fmt.Errorf("未找到注册项: %s", name)
	}
	delete(servers, name)
	if err := saveClaudeMCPServers(path, servers, raw); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	fmt.Printf("已从 Claude Code 移除: %s\n", name)
	return nil
}

func statusClaude() error {
	path, err := claudeJSONPath()
	if err != nil {
		return err
	}
	servers, _, err := loadClaudeMCPServers(path)
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		fmt.Println("未注册任何 MCP server")
		return nil
	}
	data, _ := json.MarshalIndent(servers, "", "  ")
	fmt.Println(string(data))
	return nil
}

// ── OpenCode (~/.opencode/config.json) ───────────────────────────────────────

type opencodeConfig struct {
	MCP map[string]opencodeMCPEntry `json:"mcp"`
}

type opencodeMCPEntry struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func opencodeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".opencode", "config.json"), nil
}

func loadOpencodeConfig(path string) (*opencodeConfig, error) {
	s := &opencodeConfig{MCP: map[string]opencodeMCPEntry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 config.json 失败: %w", err)
	}
	if v, ok := raw["mcp"]; ok {
		_ = json.Unmarshal(v, &s.MCP)
	}
	return s, nil
}

func saveOpencodeConfig(path string, s *opencodeConfig) error {
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(s.MCP)
	raw["mcp"] = b
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0600)
}

func registerOpencode(name, url string) error {
	path, err := opencodeSettingsPath()
	if err != nil {
		return err
	}
	s, err := loadOpencodeConfig(path)
	if err != nil {
		return err
	}
	s.MCP[name] = opencodeMCPEntry{Type: "remote", URL: url}
	if err := saveOpencodeConfig(path, s); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	fmt.Printf("已注册到 OpenCode: %s -> %s\n配置文件: %s\n", name, url, path)
	return nil
}

func unregisterOpencode(name string) error {
	path, err := opencodeSettingsPath()
	if err != nil {
		return err
	}
	s, err := loadOpencodeConfig(path)
	if err != nil {
		return err
	}
	if _, ok := s.MCP[name]; !ok {
		return fmt.Errorf("未找到注册项: %s", name)
	}
	delete(s.MCP, name)
	if err := saveOpencodeConfig(path, s); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	fmt.Printf("已从 OpenCode 移除: %s\n", name)
	return nil
}

func statusOpencode() error {
	path, err := opencodeSettingsPath()
	if err != nil {
		return err
	}
	s, err := loadOpencodeConfig(path)
	if err != nil {
		return err
	}
	if len(s.MCP) == 0 {
		fmt.Println("未注册任何 MCP server")
		return nil
	}
	data, _ := json.MarshalIndent(s.MCP, "", "  ")
	fmt.Println(string(data))
	return nil
}

// ── Codex (~/.codex/config.toml) ─────────────────────────────────────────────
// Codex 只支持 stdio，不支持 SSE。注册时写入 stdio 包装命令。

func codexSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func registerCodex(name, url string) error {
	path, err := codexSettingsPath()
	if err != nil {
		return err
	}
	fmt.Printf("注意: Codex 不支持 SSE，将跳过 URL %s\n", url)
	fmt.Printf("Codex 只支持 stdio 模式，请手动在 %s 中添加：\n\n", path)
	fmt.Printf("[mcp_servers.%s]\ncommand = \"/path/to/spider\"\nargs = []\n\n", name)
	fmt.Println("其中 spider 二进制需以 stdio 模式运行（当前版本为 SSE 模式，暂不支持）。")
	return nil
}

func unregisterCodex(name string) error {
	path, err := codexSettingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("配置文件不存在: %s", path)
		}
		return err
	}
	// 简单删除 [mcp_servers.<name>] 段落
	content := string(data)
	section := fmt.Sprintf("[mcp_servers.%s]", name)
	start := indexOf(content, section)
	if start < 0 {
		return fmt.Errorf("未找到注册项: %s", name)
	}
	// 找到下一个 [ 或文件末尾
	end := indexOf(content[start+len(section):], "\n[")
	var newContent string
	if end < 0 {
		newContent = content[:start]
	} else {
		newContent = content[:start] + content[start+len(section)+end+1:]
	}
	if err := os.WriteFile(path, []byte(newContent), 0600); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	fmt.Printf("已从 Codex 移除: %s\n", name)
	return nil
}

func statusCodex() error {
	path, err := codexSettingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("配置文件不存在")
			return nil
		}
		return err
	}
	content := string(data)
	if indexOf(content, "[mcp_servers.") < 0 {
		fmt.Println("未注册任何 MCP server")
		return nil
	}
	fmt.Println(content)
	return nil
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
