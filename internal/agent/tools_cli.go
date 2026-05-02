package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

type ExecuteCLITool struct {
	hosts   *store.HostStore
	sshPool *ssh.Pool
	logs    *store.LogStore
	sshKeys *store.SSHKeyStore
}

func NewExecuteCLITool(hosts *store.HostStore, sshPool *ssh.Pool, logs *store.LogStore, sshKeys *store.SSHKeyStore) *ExecuteCLITool {
	return &ExecuteCLITool{hosts: hosts, sshPool: sshPool, logs: logs, sshKeys: sshKeys}
}

func (t *ExecuteCLITool) Name() string { return "execute_cli" }

func (t *ExecuteCLITool) Description() string {
	return "Execute a CLI command on a remote host via SSH"
}

func (t *ExecuteCLITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_id":    map[string]any{"type": "string", "description": "Host ID"},
			"command":    map[string]any{"type": "string", "description": "CLI command to execute"},
			"risk_level": map[string]any{"type": "string", "enum": []string{"safe", "moderate", "dangerous"}},
		},
		"required": []string{"host_id", "command"},
	}
}

func (t *ExecuteCLITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	hostID, _ := input["host_id"].(string)
	command, _ := input["command"].(string)
	riskStr, _ := input["risk_level"].(string)

	if hostID == "" || command == "" {
		return &ToolResult{Content: "missing required fields: host_id, command", IsError: true, RiskLevel: RiskModerate}, nil
	}

	risk := RiskModerate
	switch RiskLevel(riskStr) {
	case RiskSafe:
		risk = RiskSafe
	case RiskDangerous:
		risk = RiskDangerous
	}

	h, err := t.hosts.GetByIDOrName(hostID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("host not found: %v", err), IsError: true, RiskLevel: risk}, nil
	}

	client, err := t.sshPool.Get(h, t.hosts, t.sshKeys)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("SSH connect failed: %v", err), IsError: true, RiskLevel: risk}, nil
	}
	defer t.sshPool.Release(h.ID)

	result, err := client.Execute(ctx, command)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("execute failed: %v", err), IsError: true, RiskLevel: risk}, nil
	}

	_ = t.logs.Save(&models.ExecutionLog{
		HostID:      h.ID,
		Command:     command,
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		DurationMs:  result.Duration.Milliseconds(),
		TriggeredBy: "agent",
	})

	out, err := json.Marshal(map[string]any{
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"exit_code":   result.ExitCode,
		"duration_ms": result.Duration.Milliseconds(),
	})
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("marshal error: %v", err), IsError: true, RiskLevel: risk}, nil
	}
	return &ToolResult{Content: string(out), RiskLevel: risk}, nil
}
