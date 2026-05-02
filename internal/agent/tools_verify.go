package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

type VerifyTool struct {
	hosts   *store.HostStore
	sshPool *ssh.Pool
	sshKeys *store.SSHKeyStore
}

func NewVerifyTool(hosts *store.HostStore, sshPool *ssh.Pool, sshKeys *store.SSHKeyStore) *VerifyTool {
	return &VerifyTool{hosts: hosts, sshPool: sshPool, sshKeys: sshKeys}
}

func (t *VerifyTool) Name() string        { return "verify" }
func (t *VerifyTool) Description() string { return "Verify conditions on remote hosts with retry polling" }

func (t *VerifyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"checks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"host_id": map[string]any{"type": "string"},
						"command": map[string]any{"type": "string"},
						"expect":  map[string]any{"type": "string"},
					},
					"required": []string{"host_id", "command", "expect"},
				},
				"description": "List of checks to verify",
			},
			"timeout":  map[string]any{"type": "integer", "description": "Timeout in seconds (default 60)"},
			"interval": map[string]any{"type": "integer", "description": "Poll interval in seconds (default 5)"},
		},
		"required": []string{"checks"},
	}
}

type verifyCheck struct {
	HostID  string
	Command string
	Expect  string
}

type checkResult struct {
	HostID  string `json:"host_id"`
	Command string `json:"command"`
	Expect  string `json:"expect"`
	Passed  bool   `json:"passed"`
	Stdout  string `json:"stdout,omitempty"`
	Error   string `json:"error,omitempty"`
}

func parseChecks(input map[string]any) ([]verifyCheck, error) {
	raw, ok := input["checks"].([]any)
	if !ok {
		return nil, fmt.Errorf("checks must be an array")
	}
	checks := make([]verifyCheck, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		checks = append(checks, verifyCheck{
			HostID:  fmt.Sprintf("%v", m["host_id"]),
			Command: fmt.Sprintf("%v", m["command"]),
			Expect:  fmt.Sprintf("%v", m["expect"]),
		})
	}
	return checks, nil
}

func (t *VerifyTool) runCheck(ctx context.Context, c verifyCheck) checkResult {
	r := checkResult{HostID: c.HostID, Command: c.Command, Expect: c.Expect}
	host, err := t.hosts.GetByID(c.HostID)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	client, err := t.sshPool.Get(host, t.hosts, t.sshKeys)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	defer t.sshPool.Release(host.ID)
	res, err := client.Execute(ctx, c.Command)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Stdout = res.Stdout
	r.Passed = res.ExitCode == 0 && strings.Contains(res.Stdout, c.Expect)
	return r
}

func (t *VerifyTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	checks, err := parseChecks(input)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true, RiskLevel: RiskSafe}, nil
	}

	timeoutSec := 60
	if v, ok := input["timeout"].(float64); ok {
		timeoutSec = int(v)
	}
	intervalSec := 5
	if v, ok := input["interval"].(float64); ok {
		intervalSec = int(v)
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	interval := time.Duration(intervalSec) * time.Second

	var lastResults []checkResult
	for time.Now().Before(deadline) {
		lastResults = make([]checkResult, len(checks))
		allPassed := true
		for i, c := range checks {
			lastResults[i] = t.runCheck(ctx, c)
			if !lastResults[i].Passed {
				allPassed = false
			}
		}
		if allPassed {
			out, _ := json.Marshal(map[string]any{"status": "ok", "checks": lastResults})
			return &ToolResult{Content: string(out), RiskLevel: RiskSafe}, nil
		}
		select {
		case <-ctx.Done():
			break
		case <-time.After(interval):
		}
	}

	out, _ := json.Marshal(map[string]any{"status": "timeout", "checks": lastResults})
	return &ToolResult{Content: string(out), IsError: true, RiskLevel: RiskSafe}, nil
}
