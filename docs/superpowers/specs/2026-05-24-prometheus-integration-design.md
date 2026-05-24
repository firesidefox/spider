# Prometheus Integration Design

**Date:** 2026-05-24  
**Status:** Approved

## Overview

Add Prometheus query integration to spider.ai. Prometheus is modeled as a monitoring data source (`prometheus_sources`), separate from `AccessFace` (which expresses access channels, not observation sources). Sources are scoped to a topology layer or individual host. One agent tool executes free PromQL queries. Alert integration is tracked separately in the PRD.

## 1. Data Model

### 1.1 `prometheus_sources`

Defines a Prometheus instance (URL + auth). No scope here — scope is in bindings.

```sql
CREATE TABLE prometheus_sources (
  id                    TEXT PRIMARY KEY,
  name                  TEXT NOT NULL,
  base_url              TEXT NOT NULL,           -- e.g. http://prom:9090
  auth_type             TEXT NOT NULL DEFAULT 'none',  -- none | bearer | basic
  encrypted_credential  TEXT NOT NULL DEFAULT '', -- encrypted via crypto.Manager
  created_at            DATETIME NOT NULL,
  updated_at            DATETIME NOT NULL
)
```

Go model field: `EncryptedCredential` (DB column: `encrypted_credential`), same pattern as `AccessFace.EncryptedCred`.

### 1.2 `prometheus_bindings`

Binds a source to a scope. Two scope types:

| scope_type | covers | unique constraint |
|---|---|---|
| `topology_layer` | all topology nodes in `(topology_id, layer)` | UNIQUE(topology_id, layer) |
| `host` | one specific host (override) | UNIQUE(host_id) |

```sql
CREATE TABLE prometheus_bindings (
  id           TEXT PRIMARY KEY,
  source_id    TEXT NOT NULL REFERENCES prometheus_sources(id) ON DELETE CASCADE,
  scope_type   TEXT NOT NULL CHECK (scope_type IN ('topology_layer', 'host')),
  topology_id  TEXT REFERENCES topologies(id) ON DELETE CASCADE,  -- scope_type=topology_layer
  layer        TEXT,                                               -- scope_type=topology_layer
  host_id      TEXT REFERENCES hosts(id) ON DELETE CASCADE,       -- scope_type=host
  created_at   DATETIME NOT NULL
)
CREATE UNIQUE INDEX idx_pb_topology_layer ON prometheus_bindings(topology_id, layer)
  WHERE scope_type = 'topology_layer';
CREATE UNIQUE INDEX idx_pb_host ON prometheus_bindings(host_id)
  WHERE scope_type = 'host';
```

### 1.3 Source lookup for a given host

Priority: host binding first, topology+layer binding as fallback.

```
1. SELECT source via binding WHERE scope_type='host' AND host_id=?
2. if not found:
     find topology_node WHERE host_id=? → get (topology_id, layer)
     SELECT source via binding WHERE scope_type='topology_layer'
       AND topology_id=? AND layer=?
3. if still not found: return error "no Prometheus source configured for this host"
```

## 2. Configuration UI

| Scope | Entry point | Action |
|---|---|---|
| Topology layer | Topology detail page → layer row | Configure which Prometheus source covers this layer |
| Host override | Host detail page → "监控源" section (separate from AccessFace list) | Bind a specific source to this host |

## 3. Agent Tool: `QueryMetrics`

Execute a free PromQL expression for a given host.

**Input schema:**

```json
{
  "host_id": "string (required) — host whose metrics to query",
  "query":   "string (required) — PromQL expression",
  "start":   "string (optional) — RFC3339 or Unix timestamp",
  "end":     "string (optional) — RFC3339 or Unix timestamp",
  "step":    "string (optional) — duration string, e.g. '1m', '30s'"
}
```

**Behavior:**
- `start` and `end` must both be present or both absent; providing only one is an error
- If absent → instant query (`GET /api/v1/query`)
- If present → range query (`GET /api/v1/query_range`)
- `step` default: `(end - start) / 100`, minimum 1s
- Max time window: 7 days; max data points: 10,000 — exceeded → error, not truncation
- Source resolved via §1.3 lookup; error if no source found
- Risk level: `L1` (read-only), concurrency safe

**Output:** summarized result (not raw JSON)
- `result_type`, `series_count`, per-series: `metric` labels, `latest`, `min`, `max`, `avg`, up to 20 samples
- `raw: true` input parameter returns full Prometheus JSON (for debugging)

**System prompt guidance:**
- Resolve the host's Prometheus source automatically — do not ask user for Prometheus URL
- Construct PromQL with host IP as label selector, e.g. `node_cpu_seconds_total{instance="<IP>:9100"}`
- Instant query for current state; range query for trend analysis
- Avoid label-less queries (`{}`) on large clusters

## 4. Internal Prometheus Client

```go
// internal/prometheus/client.go
type Client struct { baseURL, authType, credential string }
func NewClient(source *models.PrometheusSource, decryptedCred string) *Client
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error)
func (c *Client) QueryRange(ctx context.Context, query, start, end, step string) (*QueryResult, error)
```

Thin HTTP wrapper. No retry. Timeout: 30s (context-cancelable).

## 5. Out of Scope

- Prometheus metric discovery / autocomplete
- Grafana integration
- Alert integration (GetAlerts tool, Alertmanager webhook) — tracked in PRD
- Multiple bindings per host (one override per host only)
