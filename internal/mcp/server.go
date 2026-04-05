package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/store"
	sshpkg "github.com/spiderai/spider/internal/ssh"
)

// App 聚合所有依赖，供 MCP tool handler 使用。
type App struct {
	HostStore *store.HostStore
	LogStore  *store.LogStore
	Pool      *sshpkg.Pool
	Config    *config.Config
	DB        *sql.DB
}

// newMCPServer 创建并注册工具的 MCP server。
func newMCPServer(app *App) *server.MCPServer {
	s := server.NewMCPServer(
		"spider",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	registerTools(s, app)
	return s
}

// NewHTTPHandler 返回 MCP Streamable HTTP server 的 http.Handler，供外部 mux 挂载于 /mcp。
func NewHTTPHandler(app *App) http.Handler {
	s := newMCPServer(app)
	return server.NewStreamableHTTPServer(s)
}

// Serve 以 Streamable HTTP 模式启动 MCP server（阻塞，支持优雅关闭）。
func Serve(app *App) error {
	s := newMCPServer(app)
	h := server.NewStreamableHTTPServer(s)

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "Spider MCP server 启动，监听 %s\n", app.Config.SSE.Addr)
		errCh <- h.Start(app.Config.SSE.Addr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return h.Shutdown(ctx)
	}
}

