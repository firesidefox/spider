package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/spiderai/spider/internal/models"
	sshpkg "github.com/spiderai/spider/internal/ssh"
)

// makeListHosts 返回 list_hosts 的 handler。
func makeListHosts(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		tag := getString(req.Params.Arguments, "tag")
		hosts, err := app.HostStore.List(tag)
		if err != nil {
			return toolError(fmt.Sprintf("查询主机列表失败: %v", err))
		}
		safeHosts := make([]*models.SafeHost, len(hosts))
		for i, h := range hosts {
			safeHosts[i] = h.Safe()
		}
		data, _ := json.MarshalIndent(safeHosts, "", "  ")
		return toolText(string(data))
	}
}

// makeAddHost 返回 add_host 的 handler。
func makeAddHost(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		authType := models.AuthType(getString(args, "auth_type"))
		switch authType {
		case models.AuthPassword, models.AuthKey, models.AuthKeyPassword:
		default:
			return toolError(fmt.Sprintf("无效的 auth_type: %s，必须是 password | key | key_password", authType))
		}

		addReq := &models.AddHostRequest{
			Name:        getString(args, "name"),
			IP:          getString(args, "ip"),
			Port:        getInt(args, "port", 22),
			Username:    getString(args, "username"),
			AuthType:    authType,
			Credential:  getString(args, "credential"),
			Passphrase:  getString(args, "passphrase"),
			ProxyHostID: getString(args, "proxy_host_id"),
			Tags:        splitTags(getString(args, "tags")),
		}

		host, err := app.HostStore.Add(addReq)
		if err != nil {
			return toolError(fmt.Sprintf("添加主机失败: %v", err))
		}
		data, _ := json.MarshalIndent(host.Safe(), "", "  ")
		return toolText(fmt.Sprintf("主机添加成功:\n%s", string(data)))
	}
}

// makeRemoveHost 返回 remove_host 的 handler。
func makeRemoveHost(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		idOrName := getString(req.Params.Arguments, "id")
		if idOrName == "" {
			return toolError("id 不能为空")
		}
		host, err := app.HostStore.GetByIDOrName(idOrName)
		if err != nil {
			return toolError(fmt.Sprintf("主机不存在: %s", idOrName))
		}
		if err := app.HostStore.Delete(host.ID); err != nil {
			return toolError(fmt.Sprintf("删除主机失败: %v", err))
		}
		return toolText(fmt.Sprintf("主机 %s (%s) 已删除", host.Name, host.ID))
	}
}

// makeUpdateHost 返回 update_host 的 handler。
func makeUpdateHost(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		id := getString(args, "id")
		if id == "" {
			return toolError("id 不能为空")
		}

		host, err := app.HostStore.GetByIDOrName(id)
		if err != nil {
			return toolError(fmt.Sprintf("主机不存在: %s", id))
		}

		updateReq := &models.UpdateHostRequest{}
		if v := getString(args, "name"); v != "" {
			updateReq.Name = &v
		}
		if v := getString(args, "ip"); v != "" {
			updateReq.IP = &v
		}
		if v := getInt(args, "port", 0); v != 0 {
			updateReq.Port = &v
		}
		if v := getString(args, "username"); v != "" {
			updateReq.Username = &v
		}
		if v := getString(args, "auth_type"); v != "" {
			at := models.AuthType(v)
			updateReq.AuthType = &at
		}
		if v := getString(args, "credential"); v != "" {
			updateReq.Credential = &v
		}
		if v := getString(args, "passphrase"); v != "" {
			updateReq.Passphrase = &v
		}
		if v := getString(args, "proxy_host_id"); v != "" {
			updateReq.ProxyHostID = &v
		}
		if v := getString(args, "tags"); v != "" {
			updateReq.Tags = splitTags(v)
		}

		updated, err := app.HostStore.Update(host.ID, updateReq)
		if err != nil {
			return toolError(fmt.Sprintf("更新主机失败: %v", err))
		}
		data, _ := json.MarshalIndent(updated.Safe(), "", "  ")
		return toolText(fmt.Sprintf("主机更新成功:\n%s", string(data)))
	}
}

