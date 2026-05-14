# Host Monitor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add real-time TCP liveness probing for all hosts, pushing online/offline status to the frontend heat matrix via SSE.

**Architecture:** A new `internal/monitor` package runs a background goroutine that probes each host's TCP port every 2 seconds (per-host configurable). Status changes are broadcast via a new global SSE endpoint `/api/v1/stream`. The frontend subscribes on mount and updates `devices.value` reactively.

**Tech Stack:** Go `net.DialTimeout`, `sync.RWMutex`, `golang.org/x/sync/semaphore`, Vue 3 `EventSource`, existing spider.ai SSE infrastructure.

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/monitor/monitor.go` | TCP probe loop, status tracking, onChange callback |
| Modify | `internal/models/host.go` | Add `ProbePort`, `ProbeInterval` fields to `AccessFace` |
| Modify | `internal/db/schema.go` | Add `probe_port`, `probe_interval` columns to `access_faces` |
| Modify | `internal/store/access_face_store.go` | Read/write new probe fields |
| Modify | `internal/mcp/server.go` | Add global SSE infrastructure + `Monitor` field |
| Modify | `internal/api/handler.go` | Register `GET /api/v1/hosts/statuses` and `GET /api/v1/stream` |
| Create | `internal/api/monitor.go` | Handler functions for the two new endpoints |
| Modify | `web/src/views/ChatView.vue` | `loadDevices` + global SSE subscription + `executingHosts` set |

---

## Task 1: Add probe fields to AccessFace model and schema

**Files:**
- Modify: `internal/models/host.go`
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Add fields to AccessFace struct**

In `internal/models/host.go`, add two fields to `AccessFace` after `KnowledgeSources`:

```go
ProbePort     int `json:"probe_port,omitempty"`
ProbeInterval int `json:"probe_interval,omitempty"`
```

- [ ] **Step 2: Add columns to schema**

In `internal/db/schema.go`, find the `access_faces` CREATE TABLE statement and add before the closing `)`:

```sql
probe_port INTEGER NOT NULL DEFAULT 0,
probe_interval INTEGER NOT NULL DEFAULT 0,
```

Default 0 means "use system default" (port 22, interval 2s) — avoids breaking existing rows.

- [ ] **Step 3: Build to verify**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/models/host.go internal/db/schema.go
git commit -m "feat(monitor): add probe_port and probe_interval to AccessFace"
```

---

## Task 2: Update AccessFaceStore to read/write probe fields

**Files:**
- Modify: `internal/store/access_face_store.go`

- [ ] **Step 1: Read the current store file**

```bash
grep -n "probe\|INSERT\|UPDATE\|SELECT\|Scan" internal/store/access_face_store.go | head -40
```

Find the INSERT, UPDATE, and SELECT/Scan calls for access_faces.

- [ ] **Step 2: Add probe fields to INSERT**

Find the INSERT statement in `Create` or equivalent method. Add `probe_port, probe_interval` to the column list and `?, ?` to the values, passing `f.ProbePort, f.ProbeInterval`.

- [ ] **Step 3: Add probe fields to UPDATE**

Find the UPDATE statement. Add `probe_port = ?, probe_interval = ?` and pass the values.

- [ ] **Step 4: Add probe fields to SELECT/Scan**

Find the SELECT statement and add `probe_port, probe_interval` to the column list. Find the corresponding `Scan(...)` call and add `&f.ProbePort, &f.ProbeInterval` in the same order.

- [ ] **Step 5: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/store/access_face_store.go
git commit -m "feat(monitor): persist probe_port and probe_interval in access_faces"
```

---

## Task 3: Implement the Monitor package

**Files:**
- Create: `internal/monitor/monitor.go`

- [ ] **Step 1: Create the file**

```go
package monitor

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

const (
	defaultProbeInterval = 2 * time.Second
	defaultProbePort     = 22
	dialTimeout          = 2 * time.Second
	maxConcurrent        = 20
)

type Monitor struct {
	hostStore *store.HostStore
	faceStore *store.AccessFaceStore
	onChange  func(hostID string, online bool)

	statuses   map[string]bool
	lastProbed map[string]time.Time
	mu         sync.RWMutex

	cancel context.CancelFunc
}

func New(
	hostStore *store.HostStore,
	faceStore *store.AccessFaceStore,
	onChange func(hostID string, online bool),
) *Monitor {
	return &Monitor{
		hostStore:  hostStore,
		faceStore:  faceStore,
		onChange:   onChange,
		statuses:   make(map[string]bool),
		lastProbed: make(map[string]time.Time),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	go m.loop(ctx)
}

func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// GetStatus returns the last known status for a host. Returns true (online) if unknown.
func (m *Monitor) GetStatus(hostID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.statuses[hostID]
	if !ok {
		return true
	}
	return v
}

// Statuses returns a snapshot of all known statuses.
func (m *Monitor) Statuses() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]bool, len(m.statuses))
	for k, v := range m.statuses {
		out[k] = v
	}
	return out
}