// registerTools 注册所有 MCP 工具。
func registerTools(s *server.MCPServer, app *App) {
	// list_hosts
	s.AddTool(mcpgo.NewTool("list_hosts",
		mcpgo.WithDescription("列出所有被管理的主机，可按 tag 过滤"),
		mcpgo.WithString("tag", mcpgo.Description("按标签过滤，例如 prod、web")),
	), makeListHosts(app))

	// add_host
	s.AddTool(mcpgo.NewTool("add_host",
		mcpgo.WithDescription("添加一台新的被管理主机"),
		mcpgo.WithString("name", mcpgo.Required(), mcpgo.Description("主机唯一名称")),
		mcpgo.WithString("ip", mcpgo.Required(), mcpgo.Description("主机 IP 地址")),
		mcpgo.WithNumber("port", mcpgo.Description("SSH 端口，默认 22")),
		mcpgo.WithString("username", mcpgo.Required(), mcpgo.Description("SSH 登录用户名")),
		mcpgo.WithString("auth_type", mcpgo.Required(), mcpgo.Description("认证类型: password | key | key_password")),
		mcpgo.WithString("credential", mcpgo.Required(), mcpgo.Description("密码明文 或 SSH 私钥内容（PEM 格式）")),
		mcpgo.WithString("passphrase", mcpgo.Description("私钥 passphrase（auth_type=key_password 时使用）")),
		mcpgo.WithString("proxy_host_id", mcpgo.Description("跳板机主机 ID（ProxyJump）")),
		mcpgo.WithString("tags", mcpgo.Description("逗号分隔的标签，例如 prod,web")),
	), makeAddHost(app))

	// remove_host
	s.AddTool(mcpgo.NewTool("remove_host",
		mcpgo.WithDescription("删除一台被管理的主机"),
		mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("主机 ID 或名称")),
	), makeRemoveHost(app))

	// update_host
	s.AddTool(mcpgo.NewTool("update_host",
		mcpgo.WithDescription("更新主机信息（所有字段可选）"),
		mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("主机 ID")),
		mcpgo.WithString("name", mcpgo.Description("新名称")),
		mcpgo.WithString("ip", mcpgo.Description("新 IP")),
		mcpgo.WithNumber("port", mcpgo.Description("新端口")),
		mcpgo.WithString("username", mcpgo.Description("新用户名")),
		mcpgo.WithString("auth_type", mcpgo.Description("新认证类型")),
		mcpgo.WithString("credential", mcpgo.Description("新凭据")),
		mcpgo.WithString("passphrase", mcpgo.Description("新 passphrase")),
		mcpgo.WithString("proxy_host_id", mcpgo.Description("新跳板机 ID")),
		mcpgo.WithString("tags", mcpgo.Description("新标签（逗号分隔）")),
	), makeUpdateHost(app))

	// execute_command
	s.AddTool(mcpgo.NewTool("execute_command",
		mcpgo.WithDescription("在指定主机上执行 Shell 命令"),
		mcpgo.WithString("host_id", mcpgo.Required(), mcpgo.Description("主机 ID 或名称")),
		mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("要执行的命令")),
		mcpgo.WithNumber("timeout_seconds", mcpgo.Description("超时秒数，默认 30")),
	), makeExecuteCommand(app))

	// execute_command_batch
	s.AddTool(mcpgo.NewTool("execute_command_batch",
		mcpgo.WithDescription("在多台主机上批量执行同一命令"),
		mcpgo.WithString("command", mcpgo.Required(), mcpgo.Description("要执行的命令")),
		mcpgo.WithString("host_ids", mcpgo.Description("逗号分隔的主机 ID 或名称列表")),
		mcpgo.WithString("tag", mcpgo.Description("按标签批量执行（与 host_ids 二选一）")),
		mcpgo.WithNumber("timeout_seconds", mcpgo.Description("超时秒数，默认 30")),
	), makeExecuteCommandBatch(app))

	// check_connectivity
	s.AddTool(mcpgo.NewTool("check_connectivity",
		mcpgo.WithDescription("测试与主机的 SSH 连通性"),
		mcpgo.WithString("host_id", mcpgo.Required(), mcpgo.Description("主机 ID 或名称")),
	), makeCheckConnectivity(app))

	// upload_file
	s.AddTool(mcpgo.NewTool("upload_file",
		mcpgo.WithDescription("将本地文件上传到远程主机"),
		mcpgo.WithString("host_id", mcpgo.Required(), mcpgo.Description("主机 ID 或名称")),
		mcpgo.WithString("local_path", mcpgo.Required(), mcpgo.Description("本地文件路径")),
		mcpgo.WithString("remote_path", mcpgo.Required(), mcpgo.Description("远程目标路径")),
	), makeUploadFile(app))

	// download_file
	s.AddTool(mcpgo.NewTool("download_file",
		mcpgo.WithDescription("从远程主机下载文件到本地"),
		mcpgo.WithString("host_id", mcpgo.Required(), mcpgo.Description("主机 ID 或名称")),
		mcpgo.WithString("remote_path", mcpgo.Required(), mcpgo.Description("远程文件路径")),
		mcpgo.WithString("local_path", mcpgo.Required(), mcpgo.Description("本地保存路径")),
	), makeDownloadFile(app))

	// get_execution_history
	s.AddTool(mcpgo.NewTool("get_execution_history",
		mcpgo.WithDescription("查询命令执行历史记录"),
		mcpgo.WithString("host_id", mcpgo.Description("按主机 ID 或名称过滤（可选）")),
		mcpgo.WithNumber("limit", mcpgo.Description("返回条数，默认 20")),
		mcpgo.WithNumber("offset", mcpgo.Description("分页偏移，默认 0")),
	), makeGetExecutionHistory(app))
}

// toolError 返回 MCP 错误响应。
func toolError(msg string) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultError(msg), nil
}

// toolText 返回 MCP 文本响应。
func toolText(text string) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultText(text), nil
}

// getTimeout 从参数中获取超时时间。
func getTimeout(args map[string]any, defaultCfg *config.Config) time.Duration {
	if v, ok := args["timeout_seconds"]; ok {
		if f, ok := v.(float64); ok && f > 0 {
			return time.Duration(f) * time.Second
		}
	}
	return time.Duration(defaultCfg.SSH.DefaultTimeout) * time.Second
}

// getString 从 args 中安全获取字符串。
func getString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getInt 从 args 中安全获取整数。
func getInt(args map[string]any, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultVal
}

// splitTags 将逗号分隔的标签字符串转为切片。
func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	var tags []string
	for _, t := range splitComma(s) {
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func splitComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	result = append(result, trimSpace(s[start:]))
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