// makeExecuteCommand 返回 execute_command 的 handler。
func makeExecuteCommand(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		hostIDOrName := getString(args, "host_id")
		command := getString(args, "command")
		if hostIDOrName == "" || command == "" {
			return toolError("host_id 和 command 不能为空")
		}

		host, err := app.HostStore.GetByIDOrName(hostIDOrName)
		if err != nil {
			return toolError(fmt.Sprintf("主机不存在: %s", hostIDOrName))
		}

		timeout := getTimeout(args, app.Config)
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		client, err := app.Pool.Get(host, app.HostStore)
		if err != nil {
			return toolError(fmt.Sprintf("建立 SSH 连接失败: %v", err))
		}
		defer app.Pool.Release(host.ID)

		result, err := client.Execute(execCtx, command)
		if err != nil {
			return toolError(fmt.Sprintf("执行命令失败: %v", err))
		}

		// 记录审计日志
		_ = app.LogStore.Save(&models.ExecutionLog{
			HostID:      host.ID,
			Command:     command,
			Stdout:      result.Stdout,
			Stderr:      result.Stderr,
			ExitCode:    result.ExitCode,
			DurationMs:  result.Duration.Milliseconds(),
			TriggeredBy: "mcp",
		})

		output := map[string]any{
			"host":        host.Name,
			"command":     command,
			"stdout":      result.Stdout,
			"stderr":      result.Stderr,
			"exit_code":   result.ExitCode,
			"duration_ms": result.Duration.Milliseconds(),
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		return toolText(string(data))
	}
}

// makeExecuteCommandBatch 返回 execute_command_batch 的 handler。
func makeExecuteCommandBatch(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		command := getString(args, "command")
		if command == "" {
			return toolError("command 不能为空")
		}

		// 收集目标主机
		var hosts []*models.Host
		if tag := getString(args, "tag"); tag != "" {
			hs, err := app.HostStore.List(tag)
			if err != nil {
				return toolError(fmt.Sprintf("按标签查询主机失败: %v", err))
			}
			hosts = hs
		} else if hostIDs := getString(args, "host_ids"); hostIDs != "" {
			for _, id := range splitComma(hostIDs) {
				h, err := app.HostStore.GetByIDOrName(strings.TrimSpace(id))
				if err != nil {
					return toolError(fmt.Sprintf("主机不存在: %s", id))
				}
				hosts = append(hosts, h)
			}
		} else {
			return toolError("必须提供 host_ids 或 tag")
		}

		if len(hosts) == 0 {
			return toolText("没有匹配的主机")
		}

		timeout := getTimeout(args, app.Config)
		type hostResult struct {
			Host     string `json:"host"`
			Command  string `json:"command"`
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
			ExitCode int    `json:"exit_code"`
			DurationMs int64 `json:"duration_ms"`
			Error    string `json:"error,omitempty"`
		}

		results := make([]hostResult, len(hosts))
		// 并发执行
		type job struct {
			idx  int
			host *models.Host
		}
		jobs := make(chan job, len(hosts))
		for i, h := range hosts {
			jobs <- job{i, h}
		}
		close(jobs)

		done := make(chan struct{}, len(hosts))
		for range hosts {
			go func() {
				defer func() { done <- struct{}{} }()
				j := <-jobs
				execCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()

				client, err := app.Pool.Get(j.host, app.HostStore)
				if err != nil {
					results[j.idx] = hostResult{Host: j.host.Name, Command: command, Error: err.Error()}
					return
				}
				defer app.Pool.Release(j.host.ID)

				res, err := client.Execute(execCtx, command)
				if err != nil {
					results[j.idx] = hostResult{Host: j.host.Name, Command: command, Error: err.Error()}
					return
				}
				results[j.idx] = hostResult{
					Host: j.host.Name, Command: command,
					Stdout: res.Stdout, Stderr: res.Stderr,
					ExitCode: res.ExitCode, DurationMs: res.Duration.Milliseconds(),
				}
				_ = app.LogStore.Save(&models.ExecutionLog{
					HostID: j.host.ID, Command: command,
					Stdout: res.Stdout, Stderr: res.Stderr,
					ExitCode: res.ExitCode, DurationMs: res.Duration.Milliseconds(),
					TriggeredBy: "mcp",
				})
			}()
		}
		for range hosts {
			<-done
		}

		data, _ := json.MarshalIndent(results, "", "  ")
		return toolText(string(data))
	}
}