func (m *Monitor) loop(ctx context.Context) {
	ticker := time.NewTicker(defaultProbeInterval)
	defer ticker.Stop()
	// probe immediately on start
	m.probeAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.probeAll(ctx)
		}
	}
}

func (m *Monitor) probeAll(ctx context.Context) {
	hosts, err := m.hostStore.List("")
	if err != nil {
		return
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, h := range hosts {
		h := h
		interval, port := m.probeConfig(h.ID)

		m.mu.RLock()
		last := m.lastProbed[h.ID]
		m.mu.RUnlock()

		if time.Since(last) < interval {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			m.probeOne(ctx, h, port)
		}()
	}
	wg.Wait()
}

func (m *Monitor) probeOne(ctx context.Context, h *models.Host, port int) {
	addr := net.JoinHostPort(h.IP, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	online := err == nil
	if conn != nil {
		conn.Close()
	}

	m.mu.Lock()
	prev, known := m.statuses[h.ID]
	m.statuses[h.ID] = online
	m.lastProbed[h.ID] = time.Now()
	m.mu.Unlock()

	if !known || prev != online {
		m.onChange(h.ID, online)
	}
}

func (m *Monitor) probeConfig(hostID string) (time.Duration, int) {
	faces, err := m.faceStore.ListByHost(hostID)
	if err != nil || len(faces) == 0 {
		return defaultProbeInterval, defaultProbePort
	}
	// use first SSH face
	for _, f := range faces {
		if f.Type == models.AccessFaceTypeSSH {
			interval := defaultProbeInterval
			port := defaultProbePort
			if f.ProbeInterval > 0 {
				interval = time.Duration(f.ProbeInterval) * time.Second
			}
			if f.ProbePort > 0 {
				port = f.ProbePort
			}
			return interval, port
		}
	}
	return defaultProbeInterval, defaultProbePort
}
```

- [ ] **Step 2: Add missing import**

The file uses `fmt.Sprintf` — add `"fmt"` to the import block.

- [ ] **Step 3: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/monitor/monitor.go
git commit -m "feat(monitor): implement TCP probe monitor"
```

---

## Task 4: Add global SSE infrastructure to App

**Files:**
- Modify: `internal/mcp/server.go`

- [ ] **Step 1: Add Monitor field and global SSE fields to App struct**

In `internal/mcp/server.go`, find the `App` struct. Add after `sseClients`:

```go
Monitor *monitor.Monitor

globalSSEClients   []chan []byte
globalSSEClientsMu sync.Mutex
```

- [ ] **Step 2: Add import**

Add `"github.com/spiderai/spider/internal/monitor"` to the import block in `server.go`.

- [ ] **Step 3: Add BroadcastGlobalSSE method**

After the existing `BroadcastSSE` method, add:

```go
func (a *App) AddGlobalSSEClient(ch chan []byte) {
	a.globalSSEClientsMu.Lock()
	defer a.globalSSEClientsMu.Unlock()
	a.globalSSEClients = append(a.globalSSEClients, ch)
}

func (a *App) RemoveGlobalSSEClient(ch chan []byte) {
	a.globalSSEClientsMu.Lock()
	defer a.globalSSEClientsMu.Unlock()
	clients := make([]chan []byte, 0, len(a.globalSSEClients))
	for _, c := range a.globalSSEClients {
		if c != ch {
			clients = append(clients, c)
		}
	}
	a.globalSSEClients = clients
}

func (a *App) BroadcastGlobalSSE(data []byte) {
	a.globalSSEClientsMu.Lock()
	defer a.globalSSEClientsMu.Unlock()
	for _, ch := range a.globalSSEClients {
		select {
		case ch <- data:
		default:
		}
	}
}
```

- [ ] **Step 4: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go
git commit -m "feat(monitor): add global SSE infrastructure to App"
```

---

---

## Task 5: Wire Monitor into app startup

**Files:**
- Modify: `cmd/spider/main.go` (App is constructed at line ~195 with `ShutdownCtx`)

- [ ] **Step 1: Initialize and start Monitor**

In `cmd/spider/main.go`, after the line `ShutdownCtx: shutdownCtx,` where the App struct is built, add:

```go
app.Monitor = monitor.New(
    app.HostStore,
    app.AccessFaceStore,
    func(hostID string, online bool) {
        data, _ := json.Marshal(map[string]any{
            "type": "host_status",
            "content": map[string]any{
                "host_id": hostID,
                "online":  online,
            },
        })
        app.BroadcastGlobalSSE(data)
    },
)
app.Monitor.Start(app.ShutdownCtx)
```

- [ ] **Step 3: Add imports**

Add `"encoding/json"` and `"github.com/spiderai/spider/internal/monitor"` where needed.

- [ ] **Step 4: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add main.go  # or whichever file was modified
git commit -m "feat(monitor): start Monitor at app init"
```

---

## Task 6: Add API handlers for /api/v1/hosts/statuses and /api/v1/stream

**Files:**
- Create: `internal/api/monitor.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Create handler file**

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func hostStatuses(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	if app.Monitor == nil {
		writeJSON(w, 200, []any{})
		return
	}
	statuses := app.Monitor.Statuses()
	type item struct {
		HostID    string    `json:"host_id"`
		Online    bool      `json:"online"`
		CheckedAt time.Time `json:"checked_at"`
	}
	out := make([]item, 0, len(statuses))
	for id, online := range statuses {
		out = append(out, item{HostID: id, Online: online, CheckedAt: time.Now()})
	}
	writeJSON(w, 200, out)
}

func globalStream(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 32)
	app.AddGlobalSSEClient(ch)
	defer app.RemoveGlobalSSEClient(ch)

	// send initial ping to confirm connection
	fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 2: Register routes in handler.go**

In `internal/api/handler.go`, find where `/api/v1/hosts/` is handled. Add a case for `statuses` path (before the `{id}` catch-all):

```go
// GET /api/v1/hosts/statuses  — must be registered BEFORE the /{id} handler
mux.HandleFunc("/api/v1/hosts/statuses", authmw.RequireAuth(app)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        hostStatuses(app, w, r)
    }
})))
```

Also register the global stream:

```go
mux.HandleFunc("/api/v1/stream", authmw.RequireAuth(app)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        globalStream(app, w, r)
    }
})))
```

- [ ] **Step 3: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/api/monitor.go internal/api/handler.go
git commit -m "feat(monitor): add /api/v1/hosts/statuses and /api/v1/stream endpoints"
```

