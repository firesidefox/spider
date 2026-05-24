# CheckConnectivity Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `CheckConnectivity` agent tool that probes host reachability (ICMP) and face port availability (TCP dial) in parallel before explore-phase commands.

**Architecture:** New tool `CheckConnectivityTool` in `internal/agent/tools_connectivity.go`. Two-level probe: ICMP ping per host via `golang.org/x/net/icmp` unprivileged mode, then TCP dial per face only if host is reachable. Registered in `factory.go` alongside existing tools.

**Tech Stack:** Go, `golang.org/x/net/icmp`, `golang.org/x/net/ipv4`, `net.DialTimeout`, `sync.WaitGroup`

---

## File Map

| Action | File | Purpose |
|--------|------|---------|
| Create | `internal/agent/tools_connectivity.go` | Tool implementation + system prompt section |
| Create | `internal/agent/tools_connectivity_test.go` | Unit tests |
| Modify | `go.mod` / `go.sum` | Add `golang.org/x/net` |
| Modify | `internal/agent/factory.go` | Register `CheckConnectivityTool` |

---

### Task 1: Add `golang.org/x/net` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add dependency**

```bash
cd /Users/cw/fty.ai/spider.ai
go get golang.org/x/net@latest
```

Expected: `go.mod` now contains `golang.org/x/net vX.Y.Z`

- [ ] **Step 2: Verify build still passes**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add golang.org/x/net for ICMP ping support"
```

---

### Task 2: Write failing tests

**Files:**
- Create: `internal/agent/tools_connectivity_test.go`

- [ ] **Step 1: Write tests**

```go
package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

func TestCheckConnectivityTool_Name(t *testing.T) {
	tool := &CheckConnectivityTool{}
	if tool.Name() != "CheckConnectivity" {
		t.Errorf("got %q, want CheckConnectivity", tool.Name())
	}
}

func TestCheckConnectivityTool_RiskLevel(t *testing.T) {
	tool := &CheckConnectivityTool{}
	if tool.DefaultRiskLevel() != RiskL1 {
		t.Error("CheckConnectivity must be L1")
	}
}

func TestCheckConnectivityTool_ConcurrencySafe(t *testing.T) {
	tool := &CheckConnectivityTool{}
	if !tool.IsConcurrencySafe(nil) {
		t.Error("CheckConnectivity must be concurrency safe")
	}
}

func TestCheckConnectivityTool_InputSchema(t *testing.T) {
	tool := &CheckConnectivityTool{}
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["host_ids"]; !ok {
		t.Error("host_ids missing from InputSchema")
	}
	// host_ids is optional — must NOT be in required
	required, _ := schema["required"].([]string)
	for _, r := range required {
		if r == "host_ids" {
			t.Error("host_ids must not be required")
		}
	}
}

func TestCheckConnectivityTool_SystemPromptSection(t *testing.T) {
	tool := &CheckConnectivityTool{}
	section := tool.SystemPromptSection()
	if section == "" {
		t.Error("SystemPromptSection must not be empty")
	}
	for _, keyword := range []string{"CheckConnectivity", "Explore", "unreachable"} {
		if !containsStr(section, keyword) {
			t.Errorf("SystemPromptSection missing keyword %q", keyword)
		}
	}
}

// TestCheckConnectivityTool_TCPFaceProbe_Reachable starts a real TCP listener
// and verifies the face probe reports reachable=true.
func TestCheckConnectivityTool_TCPFaceProbe_Reachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "test-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type: models.FaceSSH,
		IP:   "127.0.0.1",
		Port: port,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v — content: %s", err, result.Content)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if len(got[0].Faces) != 1 {
		t.Fatalf("expected 1 face, got %d", len(got[0].Faces))
	}
	if !got[0].Faces[0].Reachable {
		t.Errorf("face should be reachable, got error: %s", got[0].Faces[0].Error)
	}
}

// TestCheckConnectivityTool_TCPFaceProbe_Unreachable uses a port with no listener.
func TestCheckConnectivityTool_TCPFaceProbe_Unreachable(t *testing.T) {
	// Find a free port then immediately close it so nothing listens on it
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "dead-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type: models.FaceSSH,
		IP:   "127.0.0.1",
		Port: port,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v — content: %s", err, result.Content)
	}
	if len(got) != 1 || len(got[0].Faces) != 1 {
		t.Fatalf("unexpected shape: %+v", got)
	}
	if got[0].Faces[0].Reachable {
		t.Error("face should be unreachable")
	}
}

