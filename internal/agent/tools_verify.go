package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

type PollUntilTool struct {
	hosts   *store.HostStore
	faces   *store.AccessFaceStore
	sshPool *ssh.Pool
	sshKeys *store.SSHKeyStore
}

func NewVerifyTool(hosts *store.HostStore, faces *store.AccessFaceStore, sshPool *ssh.Pool, sshKeys *store.SSHKeyStore) *PollUntilTool {
	return &PollUntilTool{hosts: hosts, faces: faces, sshPool: sshPool, sshKeys: sshKeys}
}

func (t *PollUntilTool) DefaultRiskLevel() RiskLevel              { return RiskL1 }
func (t *PollUntilTool) IsConcurrencySafe(_ map[string]any) bool { return false }
func (t *PollUntilTool) Name() string                { return "PollUntil" }
func (t *PollUntilTool) Description() string {
	return "Poll remote hosts until all conditions are met or timeout. Read-only. Use after deployments or config changes to wait for services to become ready."
}

func (t *PollUntilTool) InputSchema() map[string]any {
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
						"expect":  map[string]any{"type": "string", "description": "Substring expected in stdout"},
					},
					"required": []string{"host_id", "command", "expect"},
				},
				"description": "Conditions to wait for — all must pass",
			},
			"timeout":  map[string]any{"type": "integer", "description": "Total wait time in seconds (default 60)"},
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

func (t *PollUntilTool) runCheck(ctx context.Context, c verifyCheck) checkResult {
	r := checkResult{HostID: c.HostID, Command: c.Command, Expect: c.Expect}
	host, err := t.hosts.GetByID(c.HostID)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	face, err := t.faces.GetSSHFaceForHost(host.ID)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	client, err := t.sshPool.Get(face, t.faces, t.sshKeys)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	defer t.sshPool.Release(face.ID)
	res, err := client.Execute(ctx, c.Command)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Stdout = res.Stdout
	r.Passed = res.ExitCode == 0 && strings.Contains(res.Stdout, c.Expect)
	return r
}

func (t *PollUntilTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	checks, err := parseChecks(input)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true, RiskLevel: RiskL1}, nil
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
	checkTimeout := interval

	var lastResults []checkResult
loop:
	for time.Now().Before(deadline) {
		lastResults = make([]checkResult, len(checks))
		var wg sync.WaitGroup
		for i, c := range checks {
			wg.Add(1)
			go func(idx int, chk verifyCheck) {
				defer wg.Done()
				checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
				defer cancel()
				lastResults[idx] = t.runCheck(checkCtx, chk)
			}(i, c)
		}
		wg.Wait()

		allPassed := true
		for _, r := range lastResults {
			if !r.Passed {
				allPassed = false
				break
			}
		}
		if allPassed {
			return &ToolResult{Content: formatPollResults("ok", lastResults), RiskLevel: RiskL1, Summary: "ok"}, nil
		}
		select {
		case <-ctx.Done():
			break loop
		case <-time.After(interval):
		}
	}

	return &ToolResult{Content: formatPollResults("timeout", lastResults), IsError: true, RiskLevel: RiskL1, Summary: "failed"}, nil
}

func formatPollResults(status string, results []checkResult) string {
	var b strings.Builder
	if status == "ok" {
		b.WriteString("All checks passed.\n\n")
	} else {
		b.WriteString("Timeout — not all checks passed.\n\n")
	}
	for _, r := range results {
		if r.Passed {
			fmt.Fprintf(&b, "✓ %s: %s\n", r.HostID, r.Command)
		} else if r.Error != "" {
			fmt.Fprintf(&b, "✗ %s: %s — error: %s\n", r.HostID, r.Command, r.Error)
		} else {
			fmt.Fprintf(&b, "✗ %s: %s — expected %q, got %q\n", r.HostID, r.Command, r.Expect, strings.TrimSpace(r.Stdout))
		}
	}
	return b.String()
}

const pollUntilPromptSection = `### PollUntil (read-only, polls until all conditions pass or timeout)

**Core behavior:** Runs all checks in parallel, repeats every "interval" seconds until every check passes or "timeout" is reached.

**When to use:**
- After a state-changing operation (restart, deploy, config reload) when the result takes time to appear
- When you need to wait for a condition, not just observe it once

**When NOT to use:**
- One-shot observation — use RunCommand instead (PollUntil always waits at least one interval)
- Checks that are expected to pass immediately

**How to set timeout and interval:**
- Default: timeout=60s, interval=5s
- For fast services (nginx, systemd unit): interval=3, timeout=30
- For slow deploys or DB migrations: interval=10, timeout=120

**expect field:** substring match against stdout. Command must exit 0 AND stdout must contain expect string.

<example>
User: Restart nginx and confirm it's up.
Assistant: RunCommand → "systemctl restart nginx", then PollUntil → checks=[{host_id, "systemctl is-active nginx", "active"}], interval=3, timeout=30
</example>

<example>
User: Deploy the app and wait for it to be healthy.
Assistant: RunCommand → deploy script, then PollUntil → checks=[{host_id, "curl -sf http://localhost:8080/health", "ok"}], interval=5, timeout=120
</example>

<example>
User: Is port 80 open on web-01?
Assistant: RunCommand → "ss -tlnp | grep :80". Does NOT use PollUntil — this is a one-shot check.
</example>

**Rule:** If the state change has not yet been triggered, do NOT call PollUntil — trigger the change first with RunCommand.`

func (t *PollUntilTool) SystemPromptSection() string { return pollUntilPromptSection }