// makeCheckConnectivity 返回 check_connectivity 的 handler。
func makeCheckConnectivity(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		hostIDOrName := getString(req.Params.Arguments, "host_id")
		if hostIDOrName == "" {
			return toolError("host_id 不能为空")
		}
		host, err := app.HostStore.GetByIDOrName(hostIDOrName)
		if err != nil {
			return toolError(fmt.Sprintf("主机不存在: %s", hostIDOrName))
		}

		latency, err := sshpkg.CheckConnectivity(host, app.HostStore)
		result := map[string]any{
			"host":       host.Name,
			"connected":  err == nil,
			"latency_ms": latency.Milliseconds(),
		}
		if err != nil {
			result["error"] = err.Error()
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return toolText(string(data))
	}
}

// makeUploadFile 返回 upload_file 的 handler。
func makeUploadFile(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		hostIDOrName := getString(args, "host_id")
		localPath := getString(args, "local_path")
		remotePath := getString(args, "remote_path")
		if hostIDOrName == "" || localPath == "" || remotePath == "" {
			return toolError("host_id、local_path、remote_path 不能为空")
		}

		host, err := app.HostStore.GetByIDOrName(hostIDOrName)
		if err != nil {
			return toolError(fmt.Sprintf("主机不存在: %s", hostIDOrName))
		}

		client, err := app.Pool.Get(host, app.HostStore)
		if err != nil {
			return toolError(fmt.Sprintf("建立 SSH 连接失败: %v", err))
		}
		defer app.Pool.Release(host.ID)

		uploadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		if err := client.Upload(uploadCtx, localPath, remotePath); err != nil {
			return toolError(fmt.Sprintf("上传文件失败: %v", err))
		}
		return toolText(fmt.Sprintf("文件上传成功: %s -> %s:%s", localPath, host.Name, remotePath))
	}
}

// makeDownloadFile 返回 download_file 的 handler。
func makeDownloadFile(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		hostIDOrName := getString(args, "host_id")
		remotePath := getString(args, "remote_path")
		localPath := getString(args, "local_path")
		if hostIDOrName == "" || remotePath == "" || localPath == "" {
			return toolError("host_id、remote_path、local_path 不能为空")
		}

		host, err := app.HostStore.GetByIDOrName(hostIDOrName)
		if err != nil {
			return toolError(fmt.Sprintf("主机不存在: %s", hostIDOrName))
		}

		client, err := app.Pool.Get(host, app.HostStore)
		if err != nil {
			return toolError(fmt.Sprintf("建立 SSH 连接失败: %v", err))
		}
		defer app.Pool.Release(host.ID)

		downloadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		if err := client.Download(downloadCtx, remotePath, localPath); err != nil {
			return toolError(fmt.Sprintf("下载文件失败: %v", err))
		}
		return toolText(fmt.Sprintf("文件下载成功: %s:%s -> %s", host.Name, remotePath, localPath))
	}
}

// makeGetExecutionHistory 返回 get_execution_history 的 handler。
func makeGetExecutionHistory(app *App) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.Params.Arguments
		hostIDOrName := getString(args, "host_id")
		limit := getInt(args, "limit", 20)
		offset := getInt(args, "offset", 0)

		hostID := ""
		if hostIDOrName != "" {
			host, err := app.HostStore.GetByIDOrName(hostIDOrName)
			if err != nil {
				return toolError(fmt.Sprintf("主机不存在: %s", hostIDOrName))
			}
			hostID = host.ID
		}

		logs, err := app.LogStore.List(hostID, limit, offset)
		if err != nil {
			return toolError(fmt.Sprintf("查询执行历史失败: %v", err))
		}
		if logs == nil {
			logs = []*models.ExecutionLog{}
		}
		data, _ := json.MarshalIndent(logs, "", "  ")
		return toolText(string(data))
	}
}