---

## Task 7: Frontend — loadDevices + global SSE subscription

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Add hostStatuses API call to chat.ts**

In `web/src/api/chat.ts`, add:

```ts
export interface HostStatusItem {
  host_id: string
  online: boolean
  checked_at: string
}

export async function getHostStatuses(): Promise<HostStatusItem[]> {
  const res = await fetch('/api/v1/hosts/statuses', { headers: authHeaders() })
  if (!res.ok) return []
  return res.json()
}
```

- [ ] **Step 2: Update loadDevices in ChatView.vue**

Replace the existing `loadDevices` function:

```ts
async function loadDevices() {
  const [hosts, statuses] = await Promise.all([
    listHosts(),
    getHostStatuses(),
  ])
  allHosts.value = hosts
  const statusMap = new Map(statuses.map(s => [s.host_id, s.online]))
  devices.value = hosts.map(h => ({
    id: h.id, name: h.name, ip: h.ip,
    vendor: '',
    status: (statusMap.get(h.id) === false ? 'offline' : 'online') as DeviceStatus['status'],
  }))
}
```

- [ ] **Step 3: Add executingHosts set and monitorStatuses map**

After the `devices` and `allHosts` declarations (around line 202), add:

```ts
const executingHosts = new Set<string>()
const monitorStatuses = new Map<string, boolean>() // hostID -> online
```

- [ ] **Step 4: Update markDevicesExecuting to track executingHosts**

Find `markDevicesExecuting` and add tracking:

```ts
function markDevicesExecuting(hostNames: string[]) {
  for (const name of hostNames) {
    const d = devices.value.find(d => d.name === name)
    if (d) executingHosts.add(d.id)
    const t = deviceResetTimers.get(name)
    if (t) { clearTimeout(t); deviceResetTimers.delete(name) }
    setDeviceStatus(name, 'executing')
  }
}
```

- [ ] **Step 5: Update markDevicesDone to use monitorStatuses**

```ts
function markDevicesDone(hostNames: string[], failed: boolean) {
  const finalStatus = failed ? 'failed' : 'success'
  for (const name of hostNames) {
    setDeviceStatus(name, finalStatus)
    const d = devices.value.find(d => d.name === name)
    const t = setTimeout(() => {
      if (d) {
        executingHosts.delete(d.id)
        const monitorOnline = d ? monitorStatuses.get(d.id) : undefined
        setDeviceStatus(name, monitorOnline === false ? 'offline' : 'online')
      }
      deviceResetTimers.delete(name)
    }, 2000)
    deviceResetTimers.set(name, t)
  }
}
```

