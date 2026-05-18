package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/permission"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

type ExecuteCLITool struct {
	hosts   *store.HostStore
	faces   *store.AccessFaceStore
	sshPool *ssh.Pool
	logs    *store.LogStore
	sshKeys *store.SSHKeyStore
}

func NewExecuteCLITool(hosts *store.HostStore, faces *store.AccessFaceStore, sshPool *ssh.Pool, logs *store.LogStore, sshKeys *store.SSHKeyStore) *ExecuteCLITool {
	return &ExecuteCLITool{hosts: hosts, faces: faces, sshPool: sshPool, logs: logs, sshKeys: sshKeys}
}

func (t *ExecuteCLITool) DefaultRiskLevel() RiskLevel              { return RiskL2 }
func (t *ExecuteCLITool) IsConcurrencySafe(_ map[string]any) bool { return false }
func (t *ExecuteCLITool) Name() string                  { return "RunCommand" }

func (t *ExecuteCLITool) Description() string {
	return `Execute a CLI command on a remote host via SSH. Has side effects. Check ListHosts "access_faces" first — do NOT call if "ssh" is absent. Always set ` + "`intent`" + ` to a short goal description (e.g. "重启 nginx 使配置生效").
Risk depends on the command:
- Read-only commands (ls, cat, grep, ps, df, free, uname, systemctl status): safe, can use in Explore phase
- State-changing commands (rm, kill, systemctl start|stop|restart, apt, yum, chmod, chown): use only in Act phase`
}

func (t *ExecuteCLITool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_id":    map[string]any{"type": "string", "description": "Target host ID or name"},
			"command":    map[string]any{"type": "string", "description": "Shell command to execute"},
			"risk_level": map[string]any{"type": "string", "enum": []string{"L1", "L2", "L3", "L4"}, "description": "Risk level. L1=read-only, L2=standard change, L3=destructive, L4=critical"},
			"intent":     map[string]any{"type": "string", "description": "What you are trying to achieve with this command (goal only, no device names). Required for L2/L3/L4."},
		},
		"required": []string{"host_id", "command", "intent"},
	}
}

func (t *ExecuteCLITool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	hostID, _ := input["host_id"].(string)
	command, _ := input["command"].(string)
	riskStr, _ := input["risk_level"].(string)
	if hostID == "" || command == "" {
		return &ToolResult{Content: "missing required fields: host_id, command", IsError: true, RiskLevel: RiskL2}, nil
	}
	risk := RiskL2
	if riskStr != "" {
		risk = permission.ParseRiskLevel(riskStr)
	}

	intent, _ := input["intent"].(string)
	if intent == "" && risk != RiskL1 {
		log.Printf("WARNING: RunCommand called without intent field (host=%s, risk=%s)", hostID, riskStr)
	}

	h, err := t.hosts.GetByIDOrName(hostID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("host not found: %v", err), IsError: true, RiskLevel: risk}, nil
	}

	face, err := t.faces.GetSSHFaceForHost(h.ID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("no SSH access face: %v", err), IsError: true, RiskLevel: risk}, nil
	}

	client, err := t.sshPool.Get(face, t.faces, t.sshKeys)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("SSH connect failed: %v", err), IsError: true, RiskLevel: risk}, nil
	}
	defer t.sshPool.Release(face.ID)

	execCtx, execCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer execCancel()
	result, err := client.Execute(execCtx, command)
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
	return &ToolResult{Content: string(out), Nudge: execNudge, RiskLevel: risk, Summary: cliSummary(result.ExitCode, result.Stderr)}, nil
}

func cliSummary(exitCode int, stderr string) string {
	if exitCode == 0 {
		return "exit 0"
	}
	firstLine, _, _ := strings.Cut(stderr, "\n")
	if len(firstLine) > 60 {
		firstLine = firstLine[:60] + "…"
	}
	if firstLine == "" {
		return fmt.Sprintf("exit %d", exitCode)
	}
	return fmt.Sprintf("exit %d: %s", exitCode, firstLine)
}

const runCommandPromptSection = `### RunCommand / RunCommandBatch (has side effects)

**When to use:**
- Explore phase: read-only commands (ls, cat, grep, ps, df, systemctl status) — use freely
- Act phase: state-changing commands (rm, kill, systemctl restart, apt, chmod) — only after confirming intent

**When NOT to use:** Do not run state-changing commands before the user has confirmed the plan.

**Command complexity:** One goal per command call. Do not chain many && operators in a single command — the tool will reject overly complex commands. For multi-metric checks (CPU, memory, disk, logins, ports), use separate RunCommand calls — one per metric.

<example>
User: Check CPU, memory, disk, and listening ports on all servers.
Bad:  RunCommandBatch with "uptime && free -h && df -h && ss -tlnp"
Good: Four separate RunCommandBatch calls — uptime / free -h / df -h / ss -tlnp.
</example>

<example>
User: Clean up logs older than 30 days on all app servers.
Assistant: First calls RunCommandBatch with "find /var/log -mtime +30" to preview what would be deleted. Confirms with user. Then runs the delete command.
</example>

<example>
User: Restart the database service.
Assistant: Confirms the target host and service name, then calls RunCommand with "systemctl restart postgresql".
</example>`

func (t *ExecuteCLITool) SystemPromptSection() string {
	return runCommandPromptSection
}
