package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

type BatchExecuteTool struct {
	hosts   *store.HostStore
	sshPool *ssh.Pool
	logs    *store.LogStore
	sshKeys *store.SSHKeyStore
}

func NewBatchExecuteTool(hosts *store.HostStore, sshPool *ssh.Pool, logs *store.LogStore, sshKeys *store.SSHKeyStore) *BatchExecuteTool {
	return &BatchExecuteTool{hosts: hosts, sshPool: sshPool, logs: logs, sshKeys: sshKeys}
}

func (t *BatchExecuteTool) Name() string        { return "batch_execute" }
func (t *BatchExecuteTool) Description() string { return "Execute a CLI command on multiple hosts in parallel" }

func (t *BatchExecuteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of host IDs"},
			"tag":      map[string]any{"type": "string", "description": "Host tag to select hosts (alternative to host_ids)"},
			"command":  map[string]any{"type": "string", "description": "CLI command to execute on all hosts"},
		},
		"required": []string{"command"},
	}
}

type batchHostResult struct {
	HostID     string `json:"host_id"`
	HostName   string `json:"host_name"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func (t *BatchExecuteTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	command, _ := input["command"].(string)
	if command == "" {
		return &ToolResult{Content: "command is required", IsError: true, RiskLevel: RiskDangerous}, nil
	}

	var hostList []*models.Host
	if tag, ok := input["tag"].(string); ok && tag != "" {
		hosts, err := t.hosts.List(tag)
		if err != nil {
			return &ToolResult{Content: fmt.Sprintf("failed to list hosts by tag: %v", err), IsError: true, RiskLevel: RiskDangerous}, nil
		}
		hostList = hosts
	} else if ids, ok := input["host_ids"].([]any); ok {
		for _, id := range ids {
			if sid, ok := id.(string); ok {
				h, err := t.hosts.GetByID(sid)
				if err != nil {
					return &ToolResult{Content: fmt.Sprintf("host %s not found: %v", sid, err), IsError: true, RiskLevel: RiskDangerous}, nil
				}
				hostList = append(hostList, h)
			}
		}
	}
	if len(hostList) == 0 {
		return &ToolResult{Content: "no hosts selected", IsError: true, RiskLevel: RiskDangerous}, nil
	}

	results := make([]batchHostResult, len(hostList))
	var wg sync.WaitGroup
	var mu sync.Mutex
	_ = mu

	for i, host := range hostList {
		wg.Add(1)
		go func(idx int, h *models.Host) {
			defer wg.Done()
			r := batchHostResult{HostID: h.ID, HostName: h.Name}
			client, err := t.sshPool.Get(h, t.hosts, t.sshKeys)
			if err != nil {
				r.Error = err.Error()
				results[idx] = r
				return
			}
			defer t.sshPool.Release(h.ID)
			res, err := client.Execute(ctx, command)
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Stdout = res.Stdout
				r.Stderr = res.Stderr
				r.ExitCode = res.ExitCode
				r.DurationMs = res.Duration.Milliseconds()
			}
			results[idx] = r
			_ = t.logs.Save(&models.ExecutionLog{
				HostID: h.ID, Command: command,
				Stdout: r.Stdout, Stderr: r.Stderr,
				ExitCode: r.ExitCode, DurationMs: r.DurationMs,
				TriggeredBy: "agent",
			})
		}(i, host)
	}
	wg.Wait()

	out, _ := json.Marshal(results)
	return &ToolResult{Content: string(out), RiskLevel: RiskDangerous}, nil
}
