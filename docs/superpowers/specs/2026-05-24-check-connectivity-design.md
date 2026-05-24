# CheckConnectivity Tool Design

**Date:** 2026-05-24  
**Status:** Approved

## Problem

Agent explore phase has no way to know which hosts are reachable before running commands. Unreachable hosts cause SSH timeouts that stack up and slow down multi-host tasks. No structured reachability signal means the agent can't skip dead hosts proactively.

## Solution

New `CheckConnectivity` agent tool. Two-level parallel probe: ICMP ping at host level, TCP dial at face level. L1 risk, auto-allowed in explore phase.

## Tool Spec

**Name:** `CheckConnectivity`

**Input:**
```json
{
  "host_ids": ["id1", "id2"]  // optional; empty = all hosts
}
```

**Behavior:**
1. Load hosts from store (filtered by `host_ids` if provided, else all hosts)
2. For each host, run two probes in parallel:
   - **Host probe:** ICMP ping to `Host.IP` via `golang.org/x/net/icmp` unprivileged mode, timeout 3s
   - **Face probes:** TCP dial to `Face.IP : (ProbePort if set, else Port)` for each face, timeout 3s — only runs if host ICMP probe succeeds
3. All hosts probed concurrently

**Output:**
```json
[
  {
    "host_id": "abc",
    "name": "web-01",
    "ip": "10.0.0.1",
    "reachable": true,
    "latency_ms": 12,
    "faces": [
      {
        "face_id": "f1",
        "type": "ssh",
        "ip": "10.0.0.1",
        "port": 22,
        "reachable": true,
        "latency_ms": 8,
        "error": ""
      },
      {
        "face_id": "f2",
        "type": "restapi",
        "ip": "10.0.0.1",
        "port": 8080,
        "reachable": false,
        "latency_ms": 0,
        "error": "dial tcp: connection refused"
      }
    ]
  }
]
```

**Risk level:** L1 (read-only, no side effects)  
**Concurrency safe:** yes

## Dependencies

Add `golang.org/x/net` to `go.mod`. Use `golang.org/x/net/icmp` + `golang.org/x/net/ipv4` for unprivileged ICMP (works on macOS and Linux 3.11+ without root).

## System Prompt Section

```
## CheckConnectivity

**When to use:** At the start of any explore-phase task that targets multiple hosts — before RunCommand, RunCommandBatch, or CallAPI.

**When NOT to use:** Single-host tasks where connectivity is obvious; tasks that don't involve remote execution.

**Rules:**
- Host unreachable → skip all operations on that host; report to user
- Host reachable but face unreachable → skip that face's operations (RunCommand for ssh face, CallAPI for restapi face); report to user
- Proceed with reachable hosts/faces without waiting for user confirmation

<example>
User: Restart nginx on all web servers.
Assistant: GetHosts → CheckConnectivity → skip unreachable hosts → RunCommandBatch "systemctl restart nginx" on reachable ssh faces only
</example>
```

## Implementation Plan

### Files to create
- `internal/agent/tools_connectivity.go` — tool implementation

### Files to modify
- `go.mod` / `go.sum` — add `golang.org/x/net`
- `internal/agent/factory.go` — register `CheckConnectivityTool`

### Tool struct
```go
type CheckConnectivityTool struct {
    hosts *store.HostStore
    faces *store.AccessFaceStore
}
```

### ICMP probe
Use `golang.org/x/net/icmp.ListenPacket("udp4", "")` for unprivileged mode. Send one echo request, wait for reply with 3s deadline. Parse RTT from send/receive timestamps.

### TCP face probe
`net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 3*time.Second)`. Use `ProbePort` if non-zero, else `Port`.

### Concurrency
`sync.WaitGroup` + goroutine per host. Within each host, goroutine per face after ICMP completes.