- [ ] **Step 6: Add global SSE subscription in onMounted**

Find `onMounted` in ChatView.vue. Add global SSE setup before the existing initialization calls:

```ts
// Global SSE for host status updates
let globalEs: EventSource | null = null
function startGlobalSSE() {
  globalEs = new EventSource('/api/v1/stream')
  globalEs.onmessage = (e) => {
    try {
      const event = JSON.parse(e.data)
      if (event.type === 'host_status') {
        const { host_id, online } = event.content
        monitorStatuses.set(host_id, online)
        if (!executingHosts.has(host_id)) {
          const idx = devices.value.findIndex(d => d.id === host_id)
          if (idx !== -1) {
            devices.value = devices.value.map((d, i) =>
              i === idx ? { ...d, status: online ? 'online' : 'offline' } : d
            )
          }
        }
      }
    } catch { /* skip malformed */ }
  }
  globalEs.onerror = () => { /* auto-reconnects */ }
}
```

Call `startGlobalSSE()` inside `onMounted`.

- [ ] **Step 7: Close global SSE on unmount**

In `onUnmounted` (or add if missing):

```ts
onUnmounted(() => {
  globalEs?.close()
  // ... existing cleanup
})
```

- [ ] **Step 8: Add import**

Add `getHostStatuses` to the import from `'../api/chat'`.

- [ ] **Step 9: Build frontend**

```bash
cd web && npm run build 2>&1 | tail -20
```

Expected: no TypeScript errors, build succeeds.

- [ ] **Step 10: Commit**

```bash
git add web/src/api/chat.ts web/src/views/ChatView.vue
git commit -m "feat(monitor): frontend loadDevices + global SSE host status updates"
```

---

## Task 8: Manual verification

- [ ] **Step 1: Build and run**

```bash
go run ./cmd/spider serve --addr :8090 --data-dir ~/.spider/data
```

In a separate terminal:
```bash
cd web && npm run dev
```

- [ ] **Step 2: Open browser at http://localhost:5173/chat**

Verify heat matrix shows correct colors (online hosts green, any offline hosts grey).

- [ ] **Step 3: Verify /api/v1/hosts/statuses**

```bash
curl -s -b ~/.spider/data/session.cookie http://localhost:8090/api/v1/hosts/statuses | jq .
```

Expected: array of `{ host_id, online, checked_at }` objects.

- [ ] **Step 4: Verify SSE stream**

```bash
curl -s -N -b ~/.spider/data/session.cookie http://localhost:8090/api/v1/stream
```

Expected: `data: {"type":"ping"}` immediately, then `data: {"type":"host_status",...}` events as statuses change.

- [ ] **Step 5: Simulate offline**

Block a host's port temporarily (or use a non-existent IP in test data). Verify heat matrix cell turns grey within 2 seconds.

- [ ] **Step 6: Final commit if any fixes needed**

```bash
git add -p
git commit -m "fix(monitor): <describe fix>"
```


---

## Task 9: Access face UI — add probe config fields

**Files:**
- Modify: `web/src/views/HostsView.vue` (or wherever the access face editor form is)

- [ ] **Step 1: Find the access face editor**

```bash
grep -n "probe_port\|ProbePort\|access.face\|AccessFace\|ssh_auth" web/src/views/HostsView.vue | head -20
```

Locate the form section for SSH access face fields.

- [ ] **Step 2: Add probe port field**

In the SSH access face form, after the existing port field, add:

```html
<div class="form-row">
  <label>存活探测端口</label>
  <input
    type="number"
    v-model.number="face.probe_port"
    placeholder="22"
    min="1"
    max="65535"
  />
  <span class="hint">默认 22，留空使用默认值</span>
</div>
```

- [ ] **Step 3: Add probe interval field**

After the probe port field:

```html
<div class="form-row">
  <label>探测间隔（秒）</label>
  <input
    type="number"
    v-model.number="face.probe_interval"
    placeholder="2"
    min="1"
    max="3600"
  />
  <span class="hint">默认 2 秒</span>
</div>
```

- [ ] **Step 4: Ensure face object includes probe fields**

Find where the face object is initialized (new face form). Make sure `probe_port` and `probe_interval` are included with default 0:

```ts
const emptyFace = () => ({
  // ... existing fields ...
  probe_port: 0,
  probe_interval: 0,
})
```

- [ ] **Step 5: Build frontend**

```bash
cd web && npm run build 2>&1 | tail -10
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/views/HostsView.vue
git commit -m "feat(monitor): add probe_port and probe_interval to access face UI"
```