// TestCheckConnectivityTool_ProbePort uses ProbePort when set.
func TestCheckConnectivityTool_ProbePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	probePort := ln.Addr().(*net.TCPAddr).Port

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "probe-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	// Port=9999 (nothing listening), ProbePort=probePort (listener active)
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type:      models.FaceRESTAPI,
		IP:        "127.0.0.1",
		Port:      9999,
		ProbePort: probePort,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got[0].Faces[0].Reachable {
		t.Errorf("should use ProbePort and be reachable, got: %s", got[0].Faces[0].Error)
	}
	if got[0].Faces[0].Port != probePort {
		t.Errorf("reported port should be probePort %d, got %d", probePort, got[0].Faces[0].Port)
	}
}

// TestCheckConnectivityTool_EmptyHostIDs_AllHosts verifies empty host_ids returns all hosts.
func TestCheckConnectivityTool_EmptyHostIDs_AllHosts(t *testing.T) {
	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	for _, name := range []string{"h1", "h2", "h3"} {
		if _, err := hosts.Add(&models.AddHostRequest{Name: name, IP: "127.0.0.1", Tags: []string{}}); err != nil {
			t.Fatal(err)
		}
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 results, got %d", len(got))
	}
}

// TestCheckConnectivityTool_RESTAPIFaceReachable starts an HTTP server and checks REST face.
func TestCheckConnectivityTool_RESTAPIFaceReachable(t *testing.T) {
	srv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	database := setupTestDB(t)
	hosts := store.NewHostStore(database)
	cm, err := crypto.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	faces := store.NewAccessFaceStore(database, cm)

	host, err := hosts.Add(&models.AddHostRequest{Name: "api-host", IP: "127.0.0.1", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := faces.Add(host.ID, &models.AddAccessFaceRequest{
		Type: models.FaceRESTAPI,
		IP:   "127.0.0.1",
		Port: port,
	}); err != nil {
		t.Fatal(err)
	}

	tool := NewCheckConnectivityTool(hosts, faces)
	result, err := tool.Execute(context.Background(), map[string]any{
		"host_ids": []any{host.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []connectivityResult
	if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got[0].Faces[0].Reachable {
		t.Errorf("REST API face should be reachable: %s", got[0].Faces[0].Error)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStrHelper(s, sub))
}

func containsStrHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/ -run "TestCheckConnectivity" -v 2>&1 | head -30
```

Expected: compile error — `CheckConnectivityTool`, `connectivityResult`, `NewCheckConnectivityTool` undefined

- [ ] **Step 3: Commit failing tests**

```bash
git add internal/agent/tools_connectivity_test.go
git commit -m "test(agent): add failing tests for CheckConnectivity tool"
```

---

### Task 3: Implement `CheckConnectivityTool`

**Files:**
- Create: `internal/agent/tools_connectivity.go`

- [ ] **Step 1: Write implementation**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type CheckConnectivityTool struct {
	hosts *store.HostStore
	faces *store.AccessFaceStore
}

func NewCheckConnectivityTool(hosts *store.HostStore, faces *store.AccessFaceStore) *CheckConnectivityTool {
	return &CheckConnectivityTool{hosts: hosts, faces: faces}
}

func (t *CheckConnectivityTool) Name() string        { return "CheckConnectivity" }
func (t *CheckConnectivityTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
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

func (t *CheckConnectivityTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	hosts, err := t.hosts.List("")
	if err != nil {
		return &ToolResult{Content: "failed to list hosts: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}

	// Filter by host_ids if provided
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
				filtered := hosts[:0]
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
			results[idx] = t.probeHost(host)
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

func (t *CheckConnectivityTool) probeHost(host *models.Host) connectivityResult {
	res := connectivityResult{
		HostID: host.ID,
		Name:   host.Name,
		IP:     host.IP,
		Faces:  []faceResult{},
	}

	latency, err := icmpPing(host.IP, 3*time.Second)
	if err != nil {
		res.Reachable = false
		res.Error = err.Error()
		// Still probe faces even if ICMP fails (ICMP may be blocked by firewall)
	} else {
		res.Reachable = true
		res.LatencyMs = latency.Milliseconds()
	}

	// Probe faces regardless of ICMP result (ICMP may be firewalled)
	if t.faces != nil {
		faceList, err := t.faces.ListByHost(host.ID)
		if err == nil && len(faceList) > 0 {
			faceResults := make([]faceResult, len(faceList))
			var fwg sync.WaitGroup
			for i, f := range faceList {
				fwg.Add(1)
				go func(idx int, face *models.AccessFace) {
					defer fwg.Done()
					faceResults[idx] = t.probeFace(face)
				}(i, f)
			}
			fwg.Wait()
			res.Faces = faceResults
		}
	}

	return res
}

func (t *CheckConnectivityTool) probeFace(face *models.AccessFace) faceResult {
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
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", face.IP, port), 3*time.Second)
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
// Uses "udp4" network which does not require root on Linux 3.11+ or macOS.
func icmpPing(ip string, timeout time.Duration) (time.Duration, error) {
	conn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		// Fallback: if ICMP unavailable, treat as unreachable
		return 0, fmt.Errorf("icmp listen: %w", err)
	}
	defer conn.Close()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   1,
			Seq:  1,
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

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, fmt.Errorf("set deadline: %w", err)
	}

	rb := make([]byte, 1500)
	_, _, err = conn.ReadFrom(rb)
	if err != nil {
		return 0, fmt.Errorf("icmp timeout or unreachable: %w", err)
	}
	return time.Since(start), nil
}
```

- [ ] **Step 2: Run tests**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/ -run "TestCheckConnectivity" -v 2>&1
```

Expected: all tests pass except ICMP-dependent ones (ICMP to 127.0.0.1 may or may not work in test env — TCP face tests must pass)

Note: `TestCheckConnectivityTool_TCPFaceProbe_Reachable`, `_Unreachable`, `_ProbePort`, `_EmptyHostIDs_AllHosts`, `_RESTAPIFaceReachable` must all PASS. The host-level ICMP result in these tests may show `reachable=false` if ICMP is blocked, but face probes run regardless.

- [ ] **Step 3: Run full agent test suite**

```bash
go test ./internal/agent/ -v 2>&1 | tail -20
```

Expected: all existing tests still pass

- [ ] **Step 4: Commit**

```bash
git add internal/agent/tools_connectivity.go
git commit -m "feat(agent): add CheckConnectivity tool with ICMP ping and TCP face probe"
```

---

### Task 4: Register tool in factory

**Files:**
- Modify: `internal/agent/factory.go:293-310`

- [ ] **Step 1: Add registration**

In `buildRegistryWithHosts`, after the existing `registry.Register(NewGetHostsTool(...))` line, add:

```go
registry.Register(NewCheckConnectivityTool(f.Hosts, f.AccessFaces))
```

The full updated block (lines ~293-312):

```go
func (f *Factory) buildRegistryWithHosts(conversationID string, selectedHostIDs []string) *ToolRegistry {
	registry := NewToolRegistry()
	listTool := NewGetHostsTool(f.Hosts, f.AccessFaces)
	listTool.selectedHostIDs = selectedHostIDs
	listTool.knowledgeStore = f.KnowledgeStore
	registry.Register(listTool)
	registry.Register(NewCheckConnectivityTool(f.Hosts, f.AccessFaces))
	registry.Register(NewExecuteCLITool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewBatchExecuteTool(f.Hosts, f.AccessFaces, f.SSHPool, f.Logs, f.SSHKeys))
	registry.Register(NewVerifyTool(f.Hosts, f.AccessFaces, f.SSHPool, f.SSHKeys))
	registry.Register(NewCallRESTAPITool(f.AccessFaces))
	registry.Register(NewSearchDocsTool(f.KnowledgeStore, f.Embedder))
	registry.Register(NewTodoTool(f.TodoStore, f.SSEBroadcaster, conversationID))
	registry.Register(NewGetTopologyTool(f.TopologyStore))
	registry.Register(NewGetTopologyContextTool(f.TopologyStore))
	registry.Register(NewCreateTaskTool(f.TaskStore, conversationID))
	registry.Register(NewInvokeSkillTool(f.DataDir))
	return registry
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 3: Run full test suite**

```bash
go test ./internal/agent/ -v 2>&1 | tail -30
```

Expected: all tests pass

- [ ] **Step 4: Verify tool appears in system prompt**

```bash
go test ./internal/agent/ -run "TestFactory" -v 2>&1
```

Expected: existing factory tests pass (system prompt now includes CheckConnectivity section)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/factory.go
git commit -m "feat(agent): register CheckConnectivity tool in agent factory"
```

---

### Task 5: Build and smoke test

**Files:** none (verification only)

- [ ] **Step 1: Build binary**

```bash
cd /Users/cw/fty.ai/spider.ai
go build -a -o /tmp/spider-connectivity-test ./cmd/spider
```

Expected: binary built at `/tmp/spider-connectivity-test`

- [ ] **Step 2: Start test server**

```bash
/tmp/spider-connectivity-test serve --addr :8003 --data-dir ~/.spider/data
```

- [ ] **Step 3: Verify tool in system prompt via API**

In a separate terminal:
```bash
curl -s http://localhost:8003/api/v1/agent/system-prompt 2>/dev/null | grep -A5 "CheckConnectivity" || echo "check agent debug endpoint"
```

If no debug endpoint, verify by checking the agent chat UI at http://localhost:8003 — start a conversation and ask "what tools do you have?" The agent should mention CheckConnectivity.

- [ ] **Step 4: Kill test server and clean up**

```bash
pkill -f spider-connectivity-test
rm /tmp/spider-connectivity-test
```

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -p
git commit -m "fix(agent): <describe fix if any>"
```
