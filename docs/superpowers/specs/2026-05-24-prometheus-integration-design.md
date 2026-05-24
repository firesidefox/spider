# Prometheus Integration Design

**Date:** 2026-05-24  
**Status:** Approved

## Overview

Add Prometheus query integration to spider.ai. Prometheus instances are configured as a new `AccessFace` type (`prometheus`) on individual hosts. One agent tool exposes free PromQL queries. Alert integration is tracked separately in the PRD.

## 1. Data Model

### New AccessFace type

```go
// internal/models/host.go
FacePrometheus AccessFaceType = "prometheus"
```

No new DB columns. Existing `AccessFace` fields cover all Prometheus needs:

| AccessFace field     | Prometheus use              |
|----------------------|-----------------------------|
| `base_url`           | Prometheus base URL (e.g. `http://prom:9090`) |
| `rest_auth_type`     | `none` \| `bearer` \| `basic` |
| `encrypted_credential` | Bearer token or Basic credentials |

### Constraints

- A prometheus face does not use `ip`, `port`, `username`, or any SSH fields.
- `base_url` is required; validation rejects empty `base_url` for prometheus faces.
- A host may have at most one prometheus face (enforced at store layer).

## 2. Agent Tools

### `QueryMetrics`

Execute a free PromQL expression against the Prometheus instance bound to a host.

**Input schema:**

```json
{
  "host_id":  "string (required) — host with a prometheus face",
  "query":    "string (required) — PromQL expression",
  "start":    "string (optional) — RFC3339 or Unix timestamp; triggers range query",
  "end":      "string (optional) — RFC3339 or Unix timestamp",
  "step":     "string (optional) — duration string, e.g. '1m', '30s'"
}
```

**Behavior:**
- If `start`/`end` omitted → instant query (`GET /api/v1/query`)
- If `start`/`end` present → range query (`GET /api/v1/query_range`)
- Returns raw Prometheus JSON (`resultType` + `result`)
- Risk level: `L1` (read-only)
- Concurrency safe: yes

**System prompt guidance:**
- Use after `GetHosts` to identify which host has the prometheus face
- Prefer instant queries for current state; range queries for trend analysis
- Do not construct queries wider than needed (avoid `{}` without label matchers on large clusters)

## 3. Internal Prometheus Client

Shared client used by the agent tool:

```go
// internal/prometheus/client.go
type Client struct { baseURL, authType, token string }
func NewClient(face *models.AccessFace, decryptedCred string) *Client
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error)
func (c *Client) QueryRange(ctx context.Context, query, start, end, step string) (*QueryResult, error)
```

Thin HTTP wrapper. No retry logic. Timeout: 30s default (context-cancelable).

## 4. Out of Scope

- Prometheus metric discovery / autocomplete
- Grafana integration
- Alert integration (GetAlerts tool, Alertmanager webhook, auto-task creation) — tracked in PRD
- Multiple prometheus faces per host
- Topology-level Prometheus config (deferred)
