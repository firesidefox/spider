# Host Monitor Design

**Date:** 2026-05-15  
**Status:** Approved

## Overview

Add real-time host liveness monitoring to spider.ai. The heat matrix in TargetPanel currently hardcodes all hosts as `online`. This feature makes it reflect actual reachability via TCP half-open probing.

## Goals

- Detect host online/offline status via TCP dial probe
- Push status changes to frontend in real time via SSE
- Per-host probe configuration in access face settings
- No external dependencies (no DeepSight process required)

## Architecture

### Backend: `internal/monitor` package

New `Monitor` struct responsible for probing all hosts on a fixed interval.

**Probe method:** `net.DialTimeout("tcp", host:port, 2s)` — TCP dial to the host's SSH port. Success = online, timeout/refused = offline. No raw socket or root privileges required.

**Probe interval:** Global tick every 2 seconds. Per-host `ProbeInterval` sets the minimum interval for that host — Monitor records each host's last probe time and skips hosts that haven't reached their interval yet. Default per-host interval: 2s.

**Concurrency:** Semaphore-limited to 20 concurrent probes to avoid network storms.

**Lifecycle:**
- Started at app init alongside other background services
- Dynamically updates host list when hosts are added/removed
- Continues probing offline hosts; broadcasts `online` on recovery

**State:** In-memory only. No DB persistence. On restart, one probe round completes in ~2s.

```go
type HostStatus struct {
    HostID    string
    Online    bool
    CheckedAt time.Time
}

type Monitor struct {
    hostStore  HostStore
    faceStore  AccessFaceStore
    broadcast  func(hostID string, online bool)
    statuses   map[string]bool  // hostID -> online
    mu         sync.RWMutex
}
```

### Access Face Config: new probe fields

In the access face configuration (per host), add two optional fields:

```go
type AccessFace struct {
    // ... existing fields ...
    ProbePort     int  `json:"probe_port"`      // default: 22
    ProbeInterval int  `json:"probe_interval"`  // seconds, default: 2
}
```

UI: in the access face editor, add "存活探测端口" (default 22) and "探测间隔(秒)" (default 2).

### New API endpoints

**`GET /api/v1/hosts/statuses`**

Returns current status snapshot for all hosts. Called once on page load.

```json
[
  { "host_id": "abc", "online": true,  "checked_at": "2026-05-15T..." },
  { "host_id": "def", "online": false, "checked_at": "2026-05-15T..." }
]
```

**`GET /api/v1/stream`**

Global SSE stream (not tied to a conversation). Pushes `host_status` events when status changes.

```json
{ "type": "host_status", "content": { "host_id": "abc", "online": false } }
```

Frontend subscribes on mount, independent of conversation SSE. Uses cookie auth (same as conversation SSE — `EventSource` relies on browser cookies, no custom header needed).

### Frontend changes

**`loadDevices()` in ChatView.vue:**

Fetch `/api/v1/hosts` and `/api/v1/hosts/statuses` in parallel. Map status onto `DeviceStatus.status` field (`online` or `offline`).

**Global SSE subscription:**

On `onMounted`, open `EventSource('/api/v1/stream')`. On `host_status` event, update `devices.value` reactively.

**Global SSE handler (separate from conversation SSE):**

```ts
// in onMounted — independent EventSource, not handleConvEvent
const globalEs = new EventSource('/api/v1/stream')
globalEs.onmessage = (e) => {
  const event = JSON.parse(e.data)
  if (event.type === 'host_status') {
    const { host_id, online } = event.content
    const idx = devices.value.findIndex(d => d.id === host_id)
    if (idx !== -1) {
      devices.value = devices.value.map((d, i) =>
        i === idx ? { ...d, status: online ? 'online' : 'offline' } : d
      )
    }
  }
}
```

**Status priority:**

Execution states (`executing`, `success`, `failed`) take precedence over monitor states. Frontend maintains an `executingHosts` Set — hosts currently in a tool call. The global SSE handler skips updating any host in this set. When `markDevicesDone` resets a host back to `online`/`offline`, it uses the Monitor's last known value for that host rather than hardcoding `online`.

## Data Flow

```
Monitor (2s tick)
  → TCP dial each host:port
  → status changed?
    → update in-memory map
    → app.BroadcastGlobalSSE(host_status event)
      → /api/v1/stream subscribers (frontend)
        → devices.value updated
          → TargetPanel heat matrix re-renders
```

## Out of Scope

- ICMP ping (requires raw socket / root)
- HTTP health checks
- Historical uptime data
- Alert notifications on offline
- Multi-port probing
