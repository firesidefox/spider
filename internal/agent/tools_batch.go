package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/permission"
	"github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

type BatchExecuteTool struct {
	hosts   *store.HostStore
	faces   *store.AccessFaceStore
	sshPool *ssh.Pool
	logs    *store.LogStore
	sshKeys *store.SSHKeyStore
}

func NewBatchExecuteTool(hosts *store.HostStore, faces *store.AccessFaceStore, sshPool *ssh.Pool, logs *store.LogStore, sshKeys *store.SSHKeyStore) *BatchExecuteTool {
	return &BatchExecuteTool{hosts: hosts, faces: faces, sshPool: sshPool, logs: logs, sshKeys: sshKeys}
}

func (t *BatchExecuteTool) DefaultRiskLevel() RiskLevel { return RiskL2 }
func (t *BatchExecuteTool) Name() string                  { return "RunCommandBatch" }
func (t *BatchExecuteTool) Description() string {
	return "Execute a CLI command on multiple hosts in parallel. Has side effects. Use only after confirming intent in Plan phase. Always set `intent` to a short goal description (e.g. \"重启 nginx 使配置生效\"). Check ListHosts \"access_faces\" first — only target hosts with \"ssh\". If you are unsure of host IDs, call ListHosts first — never guess host IDs."
}

func (t *BatchExecuteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Host IDs only (NOT names). Call ListHosts first if you don't have IDs."},
			"tag":        map[string]any{"type": "string", "description": "Target all hosts with this tag (use instead of host_ids)"},
			"command":    map[string]any{"type": "string", "description": "Shell command to execute on all target hosts"},
			"risk_level": map[string]any{"type": "string", "enum": []string{"L1", "L2", "L3", "L4"}, "description": "Risk level. L1=read-only, L2=standard change, L3=destructive, L4=critical"},
			"intent":     map[string]any{"type": "string", "description": "What you are trying to achieve with this command (goal only, no device names). Required for L2/L3/L4."},
		},
		"required": []string{"command", "intent"},
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
		return &ToolResult{Content: "command is required", IsError: true, RiskLevel: RiskL2}, nil
	}

	riskStr, _ := input["risk_level"].(string)
	risk := RiskL2
	if riskStr != "" {
		risk = permission.ParseRiskLevel(riskStr)
	}

	intent, _ := input["intent"].(string)
	if intent == "" && risk != RiskL1 {
		log.Printf("WARNING: RunCommandBatch called without intent field (risk=%s)", risk)
	}

	var hostList []*models.Host
	if tag, ok := input["tag"].(string); ok && tag != "" {
		hosts, err := t.hosts.List(tag)
		if err != nil {
			return &ToolResult{Content: fmt.Sprintf("failed to list hosts by tag: %v", err), IsError: true, RiskLevel: risk}, nil
		}
		hostList = hosts
	} else if ids, ok := input["host_ids"].([]any); ok {
		var notFound []string
		for _, id := range ids {
			if sid, ok := id.(string); ok {
				h, err := t.hosts.GetByID(sid)
				if err != nil {
					notFound = append(notFound, sid)
				} else {
					hostList = append(hostList, h)
				}
			}
		}
		if len(notFound) > 0 && len(hostList) == 0 {
			return &ToolResult{Content: fmt.Sprintf("hosts not found: %v\nhost_ids requires IDs, not names. Call ListHosts first and use the \"id\" field from the response.", notFound), IsError: true, RiskLevel: risk}, nil
		}
		if len(notFound) > 0 {
			log.Printf("WARNING: RunCommandBatch skipping unknown host IDs: %v", notFound)
		}
	}
	if len(hostList) == 0 {
		return &ToolResult{Content: "no hosts selected", IsError: true, RiskLevel: risk}, nil
	}

	results := make([]batchHostResult, len(hostList))
	var wg sync.WaitGroup

	execCtx, execCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer execCancel()

	for i, host := range hostList {
		wg.Add(1)
		go func(idx int, h *models.Host) {
			defer wg.Done()
			r := batchHostResult{HostID: h.ID, HostName: h.Name}
			face, ferr := t.faces.GetSSHFaceForHost(h.ID)
			if ferr != nil {
				r.Error = ferr.Error()
				results[idx] = r
				return
			}
			client, err := t.sshPool.Get(face, t.faces, t.sshKeys)
			if err != nil {
				r.Error = err.Error()
				results[idx] = r
				return
			}
			defer t.sshPool.Release(face.ID)
			res, err := client.Execute(execCtx, command)
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

	// Format results as readable text
	var output strings.Builder
	okCount, failCount := 0, 0
	for _, r := range results {
		fmt.Fprintf(&output, "Host: %s (%s)\n", r.HostName, r.HostID)
		if r.Error != "" {
			failCount++
			fmt.Fprintf(&output, "  Error: %s\n", r.Error)
		} else {
			if r.ExitCode == 0 {
				okCount++
			} else {
				failCount++
			}
			fmt.Fprintf(&output, "  Exit Code: %d\n", r.ExitCode)
			fmt.Fprintf(&output, "  Duration: %dms\n", r.DurationMs)
			if r.Stdout != "" {
				fmt.Fprintf(&output, "  Stdout:\n%s\n", r.Stdout)
			}
			if r.Stderr != "" {
				fmt.Fprintf(&output, "  Stderr:\n%s\n", r.Stderr)
			}
		}
		output.WriteString("\n")
	}
	var summary string
	if failCount == 0 {
		summary = fmt.Sprintf("%d hosts ok", okCount)
	} else {
		summary = fmt.Sprintf("%d ok, %d failed", okCount, failCount)
	}
	return &ToolResult{Content: output.String(), Nudge: execNudge, RiskLevel: risk, Summary: summary}, nil
}

func (t *BatchExecuteTool) SystemPromptSection() string { return runCommandPromptSection }
