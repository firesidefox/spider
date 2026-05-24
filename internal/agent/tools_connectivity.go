package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

var icmpSeq uint32

type CheckConnectivityTool struct {
	hosts *store.HostStore
	faces *store.AccessFaceStore
}

func NewCheckConnectivityTool(hosts *store.HostStore, faces *store.AccessFaceStore) *CheckConnectivityTool {
	return &CheckConnectivityTool{hosts: hosts, faces: faces}
}

func (t *CheckConnectivityTool) Name() string                            { return "CheckConnectivity" }
func (t *CheckConnectivityTool) DefaultRiskLevel() RiskLevel             { return RiskL1 }
func (t *CheckConnectivityTool) IsConcurrencySafe(_ map[string]any) bool { return true }

func (t *CheckConnectivityTool) Description() string {
	return "Probe host reachability (ICMP ping) and access face port availability (TCP dial) in parallel. Read-only. No side effects. Use at the start of Explore phase for multi-host tasks."
}

func (t *CheckConnectivityTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Host IDs to probe. Empty or omitted = all hosts.",
			},
		},
	}
}

const checkConnectivityPromptSection = `## CheckConnectivity

**When to use:** At the start of any Explore-phase task that targets multiple hosts — before RunCommand, RunCommandBatch, or CallAPI.

**When NOT to use:** Single-host tasks where connectivity is obvious; tasks that don't involve remote execution.

**Rules:**
- Host unreachable → skip all operations on that host; report to user
- Host reachable but face unreachable → skip that face's operations (RunCommand for ssh face, CallAPI for restapi face); report to user
- Proceed with reachable hosts/faces without waiting for user confirmation

<example>
User: Restart nginx on all web servers.
Assistant: GetHosts → CheckConnectivity → skip unreachable hosts → RunCommandBatch "systemctl restart nginx" on reachable ssh faces only
</example>`

func (t *CheckConnectivityTool) SystemPromptSection() string {
	return checkConnectivityPromptSection
}

type faceResult struct {
	FaceID    string `json:"face_id"`
	Type      string `json:"type"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type connectivityResult struct {
	HostID    string       `json:"host_id"`
	Name      string       `json:"name"`
	IP        string       `json:"ip"`
	Reachable bool         `json:"reachable"`
	LatencyMs int64        `json:"latency_ms"`
	Error     string       `json:"error,omitempty"`
	Faces     []faceResult `json:"faces"`
}

func (t *CheckConnectivityTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	hosts, err := t.hosts.List("")
	if err != nil {
		return &ToolResult{Content: "failed to list hosts: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}

	if ids, ok := input["host_ids"]; ok {
		switch v := ids.(type) {
		case []any:
			if len(v) > 0 {
				allowed := make(map[string]bool, len(v))
				for _, id := range v {
					if s, ok := id.(string); ok {
						allowed[s] = true
					}
				}
				filtered := make([]*models.Host, 0, len(hosts))
				for _, h := range hosts {
					if allowed[h.ID] {
						filtered = append(filtered, h)
					}
				}
				hosts = filtered
			}
		}
	}

	results := make([]connectivityResult, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(idx int, host *models.Host) {
			defer wg.Done()
			results[idx] = t.probeHost(ctx, host)
		}(i, h)
	}
	wg.Wait()

	out, _ := json.Marshal(results)
	reachable := 0
	for _, r := range results {
		if r.Reachable {
			reachable++
		}
	}
	return &ToolResult{
		Content:   string(out),
		RiskLevel: RiskL1,
		Summary:   fmt.Sprintf("%d/%d hosts reachable", reachable, len(results)),
	}, nil
}

func (t *CheckConnectivityTool) probeHost(ctx context.Context, host *models.Host) connectivityResult {
	res := connectivityResult{
		HostID: host.ID,
		Name:   host.Name,
		IP:     host.IP,
		Faces:  []faceResult{},
	}

	latency, err := icmpPing(ctx, host.IP, 3*time.Second)
	if err != nil {
		res.Reachable = false
		res.Error = err.Error()
	} else {
		res.Reachable = true
		res.LatencyMs = latency.Milliseconds()
	}

	if t.faces != nil {
		faceList, err := t.faces.ListByHost(host.ID)
		if err == nil && len(faceList) > 0 {
			faceResults := make([]faceResult, len(faceList))
			var fwg sync.WaitGroup
			for i, f := range faceList {
				fwg.Add(1)
				go func(idx int, face *models.AccessFace) {
					defer fwg.Done()
					faceResults[idx] = t.probeFace(ctx, face)
				}(i, f)
			}
			fwg.Wait()
			res.Faces = faceResults
		}
	}

	return res
}

func (t *CheckConnectivityTool) probeFace(ctx context.Context, face *models.AccessFace) faceResult {
	port := face.Port
	if face.ProbePort != 0 {
		port = face.ProbePort
	}
	fr := faceResult{
		FaceID: face.ID,
		Type:   string(face.Type),
		IP:     face.IP,
		Port:   port,
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", fmt.Sprintf("%s:%d", face.IP, port))
	if err != nil {
		fr.Reachable = false
		fr.Error = err.Error()
		return fr
	}
	conn.Close()
	fr.Reachable = true
	fr.LatencyMs = time.Since(start).Milliseconds()
	return fr
}

// icmpPing sends one unprivileged ICMP echo to ip and returns RTT.
// Uses "udp4" which does not require root on Linux 3.11+ or macOS.
func icmpPing(ctx context.Context, ip string, timeout time.Duration) (time.Duration, error) {
	conn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		return 0, fmt.Errorf("icmp listen: %w", err)
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	seq := int(atomic.AddUint32(&icmpSeq, 1) & 0xffff)

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("spider"),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return 0, fmt.Errorf("marshal icmp: %w", err)
	}

	dst := &net.UDPAddr{IP: net.ParseIP(ip)}
	start := time.Now()
	if _, err := conn.WriteTo(wb, dst); err != nil {
		return 0, fmt.Errorf("write icmp: %w", err)
	}

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return 0, fmt.Errorf("set deadline: %w", err)
	}

	rb := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(rb)
		if err != nil {
			return 0, fmt.Errorf("icmp timeout or unreachable: %w", err)
		}
		reply, err := icmp.ParseMessage(1 /* iana.ProtocolICMP */, rb[:n])
		if err != nil {
			continue
		}
		if reply.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		echo, ok := reply.Body.(*icmp.Echo)
		if !ok || echo.ID != id || echo.Seq != seq {
			continue
		}
		return time.Since(start), nil
	}
}
